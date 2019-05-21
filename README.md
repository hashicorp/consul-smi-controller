# Consul SMI CRD
Experimental repository containing Kubernetes CRDs for the Service Mesh Interface spec (SMI)

## Service Mesh Interface
Microsoft’s Service Mesh Interface is a series of Kubernetes controllers for implementing various service mesh capabilities. At launch, SMI will support four primary functions:
- Traffic Specs - define traffic routing on a per-protocol basis. These resources work in unison with access control and other types of policy to manage traffic at a protocol level.
- Traffic Access Control - configure access to specific pods and routes based on the identity of a client,  to only allow specific users and services.
- Traffic Split - direct weighted traffic between services or versions of a service, enabling Canary Testing or Dark Launches.
- Traffic Metrics - expose common traffic metrics for use by tools such as dashboards and autoscalers.

At launch, HashiCorp Consul will support the Traffic Access Control specification, with possible integrations for the others in the future.     

## TrafficTarget CRD
One of the custom resources defined by SMI is the TrafficTarget resource, developed by us in collaboration with the Microsoft team to assist with the challenge of securing service to service traffic. This resource enables the user to define Consul Connect intentions in a Kubernetes custom resource (CRD) and manage them through `kubectl`, `Helm` or `Terraform`, rather than having to configure them directly through Consul. This enables developers to ensure that newly deployed applications have a secure connection to resources through a single workflow.

## How to install
To use the Consul SMI Controller, you will need to have a running Consul cluster with Connect enabled.

### Installing Consul
Paying attention to the values shown below, you can use the official Consul Helm chart by cloning `https://github.com/hashicorp/consul-helm.git`.
Then installing it by running `helm install -f values.yaml --name <name> ./consul-helm`.

```yaml
# Enable bootstrapping of ACLs, available in Consul 1.5.0+
global:
  image: "consul:1.5.0"
  imageK8S: "hashicorp/consul-k8s:0.8.1"

  bootstrapACLs: true

# Enable connect in order to use Service Mesh functionality
server:
  enabled: true
  replicas: 3
  bootstrapExpect: 3 # Should <= replicas count

  connect: true

client:
  enabled: true
  grpc: true

ui:
  enabled: true

# Synchronize services between Kubernetes and Consul
syncCatalog:
  enabled: true
  default: true
  toConsul: true
  toK8S: true
  syncClusterIPServices: true

# ConnectInject will enable the automatic Connect sidecar injector
connectInject:
  enabled: true
  default: false # true will inject by default, otherwise requires annotation.

  # Requires Consul v1.5+ and consul-k8s v0.8.0+.
  aclBindingRuleSelector: "serviceaccount.name!=default"

  # Enable central configuration for easier management for services and proxies.
  centralConfig:
    enabled: true
```

### Deploying the Consul SMI Controller
In order for the Consul SMI Controller to work, it needs to be able to read and write Intentions in Consul. 
To do this, you need to issue an ACL token with the policy *global-management* `consul acl token create -description "read/write access for the consul-smi-controller" -policy-name global-management`, and copy the token that it outputs.

With this token, you create a secret named `consul-smi-controller-acl-token` in Kubernetes that the Consul SMI Controller can read and use.
```yaml
$ kubectl create secret generic consul-smi-acl-token --from-literal=token=[your token]
```

And then deploy the Consul SMI Controller using `kubectl apply -f consul-smi-controller.yaml`.

## How to use
### Deploying the applications
Now that you have a running Consul cluster, you can deploy the applications with an annotation of `'consul.hashicorp.com/connect-inject': 'true'` and `"consul.hashicorp.com/connect-service": "<service name>"`. A sidecar proxy will automatically be injected and the service automatically registered in the service catalog of Consul.

Authentication is done using Kubernetes service accounts to ensure the service is who it says it is.
Lets create a service account for both the frontend and backend service.

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: counting
automountServiceAccountToken: false
```

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: dashboard
automountServiceAccountToken: false
```

Then create the pods using those service accounts and adding the annotations, so the services get registered and a sidecar is injected.

```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: counting
  annotations:
    'consul.hashicorp.com/connect-inject': 'true'
    "consul.hashicorp.com/connect-service": "counting"
spec:
  serviceAccountName: counting
  automountServiceAccountToken: true
  containers:
    - name: counting
      image: hashicorp/counting-service:0.0.2
      ports:
        - containerPort: 9001
          name: http
```

```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: dashboard
  labels:
    app: 'dashboard'
  annotations:
    'consul.hashicorp.com/connect-inject': 'true'
    'consul.hashicorp.com/connect-service-upstreams': 'counting:9001'
spec:
  serviceAccountName: dashboard
  automountServiceAccountToken: true
  containers:
    - name: dashboard
      image: hashicorp/dashboard-service:0.0.3
      ports:
        - containerPort: 9002
          name: http
      env:
        - name: COUNTING_SERVICE_URL
          value: 'http://localhost:9001'
```

Notice how the dashboard has an upstream defined as counting:9001. This will send all traffic to localhost:9001 to the sidecar proxy of the counting service.

### Creating intentions
Assuming you now have two services running in Kubernetes: a dashboard that shows the current value and a counting service that increases the count with each request. Both are configured to communicate via the Envoy sidecar proxy.

By default, Consul Connect denies all traffic through the service mesh. In order for traffic from the dashboard to be able to reach the backend service, you need to define an intention that allows traffic from the dashboard to the backend service.

You can create this intention using the TrafficTarget CRD below, store it as `intention.yaml` and apply it using `kubectl apply -f intention.yaml`.

```yaml
# TCPRoute for Counting Service.
---
apiVersion: specs.smi-spec.io/v1alpha1
kind: TCPRoute
metadata:
  name: service-counting-tcp-route
```

```yaml
# TrafficTarget defines allowed routes for counting.
# In this example dashboard is allow to connect using
# TCP.
---
kind: TrafficTarget
apiVersion: access.smi-spec.io/v1alpha1
metadata:
  name: counting-traffic-target
  namespace: default
destination:
  kind: ServiceAccount
  name: counting
  namespace: default
sources:
- kind: ServiceAccount
  name: dashboard
  namespace: default
specs:
- kind: TCPRoute
  name: counting-tcp-route
```

This will create an intention in Consul that allows traffic from the dashboard service to the counting service.
With this intention created the dashboard will be able to show the current value retrieved from the counting backend.
