KUBECONFIG := ${HOME}/.kube/config

build:
	go build -o bin/trafficspec trafficspec/**

run-consul:
	scripts/helper.sh consul

run-controller:
	bin/trafficspec --consul-http-addr=http://localhost:18500 --consul-http-token=${TOKEN} --kubeconfig=${KUBECONFIG}