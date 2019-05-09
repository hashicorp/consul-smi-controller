package clients

import "github.com/hashicorp/consul/api"

// Consul defines an interface for a Consul client
type Consul interface {
	// IntentionsExists returns true of false if the intention exists or not
	IntentionExists(source string, destination string) (bool, error)

	// DeleteIntention deletes an intention in Consul
	DeleteIntention() error

	// CreateIntention creates an intention in Consul
	CreateIntention(source string, destination string) (bool, error)
}

// ConsulImpl concrete implementation of the Consul client interface
type ConsulImpl struct {
	client *api.Client
}

// NewConsul creates a new Consul client
func NewConsul(httpAddr, aclToken string) (Consul, error) {
	conf := api.DefaultConfig()
	conf.Address = httpAddr
	conf.Token = aclToken

	cli, err := api.NewClient(conf)
	if err != nil {
		return nil, err
	}

	return &ConsulImpl{cli}, nil
}

// IntentionExists returns true of false if the intention exists or not
func (c *ConsulImpl) IntentionExists(source string, destination string) (bool, error) {
	args := api.IntentionCheck{
		Source:      source,
		Destination: destination,
	}

	ok, _, err := c.client.Connect().IntentionCheck(&args, nil)
	if err != nil {
		return false, err
	}

	return ok, nil
}

// CreateIntention creates an intention in Consul
func (c *ConsulImpl) CreateIntention(source string, destination string) (bool, error) {
	// first check to see if we need to create
	ok, err := c.IntentionExists(source, destination)
	if err != nil {
		return false, err
	}

	// if we have an intention just return
	if ok {
		return false, nil
	}

	in := api.Intention{
		SourceName:      source,
		DestinationName: destination,
		Action:          api.IntentionActionAllow,
		Description:     "Automatically added by Kubernetes",
	}

	_, _, err = c.client.Connect().IntentionCreate(&in, nil)
	if err != nil {
		return false, err
	}

	return true, nil
}

// DeleteIntention deletes an intention in Consul
func (c *ConsulImpl) DeleteIntention() error {
	return nil
}
