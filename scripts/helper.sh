#!/bin/bash

function cleanup() {
  echo ""
  echo "Exiting ..."

  # Stop the process and cleanup the pid
  pkill -F .pid_$1
  rm .pid_$1

  exit 0
}

function open_service() {
  kubectl port-forward --namespace=$5 --address 0.0.0.0 svc/$3 $1:$2 & echo $! > .pid_$3

  echo " "
	echo "Opening $3, To quit, press Ctrl-C"
	sleep 5
	trap "cleanup $3" SIGINT
 
  # block until we exit cleanly
  for number in $(seq 1000000); do
    sleep 2
  done
}

case "$1" in
  consul)
    CONSUL_ACL_TOKEN=$(kubectl get secret consul-consul-bootstrap-acl-token -o json | jq -r '.data.token' | base64 -d)
    echo "Consul ACL Token: ${CONSUL_ACL_TOKEN}"
    echo "Consul HTTP Address: http://localhost:18500"
    echo ""
    echo "To interact with Consul using the CLI or curl set the following environment variables"
    echo "export CONSUL_HTTP_ADDR=http://localhost:18500"
    echo "export CONSUL_HTTP_TOKEN=${CONSUL_ACL_TOKEN}"
    echo ""
    open_service 18500 8500 consul-consul-server http default
    ;;
  consul-ui)
    xdg-open http://$(kubectl get svc consul-consul-ui -o jsonpath="{.status.loadBalancer.ingress[0].ip}")
    ;;
  *)
    echo "Usage:"
    echo "consul     - Start a proxy to the Consul server API"
    exit 1
esac
