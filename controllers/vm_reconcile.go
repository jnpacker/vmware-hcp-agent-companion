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
	"fmt"

	"github.com/go-logr/logr"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
	"github.com/example/vmware-hcp-agent-companion/pkg/vsphere"
)

// reconcileVMs ensures the correct number of VMs exist and are in the desired state
func (r *VMwareNodePoolTemplateReconciler) reconcileVMs(ctx context.Context, vClient *vsphere.Client, template *vmwarev1alpha1.VMwareNodePoolTemplate, log logr.Logger) error {
	desiredReplicas := template.Status.DesiredReplicas

	// Get current VMs
	currentVMs, err := r.getCurrentVMs(ctx, vClient, template, log)
	if err != nil {
		return fmt.Errorf("failed to get current VMs: %w", err)
	}

	currentReplicas := int32(len(currentVMs))
	template.Status.CurrentReplicas = currentReplicas

	// Calculate scaling
	diff := desiredReplicas - currentReplicas

	if diff > 0 {
		// Scale up - prepare ISO before creating VMs
		log.Info("Scaling up VMs", "current", currentReplicas, "desired", desiredReplicas, "toCreate", diff)
		r.Recorder.Eventf(template, corev1.EventTypeNormal, "ScalingUp", "Creating %d new VMs", diff)

		// Prepare ISO only when we need to create VMs
		isoPath, err := r.prepareISO(ctx, vClient, template, log)
		if err != nil {
			log.Error(err, "Failed to prepare ISO")
			r.Recorder.Event(template, corev1.EventTypeWarning, "ISOError", err.Error())
			return fmt.Errorf("failed to prepare ISO: %w", err)
		}

		// Update ISO path in status
		template.Status.ISOPath = isoPath

		for i := int32(0); i < diff; i++ {
			vmName := r.generateVMName(template, currentReplicas+i+1)
			if err := r.createVM(ctx, vClient, template, vmName, log); err != nil {
				log.Error(err, "Failed to create VM", "vmName", vmName)
				r.Recorder.Eventf(template, corev1.EventTypeWarning, "VMCreationFailed", "Failed to create VM %s: %v", vmName, err)
				// Continue creating other VMs
			} else {
				log.Info("Successfully created VM", "vmName", vmName)
				r.Recorder.Eventf(template, corev1.EventTypeNormal, "VMCreated", "Created VM %s", vmName)

				if r.Metrics != nil {
					r.Metrics.RecordVMCreated(template.Name, template.Namespace)
				}
			}
		}

		// Refresh VM list
		currentVMs, err = r.getCurrentVMs(ctx, vClient, template, log)
		if err != nil {
			return fmt.Errorf("failed to refresh VM list: %w", err)
		}

	} else if diff < 0 {
		// Scale down: delete Agents (finalizers will handle VM cleanup)
		toDelete := -diff
		log.Info("Scaling down", "current", currentReplicas, "desired", desiredReplicas, "toDelete", toDelete)
		r.Recorder.Eventf(template, corev1.EventTypeNormal, "ScalingDown", "Scaling down by %d VMs", toDelete)

		// Get the agent namespace
		agentNamespace, err := r.getAgentNamespace(ctx, template, log)
		if err != nil {
			log.Error(err, "Failed to get agent namespace")
			agentNamespace = template.Namespace
		}

		// Process VMs from the end (newest first)
		for i := int32(0); i < toDelete && i < int32(len(currentVMs)); i++ {
			vm := currentVMs[len(currentVMs)-1-int(i)]
			agentName := vm.AgentName

			if agentName == "" {
				// No Agent associated - skip this VM
				log.Info("No Agent associated with VM - skipping", "vm", vm.Name)
				continue
			}

			// Get the Agent
			agent := &unstructured.Unstructured{}
			agent.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "agent-install.openshift.io",
				Version: "v1beta1",
				Kind:    "Agent",
			})

			if err := r.Get(ctx, client.ObjectKey{Name: agentName, Namespace: agentNamespace}, agent); err != nil {
				if errors.IsNotFound(err) {
					log.Info("Agent not found - already deleted", "agent", agentName, "vm", vm.Name)
				} else {
					log.Error(err, "Failed to get Agent for scale-down", "agent", agentName)
				}
				continue
			}

			// Delete the Agent (finalizer will handle VM cleanup)
			log.Info("Deleting Agent (VM will be cleaned up by finalizer)", "agent", agentName, "vm", vm.Name)
			if err := r.Delete(ctx, agent); err != nil {
				if !errors.IsNotFound(err) {
					log.Error(err, "Failed to delete Agent", "agent", agentName)
					r.Recorder.Eventf(template, corev1.EventTypeWarning, "AgentDeletionFailed",
						"Failed to delete Agent %s: %v", agentName, err)
				}
			} else {
				log.Info("Deleted Agent", "agent", agentName)
				r.Recorder.Eventf(template, corev1.EventTypeNormal, "AgentDeleted",
					"Deleted Agent %s (VM cleanup via finalizer)", agentName)
			}
		}

		// Refresh VM list
		currentVMs, err = r.getCurrentVMs(ctx, vClient, template, log)
		if err != nil {
			return fmt.Errorf("failed to refresh VM list: %w", err)
		}
	}

	// Update status with current VM information
	vmStatusList := make([]vmwarev1alpha1.VMStatus, 0, len(currentVMs))
	readyCount := int32(0)

	for _, vmInfo := range currentVMs {
		vmStatus := vmwarev1alpha1.VMStatus{
			Name:               vmInfo.Name,
			UUID:               vmInfo.UUID,
			SerialNumber:       vmInfo.SerialNumber,
			PowerState:         vmInfo.PowerState,
			Phase:              vmInfo.Phase,
			Message:            vmInfo.Message,
			LastTransitionTime: metav1.Now(),
		}

		if vmInfo.PowerState == "poweredOn" {
			vmStatus.Phase = "Running"
			readyCount++
		} else {
			vmStatus.Phase = "NotReady"
		}

		vmStatusList = append(vmStatusList, vmStatus)
	}

	template.Status.VMStatus = vmStatusList
	template.Status.CurrentReplicas = int32(len(currentVMs))
	template.Status.ReadyReplicas = readyCount

	template.SetCondition(vmwarev1alpha1.ConditionTypeVMsCreated, metav1.ConditionTrue,
		"VMsReconciled", fmt.Sprintf("Successfully reconciled VMs: %d/%d ready", readyCount, desiredReplicas))

	return nil
}

// VMInfo holds information about a VM
type VMInfo struct {
	VM           *object.VirtualMachine
	Name         string
	UUID         string
	SerialNumber string
	PowerState   string
	Phase        string
	Message      string
	AgentName    string // Associated Agent name for cascade deletion
}

// getCurrentVMs returns the list of VMs managed by this template
func (r *VMwareNodePoolTemplateReconciler) getCurrentVMs(ctx context.Context, vClient *vsphere.Client, template *vmwarev1alpha1.VMwareNodePoolTemplate, log logr.Logger) ([]VMInfo, error) {
	folder, err := vClient.GetFolder(ctx, template.Spec.VMTemplate.Folder)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}

	vms, err := vClient.ListVMsInFolder(ctx, folder)
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	// Filter VMs that belong to this template
	prefix := r.getVMNamePrefix(template)
	var result []VMInfo

	for _, vm := range vms {
		info, err := vClient.GetVMInfo(ctx, vm)
		if err != nil {
			log.Error(err, "Failed to get VM info", "vm", vm.Name())
			continue
		}

		// Check if VM name starts with our prefix
		if len(info.Name) > len(prefix) && info.Name[:len(prefix)] == prefix {
			// Check template ownership tag if it exists
			templateOwner := vsphere.GetVMTag(info, "template")
			expectedOwner := fmt.Sprintf("%s/%s", template.Namespace, template.Name)

			// If tag exists and doesn't match, skip this VM
			if templateOwner != "" && templateOwner != expectedOwner {
				log.Info("Skipping VM - belongs to different template", "vm", info.Name, "expectedOwner", expectedOwner, "actualOwner", templateOwner)
				continue
			}

			// If no tag exists, log it but include the VM (it may be an existing VM from before tagging was added)
			if templateOwner == "" {
				log.V(1).Info("VM has no template ownership tag - assuming it belongs to this template", "vm", info.Name)
			}

			vmInfo := VMInfo{
				VM:           vm,
				Name:         info.Name,
				UUID:         info.Config.Uuid,                                   // From VMware API: config.uuid
				SerialNumber: vsphere.UUIDToVMwareSerialNumber(info.Config.Uuid), // For Agent matching: convert to VMware format
				PowerState:   string(info.Runtime.PowerState),
			}

			if info.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
				vmInfo.Phase = "Running"
			} else {
				vmInfo.Phase = "NotReady"
			}

			// Look up associated Agent name from template status
			for _, vmStatus := range template.Status.VMStatus {
				if vmStatus.UUID == vmInfo.UUID || vmStatus.Name == vmInfo.Name {
					vmInfo.AgentName = vmStatus.AgentName
					break
				}
			}

			result = append(result, vmInfo)
		}
	}

	return result, nil
}

// createVM creates a new VM with the given name
func (r *VMwareNodePoolTemplateReconciler) createVM(ctx context.Context, vClient *vsphere.Client, template *vmwarev1alpha1.VMwareNodePoolTemplate, vmName string, log logr.Logger) error {
	// Get resource pool
	resourcePool, err := vClient.GetResourcePool(ctx, template.Spec.VMTemplate.Cluster, template.Spec.VMTemplate.ResourcePool)
	if err != nil {
		return fmt.Errorf("failed to get resource pool: %w", err)
	}

	// Get folder
	folder, err := vClient.GetFolder(ctx, template.Spec.VMTemplate.Folder)
	if err != nil {
		return fmt.Errorf("failed to get folder: %w", err)
	}

	// Build VM spec
	// Get DiskEnableUUID from advanced config, default to true
	diskEnableUUID := true
	if template.Spec.VMTemplate.AdvancedConfig != nil && template.Spec.VMTemplate.AdvancedConfig.DiskEnableUUID != nil {
		diskEnableUUID = *template.Spec.VMTemplate.AdvancedConfig.DiskEnableUUID
	}

	// Get NestedVirtualization from advanced config, default to true
	nestedVirtualization := true
	if template.Spec.VMTemplate.AdvancedConfig != nil && template.Spec.VMTemplate.AdvancedConfig.NestedVirtualization != nil {
		nestedVirtualization = *template.Spec.VMTemplate.AdvancedConfig.NestedVirtualization
	}

	// Build ExtraConfig with ownership tag included (avoids separate reconfigure)
	extraConfig := make(map[string]string)
	if template.Spec.VMTemplate.ExtraConfig != nil {
		for k, v := range template.Spec.VMTemplate.ExtraConfig {
			extraConfig[k] = v
		}
	}
	// Add ownership tag (CRITICAL for safety - identifies which template owns this VM)
	templateOwner := fmt.Sprintf("%s/%s", template.Namespace, template.Name)
	extraConfig["guestinfo.vmware-hcp.template"] = templateOwner

	spec := vsphere.VMSpec{
		Name:                 vmName,
		NumCPUs:              template.Spec.VMTemplate.NumCPUs,
		MemoryMB:             template.Spec.VMTemplate.MemoryMB,
		DiskSizeGB:           template.Spec.VMTemplate.DiskSizeGB,
		GuestID:              template.Spec.VMTemplate.GuestID,
		Firmware:             template.Spec.VMTemplate.Firmware,
		NetworkName:          template.Spec.VMTemplate.Network,
		DatastoreName:        template.Spec.VMTemplate.Datastore,
		ISOPath:              template.Status.ISOPath,
		ExtraConfig:          extraConfig,
		DiskEnableUUID:       diskEnableUUID,
		NestedVirtualization: nestedVirtualization,
	}

	// Create VM with all settings in one operation (no separate reconfigure needed)
	vm, err := vClient.CreateVM(ctx, spec, resourcePool, folder)
	if err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	// Verify disk.enableUUID is set (helps with OpenShift Agent validation)
	if diskEnableUUID {
		vmInfo, err := vClient.GetVMInfo(ctx, vm)
		if err != nil {
			log.Error(err, "Failed to verify VM configuration", "vmName", vmName)
		} else {
			// Check if disk.enableUUID is in ExtraConfig
			diskUUIDFound := false
			if vmInfo.Config != nil && vmInfo.Config.ExtraConfig != nil {
				for _, opt := range vmInfo.Config.ExtraConfig {
					if optVal, ok := opt.(*types.OptionValue); ok {
						if optVal.Key == "disk.enableUUID" {
							diskUUIDFound = true
							log.Info("Verified disk.enableUUID setting", "vmName", vmName, "value", optVal.Value)
							break
						}
					}
				}
			}
			if !diskUUIDFound {
				log.Error(fmt.Errorf("disk.enableUUID not found in VM configuration"), "VM configuration issue", "vmName", vmName)
			}
		}
	}

	// Power on VM
	if err := vClient.PowerOnVM(ctx, vm); err != nil {
		return fmt.Errorf("failed to power on VM: %w", err)
	}

	return nil
}

// generateVMName generates a unique VM name with format "prefix-n0X"
// where prefix is the NamePrefix or template name, and X is the node counter (zero-padded)
func (r *VMwareNodePoolTemplateReconciler) generateVMName(template *vmwarev1alpha1.VMwareNodePoolTemplate, index int32) string {
	prefix := r.getVMNamePrefix(template)
	return fmt.Sprintf("%s-n%02d", prefix, index)
}

// getVMNamePrefix returns the prefix for VM names.
// If NamePrefix is specified in the template, uses that; otherwise uses the template name.
func (r *VMwareNodePoolTemplateReconciler) getVMNamePrefix(template *vmwarev1alpha1.VMwareNodePoolTemplate) string {
	if template.Spec.VMTemplate.NamePrefix != "" {
		return template.Spec.VMTemplate.NamePrefix
	}
	return template.Name
}
