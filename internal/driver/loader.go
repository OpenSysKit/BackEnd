package driver

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	loaderDeviceName = `\\.\DriverLoader`
	loaderSvcName    = "DriverLoader"
	loaderDispName   = "Driver Loader Service"

	// WinDrive IOCTLs
	ioctlLoadDriver   = 0x80002000 // CTL_CODE(0x8000, 0x800, METHOD_BUFFERED, FILE_ANY_ACCESS)
	ioctlUnloadDriver = 0x80002004 // CTL_CODE(0x8000, 0x801, METHOD_BUFFERED, FILE_ANY_ACCESS)
	ioctlAllowUnload  = 0x8000200C // CTL_CODE(0x8000, 0x803, METHOD_BUFFERED, FILE_ANY_ACCESS)

	maxDriverPath = 520
)

// loaderRequest 请求结构
type loadDriverRequest struct {
	DriverPath [maxDriverPath]uint16
	Flags      uint32
}

type loadDriverResponse struct {
	Status       uint32
	DriverBase   uint64
	DriverHandle uint64
}

type unloadDriverRequest struct {
	DriverHandle uint64
}

// Loader 管理 WinDrive 加载器
type Loader struct {
	handle        syscall.Handle
	m             *mgr.Mgr
	mappedHandles []uint64
	ownService    bool
}

// NewLoader 创建并连接到加载器，必要时安装服务
func NewLoader(loaderSysPath string) (*Loader, error) {
	l := &Loader{handle: syscall.InvalidHandle}

	// 尝试直接打开
	if err := l.open(); err == nil {
		l.ownService = false
		return l, nil
	}

	// 没打开，尝试通过服务安装
	m, err := mgr.Connect()
	if err != nil {
		return nil, fmt.Errorf("无法连接服务管理器(需要管理员权限): %w", err)
	}
	l.m = m
	l.ownService = true

	if err := l.installAndStart(loaderSysPath); err != nil {
		l.m.Disconnect()
		return nil, fmt.Errorf("安装加载器服务失败: %w", err)
	}

	// 驱动启动需要一定时间来注册符号链接，采用带延时的重试机制
	var openErr error
	for i := 0; i < 10; i++ {
		if openErr = l.open(); openErr == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if openErr != nil {
		l.m.Disconnect()
		return nil, fmt.Errorf("服务启动后仍无法打开设备(重试后): %w", openErr)
	}

	return l, nil
}

// open 打开加载器设备
func (l *Loader) open() error {
	pathPtr, err := syscall.UTF16PtrFromString(loaderDeviceName)
	if err != nil {
		return err
	}
	h, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return err
	}
	l.handle = h
	return nil
}

// installAndStart 安装并启动服务
func (l *Loader) installAndStart(sysPath string) error {
	// 获取绝对路径
	fullPath, err := syscall.FullPath(sysPath)
	if err != nil {
		return err
	}
	fmt.Printf("[Loader] 准备安装服务, 驱动路径: %s\n", fullPath)

	s, err := l.m.OpenService(loaderSvcName)
	if err == nil {
		// 服务已存在，为防止之前的路径错误或文件位置改变，我们可以尝试更新配置或直接删除重建
		// 简单起见，如果存在我们就直接更新 Config 中的 BinaryPathName
		cfg, err := s.Config()
		if err == nil && cfg.BinaryPathName != fullPath {
			fmt.Printf("[Loader] 更新已存在服务的驱动路径从 %s 到 %s\n", cfg.BinaryPathName, fullPath)
			cfg.BinaryPathName = fullPath
			s.UpdateConfig(cfg)
		}
	} else {
		// 服务不存在，创建它
		cfg := mgr.Config{
			ServiceType:  windows.SERVICE_KERNEL_DRIVER,
			StartType:    mgr.StartManual,
			ErrorControl: mgr.ErrorNormal,
			DisplayName:  loaderDispName,
		}
		s, err = l.m.CreateService(loaderSvcName, fullPath, cfg)
		if err != nil {
			return fmt.Errorf("CreateService err: %w", err)
		}
	}
	defer s.Close()

	// 启动服务
	err = s.Start()
	if err != nil {
		// 这里可能是 ERROR_SERVICE_ALREADY_RUNNING 等等
		// 记录一下但不直接退出，因为后续通过 open 探测才最准确
		fmt.Printf("[Loader] StartService result: %v\n", err)
	}

	return nil
}

// MapDriver 使用 WinDrive 映射未签名驱动
func (l *Loader) MapDriver(sysPath string) (uint64, error) {
	fullPath, err := syscall.FullPath(sysPath)
	if err != nil {
		return 0, err
	}
	// 转为 NT 路径
	ntPath := "\\??\\" + fullPath
	ntPath16, err := syscall.UTF16FromString(ntPath)
	if err != nil {
		return 0, err
	}

	req := loadDriverRequest{
		Flags: 3, // LOAD_FLAG_SKIP_SIGNATURE | LOAD_FLAG_CALL_ENTRY
	}
	copy(req.DriverPath[:], ntPath16)

	var resp loadDriverResponse
	var bytesReturned uint32

	err = syscall.DeviceIoControl(
		l.handle,
		ioctlLoadDriver,
		(*byte)(unsafe.Pointer(&req)),
		uint32(unsafe.Sizeof(req)),
		(*byte)(unsafe.Pointer(&resp)),
		uint32(unsafe.Sizeof(resp)),
		&bytesReturned,
		nil,
	)

	if err != nil {
		return 0, fmt.Errorf("映射请求失败: %w", err)
	}

	if resp.Status != 0 {
		return 0, fmt.Errorf("加载器返回错误状态: 0x%x", resp.Status)
	}

	l.mappedHandles = append(l.mappedHandles, resp.DriverHandle)
	return resp.DriverHandle, nil
}

// Close 释放资源，包含卸载已映射的驱动和停止加载器
func (l *Loader) Close() {
	if l.handle == syscall.InvalidHandle {
		return
	}

	// 1. 卸载所有映射的驱动
	for _, handle := range l.mappedHandles {
		req := unloadDriverRequest{DriverHandle: handle}
		var returned uint32
		_ = syscall.DeviceIoControl(
			l.handle,
			ioctlUnloadDriver,
			(*byte)(unsafe.Pointer(&req)),
			uint32(unsafe.Sizeof(req)),
			nil,
			0,
			&returned,
			nil,
		)
	}
	l.mappedHandles = nil

	// 2. 关闭设备句柄
	syscall.Close(l.handle)
	l.handle = syscall.InvalidHandle

	// 3. 不主动卸载 DriverLoader 服务，保持加载器常驻。
	// 仅释放当前进程资源，避免服务卸载链路带来的系统稳定性风险。
	if l.m != nil {
		l.m.Disconnect()
		l.m = nil
	}
}
