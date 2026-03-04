//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func scheduleSelfUninstall(delay time.Duration, handles []uint64) error {
	if len(handles) == 0 {
		return fmt.Errorf("未提供可卸载的 handle")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前可执行文件路径失败: %w", err)
	}

	seconds := int(delay / time.Second)
	if seconds < 1 {
		seconds = 1
	}

	handleArgs := make([]string, 0, len(handles))
	for _, h := range handles {
		handleArgs = append(handleArgs, strconv.FormatUint(h, 10))
	}

	cmd := exec.Command(
		exePath,
		"autouninstall",
		fmt.Sprintf("--delay=%d", seconds),
		fmt.Sprintf("--handles=%s", strings.Join(handleArgs, ",")),
	)
	cmd.Dir = filepath.Dir(exePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("启动卸载子进程失败: %w", err)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}

	return nil
}
