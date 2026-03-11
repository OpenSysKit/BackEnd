//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/OpenSysKit/backend/internal/driver"
)

// selfProtect 通过 WinDrive 的 ObRegisterCallbacks 保护指定 PID，
// 阻止其他进程对其执行 WriteProcessMemory / TerminateProcess / DLL 注入等操作。
type selfProtect struct {
	dev  driver.Device
	pids []uint32
}

func newSelfProtect(dev driver.Device) *selfProtect {
	return &selfProtect{dev: dev}
}

func (sp *selfProtect) applyHighPolicy() error {
	req := driver.ProtectPolicyRequest{
		Version:        1,
		DenyAccessMask: 0x00000A6B, // TERMINATE | CREATE_THREAD | VM_OPERATION | VM_WRITE | DUP_HANDLE | SET_INFORMATION | SUSPEND_RESUME
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, req); err != nil {
		return fmt.Errorf("构造策略请求失败: %w", err)
	}
	if _, err := sp.dev.IoControl(driver.IOCTL_WINDRIVE_SET_PROTECT_POLICY, buf.Bytes(), 0); err != nil {
		return fmt.Errorf("设置高保护策略失败: %w", err)
	}
	log.Println("[自保护] 已设置 high 级别保护策略 (deny=0x00000A6B)")
	return nil
}

func (sp *selfProtect) protect(pid uint32) error {
	req := driver.ProcessRequest{ProcessId: pid}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, req); err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	if _, err := sp.dev.IoControl(driver.IOCTL_WINDRIVE_PROTECT_PROCESS, buf.Bytes(), 0); err != nil {
		return fmt.Errorf("保护进程(pid=%d)失败: %w", pid, err)
	}
	sp.pids = append(sp.pids, pid)
	log.Printf("[自保护] 已保护 PID %d", pid)
	return nil
}

func (sp *selfProtect) cleanup() {
	for _, pid := range sp.pids {
		req := driver.ProcessRequest{ProcessId: pid}
		buf := new(bytes.Buffer)
		_ = binary.Write(buf, binary.LittleEndian, req)
		if _, err := sp.dev.IoControl(driver.IOCTL_WINDRIVE_UNPROTECT_PROCESS, buf.Bytes(), 0); err != nil {
			log.Printf("[自保护] 取消保护 PID %d 失败: %v", pid, err)
		} else {
			log.Printf("[自保护] 已取消保护 PID %d", pid)
		}
	}
	sp.pids = nil
}
