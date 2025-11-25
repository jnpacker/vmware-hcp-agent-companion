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

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
)

func TestCalculateEstimatedVMCapacity(t *testing.T) {
	tests := []struct {
		name                 string
		datastoreUtil        vmwarev1alpha1.DatastoreUtilization
		computeUtil          vmwarev1alpha1.ComputeUtilization
		vmSpec               vmwarev1alpha1.VMTemplateSpec
		useEffectiveCapacity bool
		expected             int32
		description          string
	}{
		{
			name: "default - use total capacity (flag false)",
			datastoreUtil: vmwarev1alpha1.DatastoreUtilization{
				FreeSpaceGB: 1000,
			},
			computeUtil: vmwarev1alpha1.ComputeUtilization{
				CpuTotalMhz:       100000, // 100 GHz total
				CpuEffectiveMhz:   90000,  // 90 GHz effective (10 GHz HA overhead)
				CpuUsedMhz:        50000,  // 50 GHz used
				MemoryTotalMb:     100000, // ~97 GB total
				MemoryEffectiveMb: 90000,  // ~88 GB effective
				MemoryUsedMb:      50000,  // ~49 GB used
			},
			vmSpec: vmwarev1alpha1.VMTemplateSpec{
				NumCPUs:    4,    // 4 vCPUs × 2000 MHz = 8000 MHz per VM
				MemoryMB:   8192, // 8 GB per VM
				DiskSizeGB: 120,  // 120 GB per VM
			},
			useEffectiveCapacity: false, // Use total - used
			expected:             6,     // Limited by memory: (100000 - 50000) / 8192 = 6.1
			description:          "Default mode uses total capacity, allowing more VMs (not accounting for HA)",
		},
		{
			name: "use effective capacity (flag true)",
			datastoreUtil: vmwarev1alpha1.DatastoreUtilization{
				FreeSpaceGB: 1000,
			},
			computeUtil: vmwarev1alpha1.ComputeUtilization{
				CpuTotalMhz:       100000, // 100 GHz total
				CpuEffectiveMhz:   90000,  // 90 GHz effective (10 GHz HA overhead)
				CpuUsedMhz:        50000,  // 50 GHz used
				MemoryTotalMb:     100000, // ~97 GB total
				MemoryEffectiveMb: 90000,  // ~88 GB effective
				MemoryUsedMb:      50000,  // ~49 GB used
			},
			vmSpec: vmwarev1alpha1.VMTemplateSpec{
				NumCPUs:    4,    // 4 vCPUs × 2000 MHz = 8000 MHz per VM
				MemoryMB:   8192, // 8 GB per VM
				DiskSizeGB: 120,  // 120 GB per VM
			},
			useEffectiveCapacity: true, // Use effective - used
			expected:             4,    // Limited by memory: (90000 - 50000) / 8192 = 4.8
			description:          "Effective mode accounts for HA overhead, allowing fewer VMs (safer)",
		},
		{
			name: "storage constrained",
			datastoreUtil: vmwarev1alpha1.DatastoreUtilization{
				FreeSpaceGB: 500, // Only 500 GB available
			},
			computeUtil: vmwarev1alpha1.ComputeUtilization{
				CpuTotalMhz:       100000,
				CpuEffectiveMhz:   90000,
				CpuUsedMhz:        10000,
				MemoryTotalMb:     100000,
				MemoryEffectiveMb: 90000,
				MemoryUsedMb:      10000,
			},
			vmSpec: vmwarev1alpha1.VMTemplateSpec{
				NumCPUs:    2,
				MemoryMB:   4096,
				DiskSizeGB: 120,
			},
			useEffectiveCapacity: false,
			expected:             4, // Storage: 500 / 120 = 4.1
			description:          "Storage is most constrained resource",
		},
		{
			name: "over-committed cluster - effective capacity exceeded",
			datastoreUtil: vmwarev1alpha1.DatastoreUtilization{
				FreeSpaceGB: 1000,
			},
			computeUtil: vmwarev1alpha1.ComputeUtilization{
				CpuTotalMhz:       100000,
				CpuEffectiveMhz:   90000,
				CpuUsedMhz:        95000, // Using MORE than effective!
				MemoryTotalMb:     100000,
				MemoryEffectiveMb: 90000,
				MemoryUsedMb:      92000, // Using MORE than effective!
			},
			vmSpec: vmwarev1alpha1.VMTemplateSpec{
				NumCPUs:    2,
				MemoryMB:   4096,
				DiskSizeGB: 120,
			},
			useEffectiveCapacity: true,
			expected:             0, // Effective - Used is negative, clamped to 0
			description:          "Over-committed cluster shows 0 capacity with effective mode",
		},
		{
			name: "over-committed cluster - total still has space",
			datastoreUtil: vmwarev1alpha1.DatastoreUtilization{
				FreeSpaceGB: 1000,
			},
			computeUtil: vmwarev1alpha1.ComputeUtilization{
				CpuTotalMhz:       100000,
				CpuEffectiveMhz:   90000,
				CpuUsedMhz:        95000, // Using MORE than effective but less than total
				MemoryTotalMb:     100000,
				MemoryEffectiveMb: 90000,
				MemoryUsedMb:      92000,
			},
			vmSpec: vmwarev1alpha1.VMTemplateSpec{
				NumCPUs:    2,    // 2 × 2000 = 4000 MHz per VM
				MemoryMB:   4096, // 4 GB per VM
				DiskSizeGB: 120,
			},
			useEffectiveCapacity: false,
			expected:             1, // Total mode: (100000 - 95000) / 4000 = 1.25
			description:          "Over-committed cluster still shows capacity with total mode",
		},
		{
			name: "default VM specs",
			datastoreUtil: vmwarev1alpha1.DatastoreUtilization{
				FreeSpaceGB: 1000,
			},
			computeUtil: vmwarev1alpha1.ComputeUtilization{
				CpuTotalMhz:       100000,
				CpuEffectiveMhz:   90000,
				CpuUsedMhz:        50000,
				MemoryTotalMb:     100000,
				MemoryEffectiveMb: 90000,
				MemoryUsedMb:      50000,
			},
			vmSpec: vmwarev1alpha1.VMTemplateSpec{
				// All defaults: 2 CPUs, 4096 MB, 120 GB
			},
			useEffectiveCapacity: false,
			expected:             8, // Storage: 1000/120=8, CPU: 50000/4000=12, Memory: 50000/4096=12
			description:          "Test with default VM specifications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateEstimatedVMCapacity(
				tt.datastoreUtil,
				tt.computeUtil,
				tt.vmSpec,
				tt.useEffectiveCapacity,
			)

			if result != tt.expected {
				t.Errorf("%s: expected %d VMs, got %d VMs\nDescription: %s\nDetails:\n  Storage: %d GB free, need %d GB/VM\n  CPU: Total=%d, Effective=%d, Used=%d MHz, need %d MHz/VM\n  Memory: Total=%d, Effective=%d, Used=%d MB, need %d MB/VM\n  UseEffective: %v",
					tt.name,
					tt.expected,
					result,
					tt.description,
					tt.datastoreUtil.FreeSpaceGB,
					getDiskSize(tt.vmSpec),
					tt.computeUtil.CpuTotalMhz,
					tt.computeUtil.CpuEffectiveMhz,
					tt.computeUtil.CpuUsedMhz,
					getCPUPerVM(tt.vmSpec),
					tt.computeUtil.MemoryTotalMb,
					tt.computeUtil.MemoryEffectiveMb,
					tt.computeUtil.MemoryUsedMb,
					getMemoryPerVM(tt.vmSpec),
					tt.useEffectiveCapacity,
				)
			}
		})
	}
}

// Helper functions for test output
func getDiskSize(spec vmwarev1alpha1.VMTemplateSpec) int64 {
	if spec.DiskSizeGB == 0 {
		return 120
	}
	return spec.DiskSizeGB
}

func getCPUPerVM(spec vmwarev1alpha1.VMTemplateSpec) int64 {
	numCPUs := spec.NumCPUs
	if numCPUs == 0 {
		numCPUs = 2
	}
	return int64(numCPUs) * 2000
}

func getMemoryPerVM(spec vmwarev1alpha1.VMTemplateSpec) int64 {
	if spec.MemoryMB == 0 {
		return 4096
	}
	return spec.MemoryMB
}
