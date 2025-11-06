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

package controllers

import (
	"context"
	"crypto/sha256"
	"fmt"

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
	"github.com/example/vmware-hcp-agent-companion/pkg/vsphere"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// prepareISO ensures the Agent ISO is available in vSphere and manages the ISOReady condition
// On success: sets condition to True
// On failure: sets condition to False and returns error
func (r *VMwareNodePoolTemplateReconciler) prepareISO(ctx context.Context, vClient *vsphere.Client, template *vmwarev1alpha1.VMwareNodePoolTemplate, log logr.Logger) (string, error) {
	isoSpec := template.Spec.AgentISO
	isoPath := ""
	var prepareErr error

	switch isoSpec.Type {
	case "datastore":
		// ISO should already exist in datastore
		if isoSpec.DatastorePath == "" {
			prepareErr = fmt.Errorf("datastorePath is required when ISO type is 'datastore'")
		} else {
			// Verify the ISO exists
			datastore, err := vClient.GetDatastore(ctx, template.Spec.VMTemplate.Datastore)
			if err != nil {
				prepareErr = fmt.Errorf("failed to get datastore: %w", err)
			} else {
				exists, err := vClient.ISOExists(ctx, datastore, isoSpec.DatastorePath)
				if err != nil {
					prepareErr = fmt.Errorf("failed to check if ISO exists: %w", err)
				} else if !exists {
					prepareErr = fmt.Errorf("ISO not found at path: %s", isoSpec.DatastorePath)
				} else {
					// Success
					isoPath = isoSpec.DatastorePath
					log.Info("Using existing ISO from datastore", "path", isoSpec.DatastorePath)
				}
			}
		}

	case "url":
		// Download and upload ISO
		if isoSpec.URL == "" {
			prepareErr = fmt.Errorf("url is required when ISO type is 'url'")
		} else {
			datastore, err := vClient.GetDatastore(ctx, template.Spec.VMTemplate.Datastore)
			if err != nil {
				prepareErr = fmt.Errorf("failed to get datastore: %w", err)
			} else {
				// Generate ISO name
				isoName := isoSpec.UploadedISOName
				if isoName == "" {
					// Use hash of URL as name
					hash := sha256.Sum256([]byte(isoSpec.URL))
					isoName = fmt.Sprintf("agent-%x.iso", hash[:8])
				}

				// Full path in datastore
				isoRelPath := fmt.Sprintf("iso/%s", isoName)
				fullISOPath := vsphere.GetDatastorePath(template.Spec.VMTemplate.Datastore, isoRelPath)

				// Check if already uploaded
				if template.Status.ISOUploaded && template.Status.ISOPath == fullISOPath {
					exists, err := vClient.ISOExists(ctx, datastore, fullISOPath)
					if err == nil && exists {
						log.V(1).Info("ISO already uploaded", "path", fullISOPath)
						isoPath = fullISOPath
					} else {
						prepareErr = fmt.Errorf("ISO was marked as uploaded but not found at path: %s", fullISOPath)
					}
				}

				// Upload ISO if not already done
				if prepareErr == nil && isoPath == "" {
					log.Info("Uploading ISO from URL", "url", isoSpec.URL, "destination", fullISOPath)
					if err := vClient.UploadISO(ctx, datastore, isoSpec.URL, isoRelPath); err != nil {
						prepareErr = fmt.Errorf("failed to upload ISO: %w", err)
					} else {
						template.Status.ISOUploaded = true
						isoPath = fullISOPath
						log.Info("Successfully uploaded ISO", "path", fullISOPath)
					}
				}
			}
		}

	default:
		prepareErr = fmt.Errorf("unsupported ISO type: %s (must be 'datastore' or 'url')", isoSpec.Type)
	}

	// Handle condition management for all cases
	if prepareErr != nil {
		// Set condition to False on any failure
		template.SetCondition(vmwarev1alpha1.ConditionTypeISOReady, metav1.ConditionFalse,
			"ISOPreparationFailed", fmt.Sprintf("Failed to prepare ISO: %v", prepareErr))
		return "", prepareErr
	}

	// Set condition to True on success
	template.SetCondition(vmwarev1alpha1.ConditionTypeISOReady, metav1.ConditionTrue,
		"ISOReady", "ISO is ready for use")
	return isoPath, nil
}
