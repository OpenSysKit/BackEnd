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

// IOCTL 控制码 参考 Driver/src/driver.h
var (
	IOCTL_ENUM_PROCESSES    = CTL_CODE(deviceTypeOpenSysKit, 0x800, methodBuffered, fileAnyAccess)
	IOCTL_KILL_PROCESS      = CTL_CODE(deviceTypeOpenSysKit, 0x801, methodBuffered, fileAnyAccess)
	IOCTL_FREEZE_PROCESS    = CTL_CODE(deviceTypeOpenSysKit, 0x802, methodBuffered, fileAnyAccess)
	IOCTL_UNFREEZE_PROCESS  = CTL_CODE(deviceTypeOpenSysKit, 0x803, methodBuffered, fileAnyAccess)
	IOCTL_PROTECT_PROCESS   = CTL_CODE(deviceTypeOpenSysKit, 0x804, methodBuffered, fileAnyAccess)
	IOCTL_UNPROTECT_PROCESS = CTL_CODE(deviceTypeOpenSysKit, 0x805, methodBuffered, fileAnyAccess)
)
