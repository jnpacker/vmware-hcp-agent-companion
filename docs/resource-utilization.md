# Resource Utilization and Capacity Planning

This document explains how the VMwareNodePoolTemplate tracks vSphere resource utilization and estimates VM capacity.

## Overview

The controller automatically polls vSphere to:
1. **Validate** that all specified resources (datacenter, cluster, datastore, network) exist
2. **Track utilization** of CPU, memory, and storage resources
3. **Estimate capacity** to determine how many VMs can be created

## Resource Metrics Tracked

### Datastore (Storage)
- **CapacityGB**: Total datastore capacity
- **FreeSpaceGB**: Available free space
- **UsedGB**: Currently used space
- **PercentUsed**: Utilization percentage (0-100)

### Compute (Cluster or ResourcePool)
- **CpuTotalMhz**: Total CPU capacity in MHz
- **CpuEffectiveMhz**: Effective CPU after HA/DRS overhead (cluster only)
- **CpuUsedMhz**: Actual CPU usage
- **CpuAvailableMhz**: Available = Total - Used
- **CpuPercentUsed**: CPU utilization percentage

- **MemoryTotalMb**: Total memory capacity in MB
- **MemoryEffectiveMb**: Effective memory after HA/DRS overhead (cluster only)
- **MemoryUsedMb**: Actual memory usage
- **MemoryAvailableMb**: Available = Total - Used
- **MemoryPercentUsed**: Memory utilization percentage

### Estimated VM Capacity
- **EstimatedVMCapacity**: Number of VMs that can be created based on available resources
  - Calculated as the **minimum** of storage, CPU, and memory constraints
  - Configurable to use either total capacity or effective capacity (see below)

## Understanding Total vs Effective Capacity

### Total Capacity
The raw hardware capacity of your cluster.

**Example:** 10 hosts × 40 GHz = 400 GHz total CPU

### Effective Capacity
Capacity available for VMs after accounting for HA/DRS overhead.

**Example:** With 10% HA admission control, effective capacity = 360 GHz

### Difference Explained

VMware reserves some capacity for High Availability (HA) and Distributed Resource Scheduler (DRS):
- **HA Admission Control**: Reserves capacity to handle host failures
- **DRS Overhead**: Small overhead for DRS operations

**When Usage Can Exceed Effective:**
1. During HA failover events (using reserved capacity)
2. Manual admin overrides
3. DRS imbalances or maintenance mode

## Configuring VM Capacity Estimation

You can control whether the estimated VM capacity calculation uses **total** or **effective** capacity:

### Default Behavior (useEffectiveCapacity: false)

**Uses:** `Total Capacity - Used`

**Best For:**
- Simple environments without HA
- Single-tenant deployments
- When you want optimistic capacity estimates

**Example:**
```yaml
spec:
  useEffectiveCapacity: false  # or omit (default)
```

**Calculation:**
- Total CPU: 400 GHz
- Used CPU: 200 GHz
- **Available for VMs: 200 GHz** (doesn't account for HA)

### Effective Mode (useEffectiveCapacity: true)

**Uses:** `Effective Capacity - Used`

**Best For:**
- Production environments with HA enabled
- When you need conservative, safe capacity estimates
- Avoiding HA admission control violations

**Example:**
```yaml
spec:
  useEffectiveCapacity: true
```

**Calculation:**
- Total CPU: 400 GHz
- Effective CPU: 360 GHz (HA reserves 40 GHz)
- Used CPU: 200 GHz
- **Available for VMs: 160 GHz** (accounts for HA)

## Polling Behavior

Resource utilization is polled using **event-driven + periodic** strategy:

1. **Event-Driven**: Polls immediately when:
   - Template is created
   - Template spec is updated
   - Resource status is missing

2. **Periodic**: Polls at configured interval:
   - Default: Every 5 minutes
   - Configurable via `resourcePollingInterval`
   - Minimum: 1 minute

### Configuration Example

```yaml
spec:
  resourcePollingInterval: 3m  # Poll every 3 minutes
```

## Resource Validation

The controller validates that all vSphere resources exist and are accessible:

**Resources Validated:**
- ✅ Datacenter (required)
- ✅ Cluster (optional)
- ✅ ResourcePool (optional)
- ✅ Datastore (required)
- ✅ Network (required)
- ✅ Folder (optional, auto-created if missing)

**Status Conditions:**
- `ResourcesValidated`: True when all resources exist
- `ResourcesAvailable`: True when sufficient capacity exists

## Viewing Resource Utilization

### Quick View (kubectl get)

```bash
kubectl get vmtemplate -o wide
```

Shows: Desired, Current, Ready, **Est. Capacity**, **Storage Free**, **CPU Avail %**, **Mem Avail %**

### Detailed View (kubectl describe)

```bash
kubectl describe vmtemplate my-template
```

Shows full resource utilization including:
- Datastore capacity and usage
- CPU total, effective, used, available
- Memory total, effective, used, available
- Estimated VM capacity
- Resource validation status

### YAML View

```bash
kubectl get vmtemplate my-template -o yaml
```

Example output:
```yaml
status:
  resourceUtilization:
    datastore:
      name: datastore1
      capacityGB: 2000
      freeSpaceGB: 800
      usedGB: 1200
      percentUsed: 60
    compute:
      resourceType: Cluster
      name: Production-Cluster
      cpuTotalMhz: 377488
      cpuEffectiveMhz: 360000
      cpuUsedMhz: 197970
      cpuAvailableMhz: 179518
      cpuPercentUsed: 52
      memoryTotalMb: 2355828
      memoryEffectiveMb: 2240000
      memoryUsedMb: 1835827
      memoryAvailableMb: 520001
      memoryPercentUsed: 77
    estimatedVMCapacity: 66
    lastUpdated: "2025-01-25T10:30:00Z"

  resourceValidation:
    datacenter:
      exists: true
      message: "Datacenter 'DC1' is accessible"
    cluster:
      exists: true
      message: "Cluster 'Production-Cluster' is accessible"
    datastore:
      exists: true
      message: "Datastore 'datastore1' is accessible"
    network:
      exists: true
      message: "Network 'VM Network' is accessible"
```

## Prometheus Metrics

The controller exports Prometheus metrics for monitoring:

### Datastore Metrics
- `vmware_hcp_datastore_capacity_gb{name, namespace, datastore}`
- `vmware_hcp_datastore_free_gb{name, namespace, datastore}`
- `vmware_hcp_datastore_used_gb{name, namespace, datastore}`
- `vmware_hcp_datastore_percent_used{name, namespace, datastore}`

### Compute Metrics
- `vmware_hcp_compute_cpu_total_mhz{name, namespace, resource_type, resource_name}`
- `vmware_hcp_compute_cpu_available_mhz{name, namespace, resource_type, resource_name}`
- `vmware_hcp_compute_cpu_used_mhz{name, namespace, resource_type, resource_name}`
- `vmware_hcp_compute_cpu_percent_used{name, namespace, resource_type, resource_name}`
- `vmware_hcp_compute_memory_total_mb{name, namespace, resource_type, resource_name}`
- `vmware_hcp_compute_memory_available_mb{name, namespace, resource_type, resource_name}`
- `vmware_hcp_compute_memory_used_mb{name, namespace, resource_type, resource_name}`
- `vmware_hcp_compute_memory_percent_used{name, namespace, resource_type, resource_name}`

### Capacity & Validation
- `vmware_hcp_estimated_vm_capacity{name, namespace}`
- `vmware_hcp_resource_validation_ok{name, namespace, resource_type, resource_name}`

## Complete Configuration Example

```yaml
apiVersion: vmware.hcp.open-cluster-management.io/v1alpha1
kind: VMwareNodePoolTemplate
metadata:
  name: production-workers
  namespace: clusters
spec:
  nodePoolRef:
    name: my-nodepool

  # Resource polling configuration
  resourcePollingInterval: 5m        # Poll every 5 minutes (default)
  useEffectiveCapacity: true         # Use effective capacity for estimates (default: false)

  vmTemplate:
    datacenter: "DC1"
    cluster: "Production-Cluster"
    resourcePool: "worker-pool"      # Optional
    datastore: "datastore1"
    network: "VM Network"
    folder: "HCP-VMs"                # Optional

    # VM specifications
    numCPUs: 8
    memoryMB: 16384
    diskSizeGB: 120

    advancedConfig:
      diskEnableUUID: true
      nestedVirtualization: true

  agentISO:
    type: datastore
    datastorePath: "[datastore1] iso/agent.iso"

  agentLabelSelector:
    nodepool: "my-nodepool"
```

## Capacity Planning Best Practices

### 1. Monitor Resource Trends
Use Prometheus metrics to track resource utilization over time and predict when capacity will be exhausted.

### 2. Set Alerts
Create alerts when:
- Estimated VM capacity drops below desired replicas
- Resource utilization exceeds 80%
- Resources fail validation

### 3. Choose Appropriate Mode
- **Development/Testing**: `useEffectiveCapacity: false` (more permissive)
- **Production**: `useEffectiveCapacity: true` (respects HA)

### 4. Adjust Polling Interval
- **Fast-changing environments**: 2-3 minutes
- **Stable environments**: 10-15 minutes
- Consider API load vs freshness requirements

## Troubleshooting

### Estimated Capacity Shows 0

**Possible Causes:**
1. Resources are exhausted (check CPU, memory, storage percentages)
2. Using effective mode and cluster is over-committed
3. Resource validation failed

**Solutions:**
- Check `kubectl describe` for detailed resource breakdown
- Switch to `useEffectiveCapacity: false` if HA overhead is too conservative
- Add capacity or reduce VM resource requirements

### Resources Not Validating

**Check:**
1. vSphere connectivity (`VSphereConnected` condition)
2. Resource names are correct (case-sensitive)
3. Permissions to access resources
4. Network connectivity to vCenter

### Polling Not Updating

**Check:**
1. Last updated timestamp
2. Controller logs for errors
3. `resourcePollingInterval` setting
4. vSphere API accessibility
