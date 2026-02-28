package driver

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// Device 定义与内核驱动交互的抽象接口。
// service 层依赖此接口而非具体实现，便于测试和解耦。
type Device interface {
	IoControl(code uint32, inBuf []byte, outSize uint32) ([]byte, error)
	Close() error
}

// Client 封装与 Windows 内核驱动的通信。
// 通过 CreateFile 打开设备句柄，通过 DeviceIoControl 收发数据。
// 实现 Device 接口。
type Client struct {
	mu     sync.Mutex
	handle syscall.Handle
	path   string
}

// Open 打开驱动设备
func Open(devicePath string) (*Client, error) {
	pathPtr, err := syscall.UTF16PtrFromString(devicePath)
	if err != nil {
		return nil, fmt.Errorf("设备路径转换失败: %w", err)
	}

	handle, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("打开设备失败 [%s]: %w", devicePath, err)
	}

	return &Client{handle: handle, path: devicePath}, nil
}

// Close 关闭设备句柄。
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.handle == syscall.InvalidHandle {
		return nil
	}
	err := syscall.CloseHandle(c.handle)
	c.handle = syscall.InvalidHandle
	return err
}

// IoControl 发送 IOCTL 请求到内核驱动。
//   - code:    IOCTL 控制码
//   - inBuf:   输入缓冲区（可为 nil）
//   - outSize: 期望的输出缓冲区大小（字节）
//
// 返回驱动写回的输出数据。
func (c *Client) IoControl(code uint32, inBuf []byte, outSize uint32) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.handle == syscall.InvalidHandle {
		return nil, fmt.Errorf("设备未打开")
	}

	var inPtr unsafe.Pointer
	var inLen uint32
	if len(inBuf) > 0 {
		inPtr = unsafe.Pointer(&inBuf[0])
		inLen = uint32(len(inBuf))
	}

	outBuf := make([]byte, outSize)
	var bytesReturned uint32

	var outPtr unsafe.Pointer
	if outSize > 0 {
		outPtr = unsafe.Pointer(&outBuf[0])
	}

	err := syscall.DeviceIoControl(
		c.handle,
		code,
		(*byte)(inPtr),
		inLen,
		(*byte)(outPtr),
		outSize,
		&bytesReturned,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("DeviceIoControl 失败 [code=0x%X]: %w", code, err)
	}

	return outBuf[:bytesReturned], nil
}
