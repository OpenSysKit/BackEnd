//go:build windows

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	procGetConsoleWnd  = kernel32.NewProc("GetConsoleWindow")
	procShowWindow     = user32.NewProc("ShowWindow")
)

func hideConsoleWindow() {
	hwnd, _, _ := procGetConsoleWnd.Call()
	if hwnd != 0 {
		procShowWindow.Call(hwnd, 0) // SW_HIDE
	}
}

func setupLogFile() (*os.File, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(filepath.Dir(self), "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	logName := fmt.Sprintf("opensyskit-%s.log", time.Now().Format("2006-01-02"))
	logPath := filepath.Join(logDir, logName)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}

	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return f, nil
}
