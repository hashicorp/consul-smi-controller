package clients

import "github.com/stretchr/testify/mock"

// ConsulMock is a mock implementation of the Consul client
type ConsulMock struct {
	Mock mock.Mock
}

// SyncIntentions syncs the intentions in Consul
func (c *ConsulMock) SyncIntentions(source []string, destination string) error {
	args := c.Mock.Called(source, destination)
	return args.Error(0)
}
