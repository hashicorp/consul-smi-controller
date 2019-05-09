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
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	trafficspecv1alpha1 "github.com/deislabs/smi-sdk-go/pkg/apis/trafficspec/v1alpha1"
	clientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/trafficspec/clientset/versioned"
	trafficspecscheme "github.com/deislabs/smi-sdk-go/pkg/gen/client/trafficspec/clientset/versioned/scheme"
	informers "github.com/deislabs/smi-sdk-go/pkg/gen/client/trafficspec/informers/externalversions/trafficspec/v1alpha1"
	listers "github.com/deislabs/smi-sdk-go/pkg/gen/client/trafficspec/listers/trafficspec/v1alpha1"
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
	kubeclientset  kubernetes.Interface
	smiclientset   clientset.Interface
	bindingLister  listers.IdentityBindingLister
	bindingSynced  cache.InformerSynced
	targetLister   listers.TrafficTargetLister
	targetSynced   cache.InformerSynced
	tcpRouteLister listers.TCPRouteLister
	tcpRouteSynced cache.InformerSynced
	workqueue      workqueue.RateLimitingInterface
	recorder       record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	smiclientset clientset.Interface,
	targetInformer informers.TrafficTargetInformer,
	bindingInformer informers.IdentityBindingInformer,
	tcpRouteInformer informers.TCPRouteInformer,
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
		kubeclientset:  kubeclientset,
		smiclientset:   smiclientset,
		bindingLister:  bindingInformer.Lister(),
		bindingSynced:  bindingInformer.Informer().HasSynced,
		targetLister:   targetInformer.Lister(),
		targetSynced:   targetInformer.Informer().HasSynced,
		tcpRouteLister: tcpRouteInformer.Lister(),
		tcpRouteSynced: tcpRouteInformer.Informer().HasSynced,
		workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Foos"),
		recorder:       recorder,
	}

	klog.Info("Setting up event handlers")
	targetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Info("TrafficTarget created")
			controller.enqueueObject(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			klog.Info("TrafficTarget updated")
			controller.enqueueObject(new)
		},
		DeleteFunc: func(obj interface{}) {
			// TODO think what to do (probably clean up bindings that were referenced)
			klog.Info("TrafficTarget deleted")
			controller.enqueueObject(obj)
		},
	})

	bindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Info("IdentifyBinding created")
			binding := obj.(*trafficspecv1alpha1.IdentityBinding)

			// TO
			target, err := controller.targetLister.TrafficTargets(binding.Namespace).Get(binding.TargetRef.Name)
			if err != nil {
				klog.Errorf("No TrafficTarget found for IdentifyBinding %s/%s: %s", binding.Namespace, binding.TargetRef.Name, err.Error())
				controller.enqueueObject(obj)
			}

			// Get the pods that are referenced by the TrafficTarget.
			targetPod, err := kubeclientset.CoreV1().Pods(binding.Namespace).Get(binding.TargetRef.Name, v1.GetOptions{})
			if err != nil {
				klog.Errorf("No Pod found for TrafficTarget %s/%s: %s", binding.Namespace, binding.TargetRef.Name, err.Error())
				controller.enqueueObject(obj)
			}

			// Look for a service account for that pod.
			targetServiceAccount, err := kubeclientset.CoreV1().ServiceAccounts(binding.Namespace).Get(targetPod.Spec.ServiceAccountName, v1.GetOptions{})
			if err != nil {
				klog.Errorf("No ServiceAccount found with name %s for Pod %s/%s: %s", targetPod.Spec.ServiceAccountName, binding.Namespace, binding.TargetRef.Name, err.Error())
				controller.enqueueObject(obj)
			}

			fmt.Printf("Got service account: %#v\n\n", targetServiceAccount.Secrets)

			// Look up the service name through Consul Auth Method API.

			// FROM
			// Get the Subjects from the IdentifyBinding.
			bindingSubjects := binding.Subjects
			for _, subject := range bindingSubjects {
				// Use namespace and name to look up pod
				subjectPod, err := kubeclientset.CoreV1().Pods(subject.Namespace).Get(subject.Name, v1.GetOptions{})
				if err != nil {
					klog.Errorf("No Pod found for IdentityBinding subject %s/%s: %s", subject.Namespace, subject.Name, err.Error())
					controller.enqueueObject(obj)
				}

				// then grab a service account from that pod.
				subjectServiceAccount, err := kubeclientset.CoreV1().ServiceAccounts(subject.Namespace).Get(subjectPod.Spec.ServiceAccountName, v1.GetOptions{})
				if err != nil {
					klog.Errorf("No ServiceAccount found with name %s for Pod %s/%s: %s", subjectPod.Spec.ServiceAccountName, subject.Namespace, subject.Name, err.Error())
					controller.enqueueObject(obj)
				}

				fmt.Printf("Got service account: %#v\n\n", subjectServiceAccount.Secrets)

				var secretName string
				for _, secret := range subjectServiceAccount.Secrets {
					if strings.HasPrefix(secret.Name, fmt.Sprintf("%s-token", subjectServiceAccount.Name)) {
						secretName = secret.Name
						break
					}
				}

				secret, err := kubeclientset.CoreV1().Secrets(subject.Namespace).Get(secretName, v1.GetOptions{})
				if err != nil {
					klog.Errorf("No Secret found with name %s: %s", secretName, err.Error())
					fmt.Printf("No secret for %s", secretName)
					controller.enqueueObject(obj)
				}

				klog.Infof("token: %#v", secret.Data["token"])

			}

			// Get the ServiceAccount from the Subjects and look up in Consul (Auth Method API) to get the service name.

			// Validate that there is a TCP route (if the traffic target references Kind TCPRoute)
			// If it does not exist, show error and stop processing.
			containsTCPRoute := false
			fmt.Printf("The target (%s) is: %#v\n---\n", binding.TargetRef.Name, target)
			for _, rule := range target.Rules {
				if rule.Kind == "TCPRoute" {
					rule, err := controller.tcpRouteLister.TCPRoutes(binding.Namespace).Get(rule.Name)
					if err != nil {
						fmt.Printf("No route for %s/%s", binding.Namespace, rule.Name)
						controller.enqueueObject(obj)
						// What should happen if one of the routes referenced does not exist? should we fail?
						// What if someone deletes a TCPRoute? <- it should probably not be possible to delete a route that is referenced by a TrafficTarget.
						break
					}

					containsTCPRoute = true
					break
				}
			}

			fmt.Printf("did we find a route? %t\n\n", containsTCPRoute)

			// Intention
			// Create an intention that allows traffic from "FROM" to "TO"
			// client, err := api.NewClient(api.DefaultConfig())
			// if err != nil {
			// 	panic(err)
			// }

			// connect := client.Connect()

			// i := &api.Intention{
			// 	SourceName:      "web",
			// 	DestinationName: target.Name,
			// 	Action:          api.IntentionActionAllow,
			// }

		},
		UpdateFunc: func(old, new interface{}) {
			// TODO think what to do here
			controller.enqueueObject(new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueueObject(obj)
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

	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Foo resource will enqueue that Foo resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	// deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	// 	AddFunc: controller.handleObject,
	// 	UpdateFunc: func(old, new interface{}) {
	// 		newDepl := new.(*appsv1.Deployment)
	// 		oldDepl := old.(*appsv1.Deployment)
	// 		if newDepl.ResourceVersion == oldDepl.ResourceVersion {
	// 			// Periodic resync will send update events for all known Deployments.
	// 			// Two different versions of the same Deployment will always have different RVs.
	// 			return
	// 		}
	// 		controller.handleObject(new)
	// 	},
	// 	DeleteFunc: controller.handleObject,
	// })

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
	klog.Info("Starting Foo controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.targetSynced, c.bindingSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Foo resources
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
		// Foo resource to be synced.
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
// converge the two. It then updates the Status block of the Foo resource
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
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	fmt.Printf("synching %#v", target)

	// deploymentName := target.Spec.DeploymentName
	// if deploymentName == "" {
	// 	// We choose to absorb the error here as the worker would requeue the
	// 	// resource otherwise. Instead, the next time the resource is updated
	// 	// the resource will be queued again.
	// 	utilruntime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
	// 	return nil
	// }

	// Get the deployment with the name specified in Foo.spec
	// deployment, err := c.deploymentsLister.Deployments(foo.Namespace).Get(deploymentName)
	// // If the resource doesn't exist, we'll create it
	// if errors.IsNotFound(err) {
	// 	deployment, err = c.kubeclientset.AppsV1().Deployments(foo.Namespace).Create(newDeployment(foo))
	// }

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	// if err != nil {
	// 	return err
	// }

	// If the Deployment is not controlled by this Foo resource, we should log
	// a warning to the event recorder and ret
	// if !metav1.IsControlledBy(deployment, foo) {
	// 	msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
	// 	c.recorder.Event(foo, corev1.EventTypeWarning, ErrResourceExists, msg)
	// 	return fmt.Errorf(msg)
	// }

	// If this number of the replicas on the Foo resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	// if foo.Spec.Replicas != nil && *foo.Spec.Replicas != *deployment.Spec.Replicas {
	// 	klog.V(4).Infof("Foo %s replicas: %d, deployment replicas: %d", name, *foo.Spec.Replicas, *deployment.Spec.Replicas)
	// 	deployment, err = c.kubeclientset.AppsV1().Deployments(foo.Namespace).Update(newDeployment(foo))
	// }

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. THis could have been caused by a
	// temporary network failure, or any other transient reason.
	// if err != nil {
	// 	return err
	// }

	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world
	// err = c.updateFooStatus(foo, deployment)
	// if err != nil {
	// 	return err
	// }

	c.recorder.Event(target, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// func (c *Controller) updateFooStatus(target *specv1alpha1.TrafficTarget, deployment *appsv1.Deployment) error {
// 	// NEVER modify objects from the store. It's a read-only, local cache.
// 	// You can use DeepCopy() to make a deep copy of original object and modify this copy
// 	// Or create a copy manually for better performance
// 	targetCopy := target.DeepCopy()
// 	// targetCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
// 	// If the CustomResourceSubresources feature gate is not enabled,
// 	// we must use Update instead of UpdateStatus to update the Status block of the Foo resource.
// 	// UpdateStatus will not allow changes to the Spec of the resource,
// 	// which is ideal for ensuring nothing other than resource status has been updated.
// 	_, err := c.smiclientset.SmispecV1alpha1().TrafficTargets(target.Namespace).Update(targetCopy)
// 	return err
// }

// enqueueFoo takes a Foo resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Foo.
func (c *Controller) enqueueObject(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Foo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
// func (c *Controller) handleObject(obj interface{}) {
// 	var object v1.Object
// 	var ok bool
// 	if object, ok = obj.(v1.Object); !ok {
// 		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
// 		if !ok {
// 			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
// 			return
// 		}
// 		object, ok = tombstone.Obj.(v1.Object)
// 		if !ok {
// 			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
// 			return
// 		}
// 		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
// 	}
// 	klog.V(4).Infof("Processing object: %s", object.GetName())
// 	if ownerRef := v1.GetControllerOf(object); ownerRef != nil {
// 		// If this object is not owned by a Foo, we should not do anything more
// 		// with it.
// 		if ownerRef.Kind != "TrafficTarget" {
// 			return
// 		}

// 		foo, err := c.targetLister.TrafficTargets(object.GetNamespace()).Get(ownerRef.Name)
// 		if err != nil {
// 			klog.V(4).Infof("ignoring orphaned object '%s' of foo '%s'", object.GetSelfLink(), ownerRef.Name)
// 			return
// 		}

// 		// c.enqueueFoo(foo)
// 		return
// 	}
// }

// newDeployment creates a new Deployment for a Foo resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Foo resource that 'owns' it.
// func newDeployment(foo *samplev1alpha1.Foo) *appsv1.Deployment {
// 	labels := map[string]string{
// 		"app":        "nginx",
// 		"controller": foo.Name,
// 	}
// 	return &appsv1.Deployment{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      foo.Spec.DeploymentName,
// 			Namespace: foo.Namespace,
// 			OwnerReferences: []metav1.OwnerReference{
// 				*metav1.NewControllerRef(foo, samplev1alpha1.SchemeGroupVersion.WithKind("Foo")),
// 			},
// 		},
// 		Spec: appsv1.DeploymentSpec{
// 			Replicas: foo.Spec.Replicas,
// 			Selector: &metav1.LabelSelector{
// 				MatchLabels: labels,
// 			},
// 			Template: corev1.PodTemplateSpec{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Labels: labels,
// 				},
// 				Spec: corev1.PodSpec{
// 					Containers: []corev1.Container{
// 						{
// 							Name:  "nginx",
// 							Image: "nginx:latest",
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}
// }
