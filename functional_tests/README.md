# Functional Testing

The docker-compose file in this folder creates a two node `k3s` cluster which can be used for testing. To run the cluster and setup Consul, the following commands can be used.

## Starting the K3s cluster
```
./run.sh up
```

Once complete a Kubernetes config file `kubeconfig.yaml` will be output to this folder, you can use this to interact with the cluster.
```
export KUBECONFIG=$(pwd)/kubeconfig.yaml

➜ kubectl get pods
No resources found in default namespace.
```

## Install Helm, Consul and other pre-requisites
The following process can take several minutes as K3s will need to pull the required images to its local cache.
```
./run.sh install
```

The install script waits for Consul to become healthy, this can take 90s.

## Interacting with the Consul UI
Once everything has been installed, you can access the Consul cluster via `localhost:8500`.

Note: Consul has been configured with sane defaults and ACL tokens enabled, the root ACL token is saved into a file `consul_acl.token`. This token can be used with curl by passing it as a header.

```
➜ curl -s --header "X-Consul-Token: $(cat ./consul_acl.token)" localhost:8500/v1/status/leader

"10.42.0.12:8300"% 
```

The Consul UI can also be accessed in your browser at `http://localhost:8500`
