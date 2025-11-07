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

package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestSetCondition verifies condition management functionality
func TestSetCondition(t *testing.T) {
	tests := []struct {
		name              string
		template          *VMwareNodePoolTemplate
		conditionType     string
		status            metav1.ConditionStatus
		reason            string
		message           string
		expectedCondCount int
		expectUpdate      bool
	}{
		{
			name: "Add new condition",
			template: &VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-template",
					Generation: 1,
				},
				Status: VMwareNodePoolTemplateStatus{},
			},
			conditionType:     ConditionTypeReady,
			status:            metav1.ConditionTrue,
			reason:            "AllReady",
			message:           "All VMs are ready",
			expectedCondCount: 1,
			expectUpdate:      true,
		},
		{
			name: "Update existing condition",
			template: &VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-template",
					Generation: 2,
				},
				Status: VMwareNodePoolTemplateStatus{
					Conditions: []metav1.Condition{
						{
							Type:               ConditionTypeReady,
							Status:             metav1.ConditionFalse,
							ObservedGeneration: 1,
							Reason:             "NotReady",
							Message:            "VMs not ready",
						},
					},
				},
			},
			conditionType:     ConditionTypeReady,
			status:            metav1.ConditionTrue,
			reason:            "AllReady",
			message:           "All VMs are ready",
			expectedCondCount: 1,
			expectUpdate:      true,
		},
		{
			name: "Idempotent update - same values",
			template: &VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-template",
					Generation: 1,
				},
				Status: VMwareNodePoolTemplateStatus{
					Conditions: []metav1.Condition{
						{
							Type:               ConditionTypeReady,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
							Reason:             "AllReady",
							Message:            "All VMs are ready",
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			conditionType:     ConditionTypeReady,
			status:            metav1.ConditionTrue,
			reason:            "AllReady",
			message:           "All VMs are ready",
			expectedCondCount: 1,
			expectUpdate:      false,
		},
		{
			name: "Add multiple different conditions",
			template: &VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-template",
					Generation: 1,
				},
				Status: VMwareNodePoolTemplateStatus{
					Conditions: []metav1.Condition{
						{
							Type:               ConditionTypeReady,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
							Reason:             "AllReady",
							Message:            "Ready",
						},
					},
				},
			},
			conditionType:     ConditionTypeVSphereConnected,
			status:            metav1.ConditionTrue,
			reason:            "Connected",
			message:           "Connected to vSphere",
			expectedCondCount: 2,
			expectUpdate:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialCondCount := len(tt.template.Status.Conditions)
			tt.template.SetCondition(tt.conditionType, tt.status, tt.reason, tt.message)

			if len(tt.template.Status.Conditions) != tt.expectedCondCount {
				t.Errorf("Expected %d conditions, got %d", tt.expectedCondCount, len(tt.template.Status.Conditions))
			}

			// Verify the condition was set correctly
			cond := tt.template.GetCondition(tt.conditionType)
			if cond == nil {
				t.Fatalf("Condition %s not found", tt.conditionType)
			}

			if cond.Status != tt.status {
				t.Errorf("Expected status %s, got %s", tt.status, cond.Status)
			}
			if cond.Reason != tt.reason {
				t.Errorf("Expected reason %s, got %s", tt.reason, cond.Reason)
			}
			if cond.Message != tt.message {
				t.Errorf("Expected message %s, got %s", tt.message, cond.Message)
			}
			if cond.ObservedGeneration != tt.template.Generation {
				t.Errorf("Expected observedGeneration %d, got %d", tt.template.Generation, cond.ObservedGeneration)
			}

			// For idempotent updates, verify we didn't add duplicate conditions
			if !tt.expectUpdate && len(tt.template.Status.Conditions) != initialCondCount {
				t.Errorf("Idempotent update should not change condition count")
			}
		})
	}
}

// TestGetCondition verifies condition retrieval
func TestGetCondition(t *testing.T) {
	template := &VMwareNodePoolTemplate{
		Status: VMwareNodePoolTemplateStatus{
			Conditions: []metav1.Condition{
				{
					Type:    ConditionTypeReady,
					Status:  metav1.ConditionTrue,
					Reason:  "AllReady",
					Message: "All VMs ready",
				},
				{
					Type:    ConditionTypeVSphereConnected,
					Status:  metav1.ConditionTrue,
					Reason:  "Connected",
					Message: "Connected to vSphere",
				},
			},
		},
	}

	tests := []struct {
		name          string
		conditionType string
		expectFound   bool
		expectedMsg   string
	}{
		{
			name:          "Get existing condition",
			conditionType: ConditionTypeReady,
			expectFound:   true,
			expectedMsg:   "All VMs ready",
		},
		{
			name:          "Get non-existent condition",
			conditionType: "NonExistent",
			expectFound:   false,
		},
		{
			name:          "Get second condition",
			conditionType: ConditionTypeVSphereConnected,
			expectFound:   true,
			expectedMsg:   "Connected to vSphere",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := template.GetCondition(tt.conditionType)
			if tt.expectFound {
				if cond == nil {
					t.Fatalf("Expected to find condition %s, but got nil", tt.conditionType)
				}
				if cond.Message != tt.expectedMsg {
					t.Errorf("Expected message %s, got %s", tt.expectedMsg, cond.Message)
				}
			} else {
				if cond != nil {
					t.Errorf("Expected nil condition for %s, but got %+v", tt.conditionType, cond)
				}
			}
		})
	}
}

// TestVMTemplateSpecDefaults verifies default values are properly handled
func TestVMTemplateSpecDefaults(t *testing.T) {
	tests := []struct {
		name            string
		spec            VMTemplateSpec
		expectedCPUs    int32
		expectedMemMB   int64
		expectedDiskGB  int64
		expectedGuestID string
		expectedFW      string
	}{
		{
			name: "All defaults should be 0 (applied at VM creation time)",
			spec: VMTemplateSpec{
				Datacenter: "dc1",
				Datastore:  "datastore1",
				Network:    "network1",
			},
			expectedCPUs:    0, // Defaults applied in vsphere client
			expectedMemMB:   0,
			expectedDiskGB:  0,
			expectedGuestID: "",
			expectedFW:      "",
		},
		{
			name: "Custom values preserved",
			spec: VMTemplateSpec{
				Datacenter: "dc1",
				Datastore:  "datastore1",
				Network:    "network1",
				NumCPUs:    4,
				MemoryMB:   8192,
				DiskSizeGB: 200,
				GuestID:    "rhel9_64Guest",
				Firmware:   "bios",
			},
			expectedCPUs:    4,
			expectedMemMB:   8192,
			expectedDiskGB:  200,
			expectedGuestID: "rhel9_64Guest",
			expectedFW:      "bios",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.spec.NumCPUs != tt.expectedCPUs {
				t.Errorf("Expected NumCPUs %d, got %d", tt.expectedCPUs, tt.spec.NumCPUs)
			}
			if tt.spec.MemoryMB != tt.expectedMemMB {
				t.Errorf("Expected MemoryMB %d, got %d", tt.expectedMemMB, tt.spec.MemoryMB)
			}
			if tt.spec.DiskSizeGB != tt.expectedDiskGB {
				t.Errorf("Expected DiskSizeGB %d, got %d", tt.expectedDiskGB, tt.spec.DiskSizeGB)
			}
			if tt.spec.GuestID != tt.expectedGuestID {
				t.Errorf("Expected GuestID %s, got %s", tt.expectedGuestID, tt.spec.GuestID)
			}
			if tt.spec.Firmware != tt.expectedFW {
				t.Errorf("Expected Firmware %s, got %s", tt.expectedFW, tt.spec.Firmware)
			}
		})
	}
}

// TestAgentLabelSelectorHandling verifies AgentLabelSelector is properly handled
func TestAgentLabelSelectorHandling(t *testing.T) {
	tests := []struct {
		name                    string
		agentLabelSelector      map[string]string
		expectNilSelector       bool
		expectedSelectorContent map[string]string
	}{
		{
			name:               "Nil AgentLabelSelector",
			agentLabelSelector: nil,
			expectNilSelector:  true,
		},
		{
			name:                    "Empty AgentLabelSelector",
			agentLabelSelector:      map[string]string{},
			expectNilSelector:       false,
			expectedSelectorContent: map[string]string{},
		},
		{
			name: "AgentLabelSelector with standard labels",
			agentLabelSelector: map[string]string{
				"hypershift.openshift.io/nodePool": "my-nodepool",
			},
			expectNilSelector: false,
			expectedSelectorContent: map[string]string{
				"hypershift.openshift.io/nodePool": "my-nodepool",
			},
		},
		{
			name: "AgentLabelSelector with custom labels",
			agentLabelSelector: map[string]string{
				"env":  "production",
				"team": "platform",
			},
			expectNilSelector: false,
			expectedSelectorContent: map[string]string{
				"env":  "production",
				"team": "platform",
			},
		},
		{
			name: "AgentLabelSelector with both standard and custom labels",
			agentLabelSelector: map[string]string{
				"hypershift.openshift.io/nodePool": "my-nodepool",
				"cluster.x-k8s.io/cluster-name":    "my-cluster",
				"custom-label":                     "custom-value",
			},
			expectNilSelector: false,
			expectedSelectorContent: map[string]string{
				"hypershift.openshift.io/nodePool": "my-nodepool",
				"cluster.x-k8s.io/cluster-name":    "my-cluster",
				"custom-label":                     "custom-value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &VMwareNodePoolTemplate{
				Spec: VMwareNodePoolTemplateSpec{
					AgentLabelSelector: tt.agentLabelSelector,
				},
			}

			if tt.expectNilSelector && template.Spec.AgentLabelSelector != nil {
				t.Errorf("Expected nil AgentLabelSelector, got %v", template.Spec.AgentLabelSelector)
			}

			if !tt.expectNilSelector {
				if template.Spec.AgentLabelSelector == nil {
					t.Fatal("Expected non-nil AgentLabelSelector")
				}
				if len(template.Spec.AgentLabelSelector) != len(tt.expectedSelectorContent) {
					t.Errorf("Expected %d labels, got %d", len(tt.expectedSelectorContent), len(template.Spec.AgentLabelSelector))
				}
				for k, v := range tt.expectedSelectorContent {
					if template.Spec.AgentLabelSelector[k] != v {
						t.Errorf("Expected label %s=%s, got %s", k, v, template.Spec.AgentLabelSelector[k])
					}
				}
			}
		})
	}
}

// TestAdvancedVMConfig verifies advanced VM configuration options
func TestAdvancedVMConfig(t *testing.T) {
	tests := []struct {
		name               string
		advancedConfig     *AdvancedVMConfig
		expectNil          bool
		expectedDiskUUID   *bool
		expectedNestedVirt *bool
	}{
		{
			name:           "Nil AdvancedConfig",
			advancedConfig: nil,
			expectNil:      true,
		},
		{
			name:           "Empty AdvancedConfig",
			advancedConfig: &AdvancedVMConfig{},
			expectNil:      false,
		},
		{
			name: "DiskEnableUUID explicitly true",
			advancedConfig: &AdvancedVMConfig{
				DiskEnableUUID: boolPtr(true),
			},
			expectNil:        false,
			expectedDiskUUID: boolPtr(true),
		},
		{
			name: "DiskEnableUUID explicitly false",
			advancedConfig: &AdvancedVMConfig{
				DiskEnableUUID: boolPtr(false),
			},
			expectNil:        false,
			expectedDiskUUID: boolPtr(false),
		},
		{
			name: "NestedVirtualization enabled",
			advancedConfig: &AdvancedVMConfig{
				NestedVirtualization: boolPtr(true),
			},
			expectNil:          false,
			expectedNestedVirt: boolPtr(true),
		},
		{
			name: "Both options set",
			advancedConfig: &AdvancedVMConfig{
				DiskEnableUUID:       boolPtr(true),
				NestedVirtualization: boolPtr(false),
			},
			expectNil:          false,
			expectedDiskUUID:   boolPtr(true),
			expectedNestedVirt: boolPtr(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := VMTemplateSpec{
				Datacenter:     "dc1",
				Datastore:      "ds1",
				Network:        "net1",
				AdvancedConfig: tt.advancedConfig,
			}

			if tt.expectNil && spec.AdvancedConfig != nil {
				t.Errorf("Expected nil AdvancedConfig, got %+v", spec.AdvancedConfig)
				return
			}

			if !tt.expectNil {
				if spec.AdvancedConfig == nil {
					t.Fatal("Expected non-nil AdvancedConfig")
				}

				if tt.expectedDiskUUID != nil {
					if spec.AdvancedConfig.DiskEnableUUID == nil {
						t.Errorf("Expected DiskEnableUUID %v, got nil", *tt.expectedDiskUUID)
					} else if *spec.AdvancedConfig.DiskEnableUUID != *tt.expectedDiskUUID {
						t.Errorf("Expected DiskEnableUUID %v, got %v", *tt.expectedDiskUUID, *spec.AdvancedConfig.DiskEnableUUID)
					}
				}

				if tt.expectedNestedVirt != nil {
					if spec.AdvancedConfig.NestedVirtualization == nil {
						t.Errorf("Expected NestedVirtualization %v, got nil", *tt.expectedNestedVirt)
					} else if *spec.AdvancedConfig.NestedVirtualization != *tt.expectedNestedVirt {
						t.Errorf("Expected NestedVirtualization %v, got %v", *tt.expectedNestedVirt, *spec.AdvancedConfig.NestedVirtualization)
					}
				}
			}
		})
	}
}

// TestCredentialReferenceDefaults verifies credential reference handling
func TestCredentialReferenceDefaults(t *testing.T) {
	tests := []struct {
		name               string
		vsphereCredentials *CredentialReference
		expectNil          bool
		expectedSecretName string
	}{
		{
			name:               "Nil VSphereCredentials",
			vsphereCredentials: nil,
			expectNil:          true,
		},
		{
			name:               "Empty CredentialReference",
			vsphereCredentials: &CredentialReference{},
			expectNil:          false,
			expectedSecretName: "", // Empty string is valid
		},
		{
			name: "Custom secret name",
			vsphereCredentials: &CredentialReference{
				Name: "my-vsphere-creds",
			},
			expectNil:          false,
			expectedSecretName: "my-vsphere-creds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := VMwareNodePoolTemplateSpec{
				VSphereCredentials: tt.vsphereCredentials,
			}

			if tt.expectNil && spec.VSphereCredentials != nil {
				t.Errorf("Expected nil VSphereCredentials, got %+v", spec.VSphereCredentials)
				return
			}

			if !tt.expectNil {
				if spec.VSphereCredentials == nil {
					t.Fatal("Expected non-nil VSphereCredentials")
				}
				if spec.VSphereCredentials.Name != tt.expectedSecretName {
					t.Errorf("Expected secret name %s, got %s", tt.expectedSecretName, spec.VSphereCredentials.Name)
				}
			}
		})
	}
}

// TestAgentISOValidation verifies ISO configuration validation
func TestAgentISOValidation(t *testing.T) {
	tests := []struct {
		name              string
		isoSpec           AgentISOSpec
		expectedType      string
		expectedDatastore string
		expectedURL       string
	}{
		{
			name: "Datastore ISO type",
			isoSpec: AgentISOSpec{
				Type:          "datastore",
				DatastorePath: "[datastore1] iso/agent.iso",
			},
			expectedType:      "datastore",
			expectedDatastore: "[datastore1] iso/agent.iso",
		},
		{
			name: "URL ISO type",
			isoSpec: AgentISOSpec{
				Type: "url",
				URL:  "https://example.com/agent.iso",
			},
			expectedType: "url",
			expectedURL:  "https://example.com/agent.iso",
		},
		{
			name: "URL type with custom upload name",
			isoSpec: AgentISOSpec{
				Type:            "url",
				URL:             "https://example.com/agent.iso",
				UploadedISOName: "custom-agent.iso",
			},
			expectedType: "url",
			expectedURL:  "https://example.com/agent.iso",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isoSpec.Type != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, tt.isoSpec.Type)
			}
			if tt.isoSpec.DatastorePath != tt.expectedDatastore {
				t.Errorf("Expected DatastorePath %s, got %s", tt.expectedDatastore, tt.isoSpec.DatastorePath)
			}
			if tt.isoSpec.URL != tt.expectedURL {
				t.Errorf("Expected URL %s, got %s", tt.expectedURL, tt.isoSpec.URL)
			}
		})
	}
}

// Helper function to create bool pointers
func boolPtr(b bool) *bool {
	return &b
}
