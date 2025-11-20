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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Finalizer for Agent VM cleanup
	agentFinalizerName = "vmware.hcp.open-cluster-management.io/agent-vm-cleanup"
)

var (
	// Agent GVK for OpenShift Agent Installer
	agentGVK = schema.GroupVersionKind{
		Group:   "agent-install.openshift.io",
		Version: "v1beta1",
		Kind:    "Agent",
	}
	// HostedCluster GVK for HyperShift
	hostedClusterGVK = schema.GroupVersionKind{
		Group:   "hypershift.openshift.io",
		Version: "v1beta1",
		Kind:    "HostedCluster",
	}
)

// isAgentUnbindingOrUnbound checks if an Agent is unbinding or unbound from a cluster
func isAgentUnbindingOrUnbound(agent *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(agent.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}

	// Check if any condition has reason "Unbinding" or "Unbound"
	for _, condInterface := range conditions {
		cond, ok := condInterface.(map[string]interface{})
		if !ok {
			continue
		}

		reason, found, err := unstructured.NestedString(cond, "reason")
		if err != nil || !found {
			continue
		}

		if reason == "Unbinding" || reason == "Unbound" {
			return true
		}
	}

	return false
}

// isAgentBound checks if an Agent is bound to a cluster
func isAgentBound(agent *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(agent.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}

	// Check if any condition has reason "Bound"
	for _, condInterface := range conditions {
		cond, ok := condInterface.(map[string]interface{})
		if !ok {
			continue
		}

		reason, found, err := unstructured.NestedString(cond, "reason")
		if err != nil || !found {
			continue
		}

		if reason == "Bound" {
			return true
		}
	}

	return false
}

// getAgentNamespace retrieves the agent namespace from the HostedCluster resource
// The HostedCluster is in the same namespace as the VMwareNodePoolTemplate
func (r *VMwareNodePoolTemplateReconciler) getAgentNamespace(ctx context.Context, template *vmwarev1alpha1.VMwareNodePoolTemplate, log logr.Logger) (string, error) {
	if template.Spec.NodePoolRef == nil {
		// No NodePool reference, use template namespace
		return template.Namespace, nil
	}

	// HostedCluster is in the same namespace as template/NodePool
	hostedClusterNamespace := template.Namespace

	// Try to find HostedCluster by looking for one in the namespace
	hostedClusterList := &unstructured.UnstructuredList{}
	hostedClusterList.SetGroupVersionKind(hostedClusterGVK)

	listOpts := []client.ListOption{
		client.InNamespace(hostedClusterNamespace),
	}
	if err := r.List(ctx, hostedClusterList, listOpts...); err != nil {
		log.V(1).Error(err, "Failed to list HostedClusters", "namespace", hostedClusterNamespace)
		// Fall back to template namespace
		return hostedClusterNamespace, nil
	}

	// Get agentNamespace from the first HostedCluster found
	if len(hostedClusterList.Items) > 0 {
		hc := &hostedClusterList.Items[0]
		agentNamespace, found, err := unstructured.NestedString(hc.Object, "spec", "platform", "agent", "agentNamespace")
		if err != nil {
			log.V(1).Error(err, "Failed to get agentNamespace from HostedCluster", "hostedCluster", hc.GetName())
		}
		if found && agentNamespace != "" {
			log.V(1).Info("Found agentNamespace from HostedCluster", "namespace", agentNamespace)
			return agentNamespace, nil
		}
	}

	// Fall back to template namespace
	log.V(1).Info("No agentNamespace found in HostedCluster, using template namespace", "namespace", hostedClusterNamespace)
	return hostedClusterNamespace, nil
}

// reconcileAgents handles all Agent operations in a single pass:
// - Labels agents with NodePool and management labels
// - Sets hostnames to match VM names
// - Approves agents
// - Adds finalizers for VM cleanup
// This reduces 3 List() calls and up to 4 Update() calls per agent to 1 of each
func (r *VMwareNodePoolTemplateReconciler) reconcileAgents(ctx context.Context, template *vmwarev1alpha1.VMwareNodePoolTemplate, log logr.Logger) error {
	// List all Agents in the namespace (single List call for all operations)
	agentList := &unstructured.UnstructuredList{}
	agentList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "agent-install.openshift.io",
		Version: "v1beta1",
		Kind:    "AgentList",
	})

	// Get the agent namespace from HostedCluster
	agentNamespace, err := r.getAgentNamespace(ctx, template, log)
	if err != nil {
		log.Error(err, "Failed to get agent namespace")
		agentNamespace = template.Namespace
	}

	listOpts := []client.ListOption{
		client.InNamespace(agentNamespace),
	}
	if err := r.List(ctx, agentList, listOpts...); err != nil {
		return fmt.Errorf("failed to list Agents in namespace %s: %w", agentNamespace, err)
	}

	log.V(1).Info("Reconciling Agents", "count", len(agentList.Items), "namespace", agentNamespace)

	// Build VM lookup maps (single pass through VM status)
	vmByUUID := make(map[string]string)   // UUID -> VM Name
	vmBySerial := make(map[string]string) // SerialNumber -> VM Name
	vmUUIDs := make(map[string]bool)      // For quick lookup
	vmSerials := make(map[string]bool)    // For quick lookup

	for _, vmStatus := range template.Status.VMStatus {
		if vmStatus.UUID != "" {
			vmByUUID[vmStatus.UUID] = vmStatus.Name
			vmUUIDs[vmStatus.UUID] = true
		}
		if vmStatus.SerialNumber != "" {
			vmBySerial[vmStatus.SerialNumber] = vmStatus.Name
			vmSerials[vmStatus.SerialNumber] = true
		}
	}

	// Track statistics
	updatedCount := 0

	// Single iteration through all agents
	for i := range agentList.Items {
		agent := &agentList.Items[i]

		// Extract Agent UUID and Serial (single extraction per agent)
		agentUUID, found, err := unstructured.NestedString(agent.Object, "status", "inventory", "systemVendor", "uuid")
		if err != nil {
			log.V(1).Error(err, "Failed to get UUID from Agent", "agent", agent.GetName())
		}
		if !found || agentUUID == "" {
			// Try alternative path
			agentUUID, found, err = unstructured.NestedString(agent.Object, "status", "inventory", "system", "uuid")
			if err != nil || !found || agentUUID == "" {
				agentUUID = ""
			}
		}

		agentSerial, found, err := unstructured.NestedString(agent.Object, "status", "inventory", "systemVendor", "serialNumber")
		if err != nil {
			log.V(1).Error(err, "Failed to get serialNumber from Agent", "agent", agent.GetName())
		}
		if !found || agentSerial == "" {
			agentSerial = ""
		}

		log.V(1).Info("Processing Agent", "agent", agent.GetName(), "uuid", agentUUID, "serial", agentSerial)

		// Check if this Agent matches one of our VMs (single match check)
		isMatch := (agentUUID != "" && vmUUIDs[agentUUID]) || (agentSerial != "" && vmSerials[agentSerial])
		if !isMatch {
			log.V(1).Info("Agent does not match any VM, skipping", "agent", agent.GetName())
			continue
		}

		// Log the match
		if agentSerial != "" && vmSerials[agentSerial] {
			log.V(1).Info("Found matching Agent for VM by SerialNumber", "agent", agent.GetName(), "serial", agentSerial)
		} else if agentUUID != "" && vmUUIDs[agentUUID] {
			log.V(1).Info("Found matching Agent for VM by UUID", "agent", agent.GetName(), "uuid", agentUUID)
		}

		// CRITICAL: Add finalizer and management labels FIRST (before checking unbinding status)
		// This ensures the finalizer exists before we try to delete an unbinding Agent
		needsUpdate := false

		// Add finalizer if not present
		if !controllerutil.ContainsFinalizer(agent, agentFinalizerName) {
			controllerutil.AddFinalizer(agent, agentFinalizerName)
			needsUpdate = true
			log.Info("Adding finalizer to Agent", "agent", agent.GetName(), "finalizer", agentFinalizerName)
		}

		// Add management labels if not present
		labels := agent.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}

		managedByLabel := labels["vmware.hcp.open-cluster-management.io/managed-by"]
		if managedByLabel != template.Name {
			labels["vmware.hcp.open-cluster-management.io/managed-by"] = template.Name
			needsUpdate = true
		}

		// Update now if needed (before checking unbinding status)
		if needsUpdate {
			agent.SetLabels(labels)
			if err := r.Update(ctx, agent); err != nil {
				log.Error(err, "Failed to add finalizer and labels to Agent", "agent", agent.GetName())
				continue // Skip this agent, try again on next reconciliation
			}
			log.Info("Added finalizer and management labels to Agent", "agent", agent.GetName())
			// Don't continue here - proceed with the rest of the logic
		}

		// Now check if this Agent is unbinding/unbound AND was previously managed by us
		// We need to distinguish between:
		// 1. NEW agents with "Unbound" status (never bound, no binding label)
		// 2. MANAGED agents with "Unbinding"/"Unbound" status (were bound, have binding label)
		if isAgentUnbindingOrUnbound(agent) {
			labels := agent.GetLabels()

			// Check if this agent has our management markers
			hasFinalizer := controllerutil.ContainsFinalizer(agent, agentFinalizerName)
			managedBy := ""
			nodePoolLabel := ""
			bindingLabel := ""

			if labels != nil {
				managedBy = labels["vmware.hcp.open-cluster-management.io/managed-by"]
				nodePoolLabel = labels["vmware.hcp.open-cluster-management.io/nodepool"]
				bindingLabel = labels["vmware.hcp.open-cluster-management.io/binding"]
			}

			// Only delete if agent has our management markers AND was previously bound
			// The binding label is only added when agent is bound, so this distinguishes:
			// - Agents that were bound and are now unbinding (has binding label) -> DELETE
			// - New agents that are unbound but never bound (no binding label) -> KEEP
			if hasFinalizer && managedBy != "" && bindingLabel != "" {
				// Verify the agent was managed by this template
				if managedBy != template.Name {
					log.Info("Agent is unbinding but not managed by this template - skipping",
						"agent", agent.GetName(), "managedBy", managedBy, "template", template.Name)
					continue
				}

				// Verify the agent belonged to the referenced NodePool (if not in test mode)
				if !template.Spec.TestMode && template.Spec.NodePoolRef != nil {
					expectedNodePool := template.Spec.NodePoolRef.Name
					if nodePoolLabel != expectedNodePool {
						log.Info("Agent is unbinding but did not belong to our NodePool - skipping",
							"agent", agent.GetName(), "agentNodePool", nodePoolLabel, "expectedNodePool", expectedNodePool)
						continue
					}
				}

				log.Info("Agent is unbinding/unbound and was managed by this template - deleting to trigger VM cleanup",
					"agent", agent.GetName(), "nodepool", nodePoolLabel)

				if err := r.Delete(ctx, agent); err != nil {
					if !errors.IsNotFound(err) {
						log.Error(err, "Failed to delete unbinding Agent", "agent", agent.GetName())
						r.Recorder.Eventf(template, corev1.EventTypeWarning, "AgentDeletionFailed",
							"Failed to delete unbinding Agent %s: %v", agent.GetName(), err)
					}
				} else {
					log.Info("Deleted unbinding Agent - VM cleanup will proceed via finalizer", "agent", agent.GetName())
					r.Recorder.Eventf(template, corev1.EventTypeNormal, "UnbindingAgentDeleted",
						"Deleted unbinding Agent %s from NodePool %s (VM cleanup via finalizer)", agent.GetName(), nodePoolLabel)
				}
				// Skip further processing for this agent
				continue
			} else {
				// Agent is unbinding/unbound but doesn't have binding label
				// This is likely a NEW agent that was never bound - process it normally
				// (e.g., NodePool is being created or having issues, agent not yet bound)
				log.V(1).Info("Agent has unbinding/unbound status but no binding label - treating as new agent",
					"agent", agent.GetName(), "hasFinalizer", hasFinalizer, "managedBy", managedBy, "bindingLabel", bindingLabel)
				// Fall through to normal processing
			}
		}

		// Collect all updates in a single pass
		// Note: needsUpdate was declared earlier and may already be true from adding finalizer/labels
		// Reset it here since we'll check all fields again
		needsUpdate = false

		// 1. Handle Labels
		labels = agent.GetLabels() // Re-fetch labels in case they were updated earlier
		if labels == nil {
			labels = make(map[string]string)
		}

		// Apply user-specified labels (if configured)
		for key, value := range template.Spec.AgentLabelSelector {
			if labels[key] != value {
				labels[key] = value
				needsUpdate = true
			}
		}

		// Note: managed-by label was already added earlier (before unbinding check)
		// No need to add it again here

		// Add NodePool label if NodePool is referenced (always add for matching VMs)
		if template.Spec.NodePoolRef != nil {
			nodePoolLabel := "vmware.hcp.open-cluster-management.io/nodepool"
			nodePoolName := template.Spec.NodePoolRef.Name
			if labels[nodePoolLabel] != nodePoolName {
				labels[nodePoolLabel] = nodePoolName
				needsUpdate = true
			}

			// Handle binding label - this label tracks if an agent was ever bound to a NodePool
			// Once added, it persists even after unbinding, which helps distinguish:
			// 1. New agents that are unbound (no binding label) - DON'T delete
			// 2. Agents that were bound but are now unbinding (has binding label) - DELETE
			bindingLabel := "vmware.hcp.open-cluster-management.io/binding"
			existingBindingLabel := labels[bindingLabel]

			// Add or preserve binding label
			if existingBindingLabel == "" {
				// No binding label yet - only add it when agent becomes bound
				if isAgentBound(agent) {
					labels[bindingLabel] = nodePoolName
					needsUpdate = true
					log.Info("Agent is bound, adding binding label", "agent", agent.GetName(), "nodepool", nodePoolName)
				}
			} else {
				// Binding label already exists - preserve it (even if agent is now unbound)
				// Update only if it points to wrong NodePool
				if existingBindingLabel != nodePoolName {
					labels[bindingLabel] = nodePoolName
					needsUpdate = true
					log.V(1).Info("Updating binding label to match current NodePool", "agent", agent.GetName(), "nodepool", nodePoolName)
				}
				// Note: We NEVER remove this label, even when agent unbinds
				// This is critical for detecting which unbound agents should be deleted
			}
		}

		if needsUpdate {
			agent.SetLabels(labels)
		}

		// 2. Handle Hostname
		var vmName string
		if agentUUID != "" {
			if name, found := vmByUUID[agentUUID]; found {
				vmName = name
			}
		}
		if vmName == "" && agentSerial != "" {
			if name, found := vmBySerial[agentSerial]; found {
				vmName = name
			}
		}
		// Fall back to agent name if no VM match
		if vmName == "" {
			vmName = agent.GetName()
		}

		currentHostname, _, _ := unstructured.NestedString(agent.Object, "spec", "hostname")
		if currentHostname != vmName {
			if err := unstructured.SetNestedField(agent.Object, vmName, "spec", "hostname"); err != nil {
				log.Error(err, "Failed to set Agent hostname", "agent", agent.GetName(), "hostname", vmName)
				continue
			}
			needsUpdate = true
		}

		// 3. Handle Approval
		approved, found, err := unstructured.NestedBool(agent.Object, "spec", "approved")
		if err != nil {
			log.Error(err, "Failed to get approved status from Agent", "agent", agent.GetName())
		}
		if !found || !approved {
			if err := unstructured.SetNestedField(agent.Object, true, "spec", "approved"); err != nil {
				log.Error(err, "Failed to set Agent approved", "agent", agent.GetName())
				continue
			}
			needsUpdate = true
		}

		// Note: Finalizer was already added earlier (before unbinding check)
		// No need to add it again here

		// Add annotations with VM deletion metadata (so we can clean up even if template is deleted)
		annotations := agent.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		// Store critical info needed to delete the VM without the template
		if vmName != "" && annotations["vmware.hcp.open-cluster-management.io/vm-name"] != vmName {
			annotations["vmware.hcp.open-cluster-management.io/vm-name"] = vmName
			annotations["vmware.hcp.open-cluster-management.io/datacenter"] = template.Spec.VMTemplate.Datacenter
			annotations["vmware.hcp.open-cluster-management.io/folder"] = template.Spec.VMTemplate.Folder

			// Store credential reference
			credName := defaultCredentialSecretName
			if template.Spec.VSphereCredentials != nil && template.Spec.VSphereCredentials.Name != "" {
				credName = template.Spec.VSphereCredentials.Name
			}
			annotations["vmware.hcp.open-cluster-management.io/credential-secret"] = credName
			annotations["vmware.hcp.open-cluster-management.io/credential-namespace"] = template.Namespace

			agent.SetAnnotations(annotations)
			needsUpdate = true
		}

		// Single Update call per agent with all changes
		if needsUpdate {
			if err := r.Update(ctx, agent); err != nil {
				log.Error(err, "Failed to update Agent", "agent", agent.GetName())
				r.Recorder.Eventf(template, corev1.EventTypeWarning, "AgentUpdateFailed",
					"Failed to update Agent %s: %v", agent.GetName(), err)
				continue
			}

			log.Info("Successfully updated Agent", "agent", agent.GetName(), "hostname", vmName, "approved", true)
			r.Recorder.Eventf(template, corev1.EventTypeNormal, "AgentUpdated",
				"Successfully updated Agent %s", agent.GetName())
			updatedCount++

			// Update VM status with Agent name
			for i := range template.Status.VMStatus {
				if (agentUUID != "" && template.Status.VMStatus[i].UUID == agentUUID) ||
					(agentSerial != "" && template.Status.VMStatus[i].SerialNumber == agentSerial) {
					template.Status.VMStatus[i].AgentName = agent.GetName()
					break
				}
			}
		}
	}

	if updatedCount > 0 {
		log.Info("Updated Agents", "count", updatedCount)
	}

	return nil
}
