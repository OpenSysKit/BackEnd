//go:build windows

package main

import (
	"fmt"
)

func runAutoUninstallMode() error {
	return fmt.Errorf("autouninstall 已禁用：请使用显式 uninstall 命令执行完整卸载，避免主进程退出阶段蓝屏")
}
