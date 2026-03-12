package driver

// IOCTL 控制码构造。
// Windows IOCTL 控制码格式: ((DeviceType) << 16) | ((Access) << 14) | ((Function) << 2) | (Method)
//
// 参考: https://learn.microsoft.com/en-us/windows-hardware/drivers/kernel/defining-i-o-control-codes

const (
	deviceTypeOpenSysKit uint32 = 0x8000
	methodBuffered       uint32 = 0
	fileAnyAccess        uint32 = 0
)

// CTL_CODE 按 Windows 约定构造 IOCTL 控制码。
func CTL_CODE(deviceType, function, method, access uint32) uint32 {
	return (deviceType << 16) | (access << 14) | (function << 2) | method
}

// IOCTL 控制码，需与 Driver/src/driver.h 保持一致。
var (
	IOCTL_ENUM_PROCESSES       = CTL_CODE(deviceTypeOpenSysKit, 0x800, methodBuffered, fileAnyAccess)
	IOCTL_KILL_PROCESS         = CTL_CODE(deviceTypeOpenSysKit, 0x801, methodBuffered, fileAnyAccess)
	IOCTL_FREEZE_PROCESS       = CTL_CODE(deviceTypeOpenSysKit, 0x802, methodBuffered, fileAnyAccess)
	IOCTL_UNFREEZE_PROCESS     = CTL_CODE(deviceTypeOpenSysKit, 0x803, methodBuffered, fileAnyAccess)
	IOCTL_PROTECT_PROCESS      = CTL_CODE(deviceTypeOpenSysKit, 0x805, methodBuffered, fileAnyAccess)
	IOCTL_UNPROTECT_PROCESS    = CTL_CODE(deviceTypeOpenSysKit, 0x806, methodBuffered, fileAnyAccess)
	IOCTL_ELEVATE_PROCESS      = CTL_CODE(deviceTypeOpenSysKit, 0x807, methodBuffered, fileAnyAccess)
	IOCTL_ENUM_MODULES         = CTL_CODE(deviceTypeOpenSysKit, 0x808, methodBuffered, fileAnyAccess)
	IOCTL_READ_PROCESS_MEMORY  = CTL_CODE(deviceTypeOpenSysKit, 0x809, methodBuffered, fileAnyAccess)
	IOCTL_WRITE_PROCESS_MEMORY = CTL_CODE(deviceTypeOpenSysKit, 0x80A, methodBuffered, fileAnyAccess)
	IOCTL_ENUM_THREADS         = CTL_CODE(deviceTypeOpenSysKit, 0x80B, methodBuffered, fileAnyAccess)
	IOCTL_HIDE_PROCESS         = CTL_CODE(deviceTypeOpenSysKit, 0x80C, methodBuffered, fileAnyAccess)
	IOCTL_UNHIDE_PROCESS       = CTL_CODE(deviceTypeOpenSysKit, 0x80D, methodBuffered, fileAnyAccess)
	IOCTL_INJECT_DLL           = CTL_CODE(deviceTypeOpenSysKit, 0x80E, methodBuffered, fileAnyAccess)
	IOCTL_DELETE_FILE          = CTL_CODE(deviceTypeOpenSysKit, 0x810, methodBuffered, fileAnyAccess)
	IOCTL_ENUM_KERNEL_MODULES  = CTL_CODE(deviceTypeOpenSysKit, 0x820, methodBuffered, fileAnyAccess)
	IOCTL_UNLOAD_DRIVER        = CTL_CODE(deviceTypeOpenSysKit, 0x821, methodBuffered, fileAnyAccess)
	IOCTL_ENUM_HANDLES         = CTL_CODE(deviceTypeOpenSysKit, 0x830, methodBuffered, fileAnyAccess)
	IOCTL_CLOSE_HANDLE         = CTL_CODE(deviceTypeOpenSysKit, 0x831, methodBuffered, fileAnyAccess)
	IOCTL_ENUM_CONNECTIONS     = CTL_CODE(deviceTypeOpenSysKit, 0x850, methodBuffered, fileAnyAccess)
	IOCTL_DETACH_SYMLINK       = CTL_CODE(deviceTypeOpenSysKit, 0x8F0, methodBuffered, fileAnyAccess)

	// WinDrive (DriverLoader) process-protect IOCTLs。
	// 虽然 function 值与 OpenSysKit 的提权 IOCTL 有重叠，但设备句柄不同。
	IOCTL_WINDRIVE_PROTECT_PROCESS    = CTL_CODE(deviceTypeOpenSysKit, 0x807, methodBuffered, fileAnyAccess)
	IOCTL_WINDRIVE_UNPROTECT_PROCESS  = CTL_CODE(deviceTypeOpenSysKit, 0x808, methodBuffered, fileAnyAccess)
	IOCTL_WINDRIVE_SET_PROTECT_POLICY = CTL_CODE(deviceTypeOpenSysKit, 0x809, methodBuffered, fileAnyAccess)
)

const (
	ProcessKillResultVersion uint32 = 1

	ProcessKillMethodNone uint32 = 0
	ProcessKillMethodPsp  uint32 = 1
	ProcessKillMethodZw   uint32 = 2

	ElevateLevelAdmin            uint32 = 0
	ElevateLevelSystem           uint32 = 1
	ElevateLevelTrustedInstaller uint32 = 2
	ElevateLevelStandardUser     uint32 = 3

	ConnectionProtoTCP uint32 = 6
	ConnectionProtoUDP uint32 = 17
)

// ProcessRequest 对应内核中 PROCESS_REQUEST 结构体。
type ProcessRequest struct {
	ProcessId uint32
}

// ProcessElevateRequest 对应内核中 PROCESS_ELEVATE_REQUEST 结构体。
type ProcessElevateRequest struct {
	ProcessId uint32
	Level     uint32
}

// ProcessKillResult 对应内核中 PROCESS_KILL_RESULT 结构体。
type ProcessKillResult struct {
	Version         uint32
	OperationStatus uint32
	Method          uint32
	Reserved        uint32
}

// ProcessInfo 对应内核中 PROCESS_INFO 结构体。
type ProcessInfo struct {
	ProcessId       uint32
	ParentProcessId uint32
	ThreadCount     uint32
	Padding0        uint32
	WorkingSetSize  uint64
	ImageName       [260]uint16
}

// ProcessListHeader 对应内核中 PROCESS_LIST_HEADER 结构体。
type ProcessListHeader struct {
	Count     uint32
	TotalSize uint32
}

// FilePathRequest 对应内核中的 FILE_PATH_REQUEST。
type FilePathRequest struct {
	Path [520]uint16
}

// ProcessMemoryRequest 对应内核中的 PROCESS_MEMORY_REQUEST。
type ProcessMemoryRequest struct {
	ProcessId uint32
	Address   uint64
	Size      uint32
}

// ModuleInfo 对应内核中 MODULE_INFO 结构体。
type ModuleInfo struct {
	BaseAddress uint64
	SizeOfImage uint32
	FullPath    [520]uint16
	BaseName    [260]uint16
	Padding0    uint32
}

// ModuleListHeader 对应内核中 MODULE_LIST_HEADER 结构体。
type ModuleListHeader struct {
	Count     uint32
	TotalSize uint32
}

// ThreadInfo 对应内核中 THREAD_INFO 结构体。
type ThreadInfo struct {
	ThreadId      uint32
	ProcessId     uint32
	Priority      int32
	Padding0      uint32
	StartAddress  uint64
	IsTerminating uint8
	Padding1      [3]byte
	Padding2      uint32
}

// ThreadListHeader 对应内核中 THREAD_LIST_HEADER 结构体。
type ThreadListHeader struct {
	Count     uint32
	TotalSize uint32
}

// InjectDllRequest 对应内核中的 INJECT_DLL_REQUEST。
type InjectDllRequest struct {
	ProcessId uint32
	DllPath   [520]uint16
}

// KernelModuleInfo 对应内核中 KERNEL_MODULE_INFO 结构体。
type KernelModuleInfo struct {
	BaseAddress uint64
	SizeOfImage uint32
	FullPath    [520]uint16
	BaseName    [64]uint16
	Padding0    uint32
}

// KernelModuleListHeader 对应内核中 KERNEL_MODULE_LIST_HEADER 结构体。
type KernelModuleListHeader struct {
	Count     uint32
	TotalSize uint32
}

// DriverServiceRequest 对应内核中的 DRIVER_SERVICE_REQUEST。
type DriverServiceRequest struct {
	ServiceName [256]uint16
}

// HandleEnumRequest 对应内核中的 HANDLE_ENUM_REQUEST。
type HandleEnumRequest struct {
	ProcessId uint32
}

// HandleInfo 对应内核中的 HANDLE_INFO。
type HandleInfo struct {
	ProcessId       uint32
	Padding0        uint32
	Handle          uint64
	ObjectTypeIndex uint32
	GrantedAccess   uint32
	ObjectAddress   uint64
	TypeName        [64]uint16
	ObjectName      [260]uint16
}

// HandleListHeader 对应内核中的 HANDLE_LIST_HEADER。
type HandleListHeader struct {
	Count     uint32
	TotalSize uint32
}

// CloseHandleRequest 对应内核中的 CLOSE_HANDLE_REQUEST。
type CloseHandleRequest struct {
	ProcessId uint32
	Padding0  uint32
	Handle    uint64
}

// ConnectionInfo 对应内核中的 CONNECTION_INFO。
type ConnectionInfo struct {
	Protocol   uint32
	State      uint32
	ProcessId  uint32
	LocalAddr  [16]byte
	LocalPort  uint16
	RemoteAddr [16]byte
	RemotePort uint16
	IsIPv6     uint8
	Padding0   [1]byte
	Padding1   [2]byte
}

// ConnectionListHeader 对应内核中的 CONNECTION_LIST_HEADER。
type ConnectionListHeader struct {
	Count     uint32
	TotalSize uint32
}

// ProtectPolicyRequest 对应 WinDrive 中 PROTECT_POLICY_REQUEST 结构体。
type ProtectPolicyRequest struct {
	Version        uint32
	DenyAccessMask uint32
	Reserved       uint32
}
