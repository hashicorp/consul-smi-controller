module github.com/hashicorp/consul-smi-controller

go 1.12

require (
	cloud.google.com/go v0.41.0 // indirect
	contrib.go.opencensus.io/exporter/ocagent v0.5.0 // indirect
	github.com/Azure/go-autorest v12.3.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/adal v0.2.0 // indirect
	github.com/Azure/go-autorest/autorest/mocks v0.2.0 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2 // indirect
	github.com/armon/go-metrics v0.0.0-20190430140413-ec5e00d3c878 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/deislabs/smi-sdk-go v0.0.0-20190621175932-114e91dce170
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20190711103511-473e67f1d7d2 // indirect
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/go-kit/kit v0.9.0 // indirect
	github.com/go-openapi/spec v0.19.2 // indirect
	github.com/go-openapi/swag v0.19.4 // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/gophercloud/gophercloud v0.2.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.9.4 // indirect
	github.com/hashicorp/consul/api v1.1.0
	github.com/hashicorp/go-immutable-radix v1.1.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-retryablehttp v0.5.4 // indirect
	github.com/hashicorp/go-rootcerts v1.0.1 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/go-version v1.2.0 // indirect
	github.com/hashicorp/mdns v1.0.1 // indirect
	github.com/hashicorp/memberlist v0.1.4 // indirect
	github.com/hashicorp/serf v0.8.3 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/kisielk/errcheck v1.2.0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pty v1.1.8 // indirect
	github.com/kubernetes-client/go v0.0.0-20190625181339-cd8e39e789c7 // indirect
	github.com/mailru/easyjson v0.0.0-20190626092158-b2ccc519800e // indirect
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/miekg/dns v1.1.15 // indirect
	github.com/mitchellh/gox v1.0.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20190414153302-2ae31c8b6b30 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/onsi/ginkgo v1.8.0 // indirect
	github.com/onsi/gomega v1.5.0 // indirect
	github.com/posener/complete v1.2.1 // indirect
	github.com/prometheus/common v0.6.0 // indirect
	github.com/prometheus/procfs v0.0.3 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20190512091148-babf20351dd7 // indirect
	github.com/rogpeppe/fastuuid v1.2.0 // indirect
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/stretchr/testify v1.3.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/exp v0.0.0-20190627132806-fd42eb6b336f // indirect
	golang.org/x/image v0.0.0-20190703141733-d6a02ce849c9 // indirect
	golang.org/x/mobile v0.0.0-20190711165009-e47acb2ca7f9 // indirect
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7 // indirect
	golang.org/x/sys v0.0.0-20190712062909-fae7ac547cb7 // indirect
	golang.org/x/tools v0.0.0-20190716150014-919acb9f1ffd // indirect
	gonum.org/v1/gonum v0.0.0-20190710053202-4340aa3071a0 // indirect
	google.golang.org/genproto v0.0.0-20190708153700-3bdd9d9f5532 // indirect
	google.golang.org/grpc v1.22.0 // indirect
	k8s.io/api v0.0.0-20190712022805-31fe033ae6f9
	k8s.io/apimachinery v0.0.0-20190715170309-6171873045ff
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a // indirect
	k8s.io/klog v0.3.3
	k8s.io/kube-openapi v0.0.0-20190709113604-33be087ad058 // indirect
	k8s.io/sample-controller v0.0.0-20190713023659-499fb3ff94b9
	k8s.io/utils v0.0.0-20190712204705-3dccf664f023 // indirect
	sigs.k8s.io/structured-merge-diff v0.0.0-20190711200306-eaa53bff5a75 // indirect
)

replace (
	golang.org/x/sync => golang.org/x/sync v0.0.0-20181108010431-42b317875d0f
	golang.org/x/sys => golang.org/x/sys v0.0.0-20190209173611-3b5209105503
	golang.org/x/tools => golang.org/x/tools v0.0.0-20190313210603-aa82965741a9
	k8s.io/api => k8s.io/api v0.0.0-20190425012535-181e1f9c52c1
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190425132440-17f84483f500
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190425172711-65184652c889
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190419212335-ff26e7842f9d
)

replace k8s.io/component-base => k8s.io/component-base v0.0.0-20190424053038-9fe063da3132
