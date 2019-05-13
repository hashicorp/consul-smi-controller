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
