package service

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

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

// ListDirectoryArgs 文件管理-列目录请求参数
// Path 支持绝对路径，空值时使用系统盘根目录。
type ListDirectoryArgs struct {
	Path string `json:"path"`
}

// FileEntryModel 单个目录项
type FileEntryModel struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// ListDirectoryReply 文件管理-列目录响应
type ListDirectoryReply struct {
	CurrentPath string           `json:"current_path"`
	ParentPath  string           `json:"parent_path"`
	Entries     []FileEntryModel `json:"entries"`
}

// ListDirectory 列出目录内容（目录优先、名称排序）
func (t *ToolkitService) ListDirectory(args *ListDirectoryArgs, reply *ListDirectoryReply) error {
	path := filepath.Clean(args.Path)
	if path == "." || path == "" {
		path = `C:\\`
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("路径解析失败: %w", err)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Errorf("读取目录失败: %w", err)
	}

	reply.CurrentPath = absPath
	reply.ParentPath = filepath.Dir(absPath)
	if reply.ParentPath == absPath {
		reply.ParentPath = ""
	}

	models := make([]FileEntryModel, 0, len(entries))
	for _, entry := range entries {
		fullPath := filepath.Join(absPath, entry.Name())
		model := FileEntryModel{
			Name:  entry.Name(),
			Path:  fullPath,
			IsDir: entry.IsDir(),
		}

		if info, infoErr := entry.Info(); infoErr == nil {
			if !entry.IsDir() {
				model.Size = info.Size()
			}
			model.ModTime = info.ModTime().Format(time.RFC3339)
		}

		models = append(models, model)
	}

	sort.SliceStable(models, func(i, j int) bool {
		if models[i].IsDir != models[j].IsDir {
			return models[i].IsDir
		}
		return models[i].Name < models[j].Name
	})

	reply.Entries = models
	return nil
}

func normalizeKernelPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return p
	}

	if strings.HasPrefix(p, `\\??\\`) || strings.HasPrefix(p, `\\Device\\`) {
		return p
	}

	if len(p) >= 2 && p[1] == ':' {
		return `\\??\\` + p
	}

	return p
}

// DeleteFileKernelArgs 内核删除文件请求参数
type DeleteFileKernelArgs struct {
	Path string `json:"path"`
}

// DeleteFileKernelReply 内核删除文件响应
type DeleteFileKernelReply struct {
	Success bool `json:"success"`
}

// DeleteFileKernel 使用 OpenSysKit 内核 IOCTL 删除文件
func (t *ToolkitService) DeleteFileKernel(args *DeleteFileKernelArgs, reply *DeleteFileKernelReply) error {
	if t.Driver == nil {
		return fmt.Errorf("驱动未加载")
	}

	if args.Path == "" {
		return fmt.Errorf("path 不能为空")
	}

	kernelPath := normalizeKernelPath(args.Path)
	utf16Path, err := syscall.UTF16FromString(kernelPath)
	if err != nil {
		return fmt.Errorf("路径编码失败: %w", err)
	}

	var req driver.FilePathRequest
	if len(utf16Path) > len(req.Path) {
		return fmt.Errorf("路径过长，最大支持 %d UTF-16 字符", len(req.Path)-1)
	}
	copy(req.Path[:], utf16Path)

	inBuf := new(bytes.Buffer)
	if err := binary.Write(inBuf, binary.LittleEndian, req); err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}

	if _, err := t.Driver.IoControl(driver.IOCTL_DELETE_FILE, inBuf.Bytes(), 0); err != nil {
		reply.Success = false
		return fmt.Errorf("内核删除文件失败: %w", err)
	}

	reply.Success = true
	return nil
}

// KillFileLockingProcessesArgs 结束占用文件进程请求参数
type KillFileLockingProcessesArgs struct {
	Path string `json:"path"`
}

// KillResult 单个 PID 的处理结果
type KillResult struct {
	ProcessId uint32 `json:"process_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// KillFileLockingProcessesReply 结束占用文件进程响应
type KillFileLockingProcessesReply struct {
	FoundPids []uint32     `json:"found_pids"`
	Results   []KillResult `json:"results"`
}

// KillFileLockingProcesses 先找占用文件 PID，再通过内核 IOCTL 结束进程
func (t *ToolkitService) KillFileLockingProcesses(args *KillFileLockingProcessesArgs, reply *KillFileLockingProcessesReply) error {
	if t.Driver == nil {
		return fmt.Errorf("驱动未加载")
	}
	if args.Path == "" {
		return fmt.Errorf("path 不能为空")
	}

	pids, err := findLockingProcessIDs(args.Path)
	if err != nil {
		return fmt.Errorf("查询占用进程失败: %w", err)
	}

	reply.FoundPids = pids
	reply.Results = make([]KillResult, 0, len(pids))

	for _, pid := range pids {
		if pid == 0 {
			continue
		}

		req := driver.ProcessRequest{ProcessId: pid}
		inBuf := new(bytes.Buffer)
		if err := binary.Write(inBuf, binary.LittleEndian, req); err != nil {
			reply.Results = append(reply.Results, KillResult{ProcessId: pid, Success: false, Error: err.Error()})
			continue
		}

		if _, err := t.Driver.IoControl(driver.IOCTL_KILL_PROCESS, inBuf.Bytes(), 0); err != nil {
			reply.Results = append(reply.Results, KillResult{ProcessId: pid, Success: false, Error: err.Error()})
			continue
		}

		reply.Results = append(reply.Results, KillResult{ProcessId: pid, Success: true})
	}

	return nil
}

// EnumProcessModulesArgs 进程模块枚举请求参数
type EnumProcessModulesArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// ProcessModuleModel 进程模块信息
type ProcessModuleModel struct {
	ProcessId   uint32 `json:"process_id"`
	ModuleName  string `json:"module_name"`
	BaseAddress uint64 `json:"base_address"`
	Size        uint32 `json:"size"`
	Path        string `json:"path"`
}

// EnumProcessModulesReply 进程模块枚举响应
type EnumProcessModulesReply struct {
	ProcessId uint32               `json:"process_id"`
	Modules   []ProcessModuleModel `json:"modules"`
}

// EnumProcessModules 枚举指定进程加载模块
func (t *ToolkitService) EnumProcessModules(args *EnumProcessModulesArgs, reply *EnumProcessModulesReply) error {
	if args.ProcessId == 0 {
		return fmt.Errorf("process_id must be > 0")
	}

	modules, err := enumProcessModules(args.ProcessId)
	if err != nil {
		return fmt.Errorf("枚举进程模块失败: %w", err)
	}

	reply.ProcessId = args.ProcessId
	reply.Modules = modules
	return nil
}

// EnumNetworkConnectionsArgs 网络连接枚举请求参数
type EnumNetworkConnectionsArgs struct {
	Protocol string `json:"protocol"`
}

// NetworkConnectionModel 网络连接信息
type NetworkConnectionModel struct {
	Protocol    string `json:"protocol"`
	LocalIP     string `json:"local_ip"`
	LocalPort   uint16 `json:"local_port"`
	RemoteIP    string `json:"remote_ip"`
	RemotePort  uint16 `json:"remote_port"`
	State       string `json:"state"`
	ProcessId   uint32 `json:"process_id"`
	ProcessName string `json:"process_name"`
}

// EnumNetworkConnectionsReply 网络连接枚举响应
type EnumNetworkConnectionsReply struct {
	Protocol    string                   `json:"protocol"`
	Connections []NetworkConnectionModel `json:"connections"`
}

// EnumNetworkConnections 枚举 TCP/UDP 到 PID 的关联信息
func (t *ToolkitService) EnumNetworkConnections(args *EnumNetworkConnectionsArgs, reply *EnumNetworkConnectionsReply) error {
	protocol := strings.ToLower(strings.TrimSpace(args.Protocol))
	if protocol == "" {
		protocol = "all"
	}
	if protocol != "all" && protocol != "tcp" && protocol != "udp" {
		return fmt.Errorf("protocol 仅支持 all/tcp/udp")
	}

	connections, err := enumNetworkConnections(protocol)
	if err != nil {
		return fmt.Errorf("枚举网络连接失败: %w", err)
	}

	reply.Protocol = protocol
	reply.Connections = connections
	return nil
}

// HealthCheckArgs 健康检查请求参数
type HealthCheckArgs struct{}

// HealthComponent 健康检查组件结果
type HealthComponent struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// HealthCheckReply 健康检查响应
type HealthCheckReply struct {
	OverallStatus string            `json:"overall_status"`
	GeneratedAt   string            `json:"generated_at"`
	Components    []HealthComponent `json:"components"`
}

// HealthCheck 执行后端链路与能力自检
func (t *ToolkitService) HealthCheck(_ *HealthCheckArgs, reply *HealthCheckReply) error {
	components := make([]HealthComponent, 0, 5)

	components = append(components, HealthComponent{
		Name:    "backend",
		Status:  "ok",
		Message: "rpc service running",
	})

	if t.Driver == nil {
		components = append(components, HealthComponent{
			Name:    "opensyskit_driver",
			Status:  "down",
			Message: "driver not connected",
		})
	} else {
		_, err := t.Driver.IoControl(driver.IOCTL_ENUM_PROCESSES, nil, 8)
		if err != nil {
			components = append(components, HealthComponent{
				Name:    "opensyskit_driver",
				Status:  "degraded",
				Message: err.Error(),
			})
		} else {
			components = append(components, HealthComponent{
				Name:    "opensyskit_driver",
				Status:  "ok",
				Message: "ioctl enum_processes ok",
			})
		}
	}

	if t.WinDriveDriver == nil {
		components = append(components, HealthComponent{
			Name:    "windrive_driver",
			Status:  "degraded",
			Message: "windrive not connected",
		})
	} else {
		components = append(components, HealthComponent{
			Name:    "windrive_driver",
			Status:  "ok",
			Message: "connected",
		})
	}

	if _, err := enumProcessModules(uint32(os.Getpid())); err != nil {
		components = append(components, HealthComponent{
			Name:    "module_enumeration",
			Status:  "degraded",
			Message: err.Error(),
		})
	} else {
		components = append(components, HealthComponent{
			Name:    "module_enumeration",
			Status:  "ok",
			Message: "toolhelp snapshot ok",
		})
	}

	if _, err := enumNetworkConnections("all"); err != nil {
		components = append(components, HealthComponent{
			Name:    "network_enumeration",
			Status:  "degraded",
			Message: err.Error(),
		})
	} else {
		components = append(components, HealthComponent{
			Name:    "network_enumeration",
			Status:  "ok",
			Message: "iphlpapi query ok",
		})
	}

	overall := "ok"
	for _, c := range components {
		if c.Status == "down" {
			overall = "down"
			break
		}
		if c.Status == "degraded" && overall != "down" {
			overall = "degraded"
		}
	}

	reply.OverallStatus = overall
	reply.GeneratedAt = time.Now().Format(time.RFC3339)
	reply.Components = components
	return nil
}
