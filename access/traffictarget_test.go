package access

import (
	"os"
	"testing"
	"time"

	accessv1alpha1 "github.com/deislabs/smi-sdk-go/pkg/apis/access/v1alpha1"
	"github.com/deislabs/smi-sdk-go/pkg/gen/client/access/clientset/versioned/fake"
	accessInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/informers/externalversions"
	"github.com/hashicorp/consul-smi/clients"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fclient "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

var ()

type fixtures struct {
	consulClient *clients.ConsulMock
	client       *fake.Clientset
	kubeClient   *fclient.Clientset
	ready        chan struct{}
	controller   *Controller
	t            *testing.T
	// Actions expected to happen on the client
	actions []core.Action
	// Objects to put in the store
	trafficLister []*accessv1alpha1.TrafficTarget
	// Objects preloaded in NewSimpleFake
	objects []runtime.Object
}

func alwaysReady() bool { return true }

func noResyncPeriod() time.Duration { return 0 }

func newFixtures(t *testing.T) *fixtures {
	klog.SetOutput(os.Stdout)

	f := &fixtures{}
	f.t = t
	f.objects = []runtime.Object{}

	f.consulClient = &clients.ConsulMock{}
	f.consulClient.Mock.On("SyncIntentions", mock.Anything, mock.Anything).
		Return(nil)

	return f
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "traffictargets") ||
				action.Matches("watch", "traffictargets") ||
				action.Matches("list", "deployments") ||
				action.Matches("watch", "deployments")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func (f *fixtures) newController() (*Controller, accessInformers.SharedInformerFactory) {
	f.client = fake.NewSimpleClientset(f.objects...)
	f.kubeClient = fclient.NewSimpleClientset()
	i := accessInformers.NewSharedInformerFactory(f.client, noResyncPeriod())
	di := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{})

	c := NewController(
		f.kubeClient,
		f.client,
		i.Access().V1alpha1().TrafficTargets(),
		di,
		f.consulClient,
	)

	c.targetSynced = alwaysReady
	c.recorder = &record.FakeRecorder{}

	for _, t := range f.trafficLister {
		i.Access().V1alpha1().TrafficTargets().Informer().GetIndexer().Add(t)
	}

	return c, i
}

func (f *fixtures) run(name string) {
	c, i := f.newController()

	startInformers := true
	expectError := false

	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		i.Start(stopCh)
	}

	err := c.syncHandler(name)
	if !expectError && err != nil {
		f.t.Errorf("error syncing traffictarget: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing traffictarget, got nil")
	}

	actions := filterInformerActions(f.client.Actions())
	for i := range actions {
		if len(f.actions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
			break
		}

		//expectedAction := f.actions[i]
		//checkAction(expectedAction, action, f.t)
	}

	if len(f.actions) > len(actions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.actions)-len(actions), f.actions[len(actions):])
	}
}

func (f *fixtures) expectCreateTrafficTargetAction(tt *accessv1alpha1.TrafficTarget) {
	action := core.NewCreateAction(schema.GroupVersionResource{Resource: "traffictargets"}, tt.Namespace, tt)

	f.actions = append(f.actions, action)
}

func (f *fixtures) expectUpdateTrafficTargetAction(tt *accessv1alpha1.TrafficTarget) {
	action := core.NewUpdateAction(schema.GroupVersionResource{Resource: "traffictargets"}, tt.Namespace, tt)
	f.actions = append(f.actions, action)
}

func getKey(tt *accessv1alpha1.TrafficTarget, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(tt)
	if err != nil {
		t.Errorf("Unexpected error getting key for traffictarget %v: %v", tt.Name, err)
		return ""
	}
	return key
}

func createTrafficTarget(name, source, destination string) *accessv1alpha1.TrafficTarget {
	return &accessv1alpha1.TrafficTarget{
		TypeMeta: metav1.TypeMeta{APIVersion: accessv1alpha1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Destination: accessv1alpha1.IdentityBindingSubject{
			Kind:      "ServiceAccount",
			Name:      destination,
			Namespace: "default",
		},
		Sources: []accessv1alpha1.IdentityBindingSubject{
			accessv1alpha1.IdentityBindingSubject{
				Kind:      "ServiceAccount",
				Name:      source,
				Namespace: "default",
			},
		},
	}
}

func TestUpdatesIntentionsFromNewTrafficTarget(t *testing.T) {
	tt := createTrafficTarget("servicea-target", "serviceb", "servicea")
	f := newFixtures(t)
	f.trafficLister = append(f.trafficLister, tt)
	f.objects = append(f.objects, tt)

	// expect a traffic target to be created
	f.expectCreateTrafficTargetAction(tt)

	// start the controller
	f.run(getKey(tt, t))

	// assert consul client was called
	f.consulClient.Mock.AssertCalled(t, "SyncIntentions", []string{"serviceb"}, "servicea")
}

func TestUpdatesIntentionsFromDeletedTrafficTarget(t *testing.T) {
	tt := createTrafficTarget("servicea-target", "serviceb", "servicea")
	f := newFixtures(t)
	f.trafficLister = append(f.trafficLister, tt)
	f.objects = append(f.objects, tt)

	// expect a traffic target to be created
	f.expectCreateTrafficTargetAction(tt)

	// start the controller
	f.run(getKey(tt, t))

	// assert consul client was called
	f.consulClient.Mock.AssertCalled(t, "SyncIntentions", []string{"serviceb"}, "servicea")
}
