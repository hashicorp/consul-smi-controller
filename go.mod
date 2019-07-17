module github.com/hashicorp/consul-smi-controller

go 1.12

require (
	cloud.google.com/go v0.41.0 // indirect
	github.com/armon/go-metrics v0.0.0-20190430140413-ec5e00d3c878 // indirect
	github.com/deislabs/smi-sdk-go v0.0.0-20190621175932-114e91dce170
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/protobuf v1.3.2 // indirect
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/hashicorp/consul/api v1.1.0
	github.com/hashicorp/go-immutable-radix v1.1.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-rootcerts v1.0.1 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/memberlist v0.1.4 // indirect
	github.com/hashicorp/serf v0.8.3 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/miekg/dns v1.1.15 // indirect
	github.com/onsi/ginkgo v1.8.0 // indirect
	github.com/onsi/gomega v1.5.0 // indirect
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.3.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7 // indirect
	golang.org/x/sys v0.0.0-20190712062909-fae7ac547cb7 // indirect
	k8s.io/api v0.0.0-20190712022805-31fe033ae6f9
	k8s.io/apimachinery v0.0.0-20190715170309-6171873045ff
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v0.3.3
	k8s.io/kube-openapi v0.0.0-20190709113604-33be087ad058 // indirect
	k8s.io/sample-controller v0.0.0-20190713023659-499fb3ff94b9
	k8s.io/utils v0.0.0-20190712204705-3dccf664f023 // indirect
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
