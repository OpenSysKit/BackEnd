package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/OpenSysKit/backend/internal/driver"
)

// runInlineUninstall 在当前进程内同步卸载映射驱动，
// 使用已持有的 loader 实例，无需启动子进程。
// 调用前必须已关闭 OpenSysKit 设备句柄。
func runInlineUninstall(loader *driver.Loader, handles []uint64) error {
	if loader == nil {
		return fmt.Errorf("loader 不可用")
	}

	sort.Slice(handles, func(i, j int) bool { return handles[i] > handles[j] })

	log.Printf("[uninstall] 同步卸载 handles: %s", formatHandleList(handles))
	for _, handle := range handles {
		if err := unloadHandleWithRetry(loader, handle, 5); err != nil {
			return err
		}
	}

	after, err := loader.ListMappedDrivers()
	if err != nil {
		return err
	}
	if len(after) != 0 {
		return fmt.Errorf("卸载后仍存在映射驱动: %s", formatHandles(after))
	}

	log.Println("[uninstall] 下发 WinDrive allow-unload")
	if err = loader.AllowUnload(); err != nil {
		return err
	}

	loader.Close()

	log.Println("[uninstall] 卸载 DriverLoader 服务")
	if err = driver.UninstallLoaderService(); err != nil {
		return err
	}

	return nil
}

func runUninstallMode() error {
	log.Println("[uninstall] 进入手动卸载模式")

	loader, err := driver.OpenExistingLoader()
	if err != nil {
		return fmt.Errorf("无法连接 WinDrive(\\\\.\\DriverLoader): %w", err)
	}
	defer loader.Close()

	drivers, err := loader.ListMappedDrivers()
	if err != nil {
		return err
	}

	log.Printf("[uninstall] 当前 WinDrive 映射驱动数量: %d", len(drivers))
	if len(drivers) == 0 {
		log.Println("[uninstall] 未检测到映射驱动，跳过 OpenSysKit 卸载")
	} else {
		targetHandles, parseErr := resolveTargetHandlesFromArgs(drivers)
		if parseErr != nil {
			return parseErr
		}
		sort.Slice(targetHandles, func(i, j int) bool { return targetHandles[i] > targetHandles[j] })

		if waitForDeviceRelease(`\\.\OpenSysKit`, 15*time.Second) {
			log.Println("[uninstall] 设备引用已释放，开始卸载")
		} else {
			log.Println("[uninstall] 警告: 设备可能仍被占用，仍尝试卸载")
		}

		log.Printf("[uninstall] 计划卸载 handle: %s", formatHandleList(targetHandles))
		for _, handle := range targetHandles {
			if err = unloadHandleWithRetry(loader, handle, 5); err != nil {
				return err
			}
		}
	}

	after, err := loader.ListMappedDrivers()
	if err != nil {
		return err
	}
	if len(after) != 0 {
		return fmt.Errorf("OpenSysKit 卸载后仍存在映射驱动，拒绝卸载 WinDrive: %s", formatHandles(after))
	}

	log.Println("[uninstall] 下发 WinDrive allow-unload")
	if err = loader.AllowUnload(); err != nil {
		return err
	}

	loader.Close()

	log.Println("[uninstall] 卸载 DriverLoader 服务")
	if err = driver.UninstallLoaderService(); err != nil {
		return err
	}

	log.Println("[uninstall] 卸载完成")
	return nil
}

func formatHandles(rows []driver.LoadedDriverInfo) string {
	if len(rows) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, fmt.Sprintf("%d", row.Handle))
	}
	return strings.Join(parts, ",")
}

func resolveTargetHandlesFromArgs(drivers []driver.LoadedDriverInfo) ([]uint64, error) {
	// 支持:
	// 1) OpenSysKit.exe uninstall            -> 卸载全部映射驱动
	// 2) OpenSysKit.exe uninstall 1 2        -> 指定 handle 列表
	// 3) OpenSysKit.exe uninstall --handle=1
	// 4) OpenSysKit.exe uninstall --handles=1,2
	if len(os.Args) <= 2 {
		out := make([]uint64, 0, len(drivers))
		for _, d := range drivers {
			out = append(out, d.Handle)
		}
		return dedupHandles(out), nil
	}

	existing := make(map[uint64]struct{}, len(drivers))
	for _, d := range drivers {
		existing[d.Handle] = struct{}{}
	}

	var requested []uint64
	for _, arg := range os.Args[2:] {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}

		switch {
		case strings.HasPrefix(arg, "--handle="):
			v := strings.TrimSpace(strings.TrimPrefix(arg, "--handle="))
			h, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("无效 handle 参数: %s", arg)
			}
			requested = append(requested, h)
		case strings.HasPrefix(arg, "--handles="):
			list := strings.TrimSpace(strings.TrimPrefix(arg, "--handles="))
			for _, part := range strings.Split(list, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				h, err := strconv.ParseUint(part, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("无效 handles 参数: %s", arg)
				}
				requested = append(requested, h)
			}
		default:
			// 兼容直接传多个 handle（如 uninstall 1 2）
			h, err := strconv.ParseUint(arg, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("无法解析参数为 handle: %s", arg)
			}
			requested = append(requested, h)
		}
	}

	requested = dedupHandles(requested)
	if len(requested) == 0 {
		return nil, fmt.Errorf("未解析到有效 handle 参数")
	}
	for _, h := range requested {
		if _, ok := existing[h]; !ok {
			return nil, fmt.Errorf("指定的 handle 不存在: %d (当前映射: %s)", h, formatHandles(drivers))
		}
	}
	return requested, nil
}

func dedupHandles(in []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(in))
	out := make([]uint64, 0, len(in))
	for _, h := range in {
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	return out
}

func formatHandleList(handles []uint64) string {
	if len(handles) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(handles))
	for _, h := range handles {
		parts = append(parts, fmt.Sprintf("%d", h))
	}
	return strings.Join(parts, ",")
}

// waitForDeviceRelease 轮询检测 OpenSysKit 设备是否已无其他占用者。
// 尝试以独占模式打开设备：成功则说明无其他句柄，立即关闭并返回 true。
// ERROR_SHARING_VIOLATION 说明仍有占用，继续等待。
// 其他错误（如设备不存在）也视为"已释放"。
func waitForDeviceRelease(devicePath string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	pathPtr, err := syscall.UTF16PtrFromString(devicePath)
	if err != nil {
		return false
	}

	const ERROR_SHARING_VIOLATION syscall.Errno = 32

	for time.Now().Before(deadline) {
		h, err := syscall.CreateFile(
			pathPtr,
			syscall.GENERIC_READ|syscall.GENERIC_WRITE,
			0, // dwShareMode=0 → 独占
			nil,
			syscall.OPEN_EXISTING,
			syscall.FILE_ATTRIBUTE_NORMAL,
			0,
		)
		if err == nil {
			syscall.CloseHandle(h)
			return true
		}
		if errno, ok := err.(syscall.Errno); ok && errno == ERROR_SHARING_VIOLATION {
			log.Printf("[uninstall] 设备 %s 仍被占用，等待释放...", devicePath)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		// 设备不存在或其他错误 → 视为已释放
		return true
	}
	return false
}

func unloadHandleWithRetry(loader *driver.Loader, handle uint64, maxAttempts int) error {
	var lastErr error
	for i := 1; i <= maxAttempts; i++ {
		err := loader.UnloadMappedDriver(handle)
		if err == nil {
			log.Printf("[uninstall] 卸载成功 handle=%d (attempt=%d)", handle, i)
			return nil
		}
		lastErr = err
		log.Printf("[uninstall] 卸载失败 handle=%d (attempt=%d/%d): %v", handle, i, maxAttempts, err)
		if i < maxAttempts {
			backoff := time.Duration(i) * 2 * time.Second
			log.Printf("[uninstall] 等待 %v 后重试（等待设备引用释放）...", backoff)
			time.Sleep(backoff)
		}
	}
	return fmt.Errorf("卸载映射驱动失败(handle=%d): %w", handle, lastErr)
}
