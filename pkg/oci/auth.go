package oci

import (
	"github.com/oras-project/oras-credentials-go"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// NewAuthClient creates an auth.Client that resolves credentials from the
// Docker credential store (~/.docker/config.json and credential helpers).
func NewAuthClient() *auth.Client {
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		// Fall back to no auth if Docker config is unavailable
		return &auth.Client{
			Client: retry.DefaultClient,
			Cache:  auth.NewCache(),
		}
	}
	return &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(store),
	}
}
