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
	"log"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"k8s.io/sample-controller/pkg/signals"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/deislabs/smi-sdk-go/pkg/gen/client/trafficspec/clientset/versioned"
	informers "github.com/deislabs/smi-sdk-go/pkg/gen/client/trafficspec/informers/externalversions"
	"github.com/hashicorp/consul-smi/clients"
	// "k8s.io/sample-controller/pkg/signals"
)

var (
	masterURL      string
	kubeconfig     string
	consulACLToken string
	consulHTTPAddr string
)

func main() {
	flag.Parse()

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

	smiClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building smi clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	smiInformerFactory := informers.NewSharedInformerFactory(smiClient, time.Second*30)

	controller := NewController(
		kubeClient,
		smiClient,
		smiInformerFactory.Smispec().V1alpha1().TrafficTargets(),
		smiInformerFactory.Smispec().V1alpha1().IdentityBindings(),
		smiInformerFactory.Smispec().V1alpha1().TCPRoutes(),
		consulClient,
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	smiInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&consulACLToken, "consul-http-token", "", "ACL Token for communicating with Consul")
	flag.StringVar(&consulHTTPAddr, "consul-http-addr", "http://localhost:8500", "Address of the consul server, default http://localhost:8500")
}
