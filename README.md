# VMware HCP Agent Companion Controller

A Kubernetes controller that automates the provisioning of VMware virtual machines for Hosted Control Plane (HCP) Agent-based deployments. This controller watches NodePool resources and dynamically creates, scales, and manages VMware VMs that boot to the OpenShift Agent ISO.

## Features

- **Automated VM Provisioning**: Automatically creates VMware VMs based on NodePool specifications
- **Dynamic Scaling**: Automatically adds or removes VMs when NodePool replica count changes
- **Agent Integration**: Labels discovered Agents to associate them with specific NodePools
- **Flexible ISO Handling**: Supports both pre-configured datastore ISOs and automatic download/upload
- **Test Mode**: Standalone validation mode for testing vSphere connectivity without NodePools
- **Full VMware Customization**: Supports all VMware VM specifications (CPU, memory, disk, network, etc.)
- **Observability**: Built-in Prometheus metrics and detailed status conditions
- **Event Recording**: Kubernetes events for major lifecycle operations

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster (Hub)                      │
│                                                                  │
│  ┌──────────────────┐        ┌─────────────────────────────┐   │
│  │   NodePool       │        │ VMwareNodePoolTemplate      │   │
│  │  (HyperShift)    │◄───────│                             │   │
│  │                  │        │  - VM Specifications        │   │
│  │  replicas: 3     │        │  - ISO Configuration        │   │
│  └──────────────────┘        │  - Agent Labels             │   │
│                              └──────────┬──────────────────┘   │
│                                         │                       │
│  ┌──────────────────┐                   │                       │
│  │  Agent Resources │◄──────────────────┤                       │
│  │                  │    (Labels VMs)    │                       │
│  └──────────────────┘                   │                       │
│                                         │                       │
└─────────────────────────────────────────┼───────────────────────┘
                                          │
                        (Creates/Manages VMs via vSphere API)
                                          │
                                          ▼
                          ┌───────────────────────────┐
                          │   VMware vSphere          │
                          │                           │
                          │  ┌────┐ ┌────┐ ┌────┐   │
                          │  │VM-1│ │VM-2│ │VM-3│   │
                          │  └────┘ └────┘ └────┘   │
                          │    ▲      ▲      ▲       │
                          │    └──────┴──────┘       │
                          │    Booted from Agent ISO │
                          └───────────────────────────┘
```

## Prerequisites

- Kubernetes cluster with:
  - OpenShift Advanced Cluster Management (ACM) or equivalent
  - HyperShift operator installed
  - Agent-Install operator installed
- VMware vSphere environment:
  - vCenter Server 7.0 or later
  - Appropriate permissions to create/delete VMs
  - Network connectivity from controller to vCenter
- Go 1.21+ (for building from source)

**IMPORTANT - Namespace Requirements:**
- The **VMwareNodePoolTemplate**, **NodePool**, and **vSphere credentials secret** must all be in the **same namespace**
- This namespace scoping ensures proper RBAC and resource association

## Installation

### 1. Install CRDs

```bash
make install
```

Or manually:

```bash
kubectl apply -f config/crd/bases/
```

### 2. Create vSphere Credentials Secret

Create a secret with your vSphere credentials:

```bash
kubectl create secret generic vsphere-credentials \
  --from-literal=server=vcenter.example.com \
  --from-literal=username=administrator@vsphere.local \
  --from-literal=password=your-password \
  --from-literal=insecure=true \
  -n your-namespace
```

**Note:** The secret must be created in the **same namespace** where you will deploy the VMwareNodePoolTemplate and NodePool.

Or use the example:

```bash
# Edit config/samples/vsphere-credentials.yaml with your credentials
# Make sure to set the namespace to match your NodePool namespace
kubectl apply -f config/samples/vsphere-credentials.yaml
```

### 3. Deploy the Controller

```bash
# Build and push your image
export IMG=your-registry/vmware-hcp-controller:latest
make docker-build docker-push IMG=$IMG

# Update the image in deployment
cd config/manager && kustomize edit set image controller=$IMG

# Deploy
make deploy
```

Or run locally for development:

```bash
make run
```

## Usage

### Test Mode (Standalone Validation)

Test mode allows you to validate your vSphere configuration without requiring a NodePool:

```yaml
apiVersion: vmware.hcp.open-cluster-management.io/v1alpha1
kind: VMwareNodePoolTemplate
metadata:
  name: vsphere-test
  namespace: default
spec:
  testMode: true
  replicas: 2

  vmTemplate:
    datacenter: "Datacenter1"
    cluster: "Cluster1"
    datastore: "datastore1"
    network: "VM Network"
    numCPUs: 2
    memoryMB: 4096
    diskSizeGB: 120

  agentISO:
    type: datastore
    datastorePath: "[datastore1] iso/agent.iso"
```

Apply and verify:

```bash
kubectl apply -f config/samples/vmware_v1alpha1_vmwarenodepooltemplate_testmode.yaml

# Check status
kubectl get vmwarenodepooltemplate vsphere-test -o yaml

# Watch events
kubectl get events --field-selector involvedObject.name=vsphere-test
```

### Production Mode (with NodePool)

Create a VMwareNodePoolTemplate that tracks a NodePool:

```yaml
apiVersion: vmware.hcp.open-cluster-management.io/v1alpha1
kind: VMwareNodePoolTemplate
metadata:
  name: production-workers
  namespace: clusters
spec:
  nodePoolRef:
    name: my-nodepool
    namespace: clusters

  vmTemplate:
    datacenter: "Datacenter1"
    cluster: "Cluster1"
    datastore: "datastore1"
    network: "VM Network"
    numCPUs: 4
    memoryMB: 8192
    diskSizeGB: 200
    folder: "HCP-Workers"

  agentISO:
    type: url
    url: "https://mirror.example.com/agent.iso"

  agentLabelSelector:
    nodepool: "my-nodepool"
    environment: "production"
```

**IMPORTANT:** Both the VMwareNodePoolTemplate and the NodePool must be in the **same namespace** (in this example, `clusters`). The vSphere credentials secret must also exist in this namespace.

The controller will:
1. Watch the referenced NodePool
2. Create VMs matching the NodePool's replica count
3. Boot VMs from the Agent ISO
4. Label discovered Agents with the specified labels
5. Automatically scale VMs up/down when NodePool replicas change

### Minimal Configuration

The minimal required configuration:

```yaml
apiVersion: vmware.hcp.open-cluster-management.io/v1alpha1
kind: VMwareNodePoolTemplate
metadata:
  name: minimal-example
spec:
  vmTemplate:
    datacenter: "Datacenter1"
    datastore: "datastore1"
    network: "VM Network"
    cluster: "Cluster1"

  agentISO:
    type: datastore
    datastorePath: "[datastore1] iso/agent.iso"

  testMode: true
  replicas: 1
```

All other VM specifications will use defaults:
- NumCPUs: 2
- MemoryMB: 4096
- DiskSizeGB: 120
- GuestID: otherLinux64Guest
- Firmware: bios

## Configuration Reference

### VMwareNodePoolTemplate Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `nodePoolRef` | object | No* | Reference to NodePool (required unless testMode=true) |
| `vSphereCredentials` | object | No | Credentials reference (default: vsphere-credentials in same namespace) |
| `vmTemplate` | object | Yes | VM specifications |
| `agentISO` | object | Yes | Agent ISO configuration |
| `agentLabelSelector` | map | No | Labels to apply to discovered Agents |
| `testMode` | boolean | No | Enable test mode without NodePool |
| `replicas` | int32 | No | Number of VMs in test mode |

### AdvancedVMConfig Spec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `diskEnableUUID` | boolean | No | true | Enable UUID for virtual disks (required for Kubernetes storage) |
| `nestedVirtualization` | boolean | No | true | Enable hardware-assisted virtualization to guest OS (nested virtualization) |

### VMTemplate Spec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `datacenter` | string | Yes | - | vSphere datacenter name |
| `cluster` | string | No | - | vSphere cluster name |
| `resourcePool` | string | No | cluster root | Resource pool name |
| `datastore` | string | Yes | - | Datastore name |
| `folder` | string | No | datacenter VM folder | VM folder path |
| `network` | string | Yes | - | Network name |
| `numCPUs` | int32 | No | 2 | Number of virtual CPUs |
| `memoryMB` | int64 | No | 4096 | Memory in MB |
| `diskSizeGB` | int64 | No | 120 | Disk size in GB |
| `guestID` | string | No | otherLinux64Guest | Guest OS identifier |
| `firmware` | string | No | bios | Firmware type (bios or efi) |
| `namePrefix` | string | No | template name | VM name prefix |
| `extraConfig` | map | No | - | Additional VM configuration |
| `advancedConfig` | object | No | - | Advanced VM configuration (see AdvancedVMConfig) |

### AgentISO Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | ISO source type: "datastore" or "url" |
| `datastorePath` | string | Conditional | Path to ISO in datastore (required if type=datastore) |
| `url` | string | Conditional | URL to download ISO from (required if type=url) |
| `uploadedISOName` | string | No | Name for uploaded ISO (default: agent-<hash>.iso) |

## Status Conditions

The controller maintains several status conditions:

- **Ready**: Overall readiness status
- **VSphereConnected**: vSphere connectivity status
- **ISOReady**: Agent ISO availability
- **NodePoolFound**: NodePool discovery status (production mode)
- **VMsCreated**: VM creation/management status

Check status:

```bash
kubectl get vmwarenodepooltemplate -o wide
kubectl describe vmwarenodepooltemplate <name>
```

## Monitoring

### Prometheus Metrics

The controller exposes metrics on `:8080/metrics`:

- `vmware_nodepool_vm_count`: Current number of VMs
- `vmware_nodepool_vm_ready`: Number of ready VMs
- `