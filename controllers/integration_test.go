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
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
)

// TestAgentLabelMismatchScenarios tests scenarios where label selectors don't match
func TestAgentLabelMismatchScenarios(t *testing.T) {
	tests := []struct {
		name               string
		agentLabelSelector map[string]string
		agentLabels        map[string]string
		expectManaged      bool
		description        string
	}{
		{
			name: "Missing required NodePool label",
			agentLabelSelector: map[string]string{
				"hypershift.openshift.io/nodePool": "my-nodepool",
			},
			agentLabels: map[string]string{
				"hypershift.openshift.io/nodePool": "different-nodepool",
			},
			expectManaged: true, // Our controller will still manage it and update labels
			description:   "Agent with wrong nodePool label should be updated",
		},
		{
			name: "Agent has no labels at all",
			agentLabelSelector: map[string]string{
				"hypershift.openshift.io/nodePool": "my-nodepool",
			},
			agentLabels:   nil,
			expectManaged: true, // Will be managed and labels will be added
			description:   "Agent without labels should have labels added",
		},
		{
			name: "Extra labels on agent",
			agentLabelSelector: map[string]string{
				"env": "production",
			},
			agentLabels: map[string]string{
				"env":         "production",
				"team":        "platform",
				"extra-label": "extra-value",
			},
			expectManaged: true,
			description:   "Extra labels should be preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := createFakeAgent("test-agent", "test-namespace", tt.agentLabels, "test-uuid", "")

			template := &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-template",
					Namespace: "test-namespace",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					NodePoolRef: &vmwarev1alpha1.NodePoolReference{
						Name: "my-nodepool",
					},
					AgentLabelSelector: tt.agentLabelSelector,
				},
				Status: vmwarev1alpha1.VMwareNodePoolTemplateStatus{
					VMStatus: []vmwarev1alpha1.VMStatus{
						{
							Name: "test-vm",
							UUID: "test-uuid",
						},
					},
				},
			}

			s := runtime.NewScheme()
			_ = vmwarev1alpha1.AddToScheme(s)
			_ = scheme.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(template).
				WithRuntimeObjects(agent).
				Build()

			r := &VMwareNodePoolTemplateReconciler{
				Client:   fakeClient,
				Scheme:   s,
				Recorder: &fakeRecorder{},
			}

			ctx := context.Background()
			err := r.reconcileAgents(ctx, template, logr.Discard())
			if err != nil {
				t.Fatalf("reconcileAgents failed: %v", err)
			}

			// Verify the agent was managed
			updatedAgent := &unstructured.Unstructured{}
			updatedAgent.SetGroupVersionKind(agentGVK)
			err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-agent", Namespace: "test-namespace"}, updatedAgent)
			if err != nil {
				t.Fatalf("Failed to get updated agent: %v", err)
			}

			labels := updatedAgent.GetLabels()
			hasManagementLabel := labels["vmware.hcp.open-cluster-management.io/managed-by"] == "test-template"

			if tt.expectManaged && !hasManagementLabel {
				t.Errorf("%s: Expected agent to be managed but wasn't", tt.description)
			}

			// Verify user-defined labels were applied
			for key, expectedValue := range tt.agentLabelSelector {
				actualValue, found := labels[key]
				if !found {
					t.Errorf("Expected user-defined label %s not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("User-defined label %s: expected %s, got %s", key, expectedValue, actualValue)
				}
			}
		})
	}
}

// TestMissingAnnotationsForVMCleanup tests scenarios where annotations might be missing
func TestMissingAnnotationsForVMCleanup(t *testing.T) {
	tests := []struct {
		name                 string
		existingAnnotations  map[string]string
		expectAnnotationsSet bool
	}{
		{
			name:                 "No existing annotations",
			existingAnnotations:  nil,
			expectAnnotationsSet: true,
		},
		{
			name: "Some existing annotations",
			existingAnnotations: map[string]string{
				"other-annotation": "other-value",
			},
			expectAnnotationsSet: true,
		},
		{
			name: "Cleanup annotations already set correctly",
			existingAnnotations: map[string]string{
				"vmware.hcp.open-cluster-management.io/vm-name":              "test-vm",
				"vmware.hcp.open-cluster-management.io/datacenter":           "dc1",
				"vmware.hcp.open-cluster-management.io/folder":               "/vms",
				"vmware.hcp.open-cluster-management.io/credential-secret":    "vsphere-credentials",
				"vmware.hcp.open-cluster-management.io/credential-namespace": "test-namespace",
			},
			expectAnnotationsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := createFakeAgent("test-agent", "test-namespace", nil, "test-uuid", "")
			if tt.existingAnnotations != nil {
				agent.SetAnnotations(tt.existingAnnotations)
			}

			template := &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-template",
					Namespace: "test-namespace",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					NodePoolRef: &vmwarev1alpha1.NodePoolReference{
						Name: "test-nodepool",
					},
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{
						Datacenter: "dc1",
						Folder:     "/vms",
					},
				},
				Status: vmwarev1alpha1.VMwareNodePoolTemplateStatus{
					VMStatus: []vmwarev1alpha1.VMStatus{
						{
							Name: "test-vm",
							UUID: "test-uuid",
						},
					},
				},
			}

			s := runtime.NewScheme()
			_ = vmwarev1alpha1.AddToScheme(s)
			_ = scheme.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(template).
				WithRuntimeObjects(agent).
				Build()

			r := &VMwareNodePoolTemplateReconciler{
				Client:   fakeClient,
				Scheme:   s,
				Recorder: &fakeRecorder{},
			}

			ctx := context.Background()
			err := r.reconcileAgents(ctx, template, logr.Discard())
			if err != nil {
				t.Fatalf("reconcileAgents failed: %v", err)
			}

			// Verify annotations were set
			updatedAgent := &unstructured.Unstructured{}
			updatedAgent.SetGroupVersionKind(agentGVK)
			err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-agent", Namespace: "test-namespace"}, updatedAgent)
			if err != nil {
				t.Fatalf("Failed to get updated agent: %v", err)
			}

			annotations := updatedAgent.GetAnnotations()
			if tt.expectAnnotationsSet {
				if annotations == nil {
					t.Fatal("Expected annotations to be set, but got nil")
				}

				requiredAnnotations := []string{
					"vmware.hcp.open-cluster-management.io/vm-name",
					"vmware.hcp.open-cluster-management.io/datacenter",
					"vmware.hcp.open-cluster-management.io/credential-secret",
					"vmware.hcp.open-cluster-management.io/credential-namespace",
				}

				for _, key := range requiredAnnotations {
					if _, found := annotations[key]; !found {
						t.Errorf("Required annotation %s not found", key)
					}
				}
			}
		})
	}
}

// TestMultipleAgentsWithSameUUID tests handling of duplicate UUIDs (edge case)
func TestMultipleAgentsWithSameUUID(t *testing.T) {
	agent1 := createFakeAgent("agent-1", "test-namespace", nil, "duplicate-uuid", "")
	agent2 := createFakeAgent("agent-2", "test-namespace", nil, "duplicate-uuid", "")

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
		Status: vmwarev1alpha1.VMwareNodePoolTemplateStatus{
			VMStatus: []vmwarev1alpha1.VMStatus{
				{
					Name: "test-vm",
					UUID: "duplicate-uuid",
				},
			},
		},
	}

	s := runtime.NewScheme()
	_ = vmwarev1alpha1.AddToScheme(s)
	_ = scheme.AddToScheme(s)

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(template).
		WithRuntimeObjects(agent1, agent2).
		Build()

	r := &VMwareNodePoolTemplateReconciler{
		Client:   fakeClient,
		Scheme:   s,
		Recorder: &fakeRecorder{},
	}

	ctx := context.Background()
	err := r.reconcileAgents(ctx, template, logr.Discard())
	if err != nil {
		t.Fatalf("reconcileAgents failed: %v", err)
	}

	// Both agents should be labeled (this is an edge case but should be handled gracefully)
	for _, agentName := range []string{"agent-1", "agent-2"} {
		updatedAgent := &unstructured.Unstructured{}
		updatedAgent.SetGroupVersionKind(agentGVK)
		err = fakeClient.Get(ctx, client.ObjectKey{Name: agentName, Namespace: "test-namespace"}, updatedAgent)
		if err != nil {
			t.Fatalf("Failed to get agent %s: %v", agentName, err)
		}

		labels := updatedAgent.GetLabels()
		if labels["vmware.hcp.open-cluster-management.io/managed-by"] != "test-template" {
			t.Errorf("Agent %s was not labeled correctly", agentName)
		}
	}
}

// TestNodePoolDeletedWhileTemplateExists tests cleanup when NodePool is deleted
func TestNodePoolDeletedWhileTemplateExists(t *testing.T) {
	// This is already covered in nodepool_test.go, but we include it here for completeness
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
				Name: "deleted-nodepool",
			},
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
	_, err := r.getNodePool(ctx, template, logr.Discard())

	// Should get NotFound error
	if err == nil {
		t.Error("Expected error when NodePool doesn't exist")
	}
}

// TestEmptyVMStatusList tests handling of empty VM status list
func TestEmptyVMStatusList(t *testing.T) {
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
		Status: vmwarev1alpha1.VMwareNodePoolTemplateStatus{
			VMStatus: []vmwarev1alpha1.VMStatus{}, // Empty list
		},
	}

	s := runtime.NewScheme()
	_ = vmwarev1alpha1.AddToScheme(s)
	_ = scheme.AddToScheme(s)

	// Create an agent that won't match any VM
	agent := createFakeAgent("orphaned-agent", "test-namespace", nil, "orphaned-uuid", "")

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(template).
		WithRuntimeObjects(agent).
		Build()

	r := &VMwareNodePoolTemplateReconciler{
		Client:   fakeClient,
		Scheme:   s,
		Recorder: &fakeRecorder{},
	}

	ctx := context.Background()
	err := r.reconcileAgents(ctx, template, logr.Discard())
	if err != nil {
		t.Fatalf("reconcileAgents should handle empty VM status gracefully: %v", err)
	}

	// Verify the orphaned agent was not labeled (no matching VM)
	updatedAgent := &unstructured.Unstructured{}
	updatedAgent.SetGroupVersionKind(agentGVK)
	err = fakeClient.Get(ctx, client.ObjectKey{Name: "orphaned-agent", Namespace: "test-namespace"}, updatedAgent)
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}

	labels := updatedAgent.GetLabels()
	if labels != nil && labels["vmware.hcp.open-cluster-management.io/managed-by"] != "" {
		t.Error("Orphaned agent should not be labeled when no VMs match")
	}
}

// TestAgentWithBothUUIDAndSerial tests matching priority (UUID vs Serial)
func TestAgentWithBothUUIDAndSerial(t *testing.T) {
	tests := []struct {
		name        string
		agentUUID   string
		agentSerial string
		vmUUID      string
		vmSerial    string
		expectMatch bool
	}{
		{
			name:        "Both UUID and Serial match",
			agentUUID:   "uuid-123",
			agentSerial: "serial-abc",
			vmUUID:      "uuid-123",
			vmSerial:    "serial-abc",
			expectMatch: true,
		},
		{
			name:        "UUID matches, Serial doesn't",
			agentUUID:   "uuid-123",
			agentSerial: "serial-xyz",
			vmUUID:      "uuid-123",
			vmSerial:    "serial-abc",
			expectMatch: true, // UUID match is sufficient
		},
		{
			name:        "Serial matches, UUID doesn't",
			agentUUID:   "uuid-456",
			agentSerial: "serial-abc",
			vmUUID:      "uuid-123",
			vmSerial:    "serial-abc",
			expectMatch: true, // Serial match is sufficient
		},
		{
			name:        "Neither matches",
			agentUUID:   "uuid-456",
			agentSerial: "serial-xyz",
			vmUUID:      "uuid-123",
			vmSerial:    "serial-abc",
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := createFakeAgent("test-agent", "test-namespace", nil, tt.agentUUID, tt.agentSerial)

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
				Status: vmwarev1alpha1.VMwareNodePoolTemplateStatus{
					VMStatus: []vmwarev1alpha1.VMStatus{
						{
							Name:         "test-vm",
							UUID:         tt.vmUUID,
							SerialNumber: tt.vmSerial,
						},
					},
				},
			}

			s := runtime.NewScheme()
			_ = vmwarev1alpha1.AddToScheme(s)
			_ = scheme.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(template).
				WithRuntimeObjects(agent).
				Build()

			r := &VMwareNodePoolTemplateReconciler{
				Client:   fakeClient,
				Scheme:   s,
				Recorder: &fakeRecorder{},
			}

			ctx := context.Background()
			err := r.reconcileAgents(ctx, template, logr.Discard())
			if err != nil {
				t.Fatalf("reconcileAgents failed: %v", err)
			}

			updatedAgent := &unstructured.Unstructured{}
			updatedAgent.SetGroupVersionKind(agentGVK)
			err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-agent", Namespace: "test-namespace"}, updatedAgent)
			if err != nil {
				t.Fatalf("Failed to get agent: %v", err)
			}

			labels := updatedAgent.GetLabels()
			hasManagementLabel := labels != nil && labels["vmware.hcp.open-cluster-management.io/managed-by"] == "test-template"

			if tt.expectMatch && !hasManagementLabel {
				t.Error("Expected agent to match and be labeled, but it wasn't")
			}
			if !tt.expectMatch && hasManagementLabel {
				t.Error("Expected agent NOT to match, but it was labeled")
			}
		})
	}
}

// TestNilNodePoolRefHandling tests proper handling of nil NodePoolRef
func TestNilNodePoolRefHandling(t *testing.T) {
	template := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template",
			Namespace: "test-namespace",
		},
		Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
			NodePoolRef: nil, // Nil reference
			TestMode:    false,
		},
	}

	s := runtime.NewScheme()
	_ = vmwarev1alpha1.AddToScheme(s)
	_ = scheme.AddToScheme(s)

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(template).
		Build()

	r := &VMwareNodePoolTemplateReconciler{
		Client: fakeClient,
		Scheme: s,
	}

	ctx := context.Background()

	// Should handle nil NodePoolRef gracefully
	_, err := r.getNodePoolReplicas(ctx, template, logr.Discard())
	if err == nil {
		t.Error("Expected error when NodePoolRef is nil and not in test mode")
	}
}
