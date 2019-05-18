package access

import (
	"os"
	"testing"
	"time"

	accessv1alpha1 "github.com/deislabs/smi-sdk-go/pkg/apis/access/v1alpha1"
	accessClientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/clientset/versioned/fake"
	accessInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/informers/externalversions"
	"github.com/hashicorp/consul-smi/clients"
	fclient "k8s.io/client-go/kubernetes/fake"
	fcache "k8s.io/client-go/tools/cache/testing"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type testObjects struct {
	fakeAccessClientSet *accessClientset.Clientset
	fakeConsul          *clients.ConsulMock
	fakeKubeClient      *fclient.Clientset
	fakeInformer        accessInformers.SharedInformerFactory
	fakeCache           *fcache.FakeControllerSource
}

func setupTrafficTarget(t *testing.T) (*Controller, *testObjects) {
	klog.SetOutput(os.Stdout)
	//fakeLister := accessListers.NewTrafficTargetLister(nil)
	to := &testObjects{}
	to.fakeAccessClientSet = accessClientset.NewSimpleClientset(&accessv1alpha1.TrafficTarget{})
	to.fakeConsul = &clients.ConsulMock{}
	to.fakeKubeClient = fclient.NewSimpleClientset()
	to.fakeCache = fcache.NewFakeControllerSource()
	to.fakeInformer = accessInformers.NewSharedInformerFactory(to.fakeAccessClientSet, noResyncPeriodFunc())

	c := NewController(
		to.fakeKubeClient,
		to.fakeAccessClientSet,
		to.fakeInformer.Access().V1alpha1().TrafficTargets(),
		to.fakeConsul,
	)
	c.targetSynced = alwaysReady
	c.recorder = &record.FakeRecorder{}

	return c, to
}

func TestControllerRuns(t *testing.T) {
	c, _ := setupTrafficTarget(t)
	stopChan := make(chan struct{})
	go c.Run(1, stopChan)
	time.Sleep(1000 * time.Millisecond)
}
