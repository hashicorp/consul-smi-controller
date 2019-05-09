package clients

import "github.com/hashicorp/consul/api"

// Consul defines an interface for a Consul client
type Consul interface {
	// LookupServiceWithJWT allows you to lookup a service in consul using a
	// Kubernetes service token. This requires that the service has been configured
	// to use ACL Auth Mounts
	LookupServiceWithJWT(jwt string) (string, error)

	// GetIntentions returns a list of intentions currently configured in
	// Consul
	GetIntentions() error

	// DeleteIntention deletes an intention in Consul
	DeleteIntention() error

	// CreateIntention creates an intention in Consul
	CreateIntention(in api.Intention) (bool, error)
}

// ConsulImpl concrete implementation of the Consul client interface
type ConsulImpl struct {
	client *api.Client
}

// NewConsul creates a new Consul client
func NewConsul() {}

// LookupServiceWithJWT looks up a Connect service using a K8s JWT
func (c *ConsulImpl) LookupServiceWithJWT(jwt string) (string, error) {

}
