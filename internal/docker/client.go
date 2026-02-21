package docker

import "github.com/moby/moby/client"

func NewClient() (*client.Client, error) {
	return client.New(client.WithAPIVersionFromEnv())
}
