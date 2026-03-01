package service

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"syscall"

	"github.com/OpenSysKit/backend/internal/driver"
)

// ToolkitService 暴露给前端的 JSON-RPC 服务。
// 所有方法通过 Driver 接口与内核驱动通信。
type ToolkitService struct {
	Driver         driver.Device
	WinDriveDriver driver.Device
}

// PingArgs 连通性测试请求参数。
type PingArgs struct{}

// PingReply 连通性测试响应。
type PingReply struct {
	Status string `json:"status"`
}

// Ping 连通性测试，前端可用于检测后端服务是否存活。
func (t *ToolkitService) Ping(_ *PingArgs, reply *PingReply) error {
	reply.Status = "ok"
	return nil
}

// ProcessInfoModel 返回给前端的进程信息
type ProcessInfoModel struct {
	ProcessId       uint32 `json:"process_id"`
	ParentProcessId uint32 `json:"parent_process_id"`
	ThreadCount     uint32 `json:"thread_count"`
	WorkingSetSize  uint64 `json:"working_set_size"`
	ImageName       string `json:"image_name"`
}

// EnumProcessesArgs 枚举进程请求参数
type EnumProcessesArgs struct{}

// EnumProcessesReply 枚举进程响应
type EnumProcessesReply struct {
	Processes []ProcessInfoModel `json:"processes"`
}

// EnumProcesses 枚举系统进程
func (t *ToolkitService) EnumProcesses(_ *EnumProcessesArgs, reply *EnumProcessesReply) error {
	if t.Driver == nil {
		return fmt.Errorf("驱动未加载")
	}

	// 首先尝试使用合理的初始大小，如果不够，驱动会返回所需的总大小
	initialSize := uint32(1024 * 1024) // 1MB should be enough for most cases
	outBuf, err := t.Driver.IoControl(driver.IOCTL_ENUM_PROCESSES, nil, initialSize)
	if err != nil {
		return fmt.Errorf("枚举进程失败: %w", err)
	}

	if len(outBuf) < 8 {
		return fmt.Errorf("返回数据过小")
	}

	header := driver.ProcessListHeader{}
	err = binary.Read(bytes.NewReader(outBuf[:8]), binary.LittleEndian, &header)
	if err != nil {
		return fmt.Errorf("解析头部失败: %w", err)
	}

	// 解析进程信息
	offset := uint32(8)
	reply.Processes = make([]ProcessInfoModel, 0, header.Count)
	for i := uint32(0); i < header.Count; i++ {
		if offset+uint32(binary.Size(driver.ProcessInfo{})) > uint32(len(outBuf)) {
			break
		}

		info := driver.ProcessInfo{}
		err = binary.Read(bytes.NewReader(outBuf[offset:offset+uint32(binary.Size(info))]), binary.LittleEndian, &info)
		if err != nil {
			break
		}

		reply.Processes = append(reply.Processes, ProcessInfoModel{
			ProcessId:       info.ProcessId,
			ParentProcessId: info.ParentProcessId,
			ThreadCount:     info.ThreadCount,
			WorkingSetSize:  info.WorkingSetSize,
			ImageName:       syscall.UTF16ToString(info.ImageName[:]),
		})

		offset += uint32(binary.Size(info))
	}

	return nil
}

// KillProcessArgs 结束进程请求参数
type KillProcessArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// KillProcessReply 结束进程响应
type KillProcessReply struct {
	Success bool `json:"success"`
}

// KillProcess 结束指定进程
func (t *ToolkitService) KillProcess(args *KillProcessArgs, reply *KillProcessReply) error {
	if t.Driver == nil {
		return fmt.Errorf("驱动未加载")
	}

	req := driver.ProcessRequest{ProcessId: args.ProcessId}
	inBuf := new(bytes.Buffer)
	err := binary.Write(inBuf, binary.LittleEndian, req)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}

	_, err = t.Driver.IoControl(driver.IOCTL_KILL_PROCESS, inBuf.Bytes(), 0)
	if err != nil {
		reply.Success = false
		return fmt.Errorf("结束进程失败: %w", err)
	}

	reply.Success = true
	return nil
}

// ProtectProcessArgs 保护进程请求参数
type ProtectProcessArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// ProtectProcessReply 保护进程响应
type ProtectProcessReply struct {
	Success bool `json:"success"`
}

// ProtectProcess 保护指定进程
func (t *ToolkitService) ProtectProcess(args *ProtectProcessArgs, reply *ProtectProcessReply) error {
	if t.WinDriveDriver == nil {
		return fmt.Errorf("WinDrive 未加载")
	}

	req := driver.ProcessRequest{ProcessId: args.ProcessId}
	inBuf := new(bytes.Buffer)
	err := binary.Write(inBuf, binary.LittleEndian, req)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}

	_, err = t.WinDriveDriver.IoControl(driver.IOCTL_WINDRIVE_PROTECT_PROCESS, inBuf.Bytes(), 0)
	if err != nil {
		reply.Success = false
		return fmt.Errorf("保护进程失败: %w", err)
	}

	reply.Success = true
	return nil
}

// UnprotectProcessArgs 取消保护进程请求参数
type UnprotectProcessArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// UnprotectProcessReply 取消保护进程响应
type UnprotectProcessReply struct {
	Success bool `json:"success"`
}

// UnprotectProcess 取消保护指定进程
func (t *ToolkitService) UnprotectProcess(args *UnprotectProcessArgs, reply *UnprotectProcessReply) error {
	if t.WinDriveDriver == nil {
		return fmt.Errorf("WinDrive 未加载")
	}

	req := driver.ProcessRequest{ProcessId: args.ProcessId}
	inBuf := new(bytes.Buffer)
	err := binary.Write(inBuf, binary.LittleEndian, req)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}

	_, err = t.WinDriveDriver.IoControl(driver.IOCTL_WINDRIVE_UNPROTECT_PROCESS, inBuf.Bytes(), 0)
	if err != nil {
		reply.Success = false
		return fmt.Errorf("取消保护进程失败: %w", err)
	}

	reply.Success = true
	return nil
}

// SetProtectPolicyArgs 设置 WinDrive 保护策略请求参数
type SetProtectPolicyArgs struct {
	Version        uint32 `json:"version"`
	DenyAccessMask uint32 `json:"deny_access_mask"`
}

// SetProtectPolicyReply 设置 WinDrive 保护策略响应
type SetProtectPolicyReply struct {
	Success bool `json:"success"`
}

// SetProtectPolicy 下发 WinDrive 保护策略
func (t *ToolkitService) SetProtectPolicy(args *SetProtectPolicyArgs, reply *SetProtectPolicyReply) error {
	if t.WinDriveDriver == nil {
		return fmt.Errorf("WinDrive 未加载")
	}

	req := driver.ProtectPolicyRequest{
		Version:        args.Version,
		DenyAccessMask: args.DenyAccessMask,
	}
	inBuf := new(bytes.Buffer)
	err := binary.Write(inBuf, binary.LittleEndian, req)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}

	_, err = t.WinDriveDriver.IoControl(driver.IOCTL_WINDRIVE_SET_PROTECT_POLICY, inBuf.Bytes(), 0)
	if err != nil {
		reply.Success = false
		return fmt.Errorf("设置保护策略失败: %w", err)
	}

	reply.Success = true
	return nil
}
