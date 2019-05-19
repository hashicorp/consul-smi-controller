KUBECONFIG := ${HOME}/.kube/config
VERSION := v0.0.0-alpha.1
DOCKER_TAG := quay.io/nicholasjackson/smi-traffic-controller


build:
	CGO_ENABLED=0 go build -o bin/smi-controller ./

run-consul:
	scripts/helper.sh consul

run-controller: build
	bin/smi-controller --consul-http-addr=http://localhost:18500 --consul-http-token=${TOKEN} --kubeconfig=${KUBECONFIG}

build-docker: build
	docker build -f ./Dockerfile -t ${DOCKER_TAG}:${VERSION} .

