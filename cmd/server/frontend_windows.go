//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	PROCESS_ALL_ACCESS       = 0x1F0FFF
	SYNCHRONIZE              = 0x00100000
	PROCESS_QUERY_INFORMATION = 0x0400
)

// frontendGuard 负责启动前端、保持 OpenProcess 句柄、监控退出
type frontendGuard struct {
	exePath string
	proc    *os.Process
	handle  windows.Handle
	done    chan struct{}
}

func newFrontendGuard() (*frontendGuard, error) {
	// 前端 exe 与后端 exe 同目录
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取自身路径失败: %w", err)
	}
	dir := filepath.Dir(self)
	exePath := filepath.Join(dir, "OpenSysKit.UI.exe")

	if _, err := os.Stat(exePath); err != nil {
		return nil, fmt.Errorf("前端可执行文件不存在 (%s): %w", exePath, err)
	}

	return &frontendGuard{
		exePath: exePath,
		done:    make(chan struct{}),
	}, nil
}

// Start 启动前端进程，获取内核句柄，开启监控 goroutine
func (g *frontendGuard) Start() error {
	cmd := exec.Command(g.exePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动前端失败: %w", err)
	}
	g.proc = cmd.Process
	log.Printf("[前端守护] 前端已启动，PID = %d", g.proc.Pid)

	// 获取内核 PROCESS 句柄，用于后续 WaitForSingleObject
	handle, err := windows.OpenProcess(
		PROCESS_ALL_ACCESS,
		false,
		uint32(g.proc.Pid),
	)
	if err != nil {
		// 句柄获取失败不致命，但要警告
		log.Printf("[前端守护] 警告: OpenProcess 失败 (PID %d): %v，将使用轮询方式监控", g.proc.Pid, err)
		go g.watchByPoll(cmd)
		return nil
	}
	g.handle = handle
	log.Printf("[前端守护] OpenProcess 句柄已获取: 0x%X", uintptr(unsafe.Pointer(&handle)))

	go g.watchByHandle(cmd)
	return nil
}

// watchByHandle 使用 WaitForSingleObject 等待前端退出（精确、零 CPU 占用）
func (g *frontendGuard) watchByHandle(cmd *exec.Cmd) {
	defer close(g.done)

	event, err := windows.WaitForSingleObject(g.handle, windows.INFINITE)
	_ = windows.CloseHandle(g.handle)

	switch event {
	case windows.WAIT_OBJECT_0:
		var code uint32
		_ = windows.GetExitCodeProcess(g.handle, &code)
		log.Printf("[前端守护] 前端进程已退出 (PID %d, 退出码 %d)，后端即将退出", g.proc.Pid, code)
	default:
		log.Printf("[前端守护] WaitForSingleObject 返回异常值 %d，后端即将退出", event)
	}
	_ = cmd.Wait()
}

// watchByPoll 降级方案：轮询检测进程是否存活
func (g *frontendGuard) watchByPoll(cmd *exec.Cmd) {
	defer close(g.done)
	_ = cmd.Wait()
	log.Printf("[前端守护] 前端进程已退出 (PID %d)，后端即将退出", g.proc.Pid)
}

// Done 返回一个 channel，前端退出时关闭
func (g *frontendGuard) Done() <-chan struct{} {
	return g.done
}

// Kill 强制结束前端（后端主动退出时使用）
func (g *frontendGuard) Kill() {
	if g.proc != nil {
		_ = g.proc.Kill()
	}
	if g.handle != 0 {
		_ = windows.CloseHandle(g.handle)
	}
}

// pidOf 返回前端 PID（供日志使用）
func (g *frontendGuard) pidOf() int {
	if g.proc != nil {
		return g.proc.Pid
	}
	return -1
}

// waitForPipe 等待命名管道就绪（RPC 服务器启动后再拉前端）
func waitForPipe(pipeName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// 用 WaitNamedPipe 检测管道是否可连接
		name, err := windows.UTF16PtrFromString(pipeName)
		if err != nil {
			return err
		}
		err = windows.WaitNamedPipe(name, 200)
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("等待命名管道 %s 超时", pipeName)
}
