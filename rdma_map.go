package rdmamap

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	netlink "github.com/vishvananda/netlink"
)

const (
	RdmaClassName = "infiniband"

	RdmaUcmFilePrefix = "ucm"

	RdmaIssmFilePrefix = "issm"
	RdmaUmadFilxPrefix = "umad"

	RdmaUverbsFilxPrefix = "uverbs"

	RdmaGidAttrDir     = "gid_attrs" //nolint:stylecheck,revive
	RdmaGidAttrNdevDir = "ndevs"     //nolint:stylecheck,revive
	RdmaPortsdir       = "ports"

	RdmaNodeGuidFile = "node_guid" //nolint:stylecheck,revive

	RdmaCountersDir   = "counters"
	RdmaHwCountersDir = "hw_counters"

	// For local usage
	prevDir        = ".."
	nibbleBitSize  = 4
	loopBackIfName = "lo"

	ReadOnlyPermissions = 0444
)

var (
	RdmaClassDir = "/sys/class/infiniband"
	RdmaIbUcmDir = "/sys/class/infiniband_cm"

	RdmaUmadDir = "/sys/class/infiniband_mad"

	RdmaUverbsDir = "/sys/class/infiniband_verbs"

	RdmaUcmDevice = "/dev/infiniband/rdma_cm"
	RdmaDeviceDir = "/dev/infiniband"

	PciDevDir = "/sys/bus/pci/devices"
	AuxDevDir = "/sys/bus/auxiliary/devices"
)

// GetRdmaDeviceList Returns a list of rdma device names
//
//nolint:prealloc
func GetRdmaDeviceList() []string {
	var rdmaDevices []string
	fd, err := os.Open(RdmaClassDir)
	if err != nil {
		return nil
	}
	defer fd.Close()

	fileInfos, err := fd.Readdir(-1)
	if err != nil {
		return nil
	}

	for i := range fileInfos {
		if fileInfos[i].IsDir() {
			continue
		}
		rdmaDevices = append(rdmaDevices, fileInfos[i].Name())
	}
	return rdmaDevices
}

func isDirForRdmaDevice(rdmaDeviceName, dirName string) bool {
	fileName := filepath.Join(dirName, "ibdev")

	fd, err := os.OpenFile(fileName, os.O_RDONLY, ReadOnlyPermissions)
	if err != nil {
		return false
	}
	defer fd.Close()

	if _, err = fd.Seek(0, io.SeekStart); err != nil {
		return false
	}

	data, err := io.ReadAll(fd)
	if err != nil {
		return false
	}
	return strings.Trim(string(data), "\n") == rdmaDeviceName
}

func getCharDevice(rdmaDeviceName, classDir, charDevPrefix string) (string, error) {
	fd, err := os.Open(classDir)
	if err != nil {
		return "", err
	}
	defer fd.Close()
	fileInfos, err := fd.Readdir(-1)
	if err != nil {
		return "", nil
	}

	for i := range fileInfos {
		if fileInfos[i].Name() == "." || fileInfos[i].Name() == prevDir {
			continue
		}
		if !strings.Contains(fileInfos[i].Name(), charDevPrefix) {
			continue
		}
		dirName := filepath.Join(classDir, fileInfos[i].Name())
		if !isDirForRdmaDevice(rdmaDeviceName, dirName) {
			continue
		}
		deviceFile := filepath.Join("/dev/infiniband", fileInfos[i].Name()) //nolint:gocritic
		return deviceFile, nil
	}
	return "", fmt.Errorf("no ucm device found")
}

func getUcmDevice(rdmaDeviceName string) (string, error) {
	return getCharDevice(rdmaDeviceName,
		RdmaIbUcmDir,
		RdmaUcmFilePrefix)
}

func getIssmDevice(rdmaDeviceName string) (string, error) {
	return getCharDevice(rdmaDeviceName,
		RdmaUmadDir,
		RdmaIssmFilePrefix)
}

func getUmadDevice(rdmaDeviceName string) (string, error) {
	return getCharDevice(rdmaDeviceName,
		RdmaUmadDir,
		RdmaUmadFilxPrefix)
}

func getUverbDevice(rdmaDeviceName string) (string, error) {
	return getCharDevice(rdmaDeviceName,
		RdmaUverbsDir,
		RdmaUverbsFilxPrefix)
}

func getRdmaUcmDevice() (string, error) {
	info, err := os.Stat(RdmaUcmDevice)
	if err != nil {
		return "", err
	}
	if info.Name() == "rdma_cm" {
		return RdmaUcmDevice, nil
	}

	return "", fmt.Errorf("invalid file name rdma_cm")
}

// Returns a list of character device absolute path for a requested
// rdmaDeviceName.
// Returns nil if no character devices are found.
func GetRdmaCharDevices(rdmaDeviceName string) []string {
	var rdmaCharDevices []string

	ucm, err := getUcmDevice(rdmaDeviceName)
	if err == nil {
		rdmaCharDevices = append(rdmaCharDevices, ucm)
	}
	issm, err := getIssmDevice(rdmaDeviceName)
	if err == nil {
		rdmaCharDevices = append(rdmaCharDevices, issm)
	}
	umad, err := getUmadDevice(rdmaDeviceName)
	if err == nil {
		rdmaCharDevices = append(rdmaCharDevices, umad)
	}
	uverb, err := getUverbDevice(rdmaDeviceName)
	if err == nil {
		rdmaCharDevices = append(rdmaCharDevices, uverb)
	}
	rdmaCm, err := getRdmaUcmDevice()
	if err == nil {
		rdmaCharDevices = append(rdmaCharDevices, rdmaCm)
	}

	return rdmaCharDevices
}

// Gets a list of ports for a specified device
//
//nolint:prealloc
func GetPorts(rdmaDeviceName string) []string {
	var ports []string

	portsDir := filepath.Join(RdmaClassDir, rdmaDeviceName, RdmaPortsdir)
	fd, err := os.Open(portsDir)
	if err != nil {
		return nil
	}
	defer fd.Close()

	fileInfos, err := fd.Readdir(-1)
	if err != nil {
		return nil
	}

	for i := range fileInfos {
		if fileInfos[i].Name() == "." || fileInfos[i].Name() == prevDir {
			continue
		}
		ports = append(ports, fileInfos[i].Name())
	}
	return ports
}

//nolint:prealloc
func getNetdeviceIds(rdmaDeviceName, port string) []string {
	var indices []string

	dir := filepath.Join(RdmaClassDir, rdmaDeviceName, RdmaPortsdir, port,
		RdmaGidAttrDir, RdmaGidAttrNdevDir)

	fd, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer fd.Close()

	fileInfos, err := fd.Readdir(-1)
	if err != nil {
		return nil
	}

	for i := range fileInfos {
		if fileInfos[i].Name() == "." || fileInfos[i].Name() == prevDir {
			continue
		}
		indices = append(indices, fileInfos[i].Name())
	}
	return indices
}

func isNetdevForRdma(rdmaDeviceName, port, index, netdevName string) bool {
	fileName := filepath.Join(RdmaClassDir, rdmaDeviceName, RdmaPortsdir, port,
		RdmaGidAttrDir, RdmaGidAttrNdevDir, index)

	fd, err := os.OpenFile(fileName, os.O_RDONLY, ReadOnlyPermissions)
	if err != nil {
		return false
	}
	defer fd.Close()

	if _, err = fd.Seek(0, io.SeekStart); err != nil {
		return false
	}

	data, err := io.ReadAll(fd)
	if err != nil {
		return false
	}
	return (strings.TrimSuffix(string(data), "\n") == netdevName)
}

func getRdmaDeviceForEth(netdevName string) (string, error) {
	// Iterate over the list of rdma devices,
	// read the gid table attribute netdev
	// if the netdev matches, found the matching rdma device

	devices := GetRdmaDeviceList()
	for _, dev := range devices {
		ports := GetPorts(dev)
		for _, port := range ports {
			indices := getNetdeviceIds(dev, port)
			for _, index := range indices {
				found := isNetdevForRdma(dev, port, index, netdevName)
				if found {
					return dev, nil
				}
			}
		}
	}
	return "", fmt.Errorf("rdma device not found for netdev %v", netdevName)
}

func getNodeGUID(rdmaDeviceName string) ([]byte, error) {
	var nodeGUID []byte

	fileName := filepath.Join(RdmaClassDir, rdmaDeviceName, RdmaNodeGuidFile)

	fd, err := os.OpenFile(fileName, os.O_RDONLY, ReadOnlyPermissions)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	if _, err = fd.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}
	data = data[:len(data)-1]
	var j int
	for _, b := range data {
		if b == ':' {
			continue
		}
		c, err := strconv.ParseUint(string(b), 16, 8)
		if err != nil {
			return nil, err
		}
		if (j % 2) == 0 {
			nodeGUID = append(nodeGUID, byte(c)<<nibbleBitSize)
		} else {
			nodeGUID[j/2] |= byte(c)
		}
		j++
	}
	return nodeGUID, nil
}

func getRdmaDeviceForIb(linkAttr *netlink.LinkAttrs) (string, error) {
	// Match the node_guid EUI bytes with the IpoIB netdevice hw address EUI
	lleui64 := linkAttr.HardwareAddr[12:]

	devices := GetRdmaDeviceList()
	for _, dev := range devices {
		nodeGUID, err := getNodeGUID(dev)
		if err != nil {
			return "", err
		}
		if bytes.Equal(lleui64, nodeGUID) {
			return dev, nil
		}
	}
	return "", nil
}

// Get RDMA device for the netdevice
func GetRdmaDeviceForNetdevice(netdevName string) (string, error) {
	handle, err := netlink.LinkByName(netdevName)
	if err != nil {
		return "", err
	}
	netAttr := handle.Attrs()
	if netAttr.EncapType == "ether" {
		return getRdmaDeviceForEth(netdevName)
	} else if netAttr.EncapType == "infiniband" {
		return getRdmaDeviceForIb(netAttr)
	} else {
		return "", fmt.Errorf("unknown device type")
	}
}

// Returns true if rdma device exist for netdevice, else false
func IsRDmaDeviceForNetdevice(netdevName string) bool {
	rdma, _ := GetRdmaDeviceForNetdevice(netdevName)

	return (rdma != "")
}

//nolint:prealloc
func getRdmaDevicesFromDir(dirName string) []string {
	var rdmadevs []string

	entries, err := os.ReadDir(dirName)
	if err != nil {
		return rdmadevs
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rdmadevs = append(rdmadevs, entry.Name())
	}
	return rdmadevs
}

// Get list of RDMA devices for a pci device.
// When switchdev mode is used, there may be more than one rdma device.
// Example pcidevName: 0000:05:00.0,
// when found, returns list of devices one or more devices names such as
// mlx5_0, mlx5_10
func GetRdmaDevicesForPcidev(pcidevName string) []string {
	dirName := filepath.Join(PciDevDir, pcidevName, RdmaClassName)
	return getRdmaDevicesFromDir(dirName)
}

// Get list of RDMA devices for an auxiliary device.
// When switchdev mode is used, there may be more than one rdma device.
// Example deviceID: mlx5_core.sf.4,
// when found, returns list of devices one or more devices names such as
// mlx5_0, mlx5_10
func GetRdmaDevicesForAuxdev(deviceID string) []string {
	dirName := filepath.Join(AuxDevDir, deviceID, RdmaClassName)
	return getRdmaDevicesFromDir(dirName)
}
