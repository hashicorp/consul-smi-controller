KUBECONFIG := ${HOME}/.kube/config
VERSION := v0.1.0
DOCKER_TAG := nicholasjackson/testing123


build:
	CGO_ENABLED=0 go build -o bin/trafficspec ./trafficspec

run-consul:
	scripts/helper.sh consul

run-controller:
	bin/trafficspec --consul-http-addr=http://localhost:18500 --consul-http-token=${TOKEN} --kubeconfig=${KUBECONFIG}

build-docker: build
	docker build -f ./trafficspec/Dockerfile -t ${DOCKER_TAG}:${VERSION} .

