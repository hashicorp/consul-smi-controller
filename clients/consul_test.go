package clients

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"os"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
	"k8s.io/klog"
)

type testObjects struct {
	testAPIServer      *httptest.Server
	createdIntentions  []api.Intention
	deletedIntentions  []string
	intentionsResponse string
}

func (to *testObjects) apiHandler(rw http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.URL.Path == "/v1/connect/intentions/match" {
		to.handleIntentionMatch(rw, r)
	}

	if strings.HasPrefix(r.URL.Path, "/v1/connect/intentions") {
		to.handleIntention(rw, r)
	}
}

func (to *testObjects) handleIntentionMatch(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte(to.intentionsResponse))
}

func (to *testObjects) handleIntention(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		i := api.Intention{}
		err := json.NewDecoder(r.Body).Decode(&i)
		if err != nil {
			panic(err)
		}

		to.createdIntentions = append(to.createdIntentions, i)

		// send the response
		rw.Write([]byte(`{"ID": "abc123"}`))
	}

	if r.Method == http.MethodDelete {
		id := strings.Replace(r.URL.Path, "/v1/connect/intentions/", "", -1)
		to.deletedIntentions = append(to.deletedIntentions, id)
	}
}

func setupClient(t *testing.T, ir string) (Consul, *testObjects) {
	klog.SetOutput(os.Stdout)
	to := testObjects{
		intentionsResponse: ir,
		createdIntentions:  make([]api.Intention, 0),
		deletedIntentions:  make([]string, 0),
	}

	to.testAPIServer = httptest.NewServer(http.HandlerFunc(to.apiHandler))

	c, err := NewConsul(to.testAPIServer.URL, "")
	if err != nil {
		t.Fatal(err)
	}

	return c, &to
}

func TestSyncCreatesIntentions(t *testing.T) {
	c, to := setupClient(t, intentionsWithMeta)

	c.SyncIntentions([]string{"a", "d"}, "b")

	assert.Equal(t, 2, len(to.createdIntentions))

	assert.Equal(t, "a", to.createdIntentions[0].SourceName)
	assert.Equal(t, "b", to.createdIntentions[0].DestinationName)
	assert.Equal(t, "SMI", to.createdIntentions[0].Meta["CreatedBy"])

	assert.Equal(t, "d", to.createdIntentions[1].SourceName)
	assert.Equal(t, "b", to.createdIntentions[1].DestinationName)
	assert.Equal(t, "SMI", to.createdIntentions[1].Meta["CreatedBy"])
}

func TestSyncDoesNotCreateIntentionWhenExists(t *testing.T) {
	c, to := setupClient(t, intentionsWithMeta)

	c.SyncIntentions([]string{"c"}, "b")

	assert.Equal(t, 0, len(to.createdIntentions))
}

func TestSyncDeletesIntention(t *testing.T) {
	c, to := setupClient(t, intentionsWithMeta)

	c.SyncIntentions([]string{}, "b")

	assert.Equal(t, 2, len(to.deletedIntentions))
	assert.Equal(t, "ed16f6a6-d863-1bec-af45-96bbdcbe02be", to.deletedIntentions[0])
	assert.Equal(t, "e9ebc19f-d481-42b1-4871-4d298d3acd5c", to.deletedIntentions[1])
}

func TestSyncDeletesIntentionHonoringMeta(t *testing.T) {
	c, to := setupClient(t, intentionsOnly1WithMeta)

	c.SyncIntentions([]string{}, "b")

	assert.Equal(t, 1, len(to.deletedIntentions))
	assert.Equal(t, "ed16f6a6-d863-1bec-af45-96bbdcbe02be", to.deletedIntentions[0])
}

var intentionsWithMeta = `
{
  "b": [
    {
      "ID": "ed16f6a6-d863-1bec-af45-96bbdcbe02be",
      "Description": "",
      "SourceNS": "default",
      "SourceName": "c",
      "DestinationNS": "default",
      "DestinationName": "b",
      "SourceType": "consul",
      "Action": "deny",
      "DefaultAddr": "",
      "DefaultPort": 0,
			"Meta": {"CreatedBy":"SMI"},
      "CreatedAt": "2018-05-21T16:41:33.296693825Z",
      "UpdatedAt": "2018-05-21T16:41:33.296694288Z",
      "CreateIndex": 12,
      "ModifyIndex": 12
    },
    {
      "ID": "e9ebc19f-d481-42b1-4871-4d298d3acd5c",
      "Description": "",
      "SourceNS": "default",
      "SourceName": "web",
      "DestinationNS": "default",
      "DestinationName": "b",
      "SourceType": "consul",
      "Action": "allow",
      "DefaultAddr": "",
      "DefaultPort": 0,
			"Meta": {"CreatedBy":"SMI"},
      "CreatedAt": "2018-05-21T16:41:27.977155457Z",
      "UpdatedAt": "2018-05-21T16:41:27.977157724Z",
      "CreateIndex": 11,
      "ModifyIndex": 11
    }
  ]
}
`

var intentionsOnly1WithMeta = `
{
  "b": [
    {
      "ID": "ed16f6a6-d863-1bec-af45-96bbdcbe02be",
      "Description": "",
      "SourceNS": "default",
      "SourceName": "c",
      "DestinationNS": "default",
      "DestinationName": "b",
      "SourceType": "consul",
      "Action": "deny",
      "DefaultAddr": "",
      "DefaultPort": 0,
			"Meta": {"CreatedBy":"SMI"},
      "CreatedAt": "2018-05-21T16:41:33.296693825Z",
      "UpdatedAt": "2018-05-21T16:41:33.296694288Z",
      "CreateIndex": 12,
      "ModifyIndex": 12
    },
    {
      "ID": "e9ebc19f-d481-42b1-4871-4d298d3acd5c",
      "Description": "",
      "SourceNS": "default",
      "SourceName": "web",
      "DestinationNS": "default",
      "DestinationName": "b",
      "SourceType": "consul",
      "Action": "allow",
      "DefaultAddr": "",
      "DefaultPort": 0,
			"Meta": {},
      "CreatedAt": "2018-05-21T16:41:27.977155457Z",
      "UpdatedAt": "2018-05-21T16:41:27.977157724Z",
      "CreateIndex": 11,
      "ModifyIndex": 11
    }
  ]
}
`
