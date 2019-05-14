# Consul SMI Demo

## Example Walkthrough
https://www.loom.com/share/6907c66621294bbea75cc021c70a89c5
password: smisecret

## Steps

1. Deploy helm chart to Kubernetes Server

```bash
$ kubectl apply -f ./consul --validate=false
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
