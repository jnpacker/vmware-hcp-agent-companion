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
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
)

// TestGetNodePoolReplicas tests fetching replica count from NodePool
func TestGetNodePoolReplicas(t *testing.T) {
	tests := []struct {
		name             string
		nodePoolExists   bool
		nodePoolName     string
		replicas         int64
		expectError      bool
		expectedReplicas int32
	}{
		{
			name:             "NodePool with replicas",
			nodePoolExists:   true,
			nodePoolName:     "test-nodepool",
			replicas:         3,
			expectError:      false,
			expectedReplicas: 3,
		},
		{
			name:             "NodePool with zero replicas",
			nodePoolExists:   true,
			nodePoolName:     "test-nodepool",
			replicas:         0,
			expectError:      false,
			expectedReplicas: 0,
		},
		{
			name:             "NodePool with large replica count",
			nodePoolExists:   true,
			nodePoolName:     "test-nodepool",
			replicas:         100,
			expectError:      false,
			expectedReplicas: 100,
		},
		{
			name:           "NodePool not found",
			nodePoolExists: false,
			nodePoolName:   "missing-nodepool",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create scheme
			s := runtime.NewScheme()
			_ = vmwarev1alpha1.AddToScheme(s)
			_ = scheme.AddToScheme(s)

			// Create template
			template := &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-template",
					Namespace: "test-namespace",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					NodePoolRef: &vmwarev1alpha1.NodePoolReference{
						Name: tt.nodePoolName,
					},
				},
			}

			// Create client builder
			clientBuilder := fake.NewClientBuilder().WithScheme(s).WithObjects(template)

			// Create NodePool if it should exist
			if tt.nodePoolExists {
				nodePool := createFakeNodePool(tt.nodePoolName, "test-namespace", tt.replicas)
				clientBuilder = clientBuilder.WithRuntimeObjects(nodePool)
			}

			fakeClient := clientBuilder.Build()

			// Create reconciler
			r := &VMwareNodePoolTemplateReconciler{
				Client: fakeClient,
				Scheme: s,
			}

			// Call getNodePoolReplicas
			ctx := context.Background()
			replicas, err := r.getNodePoolReplicas(ctx, template, logr.Discard())

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if replicas != tt.expectedReplicas {
					t.Errorf("Expected replicas %d, got %d", tt.expectedReplicas, replicas)
				}
			}
		})
	}
}

// TestGetNodePool tests fetching NodePool object
func TestGetNodePool(t *testing.T) {
	tests := []struct {
		name           string
		nodePoolExists bool
		nodePoolName   string
		expectError    bool
	}{
		{
			name:           "Existing NodePool",
			nodePoolExists: true,
			nodePoolName:   "test-nodepool",
			expectError:    false,
		},
		{
			name:           "Missing NodePool",
			nodePoolExists: false,
			nodePoolName:   "missing-nodepool",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := runtime.NewScheme()
			_ = vmwarev1alpha1.AddToScheme(s)
			_ = scheme.AddToScheme(s)

			template := &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-template",
					Namespace: "test-namespace",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					NodePoolRef: &vmwarev1alpha1.NodePoolReference{
						Name: tt.nodePoolName,
					},
				},
			}

			clientBuilder := fake.NewClientBuilder().WithScheme(s).WithObjects(template)

			if tt.nodePoolExists {
				nodePool := createFakeNodePool(tt.nodePoolName, "test-namespace", 3)
				clientBuilder = clientBuilder.WithRuntimeObjects(nodePool)
			}

			fakeClient := clientBuilder.Build()

			r := &VMwareNodePoolTemplateReconciler{
				Client: fakeClient,
				Scheme: s,
			}

			ctx := context.Background()
			nodePool, err := r.getNodePool(ctx, template, logr.Discard())

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if nodePool == nil {
					t.Fatal("Expected NodePool but got nil")
				}
				if nodePool.GetName() != tt.nodePoolName {
					t.Errorf("Expected NodePool name %s, got %s", tt.nodePoolName, nodePool.GetName())
				}
			}
		})
	}
}

// TestNodePoolDeletion tests handling of deleted NodePools
func TestNodePoolDeletion(t *testing.T) {
	tests := []struct {
		name                  string
		nodePoolExists        bool
		nodePoolDeleting      bool
		expectDesiredReplicas int32
	}{
		{
			name:                  "NodePool exists and healthy",
			nodePoolExists:        true,
			nodePoolDeleting:      false,
			expectDesiredReplicas: 3,
		},
		{
			name:                  "NodePool being deleted",
			nodePoolExists:        true,
			nodePoolDeleting:      true,
			expectDesiredReplicas: 0,
		},
		{
			name:                  "NodePool already deleted",
			nodePoolExists:        false,
			nodePoolDeleting:      false,
			expectDesiredReplicas: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := runtime.NewScheme()
			_ = vmwarev1alpha1.AddToScheme(s)
			_ = scheme.AddToScheme(s)

			template := &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-template",
					Namespace: "test-namespace",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					NodePoolRef: &vmwarev1alpha1.NodePoolReference{
						Name: "test-nodepool",
					},
				},
			}

			clientBuilder := fake.NewClientBuilder().WithScheme(s).WithObjects(template)

			if tt.nodePoolExists {
				nodePool := createFakeNodePool("test-nodepool", "test-namespace", 3)
				if tt.nodePoolDeleting {
					// Add a finalizer first (required by fake client before setting deletionTimestamp)
					nodePool.SetFinalizers([]string{"test-finalizer"})
					now := metav1.Now()
					nodePool.SetDeletionTimestamp(&now)
				}
				clientBuilder = clientBuilder.WithRuntimeObjects(nodePool)
			}

			fakeClient := clientBuilder.Build()

			r := &VMwareNodePoolTemplateReconciler{
				Client: fakeClient,
				Scheme: s,
			}

			ctx := context.Background()
			nodePool, err := r.getNodePool(ctx, template, logr.Discard())

			if !tt.nodePoolExists {
				if err == nil {
					t.Fatal("Expected error for missing NodePool")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				// Check deletion timestamp
				isDeleting := nodePool.GetDeletionTimestamp() != nil
				if isDeleting != tt.nodePoolDeleting {
					t.Errorf("Expected deletion status %v, got %v", tt.nodePoolDeleting, isDeleting)
				}
			}
		})
	}
}

// TestNodePoolReferenceMissing tests handling of missing NodePoolRef
func TestNodePoolReferenceMissing(t *testing.T) {
	s := runtime.NewScheme()
	_ = vmwarev1alpha1.AddToScheme(s)
	_ = scheme.AddToScheme(s)

	template := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template",
			Namespace: "test-namespace",
		},
		Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
			NodePoolRef: nil, // Missing reference
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(template).
		Build()

	r := &VMwareNodePoolTemplateReconciler{
		Client: fakeClient,
		Scheme: s,
	}

	ctx := context.Background()

	// Should return error when NodePoolRef is nil
	_, err := r.getNodePoolReplicas(ctx, template, logr.Discard())
	if err == nil {
		t.Error("Expected error when NodePoolRef is nil")
	}

	_, err = r.getNodePool(ctx, template, logr.Discard())
	if err == nil {
		t.Error("Expected error when NodePoolRef is nil")
	}
}

// TestNodePoolInDifferentNamespace tests that NodePool must be in same namespace
func TestNodePoolInDifferentNamespace(t *testing.T) {
	s := runtime.NewScheme()
	_ = vmwarev1alpha1.AddToScheme(s)
	_ = scheme.AddToScheme(s)

	template := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template",
			Namespace: "namespace-a",
		},
		Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
			NodePoolRef: &vmwarev1alpha1.NodePoolReference{
				Name: "test-nodepool",
			},
		},
	}

	// Create NodePool in different namespace
	nodePool := createFakeNodePool("test-nodepool", "namespace-b", 3)

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(template).
		WithRuntimeObjects(nodePool).
		Build()

	r := &VMwareNodePoolTemplateReconciler{
		Client: fakeClient,
		Scheme: s,
	}

	ctx := context.Background()

	// Should not find NodePool in different namespace
	_, err := r.getNodePoolReplicas(ctx, template, logr.Discard())
	if err == nil {
		t.Error("Expected error when NodePool is in different namespace")
	}
}

// TestMapNodePoolToTemplates tests the mapping function
func TestMapNodePoolToTemplates(t *testing.T) {
	s := runtime.NewScheme()
	_ = vmwarev1alpha1.AddToScheme(s)
	_ = scheme.AddToScheme(s)

	// Create templates that reference the NodePool
	template1 := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "template-1",
			Namespace: "test-namespace",
		},
		Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
			NodePoolRef: &vmwarev1alpha1.NodePoolReference{
				Name: "test-nodepool",
			},
		},
	}

	template2 := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "template-2",
			Namespace: "test-namespace",
		},
		Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
			NodePoolRef: &vmwarev1alpha1.NodePoolReference{
				Name: "test-nodepool",
			},
		},
	}

	// Template that references a different NodePool
	template3 := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "template-3",
			Namespace: "test-namespace",
		},
		Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
			NodePoolRef: &vmwarev1alpha1.NodePoolReference{
				Name: "other-nodepool",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(template1, template2, template3).
		Build()

	r := &VMwareNodePoolTemplateReconciler{
		Client: fakeClient,
		Scheme: s,
	}

	ctx := context.Background()

	// Create NodePool object
	nodePool := createFakeNodePool("test-nodepool", "test-namespace", 3)

	// Call mapNodePoolToTemplates
	requests := r.mapNodePoolToTemplates(ctx, nodePool)

	// Should return reconcile requests for template1 and template2
	if len(requests) != 2 {
		t.Fatalf("Expected 2 reconcile requests, got %d", len(requests))
	}

	// Verify the requests are for the correct templates
	foundTemplate1 := false
	foundTemplate2 := false
	for _, req := range requests {
		if req.Name == "template-1" && req.Namespace == "test-namespace" {
			foundTemplate1 = true
		}
		if req.Name == "template-2" && req.Namespace == "test-namespace" {
			foundTemplate2 = true
		}
		if req.Name == "template-3" {
			t.Error("Should not return request for template-3 (different NodePool)")
		}
	}

	if !foundTemplate1 {
		t.Error("Expected reconcile request for template-1")
	}
	if !foundTemplate2 {
		t.Error("Expected reconcile request for template-2")
	}
}

// TestNodePoolWithoutReplicasField tests handling of NodePool without replicas field
func TestNodePoolWithoutReplicasField(t *testing.T) {
	s := runtime.NewScheme()
	_ = vmwarev1alpha1.AddToScheme(s)
	_ = scheme.AddToScheme(s)

	template := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template",
			Namespace: "test-namespace",
		},
		Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
			NodePoolRef: &vmwarev1alpha1.NodePoolReference{
				Name: "test-nodepool",
			},
		},
	}

	// Create NodePool without replicas field
	nodePool := &unstructured.Unstructured{}
	nodePool.SetGroupVersionKind(nodePoolGVK)
	nodePool.SetName("test-nodepool")
	nodePool.SetNamespace("test-namespace")
	// Deliberately omit replicas field

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(template).
		WithRuntimeObjects(nodePool).
		Build()

	r := &VMwareNodePoolTemplateReconciler{
		Client: fakeClient,
		Scheme: s,
	}

	ctx := context.Background()
	replicas, err := r.getNodePoolReplicas(ctx, template, logr.Discard())

	// Should default to 0 when replicas field is missing
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if replicas != 0 {
		t.Errorf("Expected replicas to default to 0, got %d", replicas)
	}
}

// TestSetNodePoolRefAnnotation tests setting the NodePool reference annotation
func TestSetNodePoolRefAnnotation(t *testing.T) {
	tests := []struct {
		name               string
		nodePoolRef        *vmwarev1alpha1.NodePoolReference
		expectAnnotation   bool
		expectedValue      string
		existingAnnotation string
	}{
		{
			name: "Set annotation for NodePool reference",
			nodePoolRef: &vmwarev1alpha1.NodePoolReference{
				Name: "test-nodepool",
			},
			expectAnnotation: true,
			expectedValue:    "test-namespace/test-nodepool",
		},
		{
			name:               "Remove annotation when NodePoolRef is nil",
			nodePoolRef:        nil,
			expectAnnotation:   false,
			existingAnnotation: "test-namespace/old-nodepool",
		},
		{
			name: "Update annotation when NodePool reference changes",
			nodePoolRef: &vmwarev1alpha1.NodePoolReference{
				Name: "new-nodepool",
			},
			expectAnnotation:   true,
			expectedValue:      "test-namespace/new-nodepool",
			existingAnnotation: "test-namespace/old-nodepool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := runtime.NewScheme()
			_ = vmwarev1alpha1.AddToScheme(s)
			_ = scheme.AddToScheme(s)

			template := &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-template",
					Namespace: "test-namespace",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					NodePoolRef: tt.nodePoolRef,
				},
			}

			// Set existing annotation if provided
			if tt.existingAnnotation != "" {
				template.SetAnnotations(map[string]string{
					"vmware.hcp.open-cluster-management.io/nodepool-ref": tt.existingAnnotation,
				})
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(template).
				Build()

			r := &VMwareNodePoolTemplateReconciler{
				Client: fakeClient,
				Scheme: s,
			}

			ctx := context.Background()

			// Call setNodePoolRefAnnotation
			err := r.setNodePoolRefAnnotation(ctx, template, logr.Discard())
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Fetch updated template
			updatedTemplate := &vmwarev1alpha1.VMwareNodePoolTemplate{}
			err = fakeClient.Get(ctx, client.ObjectKey{
				Name:      "test-template",
				Namespace: "test-namespace",
			}, updatedTemplate)
			if err != nil {
				t.Fatalf("Failed to get updated template: %v", err)
			}

			// Verify annotation
			annotations := updatedTemplate.GetAnnotations()
			annotation := annotations["vmware.hcp.open-cluster-management.io/nodepool-ref"]

			if tt.expectAnnotation {
				if annotation != tt.expectedValue {
					t.Errorf("Expected annotation %s, got %s", tt.expectedValue, annotation)
				}
			} else {
				if annotation != "" {
					t.Errorf("Expected no annotation, got %s", annotation)
				}
			}
		})
	}
}

// Helper function to create fake NodePool
func createFakeNodePool(name, namespace string, replicas int64) *unstructured.Unstructured {
	nodePool := &unstructured.Unstructured{}
	nodePool.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "hypershift.openshift.io",
		Version: "v1beta1",
		Kind:    "NodePool",
	})
	nodePool.SetName(name)
	nodePool.SetNamespace(namespace)

	// Set replicas in spec
	_ = unstructured.SetNestedField(nodePool.Object, replicas, "spec", "replicas")

	return nodePool
}
