VERSION := v0.0.0-alpha.2
DOCKER_TAG := hashicorp/consul-smi-controller


build:
	CGO_ENABLED=0 go build -o bin/smi-controller ./

run-consul:
	scripts/helper.sh consul

run-controller: build
	bin/smi-controller --consul-http-addr=${CONSUL_HTTP_ADDR} --consul-http-token=${CONSUL_HTTP_TOKEN} --kubeconfig=${KUBECONFIG}

build-docker: build
	docker build -f ./Dockerfile -t ${DOCKER_TAG}:${VERSION} .

push-docker:
	docker push ${DOCKER_TAG}:${VERSION}
