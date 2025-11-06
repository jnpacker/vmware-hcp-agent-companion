package vsphere

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"path"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// VMSpec defines the specification for creating a VM
type VMSpec struct {
	Name                 string
	NumCPUs              int32
	MemoryMB             int64
	DiskSizeGB           int64
	GuestID              string
	Firmware             string
	NetworkName          string
	DatastoreName        string
	ISOPath              string
	ExtraConfig          map[string]string
	DiskEnableUUID       bool
	NestedVirtualization bool
	// SCSI disk scheduling parameters
	DiskShares             string // "low", "normal", "high", or numeric value (e.g., "2000")
	DiskThroughputCap      string // "off" or numeric IOPS value (e.g., "500")
	DiskLatencySensitivity string // "low", "normal", "medium", "high"
}

// CreateVM creates a new virtual machine with the given specification
func (c *Client) CreateVM(ctx context.Context, spec VMSpec, resourcePool *object.ResourcePool, folder *object.Folder) (*object.VirtualMachine, error) {
	// Get network
	network, err := c.GetNetwork(ctx, spec.NetworkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	// Set defaults
	if spec.NumCPUs == 0 {
		spec.NumCPUs = 2
	}
	if spec.MemoryMB == 0 {
		spec.MemoryMB = 4096
	}
	if spec.DiskSizeGB == 0 {
		spec.DiskSizeGB = 120
	}
	if spec.GuestID == "" {
		spec.GuestID = "rhel8_64Guest" // Use RHEL 8 64-bit for EFI support
	}
	if spec.Firmware == "" {
		spec.Firmware = "efi" // Default to EFI with secure boot
	}

	// Set SCSI scheduling defaults
	if spec.DiskShares == "" {
		spec.DiskShares = "normal"
	}
	if spec.DiskThroughputCap == "" {
		spec.DiskThroughputCap = "off"
	}
	if spec.DiskLatencySensitivity == "" {
		spec.DiskLatencySensitivity = "normal"
	}

	// Build VM config spec with boot options set upfront
	configSpec := types.VirtualMachineConfigSpec{
		Name:     spec.Name,
		GuestId:  spec.GuestID,
		NumCPUs:  spec.NumCPUs,
		MemoryMB: spec.MemoryMB,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: fmt.Sprintf("[%s]", spec.DatastoreName),
		},
	}

	// Set firmware to EFI with secure boot enabled
	configSpec.Firmware = "efi"
	configSpec.BootOptions = &types.VirtualMachineBootOptions{
		EfiSecureBootEnabled: types.NewBool(true),
	}

	// Enable nested virtualization using the proper vSphere API field
	if spec.NestedVirtualization {
		configSpec.NestedHVEnabled = types.NewBool(true)
	}

	// Add extra config
	extraConfigSize := len(spec.ExtraConfig)
	if spec.DiskEnableUUID {
		extraConfigSize++ // Account for disk.enableUUID
	}
	// Account for SCSI scheduling parameters (always set with defaults)
	extraConfigSize += 3 // DiskShares, DiskThroughputCap, DiskLatencySensitivity

	if extraConfigSize > 0 {
		configSpec.ExtraConfig = make([]types.BaseOptionValue, 0, extraConfigSize)
		for key, value := range spec.ExtraConfig {
			configSpec.ExtraConfig = append(configSpec.ExtraConfig, &types.OptionValue{
				Key:   key,
				Value: value,
			})
		}

		// Add disk.enableUUID if requested
		if spec.DiskEnableUUID {
			configSpec.ExtraConfig = append(configSpec.ExtraConfig, &types.OptionValue{
				Key:   "disk.enableUUID",
				Value: "TRUE",
			})
		}

		// Add SCSI scheduling parameters for first disk on first controller (scsi0:0)
		configSpec.ExtraConfig = append(configSpec.ExtraConfig,
			&types.OptionValue{
				Key:   "sched.scsi0:0.shares",
				Value: spec.DiskShares,
			},
			&types.OptionValue{
				Key:   "sched.scsi0:0.throughputCap",
				Value: spec.DiskThroughputCap,
			},
			&types.OptionValue{
				Key:   "sched.scsi0:0.latencySensitivity",
				Value: spec.DiskLatencySensitivity,
			},
		)
	}

	// Add network device
	networkBacking, err := network.EthernetCardBackingInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get network backing info: %w", err)
	}

	networkDevice := &types.VirtualVmxnet3{
		VirtualVmxnet: types.VirtualVmxnet{
			VirtualEthernetCard: types.VirtualEthernetCard{
				VirtualDevice: types.VirtualDevice{
					Key:     0,
					Backing: networkBacking,
				},
				AddressType: string(types.VirtualEthernetCardMacTypeGenerated),
			},
		},
	}

	configSpec.DeviceChange = append(configSpec.DeviceChange, &types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
		Device:    networkDevice,
	})

	// Add SCSI controller (paravirtualized for better performance)
	scsiController := &types.ParaVirtualSCSIController{
		VirtualSCSIController: types.VirtualSCSIController{
			SharedBus: types.VirtualSCSISharingNoSharing,
			VirtualController: types.VirtualController{
				BusNumber: 0,
				VirtualDevice: types.VirtualDevice{
					Key: 1000,
				},
			},
		},
	}

	configSpec.DeviceChange = append(configSpec.DeviceChange, &types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
		Device:    scsiController,
	})

	// Add disk
	disk := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key:           2000,
			ControllerKey: 1000,
			UnitNumber:    new(int32),
			Backing: &types.VirtualDiskFlatVer2BackingInfo{
				DiskMode:        string(types.VirtualDiskModePersistent),
				ThinProvisioned: types.NewBool(true),
				VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
					FileName: fmt.Sprintf("[%s]", spec.DatastoreName),
				},
				// Enable UUID for proper disk identification in Kubernetes
			},
		},
		CapacityInKB: spec.DiskSizeGB * 1024 * 1024,
	}

	configSpec.DeviceChange = append(configSpec.DeviceChange, &types.VirtualDeviceConfigSpec{
		Operation:     types.VirtualDeviceConfigSpecOperationAdd,
		FileOperation: types.VirtualDeviceConfigSpecFileOperationCreate,
		Device:        disk,
	})

	// Add CD-ROM with ISO if specified
	if spec.ISOPath != "" {
		sataController := &types.VirtualAHCIController{
			VirtualSATAController: types.VirtualSATAController{
				VirtualController: types.VirtualController{
					VirtualDevice: types.VirtualDevice{
						Key: 200,
					},
				},
			},
		}

		configSpec.DeviceChange = append(configSpec.DeviceChange, &types.VirtualDeviceConfigSpec{
			Operation: types.VirtualDeviceConfigSpecOperationAdd,
			Device:    sataController,
		})

		cdrom := &types.VirtualCdrom{
			VirtualDevice: types.VirtualDevice{
				Key:           3000,
				ControllerKey: 200,
				Backing: &types.VirtualCdromIsoBackingInfo{
					VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
						FileName: spec.ISOPath,
					},
				},
				Connectable: &types.VirtualDeviceConnectInfo{
					StartConnected:    true,
					AllowGuestControl: true,
					Connected:         true,
				},
			},
		}

		configSpec.DeviceChange = append(configSpec.DeviceChange, &types.VirtualDeviceConfigSpec{
			Operation: types.VirtualDeviceConfigSpecOperationAdd,
			Device:    cdrom,
		})
	}

	// Create the VM
	task, err := folder.CreateVM(ctx, configSpec, resourcePool, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for VM creation: %w", err)
	}

	vm := object.NewVirtualMachine(c.client.Client, info.Result.(types.ManagedObjectReference))
	return vm, nil
}

// PowerOnVM powers on a virtual machine
func (c *Client) PowerOnVM(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.PowerOn(ctx)
	if err != nil {
		return fmt.Errorf("failed to power on VM: %w", err)
	}

	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("failed to wait for VM power on: %w", err)
	}

	return nil
}

// PowerOffVM powers off a virtual machine
func (c *Client) PowerOffVM(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.PowerOff(ctx)
	if err != nil {
		return fmt.Errorf("failed to power off VM: %w", err)
	}

	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("failed to wait for VM power off: %w", err)
	}

	return nil
}

// DestroyVM destroys a virtual machine
func (c *Client) DestroyVM(ctx context.Context, vm *object.VirtualMachine) error {
	// Get power state
	var props mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"runtime.powerState"}, &props); err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	// Power off if running
	if props.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
		if err := c.PowerOffVM(ctx, vm); err != nil {
			return fmt.Errorf("failed to power off VM before deletion: %w", err)
		}
	}

	// Destroy the VM
	task, err := vm.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("failed to destroy VM: %w", err)
	}

	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("failed to wait for VM destruction: %w", err)
	}

	return nil
}

// FindVMByName finds a VM by name in the datacenter
func (c *Client) FindVMByName(ctx context.Context, name string) (*object.VirtualMachine, error) {
	vm, err := c.finder.VirtualMachine(ctx, name)
	if err != nil {
		return nil, err
	}
	return vm, nil
}

// ListVMsInFolder lists all VMs in a folder
func (c *Client) ListVMsInFolder(ctx context.Context, folder *object.Folder) ([]*object.VirtualMachine, error) {
	vms, err := folder.Children(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs in folder: %w", err)
	}

	result := make([]*object.VirtualMachine, 0)
	for _, child := range vms {
		if child.Reference().Type == "VirtualMachine" {
			result = append(result, object.NewVirtualMachine(c.client.Client, child.Reference()))
		}
	}

	return result, nil
}

// GetVMInfo gets information about a VM
func (c *Client) GetVMInfo(ctx context.Context, vm *object.VirtualMachine) (*mo.VirtualMachine, error) {
	var props mo.VirtualMachine
	// Use minimal set of properties - just name, config (which includes uuid), runtime, and config.extraConfig
	if err := vm.Properties(ctx, vm.Reference(), []string{
		"name",
		"config",
		"runtime",
	}, &props); err != nil {
		return nil, fmt.Errorf("failed to get VM properties: %w", err)
	}

	return &props, nil
}

// SetVMTag sets a tag on a VM by storing it in ExtraConfig
// This allows tracking which template owns a VM
func (c *Client) SetVMTag(ctx context.Context, vm *object.VirtualMachine, key, value string) error {
	configSpec := types.VirtualMachineConfigSpec{
		ExtraConfig: []types.BaseOptionValue{
			&types.OptionValue{
				Key:   fmt.Sprintf("guestinfo.vmware-hcp.%s", key),
				Value: value,
			},
		},
	}

	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to set VM tag %s: %w", key, err)
	}

	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("failed to wait for VM tag update: %w", err)
	}

	return nil
}

// GetVMTag retrieves a tag from a VM's ExtraConfig
func GetVMTag(vmInfo *mo.VirtualMachine, key string) string {
	if vmInfo.Config == nil || vmInfo.Config.ExtraConfig == nil {
		return ""
	}

	fullKey := fmt.Sprintf("guestinfo.vmware-hcp.%s", key)
	for _, opt := range vmInfo.Config.ExtraConfig {
		if option, ok := opt.(*types.OptionValue); ok {
			if option.Key == fullKey && option.Value != nil {
				return option.Value.(string)
			}
		}
	}
	return ""
}

// UploadISO uploads an ISO file to a datastore
func (c *Client) UploadISO(ctx context.Context, datastore *object.Datastore, isoURL, destPath string) error {
	// Create HTTP client that skips TLS verification for self-signed certificates
	// TODO: Consider adding a CA certificate option in the future for production use
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	// Download ISO from URL
	resp, err := client.Get(isoURL)
	if err != nil {
		return fmt.Errorf("failed to download ISO from %s: %w", isoURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download ISO: HTTP %d", resp.StatusCode)
	}

	// Upload to datastore
	dsURL := datastore.NewURL(destPath)
	p := soap.DefaultUpload
	p.ContentLength = resp.ContentLength

	if err := c.client.Client.Upload(ctx, resp.Body, dsURL, &p); err != nil {
		return fmt.Errorf("failed to upload ISO to datastore: %w", err)
	}

	return nil
}

// ISOExists checks if an ISO file exists in the datastore
func (c *Client) ISOExists(ctx context.Context, datastore *object.Datastore, isoPath string) (bool, error) {
	// Parse the ISO path to extract the file path within the datastore
	// Format: "[datastore-name] path/to/file.iso"
	filePath := isoPath
	if len(isoPath) > 0 && isoPath[0] == '[' {
		// Extract path after datastore name
		endBracket := -1
		for i, c := range isoPath {
			if c == ']' {
				endBracket = i
				break
			}
		}
		if endBracket > 0 && len(isoPath) > endBracket+1 {
			filePath = isoPath[endBracket+2:] // Skip "] "
		}
	}

	// Use browser to check if file exists
	browser, err := datastore.Browser(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get datastore browser: %w", err)
	}

	// Get directory from file path
	dir := path.Dir(filePath)
	if dir == "." {
		dir = ""
	}

	spec := types.HostDatastoreBrowserSearchSpec{
		MatchPattern: []string{path.Base(filePath)},
	}

	dsPath := datastore.Path(dir)
	task, err := browser.SearchDatastore(ctx, dsPath, &spec)
	if err != nil {
		return false, err
	}

	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		// File not found
		if soap.IsSoapFault(err) {
			return false, nil
		}
		return false, err
	}

	if info.Result == nil {
		return false, nil
	}

	return true, nil
}

// GetDatastorePath returns the full datastore path for a file
func GetDatastorePath(datastoreName, filePath string) string {
	return fmt.Sprintf("[%s] %s", datastoreName, filePath)
}

// SanitizeFileName returns a sanitized filename suitable for datastore paths
func SanitizeFileName(name string) string {
	return path.Base(path.Clean(name))
}

// UUIDToVMwareSerialNumber converts a standard UUID to VMware SerialNumber format
// Input:  "4229222f-c6aa-3982-6e37-49b63174967f"
// Output: "VMware-42 29 22 2f c6 aa 39 82-6e 37 49 b6 31 74 96 7f"
func UUIDToVMwareSerialNumber(uuid string) string {
	if uuid == "" {
		return ""
	}

	// Remove all dashes from UUID
	clean := ""
	for _, c := range uuid {
		if c != '-' {
			clean += string(c)
		}
	}

	// Ensure we have exactly 32 hex characters (16 bytes)
	if len(clean) != 32 {
		return ""
	}

	// Insert spaces every 2 characters
	spaced := ""
	for i := 0; i < len(clean); i += 2 {
		if i > 0 {
			spaced += " "
		}
		spaced += clean[i : i+2]
	}

	// Insert dash after first 8 bytes (16 hex chars = 8 bytes with spaces = position 23)
	// Format: "42 29 22 2f c6 aa 39 82-6e 37 49 b6 31 74 96 7f"
	// Position 23 is after "42 29 22 2f c6 aa 39 82"
	if len(spaced) >= 24 {
		spaced = spaced[:23] + "-" + spaced[24:]
	}

	return "VMware-" + spaced
}
