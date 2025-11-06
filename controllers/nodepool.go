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
