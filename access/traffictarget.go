package access

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
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
	accessClientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/clientset/versioned"
	accessScheme "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/clientset/versioned/scheme"
	accessInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/informers/externalversions/access/v1alpha1"
	accessListers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/listers/access/v1alpha1"
	"github.com/hashicorp/consul-smi/clients"
)

const controllerAgentName = "traffictarget-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "%s/%s synced successfully"
)

// Controller is the controller implementation for Foo resources
type Controller struct {
	// kubeClient is a standard kubernetes clientset
	kubeClient kubernetes.Interface
	// accessClient is a clientset for our own API group
	accessClient accessClientset.Interface

	targetLister accessListers.TrafficTargetLister
	targetSynced cache.InformerSynced

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

// NewController returns a new sample controller
func NewController(
	kubeClient kubernetes.Interface,
	accessClient accessClientset.Interface,
	targetInformer accessInformers.TrafficTargetInformer,
	deletedIndexer cache.Indexer,
	consulClient clients.Consul) *Controller {

	// Create event broadcaster
	// Add controller types to the default Kubernetes Scheme so Events can be
	// logged for controller types.
	utilruntime.Must(accessScheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeClient:     kubeClient,
		accessClient:   accessClient,
		targetLister:   targetInformer.Lister(),
		targetSynced:   targetInformer.Informer().HasSynced,
		deletedIndexer: deletedIndexer,
		workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "TrafficTargets"),
		recorder:       recorder,
		consulClient:   consulClient,
	}

	klog.Info("Setting up event handlers")
	targetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.enqueueTarget,
		DeleteFunc: controller.enqueueDeleted,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueTarget(new)
		},
	})

	return controller
}

func (c *Controller) enqueueTarget(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.workqueue.Add(key)
}

func (c *Controller) enqueueDeleted(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	dt, ok := obj.(*accessv1alpha1.TrafficTarget)
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
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.targetSynced); !ok {
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
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
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
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	klog.Infof("syncHandler: key: %s name: %s", key, name)

	// Get the TrafficTarget resource with this namespace/name
	var tt *accessv1alpha1.TrafficTarget

	tt, err = c.targetLister.TrafficTargets(namespace).Get(name)
	if err != nil {
		// The TrafficTarget resource may no longer exist, in which case we stop
		// processing.
		if !errors.IsNotFound(err) {
			return err
		}

		// check to see if we have a deleted item

		item, exists, err := c.deletedIndexer.GetByKey(key)

		if !exists || err != nil {
			utilruntime.HandleError(fmt.Errorf("traffictarget '%s' in work queue no longer exists", key))
			return nil
		}

		klog.Info("Found deleted item", item)
		var ok bool
		tt, ok = item.(*accessv1alpha1.TrafficTarget)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("unable to cast '%s' to TrafficTarget", key))
			c.deletedIndexer.Delete(key)
			return nil
		}

		tt.Status = "Deleted"
	}

	// Get all targets.
	allTargets, err := c.targetLister.TrafficTargets(tt.Namespace).List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to list targets: %s", err.Error()))
		return nil
	}

	toService := tt.Destination.Name
	fromServices := []string{}

	// Loop over the targets and if it's the same destination, add the sources.
	for _, t := range allTargets {
		if t.Destination.Name == tt.Destination.Name {
			for _, s := range t.Sources {
				fromServices = append(fromServices, s.Name)
			}
		}
	}

	klog.Infof("Syncing Intentions sources: %v, destination: %s", fromServices, toService)
	// Sync the current state with the desired state
	err = c.consulClient.SyncIntentions(fromServices, toService)
	if err != nil {
		// re-add to the queue
		return nil
	}

	// Work is done, so set status to created if not deleted item
	klog.Info("Status " + tt.Status)
	if tt.Status == "Deleted" {
		// delete the cache
		c.deletedIndexer.Delete(key)
		return nil
	}

	//Not a deleted item so set the status
	klog.Infof("Setting status: %s", accessv1alpha1.StatusCreated)
	err = c.setStatus(tt, accessv1alpha1.StatusCreated)
	if err != nil {
		return err
	}
	c.recorder.Event(tt, corev1.EventTypeNormal, SuccessSynced, fmt.Sprintf(MessageResourceSynced, tt.Namespace, tt.Name))

	return nil
}

func (c *Controller) setStatus(target *accessv1alpha1.TrafficTarget, status accessv1alpha1.Status) error {
	targetCopy := target.DeepCopy()
	targetCopy.Status = status

	_, err := c.accessClient.AccessV1alpha1().TrafficTargets(target.Namespace).Update(targetCopy)
	return err
}
