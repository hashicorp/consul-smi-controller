package clients

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
)

var testAPIServer *httptest.Server
var createdIntentions []api.Intention
var deletedIntentions []string

func apiHandler(rw http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.URL.Path == "/v1/connect/intentions/match" {
		handleIntentionMatch(rw, r)
	}

	if strings.HasPrefix(r.URL.Path, "/v1/connect/intentions") {
		handleIntention(rw, r)
	}
}

func handleIntentionMatch(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte(intentionMatchWithIntentions))
}

func handleIntention(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		i := api.Intention{}
		err := json.NewDecoder(r.Body).Decode(&i)
		if err != nil {
			panic(err)
		}

		createdIntentions = append(createdIntentions, i)
	}

	if r.Method == http.MethodDelete {
		id := strings.Replace(r.URL.Path, "/v1/connect/intentions/", "", -1)
		deletedIntentions = append(deletedIntentions, id)
	}
}

func setupClient(t *testing.T) Consul {
	createdIntentions = make([]api.Intention, 0)
	testAPIServer = httptest.NewServer(http.HandlerFunc(apiHandler))

	c, err := NewConsul(testAPIServer.URL, "")
	if err != nil {
		t.Fatal(err)
	}

	return c
}

func TestSyncCreatesIntention(t *testing.T) {
	c := setupClient(t)

	c.SyncIntentions([]string{"a"}, "b")

	assert.Equal(t, 1, len(createdIntentions))
	assert.Equal(t, "a", createdIntentions[0].SourceName)
	assert.Equal(t, "b", createdIntentions[0].DestinationName)
}

func TestSyncDeletesIntention(t *testing.T) {
	c := setupClient(t)

	c.SyncIntentions([]string{}, "")

	assert.Equal(t, 2, len(deletedIntentions))
	assert.Equal(t, "ed16f6a6-d863-1bec-af45-96bbdcbe02be", deletedIntentions[0])
	assert.Equal(t, "e9ebc19f-d481-42b1-4871-4d298d3acd5c", deletedIntentions[1])
}

var intentionMatchWithIntentions = `
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
      "Meta": {},
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
      "DestinationName": "*",
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
