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

	// ReadyReplicas is the number of VMs that are ready
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// VMStatus contains status information for each VM
	VMStatus []VMStatus `json:"vmStatus,omitempty"`

	// ISOUploaded indicates if the ISO has been uploaded (for url type)
	ISOUploaded bool `json:"isoUploaded,omitempty"`

	// ISOPath is the resolved path to the ISO in vSphere
	ISOPath string `json:"isoPath,omitempty"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vmtemplate;vmtpl
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredReplicas`
// +kubebuilder:printcolumn:name="Current",type=integer,JSONPath=`.status.currentReplicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
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
