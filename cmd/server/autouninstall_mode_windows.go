//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func runAutoUninstallMode() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前可执行文件路径失败: %w", err)
	}

	logPath := filepath.Join(filepath.Dir(exePath), "auto-uninstall.log")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开自动卸载日志失败: %w", err)
	}
	defer logFile.Close()

	writeLog := func(format string, args ...any) {
		_, _ = fmt.Fprintf(logFile, "[%s] %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	}

	delaySec := 5
	handlesArg := ""
	writeLog("autouninstall invoked args=%v", os.Args)

	for _, arg := range os.Args[2:] {
		arg = strings.TrimSpace(arg)
		switch {
		case strings.HasPrefix(arg, "--delay="):
			v := strings.TrimSpace(strings.TrimPrefix(arg, "--delay="))
			n, parseErr := strconv.Atoi(v)
			if parseErr != nil || n <= 0 {
				writeLog("invalid delay arg: %s", arg)
				return fmt.Errorf("无效 delay 参数: %s", arg)
			}
			delaySec = n
		case strings.HasPrefix(arg, "--handles="):
			handlesArg = strings.TrimSpace(strings.TrimPrefix(arg, "--handles="))
		}
	}

	if handlesArg == "" {
		writeLog("missing --handles argument")
		return fmt.Errorf("缺少 --handles 参数")
	}

	writeLog("autouninstall start handles=%s delay=%ds", handlesArg, delaySec)
	time.Sleep(time.Duration(delaySec) * time.Second)

	cmd := exec.Command(exePath, "uninstall", "--handles="+handlesArg)
	cmd.Dir = filepath.Dir(exePath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Run(); err != nil {
		writeLog("autouninstall failed: %v", err)
		return fmt.Errorf("执行卸载命令失败: %w", err)
	}

	writeLog("autouninstall done")
	return nil
}
