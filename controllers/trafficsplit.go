package controllers

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	accessv1alpha1 "github.com/deislabs/smi-sdk-go/pkg/apis/access/v1alpha1"
	splitv1alpha1 "github.com/deislabs/smi-sdk-go/pkg/apis/split/v1alpha1"
	splitClientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/split/clientset/versioned"
	splitScheme "github.com/deislabs/smi-sdk-go/pkg/gen/client/split/clientset/versioned/scheme"
	splitInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/split/informers/externalversions/split/v1alpha1"
	splitListers "github.com/deislabs/smi-sdk-go/pkg/gen/client/split/listers/split/v1alpha1"
	"github.com/hashicorp/consul-smi-controller/clients"
)

// TrafficSplitAgentName is the name of the controller
const TrafficSplitAgentName = "trafficsplit-controller"

// TrafficSplit is the controller implementation for TrafficSplit resources
type TrafficSplit struct {
	// kubeClient is a standard kubernetes clientset
	kubeClient kubernetes.Interface
	// accessClient is a clientset for our own API group
	splitClient splitClientset.Interface

	splitLister splitListers.TrafficSplitLister
	splitSynced cache.InformerSynced
	// stores deleted objects
	deletedIndexer cache.Indexer

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
	// consulClient is a client for interacting with Consul
	consulClient clients.Consul
}

// NewTrafficSplit returns a new sample controller
func NewTrafficSplit(
	kubeClient kubernetes.Interface,
	splitClient splitClientset.Interface,
	splitInformer splitInformers.TrafficSplitInformer,
	deletedIndexer cache.Indexer,
	consulClient clients.Consul) *TrafficSplit {

	// Create event broadcaster
	// Add controller types to the default Kubernetes Scheme so Events can be
	// logged for controller types.
	utilruntime.Must(splitScheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: TrafficSplitAgentName})

	controller := &TrafficSplit{
		kubeClient:     kubeClient,
		splitClient:    splitClient,
		splitLister:    splitInformer.Lister(),
		splitSynced:    splitInformer.Informer().HasSynced,
		deletedIndexer: deletedIndexer,
		workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), TrafficSplitAgentName),
		recorder:       recorder,
		consulClient:   consulClient,
	}

	klog.Info("Setting up event handlers")
	splitInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.enqueueTarget,
		DeleteFunc: controller.enqueueDeleted,
		UpdateFunc: func(old, new interface{}) {
			if old == new {
				klog.Info("Skipping update, old instance is the same as the new instance")
				return
			}

			controller.enqueueTarget(new)
		},
	})

	return controller
}

func (c *TrafficSplit) enqueueTarget(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.workqueue.Add(key)
}

func (c *TrafficSplit) enqueueDeleted(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	dt, ok := obj.(*splitv1alpha1.TrafficSplit)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("Unabled to enqueue deleted item, unable to cast"))
		return
	}

	c.deletedIndexer.Add(dt)
	c.workqueue.Add(key)
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *TrafficSplit) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.splitSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Infof("Starting %d workers", threadiness)
	// Launch n workers to process resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *TrafficSplit) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *TrafficSplit) processNextWorkItem() bool {
	klog.Info("processNextWorkItem")

	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// TrafficTarget resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *TrafficSplit) syncHandler(key string) error {
	// are we doing a delete?
	// we need to track this separately as any mutation to the
	// TrafficTarget changes the hash which means we can not
	// delete it from the cache
	deleteOperation := false

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	klog.Infof("syncHandler: key: %s name: %s", key, name)

	// Get the TrafficSplit resource with this namespace/name
	var ts *splitv1alpha1.TrafficSplit

	ts, err = c.splitLister.TrafficSplits(namespace).Get(name)
	if err != nil {
		// The TrafficTarget resource may no longer exist, in which case we stop
		// processing.
		if !errors.IsNotFound(err) {
			return err
		}

		// check to see if we have a deleted item

		item, exists, err := c.deletedIndexer.GetByKey(key)

		if !exists || err != nil {
			utilruntime.HandleError(fmt.Errorf("trafficsplit '%s' in work queue no longer exists", key))
			return nil
		}

		klog.Info("Found deleted item", item)
		var ok bool
		ts, ok = item.(*splitv1alpha1.TrafficSplit)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("unable to cast '%s' to TrafficSplit", key))
			c.deletedIndexer.Delete(key)
			return nil
		}

		deleteOperation = true
	}

	/*
		// Sync the current state with the desired state
		err = c.consulClient.SyncIntentions(fromServices, toService)
		if err != nil {

			// ignore the error settings the status, we need to return the underlying error
			klog.Infof("Setting status: %s", accessv1alpha1.StatusPending)
			c.setStatus(tt, accessv1alpha1.StatusPending)
			c.recorder.Event(tt, corev1.EventTypeNormal, ErrSyncingIntentions, fmt.Sprintf(MessageResourceSyncFailed, tt.Namespace, tt.Name, err.Error()))

			// re-add to the queue
			utilruntime.HandleError(fmt.Errorf("Unable to sync intentions: %s", err.Error()))
			return err
		}
	*/

	// So set status to created if not deleted item
	if deleteOperation {
		// get the original object using the key then delete
		item, _, err := c.deletedIndexer.GetByKey(key)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to remove deleted item from cache: %s", err.Error()))
			return nil
		}

		c.deletedIndexer.Delete(item)
		return nil
	}

	//Not a deleted item so set the status
	klog.Infof("Setting status: %s", splitv1alpha1.StatusCreated)
	err = c.setStatus(ts, splitv1alpha1.StatusCreated)
	if err != nil {
		return err
	}

	c.recorder.Event(ts, corev1.EventTypeNormal, SuccessSynced, fmt.Sprintf(MessageResourceSynced, ts.Namespace, ts.Name))

	return nil
}

func (c *TrafficSplit) setStatus(target *splitv1alpha1.TrafficSplit, status accessv1alpha1.Status) error {
	targetCopy := target.DeepCopy()
	targetCopy.Status = status

	_, err := c.splitClient.SplitV1alpha1().TrafficSplits(target.Namespace).Update(targetCopy)
	return err
}
