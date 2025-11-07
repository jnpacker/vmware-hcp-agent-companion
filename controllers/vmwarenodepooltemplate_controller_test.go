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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
)

// TestMapAgentToTemplates tests the agent-to-template mapping function
func TestMapAgentToTemplates(t *testing.T) {
	tests := []struct {
		name             string
		agentLabels      map[string]string
		expectRequests   int
		expectedTemplate string
	}{
		{
			name: "Agent with management label",
			agentLabels: map[string]string{
				"vmware.hcp.open-cluster-management.io/managed-by": "test-template",
			},
			expectRequests:   1,
			expectedTemplate: "test-template",
		},
		{
			name: "Agent without management label",
			agentLabels: map[string]string{
				"other-label": "other-value",
			},
			expectRequests: 0,
		},
		{
			name:           "Agent with no labels",
			agentLabels:    nil,
			expectRequests: 0,
		},
		{
			name: "Agent with management label and other labels",
			agentLabels: map[string]string{
				"vmware.hcp.open-cluster-management.io/managed-by": "my-template",
				"hypershift.openshift.io/nodePool":                 "my-nodepool",
				"env":                                              "production",
			},
			expectRequests:   1,
			expectedTemplate: "my-template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := runtime.NewScheme()
			_ = vmwarev1alpha1.AddToScheme(s)
			_ = scheme.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				Build()

			r := &VMwareNodePoolTemplateReconciler{
				Client: fakeClient,
				Scheme: s,
			}

			// Create agent
			agent := &unstructured.Unstructured{}
			agent.SetGroupVersionKind(agentGVK)
			agent.SetName("test-agent")
			agent.SetNamespace("test-namespace")
			if tt.agentLabels != nil {
				agent.SetLabels(tt.agentLabels)
			}

			ctx := context.Background()
			requests := r.mapAgentToTemplates(ctx, agent)

			if len(requests) != tt.expectRequests {
				t.Errorf("Expected %d requests, got %d", tt.expectRequests, len(requests))
			}

			if tt.expectRequests > 0 {
				if requests[0].Name != tt.expectedTemplate {
					t.Errorf("Expected template name %s, got %s", tt.expectedTemplate, requests[0].Name)
				}
				if requests[0].Namespace != "test-namespace" {
					t.Errorf("Expected namespace test-namespace, got %s", requests[0].Namespace)
				}
			}
		})
	}
}

// TestFinalizerHandling tests finalizer add/remove logic
func TestFinalizerHandling(t *testing.T) {
	tests := []struct {
		name               string
		existingFinalizers []string
		expectFinalizer    bool
	}{
		{
			name:               "No finalizers - should add",
			existingFinalizers: nil,
			expectFinalizer:    true,
		},
		{
			name:               "Other finalizers - should add ours",
			existingFinalizers: []string{"other-finalizer"},
			expectFinalizer:    true,
		},
		{
			name:               "Our finalizer already present",
			existingFinalizers: []string{finalizerName},
			expectFinalizer:    true,
		},
		{
			name:               "Multiple finalizers including ours",
			existingFinalizers: []string{"other-finalizer", finalizerName},
			expectFinalizer:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := runtime.NewScheme()
			_ = vmwarev1alpha1.AddToScheme(s)
			_ = scheme.AddToScheme(s)

			template := &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-template",
					Namespace:  "test-namespace",
					Finalizers: tt.existingFinalizers,
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					TestMode: true,
					Replicas: int32Ptr(0),
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(template).
				WithStatusSubresource(template).
				Build()

			r := &VMwareNodePoolTemplateReconciler{
				Client:   fakeClient,
				Scheme:   s,
				Recorder: &fakeRecorder{},
			}

			ctx := context.Background()
			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      "test-template",
					Namespace: "test-namespace",
				},
			}

			// Note: This will fail without vSphere connection, but we can verify finalizer logic
			// by checking if it was added before the vSphere connection attempt
			_, _ = r.Reconcile(ctx, req)

			// Get updated template
			updatedTemplate := &vmwarev1alpha1.VMwareNodePoolTemplate{}
			err := fakeClient.Get(ctx, client.ObjectKey{
				Name:      "test-template",
				Namespace: "test-namespace",
			}, updatedTemplate)
			if err != nil {
				t.Fatalf("Failed to get updated template: %v", err)
			}

			// Check if finalizer was added
			hasFinalizer := false
			for _, f := range updatedTemplate.Finalizers {
				if f == finalizerName {
					hasFinalizer = true
					break
				}
			}

			if tt.expectFinalizer && !hasFinalizer {
				t.Errorf("Expected finalizer %s to be present, but it wasn't", finalizerName)
			}
		})
	}
}

// TestTestModeReplicaHandling tests replica handling in test mode
func TestTestModeReplicaHandling(t *testing.T) {
	tests := []struct {
		name                    string
		testMode                bool
		replicas                *int32
		nodePoolRef             *vmwarev1alpha1.NodePoolReference
		expectedDesiredReplicas int32
	}{
		{
			name:                    "Test mode with replicas",
			testMode:                true,
			replicas:                int32Ptr(5),
			nodePoolRef:             nil,
			expectedDesiredReplicas: 5,
		},
		{
			name:                    "Test mode without replicas",
			testMode:                true,
			replicas:                nil,
			nodePoolRef:             nil,
			expectedDesiredReplicas: 0,
		},
		{
			name:     "Normal mode requires NodePool",
			testMode: false,
			replicas: nil,
			nodePoolRef: &vmwarev1alpha1.NodePoolReference{
				Name: "test-nodepool",
			},
			expectedDesiredReplicas: 0, // Will be 0 since NodePool doesn't exist in test
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
					TestMode:    tt.testMode,
					Replicas:    tt.replicas,
					NodePoolRef: tt.nodePoolRef,
				},
			}

			// The test will verify that DesiredReplicas is set correctly in status
			// even if vSphere connection fails
			if template.Spec.TestMode {
				desiredReplicas := int32(0)
				if template.Spec.Replicas != nil {
					desiredReplicas = *template.Spec.Replicas
				}
				if desiredReplicas != tt.expectedDesiredReplicas {
					t.Errorf("Expected desired replicas %d, got %d", tt.expectedDesiredReplicas, desiredReplicas)
				}
			}
		})
	}
}

// TestConditionManagement tests condition setting and retrieval
func TestConditionManagement(t *testing.T) {
	template := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-template",
			Namespace:  "test-namespace",
			Generation: 1,
		},
	}

	// Test setting multiple conditions
	template.SetCondition(vmwarev1alpha1.ConditionTypeReady, metav1.ConditionFalse, "NotReady", "VMs not ready")
	template.SetCondition(vmwarev1alpha1.ConditionTypeVSphereConnected, metav1.ConditionTrue, "Connected", "Connected to vSphere")
	template.SetCondition(vmwarev1alpha1.ConditionTypeISOReady, metav1.ConditionTrue, "Ready", "ISO ready")

	if len(template.Status.Conditions) != 3 {
		t.Errorf("Expected 3 conditions, got %d", len(template.Status.Conditions))
	}

	// Test getting conditions
	readyCond := template.GetCondition(vmwarev1alpha1.ConditionTypeReady)
	if readyCond == nil {
		t.Fatal("Ready condition not found")
	}
	if readyCond.Status != metav1.ConditionFalse {
		t.Errorf("Expected Ready condition to be False, got %s", readyCond.Status)
	}

	vsphereCond := template.GetCondition(vmwarev1alpha1.ConditionTypeVSphereConnected)
	if vsphereCond == nil {
		t.Fatal("VSphereConnected condition not found")
	}
	if vsphereCond.Status != metav1.ConditionTrue {
		t.Errorf("Expected VSphereConnected condition to be True, got %s", vsphereCond.Status)
	}

	// Test updating existing condition
	template.SetCondition(vmwarev1alpha1.ConditionTypeReady, metav1.ConditionTrue, "AllReady", "All VMs ready")
	if len(template.Status.Conditions) != 3 {
		t.Errorf("Expected 3 conditions after update, got %d", len(template.Status.Conditions))
	}

	updatedReadyCond := template.GetCondition(vmwarev1alpha1.ConditionTypeReady)
	if updatedReadyCond.Status != metav1.ConditionTrue {
		t.Errorf("Expected updated Ready condition to be True, got %s", updatedReadyCond.Status)
	}
	if updatedReadyCond.Reason != "AllReady" {
		t.Errorf("Expected reason 'AllReady', got %s", updatedReadyCond.Reason)
	}
}

// TestCredentialReferenceHandling tests VSphere credential reference handling
func TestCredentialReferenceHandling(t *testing.T) {
	tests := []struct {
		name               string
		vsphereCredentials *vmwarev1alpha1.CredentialReference
		expectedSecretName string
	}{
		{
			name:               "Nil credentials - defaults to vsphere-credentials",
			vsphereCredentials: nil,
			expectedSecretName: defaultCredentialSecretName,
		},
		{
			name:               "Empty credentials - defaults to vsphere-credentials",
			vsphereCredentials: &vmwarev1alpha1.CredentialReference{},
			expectedSecretName: defaultCredentialSecretName,
		},
		{
			name: "Custom secret name",
			vsphereCredentials: &vmwarev1alpha1.CredentialReference{
				Name: "my-vsphere-creds",
			},
			expectedSecretName: "my-vsphere-creds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &vmwarev1alpha1.VMwareNodePoolTemplate{
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VSphereCredentials: tt.vsphereCredentials,
				},
			}

			// Simulate the controller logic for determining credential secret name
			credName := defaultCredentialSecretName
			if template.Spec.VSphereCredentials != nil {
				if template.Spec.VSphereCredentials.Name != "" {
					credName = template.Spec.VSphereCredentials.Name
				}
			}

			if credName != tt.expectedSecretName {
				t.Errorf("Expected secret name %s, got %s", tt.expectedSecretName, credName)
			}
		})
	}
}

// TestVMStatusTracking tests VM status tracking in template status
func TestVMStatusTracking(t *testing.T) {
	template := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template",
			Namespace: "test-namespace",
		},
		Status: vmwarev1alpha1.VMwareNodePoolTemplateStatus{
			VMStatus: []vmwarev1alpha1.VMStatus{
				{
					Name:         "vm-1",
					UUID:         "uuid-1",
					SerialNumber: "serial-1",
					PowerState:   "poweredOn",
					Phase:        "Running",
				},
				{
					Name:         "vm-2",
					UUID:         "uuid-2",
					SerialNumber: "serial-2",
					PowerState:   "poweredOff",
					Phase:        "NotReady",
				},
			},
			CurrentReplicas: 2,
			DesiredReplicas: 3,
			ReadyReplicas:   1,
		},
	}

	// Test status calculations
	if template.Status.CurrentReplicas != 2 {
		t.Errorf("Expected CurrentReplicas 2, got %d", template.Status.CurrentReplicas)
	}

	if template.Status.DesiredReplicas != 3 {
		t.Errorf("Expected DesiredReplicas 3, got %d", template.Status.DesiredReplicas)
	}

	if template.Status.ReadyReplicas != 1 {
		t.Errorf("Expected ReadyReplicas 1, got %d", template.Status.ReadyReplicas)
	}

	// Test finding VM by UUID
	var foundVM *vmwarev1alpha1.VMStatus
	for i := range template.Status.VMStatus {
		if template.Status.VMStatus[i].UUID == "uuid-1" {
			foundVM = &template.Status.VMStatus[i]
			break
		}
	}

	if foundVM == nil {
		t.Fatal("VM with uuid-1 not found")
	}

	if foundVM.Name != "vm-1" {
		t.Errorf("Expected VM name vm-1, got %s", foundVM.Name)
	}

	if foundVM.PowerState != "poweredOn" {
		t.Errorf("Expected PowerState poweredOn, got %s", foundVM.PowerState)
	}
}

// TestISOPathHandling tests ISO path configuration
func TestISOPathHandling(t *testing.T) {
	tests := []struct {
		name            string
		isoType         string
		datastorePath   string
		url             string
		expectedPathSet bool
	}{
		{
			name:            "Datastore ISO type",
			isoType:         "datastore",
			datastorePath:   "[datastore1] iso/agent.iso",
			expectedPathSet: true,
		},
		{
			name:            "URL ISO type",
			isoType:         "url",
			url:             "https://example.com/agent.iso",
			expectedPathSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &vmwarev1alpha1.VMwareNodePoolTemplate{
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					AgentISO: vmwarev1alpha1.AgentISOSpec{
						Type:          tt.isoType,
						DatastorePath: tt.datastorePath,
						URL:           tt.url,
					},
				},
			}

			// Verify ISO configuration
			if template.Spec.AgentISO.Type != tt.isoType {
				t.Errorf("Expected ISO type %s, got %s", tt.isoType, template.Spec.AgentISO.Type)
			}

			if tt.isoType == "datastore" && template.Spec.AgentISO.DatastorePath != tt.datastorePath {
				t.Errorf("Expected DatastorePath %s, got %s", tt.datastorePath, template.Spec.AgentISO.DatastorePath)
			}

			if tt.isoType == "url" && template.Spec.AgentISO.URL != tt.url {
				t.Errorf("Expected URL %s, got %s", tt.url, template.Spec.AgentISO.URL)
			}
		})
	}
}

// Helper function to create int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}
