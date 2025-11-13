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
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
)

var (
	// NodePool GVK for HyperShift
	nodePoolGVK = schema.GroupVersionKind{
		Group:   "hypershift.openshift.io",
		Version: "v1beta1",
		Kind:    "NodePool",
	}
	// Annotation to track NodePool connection on VMwareNodePoolTemplate
	nodePoolRefAnnotation = "vmware.hcp.open-cluster-management.io/nodepool-ref"
)

// getNodePoolReplicas fetches the replica count from the referenced NodePool
// The NodePool must be in the same namespace as the VMwareNodePoolTemplate
func (r *VMwareNodePoolTemplateReconciler) getNodePoolReplicas(ctx context.Context, template *vmwarev1alpha1.VMwareNodePoolTemplate, log logr.Logger) (int32, error) {
	if template.Spec.NodePoolRef == nil {
		return 0, fmt.Errorf("NodePoolRef not specified")
	}

	// NodePool must be in the same namespace as the template
	namespace := template.Namespace
	nodePoolName := template.Spec.NodePoolRef.Name

	// Get NodePool as unstructured (we don't have the type imported)
	nodePool := &unstructured.Unstructured{}
	nodePool.SetGroupVersionKind(nodePoolGVK)

	if err := r.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      nodePoolName,
	}, nodePool); err != nil {
		// Provide a detailed, user-friendly error message
		detail := fmt.Sprintf("NodePool '%s' not found in namespace '%s'. VMwareNodePoolTemplate must be created in the same namespace as the NodePool. Please verify the NodePool exists in this namespace.", nodePoolName, namespace)
		log.Error(err, detail, "nodePool", nodePoolName, "namespace", namespace)
		return 0, fmt.Errorf("%s", detail)
	}

	// Get replicas from spec
	replicas, found, err := unstructured.NestedInt64(nodePool.Object, "spec", "replicas")
	if err != nil {
		return 0, fmt.Errorf("failed to read replicas field from NodePool '%s/%s': %w", namespace, nodePoolName, err)
	}
	if !found {
		// Default to 0 if not specified
		log.Info("NodePool has no replicas field, defaulting to 0", "nodePool", nodePoolName, "namespace", namespace)
		return 0, nil
	}

	log.Info("Successfully read NodePool replicas", "nodePool", nodePoolName, "namespace", namespace, "replicas", replicas)
	return int32(replicas), nil
}

// getNodePool fetches the referenced NodePool object
func (r *VMwareNodePoolTemplateReconciler) getNodePool(ctx context.Context, template *vmwarev1alpha1.VMwareNodePoolTemplate, log logr.Logger) (*unstructured.Unstructured, error) {
	if template.Spec.NodePoolRef == nil {
		return nil, fmt.Errorf("NodePoolRef not specified")
	}

	namespace := template.Namespace
	nodePoolName := template.Spec.NodePoolRef.Name

	nodePool := &unstructured.Unstructured{}
	nodePool.SetGroupVersionKind(nodePoolGVK)

	if err := r.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      nodePoolName,
	}, nodePool); err != nil {
		return nil, err
	}

	return nodePool, nil
}

// ensureNodePoolMatchLabels ensures that the NodePool has matchLabels configured
// If matchLabels is not set, it will be populated with:
// - The nodepool name label (vmware.hcp.open-cluster-management.io/nodepool)
// - Any labels from the template's agentLabelSelector
func (r *VMwareNodePoolTemplateReconciler) ensureNodePoolMatchLabels(
	ctx context.Context,
	nodePool *unstructured.Unstructured,
	template *vmwarev1alpha1.VMwareNodePoolTemplate,
	log logr.Logger,
) error {
	// Check if matchLabels already exists
	matchLabels, found, err := unstructured.NestedMap(nodePool.Object, "spec", "platform", "agent", "agentLabelSelector", "matchLabels")
	if err != nil {
		return fmt.Errorf("failed to check matchLabels: %w", err)
	}

	// If matchLabels is already set, don't modify it
	if found && len(matchLabels) > 0 {
		log.V(1).Info("NodePool already has matchLabels configured, skipping update")
		return nil
	}

	// Build the matchLabels map
	newMatchLabels := make(map[string]interface{})

	// Add the nodepool name label
	nodePoolLabel := "vmware.hcp.open-cluster-management.io/nodepool"
	nodePoolName := template.Spec.NodePoolRef.Name
	newMatchLabels[nodePoolLabel] = nodePoolName

	// Add any labels from the template's agentLabelSelector
	for key, value := range template.Spec.AgentLabelSelector {
		newMatchLabels[key] = value
	}

	// Set the matchLabels in the NodePool
	if err := unstructured.SetNestedMap(nodePool.Object, newMatchLabels, "spec", "platform", "agent", "agentLabelSelector", "matchLabels"); err != nil {
		return fmt.Errorf("failed to set matchLabels: %w", err)
	}

	// Update the NodePool
	if err := r.Update(ctx, nodePool); err != nil {
		return fmt.Errorf("failed to update NodePool with matchLabels: %w", err)
	}

	log.Info("Updated NodePool matchLabels", "nodePool", nodePool.GetName(), "matchLabels", newMatchLabels)
	return nil
}

// setNodePoolRefAnnotation adds or updates the NodePool reference annotation on the template
// This annotation tracks which NodePool is connected to this template for visibility
func (r *VMwareNodePoolTemplateReconciler) setNodePoolRefAnnotation(
	ctx context.Context,
	template *vmwarev1alpha1.VMwareNodePoolTemplate,
	log logr.Logger,
) error {
	if template.Spec.NodePoolRef == nil {
		// Remove annotation if no NodePool is referenced
		annotations := template.GetAnnotations()
		if annotations != nil && annotations[nodePoolRefAnnotation] != "" {
			delete(annotations, nodePoolRefAnnotation)
			template.SetAnnotations(annotations)
			if err := r.Update(ctx, template); err != nil {
				log.Error(err, "Failed to remove NodePool reference annotation")
				return err
			}
			log.V(1).Info("Removed NodePool reference annotation")
		}
		return nil
	}

	// Set annotation with NodePool reference
	annotations := template.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	nodePoolRef := fmt.Sprintf("%s/%s", template.Namespace, template.Spec.NodePoolRef.Name)
	if annotations[nodePoolRefAnnotation] != nodePoolRef {
		annotations[nodePoolRefAnnotation] = nodePoolRef
		template.SetAnnotations(annotations)
		if err := r.Update(ctx, template); err != nil {
			log.Error(err, "Failed to set NodePool reference annotation")
			return err
		}
		log.Info("Updated NodePool reference annotation", "nodePoolRef", nodePoolRef)
	}

	return nil
}
