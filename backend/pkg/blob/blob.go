// Package blob owns the Azure Blob Storage client (Azurite in local dev). It
// ensures the resumes container exists on boot and exposes a health check.
package blob

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

// Client wraps the azblob client together with the working container name.
type Client struct {
	client    *azblob.Client
	container string
}

// Connect builds a client from the connection string and ensures the container
// exists (create-if-not-exists is idempotent).
func Connect(ctx context.Context, connString, containerName string) (*Client, error) {
	c, err := azblob.NewClientFromConnectionString(connString, nil)
	if err != nil {
		return nil, fmt.Errorf("blob: client: %w", err)
	}
	bc := &Client{client: c, container: containerName}
	if err := bc.ensureContainer(ctx); err != nil {
		return nil, err
	}
	return bc, nil
}

// ensureContainer creates the container, treating "already exists" as success.
func (c *Client) ensureContainer(ctx context.Context) error {
	_, err := c.client.CreateContainer(ctx, c.container, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return fmt.Errorf("blob: create container %q: %w", c.container, err)
	}
	return nil
}

// HealthCheck confirms the storage endpoint is reachable and the container is
// listable by requesting the first page of blobs.
func (c *Client) HealthCheck(ctx context.Context) error {
	pager := c.client.NewListBlobsFlatPager(c.container, nil)
	if _, err := pager.NextPage(ctx); err != nil {
		return fmt.Errorf("blob: health: %w", err)
	}
	return nil
}
