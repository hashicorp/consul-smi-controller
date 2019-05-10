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
package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	accessv1alpha1 "github.com/deislabs/smi-sdk-go/pkg/apis/access/v1alpha1"
	accessClientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/clientset/versioned"
	trafficspecscheme "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/clientset/versioned/scheme"
	accessInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/informers/externalversions/access/v1alpha1"
	accessListers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/listers/access/v1alpha1"
	specsClientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/specs/clientset/versioned"
	specsInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/specs/informers/externalversions/specs/v1alpha1"
	specsListers "github.com/deislabs/smi-sdk-go/pkg/gen/client/specs/listers/specs/v1alpha1"
	"github.com/hashicorp/consul-smi/clients"
)

const controllerAgentName = "smi-controller"

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
	MessageResourceSynced = "Foo synced successfully"
)

// Controller is the controller implementation for Foo resources
type Controller struct {
	kubeclientset   kubernetes.Interface
	accessClientset accessClientset.Interface
	specsClientset  specsClientset.Interface
	bindingLister   accessListers.IdentityBindingLister
	bindingSynced   cache.InformerSynced
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
	bindingInformer accessInformers.IdentityBindingInformer,
	tcpRouteInformer specsInformers.TCPRouteInformer,
	consulClient clients.Consul,
) *Controller {

	klog.InitFlags(nil)
	flag.Set("v", "4")

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
		bindingLister:   bindingInformer.Lister(),
		bindingSynced:   bindingInformer.Informer().HasSynced,
		targetLister:    targetInformer.Lister(),
		targetSynced:    targetInformer.Informer().HasSynced,
		tcpRouteLister:  tcpRouteInformer.Lister(),
		tcpRouteSynced:  tcpRouteInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "traffic-controller"),
		recorder:        recorder,
		consulClient:    consulClient,
	}

	klog.Info("Setting up event handlers")
	targetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Info("TrafficTarget created")
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				controller.workqueue.Add(key)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			klog.Info("TrafficTarget updated")
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err != nil {
				controller.workqueue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			// TODO think what to do (probably clean up bindings that were referenced)
			klog.Info("TrafficTarget deleted")
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				controller.workqueue.Add(key)
			}
		},
	})

	bindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Info("IdentifyBinding created")
			binding := obj.(*accessv1alpha1.IdentityBinding)

			// TO
			target, err := controller.targetLister.TrafficTargets(binding.Namespace).Get(binding.TargetRef.Name)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("No TrafficTarget found for IdentifyBinding %s/%s: %s", binding.Namespace, binding.TargetRef.Name, err))
				return
			}

			labelSelectors := []string{}
			for k, v := range target.Selector.MatchLabels {
				labelSelectors = append(labelSelectors, fmt.Sprintf("%s=%s", k, v))
			}

			targetLabels := strings.Join(labelSelectors, ",")

			// Get a "list" of pods that have the correct labels.
			targetPods, err := kubeclientset.CoreV1().Pods(binding.Namespace).List(v1.ListOptions{
				LabelSelector: targetLabels,
				Limit:         1,
			})

			// TODO: make this a bit more robust ...
			// TODO: should we throw an error if multiple pods match the label? ... or should we create intention rules for each of them?
			if targetPods.Size() == 0 {
				utilruntime.HandleError(fmt.Errorf("No Pods match the labels defined in the TrafficTarget: %s", targetLabels))
			}

			toService := targetPods.Items[0].Spec.ServiceAccountName
			fromServices := make([]string, 0)

			// FROM
			// Get the Subjects from the IdentifyBinding.
			bindingSubjects := binding.Subjects
			for _, subject := range bindingSubjects {
				// Use namespace and name to look up pod
				subjectPod, err := kubeclientset.CoreV1().Pods(subject.Namespace).Get(subject.Name, v1.GetOptions{})
				if err != nil {
					utilruntime.HandleError(fmt.Errorf("No Pod found for IdentityBinding subject %s/%s: %s", subject.Namespace, subject.Name, err))
					return
				}

				fromServices = append(fromServices, subjectPod.Spec.ServiceAccountName)
			}

			// Validate that there is a TCP route (if the traffic target references Kind TCPRoute)
			// If it does not exist, show error and stop processing.
			containsTCPRoute := false
			fmt.Printf("The target (%s) is: %#v\n---\n", binding.TargetRef.Name, target)
			for _, spec := range target.Specs {
				if spec.Kind == "TCPRoute" {
					_, err := controller.tcpRouteLister.TCPRoutes(binding.Namespace).Get(spec.Name)
					if err != nil {
						// What should happen if one of the routes referenced does not exist? should we fail?
						// What if someone deletes a TCPRoute? <- it should probably not be possible to delete a route that is referenced by a TrafficTarget.
						utilruntime.HandleError(fmt.Errorf("No route for %s/%s", binding.Namespace, spec.Name))
						return
					}

					containsTCPRoute = true
					break
				}
			}

			if !containsTCPRoute {
				return
			}

			fmt.Printf("creating intention from: %#v, to: %#v", fromServices, toService)
			for _, fromService := range fromServices {
				ok, err := controller.consulClient.CreateIntention(fromService, toService)
				if err != nil {
					utilruntime.HandleError(fmt.Errorf("Unable to create intention: %s", err))
					return
				}

				if ok {
					klog.Info("Intention successfully created")
				} else {
					klog.Info("Intention not created")
				}
			}
		},
		UpdateFunc: func(old, new interface{}) {
			// TODO think what to do here
		},
		DeleteFunc: func(obj interface{}) {
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
	if ok := cache.WaitForCacheSync(stopCh, c.targetSynced, c.bindingSynced); !ok {
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
		// Resource to be synced.
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
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Foo resource with this namespace/name
	target, err := c.targetLister.TrafficTargets(namespace).Get(name)
	if err != nil {
		// The resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("%s in work queue no longer exists", key))
			return nil
		}

		return err
	}

	fmt.Printf("synching %#v", target)

	c.recorder.Event(target, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
