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
	trafficspecscheme "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/clientset/versioned/scheme"
	accessInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/informers/externalversions/access/v1alpha1"
	accessListers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/listers/access/v1alpha1"
	specsClientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/specs/clientset/versioned"
	specsInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/specs/informers/externalversions/specs/v1alpha1"
	specsListers "github.com/deislabs/smi-sdk-go/pkg/gen/client/specs/listers/specs/v1alpha1"
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
	specsClientset  specsClientset.Interface
	targetLister    accessListers.TrafficTargetLister
	targetSynced    cache.InformerSynced
	tcpRouteLister  specsListers.TCPRouteLister
	tcpRouteSynced  cache.InformerSynced
	workqueue       workqueue.RateLimitingInterface
	recorder        record.EventRecorder
	consulClient    clients.Consul
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	accessClientset accessClientset.Interface,
	specsClientset specsClientset.Interface,
	targetInformer accessInformers.TrafficTargetInformer,
	tcpRouteInformer specsInformers.TCPRouteInformer,
	consulClient clients.Consul,
) *Controller {
	// Create event broadcaster
	// Add smi-controller types to the default Kubernetes Scheme so Events can be
	// logged for smi-controller types.
	utilruntime.Must(trafficspecscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:   kubeclientset,
		accessClientset: accessClientset,
		specsClientset:  specsClientset,
		targetLister:    targetInformer.Lister(),
		targetSynced:    targetInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerAgentName),
		recorder:        recorder,
		consulClient:    consulClient,
	}

	klog.Info("Setting up event handlers")
	targetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			fmt.Println("Add func")
			target := obj.(*accessv1alpha1.TrafficTarget)

			if target.Status != accessv1alpha1.StatusCreated {
				controller.setStatus(target, accessv1alpha1.StatusPending)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			fmt.Println("Update func")

			newTarget := new.(*accessv1alpha1.TrafficTarget)
			oldTarget := old.(*accessv1alpha1.TrafficTarget)
			if newTarget.ResourceVersion == oldTarget.ResourceVersion {
				return
			}

			if oldTarget.Status == accessv1alpha1.StatusPending && newTarget.Status == accessv1alpha1.StatusCreated {
				return
			}

			controller.setStatus(newTarget, accessv1alpha1.StatusPending)
			controller.workqueue.Add(newTarget)
		},
		DeleteFunc: func(obj interface{}) {
			fmt.Println("Del func")

			target := obj.(*accessv1alpha1.TrafficTarget)
			target.SetDeletionTimestamp(&apiv1.Time{Time: time.Now()})
			controller.workqueue.Add(target)
			// TO
			// Get the TrafficTarget name and look it up.
			// Get the pods that are referenced by the TrafficTarget.
			// Look for a service account for that pod.
			// Look up the service name through Consul Auth Method API.

			// FROM
			// Get the Subjects from the IdentifyBinding.
			// Get the ServiceAccount from the Subjects and look up in Consul (Auth Method API) to get the service name.

			// Intention
			// Delete an intention that allows traffic from "FROM" to "TO"
		},
	})

	// TODO: Create informer that listens for TCPRoute objects
	// Informer will just log create/update/delete
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

	klog.Info("Starting workers")
	// Launch two workers to process resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
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

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer c.workqueue.Done(obj)

	// var key string
	// var ok bool
	// // We expect strings to come off the workqueue. These are of the
	// // form namespace/name. We do this as the delayed nature of the
	// // workqueue means the items in the informer cache may actually be
	// // more up to date that when the item was initially put onto the
	// // workqueue.
	// if key, ok = obj.(string); !ok {
	// 	// As the item in the workqueue is actually invalid, we call
	// 	// Forget here else we'd go into a loop of attempting to
	// 	// process a work item that is invalid.
	// 	c.workqueue.Forget(obj)
	// 	utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
	// }

	// Run the syncHandler, passing it the namespace/name string of the
	// Resource to be synced.
	if err := c.syncHandler(obj); err != nil {
		// Put the item back on the workqueue to handle any transient errors.
		c.workqueue.AddRateLimited(obj)
		utilruntime.HandleError(fmt.Errorf("error syncing '%#v': %s, requeuing", obj, err.Error()))
	}

	// Finally, if no error occurs we Forget this item so it does not
	// get queued again until another change happens.
	c.workqueue.Forget(obj)
	klog.Infof("Successfully synced '%s'", obj)

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

	klog.Infof("allstargets: %#v", allTargets)

	toService := currentTarget.Destination.Name

	klog.Infof("toservice: %s", toService)

	fromServices := []string{}
	// Loop over the targets.
	for _, t := range allTargets {
		// TODO: When HTTP/GRPC routes are added, also check the spec of the TrafficTargets.
		// If the targets are the same
		if t.Destination.Name == currentTarget.Destination.Name {
			// Add the sources of the target to the from list.
			for _, s := range t.Sources {
				fromServices = append(fromServices, s.Name)
			}
		}
	}

	// klog.Infof("Creating intention from %s to %s", fromServices, toService)
	err = c.consulClient.SyncIntentions(fromServices, toService)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to create intention: %s", err.Error()))
		return nil
	}

	klog.Infof("I'm dead: %#v", currentTarget.GetDeletionTimestamp())
	if currentTarget.GetDeletionTimestamp() != nil {
		return nil
	}

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
