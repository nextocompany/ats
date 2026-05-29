// Package blob owns the Azure Blob Storage client (Azurite in local dev). It
// ensures the resumes container exists on boot and exposes a health check.
package blob

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
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

// Upload writes data to the named blob (overwriting if present) and returns the
// blob URL. Uploads are idempotent by name, which the pipeline relies on for
// safe task retries.
func (c *Client) Upload(ctx context.Context, name string, data []byte, contentType string) (string, error) {
	_, err := c.client.UploadBuffer(ctx, c.container, name, data, &azblob.UploadBufferOptions{
		HTTPHeaders: &blob.HTTPHeaders{BlobContentType: &contentType},
	})
	if err != nil {
		return "", fmt.Errorf("blob: upload %q: %w", name, err)
	}
	return strings.TrimRight(c.client.ServiceClient().URL(), "/") + "/" + c.container + "/" + name, nil
}

// Download reads the named blob into memory.
func (c *Client) Download(ctx context.Context, name string) ([]byte, error) {
	resp, err := c.client.DownloadStream(ctx, c.container, name, nil)
	if err != nil {
		return nil, fmt.Errorf("blob: download %q: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("blob: read %q: %w", name, err)
	}
	return buf.Bytes(), nil
}
