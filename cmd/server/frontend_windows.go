//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const processAllAccess = 0x1F0FFF

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var procWaitNamedPipe = kernel32.NewProc("WaitNamedPipeW")

func callWaitNamedPipe(name *uint16, timeoutMs uint32) error {
	r, _, err := procWaitNamedPipe.Call(uintptr(unsafe.Pointer(name)), uintptr(timeoutMs))
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

	if err := verifyFrontendHash(exePath); err != nil {
		return nil, fmt.Errorf("前端完整性校验失败: %w", err)
	}

	return &frontendGuard{exePath: exePath, done: make(chan struct{})}, nil
}

// verifyFrontendHash 校验前端 EXE 的 SHA256 是否与编译时注入的值一致。
// 开关逻辑：
//   - frontendSHA256 为空（本地 dev 构建）=> 跳过
//   - 环境变量 OPENSYSKIT_SKIP_HASH_CHECK=1 => 跳过（本地测试用）
func verifyFrontendHash(exePath string) error {
	if frontendSHA256 == "" {
		log.Println("[integrity] 前端 hash 未注入（dev 构建），跳过校验")
		return nil
	}

	skipEnv := strings.TrimSpace(os.Getenv("OPENSYSKIT_SKIP_HASH_CHECK"))
	if skipEnv == "1" || strings.EqualFold(skipEnv, "true") {
		log.Println("[integrity] OPENSYSKIT_SKIP_HASH_CHECK=1，跳过前端 hash 校验")
		return nil
	}

	f, err := os.Open(exePath)
	if err != nil {
		return fmt.Errorf("打开前端文件失败: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("计算 SHA256 失败: %w", err)
	}

	actual := strings.ToLower(hex.EncodeToString(h.Sum(nil)))
	expected := strings.ToLower(strings.TrimSpace(frontendSHA256))

	if actual != expected {
		return fmt.Errorf("前端 SHA256 不匹配\n  期望: %s\n  实际: %s\n  文件可能被篡改", expected, actual)
	}

	log.Printf("[integrity] 前端 hash 校验通过: %s", actual)
	return nil
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

	handle, err := windows.OpenProcess(processAllAccess, false, uint32(g.proc.Pid))
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
