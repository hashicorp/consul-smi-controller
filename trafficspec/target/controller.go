/*
Copyright 2017 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package target

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// specsv1alpha1 "github.com/deislabs/smi-sdk-go/pkg/apis/specs/v1alpha1"
	accessClientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/clientset/versioned"
	accessScheme "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/clientset/versioned/scheme"
	accessInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/informers/externalversions/access/v1alpha1"
	accessListers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/listers/access/v1alpha1"
	"github.com/hashicorp/consul-smi/clients"
)

const controllerAgentName = "traffic-controller"

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
	kubeclientset   kubernetes.Interface
	accessClientset accessClientset.Interface
	targetLister    accessListers.TrafficTargetLister
	targetSynced    cache.InformerSynced
	workqueue       workqueue.RateLimitingInterface
	recorder        record.EventRecorder
	consulClient    clients.Consul
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	accessClientset accessClientset.Interface,
	targetInformer accessInformers.TrafficTargetInformer,
	consulClient clients.Consul,
) *Controller {
	// Create event broadcaster
	// Add controller types to the default Kubernetes Scheme so Events can be
	// logged for controller types.
	utilruntime.Must(accessScheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:   kubeclientset,
		accessClientset: accessClientset,
		targetLister:    targetInformer.Lister(),
		targetSynced:    targetInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerAgentName),
		recorder:        recorder,
		consulClient:    consulClient,
	}

	klog.Info("Setting up event handlers")
	targetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			target := obj.(*accessv1alpha1.TrafficTarget)

			if target.Status != accessv1alpha1.StatusCreated {
				controller.setStatus(target, accessv1alpha1.StatusPending)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			newTarget := new.(*accessv1alpha1.TrafficTarget)
			oldTarget := old.(*accessv1alpha1.TrafficTarget)
			if newTarget.ResourceVersion == oldTarget.ResourceVersion {
				return
			}

			/// I bet here setting the status is double queuing
			if oldTarget.Status == accessv1alpha1.StatusPending && newTarget.Status == accessv1alpha1.StatusCreated {
				fmt.Println("Ignore")
				return
			}

			newTarget.Status = accessv1alpha1.StatusPending
			controller.workqueue.Add(newTarget)
		},
		DeleteFunc: func(obj interface{}) {
			target := obj.(*accessv1alpha1.TrafficTarget)
			target.SetDeletionTimestamp(&apiv1.Time{Time: time.Now()})
			controller.workqueue.Add(target)
		},
	})

	// TODO: Create informer that listens for TCPRoute objects
	return controller
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
	// Launch two workers to process resources
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
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	defer c.workqueue.Done(obj)

	// Run the syncHandler, passing it the Resource to be synced.
	if err := c.syncHandler(obj); err != nil {
		// Put the item back on the workqueue to handle any transient errors.
		c.workqueue.AddRateLimited(obj)
		utilruntime.HandleError(fmt.Errorf("error syncing '%#v': %s, requeuing", obj, err.Error()))
	}

	// Finally, if no error occurs we Forget this item so it does not
	// get queued again until another change happens.
	c.workqueue.Forget(obj)

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncHandler(obj interface{}) error {
	// Cast the queued object to a TrafficTarget.
	currentTarget, ok := obj.(*accessv1alpha1.TrafficTarget)
	if !ok {
		return nil
	}

	// Get all targets.
	allTargets, err := c.targetLister.TrafficTargets(currentTarget.Namespace).List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to list targets: %s", err.Error()))
		return nil
	}

	toService := currentTarget.Destination.Name
	fromServices := []string{}

	// Loop over the targets and if it's the same destination, add the sources.
	for _, t := range allTargets {
		// TODO: When HTTP/GRPC routes are added, also check the spec of the TrafficTargets.
		if t.Destination.Name == currentTarget.Destination.Name {
			for _, s := range t.Sources {
				fromServices = append(fromServices, s.Name)
			}
		}
	}

	// Sync the current state with the desired state.
	err = c.consulClient.SyncIntentions(fromServices, toService)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to create intention: %s", err.Error()))
		return nil
	}

	// If the target is deleted, don't update the status.
	if currentTarget.GetDeletionTimestamp() != nil {
		return nil
	}

	// Work is done, so set status to created.
	err = c.setStatus(currentTarget, accessv1alpha1.StatusCreated)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to set status: %s", err.Error()))
		return nil

	}

	c.recorder.Event(currentTarget, corev1.EventTypeNormal, SuccessSynced, fmt.Sprintf(MessageResourceSynced, currentTarget.Namespace, currentTarget.Name))

	return nil
}

func (c *Controller) setStatus(target *accessv1alpha1.TrafficTarget, status accessv1alpha1.Status) error {
	targetCopy := target.DeepCopy()
	targetCopy.Status = status

	_, err := c.accessClientset.AccessV1alpha1().TrafficTargets(target.Namespace).Update(targetCopy)
	return err
}
