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
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25/mo"

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
)

// GetDatastoreUtilization retrieves capacity information for a datastore
func (c *Client) GetDatastoreUtilization(ctx context.Context, datastoreName string) (*vmwarev1alpha1.DatastoreUtilization, error) {
	ds, err := c.GetDatastore(ctx, datastoreName)
	if err != nil {
		return nil, fmt.Errorf("failed to find datastore %s: %w", datastoreName, err)
	}

	var props mo.Datastore
	if err := ds.Properties(ctx, ds.Reference(), []string{"summary"}, &props); err != nil {
		return nil, fmt.Errorf("failed to get datastore properties: %w", err)
	}

	capacityBytes := props.Summary.Capacity
	freeSpaceBytes := props.Summary.FreeSpace
	usedBytes := capacityBytes - freeSpaceBytes

	capacityGB := capacityBytes / (1024 * 1024 * 1024)
	freeSpaceGB := freeSpaceBytes / (1024 * 1024 * 1024)
	usedGB := usedBytes / (1024 * 1024 * 1024)

	percentUsed := int32(0)
	if capacityGB > 0 {
		percentUsed = int32((usedGB * 100) / capacityGB)
	}

	return &vmwarev1alpha1.DatastoreUtilization{
		Name:        datastoreName,
		CapacityGB:  capacityGB,
		FreeSpaceGB: freeSpaceGB,
		UsedGB:      usedGB,
		PercentUsed: percentUsed,
	}, nil
}

// GetClusterUtilization retrieves CPU/memory utilization for a cluster
func (c *Client) GetClusterUtilization(ctx context.Context, clusterName string) (*vmwarev1alpha1.ComputeUtilization, error) {
	cluster, err := c.finder.ClusterComputeResource(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster %s: %w", clusterName, err)
	}

	var clusterProps mo.ClusterComputeResource
	if err := cluster.Properties(ctx, cluster.Reference(), []string{"summary", "host"}, &clusterProps); err != nil {
		return nil, fmt.Errorf("failed to get cluster properties: %w", err)
	}

	// Get total and effective capacity from cluster summary
	cpuTotalMhz := int64(clusterProps.Summary.GetComputeResourceSummary().TotalCpu)
	cpuEffectiveMhz := int64(clusterProps.Summary.GetComputeResourceSummary().EffectiveCpu)
	memoryTotalMb := clusterProps.Summary.GetComputeResourceSummary().TotalMemory / (1024 * 1024)
	memoryEffectiveMb := clusterProps.Summary.GetComputeResourceSummary().EffectiveMemory / (1024 * 1024)

	// Aggregate actual usage from all hosts in the cluster
	var cpuUsedMhz int64
	var memoryUsedMb int64

	if len(clusterProps.Host) > 0 {
		// Query all hosts for their runtime stats
		var hosts []mo.HostSystem
		err := c.client.Retrieve(ctx, clusterProps.Host, []string{"summary.quickStats"}, &hosts)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve host stats: %w", err)
		}

		// Aggregate usage across all hosts
		for _, host := range hosts {
			// overallCpuUsage is in MHz
			cpuUsedMhz += int64(host.Summary.QuickStats.OverallCpuUsage)
			// overallMemoryUsage is in MB
			memoryUsedMb += int64(host.Summary.QuickStats.OverallMemoryUsage)
		}
	}

	// Calculate available resources (total - used)
	// Note: Effective represents capacity after HA overhead, but available should be
	// simple math: total - used to match the percentage calculation
	cpuAvailableMhz := cpuTotalMhz - cpuUsedMhz
	memoryAvailableMb := memoryTotalMb - memoryUsedMb

	// Ensure non-negative values
	if cpuAvailableMhz < 0 {
		cpuAvailableMhz = 0
	}
	if memoryAvailableMb < 0 {
		memoryAvailableMb = 0
	}

	// Calculate percentages (used relative to total)
	cpuPercentUsed := int32(0)
	if cpuTotalMhz > 0 {
		cpuPercentUsed = int32((cpuUsedMhz * 100) / cpuTotalMhz)
	}

	memoryPercentUsed := int32(0)
	if memoryTotalMb > 0 {
		memoryPercentUsed = int32((memoryUsedMb * 100) / memoryTotalMb)
	}

	return &vmwarev1alpha1.ComputeUtilization{
		ResourceType:      "Cluster",
		Name:              clusterName,
		CpuTotalMhz:       cpuTotalMhz,
		CpuEffectiveMhz:   cpuEffectiveMhz,
		CpuUsedMhz:        cpuUsedMhz,
		CpuAvailableMhz:   cpuAvailableMhz,
		CpuPercentUsed:    cpuPercentUsed,
		MemoryTotalMb:     memoryTotalMb,
		MemoryEffectiveMb: memoryEffectiveMb,
		MemoryUsedMb:      memoryUsedMb,
		MemoryAvailableMb: memoryAvailableMb,
		MemoryPercentUsed: memoryPercentUsed,
	}, nil
}

// GetResourcePoolUtilization retrieves CPU/memory utilization for a resource pool
func (c *Client) GetResourcePoolUtilization(ctx context.Context, poolName string) (*vmwarev1alpha1.ComputeUtilization, error) {
	pool, err := c.finder.ResourcePool(ctx, poolName)
	if err != nil {
		return nil, fmt.Errorf("failed to find resource pool %s: %w", poolName, err)
	}

	var props mo.ResourcePool
	if err := pool.Properties(ctx, pool.Reference(), []string{"runtime", "summary"}, &props); err != nil {
		return nil, fmt.Errorf("failed to get resource pool properties: %w", err)
	}

	// Resource pools have runtime usage stats
	cpuLimit := props.Runtime.Cpu.MaxUsage
	cpuUsage := props.Runtime.Cpu.OverallUsage
	cpuAvailable := cpuLimit - cpuUsage

	memoryLimit := props.Runtime.Memory.MaxUsage / (1024 * 1024) // Convert bytes to MB
	memoryUsage := props.Runtime.Memory.OverallUsage / (1024 * 1024)
	memoryAvailable := memoryLimit - memoryUsage

	// Ensure non-negative
	if cpuAvailable < 0 {
		cpuAvailable = 0
	}
	if memoryAvailable < 0 {
		memoryAvailable = 0
	}

	cpuPercentUsed := int32(0)
	if cpuLimit > 0 {
		cpuPercentUsed = int32((cpuUsage * 100) / cpuLimit)
	}

	memoryPercentUsed := int32(0)
	if memoryLimit > 0 {
		memoryPercentUsed = int32((memoryUsage * 100) / memoryLimit)
	}

	return &vmwarev1alpha1.ComputeUtilization{
		ResourceType:      "ResourcePool",
		Name:              poolName,
		CpuTotalMhz:       cpuLimit,
		CpuEffectiveMhz:   cpuLimit, // Resource pools don't have HA overhead, so effective = total
		CpuUsedMhz:        cpuUsage,
		CpuAvailableMhz:   cpuAvailable,
		CpuPercentUsed:    cpuPercentUsed,
		MemoryTotalMb:     memoryLimit,
		MemoryEffectiveMb: memoryLimit, // Resource pools don't have HA overhead, so effective = total
		MemoryUsedMb:      memoryUsage,
		MemoryAvailableMb: memoryAvailable,
		MemoryPercentUsed: memoryPercentUsed,
	}, nil
}

// ValidateResources validates that all specified resources exist and are accessible
func (c *Client) ValidateResources(ctx context.Context, spec vmwarev1alpha1.VMTemplateSpec) (*vmwarev1alpha1.ResourceValidation, error) {
	validation := &vmwarev1alpha1.ResourceValidation{}

	// Validate datacenter (should already be set)
	if c.datacenter != nil {
		validation.Datacenter = vmwarev1alpha1.ResourceValidationStatus{
			Exists:  true,
			Message: fmt.Sprintf("Datacenter '%s' is accessible", spec.Datacenter),
		}
	} else {
		validation.Datacenter = vmwarev1alpha1.ResourceValidationStatus{
			Exists:  false,
			Message: fmt.Sprintf("Datacenter '%s' not found or not set", spec.Datacenter),
		}
	}

	// Validate cluster if specified
	if spec.Cluster != "" {
		_, err := c.finder.ClusterComputeResource(ctx, spec.Cluster)
		if err != nil {
			validation.Cluster = &vmwarev1alpha1.ResourceValidationStatus{
				Exists:  false,
				Message: fmt.Sprintf("Cluster '%s' not found: %v", spec.Cluster, err),
			}
		} else {
			validation.Cluster = &vmwarev1alpha1.ResourceValidationStatus{
				Exists:  true,
				Message: fmt.Sprintf("Cluster '%s' is accessible", spec.Cluster),
			}
		}
	}

	// Validate resource pool if specified
	if spec.ResourcePool != "" {
		_, err := c.finder.ResourcePool(ctx, spec.ResourcePool)
		if err != nil {
			validation.ResourcePool = &vmwarev1alpha1.ResourceValidationStatus{
				Exists:  false,
				Message: fmt.Sprintf("ResourcePool '%s' not found: %v", spec.ResourcePool, err),
			}
		} else {
			validation.ResourcePool = &vmwarev1alpha1.ResourceValidationStatus{
				Exists:  true,
				Message: fmt.Sprintf("ResourcePool '%s' is accessible", spec.ResourcePool),
			}
		}
	}

	// Validate datastore
	_, err := c.GetDatastore(ctx, spec.Datastore)
	if err != nil {
		validation.Datastore = vmwarev1alpha1.ResourceValidationStatus{
			Exists:  false,
			Message: fmt.Sprintf("Datastore '%s' not found: %v", spec.Datastore, err),
		}
	} else {
		validation.Datastore = vmwarev1alpha1.ResourceValidationStatus{
			Exists:  true,
			Message: fmt.Sprintf("Datastore '%s' is accessible", spec.Datastore),
		}
	}

	// Validate network
	_, err = c.GetNetwork(ctx, spec.Network)
	if err != nil {
		validation.Network = vmwarev1alpha1.ResourceValidationStatus{
			Exists:  false,
			Message: fmt.Sprintf("Network '%s' not found: %v", spec.Network, err),
		}
	} else {
		validation.Network = vmwarev1alpha1.ResourceValidationStatus{
			Exists:  true,
			Message: fmt.Sprintf("Network '%s' is accessible", spec.Network),
		}
	}

	// Validate folder if specified
	if spec.Folder != "" {
		_, err := c.finder.Folder(ctx, spec.Folder)
		if err != nil {
			validation.Folder = &vmwarev1alpha1.ResourceValidationStatus{
				Exists:  false,
				Message: fmt.Sprintf("Folder '%s' not found (will be created): %v", spec.Folder, err),
			}
		} else {
			validation.Folder = &vmwarev1alpha1.ResourceValidationStatus{
				Exists:  true,
				Message: fmt.Sprintf("Folder '%s' is accessible", spec.Folder),
			}
		}
	}

	return validation, nil
}

// CalculateEstimatedVMCapacity calculates how many VMs can be created based on available resources
func CalculateEstimatedVMCapacity(
	datastoreUtil vmwarev1alpha1.DatastoreUtilization,
	computeUtil vmwarev1alpha1.ComputeUtilization,
	vmSpec vmwarev1alpha1.VMTemplateSpec,
	useEffectiveCapacity bool,
) int32 {
	// Calculate capacity based on storage
	diskSizeGB := vmSpec.DiskSizeGB
	if diskSizeGB == 0 {
		diskSizeGB = 120 // Default disk size
	}
	storageCapacity := int32(0)
	if diskSizeGB > 0 {
		storageCapacity = int32(datastoreUtil.FreeSpaceGB / diskSizeGB)
	}

	// Calculate capacity based on CPU
	numCPUs := vmSpec.NumCPUs
	if numCPUs == 0 {
		numCPUs = 2 // Default CPU count
	}
	// Assume average of 2000 MHz per vCPU (this varies by host CPU)
	cpuMhzPerVM := int64(numCPUs) * 2000
	cpuCapacity := int32(0)
	if cpuMhzPerVM > 0 {
		var cpuAvailableForVMs int64
		if useEffectiveCapacity {
			// Use effective capacity (accounts for HA/DRS overhead)
			cpuAvailableForVMs = computeUtil.CpuEffectiveMhz - computeUtil.CpuUsedMhz
		} else {
			// Use total capacity (default - simple calculation)
			cpuAvailableForVMs = computeUtil.CpuTotalMhz - computeUtil.CpuUsedMhz
		}

		if cpuAvailableForVMs < 0 {
			cpuAvailableForVMs = 0
		}
		cpuCapacity = int32(cpuAvailableForVMs / cpuMhzPerVM)
	}

	// Calculate capacity based on memory
	memoryMB := vmSpec.MemoryMB
	if memoryMB == 0 {
		memoryMB = 4096 // Default memory
	}
	memoryCapacity := int32(0)
	if memoryMB > 0 {
		var memoryAvailableForVMs int64
		if useEffectiveCapacity {
			// Use effective capacity (accounts for HA/DRS overhead)
			memoryAvailableForVMs = computeUtil.MemoryEffectiveMb - computeUtil.MemoryUsedMb
		} else {
			// Use total capacity (default - simple calculation)
			memoryAvailableForVMs = computeUtil.MemoryTotalMb - computeUtil.MemoryUsedMb
		}

		if memoryAvailableForVMs < 0 {
			memoryAvailableForVMs = 0
		}
		memoryCapacity = int32(memoryAvailableForVMs / memoryMB)
	}

	// Return the minimum (most constraining resource)
	capacity := storageCapacity
	if cpuCapacity < capacity {
		capacity = cpuCapacity
	}
	if memoryCapacity < capacity {
		capacity = memoryCapacity
	}

	// Ensure non-negative
	if capacity < 0 {
		capacity = 0
	}

	return capacity
}

// GetComputeUtilization retrieves compute utilization for either cluster or resource pool
// This is a convenience function that chooses the appropriate method based on what's specified
func (c *Client) GetComputeUtilization(ctx context.Context, clusterName, resourcePoolName string) (*vmwarev1alpha1.ComputeUtilization, error) {
	// If a specific resource pool is provided, use that for more accurate limits
	if resourcePoolName != "" {
		return c.GetResourcePoolUtilization(ctx, resourcePoolName)
	}

	// Otherwise, use cluster-level stats
	if clusterName != "" {
		return c.GetClusterUtilization(ctx, clusterName)
	}

	return nil, fmt.Errorf("either clusterName or resourcePoolName must be specified")
}

// GetResourceUtilization retrieves complete resource utilization including validation
// This is the main entry point for the controller to get all resource information
func (c *Client) GetResourceUtilization(ctx context.Context, spec vmwarev1alpha1.VMTemplateSpec, useEffectiveCapacity bool) (*vmwarev1alpha1.ResourceUtilization, *vmwarev1alpha1.ResourceValidation, error) {
	// Validate resources first
	validation, err := c.ValidateResources(ctx, spec)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate resources: %w", err)
	}

	// Check if critical resources exist before querying utilization
	if !validation.Datastore.Exists {
		return nil, validation, fmt.Errorf("datastore '%s' does not exist", spec.Datastore)
	}

	// Get datastore utilization
	datastoreUtil, err := c.GetDatastoreUtilization(ctx, spec.Datastore)
	if err != nil {
		return nil, validation, fmt.Errorf("failed to get datastore utilization: %w", err)
	}

	// Get compute utilization (prefer resource pool if specified, otherwise use cluster)
	computeUtil, err := c.GetComputeUtilization(ctx, spec.Cluster, spec.ResourcePool)
	if err != nil {
		return nil, validation, fmt.Errorf("failed to get compute utilization: %w", err)
	}

	// Calculate estimated VM capacity
	estimatedCapacity := CalculateEstimatedVMCapacity(*datastoreUtil, *computeUtil, spec, useEffectiveCapacity)

	utilization := &vmwarev1alpha1.ResourceUtilization{
		Datastore:           *datastoreUtil,
		Compute:             *computeUtil,
		EstimatedVMCapacity: estimatedCapacity,
	}

	return utilization, validation, nil
}
