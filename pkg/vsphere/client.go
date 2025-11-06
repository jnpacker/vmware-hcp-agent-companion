/*
Copyright 2025 Red Hat, Inc.
This file contains code generated or modified with AI assistance.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vsphere

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client wraps the vSphere client and provides high-level operations
type Client struct {
	client     *govmomi.Client
	finder     *find.Finder
	datacenter *object.Datacenter
}

// Credentials holds vSphere connection information
type Credentials struct {
	Server        string
	Username      string
	Password      string
	Insecure      bool
	CACertificate string
}

// NewClient creates a new vSphere client
func NewClient(ctx context.Context, creds *Credentials) (*Client, error) {
	u, err := url.Parse(fmt.Sprintf("https://%s/sdk", creds.Server))
	if err != nil {
		return nil, fmt.Errorf("failed to parse vSphere URL: %w", err)
	}

	u.User = url.UserPassword(creds.Username, creds.Password)

	// Skip TLS verification for now (testing)
	// TODO: Implement proper TLS certificate validation
	insecure := true

	vimClient, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to create vSphere client: %w", err)
	}

	finder := find.NewFinder(vimClient.Client, true)

	return &Client{
		client: vimClient,
		finder: finder,
	}, nil
}

// LoadCredentialsFromSecret loads vSphere credentials from a Kubernetes Secret
func LoadCredentialsFromSecret(ctx context.Context, k8sClient client.Client, namespace, name string) (*Credentials, error) {
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, secret); err != nil {
		return nil, fmt.Errorf("failed to get vSphere credentials secret: %w", err)
	}

	creds := &Credentials{
		Server:        string(secret.Data["vCenter"]),
		Username:      string(secret.Data["username"]),
		Password:      string(secret.Data["password"]),
		Insecure:      string(secret.Data["insecure"]) == "true",
		CACertificate: string(secret.Data["cacertificate"]),
	}

	if creds.Server == "" || creds.Username == "" || creds.Password == "" {
		return nil, fmt.Errorf("vSphere credentials secret is missing required fields (vCenter, username, password)")
	}

	return creds, nil
}

// SetDatacenter sets the datacenter for subsequent operations
func (c *Client) SetDatacenter(ctx context.Context, datacenterName string) error {
	dc, err := c.finder.Datacenter(ctx, datacenterName)
	if err != nil {
		return fmt.Errorf("failed to find datacenter %s: %w", datacenterName, err)
	}

	c.datacenter = dc
	c.finder.SetDatacenter(dc)
	return nil
}

// Close closes the vSphere client connection
func (c *Client) Close(ctx context.Context) error {
	if c.client != nil {
		return c.client.Logout(ctx)
	}
	return nil
}

// GetDatastore finds a datastore by name
func (c *Client) GetDatastore(ctx context.Context, name string) (*object.Datastore, error) {
	return c.finder.Datastore(ctx, name)
}

// GetNetwork finds a network by name
func (c *Client) GetNetwork(ctx context.Context, name string) (object.NetworkReference, error) {
	return c.finder.Network(ctx, name)
}

// GetResourcePool finds a resource pool by name or returns the cluster's root pool
func (c *Client) GetResourcePool(ctx context.Context, clusterName, poolName string) (*object.ResourcePool, error) {
	if poolName != "" {
		return c.finder.ResourcePool(ctx, poolName)
	}

	// Get cluster's root resource pool
	cluster, err := c.finder.ClusterComputeResource(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster %s: %w", clusterName, err)
	}

	return cluster.ResourcePool(ctx)
}

// GetFolder finds or creates a VM folder
func (c *Client) GetFolder(ctx context.Context, folderPath string) (*object.Folder, error) {
	if folderPath == "" {
		// Return datacenter's VM folder
		folders, err := c.datacenter.Folders(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get datacenter folders: %w", err)
		}
		return folders.VmFolder, nil
	}

	folder, err := c.finder.Folder(ctx, folderPath)
	if err == nil {
		// Folder exists, return it
		return folder, nil
	}

	// Folder doesn't exist, try to create it
	folders, err := c.datacenter.Folders(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get datacenter folders: %w", err)
	}

	folder, err = folders.VmFolder.CreateFolder(ctx, folderPath)
	if err != nil {
		// Check if the error is because the folder already exists
		// If so, try to find it again
		if strings.Contains(err.Error(), "already exists") {
			folder, err := c.finder.Folder(ctx, folderPath)
			if err != nil {
				return nil, fmt.Errorf("failed to find existing folder %s: %w", folderPath, err)
			}
			return folder, nil
		}
		return nil, fmt.Errorf("failed to create folder %s: %w", folderPath, err)
	}

	return folder, nil
}

// Validate checks if the client can connect to vSphere and access resources
func (c *Client) Validate(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}

	// Check if we're still logged in
	_, err := c.client.SessionManager.UserSession(ctx)
	if err != nil {
		return fmt.Errorf("vSphere session validation failed: %w", err)
	}

	return nil
}
