# Consul SMI Demo

## Steps

1. Deploy helm chart to Kubernetes Server

```bash
$ kubectl apply -f ./consul/consul
```

2. Deploy the SMI CRD and Controller

```bash
$ kubectl apply -f ./crd.yaml
```

3. Deploy the demo application

```bash
$ kubectl apply -f ./example.yaml
```

4. Open the Consul UI

```bash
# Linux
$ xdg-open http://$(kubectl get svc consul-consul-ui -o jsonpath="{.status.loadBalancer.ingress[0].ip}")
# or Mac
$ open http://$(kubectl get svc consul-consul-ui -o jsonpath="{.status.loadBalancer.ingress[0].ip}")
```
