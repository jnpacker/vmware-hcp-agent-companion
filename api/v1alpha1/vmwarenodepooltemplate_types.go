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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VMwareNodePoolTemplateSpec defines the desired state of VMwareNodePoolTemplate
type VMwareNodePoolTemplateSpec struct {
	// NodePoolRef references the target NodePool to manage VMs for.
	NodePoolRef *NodePoolReference `json:"nodePoolRef"`

	// VSphereCredentials references the VMware vSphere credentials.
	// Defaults to looking for a Secret named "vsphere-credentials" in the same namespace.
	// +optional
	VSphereCredentials *CredentialReference `json:"vSphereCredentials,omitempty"`

	// VMTemplate defines the VMware VM specifications
	VMTemplate VMTemplateSpec `json:"vmTemplate"`

	// AgentISO defines how to provide the Agent ISO to VMs
	AgentISO AgentISOSpec `json:"agentISO"`

	// AgentLabelSelector defines labels to apply to discovered Agents
	// to associate them with the NodePool
	// +optional
	AgentLabelSelector map[string]string `json:"agentLabelSelector,omitempty"`

	// TestMode when true, creates VMs without requiring a NodePool.
	// Useful for validating vSphere connectivity and VM creation.
	// +optional
	TestMode bool `json:"testMode,omitempty"`

	// Replicas defines the number of VMs to create in test mode.
	// Ignored when NodePoolRef is specified (uses NodePool replica count instead).
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// ResourcePollingInterval defines how often to poll vSphere for resource utilization.
	// Format: duration string (e.g., "5m", "300s")
	// Default: 5m (5 minutes)
	// Minimum: 1m (1 minute)
	// +optional
	ResourcePollingInterval *metav1.Duration `json:"resourcePollingInterval,omitempty"`

	// UseEffectiveCapacity determines whether to use effective capacity (after HA overhead)
	// or total capacity when estimating VM capacity.
	// - false (default): Use total capacity - used (simple calculation, no HA consideration)
	// - true: Use effective capacity - used (accounts for HA/DRS overhead)
	// Default: false
	// +optional
	UseEffectiveCapacity bool `json:"useEffectiveCapacity,omitempty"`
}

// NodePoolReference references a NodePool resource
type NodePoolReference struct {
	// Name of the NodePool
	Name string `json:"name"`
}

// CredentialReference references a Secret containing vSphere credentials
type CredentialReference struct {
	// Name of the Secret containing vSphere credentials
	// +optional
	Name string `json:"name,omitempty"`
}

// VMTemplateSpec defines the VMware VM specifications
type VMTemplateSpec struct {
	// Datacenter name in vSphere
	Datacenter string `json:"datacenter"`

	// Cluster name in vSphere
	// +optional
	Cluster string `json:"cluster,omitempty"`

	// ResourcePool name. If not specified, uses the cluster's root resource pool.
	// +optional
	ResourcePool string `json:"resourcePool,omitempty"`

	// Datastore name for VM storage
	Datastore string `json:"datastore"`

	// Folder path for VMs. If not specified, VMs are created in the datacenter's VM folder.
	// +optional
	Folder string `json:"folder,omitempty"`

	// Network name to attach VMs to
	Network string `json:"network"`

	// NumCPUs number of virtual CPUs. Default: 2
	// +optional
	NumCPUs int32 `json:"numCPUs,omitempty"`

	// MemoryMB amount of memory in MB. Default: 4096
	// +optional
	MemoryMB int64 `json:"memoryMB,omitempty"`

	// DiskSizeGB size of the primary disk in GB. Default: 120
	// +optional
	DiskSizeGB int64 `json:"diskSizeGB,omitempty"`

	// GuestID the guest operating system identifier. Default: "otherLinux64Guest"
	// +optional
	GuestID string `json:"guestID,omitempty"`

	// Firmware type: bios or efi. Default: "bios"
	// +optional
	Firmware string `json:"firmware,omitempty"`

	// NamePrefix prefix for VM names. VMs will be named <prefix>-<index>
	// If not specified, uses the template resource name.
	// +optional
	NamePrefix string `json:"namePrefix,omitempty"`

	// ExtraConfig additional VM configuration options
	// +optional
	ExtraConfig map[string]string `json:"extraConfig,omitempty"`

	// AdvancedConfig contains advanced VMware configuration parameters
	// +optional
	AdvancedConfig *AdvancedVMConfig `json:"advancedConfig,omitempty"`
}

// AdvancedVMConfig defines advanced VMware VM configuration parameters
type AdvancedVMConfig struct {
	// DiskEnableUUID enables UUID for virtual disks. Required for Kubernetes storage mounting.
	// When enabled, vSphere will assign a UUID to each virtual disk for unique identification.
	// Default: true
	// +optional
	DiskEnableUUID *bool `json:"diskEnableUUID,omitempty"`

	// NestedVirtualization enables hardware-assisted virtualization to the guest OS.
	// This allows the VM to run hypervisors (nested virtualization).
	// Default: true
	// +optional
	NestedVirtualization *bool `json:"nestedVirtualization,omitempty"`
}

// AgentISOSpec defines how the Agent ISO is provided
type AgentISOSpec struct {
	// Type specifies how the ISO is provided: "datastore" or "url"
	// - datastore: ISO already exists in vSphere datastore
	// - url: Controller downloads and uploads ISO
	// +kubebuilder:validation:Enum=datastore;url
	Type string `json:"type"`

	// DatastorePath is the path to the ISO in the vSphere datastore.
	// Required when Type is "datastore".
	// Format: "[datastore-name] path/to/agent.iso"
	// +optional
	DatastorePath string `json:"datastorePath,omitempty"`

	// URL is the HTTP(S) URL to download the ISO from.
	// Required when Type is "url".
	// +optional
	URL string `json:"url,omitempty"`

	// UploadedISOName is the name to use when uploading the ISO to the datastore.
	// Used when Type is "url". Defaults to "agent-<hash>.iso"
	// +optional
	UploadedISOName string `json:"uploadedISOName,omitempty"`
}

// VMwareNodePoolTemplateStatus defines the observed state of VMwareNodePoolTemplate
type VMwareNodePoolTemplateStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DesiredReplicas is the desired number of VMs
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// CurrentReplicas is the current number of VMs
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// ReadyReplicas is the number of VMs that are ready (powered on)
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// VMStatus contains status information for each VM
	VMStatus []VMStatus `json:"vmStatus,omitempty"`

	// ISOUploaded indicates if the ISO has been uploaded (for url type)
	ISOUploaded bool `json:"isoUploaded,omitempty"`

	// ISOPath is the resolved path to the ISO in vSphere
	ISOPath string `json:"isoPath,omitempty"`

	// ResourceUtilization tracks vSphere resource availability
	// +optional
	ResourceUtilization *ResourceUtilization `json:"resourceUtilization,omitempty"`

	// ResourceValidation tracks validation status of referenced resources
	// +optional
	ResourceValidation *ResourceValidation `json:"resourceValidation,omitempty"`
}

// VMStatus represents the status of a single VM
type VMStatus struct {
	// Name of the VM
	Name string `json:"name"`

	// UUID of the VM in vSphere
	UUID string `json:"uuid,omitempty"`

	// SerialNumber of the VM in vSphere for additional agent matching
	SerialNumber string `json:"serialNumber,omitempty"`

	// PowerState of the VM
	PowerState string `json:"powerState,omitempty"`

	// AgentName is the name of the associated Agent resource
	AgentName string `json:"agentName,omitempty"`

	// Phase represents the current phase of the VM
	Phase string `json:"phase,omitempty"`

	// Message provides additional information about the VM status
	Message string `json:"message,omitempty"`

	// LastTransitionTime is the last time the VM status changed
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// ResourceUtilization tracks vSphere resource availability and capacity
type ResourceUtilization struct {
	// Datastore capacity information
	Datastore DatastoreUtilization `json:"datastore"`

	// Compute resource utilization (Cluster or ResourcePool)
	Compute ComputeUtilization `json:"compute"`

	// EstimatedVMCapacity is the estimated number of VMs that can be created
	// based on available resources (minimum of CPU, memory, and storage constraints)
	EstimatedVMCapacity int32 `json:"estimatedVMCapacity"`

	// LastUpdated is when resource utilization was last queried
	LastUpdated metav1.Time `json:"lastUpdated"`
}

// DatastoreUtilization represents storage capacity information
type DatastoreUtilization struct {
	// Name of the datastore
	Name string `json:"name"`

	// CapacityGB is the total capacity in gigabytes
	CapacityGB int64 `json:"capacityGB"`

	// FreeSpaceGB is the available free space in gigabytes
	FreeSpaceGB int64 `json:"freeSpaceGB"`

	// UsedGB is the used space in gigabytes
	UsedGB int64 `json:"usedGB"`

	// PercentUsed is the percentage of datastore capacity used (0-100)
	PercentUsed int32 `json:"percentUsed"`
}

// ComputeUtilization represents CPU and memory resource information
type ComputeUtilization struct {
	// ResourceType indicates whether this is from "Cluster" or "ResourcePool"
	ResourceType string `json:"resourceType"`

	// Name of the compute resource (cluster or resource pool)
	Name string `json:"name"`

	// CPU resources in MHz
	CpuTotalMhz     int64 `json:"cpuTotalMhz"`     // Total CPU capacity
	CpuEffectiveMhz int64 `json:"cpuEffectiveMhz"` // Effective CPU after HA/DRS overhead (cluster only)
	CpuUsedMhz      int64 `json:"cpuUsedMhz"`      // Actual CPU usage
	CpuAvailableMhz int64 `json:"cpuAvailableMhz"` // Available = Total - Used (matches percent calculation)
	CpuPercentUsed  int32 `json:"cpuPercentUsed"`  // Percent used relative to total

	// Memory resources in MB
	MemoryTotalMb     int64 `json:"memoryTotalMb"`     // Total memory capacity
	MemoryEffectiveMb int64 `json:"memoryEffectiveMb"` // Effective memory after HA/DRS overhead (cluster only)
	MemoryUsedMb      int64 `json:"memoryUsedMb"`      // Actual memory usage
	MemoryAvailableMb int64 `json:"memoryAvailableMb"` // Available = Total - Used (matches percent calculation)
	MemoryPercentUsed int32 `json:"memoryPercentUsed"` // Percent used relative to total
}

// ResourceValidation tracks validation status of vSphere resources
type ResourceValidation struct {
	// Datacenter validation status
	Datacenter ResourceValidationStatus `json:"datacenter"`

	// Cluster validation status
	// +optional
	Cluster *ResourceValidationStatus `json:"cluster,omitempty"`

	// ResourcePool validation status
	// +optional
	ResourcePool *ResourceValidationStatus `json:"resourcePool,omitempty"`

	// Datastore validation status
	Datastore ResourceValidationStatus `json:"datastore"`

	// Network validation status
	Network ResourceValidationStatus `json:"network"`

	// Folder validation status
	// +optional
	Folder *ResourceValidationStatus `json:"folder,omitempty"`
}

// ResourceValidationStatus represents the validation status of a single resource
type ResourceValidationStatus struct {
	// Exists indicates whether the resource exists and is accessible
	Exists bool `json:"exists"`

	// Message provides additional information about the validation status
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vmtemplate;vmtpl
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredReplicas`
// +kubebuilder:printcolumn:name="Current",type=integer,JSONPath=`.status.currentReplicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Est. Capacity",type=integer,JSONPath=`.status.resourceUtilization.estimatedVMCapacity`,priority=1
// +kubebuilder:printcolumn:name="Storage Free",type=string,JSONPath=`.status.resourceUtilization.datastore.freeSpaceGB`,priority=1
// +kubebuilder:printcolumn:name="CPU Avail %",type=integer,JSONPath=`.status.resourceUtilization.compute.cpuPercentUsed`,priority=1
// +kubebuilder:printcolumn:name="Mem Avail %",type=integer,JSONPath=`.status.resourceUtilization.compute.memoryPercentUsed`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VMwareNodePoolTemplate is the Schema for the vmwarenodepooltemplates API
type VMwareNodePoolTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMwareNodePoolTemplateSpec   `json:"spec,omitempty"`
	Status VMwareNodePoolTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VMwareNodePoolTemplateList contains a list of VMwareNodePoolTemplate
type VMwareNodePoolTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMwareNodePoolTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VMwareNodePoolTemplate{}, &VMwareNodePoolTemplateList{})
}

// Condition types for VMwareNodePoolTemplate
const (
	// ConditionTypeReady indicates whether the template is ready
	ConditionTypeReady = "Ready"

	// ConditionTypeVSphereConnected indicates vSphere connectivity
	ConditionTypeVSphereConnected = "VSphereConnected"

	// ConditionTypeISOReady indicates the ISO is ready for use
	ConditionTypeISOReady = "ISOReady"

	// ConditionTypeNodePoolFound indicates the NodePool was found
	ConditionTypeNodePoolFound = "NodePoolFound"

	// ConditionTypeVMsCreated indicates VMs are being created
	ConditionTypeVMsCreated = "VMsCreated"

	// ConditionTypeResourcesValidated indicates all vSphere resources exist and are accessible
	ConditionTypeResourcesValidated = "ResourcesValidated"

	// ConditionTypeResourcesAvailable indicates sufficient resources are available for requested VMs
	ConditionTypeResourcesAvailable = "ResourcesAvailable"
)

// Helper methods for setting conditions
func (t *VMwareNodePoolTemplate) SetCondition(conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: t.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	// Find and update existing condition or append new one
	for i, existing := range t.Status.Conditions {
		if existing.Type == conditionType {
			if existing.Status != status || existing.Reason != reason || existing.Message != message {
				t.Status.Conditions[i] = condition
			} else {
				// Keep the original transition time if nothing changed
				return
			}
			return
		}
	}
	t.Status.Conditions = append(t.Status.Conditions, condition)
}

// GetCondition returns the condition with the given type
func (t *VMwareNodePoolTemplate) GetCondition(conditionType string) *metav1.Condition {
	for i := range t.Status.Conditions {
		if t.Status.Conditions[i].Type == conditionType {
			return &t.Status.Conditions[i]
		}
	}
	return nil
}
