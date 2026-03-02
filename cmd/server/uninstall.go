package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/OpenSysKit/backend/internal/driver"
)

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
	switch len(drivers) {
	case 0:
		log.Println("[uninstall] 未检测到映射驱动，跳过 OpenSysKit 卸载")
	case 1:
		handle := drivers[0].Handle
		log.Printf("[uninstall] 检测到单一映射驱动，按 OpenSysKit 处理，卸载 handle=%d", handle)
		if err = loader.UnloadMappedDriver(handle); err != nil {
			return err
		}
	default:
		return fmt.Errorf("检测到多个映射驱动，拒绝自动卸载 WinDrive，请先手动处理: %s", formatHandles(drivers))
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
