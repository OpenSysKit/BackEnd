package driver

// IOCTL 控制码构造。
// Windows IOCTL 控制码格式: ((DeviceType) << 16) | ((Access) << 14) | ((Function) << 2) | (Method)
//
// 参考: https://learn.microsoft.com/en-us/windows-hardware/drivers/kernel/defining-i-o-control-codes

const (
	fileDeviceUnknown = 0x00000022
	methodBuffered    = 0
	fileAnyAccess     = 0
)

// CTL_CODE 按 Windows 约定构造 IOCTL 控制码。
func CTL_CODE(deviceType, function, method, access uint32) uint32 {
	return (deviceType << 16) | (access << 14) | (function << 2) | method
}

// TODO: 驱动开发时在这里补充实际的控制码，示例：
// var IOCTL_GET_PROCESS_LIST = CTL_CODE(fileDeviceUnknown, 0x800, methodBuffered, fileAnyAccess)
