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
	}

	// Register metrics with controller-runtime's registry
	metrics.Registry.MustRegister(
		m.vmCount,
		m.vmReady,
		m.vmCreated,
		m.vmDeleted,
		m.reconcileTotal,
		m.reconcileError,
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
