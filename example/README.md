# SMI TrafficTarget Demo
The following instructions show how to run a simple example of the SMI TrafficTarget
spec on Kubernetes with HashiCorp Consul Service Mesh.

## Example Walkthrough
A walkthrough of these instructions can be found at the following link:

https://www.loom.com/share/6907c66621294bbea75cc021c70a89c5
**password**: smisecret

## Setting up Consul
To run the example Consul and the SMI controller must be running on Kuberenetes.
The controller has been built to take advantage of the latest features in
Consul 1.5 which integrates Consul ACL tokens with K8s service tokens.

When dealing with security in a service mesh, service identity must be tightly 
controlled, services should only be allowed to assume the identity which has been
issued to them. In the event that an individual service is compromised, the service should
not be able to reconfigure itself to obtain a different service identity which allows
it to bypass network security policy. 

Consul ACL tokens control the identity which a service can obtain, however this system
is designed to work in a wider context than Kubernetes. To simplify integration when
working with Kuberentes, K8s service tokens can be associated with Consul ACL tokens.

When a service starts it uses the service token from the assigned K8s service account to
obtain a Consul ACL token.  K8s service tokens are cryptographically verifiable and before 
Consul issues the ACL token which can be used to obtain a service mesh identity it
validates the K8s service token with the K8s API. The name of the service account is 
mapped to a service identity inside of Consul, this is linked to the ACL token which is
returned to the service.

The service now has the ability to obtain a service mesh identity, and register itself
with the Consul service catalog.

All of this happens transparently when the pod starts, as long as the pod has the correct
service account assigned to it, it can participate in the service mesh.

### 1. Deploy Consul to Kubernetes
There is an official Helm chart for running Consul on Kuberenetes, but for convienience
I have already configured the options and generated flat Kuberentes config from this chart.

The following command will setup a single Consul Server instance and daemon set which runs
Consul Agent on each node. Applications do not directly communicate with Consul, they do
so through the local agent. The benefit of this approach is that the local agent can manage
caching and load balancing to the Consul Server. Should the Consul Server fail then due
to Agent caching the Service Mesh will continue to work.

```bash
$ kubectl apply -f ./consul --validate=false
```

It should take about 60 seconds for the Server to be deployed and all checks to become
healthy. For convenience the Consul UI has been mapped to a Public IP using a LoadBalancer.

### 2. Deploy the SMI CRDs and Controller
The next step is to deploy the CRDs for the SMI spec and the SMI controller for Consul.
The controller is currently available in a private repository which requires credentials
to access. The `crd.yaml` file contains the credentials for the private repository and
a pod spec to deploy the controller.

```bash
$ kubectl apply -f ./crd.yaml
```

### 3. Deploy the demo application
The demo application is a simple two tier application consisting of a graphical 
dashboard and a back end service. The dashboard shows a continually incrementing count
which is retrieved from the backend service.

Communication between the dashboard and the counting service is passed through the 
service mesh. By default all service mesh traffic is set to `Deny All`. When you first
load the dashboard you will see that it can not connect to the counting service.
This is because we have not yet configured the TrafficTarget which creates the consul
Intentions to allow traffic between two services.

```bash
$ kubectl apply -f ./example.yaml
```

The application is now setup to demo the SMI `TrafficTarget`.

### Main Demo
Open the dashboard to the counting service.

```bash
# Linux
$ xdg-open http://$(kubectl get svc counting-dashboard -o jsonpath="{.status.loadBalancer.ingress[0].ip}")
# or Mac
$ open http://$(kubectl get svc counting-dashboard -o jsonpath="{.status.loadBalancer.ingress[0].ip}")
```

Explain the dashboard connects to an upstream API and that all the traffic is flowing through the service mesh.  
By default all service mesh traffic is denied, to allow traffic between services we need to configure 
the service mesh. This configuration is specific from service mesh to service mesh. The example I am showing
uses the HashiCorp Consul Service Mesh. In Consul you need to define intentions to allow or deny traffic.

**Open the Consul UI** and show intentions tab

```bash
# Linux
$ xdg-open http://$(kubectl get svc consul-consul-ui -o jsonpath="{.status.loadBalancer.ingress[0].ip}")
# or Mac
$ open http://$(kubectl get svc consul-consul-ui -o jsonpath="{.status.loadBalancer.ingress[0].ip}")
```

At present we do not have any intentions so the dashboard will not be able to communicate with the upstream service, 
SMI defines a specification called TrafficTarget, it provides a Kubernetes centric way of managing
Service Mesh security. 

It looks like this:

```yaml
# TCPRoute for Counting Service
---
apiVersion: specs.smi-spec.io/v1alpha1
kind: TCPRoute
metadata:
  name: service-counting-tcp-route

# TrafficTarget defines allowed routes for service-a
# In this example service-b is allow to connect using
# TCP
---
kind: TrafficTarget
apiVersion: access.smi-spec.io/v1alpha1
metadata:
  name: service-counting-targets
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
  name: service-counting-tcp-route
```

When you apply this resource the controller will interpret this and make changes to the service mesh.
We are stating that we would like to allow traffic from a source with identity
dashboard to a destination with an identity counting.

If I apply this configuration:

```
$ kubectl apply -f smi.yaml
```

You can see in the Consul UI (refresh intentions), that the TrafficTarget controller
has correctly configured the Consul Service Mesh.
If I reload my dashboard, everything is now working as expected as the correct
security configuration has been applied.


### Notes
If you delete the TrafficTarget resource the controller will correctly delete
the intention however connections are persistent between a source and destination.
This is far more efficient than establishing a new connection every time and Envoy 
having to authorize the connection. To force the closure of a connection the easiest
approach is to delete the pods and re-create.

The whole process works as Consul (Control Plane) configures Envoy (Data Plane) with a TLS certificate and client
certificate. When Envoy connects to an upstream the Envoy proxy at the other end requests
that the downstream sends its client certificate (standard mTLS). The upstream then
validates that the certificate is signed by the same chain of trust that its own
certificate is signed. This completes the authentication part of the process.
Secondary to authentication Envoy uses the SPIFFE id encoded into the client certificate
and makes a call to the Control Plane requesting an Authorization check. The control
plane determines if the source service is allowed to connect to the destination using
the Intentions graph configured by the TrafficTarget.
