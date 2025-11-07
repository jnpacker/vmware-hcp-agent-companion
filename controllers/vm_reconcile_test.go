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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
)

// TestGenerateVMName tests VM name generation
func TestGenerateVMName(t *testing.T) {
	tests := []struct {
		name         string
		template     *vmwarev1alpha1.VMwareNodePoolTemplate
		index        int32
		expectedName string
	}{
		{
			name: "Default name prefix from template name",
			template: &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-template",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{},
				},
			},
			index:        1,
			expectedName: "my-template-n01",
		},
		{
			name: "Custom name prefix",
			template: &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-template",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{
						NamePrefix: "custom-vm",
					},
				},
			},
			index:        5,
			expectedName: "custom-vm-n05",
		},
		{
			name: "Index zero",
			template: &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{},
				},
			},
			index:        0,
			expectedName: "test-n00",
		},
		{
			name: "Large index",
			template: &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{
						NamePrefix: "prod-node",
					},
				},
			},
			index:        999,
			expectedName: "prod-node-n999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &VMwareNodePoolTemplateReconciler{}
			vmName := r.generateVMName(tt.template, tt.index)

			if vmName != tt.expectedName {
				t.Errorf("Expected VM name %s, got %s", tt.expectedName, vmName)
			}
		})
	}
}

// TestGetVMNamePrefix tests VM name prefix extraction
func TestGetVMNamePrefix(t *testing.T) {
	tests := []struct {
		name           string
		template       *vmwarev1alpha1.VMwareNodePoolTemplate
		expectedPrefix string
	}{
		{
			name: "No custom prefix - use template name",
			template: &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-template",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{},
				},
			},
			expectedPrefix: "my-template",
		},
		{
			name: "Custom prefix specified",
			template: &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-template",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{
						NamePrefix: "custom-vm-prefix",
					},
				},
			},
			expectedPrefix: "custom-vm-prefix",
		},
		{
			name: "Empty custom prefix - use template name",
			template: &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fallback-name",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{
						NamePrefix: "",
					},
				},
			},
			expectedPrefix: "fallback-name",
		},
		{
			name: "Template with long name",
			template: &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "very-long-template-name-for-testing",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{},
				},
			},
			expectedPrefix: "very-long-template-name-for-testing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &VMwareNodePoolTemplateReconciler{}
			prefix := r.getVMNamePrefix(tt.template)

			if prefix != tt.expectedPrefix {
				t.Errorf("Expected prefix %s, got %s", tt.expectedPrefix, prefix)
			}
		})
	}
}

// TestVMNameConsistency tests that generateVMName and getVMNamePrefix work together correctly
func TestVMNameConsistency(t *testing.T) {
	template := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-template",
		},
		Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
			VMTemplate: vmwarev1alpha1.VMTemplateSpec{
				NamePrefix: "vm",
			},
		},
	}

	r := &VMwareNodePoolTemplateReconciler{}
	prefix := r.getVMNamePrefix(template)

	// Generate several VM names and verify they all start with the prefix
	for i := int32(0); i < 10; i++ {
		vmName := r.generateVMName(template, i)

		// Check that name starts with prefix
		if len(vmName) < len(prefix) || vmName[:len(prefix)] != prefix {
			t.Errorf("VM name %s does not start with prefix %s", vmName, prefix)
		}
	}
}

// TestVMNameUniqueness tests that different indices produce different names
func TestVMNameUniqueness(t *testing.T) {
	template := &vmwarev1alpha1.VMwareNodePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
			VMTemplate: vmwarev1alpha1.VMTemplateSpec{},
		},
	}

	r := &VMwareNodePoolTemplateReconciler{}
	names := make(map[string]bool)

	// Generate 100 VM names and verify they're all unique
	for i := int32(0); i < 100; i++ {
		vmName := r.generateVMName(template, i)

		if names[vmName] {
			t.Errorf("Duplicate VM name generated: %s", vmName)
		}
		names[vmName] = true
	}

	// Verify we got 100 unique names
	if len(names) != 100 {
		t.Errorf("Expected 100 unique names, got %d", len(names))
	}
}

// TestVMNameFormat tests that generated names follow expected format
func TestVMNameFormat(t *testing.T) {
	tests := []struct {
		name        string
		template    *vmwarev1alpha1.VMwareNodePoolTemplate
		index       int32
		formatCheck func(string) bool
		description string
	}{
		{
			name: "Contains n prefix for index",
			template: &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{},
				},
			},
			index: 1,
			formatCheck: func(name string) bool {
				// Should contain "-n" before the numeric index (e.g., "test-n01")
				// The format is: prefix-nXX where XX is zero-padded
				return len(name) >= 5 && name[len(name)-4] == '-' && name[len(name)-3] == 'n'
			},
			description: "Name should have -n prefix before index",
		},
		{
			name: "Padded index with two digits",
			template: &vmwarev1alpha1.VMwareNodePoolTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: vmwarev1alpha1.VMwareNodePoolTemplateSpec{
					VMTemplate: vmwarev1alpha1.VMTemplateSpec{},
				},
			},
			index: 5,
			formatCheck: func(name string) bool {
				// Index 5 should be formatted as "05"
				return name == "test-n05"
			},
			description: "Name should pad single digit index to 2 digits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &VMwareNodePoolTemplateReconciler{}
			vmName := r.generateVMName(tt.template, tt.index)

			if !tt.formatCheck(vmName) {
				t.Errorf("VM name %s failed format check: %s", vmName, tt.description)
			}
		})
	}
}
