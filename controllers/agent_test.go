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

// TestAgentLabelingWithUserDefinedLabels tests that user-defined labels from AgentLabelSelector are applied correctly
func TestAgentLabelingWithUserDefinedLabels(t *testing.T) {
	tests := []struct {
		name                string
		agentLabelSelector  map[string]string
		existingAgentLabels map[string]string
		expectedLabels      map[string]string
		shouldUpdate        bool
	}{
		{
			name:                "Nil AgentLabelSelector - only management labels applied",
			agentLabelSelector:  nil,
			existingAgentLabels: map[string]string{},
			expectedLabels: map[string]string{
				"vmware.hcp.open-cluster-management.io/managed-by": "test-template",
				"vmware.hcp.open-cluster-management.io/nodepool":   "test-nodepool",
			},
			shouldUpdate: true,
		},
		{
			name:                "Empty AgentLabelSelector - only management labels applied",
			agentLabelSelector:  map[string]string{},
			existingAgentLabels: map[string]string{},
			expectedLabels: map[string]string{
				"vmware.hcp.open-cluster-management.io/managed-by": "test-template",
				"vmware.hcp.open-cluster-management.io/nodepool":   "test-nodepool",
			},
			shouldUpdate: true,
		},
		{
			name: "User-defined labels applied",
			agentLabelSelector: map[string]string{
				"hypershift.openshift.io/nodePool": "test-nodepool",
			},
			existingAgentLabels: map[string]string{},
			expectedLabels: map[string]string{
				"hypershift.openshift.io/nodePool":                 "test-nodepool",
				"vmware.hcp.open-cluster-management.io/managed-by": "test-template",
				"vmware.hcp.open-cluster-management.io/nodepool":   "test-nodepool",
			},
			shouldUpdate: true,
		},
		{
			name: "Multiple user-defined labels applied",
			agentLabelSelector: map[string]string{
				"hypershift.openshift.io/nodePool": "test-nodepool",
				"env":                              "production",
				"team":                             "platform",
			},
			existingAgentLabels: map[string]string{},
			expectedLabels: map[string]string{
				"hypershift.openshift.io/nodePool": "test-nodepool",
				"env":                              "production",
				"team":                             "platform",
				"vmware.hcp.open-cluster-management.io/managed-by": "test-template",
				"vmware.hcp.open-cluster-management.io/nodepool":   "test-nodepool",
			},
			shouldUpdate: true,
		},
		{
			name: "Existing labels preserved and merged",
			agentLabelSelector: map[string]string{
				"hypershift.openshift.io/nodePool": "test-nodepool",
			},
			existingAgentLabels: map[string]string{
				"existing-label": "existing-value",
			},
			expectedLabels: map[string]string{
				"hypershift.openshift.io/nodePool":                 "test-nodepool",
				"existing-label":                                   "existing-value",
				"vmware.hcp.open-cluster-management.io/managed-by": "test-template",
				"vmware.hcp.open-cluster-management.io/nodepool":   "test-nodepool",
			},
			shouldUpdate: true,
		},
		{
			name: "Labels already set - no update needed",
			agentLabelSelector: map[string]string{
				"hypershift.openshift.io/nodePool": "test-nodepool",
			},
			existingAgentLabels: map[string]string{
				"hypershift.openshift.io/nodePool":                 "test-nodepool",
				"vmware.hcp.open-cluster-management.io/managed-by": "test-template",
				"vmware.hcp.open-cluster-management.io/nodepool":   "test-nodepool",
			},
			expectedLabels: map[string]string{
				"hypershift.openshift.io/nodePool":                 "test-nodepool",
				"vmware.hcp.open-cluster-management.io/managed-by": "test-template",
				"vmware.hcp.open-cluster-management.io/nodepool":   "test-nodepool",
			},
			shouldUpdate: false,
		},
		{
			name: "User label value changed - update required",
			agentLabelSelector: map[string]string{
				"env": "production",
			},
			existingAgentLabels: map[string]string{
				"env": "staging",
				"vmware.hcp.open-cluster-management.io/managed-by": "test-template",
				"vmware.hcp.open-cluster-management.io/nodepool":   "test-nodepool",
			},
			expectedLabels: map[string]string{
				"env": "production",
				"vmware.hcp.open-cluster-management.io/managed-by": "test-template",
				"vmware.hcp.open-cluster-management.io/nodepool":   "test-nodepool",
			},
			shouldUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake agent with matching UUID
			agent := createFakeAgent("test-agent", "test-namespace", tt.existingAgentLabels, "test-uuid-123", "")

			// Create template with AgentLabelSelector
			template := &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-template",
					Namespace: "test-namespace",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					NodePoolRef: &vmwarev1alpha1.NodePoolReference{
						Name: "test-nodepool",
					},
					AgentLabelSelector: tt.agentLabelSelector,
				},
				Status: vmwarev1alpha1.VMwareNodePoolTemplateStatus{
					VMStatus: []vmwarev1alpha1.VMStatus{
						{
							Name: "test-vm",
							UUID: "test-uuid-123",
						},
					},
				},
			}

			// Create fake client
			s := runtime.NewScheme()
			_ = vmwarev1alpha1.AddToScheme(s)
			_ = scheme.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(template).
				WithRuntimeObjects(agent).
				Build()

			// Create reconciler
			r := &VMwareNodePoolTemplateReconciler{
				Client:   fakeClient,
				Scheme:   s,
				Recorder: &fakeRecorder{},
			}

			// Run reconcileAgents
			ctx := context.Background()
			err := r.reconcileAgents(ctx, template, logr.Discard())
			if err != nil {
				t.Fatalf("reconcileAgents failed: %v", err)
			}

			// Verify labels were applied correctly
			updatedAgent := &unstructured.Unstructured{}
			updatedAgent.SetGroupVersionKind(agentGVK)
			err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-agent", Namespace: "test-namespace"}, updatedAgent)
			if err != nil {
				t.Fatalf("Failed to get updated agent: %v", err)
			}

			actualLabels := updatedAgent.GetLabels()
			for key, expectedValue := range tt.expectedLabels {
				actualValue, found := actualLabels[key]
				if !found {
					t.Errorf("Expected label %s not found", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("Label %s: expected %s, got %s", key, expectedValue, actualValue)
				}
			}

			// Verify no extra labels were added
			if len(actualLabels) != len(tt.expectedLabels) {
				t.Errorf("Expected %d labels, got %d: %v", len(tt.expectedLabels), len(actualLabels), actualLabels)
			}
		})
	}
}

// TestAgentAnnotationsForVMCleanup tests that required annotations are added for VM cleanup
func TestAgentAnnotationsForVMCleanup(t *testing.T) {
	tests := []struct {
		name                string
		vmName              string
		datacenter          string
		folder              string
		credentialSecret    string
		expectedAnnotations map[string]string
	}{
		{
			name:             "Annotations added with all fields",
			vmName:           "test-vm-1",
			datacenter:       "dc1",
			folder:           "/vms/test",
			credentialSecret: "vsphere-credentials",
			expectedAnnotations: map[string]string{
				"vmware.hcp.open-cluster-management.io/vm-name":              "test-vm-1",
				"vmware.hcp.open-cluster-management.io/datacenter":           "dc1",
				"vmware.hcp.open-cluster-management.io/folder":               "/vms/test",
				"vmware.hcp.open-cluster-management.io/credential-secret":    "vsphere-credentials",
				"vmware.hcp.open-cluster-management.io/credential-namespace": "test-namespace",
			},
		},
		{
			name:             "Annotations with default credential secret",
			vmName:           "test-vm-2",
			datacenter:       "dc2",
			folder:           "",
			credentialSecret: "vsphere-credentials",
			expectedAnnotations: map[string]string{
				"vmware.hcp.open-cluster-management.io/vm-name":              "test-vm-2",
				"vmware.hcp.open-cluster-management.io/datacenter":           "dc2",
				"vmware.hcp.open-cluster-management.io/folder":               "",
				"vmware.hcp.open-cluster-management.io/credential-secret":    "vsphere-credentials",
				"vmware.hcp.open-cluster-management.io/credential-namespace": "test-namespace",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := createFakeAgent("test-agent", "test-namespace", nil, "test-uuid", "")

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
						Datacenter: tt.datacenter,
						Folder:     tt.folder,
					},
					VSphereCredentials: &vmwarev1alpha1.CredentialReference{
						Name: tt.credentialSecret,
					},
				},
				Status: vmwarev1alpha1.VMwareNodePoolTemplateStatus{
					VMStatus: []vmwarev1alpha1.VMStatus{
						{
							Name: tt.vmName,
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

			// Verify annotations
			updatedAgent := &unstructured.Unstructured{}
			updatedAgent.SetGroupVersionKind(agentGVK)
			err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-agent", Namespace: "test-namespace"}, updatedAgent)
			if err != nil {
				t.Fatalf("Failed to get updated agent: %v", err)
			}

			actualAnnotations := updatedAgent.GetAnnotations()
			for key, expectedValue := range tt.expectedAnnotations {
				actualValue, found := actualAnnotations[key]
				if !found {
					t.Errorf("Expected annotation %s not found", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("Annotation %s: expected %s, got %s", key, expectedValue, actualValue)
				}
			}
		})
	}
}

// TestAgentHostnameAndApproval tests hostname and approval settings
func TestAgentHostnameAndApproval(t *testing.T) {
	tests := []struct {
		name             string
		vmName           string
		existingHostname string
		existingApproval bool
		expectUpdate     bool
	}{
		{
			name:             "Hostname and approval not set",
			vmName:           "test-vm-1",
			existingHostname: "",
			existingApproval: false,
			expectUpdate:     true,
		},
		{
			name:             "Hostname set, approval needed",
			vmName:           "test-vm-1",
			existingHostname: "test-vm-1",
			existingApproval: false,
			expectUpdate:     true,
		},
		{
			name:             "Both already set correctly",
			vmName:           "test-vm-1",
			existingHostname: "test-vm-1",
			existingApproval: true,
			expectUpdate:     false,
		},
		{
			name:             "Hostname needs update",
			vmName:           "test-vm-2",
			existingHostname: "wrong-hostname",
			existingApproval: true,
			expectUpdate:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := createFakeAgent("test-agent", "test-namespace", nil, "test-uuid", "")

			// Set existing hostname and approval
			if tt.existingHostname != "" {
				_ = unstructured.SetNestedField(agent.Object, tt.existingHostname, "spec", "hostname")
			}
			if tt.existingApproval {
				_ = unstructured.SetNestedField(agent.Object, true, "spec", "approved")
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
				},
				Status: vmwarev1alpha1.VMwareNodePoolTemplateStatus{
					VMStatus: []vmwarev1alpha1.VMStatus{
						{
							Name: tt.vmName,
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

			// Verify hostname and approval
			updatedAgent := &unstructured.Unstructured{}
			updatedAgent.SetGroupVersionKind(agentGVK)
			err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-agent", Namespace: "test-namespace"}, updatedAgent)
			if err != nil {
				t.Fatalf("Failed to get updated agent: %v", err)
			}

			actualHostname, _, _ := unstructured.NestedString(updatedAgent.Object, "spec", "hostname")
			if actualHostname != tt.vmName {
				t.Errorf("Expected hostname %s, got %s", tt.vmName, actualHostname)
			}

			actualApproval, _, _ := unstructured.NestedBool(updatedAgent.Object, "spec", "approved")
			if !actualApproval {
				t.Error("Expected agent to be approved")
			}
		})
	}
}

// TestAgentFinalizerHandling tests finalizer addition
func TestAgentFinalizerHandling(t *testing.T) {
	tests := []struct {
		name               string
		existingFinalizers []string
		expectFinalizer    bool
	}{
		{
			name:               "No existing finalizers",
			existingFinalizers: nil,
			expectFinalizer:    true,
		},
		{
			name:               "Other finalizers present",
			existingFinalizers: []string{"other-finalizer"},
			expectFinalizer:    true,
		},
		{
			name:               "Our finalizer already present",
			existingFinalizers: []string{agentFinalizerName},
			expectFinalizer:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := createFakeAgent("test-agent", "test-namespace", nil, "test-uuid", "")
			if tt.existingFinalizers != nil {
				agent.SetFinalizers(tt.existingFinalizers)
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

			// Verify finalizer
			updatedAgent := &unstructured.Unstructured{}
			updatedAgent.SetGroupVersionKind(agentGVK)
			err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-agent", Namespace: "test-namespace"}, updatedAgent)
			if err != nil {
				t.Fatalf("Failed to get updated agent: %v", err)
			}

			finalizers := updatedAgent.GetFinalizers()
			hasFinalizer := false
			for _, f := range finalizers {
				if f == agentFinalizerName {
					hasFinalizer = true
					break
				}
			}

			if tt.expectFinalizer && !hasFinalizer {
				t.Errorf("Expected finalizer %s to be present, but it wasn't", agentFinalizerName)
			}
		})
	}
}

// TestAgentMatchingByUUIDAndSerial tests agent matching logic
func TestAgentMatchingByUUIDAndSerial(t *testing.T) {
	tests := []struct {
		name        string
		agentUUID   string
		agentSerial string
		vmUUID      string
		vmSerial    string
		expectMatch bool
	}{
		{
			name:        "Match by UUID",
			agentUUID:   "uuid-123",
			agentSerial: "serial-abc",
			vmUUID:      "uuid-123",
			vmSerial:    "",
			expectMatch: true,
		},
		{
			name:        "Match by Serial",
			agentUUID:   "",
			agentSerial: "serial-abc",
			vmUUID:      "",
			vmSerial:    "serial-abc",
			expectMatch: true,
		},
		{
			name:        "Match by both UUID and Serial",
			agentUUID:   "uuid-123",
			agentSerial: "serial-abc",
			vmUUID:      "uuid-123",
			vmSerial:    "serial-abc",
			expectMatch: true,
		},
		{
			name:        "No match - different UUID",
			agentUUID:   "uuid-456",
			agentSerial: "",
			vmUUID:      "uuid-123",
			vmSerial:    "",
			expectMatch: false,
		},
		{
			name:        "No match - different Serial",
			agentUUID:   "",
			agentSerial: "serial-xyz",
			vmUUID:      "",
			vmSerial:    "serial-abc",
			expectMatch: false,
		},
		{
			name:        "No match - empty identifiers",
			agentUUID:   "",
			agentSerial: "",
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

			// Verify agent was updated only if it should match
			updatedAgent := &unstructured.Unstructured{}
			updatedAgent.SetGroupVersionKind(agentGVK)
			err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-agent", Namespace: "test-namespace"}, updatedAgent)
			if err != nil {
				t.Fatalf("Failed to get updated agent: %v", err)
			}

			labels := updatedAgent.GetLabels()
			hasManagementLabel := labels != nil && labels["vmware.hcp.open-cluster-management.io/managed-by"] == "test-template"

			if tt.expectMatch && !hasManagementLabel {
				t.Error("Expected agent to be labeled as matching, but it wasn't")
			}
			if !tt.expectMatch && hasManagementLabel {
				t.Error("Expected agent NOT to be labeled as matching, but it was")
			}
		})
	}
}

// Helper function to create fake Agent
func createFakeAgent(name, namespace string, labels map[string]string, uuid, serial string) *unstructured.Unstructured {
	agent := &unstructured.Unstructured{}
	agent.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "agent-install.openshift.io",
		Version: "v1beta1",
		Kind:    "Agent",
	})
	agent.SetName(name)
	agent.SetNamespace(namespace)

	if labels != nil {
		agent.SetLabels(labels)
	}

	// Set UUID in status
	if uuid != "" {
		_ = unstructured.SetNestedField(agent.Object, uuid, "status", "inventory", "systemVendor", "uuid")
	}

	// Set Serial Number in status
	if serial != "" {
		_ = unstructured.SetNestedField(agent.Object, serial, "status", "inventory", "systemVendor", "serialNumber")
	}

	return agent
}

// Fake event recorder for testing
type fakeRecorder struct{}

func (f *fakeRecorder) Event(object runtime.Object, eventtype, reason, message string) {}
func (f *fakeRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
}
func (f *fakeRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
}
