//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/OpenSysKit/backend/internal/driver"
)

func runAutoUninstallMode() error {
	delay := parseAutoUninstallDelay()
	handles, err := parseAutoUninstallHandles()
	if err != nil {
		return err
	}
	if len(handles) == 0 {
		return fmt.Errorf("autouninstall 未提供可卸载的 handle")
	}

	log.Printf("autouninstall 子进程启动: delay=%s handles=%s", delay, formatHandleList(handles))
	if delay > 0 {
		time.Sleep(delay)
	}

	log.Printf("autouninstall 开始执行完整卸载")
	if err := runAutoUninstall(handles); err != nil {
		return fmt.Errorf("autouninstall 卸载失败: %w", err)
	}
	log.Printf("autouninstall 卸载完成")
	return nil
}

func runAutoUninstall(handles []uint64) error {
	loader, err := driver.OpenExistingLoader()
	if err != nil {
		return fmt.Errorf("autouninstall 无法连接 WinDrive(\\\\.\\DriverLoader): %w", err)
	}
	defer loader.Close()

	drivers, err := loader.ListMappedDrivers()
	if err != nil {
		return err
	}
	if len(drivers) == 0 {
		log.Printf("autouninstall 未检测到映射驱动，跳过 OpenSysKit 卸载")
	} else {
		existing := make(map[uint64]struct{}, len(drivers))
		for _, row := range drivers {
			existing[row.Handle] = struct{}{}
		}

		filtered := make([]uint64, 0, len(handles))
		for _, h := range dedupHandles(handles) {
			if _, ok := existing[h]; ok {
				filtered = append(filtered, h)
			} else {
				log.Printf("autouninstall 跳过不存在的 handle=%d (当前映射: %s)", h, formatHandles(drivers))
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("autouninstall 指定 handle 均不存在，当前映射: %s", formatHandles(drivers))
		}

		sort.Slice(filtered, func(i, j int) bool { return filtered[i] > filtered[j] })
		if waitForDeviceRelease(`\\.\OpenSysKit`, 15*time.Second) {
			log.Println("autouninstall 检测到设备引用已释放，开始卸载")
		} else {
			log.Println("autouninstall 警告: 设备可能仍被占用，仍尝试卸载")
		}

		log.Printf("autouninstall 计划卸载 handle: %s", formatHandleList(filtered))
		for _, handle := range filtered {
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
		return fmt.Errorf("autouninstall 卸载后仍存在映射驱动: %s", formatHandles(after))
	}

	log.Println("autouninstall 下发 WinDrive allow-unload")
	if err = loader.AllowUnload(); err != nil {
		return err
	}

	loader.Close()
	log.Println("autouninstall 卸载 DriverLoader 服务")
	if err = driver.UninstallLoaderService(); err != nil {
		return err
	}
	return nil
}

func parseAutoUninstallDelay() time.Duration {
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "--delay=") {
			raw := strings.TrimSpace(strings.TrimPrefix(arg, "--delay="))
			if raw == "" {
				continue
			}
			seconds, err := strconv.Atoi(raw)
			if err != nil || seconds < 0 {
				log.Printf("警告: 非法 autouninstall delay 参数 %q，改用 1 秒", raw)
				return time.Second
			}
			if seconds == 0 {
				return 0
			}
			return time.Duration(seconds) * time.Second
		}
	}
	return time.Second
}

func parseAutoUninstallHandles() ([]uint64, error) {
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "--handles=") {
			raw := strings.TrimSpace(strings.TrimPrefix(arg, "--handles="))
			if raw == "" {
				return nil, nil
			}
			parts := strings.Split(raw, ",")
			handles := make([]uint64, 0, len(parts))
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				h, err := strconv.ParseUint(part, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("解析 autouninstall handle 失败 %q: %w", part, err)
				}
				handles = append(handles, h)
			}
			return handles, nil
		}
	}
	return nil, nil
}
