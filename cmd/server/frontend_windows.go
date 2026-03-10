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

// WaitNamedPipe 不在 golang.org/x/sys/windows 的这个版本里，手动声明
var waitNamedPipe = windows.NewProc("WaitNamedPipeW")

func callWaitNamedPipe(name *uint16, timeoutMs uint32) error {
	r, _, err := waitNamedPipe.Call(uintptr(unsafe.Pointer(name)), uintptr(timeoutMs))
	if r == 0 {
		return err
	}
	return nil
}

// frontendGuard 负责启动前端、保持 OpenProcess 句柄、监控退出
type frontendGuard struct {
	exePath string
	proc    *os.Process
	handle  windows.Handle
	done    chan struct{}
}

func newFrontendGuard() (*frontendGuard, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取自身路径失败: %w", err)
	}
	exePath := filepath.Join(filepath.Dir(self), "OpenSysKit.UI.exe")
	if _, err := os.Stat(exePath); err != nil {
		return nil, fmt.Errorf("前端可执行文件不存在 (%s): %w", exePath, err)
	}
	return &frontendGuard{exePath: exePath, done: make(chan struct{})}, nil
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

	handle, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(g.proc.Pid))
	if err != nil {
		log.Printf("[前端守护] 警告: OpenProcess 失败 (PID %d): %v，降级为轮询监控", g.proc.Pid, err)
		go g.watchByPoll(cmd)
		return nil
	}
	g.handle = handle
	log.Printf("[前端守护] OpenProcess 句柄已获取: 0x%X", uintptr(handle))
	go g.watchByHandle(cmd)
	return nil
}

// watchByHandle 使用 WaitForSingleObject 等待前端退出
func (g *frontendGuard) watchByHandle(cmd *exec.Cmd) {
	defer close(g.done)

	// WaitForSingleObject 在此版本返回 (uint32, error)，忽略 error
	windows.WaitForSingleObject(g.handle, windows.INFINITE) //nolint

	var code uint32
	_ = windows.GetExitCodeProcess(g.handle, &code)
	_ = windows.CloseHandle(g.handle)
	g.handle = 0

	log.Printf("[前端守护] 前端进程已退出 (PID %d, 退出码 %d)，后端即将退出", g.proc.Pid, code)
	_ = cmd.Wait()
}

// watchByPoll 降级方案：等待 cmd.Wait() 返回
func (g *frontendGuard) watchByPoll(cmd *exec.Cmd) {
	defer close(g.done)
	_ = cmd.Wait()
	log.Printf("[前端守护] 前端进程已退出 (PID %d)，后端即将退出", g.proc.Pid)
}

func (g *frontendGuard) Done() <-chan struct{} { return g.done }

func (g *frontendGuard) Kill() {
	if g.proc != nil {
		_ = g.proc.Kill()
	}
	if g.handle != 0 {
		_ = windows.CloseHandle(g.handle)
		g.handle = 0
	}
}

func (g *frontendGuard) pidOf() int {
	if g.proc != nil {
		return g.proc.Pid
	}
	return -1
}

// waitForPipe 轮询等待命名管道就绪
func waitForPipe(pipeName string, timeout time.Duration) error {
	name, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := callWaitNamedPipe(name, 200); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("等待命名管道 %s 超时", pipeName)
}
