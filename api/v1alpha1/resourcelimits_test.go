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
)

func TestVMQuotaValidation(t *testing.T) {
	tests := []struct {
		name        string
		quota       *VMQuota
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil VMQuota is valid",
			quota:       nil,
			expectError: false,
		},
		{
			name:        "Empty VMQuota is valid",
			quota:       &VMQuota{},
			expectError: false,
		},
		{
			name: "Valid MaxVms only",
			quota: &VMQuota{
				MaxVms: ptr(int32(50)),
			},
			expectError: false,
		},
		{
			name: "Valid SoftMaxVms only",
			quota: &VMQuota{
				SoftMaxVms: ptr(int32(40)),
			},
			expectError: false,
		},
		{
			name: "Valid MaxVms and SoftMaxVms (SoftMaxVms < MaxVms)",
			quota: &VMQuota{
				MaxVms:     ptr(int32(50)),
				SoftMaxVms: ptr(int32(40)),
			},
			expectError: false,
		},
		{
			name: "Valid MaxVms and SoftMaxVms (SoftMaxVms == MaxVms)",
			quota: &VMQuota{
				MaxVms:     ptr(int32(50)),
				SoftMaxVms: ptr(int32(50)),
			},
			expectError: false,
		},
		{
			name: "Invalid - SoftMaxVms > MaxVms",
			quota: &VMQuota{
				MaxVms:     ptr(int32(30)),
				SoftMaxVms: ptr(int32(40)),
			},
			expectError: true,
			errorMsg:    "softMaxVms (40) cannot exceed maxVms (30)",
		},
		{
			name: "Invalid - MaxVms is zero",
			quota: &VMQuota{
				MaxVms: ptr(int32(0)),
			},
			expectError: true,
			errorMsg:    "maxVms must be greater than 0",
		},
		{
			name: "Invalid - MaxVms is negative",
			quota: &VMQuota{
				MaxVms: ptr(int32(-10)),
			},
			expectError: true,
			errorMsg:    "maxVms must be greater than 0",
		},
		{
			name: "Invalid - SoftMaxVms is zero",
			quota: &VMQuota{
				SoftMaxVms: ptr(int32(0)),
			},
			expectError: true,
			errorMsg:    "softMaxVms must be greater than 0",
		},
		{
			name: "Invalid - SoftMaxVms is negative",
			quota: &VMQuota{
				SoftMaxVms: ptr(int32(-5)),
			},
			expectError: true,
			errorMsg:    "softMaxVms must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.quota.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestIsMaxVmsReached(t *testing.T) {
	tests := []struct {
		name        string
		quota       *VMQuota
		currentVMs  int32
		expectReached bool
	}{
		{
			name:          "Nil VMQuota - never reached",
			quota:         nil,
			currentVMs:    100,
			expectReached: false,
		},
		{
			name:          "No MaxVms set - never reached",
			quota:         &VMQuota{},
			currentVMs:    100,
			expectReached: false,
		},
		{
			name: "Current VMs below limit",
			quota: &VMQuota{
				MaxVms: ptr(int32(50)),
			},
			currentVMs:    30,
			expectReached: false,
		},
		{
			name: "Current VMs exactly at limit",
			quota: &VMQuota{
				MaxVms: ptr(int32(50)),
			},
			currentVMs:    50,
			expectReached: true,
		},
		{
			name: "Current VMs above limit",
			quota: &VMQuota{
				MaxVms: ptr(int32(50)),
			},
			currentVMs:    60,
			expectReached: true,
		},
		{
			name: "Zero current VMs",
			quota: &VMQuota{
				MaxVms: ptr(int32(50)),
			},
			currentVMs:    0,
			expectReached: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reached := tt.quota.IsMaxVmsReached(tt.currentVMs)
			if reached != tt.expectReached {
				t.Errorf("Expected IsMaxVMsReached to return %v, got %v", tt.expectReached, reached)
			}
		})
	}
}

// Helper function to create pointer to int32
func ptr(i int32) *int32 {
	return &i
}
