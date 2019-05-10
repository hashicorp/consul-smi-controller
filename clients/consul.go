package clients

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

// Consul defines an interface for a Consul client
type Consul interface {
	// SyncIntetions will update the list of intentions in Consul to match
	// the provided source and destinations
	SyncIntentions(source []string, destination string) error
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

// createIntention creates an intention in Consul
func (c *ConsulImpl) createIntention(source string, destination string) error {
	in := api.Intention{
		SourceName:      source,
		DestinationName: destination,
		Action:          api.IntentionActionAllow,
		Description:     "Automatically added by Kubernetes",
	}

	_, _, err := c.client.Connect().IntentionCreate(&in, nil)
	if err != nil {
		return err
	}

	return nil
}

// deleteIntention deletes an intention in Consul
func (c *ConsulImpl) deleteIntention(id string) error {
	_, err := c.client.Connect().IntentionDelete(id, nil)
	return err
}

// SyncIntentions will update the list of intentions in Consul to match
// the provided source and destinations
func (c *ConsulImpl) SyncIntentions(source []string, destination string) error {
	// Get a list of intentions from Consul matching the destination

	fmt.Println("Syncing Intetions", source, destination)
	in, _, err := c.client.Connect().IntentionMatch(
		&api.IntentionMatch{
			By:    api.IntentionMatchDestination,
			Names: []string{destination},
		},
		&api.QueryOptions{},
	)

	if err != nil {
		return err
	}

	deleted := make([]string, 0)
	created := make([]string, 0)

	intentions := in[destination]

	// process deletions
	for _, v := range intentions {
		exists := false
		for _, s := range source {
			if v.SourceName == s && v.DestinationName == destination {
				fmt.Println("d stuff", v.SourceName, v.DestinationName)
				exists = true
				break
			}
		}

		if !exists {
			deleted = append(deleted, v.ID)
		}
	}

	// process creations
	for _, s := range source {
		exists := false
		for _, v := range intentions {
			if v.SourceName == s && v.DestinationName == destination {
				fmt.Println("c stuff", v.SourceName, v.DestinationName)
				exists = true
				break
			}
		}

		if !exists {
			created = append(created, s)
		}
	}

	for _, d := range deleted {
		fmt.Printf("Deleting: %s - %s\n", d, destination)
		if err := c.deleteIntention(d); err != nil {
			return err
		}
	}

	for _, cr := range created {
		fmt.Printf("Created: %s - %s\n", cr, destination)
		if err := c.createIntention(cr, destination); err != nil {
			return err
		}
	}

	return nil
}
