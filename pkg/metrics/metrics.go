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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Metrics holds Prometheus metrics for the controller
type Metrics struct {
	vmCount        *prometheus.GaugeVec
	vmReady        *prometheus.GaugeVec
	vmCreated      *prometheus.CounterVec
	vmDeleted      *prometheus.CounterVec
	reconcileTotal *prometheus.CounterVec
	reconcileError *prometheus.CounterVec

	// Resource utilization metrics
	datastoreCapacityGB  *prometheus.GaugeVec
	datastoreFreeGB      *prometheus.GaugeVec
	datastoreUsedGB      *prometheus.GaugeVec
	datastorePercentUsed *prometheus.GaugeVec
	cpuTotalMHz          *prometheus.GaugeVec
	cpuAvailableMHz      *prometheus.GaugeVec
	cpuUsedMHz           *prometheus.GaugeVec
	cpuPercentUsed       *prometheus.GaugeVec
	memoryTotalMB        *prometheus.GaugeVec
	memoryAvailableMB    *prometheus.GaugeVec
	memoryUsedMB         *prometheus.GaugeVec
	memoryPercentUsed    *prometheus.GaugeVec
	estimatedVMCapacity  *prometheus.GaugeVec
	resourceValidationOK *prometheus.GaugeVec
}

// NewMetrics creates a new Metrics instance and registers metrics with Prometheus
func NewMetrics() *Metrics {
	m := &Metrics{
		vmCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_nodepool_vm_count",
				Help: "Current number of VMs managed by VMwareNodePoolTemplate",
			},
			[]string{"name", "namespace"},
		),
		vmReady: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_nodepool_vm_ready",
				Help: "Number of ready VMs managed by VMwareNodePoolTemplate",
			},
			[]string{"name", "namespace"},
		),
		vmCreated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vmware_nodepool_vm_created_total",
				Help: "Total number of VMs created",
			},
			[]string{"name", "namespace"},
		),
		vmDeleted: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vmware_nodepool_vm_deleted_total",
				Help: "Total number of VMs deleted",
			},
			[]string{"name", "namespace"},
		),
		reconcileTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vmware_nodepool_reconcile_total",
				Help: "Total number of reconciliations",
			},
			[]string{"name", "namespace"},
		),
		reconcileError: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vmware_nodepool_reconcile_errors_total",
				Help: "Total number of reconciliation errors",
			},
			[]string{"name", "namespace"},
		),
		datastoreCapacityGB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_datastore_capacity_gb",
				Help: "Total datastore capacity in gigabytes",
			},
			[]string{"name", "namespace", "datastore"},
		),
		datastoreFreeGB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_datastore_free_gb",
				Help: "Free datastore space in gigabytes",
			},
			[]string{"name", "namespace", "datastore"},
		),
		datastoreUsedGB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_datastore_used_gb",
				Help: "Used datastore space in gigabytes",
			},
			[]string{"name", "namespace", "datastore"},
		),
		datastorePercentUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_datastore_percent_used",
				Help: "Datastore utilization percentage (0-100)",
			},
			[]string{"name", "namespace", "datastore"},
		),
		cpuTotalMHz: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_compute_cpu_total_mhz",
				Help: "Total CPU capacity in MHz",
			},
			[]string{"name", "namespace", "resource_type", "resource_name"},
		),
		cpuAvailableMHz: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_compute_cpu_available_mhz",
				Help: "Available CPU in MHz",
			},
			[]string{"name", "namespace", "resource_type", "resource_name"},
		),
		cpuUsedMHz: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_compute_cpu_used_mhz",
				Help: "Used CPU in MHz",
			},
			[]string{"name", "namespace", "resource_type", "resource_name"},
		),
		cpuPercentUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_compute_cpu_percent_used",
				Help: "CPU utilization percentage (0-100)",
			},
			[]string{"name", "namespace", "resource_type", "resource_name"},
		),
		memoryTotalMB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_compute_memory_total_mb",
				Help: "Total memory capacity in MB",
			},
			[]string{"name", "namespace", "resource_type", "resource_name"},
		),
		memoryAvailableMB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_compute_memory_available_mb",
				Help: "Available memory in MB",
			},
			[]string{"name", "namespace", "resource_type", "resource_name"},
		),
		memoryUsedMB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_compute_memory_used_mb",
				Help: "Used memory in MB",
			},
			[]string{"name", "namespace", "resource_type", "resource_name"},
		),
		memoryPercentUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_compute_memory_percent_used",
				Help: "Memory utilization percentage (0-100)",
			},
			[]string{"name", "namespace", "resource_type", "resource_name"},
		),
		estimatedVMCapacity: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_estimated_vm_capacity",
				Help: "Estimated number of VMs that can be created based on available resources",
			},
			[]string{"name", "namespace"},
		),
		resourceValidationOK: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_hcp_resource_validation_ok",
				Help: "Resource validation status (1=valid, 0=invalid)",
			},
			[]string{"name", "namespace", "resource_type", "resource_name"},
		),
	}

	// Register metrics with controller-runtime's registry
	metrics.Registry.MustRegister(
		m.vmCount,
		m.vmReady,
		m.vmCreated,
		m.vmDeleted,
		m.reconcileTotal,
		m.reconcileError,
		m.datastoreCapacityGB,
		m.datastoreFreeGB,
		m.datastoreUsedGB,
		m.datastorePercentUsed,
		m.cpuTotalMHz,
		m.cpuAvailableMHz,
		m.cpuUsedMHz,
		m.cpuPercentUsed,
		m.memoryTotalMB,
		m.memoryAvailableMB,
		m.memoryUsedMB,
		m.memoryPercentUsed,
		m.estimatedVMCapacity,
		m.resourceValidationOK,
	)

	return m
}

// RecordVMCount records the current VM count
func (m *Metrics) RecordVMCount(name, namespace string, count int32) {
	m.vmCount.WithLabelValues(name, namespace).Set(float64(count))
}

// RecordVMReady records the number of ready VMs
func (m *Metrics) RecordVMReady(name, namespace string, ready int32) {
	m.vmReady.WithLabelValues(name, namespace).Set(float64(ready))
}

// RecordVMCreated increments the VM created counter
func (m *Metrics) RecordVMCreated(name, namespace string) {
	m.vmCreated.WithLabelValues(name, namespace).Inc()
}

// RecordVMDeleted increments the VM deleted counter
func (m *Metrics) RecordVMDeleted(name, namespace string) {
	m.vmDeleted.WithLabelValues(name, namespace).Inc()
}

// RecordReconcile increments the reconciliation counter
func (m *Metrics) RecordReconcile(name, namespace string) {
	m.reconcileTotal.WithLabelValues(name, namespace).Inc()
}

// RecordReconcileError increments the reconciliation error counter
func (m *Metrics) RecordReconcileError(name, namespace string) {
	m.reconcileError.WithLabelValues(name, namespace).Inc()
}

// RecordDatastoreUtilization records datastore capacity metrics
func (m *Metrics) RecordDatastoreUtilization(name, namespace, datastore string, capacityGB, freeGB, usedGB int64, percentUsed int32) {
	m.datastoreCapacityGB.WithLabelValues(name, namespace, datastore).Set(float64(capacityGB))
	m.datastoreFreeGB.WithLabelValues(name, namespace, datastore).Set(float64(freeGB))
	m.datastoreUsedGB.WithLabelValues(name, namespace, datastore).Set(float64(usedGB))
	m.datastorePercentUsed.WithLabelValues(name, namespace, datastore).Set(float64(percentUsed))
}

// RecordComputeUtilization records CPU and memory utilization metrics
func (m *Metrics) RecordComputeUtilization(name, namespace, resourceType, resourceName string,
	cpuTotalMhz, cpuAvailableMhz, cpuUsedMhz int64, cpuPercentUsed int32,
	memoryTotalMb, memoryAvailableMb, memoryUsedMb int64, memoryPercentUsed int32) {

	m.cpuTotalMHz.WithLabelValues(name, namespace, resourceType, resourceName).Set(float64(cpuTotalMhz))
	m.cpuAvailableMHz.WithLabelValues(name, namespace, resourceType, resourceName).Set(float64(cpuAvailableMhz))
	m.cpuUsedMHz.WithLabelValues(name, namespace, resourceType, resourceName).Set(float64(cpuUsedMhz))
	m.cpuPercentUsed.WithLabelValues(name, namespace, resourceType, resourceName).Set(float64(cpuPercentUsed))

	m.memoryTotalMB.WithLabelValues(name, namespace, resourceType, resourceName).Set(float64(memoryTotalMb))
	m.memoryAvailableMB.WithLabelValues(name, namespace, resourceType, resourceName).Set(float64(memoryAvailableMb))
	m.memoryUsedMB.WithLabelValues(name, namespace, resourceType, resourceName).Set(float64(memoryUsedMb))
	m.memoryPercentUsed.WithLabelValues(name, namespace, resourceType, resourceName).Set(float64(memoryPercentUsed))
}

// RecordEstimatedVMCapacity records the estimated VM capacity
func (m *Metrics) RecordEstimatedVMCapacity(name, namespace string, capacity int32) {
	m.estimatedVMCapacity.WithLabelValues(name, namespace).Set(float64(capacity))
}

// RecordResourceValidation records resource validation status
func (m *Metrics) RecordResourceValidation(name, namespace, resourceType, resourceName string, valid bool) {
	value := 0.0
	if valid {
		value = 1.0
	}
	m.resourceValidationOK.WithLabelValues(name, namespace, resourceType, resourceName).Set(value)
}
