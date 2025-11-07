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
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// createTestMetrics creates a Metrics instance for testing without registering to global registry
func createTestMetrics() *Metrics {
	return &Metrics{
		vmCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_nodepool_vm_count_test",
				Help: "Current number of VMs managed by VMwareNodePoolTemplate",
			},
			[]string{"name", "namespace"},
		),
		vmReady: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmware_nodepool_vm_ready_test",
				Help: "Number of ready VMs managed by VMwareNodePoolTemplate",
			},
			[]string{"name", "namespace"},
		),
		vmCreated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vmware_nodepool_vm_created_total_test",
				Help: "Total number of VMs created",
			},
			[]string{"name", "namespace"},
		),
		vmDeleted: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vmware_nodepool_vm_deleted_total_test",
				Help: "Total number of VMs deleted",
			},
			[]string{"name", "namespace"},
		),
		reconcileTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vmware_nodepool_reconcile_total_test",
				Help: "Total number of reconciliations",
			},
			[]string{"name", "namespace"},
		),
		reconcileError: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vmware_nodepool_reconcile_errors_total_test",
				Help: "Total number of reconciliation errors",
			},
			[]string{"name", "namespace"},
		),
	}
}

func TestNewMetrics(t *testing.T) {
	metrics := createTestMetrics()

	if metrics == nil {
		t.Fatal("createTestMetrics returned nil")
	}

	// Verify all metrics are created
	if metrics.vmCount == nil {
		t.Error("vmCount metric is nil")
	}
	if metrics.vmReady == nil {
		t.Error("vmReady metric is nil")
	}
	if metrics.vmCreated == nil {
		t.Error("vmCreated metric is nil")
	}
	if metrics.vmDeleted == nil {
		t.Error("vmDeleted metric is nil")
	}
	if metrics.reconcileTotal == nil {
		t.Error("reconcileTotal metric is nil")
	}
	if metrics.reconcileError == nil {
		t.Error("reconcileError metric is nil")
	}
}

func TestRecordVMCount(t *testing.T) {
	metrics := createTestMetrics()

	// Record VM count
	metrics.RecordVMCount("test-template", "test-namespace", 5)

	// Verify the metric value
	value := testutil.ToFloat64(metrics.vmCount.WithLabelValues("test-template", "test-namespace"))
	if value != 5 {
		t.Errorf("Expected VM count 5, got %f", value)
	}

	// Update with different value
	metrics.RecordVMCount("test-template", "test-namespace", 10)
	value = testutil.ToFloat64(metrics.vmCount.WithLabelValues("test-template", "test-namespace"))
	if value != 10 {
		t.Errorf("Expected VM count 10, got %f", value)
	}
}

func TestRecordVMReady(t *testing.T) {
	metrics := createTestMetrics()

	// Record ready VMs
	metrics.RecordVMReady("test-template", "test-namespace", 3)

	// Verify the metric value
	value := testutil.ToFloat64(metrics.vmReady.WithLabelValues("test-template", "test-namespace"))
	if value != 3 {
		t.Errorf("Expected ready VMs 3, got %f", value)
	}
}

func TestRecordVMCreated(t *testing.T) {
	metrics := createTestMetrics()

	// Record VM creation
	metrics.RecordVMCreated("test-template", "test-namespace")

	// Verify the counter incremented
	value := testutil.ToFloat64(metrics.vmCreated.WithLabelValues("test-template", "test-namespace"))
	if value != 1 {
		t.Errorf("Expected VM created count 1, got %f", value)
	}

	// Record another creation
	metrics.RecordVMCreated("test-template", "test-namespace")
	value = testutil.ToFloat64(metrics.vmCreated.WithLabelValues("test-template", "test-namespace"))
	if value != 2 {
		t.Errorf("Expected VM created count 2, got %f", value)
	}
}

func TestRecordVMDeleted(t *testing.T) {
	metrics := createTestMetrics()

	// Record VM deletion
	metrics.RecordVMDeleted("test-template", "test-namespace")

	// Verify the counter incremented
	value := testutil.ToFloat64(metrics.vmDeleted.WithLabelValues("test-template", "test-namespace"))
	if value != 1 {
		t.Errorf("Expected VM deleted count 1, got %f", value)
	}
}

func TestRecordReconcile(t *testing.T) {
	metrics := createTestMetrics()

	// Record reconciliation
	metrics.RecordReconcile("test-template", "test-namespace")

	// Verify counter incremented
	value := testutil.ToFloat64(metrics.reconcileTotal.WithLabelValues("test-template", "test-namespace"))
	if value != 1 {
		t.Errorf("Expected reconcile count 1, got %f", value)
	}

	// Record another reconciliation
	metrics.RecordReconcile("test-template", "test-namespace")

	// Verify counter incremented again
	value = testutil.ToFloat64(metrics.reconcileTotal.WithLabelValues("test-template", "test-namespace"))
	if value != 2 {
		t.Errorf("Expected reconcile count 2, got %f", value)
	}
}

func TestRecordReconcileError(t *testing.T) {
	metrics := createTestMetrics()

	// Record reconcile error
	metrics.RecordReconcileError("test-template", "test-namespace")

	// Verify the counter incremented
	value := testutil.ToFloat64(metrics.reconcileError.WithLabelValues("test-template", "test-namespace"))
	if value != 1 {
		t.Errorf("Expected reconcile error count 1, got %f", value)
	}

	// Record another error
	metrics.RecordReconcileError("test-template", "test-namespace")
	value = testutil.ToFloat64(metrics.reconcileError.WithLabelValues("test-template", "test-namespace"))
	if value != 2 {
		t.Errorf("Expected reconcile error count 2, got %f", value)
	}
}

func TestMultipleTemplates(t *testing.T) {
	metrics := createTestMetrics()

	// Record metrics for multiple templates
	metrics.RecordVMCount("template-1", "namespace-a", 5)
	metrics.RecordVMCount("template-2", "namespace-a", 3)
	metrics.RecordVMCount("template-3", "namespace-b", 10)

	// Verify each template has independent metrics
	value1 := testutil.ToFloat64(metrics.vmCount.WithLabelValues("template-1", "namespace-a"))
	if value1 != 5 {
		t.Errorf("Template 1: Expected VM count 5, got %f", value1)
	}

	value2 := testutil.ToFloat64(metrics.vmCount.WithLabelValues("template-2", "namespace-a"))
	if value2 != 3 {
		t.Errorf("Template 2: Expected VM count 3, got %f", value2)
	}

	value3 := testutil.ToFloat64(metrics.vmCount.WithLabelValues("template-3", "namespace-b"))
	if value3 != 10 {
		t.Errorf("Template 3: Expected VM count 10, got %f", value3)
	}
}

func TestMetricsLabels(t *testing.T) {
	metrics := createTestMetrics()

	// Test with various label values
	testCases := []struct {
		template  string
		namespace string
		vmCount   int32
	}{
		{"my-template", "default", 1},
		{"prod-template", "production", 100},
		{"test-template-with-long-name", "namespace-with-long-name", 50},
	}

	for _, tc := range testCases {
		metrics.RecordVMCount(tc.template, tc.namespace, tc.vmCount)
		value := testutil.ToFloat64(metrics.vmCount.WithLabelValues(tc.template, tc.namespace))
		if value != float64(tc.vmCount) {
			t.Errorf("Template %s/%s: Expected VM count %d, got %f",
				tc.namespace, tc.template, tc.vmCount, value)
		}
	}
}
