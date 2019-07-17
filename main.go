package main

import (
	"flag"
	"log"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"k8s.io/sample-controller/pkg/signals"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	accessClientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/clientset/versioned"
	accessInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/access/informers/externalversions"
	specsClientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/specs/clientset/versioned"
	specsInformers "github.com/deislabs/smi-sdk-go/pkg/gen/client/specs/informers/externalversions"
	"github.com/hashicorp/consul-smi-controller/access"
	"github.com/hashicorp/consul-smi-controller/clients"
)

var (
	masterURL      string
	kubeconfig     string
	consulACLToken string
	consulHTTPAddr string
)

func main() {
	flag.Parse()

	klog.InitFlags(nil)
	flag.Set("v", "3")

	// create the consul client
	consulClient, err := clients.NewConsul(consulHTTPAddr, consulACLToken)
	if err != nil {
		log.Fatal("Unable to create Consul client", err)
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	accessClient, err := accessClientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building access clientset: %s", err.Error())
	}

	specsClient, err := specsClientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building specs clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	accessInformerFactory := accessInformers.NewSharedInformerFactory(accessClient, time.Second*30)
	specsInformerFactory := specsInformers.NewSharedInformerFactory(specsClient, time.Second*30)
	deletedIndexer := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{})

	controller := access.NewController(
		kubeClient,
		accessClient,
		accessInformerFactory.Access().V1alpha1().TrafficTargets(),
		deletedIndexer,
		consulClient,
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	accessInformerFactory.Start(stopCh)
	specsInformerFactory.Start(stopCh)

	if err = controller.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&consulACLToken, "consul-http-token", "", "ACL Token for communicating with Consul")
	flag.StringVar(&consulHTTPAddr, "consul-http-addr", "http://localhost:8500", "Address of the consul server, default http://localhost:8500")
}
