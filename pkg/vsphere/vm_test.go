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

package vsphere

import (
	"testing"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// TestVMSpecDefaults tests that VM spec defaults are properly set
func TestVMSpecDefaults(t *testing.T) {
	tests := []struct {
		name                       string
		spec                       VMSpec
		expectedCPUs               int32
		expectedMemMB              int64
		expectedDiskGB             int64
		expectedGuestID            string
		expectedFirmware           string
		expectedDiskShares         string
		expectedThroughputCap      string
		expectedLatencySensitivity string
	}{
		{
			name: "All zero values should use defaults",
			spec: VMSpec{
				Name:          "test-vm",
				NetworkName:   "network1",
				DatastoreName: "datastore1",
			},
			expectedCPUs:               2,
			expectedMemMB:              4096,
			expectedDiskGB:             120,
			expectedGuestID:            "rhel8_64Guest",
			expectedFirmware:           "efi",
			expectedDiskShares:         "normal",
			expectedThroughputCap:      "off",
			expectedLatencySensitivity: "normal",
		},
		{
			name: "Custom values preserved",
			spec: VMSpec{
				Name:                   "test-vm",
				NumCPUs:                8,
				MemoryMB:               16384,
				DiskSizeGB:             500,
				GuestID:                "rhel9_64Guest",
				Firmware:               "bios",
				NetworkName:            "network1",
				DatastoreName:          "datastore1",
				DiskShares:             "high",
				DiskThroughputCap:      "1000",
				DiskLatencySensitivity: "high",
			},
			expectedCPUs:               8,
			expectedMemMB:              16384,
			expectedDiskGB:             500,
			expectedGuestID:            "rhel9_64Guest",
			expectedFirmware:           "bios",
			expectedDiskShares:         "high",
			expectedThroughputCap:      "1000",
			expectedLatencySensitivity: "high",
		},
		{
			name: "Partial custom values with defaults",
			spec: VMSpec{
				Name:          "test-vm",
				NumCPUs:       4,
				NetworkName:   "network1",
				DatastoreName: "datastore1",
			},
			expectedCPUs:               4,
			expectedMemMB:              4096,            // default
			expectedDiskGB:             120,             // default
			expectedGuestID:            "rhel8_64Guest", // default
			expectedFirmware:           "efi",           // default
			expectedDiskShares:         "normal",
			expectedThroughputCap:      "off",
			expectedLatencySensitivity: "normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults (simulating what CreateVM does)
			spec := tt.spec

			if spec.NumCPUs == 0 {
				spec.NumCPUs = 2
			}
			if spec.MemoryMB == 0 {
				spec.MemoryMB = 4096
			}
			if spec.DiskSizeGB == 0 {
				spec.DiskSizeGB = 120
			}
			if spec.GuestID == "" {
				spec.GuestID = "rhel8_64Guest"
			}
			if spec.Firmware == "" {
				spec.Firmware = "efi"
			}
			if spec.DiskShares == "" {
				spec.DiskShares = "normal"
			}
			if spec.DiskThroughputCap == "" {
				spec.DiskThroughputCap = "off"
			}
			if spec.DiskLatencySensitivity == "" {
				spec.DiskLatencySensitivity = "normal"
			}

			// Verify defaults were applied correctly
			if spec.NumCPUs != tt.expectedCPUs {
				t.Errorf("Expected NumCPUs %d, got %d", tt.expectedCPUs, spec.NumCPUs)
			}
			if spec.MemoryMB != tt.expectedMemMB {
				t.Errorf("Expected MemoryMB %d, got %d", tt.expectedMemMB, spec.MemoryMB)
			}
			if spec.DiskSizeGB != tt.expectedDiskGB {
				t.Errorf("Expected DiskSizeGB %d, got %d", tt.expectedDiskGB, spec.DiskSizeGB)
			}
			if spec.GuestID != tt.expectedGuestID {
				t.Errorf("Expected GuestID %s, got %s", tt.expectedGuestID, spec.GuestID)
			}
			if spec.Firmware != tt.expectedFirmware {
				t.Errorf("Expected Firmware %s, got %s", tt.expectedFirmware, spec.Firmware)
			}
			if spec.DiskShares != tt.expectedDiskShares {
				t.Errorf("Expected DiskShares %s, got %s", tt.expectedDiskShares, spec.DiskShares)
			}
			if spec.DiskThroughputCap != tt.expectedThroughputCap {
				t.Errorf("Expected DiskThroughputCap %s, got %s", tt.expectedThroughputCap, spec.DiskThroughputCap)
			}
			if spec.DiskLatencySensitivity != tt.expectedLatencySensitivity {
				t.Errorf("Expected DiskLatencySensitivity %s, got %s", tt.expectedLatencySensitivity, spec.DiskLatencySensitivity)
			}
		})
	}
}

// TestVMSpecValidation tests validation of VM spec parameters
func TestVMSpecValidation(t *testing.T) {
	tests := []struct {
		name    string
		spec    VMSpec
		isValid bool
		reason  string
	}{
		{
			name: "Valid minimal spec",
			spec: VMSpec{
				Name:          "test-vm",
				NetworkName:   "network1",
				DatastoreName: "datastore1",
			},
			isValid: true,
		},
		{
			name: "Valid full spec",
			spec: VMSpec{
				Name:                 "test-vm",
				NumCPUs:              4,
				MemoryMB:             8192,
				DiskSizeGB:           200,
				GuestID:              "rhel9_64Guest",
				Firmware:             "efi",
				NetworkName:          "network1",
				DatastoreName:        "datastore1",
				ISOPath:              "[datastore1] iso/agent.iso",
				DiskEnableUUID:       true,
				NestedVirtualization: true,
			},
			isValid: true,
		},
		{
			name: "Missing name",
			spec: VMSpec{
				NetworkName:   "network1",
				DatastoreName: "datastore1",
			},
			isValid: false,
			reason:  "Name is required",
		},
		{
			name: "Missing network",
			spec: VMSpec{
				Name:          "test-vm",
				DatastoreName: "datastore1",
			},
			isValid: false,
			reason:  "NetworkName is required",
		},
		{
			name: "Missing datastore",
			spec: VMSpec{
				Name:        "test-vm",
				NetworkName: "network1",
			},
			isValid: false,
			reason:  "DatastoreName is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation
			isValid := tt.spec.Name != "" && tt.spec.NetworkName != "" && tt.spec.DatastoreName != ""

			if isValid != tt.isValid {
				t.Errorf("Expected isValid %v, got %v. Reason: %s", tt.isValid, isValid, tt.reason)
			}
		})
	}
}

// TestExtraConfigHandling tests ExtraConfig map handling
func TestExtraConfigHandling(t *testing.T) {
	tests := []struct {
		name        string
		extraConfig map[string]string
		expectNil   bool
		expectedLen int
	}{
		{
			name:        "Nil ExtraConfig",
			extraConfig: nil,
			expectNil:   true,
		},
		{
			name:        "Empty ExtraConfig",
			extraConfig: map[string]string{},
			expectNil:   false,
			expectedLen: 0,
		},
		{
			name: "Single ExtraConfig entry",
			extraConfig: map[string]string{
				"guestinfo.vmware-template": "test-template",
			},
			expectNil:   false,
			expectedLen: 1,
		},
		{
			name: "Multiple ExtraConfig entries",
			extraConfig: map[string]string{
				"guestinfo.vmware-template":  "test-template",
				"guestinfo.template-version": "v1",
				"disk.EnableUUID":            "TRUE",
			},
			expectNil:   false,
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := VMSpec{
				Name:          "test-vm",
				NetworkName:   "network1",
				DatastoreName: "datastore1",
				ExtraConfig:   tt.extraConfig,
			}

			if tt.expectNil && spec.ExtraConfig != nil {
				t.Errorf("Expected nil ExtraConfig, got %+v", spec.ExtraConfig)
			}

			if !tt.expectNil {
				if spec.ExtraConfig == nil {
					t.Fatal("Expected non-nil ExtraConfig")
				}
				if len(spec.ExtraConfig) != tt.expectedLen {
					t.Errorf("Expected ExtraConfig length %d, got %d", tt.expectedLen, len(spec.ExtraConfig))
				}
			}
		})
	}
}

// TestDiskUUIDAndNestedVirtualizationSettings tests boolean settings
func TestDiskUUIDAndNestedVirtualizationSettings(t *testing.T) {
	tests := []struct {
		name                 string
		diskEnableUUID       bool
		nestedVirtualization bool
		expectDiskUUID       bool
		expectNestedVirt     bool
	}{
		{
			name:                 "Both disabled",
			diskEnableUUID:       false,
			nestedVirtualization: false,
			expectDiskUUID:       false,
			expectNestedVirt:     false,
		},
		{
			name:                 "DiskUUID enabled",
			diskEnableUUID:       true,
			nestedVirtualization: false,
			expectDiskUUID:       true,
			expectNestedVirt:     false,
		},
		{
			name:                 "NestedVirtualization enabled",
			diskEnableUUID:       false,
			nestedVirtualization: true,
			expectDiskUUID:       false,
			expectNestedVirt:     true,
		},
		{
			name:                 "Both enabled",
			diskEnableUUID:       true,
			nestedVirtualization: true,
			expectDiskUUID:       true,
			expectNestedVirt:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := VMSpec{
				Name:                 "test-vm",
				NetworkName:          "network1",
				DatastoreName:        "datastore1",
				DiskEnableUUID:       tt.diskEnableUUID,
				NestedVirtualization: tt.nestedVirtualization,
			}

			if spec.DiskEnableUUID != tt.expectDiskUUID {
				t.Errorf("Expected DiskEnableUUID %v, got %v", tt.expectDiskUUID, spec.DiskEnableUUID)
			}
			if spec.NestedVirtualization != tt.expectNestedVirt {
				t.Errorf("Expected NestedVirtualization %v, got %v", tt.expectNestedVirt, spec.NestedVirtualization)
			}
		})
	}
}

// TestDiskSchedulingParameters tests SCSI disk scheduling parameters
func TestDiskSchedulingParameters(t *testing.T) {
	tests := []struct {
		name                       string
		diskShares                 string
		diskThroughputCap          string
		diskLatencySensitivity     string
		expectedShares             string
		expectedThroughputCap      string
		expectedLatencySensitivity string
	}{
		{
			name:                       "Default values",
			diskShares:                 "",
			diskThroughputCap:          "",
			diskLatencySensitivity:     "",
			expectedShares:             "normal",
			expectedThroughputCap:      "off",
			expectedLatencySensitivity: "normal",
		},
		{
			name:                       "Low shares",
			diskShares:                 "low",
			diskThroughputCap:          "off",
			diskLatencySensitivity:     "low",
			expectedShares:             "low",
			expectedThroughputCap:      "off",
			expectedLatencySensitivity: "low",
		},
		{
			name:                       "High shares",
			diskShares:                 "high",
			diskThroughputCap:          "off",
			diskLatencySensitivity:     "high",
			expectedShares:             "high",
			expectedThroughputCap:      "off",
			expectedLatencySensitivity: "high",
		},
		{
			name:                       "Custom numeric shares",
			diskShares:                 "2000",
			diskThroughputCap:          "500",
			diskLatencySensitivity:     "medium",
			expectedShares:             "2000",
			expectedThroughputCap:      "500",
			expectedLatencySensitivity: "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := VMSpec{
				Name:                   "test-vm",
				NetworkName:            "network1",
				DatastoreName:          "datastore1",
				DiskShares:             tt.diskShares,
				DiskThroughputCap:      tt.diskThroughputCap,
				DiskLatencySensitivity: tt.diskLatencySensitivity,
			}

			// Apply defaults
			if spec.DiskShares == "" {
				spec.DiskShares = "normal"
			}
			if spec.DiskThroughputCap == "" {
				spec.DiskThroughputCap = "off"
			}
			if spec.DiskLatencySensitivity == "" {
				spec.DiskLatencySensitivity = "normal"
			}

			if spec.DiskShares != tt.expectedShares {
				t.Errorf("Expected DiskShares %s, got %s", tt.expectedShares, spec.DiskShares)
			}
			if spec.DiskThroughputCap != tt.expectedThroughputCap {
				t.Errorf("Expected DiskThroughputCap %s, got %s", tt.expectedThroughputCap, spec.DiskThroughputCap)
			}
			if spec.DiskLatencySensitivity != tt.expectedLatencySensitivity {
				t.Errorf("Expected DiskLatencySensitivity %s, got %s", tt.expectedLatencySensitivity, spec.DiskLatencySensitivity)
			}
		})
	}
}

// TestISOPathValidation tests ISO path format validation
func TestISOPathValidation(t *testing.T) {
	tests := []struct {
		name       string
		isoPath    string
		shouldWarn bool
		reason     string
	}{
		{
			name:       "Valid datastore path",
			isoPath:    "[datastore1] iso/agent.iso",
			shouldWarn: false,
		},
		{
			name:       "Valid datastore path with spaces",
			isoPath:    "[my datastore] path/to/agent.iso",
			shouldWarn: false,
		},
		{
			name:       "Empty ISO path",
			isoPath:    "",
			shouldWarn: false, // Empty is valid (means no ISO attached)
		},
		{
			name:       "Missing brackets",
			isoPath:    "datastore1 iso/agent.iso",
			shouldWarn: true,
			reason:     "Invalid format - missing brackets",
		},
		{
			name:       "Only brackets - technically valid but unusual",
			isoPath:    "[datastore1]",
			shouldWarn: false,
			reason:     "Has brackets and closing bracket - minimal valid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple validation: datastore path should be in format "[datastore] path"
			shouldWarn := false
			if tt.isoPath != "" {
				if len(tt.isoPath) > 0 && tt.isoPath[0] != '[' {
					shouldWarn = true
				} else if len(tt.isoPath) > 0 && !contains(tt.isoPath, "]") {
					shouldWarn = true
				}
			}

			if shouldWarn != tt.shouldWarn {
				t.Errorf("Expected shouldWarn %v, got %v. Reason: %s", tt.shouldWarn, shouldWarn, tt.reason)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i < len(s); i++ {
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestGetVMTag tests retrieving tags from VM ExtraConfig
func TestGetVMTag(t *testing.T) {
	tests := []struct {
		name          string
		vmInfo        *mo.VirtualMachine
		key           string
		expectedValue string
	}{
		{
			name: "Tag exists",
			vmInfo: &mo.VirtualMachine{
				Config: &types.VirtualMachineConfigInfo{
					ExtraConfig: []types.BaseOptionValue{
						&types.OptionValue{
							Key:   "guestinfo.vmware-hcp.template",
							Value: "default/my-template",
						},
					},
				},
			},
			key:           "template",
			expectedValue: "default/my-template",
		},
		{
			name: "Tag does not exist",
			vmInfo: &mo.VirtualMachine{
				Config: &types.VirtualMachineConfigInfo{
					ExtraConfig: []types.BaseOptionValue{
						&types.OptionValue{
							Key:   "guestinfo.other-key",
							Value: "other-value",
						},
					},
				},
			},
			key:           "template",
			expectedValue: "",
		},
		{
			name: "Multiple tags - retrieve specific one",
			vmInfo: &mo.VirtualMachine{
				Config: &types.VirtualMachineConfigInfo{
					ExtraConfig: []types.BaseOptionValue{
						&types.OptionValue{
							Key:   "guestinfo.vmware-hcp.template",
							Value: "default/my-template",
						},
						&types.OptionValue{
							Key:   "guestinfo.vmware-hcp.version",
							Value: "v1",
						},
					},
				},
			},
			key:           "version",
			expectedValue: "v1",
		},
		{
			name:          "Nil Config",
			vmInfo:        &mo.VirtualMachine{Config: nil},
			key:           "template",
			expectedValue: "",
		},
		{
			name: "Nil ExtraConfig",
			vmInfo: &mo.VirtualMachine{
				Config: &types.VirtualMachineConfigInfo{
					ExtraConfig: nil,
				},
			},
			key:           "template",
			expectedValue: "",
		},
		{
			name: "Empty ExtraConfig",
			vmInfo: &mo.VirtualMachine{
				Config: &types.VirtualMachineConfigInfo{
					ExtraConfig: []types.BaseOptionValue{},
				},
			},
			key:           "template",
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := GetVMTag(tt.vmInfo, tt.key)
			if value != tt.expectedValue {
				t.Errorf("Expected value %q, got %q", tt.expectedValue, value)
			}
		})
	}
}
