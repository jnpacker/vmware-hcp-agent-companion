/*
Copyright 2025 Contributors.
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
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	vmwarev1alpha1 "github.com/example/vmware-hcp-agent-companion/api/v1alpha1"
	"github.com/example/vmware-hcp-agent-companion/pkg/metrics"
	"github.com/example/vmware-hcp-agent-companion/pkg/vsphere"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName       = "vmware.hcp.open-cluster-management.io/finalizer"
	secretFinalizerName = "vmware.hcp.open-cluster-management.io/secret-in-use"

	// Default credential secret name
	defaultCredentialSecretName = "vsphere-credentials"

	// Requeue intervals
	requeueAfterError   = 1 * time.Minute
	requeueAfterSuccess = 5 * time.Minute
)

// VMwareNodePoolTemplateReconciler reconciles a VMwareNodePoolTemplate object
//
// IMPORTANT OPERATIONAL NOTES:
//
//  1. User-Defined Labels: If AgentLabelSelector is not defined correctly,
//     agents won't match the NodePool. Ensure the labels in AgentLabelSelector
//     are applied to discovered agents so they associate with the NodePool.
//
//  2. Missing NodePoolRef: In non-test mode, a nil NodePoolRef will cause
//     reconciliation to fail and VMs will be scaled to 0. Always provide
//     NodePoolRef when not in test mode.
//
//  3. Annotation Dependencies: VM cleanup relies on annotations being set
//     during VM creation. Specifically, the template ownership annotation
//     (guestinfo.vmware-hcp.template=namespace/template-name) must be present.
//     If annotations are missing, VM deletion may fail.
//
//  4. Label Matching: The controller applies both user-defined labels
//     (from AgentLabelSelector) AND management labels (vmware.hcp.open-cluster-management.io/managed-by).
//     Ensure there are no conflicts between these label sets.
type VMwareNodePoolTemplateReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Metrics  *metrics.Metrics
}

// +kubebuilder:rbac:groups=vmware.hcp.open-cluster-management.io,resources=vmwarenodepooltemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vmware.hcp.open-cluster-management.io,resources=vmwarenodepooltemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vmware.hcp.open-cluster-management.io,resources=vmwarenodepooltemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=hypershift.openshift.io,resources=nodepools,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=agent-install.openshift.io,resources=agents,verbs=get;list;watch;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *VMwareNodePoolTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Try to fetch as VMwareNodePoolTemplate first
	template := &vmwarev1alpha1.VMwareNodePoolTemplate{}
	err := r.Get(ctx, req.NamespacedName, template)

	// If not found as template, try as Agent
	if err != nil && errors.IsNotFound(err) {
		agent := &unstructured.Unstructured{}
		agent.SetGroupVersionKind(runtimeschema.GroupVersionKind{
			Group:   "agent-install.openshift.io",
			Version: "v1beta1",
			Kind:    "Agent",
		})
		if agentErr := r.Get(ctx, req.NamespacedName, agent); agentErr == nil {
			// Successfully fetched agent, handle its finalizer
			log.Info("Reconciling Agent (not Template)", "agent", agent.GetName(), "namespace", agent.GetNamespace())
			return r.reconcileAgentFinalizer(ctx, agent, log)
		}
		// Not found as either, return
		log.V(1).Info("Resource not found as Template or Agent", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	// This is a Template - proceed with normal reconciliation

	// Handle deletion
	if !template.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, template, log)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(template, finalizerName) {
		controllerutil.AddFinalizer(template, finalizerName)
		if err := r.Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile the template
	return r.reconcileNormal(ctx, template, log)
}

func (r *VMwareNodePoolTemplateReconciler) reconcileNormal(ctx context.Context, template *vmwarev1alpha1.VMwareNodePoolTemplate, log logr.Logger) (ctrl.Result, error) {
	// Update observed generation
	template.Status.ObservedGeneration = template.Generation

	// Get vSphere credentials
	credNamespace := template.Namespace
	credName := defaultCredentialSecretName
	if template.Spec.VSphereCredentials != nil {
		if template.Spec.VSphereCredentials.Name != "" {
			credName = template.Spec.VSphereCredentials.Name
		}

	}

	creds, err := vsphere.LoadCredentialsFromSecret(ctx, r.Client, credNamespace, credName)
	if err != nil {
		log.Error(err, "Failed to load vSphere credentials")
		template.SetCondition(vmwarev1alpha1.ConditionTypeVSphereConnected, metav1.ConditionFalse,
			"CredentialsNotFound", fmt.Sprintf("Failed to load credentials: %v", err))
		r.Recorder.Event(template, corev1.EventTypeWarning, "CredentialsError", err.Error())
		if err := r.Status().Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// Create vSphere client
	vClient, err := vsphere.NewClient(ctx, creds)
	if err != nil {
		log.Error(err, "Failed to create vSphere client")
		template.SetCondition(vmwarev1alpha1.ConditionTypeVSphereConnected, metav1.ConditionFalse,
			"ConnectionFailed", fmt.Sprintf("Failed to connect: %v", err))
		r.Recorder.Event(template, corev1.EventTypeWarning, "VSphereConnectionError", err.Error())
		if err := r.Status().Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}
	defer vClient.Close(ctx)

	// Set datacenter
	if err := vClient.SetDatacenter(ctx, template.Spec.VMTemplate.Datacenter); err != nil {
		log.Error(err, "Failed to set datacenter")
		template.SetCondition(vmwarev1alpha1.ConditionTypeVSphereConnected, metav1.ConditionFalse,
			"DatacenterNotFound", fmt.Sprintf("Failed to find datacenter: %v", err))
		r.Recorder.Event(template, corev1.EventTypeWarning, "DatacenterError", err.Error())
		if err := r.Status().Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// Validate connection
	if err := vClient.Validate(ctx); err != nil {
		log.Error(err, "Failed to validate vSphere connection")
		template.SetCondition(vmwarev1alpha1.ConditionTypeVSphereConnected, metav1.ConditionFalse,
			"ValidationFailed", fmt.Sprintf("Connection validation failed: %v", err))
		if err := r.Status().Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	template.SetCondition(vmwarev1alpha1.ConditionTypeVSphereConnected, metav1.ConditionTrue,
		"Connected", "Successfully connected to vSphere")

	// Poll for resource utilization (event-driven + periodic)
	// This validates resources exist and tracks capacity
	if err := r.reconcileResourceUtilization(ctx, vClient, template, log); err != nil {
		// Don't fail reconciliation on resource polling errors, just log
		// The status will show the last successful poll
		log.Error(err, "Failed to poll resource utilization, continuing with reconciliation")
	}

	// Determine desired replica count
	desiredReplicas := int32(0)
	if template.Spec.TestMode {
		// Test mode: use replicas from spec
		if template.Spec.Replicas != nil {
			desiredReplicas = *template.Spec.Replicas
		}
		template.SetCondition(vmwarev1alpha1.ConditionTypeNodePoolFound, metav1.ConditionTrue,
			"TestMode", "Running in test mode")
	} else {
		// Normal mode: get replicas from NodePool
		if template.Spec.NodePoolRef == nil {
			err := fmt.Errorf("nodePoolRef is required when testMode is false")
			log.Error(err, "NodePool reference missing - scaling to 0")
			desiredReplicas = 0
			template.Status.DesiredReplicas = desiredReplicas
			template.SetCondition(vmwarev1alpha1.ConditionTypeNodePoolFound, metav1.ConditionFalse,
				"NodePoolRefMissing", "NodePool reference is required - scaling to 0")
			if err := r.Status().Update(ctx, template); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: requeueAfterError}, nil
		}

		// Get the NodePool object
		nodePool, err := r.getNodePool(ctx, template, log)
		if err != nil {
			// Check if NodePool was deleted
			if errors.IsNotFound(err) {
				log.Info("NodePool deleted - scaling down to 0 to clean up all Agents and VMs")
				desiredReplicas = 0
				template.Status.DesiredReplicas = desiredReplicas
				template.SetCondition(vmwarev1alpha1.ConditionTypeNodePoolFound, metav1.ConditionFalse,
					"NodePoolDeleted", "NodePool was deleted - cleaning up all resources")
				r.Recorder.Event(template, corev1.EventTypeWarning, "NodePoolDeleted",
					"NodePool was deleted - cleaning up all Agents and VMs")
			} else {
				// Other errors - requeue
				log.Error(err, "Failed to get NodePool")
				template.SetCondition(vmwarev1alpha1.ConditionTypeNodePoolFound, metav1.ConditionFalse,
					"NodePoolNotFound", fmt.Sprintf("Failed to get NodePool: %v", err))
				r.Recorder.Event(template, corev1.EventTypeWarning, "NodePoolError", err.Error())
				if err := r.Status().Update(ctx, template); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: requeueAfterError}, nil
			}
		} else {
			// NodePool exists - check if it's being deleted
			if !nodePool.GetDeletionTimestamp().IsZero() {
				log.Info("NodePool is being deleted - scaling down to 0 to clean up all Agents and VMs")
				desiredReplicas = 0
				template.SetCondition(vmwarev1alpha1.ConditionTypeNodePoolFound, metav1.ConditionFalse,
					"NodePoolDeleting", "NodePool is being deleted - cleaning up all resources")
			} else {
				// NodePool is healthy - ensure matchLabels are configured
				if err := r.ensureNodePoolMatchLabels(ctx, nodePool, template, log); err != nil {
					log.Error(err, "Failed to ensure NodePool matchLabels", "error", err)
					// Don't fail reconciliation, just log the error
				}

				// Get replicas
				replicas, found, err := unstructured.NestedInt64(nodePool.Object, "spec", "replicas")
				if err != nil || !found {
					log.Error(err, "Failed to get replicas from NodePool")
					return ctrl.Result{RequeueAfter: requeueAfterError}, nil
				}
				desiredReplicas = int32(replicas)
				template.SetCondition(vmwarev1alpha1.ConditionTypeNodePoolFound, metav1.ConditionTrue,
					"NodePoolFound", fmt.Sprintf("NodePool found with %d replicas", replicas))
			}
		}
	}

	template.Status.DesiredReplicas = desiredReplicas

	// Update annotation to track NodePool connection
	if err := r.setNodePoolRefAnnotation(ctx, template, log); err != nil {
		log.Error(err, "Failed to update NodePool reference annotation, but continuing reconciliation")
		// Don't fail reconciliation on annotation errors
	}

	// Reconcile VMs
	if err := r.reconcileVMs(ctx, vClient, template, log); err != nil {
		log.Error(err, "Failed to reconcile VMs")
		template.SetCondition(vmwarev1alpha1.ConditionTypeVMsCreated, metav1.ConditionFalse,
			"ReconciliationFailed", fmt.Sprintf("Failed to reconcile VMs: %v", err))
		r.Recorder.Event(template, corev1.EventTypeWarning, "VMReconciliationError", err.Error())
		if err := r.Status().Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// Reconcile Agents (labels, hostnames, approval, finalizers) in a single pass
	if err := r.reconcileAgents(ctx, template, log); err != nil {
		log.Error(err, "Failed to reconcile Agents", "error", err)
		// Don't fail reconciliation on Agent errors, just log
	}

	// Update overall ready condition
	if template.Status.ReadyReplicas == template.Status.DesiredReplicas {
		template.SetCondition(vmwarev1alpha1.ConditionTypeReady, metav1.ConditionTrue,
			"AllReady", fmt.Sprintf("All %d VMs are ready", template.Status.DesiredReplicas))
	} else {
		template.SetCondition(vmwarev1alpha1.ConditionTypeReady, metav1.ConditionFalse,
			"NotAllReady", fmt.Sprintf("%d/%d VMs are ready", template.Status.ReadyReplicas, template.Status.DesiredReplicas))
	}

	// Update status
	if err := r.Status().Update(ctx, template); err != nil {
		return ctrl.Result{}, err
	}

	// Record metrics
	if r.Metrics != nil {
		r.Metrics.RecordVMCount(template.Name, template.Namespace, template.Status.CurrentReplicas)
		r.Metrics.RecordVMReady(template.Name, template.Namespace, template.Status.ReadyReplicas)
	}

	return ctrl.Result{RequeueAfter: requeueAfterSuccess}, nil
}

func (r *VMwareNodePoolTemplateReconciler) reconcileDelete(ctx context.Context, template *vmwarev1alpha1.VMwareNodePoolTemplate, log logr.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(template, finalizerName) {
		return ctrl.Result{}, nil
	}

	log.Info("Deleting VMwareNodePoolTemplate - scaling down to 0")
	r.Recorder.Event(template, corev1.EventTypeNormal, "Deleting", "Scaling down to 0 (VMs will be cleaned up by Agent finalizers)")

	// Set desired replicas to 0
	template.Status.DesiredReplicas = 0

	// Get vSphere credentials
	credNamespace := template.Namespace
	credName := defaultCredentialSecretName
	if template.Spec.VSphereCredentials != nil {
		if template.Spec.VSphereCredentials.Name != "" {
			credName = template.Spec.VSphereCredentials.Name
		}
	}

	creds, err := vsphere.LoadCredentialsFromSecret(ctx, r.Client, credNamespace, credName)
	if err != nil {
		log.Error(err, "Failed to load vSphere credentials for deletion")
		// If we can't connect to vSphere, check if all VMs are already gone
		if template.Status.CurrentReplicas == 0 {
			log.Info("No VMs remaining, removing finalizer")
			controllerutil.RemoveFinalizer(template, finalizerName)
			return ctrl.Result{}, r.Update(ctx, template)
		}
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	vClient, err := vsphere.NewClient(ctx, creds)
	if err != nil {
		log.Error(err, "Failed to create vSphere client for deletion")
		// If we can't connect to vSphere, check if all VMs are already gone
		if template.Status.CurrentReplicas == 0 {
			log.Info("No VMs remaining, removing finalizer")
			controllerutil.RemoveFinalizer(template, finalizerName)
			return ctrl.Result{}, r.Update(ctx, template)
		}
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}
	defer vClient.Close(ctx)

	if err := vClient.SetDatacenter(ctx, template.Spec.VMTemplate.Datacenter); err != nil {
		log.Error(err, "Failed to set datacenter for deletion")
		// If we can't connect to vSphere, check if all VMs are already gone
		if template.Status.CurrentReplicas == 0 {
			log.Info("No VMs remaining, removing finalizer")
			controllerutil.RemoveFinalizer(template, finalizerName)
			return ctrl.Result{}, r.Update(ctx, template)
		}
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// Use normal reconciliation to scale down to 0
	if err := r.reconcileVMs(ctx, vClient, template, log); err != nil {
		log.Error(err, "Failed to reconcile VMs during deletion")
		if err := r.Status().Update(ctx, template); err != nil {
			log.Error(err, "Failed to update status during deletion")
		}
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// Update status
	if err := r.Status().Update(ctx, template); err != nil {
		log.Error(err, "Failed to update status during deletion")
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// Check if all VMs are gone
	if template.Status.CurrentReplicas > 0 {
		log.Info("Waiting for VMs to be deleted", "remaining", template.Status.CurrentReplicas)
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// All VMs have been deleted - remove finalizer from secret
	log.Info("All VMs deleted, removing secret finalizer")
	if err := r.removeSecretFinalizer(ctx, credNamespace, credName, log); err != nil {
		log.Error(err, "Failed to remove finalizer from secret, but continuing with template deletion")
		// Don't fail the deletion, just log the error
	}

	// Remove finalizer from template
	log.Info("Removing finalizer from template")
	controllerutil.RemoveFinalizer(template, finalizerName)
	if err := r.Update(ctx, template); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("VMwareNodePoolTemplate deletion complete")
	return ctrl.Result{}, nil
}

// reconcileAgentFinalizer handles the finalizer when an Agent with our finalizer is deleted
func (r *VMwareNodePoolTemplateReconciler) reconcileAgentFinalizer(ctx context.Context, agent *unstructured.Unstructured, log logr.Logger) (ctrl.Result, error) {
	agentName := agent.GetName()
	agentNamespace := agent.GetNamespace()

	log.Info("Agent finalizer reconciliation started", "agent", agentName, "namespace", agentNamespace, "deletionTimestamp", agent.GetDeletionTimestamp())

	// Check if this agent has our finalizer and is being deleted
	hasFinalizer := false
	for _, f := range agent.GetFinalizers() {
		if f == "vmware.hcp.open-cluster-management.io/agent-vm-cleanup" {
			hasFinalizer = true
			break
		}
	}

	if !hasFinalizer {
		log.Info("Agent does not have our finalizer, skipping", "agent", agentName, "finalizers", agent.GetFinalizers())
		return ctrl.Result{}, nil
	}

	// Check if agent is being deleted
	if agent.GetDeletionTimestamp() == nil {
		log.Info("Agent is not being deleted, skipping finalizer handling", "agent", agentName)
		return ctrl.Result{}, nil
	}

	log.Info("Handling finalizer for deleted Agent", "agent", agentName)

	// Get management label to find the template
	labels := agent.GetLabels()
	templateName, ok := labels["vmware.hcp.open-cluster-management.io/managed-by"]
	if !ok {
		// Not managed by us, just remove the finalizer
		controllerutil.RemoveFinalizer(agent, "vmware.hcp.open-cluster-management.io/agent-vm-cleanup")
		return ctrl.Result{}, r.Update(ctx, agent)
	}

	// Try to find the VMwareNodePoolTemplate
	template := &vmwarev1alpha1.VMwareNodePoolTemplate{}
	templateNotFound := false
	if err := r.Get(ctx, client.ObjectKey{Name: templateName, Namespace: agentNamespace}, template); err != nil {
		if errors.IsNotFound(err) {
			log.Info("VMwareNodePoolTemplate not found, will attempt VM cleanup using Agent annotations", "template", templateName)
			templateNotFound = true
			// Don't return yet - try to clean up VM using annotations
		} else {
			log.Error(err, "Failed to get VMwareNodePoolTemplate for finalizer cleanup")
			return ctrl.Result{}, err
		}
	}

	// Determine VM name and vSphere connection info
	var vmName, datacenter, credNamespace, credName string

	if templateNotFound {
		// Template was deleted - use Agent annotations
		annotations := agent.GetAnnotations()
		if annotations != nil {
			vmName = annotations["vmware.hcp.open-cluster-management.io/vm-name"]
			datacenter = annotations["vmware.hcp.open-cluster-management.io/datacenter"]
			credNamespace = annotations["vmware.hcp.open-cluster-management.io/credential-namespace"]
			credName = annotations["vmware.hcp.open-cluster-management.io/credential-secret"]
		}

		if vmName == "" {
			log.Info("No VM name found in Agent annotations, cannot clean up VM", "agent", agentName)
			// Remove finalizer and continue
			controllerutil.RemoveFinalizer(agent, "vmware.hcp.open-cluster-management.io/agent-vm-cleanup")
			return ctrl.Result{}, r.Update(ctx, agent)
		}

		log.Info("Using Agent annotations for VM cleanup", "vm", vmName, "datacenter", datacenter)
	} else {
		// Template exists - use template status and spec
		var vmToDelete *vmwarev1alpha1.VMStatus
		for i := range template.Status.VMStatus {
			if template.Status.VMStatus[i].AgentName == agentName {
				vmToDelete = &template.Status.VMStatus[i]
				break
			}
		}

		if vmToDelete == nil {
			log.V(1).Info("No VM found in template status for agent", "agent", agentName)
			// Remove finalizer and continue
			controllerutil.RemoveFinalizer(agent, "vmware.hcp.open-cluster-management.io/agent-vm-cleanup")
			return ctrl.Result{}, r.Update(ctx, agent)
		}

		vmName = vmToDelete.Name
		datacenter = template.Spec.VMTemplate.Datacenter
		credNamespace = template.Namespace
		credName = defaultCredentialSecretName
		if template.Spec.VSphereCredentials != nil && template.Spec.VSphereCredentials.Name != "" {
			credName = template.Spec.VSphereCredentials.Name
		}
	}

	// Delete the VM (handles both manual Agent deletion and cleanup after template/scale-down deletion)
	log.Info("Deleting VM for agent", "vm", vmName, "agent", agentName)

	vmDeleted := false
	creds, err := vsphere.LoadCredentialsFromSecret(ctx, r.Client, credNamespace, credName)
	if err != nil {
		log.Error(err, "Failed to load vSphere credentials for VM cleanup - will remove finalizer anyway", "credNamespace", credNamespace, "credName", credName)
	} else {
		vClient, err := vsphere.NewClient(ctx, creds)
		if err != nil {
			log.Error(err, "Failed to create vSphere client for VM cleanup - will remove finalizer anyway")
		} else {
			defer vClient.Close(ctx)

			if err := vClient.SetDatacenter(ctx, datacenter); err != nil {
				log.Error(err, "Failed to set datacenter for VM cleanup - will remove finalizer anyway", "datacenter", datacenter)
			} else {
				// Find and delete the VM
				vm, err := vClient.FindVMByName(ctx, vmName)
				if err != nil {
					log.Info("VM not found (may already be deleted)", "vm", vmName, "agent", agentName)
					vmDeleted = true // Treat as deleted if not found
				} else {
					// Validate VM ownership before deletion
					if !templateNotFound {
						expectedOwner := fmt.Sprintf("%s/%s", template.Namespace, template.Name)
						if err := vClient.ValidateVMOwnership(ctx, vm, expectedOwner); err != nil {
							log.Error(err, "VM ownership validation failed - refusing to delete VM",
								"vm", vmName,
								"agent", agentName,
								"expectedOwner", expectedOwner,
								"template", template.Name,
								"namespace", template.Namespace,
								"details", "Check VM ExtraConfig in vCenter (VM Options > Advanced > Configuration Parameters > guestinfo.vmware-hcp.template)")
							r.Recorder.Eventf(template, corev1.EventTypeWarning, "VMOwnershipValidationFailed",
								"Refusing to delete VM %s for agent %s: %v. Check VM ownership tag in vCenter: VM Options > Advanced > Configuration Parameters > guestinfo.vmware-hcp.template",
								vmName, agentName, err)
							// Don't delete the VM, but remove finalizer to prevent blocking
							// The VM will remain in vSphere for manual investigation
						} else {
							log.Info("VM ownership validated - proceeding with deletion",
								"vm", vmName,
								"agent", agentName,
								"owner", expectedOwner,
								"tag", "guestinfo.vmware-hcp.template")
							if err := vClient.DestroyVM(ctx, vm); err != nil {
								log.Error(err, "Failed to destroy VM - will remove finalizer anyway", "vm", vmName)
							} else {
								log.Info("Successfully deleted VM for agent", "vm", vmName, "agent", agentName)
								vmDeleted = true
								r.Recorder.Eventf(template, corev1.EventTypeNormal, "VMCleanedUp", "Cleaned up VM %s for deleted agent %s", vmName, agentName)
								if r.Metrics != nil {
									r.Metrics.RecordVMDeleted(template.Name, template.Namespace)
								}
							}
						}
					} else {
						// Template was deleted - we can't validate ownership, but log warning
						log.Info("Template was deleted - skipping ownership validation for VM cleanup",
							"vm", vmName,
							"agent", agentName,
							"warning", "Unable to validate VM ownership since template is deleted")
						if err := vClient.DestroyVM(ctx, vm); err != nil {
							log.Error(err, "Failed to destroy VM - will remove finalizer anyway", "vm", vmName)
						} else {
							log.Info("Successfully deleted VM for agent (template was deleted)", "vm", vmName, "agent", agentName)
							vmDeleted = true
						}
					}
				}
			}
		}
	}

	if !vmDeleted {
		log.Info("VM deletion was not confirmed successful, but removing finalizer to prevent blocking", "agent", agentName, "vm", vmName)
	}

	// Update template status if template still exists
	if !templateNotFound {
		newVMStatus := make([]vmwarev1alpha1.VMStatus, 0, len(template.Status.VMStatus))
		for i := range template.Status.VMStatus {
			if template.Status.VMStatus[i].AgentName != agentName {
				newVMStatus = append(newVMStatus, template.Status.VMStatus[i])
			}
		}
		template.Status.VMStatus = newVMStatus
		template.Status.CurrentReplicas = int32(len(newVMStatus))
		if err := r.Status().Update(ctx, template); err != nil {
			log.Error(err, "Failed to update template status after VM deletion")
		}
	}

	// Remove the finalizer
	log.Info("Removing finalizer from Agent", "agent", agentName, "finalizer", "vmware.hcp.open-cluster-management.io/agent-vm-cleanup")
	controllerutil.RemoveFinalizer(agent, "vmware.hcp.open-cluster-management.io/agent-vm-cleanup")

	log.Info("Attempting to update Agent to remove finalizer", "agent", agentName)
	if err := r.Update(ctx, agent); err != nil {
		log.Error(err, "Failed to update Agent to remove finalizer - will retry", "agent", agentName)
		return ctrl.Result{}, err
	}

	log.Info("Successfully removed finalizer from Agent", "agent", agentName)
	return ctrl.Result{}, nil
}

// mapAgentToTemplates returns reconciliation requests for templates that manage the agent
func (r *VMwareNodePoolTemplateReconciler) mapAgentToTemplates(ctx context.Context, obj client.Object) []reconcile.Request {
	agent := obj.(*unstructured.Unstructured)
	labels := agent.GetLabels()

	// Check if this agent is managed by us
	templateName, ok := labels["vmware.hcp.open-cluster-management.io/managed-by"]
	if !ok {
		return nil
	}

	// Return a reconciliation request for the template
	return []reconcile.Request{
		{
			NamespacedName: client.ObjectKey{
				Name:      templateName,
				Namespace: agent.GetNamespace(),
			},
		},
	}
}

// mapNodePoolToTemplates returns reconciliation requests for templates that reference the NodePool
func (r *VMwareNodePoolTemplateReconciler) mapNodePoolToTemplates(ctx context.Context, obj client.Object) []reconcile.Request {
	nodePool := obj.(*unstructured.Unstructured)
	nodePoolName := nodePool.GetName()
	nodePoolNamespace := nodePool.GetNamespace()

	// Find all VMwareNodePoolTemplates in the same namespace that reference this NodePool
	templateList := &vmwarev1alpha1.VMwareNodePoolTemplateList{}
	if err := r.List(ctx, templateList, client.InNamespace(nodePoolNamespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, template := range templateList.Items {
		// Check if this template references the NodePool
		if template.Spec.NodePoolRef != nil && template.Spec.NodePoolRef.Name == nodePoolName {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      template.Name,
					Namespace: template.Namespace,
				},
			})
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *VMwareNodePoolTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a builder for the main VMwareNodePoolTemplate watch
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&vmwarev1alpha1.VMwareNodePoolTemplate{})

	// Create an unstructured scheme for Agent resources
	agentGVK := unstructured.Unstructured{}
	agentGVK.SetGroupVersionKind(runtimeschema.GroupVersionKind{
		Group:   "agent-install.openshift.io",
		Version: "v1beta1",
		Kind:    "Agent",
	})

	// Watch for Agent changes and trigger reconciliation for:
	// 1. The Agent itself (for finalizer handling when deleted)
	// 2. The Template (for status updates)
	builder = builder.Watches(
		&agentGVK,
		&handler.EnqueueRequestForObject{},
	).Watches(
		&agentGVK,
		handler.EnqueueRequestsFromMapFunc(r.mapAgentToTemplates),
	)

	// Create an unstructured scheme for NodePool resources
	nodePoolGVKUnstructured := unstructured.Unstructured{}
	nodePoolGVKUnstructured.SetGroupVersionKind(runtimeschema.GroupVersionKind{
		Group:   "hypershift.openshift.io",
		Version: "v1beta1",
		Kind:    "NodePool",
	})

	// Watch for NodePool changes (especially deletions)
	// This triggers reconciliation for Templates that reference the NodePool
	builder = builder.Watches(
		&nodePoolGVKUnstructured,
		handler.EnqueueRequestsFromMapFunc(r.mapNodePoolToTemplates),
	)

	return builder.Complete(r)
}

// ensureSecretFinalizer adds the finalizer to the vSphere credentials secret
func (r *VMwareNodePoolTemplateReconciler) ensureSecretFinalizer(ctx context.Context, credNamespace, credName string, log logr.Logger) error {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: credNamespace,
		Name:      credName,
	}, secret); err != nil {
		return fmt.Errorf("failed to get secret for finalizer: %w", err)
	}

	// AddFinalizer only adds if not present and returns true if it made a change
	if controllerutil.AddFinalizer(secret, secretFinalizerName) {
		if err := r.Update(ctx, secret); err != nil {
			return fmt.Errorf("failed to add finalizer to secret: %w", err)
		}
		log.Info("Added finalizer to vSphere credentials secret", "secret", credName, "namespace", credNamespace)
	} else {
		log.V(1).Info("Secret already has finalizer", "secret", credName, "namespace", credNamespace)
	}

	return nil
}

// removeSecretFinalizer removes the finalizer from the vSphere credentials secret
func (r *VMwareNodePoolTemplateReconciler) removeSecretFinalizer(ctx context.Context, credNamespace, credName string, log logr.Logger) error {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: credNamespace,
		Name:      credName,
	}, secret); err != nil {
		// If secret doesn't exist, nothing to do
		if errors.IsNotFound(err) {
			log.V(1).Info("Secret not found, cannot remove finalizer", "secret", credName, "namespace", credNamespace)
			return nil
		}
		return fmt.Errorf("failed to get secret for finalizer removal: %w", err)
	}

	// RemoveFinalizer only removes if present and returns true if it made a change
	if controllerutil.RemoveFinalizer(secret, secretFinalizerName) {
		if err := r.Update(ctx, secret); err != nil {
			return fmt.Errorf("failed to remove finalizer from secret: %w", err)
		}
		log.Info("Removed finalizer from vSphere credentials secret", "secret", credName, "namespace", credNamespace)
	} else {
		log.V(1).Info("Secret does not have finalizer", "secret", credName, "namespace", credNamespace)
	}

	return nil
}

// reconcileResourceUtilization queries vSphere for resource utilization and updates status
// Returns true if resources should be polled (based on event-driven + periodic logic)
func (r *VMwareNodePoolTemplateReconciler) shouldPollResources(template *vmwarev1alpha1.VMwareNodePoolTemplate) bool {
	// Always poll if we've never polled before
	if template.Status.ResourceUtilization == nil {
		return true
	}

	// Get polling interval (default 5 minutes)
	pollingInterval := 5 * time.Minute
	if template.Spec.ResourcePollingInterval != nil {
		pollingInterval = template.Spec.ResourcePollingInterval.Duration
		// Enforce minimum of 1 minute
		if pollingInterval < time.Minute {
			pollingInterval = time.Minute
		}
	}

	// Poll if enough time has passed since last update
	timeSinceLastPoll := time.Since(template.Status.ResourceUtilization.LastUpdated.Time)
	return timeSinceLastPoll >= pollingInterval
}

// reconcileResourceUtilization queries vSphere for resource utilization and validation
func (r *VMwareNodePoolTemplateReconciler) reconcileResourceUtilization(
	ctx context.Context,
	vClient *vsphere.Client,
	template *vmwarev1alpha1.VMwareNodePoolTemplate,
	log logr.Logger,
) error {
	// Check if we should poll resources
	if !r.shouldPollResources(template) {
		log.V(1).Info("Skipping resource polling - not yet time",
			"lastUpdated", template.Status.ResourceUtilization.LastUpdated.Time)
		return nil
	}

	log.Info("Polling vSphere for resource utilization")

	// Get resource utilization and validation
	utilization, validation, err := vClient.GetResourceUtilization(ctx, template.Spec.VMTemplate, template.Spec.UseEffectiveCapacity)
	if err != nil {
		log.Error(err, "Failed to get resource utilization")
		template.SetCondition(vmwarev1alpha1.ConditionTypeResourcesValidated, metav1.ConditionFalse,
			"QueryFailed", fmt.Sprintf("Failed to query resources: %v", err))
		r.Recorder.Event(template, corev1.EventTypeWarning, "ResourceQueryError", err.Error())
		return err
	}

	// Update timestamp
	utilization.LastUpdated = metav1.Now()

	// Update status
	template.Status.ResourceUtilization = utilization
	template.Status.ResourceValidation = validation

	// Set resource validation condition
	allValid := validation.Datacenter.Exists && validation.Datastore.Exists && validation.Network.Exists
	if validation.Cluster != nil {
		allValid = allValid && validation.Cluster.Exists
	}
	if validation.ResourcePool != nil {
		allValid = allValid && validation.ResourcePool.Exists
	}
	if validation.Folder != nil && !validation.Folder.Exists {
		// Folder is optional and can be created, so just warn
		log.Info("Folder does not exist but will be created", "folder", template.Spec.VMTemplate.Folder)
	}

	if allValid {
		template.SetCondition(vmwarev1alpha1.ConditionTypeResourcesValidated, metav1.ConditionTrue,
			"ResourcesValidated", "All vSphere resources exist and are accessible")
	} else {
		reasons := []string{}
		if !validation.Datacenter.Exists {
			reasons = append(reasons, fmt.Sprintf("Datacenter: %s", validation.Datacenter.Message))
		}
		if !validation.Datastore.Exists {
			reasons = append(reasons, fmt.Sprintf("Datastore: %s", validation.Datastore.Message))
		}
		if !validation.Network.Exists {
			reasons = append(reasons, fmt.Sprintf("Network: %s", validation.Network.Message))
		}
		if validation.Cluster != nil && !validation.Cluster.Exists {
			reasons = append(reasons, fmt.Sprintf("Cluster: %s", validation.Cluster.Message))
		}
		if validation.ResourcePool != nil && !validation.ResourcePool.Exists {
			reasons = append(reasons, fmt.Sprintf("ResourcePool: %s", validation.ResourcePool.Message))
		}

		template.SetCondition(vmwarev1alpha1.ConditionTypeResourcesValidated, metav1.ConditionFalse,
			"ValidationFailed", fmt.Sprintf("Some resources are invalid: %v", reasons))
		r.Recorder.Event(template, corev1.EventTypeWarning, "ResourceValidationFailed",
			fmt.Sprintf("Some resources are invalid: %v", reasons))
	}

	// Set resource availability condition
	// Warn if capacity is low (< 80% of desired replicas) or critically low (< desired replicas)
	needed := template.Status.DesiredReplicas - template.Status.ReadyReplicas
	if utilization.EstimatedVMCapacity >= template.Status.DesiredReplicas {
		template.SetCondition(vmwarev1alpha1.ConditionTypeResourcesAvailable, metav1.ConditionTrue,
			"SufficientResources",
			fmt.Sprintf("Can create %d VMs (need %d)", utilization.EstimatedVMCapacity, needed))
	} else if utilization.EstimatedVMCapacity == 0 {
		template.SetCondition(vmwarev1alpha1.ConditionTypeResourcesAvailable, metav1.ConditionFalse,
			"NoCapacity",
			fmt.Sprintf("Cannot create any VMs - resources exhausted (need %d)", needed))
		r.Recorder.Event(template, corev1.EventTypeWarning, "ResourcesExhausted",
			fmt.Sprintf("Cannot create VMs - estimated capacity: %d, needed: %d",
				utilization.EstimatedVMCapacity, needed))
	} else {
		template.SetCondition(vmwarev1alpha1.ConditionTypeResourcesAvailable, metav1.ConditionFalse,
			"InsufficientResources",
			fmt.Sprintf("Can only create %d VMs but need %d", utilization.EstimatedVMCapacity, needed))
		r.Recorder.Event(template, corev1.EventTypeWarning, "InsufficientResources",
			fmt.Sprintf("Insufficient resources - estimated capacity: %d, needed: %d",
				utilization.EstimatedVMCapacity, needed))
	}

	// Record metrics
	if r.Metrics != nil {
		// Datastore metrics
		r.Metrics.RecordDatastoreUtilization(
			template.Name, template.Namespace, utilization.Datastore.Name,
			utilization.Datastore.CapacityGB, utilization.Datastore.FreeSpaceGB,
			utilization.Datastore.UsedGB, utilization.Datastore.PercentUsed,
		)

		// Compute metrics
		r.Metrics.RecordComputeUtilization(
			template.Name, template.Namespace,
			utilization.Compute.ResourceType, utilization.Compute.Name,
			utilization.Compute.CpuTotalMhz, utilization.Compute.CpuAvailableMhz,
			utilization.Compute.CpuUsedMhz, utilization.Compute.CpuPercentUsed,
			utilization.Compute.MemoryTotalMb, utilization.Compute.MemoryAvailableMb,
			utilization.Compute.MemoryUsedMb, utilization.Compute.MemoryPercentUsed,
		)

		// Estimated VM capacity
		r.Metrics.RecordEstimatedVMCapacity(template.Name, template.Namespace, utilization.EstimatedVMCapacity)

		// Resource validation metrics
		r.Metrics.RecordResourceValidation(template.Name, template.Namespace, "datacenter",
			template.Spec.VMTemplate.Datacenter, validation.Datacenter.Exists)
		r.Metrics.RecordResourceValidation(template.Name, template.Namespace, "datastore",
			template.Spec.VMTemplate.Datastore, validation.Datastore.Exists)
		r.Metrics.RecordResourceValidation(template.Name, template.Namespace, "network",
			template.Spec.VMTemplate.Network, validation.Network.Exists)

		if validation.Cluster != nil {
			r.Metrics.RecordResourceValidation(template.Name, template.Namespace, "cluster",
				template.Spec.VMTemplate.Cluster, validation.Cluster.Exists)
		}
		if validation.ResourcePool != nil {
			r.Metrics.RecordResourceValidation(template.Name, template.Namespace, "resourcepool",
				template.Spec.VMTemplate.ResourcePool, validation.ResourcePool.Exists)
		}
		if validation.Folder != nil {
			r.Metrics.RecordResourceValidation(template.Name, template.Namespace, "folder",
				template.Spec.VMTemplate.Folder, validation.Folder.Exists)
		}
	}

	log.Info("Resource utilization updated",
		"estimatedCapacity", utilization.EstimatedVMCapacity,
		"datastoreFreeGB", utilization.Datastore.FreeSpaceGB,
		"cpuAvailableMhz", utilization.Compute.CpuAvailableMhz,
		"memoryAvailableMb", utilization.Compute.MemoryAvailableMb,
	)

	return nil
}
