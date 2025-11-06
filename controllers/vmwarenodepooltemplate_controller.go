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
	finalizerName = "vmware.hcp.open-cluster-management.io/finalizer"

	// Default credential secret name
	defaultCredentialSecretName = "vsphere-credentials"

	// Requeue intervals
	requeueAfterError   = 1 * time.Minute
	requeueAfterSuccess = 5 * time.Minute
)

// VMwareNodePoolTemplateReconciler reconciles a VMwareNodePoolTemplate object
type VMwareNodePoolTemplateReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Metrics  *metrics.Metrics
}

// +kubebuilder:rbac:groups=vmware.hcp.open-cluster-management.io,resources=vmwarenodepooltemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vmware.hcp.open-cluster-management.io,resources=vmwarenodepooltemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vmware.hcp.open-cluster-management.io,resources=vmwarenodepooltemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=hypershift.openshift.io,resources=nodepools,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=hypershift.openshift.io,resources=nodepools/finalizers,verbs=update
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
				// NodePool is healthy - get replicas
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

	// All VMs have been deleted - remove finalizer
	log.Info("All VMs deleted, removing finalizer")
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
					if err := vClient.DestroyVM(ctx, vm); err != nil {
						log.Error(err, "Failed to destroy VM - will remove finalizer anyway", "vm", vmName)
					} else {
						log.Info("Successfully deleted VM for agent", "vm", vmName, "agent", agentName)
						vmDeleted = true
						if !templateNotFound {
							r.Recorder.Eventf(template, corev1.EventTypeNormal, "VMCleanedUp", "Cleaned up VM %s for deleted agent %s", vmName, agentName)
							if r.Metrics != nil {
								r.Metrics.RecordVMDeleted(template.Name, template.Namespace)
							}
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
