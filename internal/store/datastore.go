package store

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
)

// NewDatastoreClient creates a Cloud Datastore client using Application Default Credentials.
// On GAE this picks up the service account automatically.
func NewDatastoreClient(ctx context.Context, projectID string) (*datastore.Client, error) {
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("store: datastore.NewClient: %w", err)
	}
	return client, nil
}
