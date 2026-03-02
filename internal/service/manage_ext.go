package service

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/OpenSysKit/backend/internal/driver"
)

// ProcessTreeArgs 进程树请求参数
type ProcessTreeArgs struct{}

// ProcessTreeNode 进程树节点
type ProcessTreeNode struct {
	ProcessId       uint32            `json:"process_id"`
	ParentProcessId uint32            `json:"parent_process_id"`
	ImageName       string            `json:"image_name"`
	ThreadCount     uint32            `json:"thread_count"`
	WorkingSetSize  uint64            `json:"working_set_size"`
	Children        []ProcessTreeNode `json:"children"`
}

// ProcessTreeReply 进程树响应
type ProcessTreeReply struct {
	Total int               `json:"total"`
	Roots []ProcessTreeNode `json:"roots"`
}

// GetProcessTree 返回完整进程树（按 PID 升序）
func (t *ToolkitService) GetProcessTree(_ *ProcessTreeArgs, reply *ProcessTreeReply) error {
	processes, err := t.getProcessList()
	if err != nil {
		return err
	}

	roots := buildProcessTree(processes)
	reply.Total = len(processes)
	reply.Roots = roots
	return nil
}

// KillProcessTreeArgs 结束进程子树请求参数
type KillProcessTreeArgs struct {
	ProcessId    uint32 `json:"process_id"`
	IncludeRoot  bool   `json:"include_root"`
	LeavesFirst  bool   `json:"leaves_first"`
	StrictErrors bool   `json:"strict_errors"`
}

// KillProcessTreeReply 结束进程子树响应
type KillProcessTreeReply struct {
	TargetProcessId uint32       `json:"target_process_id"`
	OrderedPids     []uint32     `json:"ordered_pids"`
	Results         []KillResult `json:"results"`
}

// KillProcessTree 按子树顺序结束进程（默认叶子优先）。
func (t *ToolkitService) KillProcessTree(args *KillProcessTreeArgs, reply *KillProcessTreeReply) error {
	if t.Driver == nil {
		return fmt.Errorf("驱动未加载")
	}
	if args.ProcessId == 0 {
		return fmt.Errorf("process_id must be > 0")
	}

	processes, err := t.getProcessList()
	if err != nil {
		return err
	}

	order := collectSubtreeOrder(processes, args.ProcessId, args.IncludeRoot, args.LeavesFirst)
	reply.TargetProcessId = args.ProcessId
	reply.OrderedPids = order
	reply.Results = make([]KillResult, 0, len(order))

	for _, pid := range order {
		req := driver.ProcessRequest{ProcessId: pid}
		inBuf := new(bytes.Buffer)
		if err = binary.Write(inBuf, binary.LittleEndian, req); err != nil {
			kr := KillResult{ProcessId: pid, Success: false, Error: err.Error()}
			reply.Results = append(reply.Results, kr)
			if args.StrictErrors {
				return fmt.Errorf("构造请求失败(pid=%d): %w", pid, err)
			}
			continue
		}

		if _, err = t.Driver.IoControl(driver.IOCTL_KILL_PROCESS, inBuf.Bytes(), 0); err != nil {
			kr := KillResult{ProcessId: pid, Success: false, Error: err.Error()}
			reply.Results = append(reply.Results, kr)
			if args.StrictErrors {
				return fmt.Errorf("结束子树进程失败(pid=%d): %w", pid, err)
			}
			continue
		}

		reply.Results = append(reply.Results, KillResult{ProcessId: pid, Success: true})
	}

	return nil
}

// EnumThreadsArgs 枚举线程请求参数
type EnumThreadsArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// ThreadInfoModel 线程信息
type ThreadInfoModel struct {
	ThreadId      uint32 `json:"thread_id"`
	OwnerProcess  uint32 `json:"owner_process_id"`
	BasePriority  int32  `json:"base_priority"`
	DeltaPriority int32  `json:"delta_priority"`
}

// EnumThreadsReply 枚举线程响应
type EnumThreadsReply struct {
	ProcessId uint32            `json:"process_id"`
	Threads   []ThreadInfoModel `json:"threads"`
}

// EnumThreads 枚举指定 PID 的线程
func (t *ToolkitService) EnumThreads(args *EnumThreadsArgs, reply *EnumThreadsReply) error {
	if args.ProcessId == 0 {
		return fmt.Errorf("process_id must be > 0")
	}
	threads, err := enumThreadsByProcess(args.ProcessId)
	if err != nil {
		return fmt.Errorf("枚举线程失败: %w", err)
	}
	reply.ProcessId = args.ProcessId
	reply.Threads = threads
	return nil
}

// ThreadActionArgs 线程动作请求参数
type ThreadActionArgs struct {
	ThreadId uint32 `json:"thread_id"`
}

// ThreadActionReply 线程动作响应
type ThreadActionReply struct {
	Success      bool  `json:"success"`
	SuspendCount int32 `json:"suspend_count"`
}

// SuspendThread 挂起线程
func (t *ToolkitService) SuspendThread(args *ThreadActionArgs, reply *ThreadActionReply) error {
	if args.ThreadId == 0 {
		return fmt.Errorf("thread_id must be > 0")
	}
	count, err := suspendThread(args.ThreadId)
	if err != nil {
		reply.Success = false
		return fmt.Errorf("挂起线程失败: %w", err)
	}
	reply.Success = true
	reply.SuspendCount = count
	return nil
}

// ResumeThread 恢复线程
func (t *ToolkitService) ResumeThread(args *ThreadActionArgs, reply *ThreadActionReply) error {
	if args.ThreadId == 0 {
		return fmt.Errorf("thread_id must be > 0")
	}
	count, err := resumeThread(args.ThreadId)
	if err != nil {
		reply.Success = false
		return fmt.Errorf("恢复线程失败: %w", err)
	}
	reply.Success = true
	reply.SuspendCount = count
	return nil
}

// ListServicesArgs 服务枚举请求参数
type ListServicesArgs struct {
	NameLike string `json:"name_like"`
}

// ServiceInfoModel 服务信息
type ServiceInfoModel struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	State       string `json:"state"`
	StartType   string `json:"start_type"`
}

// ListServicesReply 服务枚举响应
type ListServicesReply struct {
	Services []ServiceInfoModel `json:"services"`
}

// ListServices 枚举服务并返回状态/启动类型
func (t *ToolkitService) ListServices(args *ListServicesArgs, reply *ListServicesReply) error {
	services, err := listWindowsServices(strings.TrimSpace(args.NameLike))
	if err != nil {
		return fmt.Errorf("枚举服务失败: %w", err)
	}
	reply.Services = services
	return nil
}

// ServiceActionArgs 服务动作请求参数
type ServiceActionArgs struct {
	Name string `json:"name"`
}

// ServiceActionReply 服务动作响应
type ServiceActionReply struct {
	Success bool `json:"success"`
}

// StartService 启动服务
func (t *ToolkitService) StartService(args *ServiceActionArgs, reply *ServiceActionReply) error {
	if strings.TrimSpace(args.Name) == "" {
		return fmt.Errorf("name 不能为空")
	}
	if err := startWindowsService(args.Name); err != nil {
		reply.Success = false
		return fmt.Errorf("启动服务失败: %w", err)
	}
	reply.Success = true
	return nil
}

// StopService 停止服务
func (t *ToolkitService) StopService(args *ServiceActionArgs, reply *ServiceActionReply) error {
	if strings.TrimSpace(args.Name) == "" {
		return fmt.Errorf("name 不能为空")
	}
	if err := stopWindowsService(args.Name); err != nil {
		reply.Success = false
		return fmt.Errorf("停止服务失败: %w", err)
	}
	reply.Success = true
	return nil
}

// SetServiceStartTypeArgs 设置服务启动类型请求参数
type SetServiceStartTypeArgs struct {
	Name      string `json:"name"`
	StartType string `json:"start_type"`
}

// SetServiceStartTypeReply 设置服务启动类型响应
type SetServiceStartTypeReply struct {
	Success bool `json:"success"`
}

// SetServiceStartType 修改服务启动类型（auto/manual/disabled）
func (t *ToolkitService) SetServiceStartType(args *SetServiceStartTypeArgs, reply *SetServiceStartTypeReply) error {
	if strings.TrimSpace(args.Name) == "" {
		return fmt.Errorf("name 不能为空")
	}
	if err := setWindowsServiceStartType(args.Name, args.StartType); err != nil {
		reply.Success = false
		return fmt.Errorf("修改服务启动类型失败: %w", err)
	}
	reply.Success = true
	return nil
}

// ApplyProtectTemplateArgs 策略模板请求参数
type ApplyProtectTemplateArgs struct {
	Template string `json:"template"`
}

// ApplyProtectTemplateReply 策略模板响应
type ApplyProtectTemplateReply struct {
	Success        bool   `json:"success"`
	Template       string `json:"template"`
	Version        uint32 `json:"version"`
	DenyAccessMask uint32 `json:"deny_access_mask"`
}

// ApplyProtectTemplate 按模板下发 WinDrive 进程保护策略
func (t *ToolkitService) ApplyProtectTemplate(args *ApplyProtectTemplateArgs, reply *ApplyProtectTemplateReply) error {
	if t.WinDriveDriver == nil {
		return fmt.Errorf("WinDrive 未加载")
	}

	template := strings.ToLower(strings.TrimSpace(args.Template))
	if template == "" {
		template = "medium"
	}

	var mask uint32
	switch template {
	case "low":
		mask = 0x00000001 // PROCESS_TERMINATE
	case "medium":
		mask = 0x00000801 // TERMINATE + SUSPEND_RESUME
	case "high":
		mask = 0x00000A21 // TERMINATE + VM_WRITE + SET_INFORMATION + SUSPEND_RESUME
	default:
		return fmt.Errorf("template 仅支持 low/medium/high")
	}

	req := driver.ProtectPolicyRequest{
		Version:        1,
		DenyAccessMask: mask,
	}
	inBuf := new(bytes.Buffer)
	if err := binary.Write(inBuf, binary.LittleEndian, req); err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}

	if _, err := t.WinDriveDriver.IoControl(driver.IOCTL_WINDRIVE_SET_PROTECT_POLICY, inBuf.Bytes(), 0); err != nil {
		reply.Success = false
		return fmt.Errorf("设置保护策略失败: %w", err)
	}

	reply.Success = true
	reply.Template = template
	reply.Version = 1
	reply.DenyAccessMask = mask
	return nil
}

func (t *ToolkitService) getProcessList() ([]ProcessInfoModel, error) {
	if t.Driver == nil {
		return nil, fmt.Errorf("驱动未加载")
	}
	tmp := &EnumProcessesReply{}
	if err := t.EnumProcesses(&EnumProcessesArgs{}, tmp); err != nil {
		return nil, err
	}
	return tmp.Processes, nil
}

func buildProcessTree(processes []ProcessInfoModel) []ProcessTreeNode {
	byPID := make(map[uint32]ProcessInfoModel, len(processes))
	children := make(map[uint32][]ProcessInfoModel, len(processes))
	for _, p := range processes {
		byPID[p.ProcessId] = p
		children[p.ParentProcessId] = append(children[p.ParentProcessId], p)
	}
	for k := range children {
		sort.SliceStable(children[k], func(i, j int) bool {
			return children[k][i].ProcessId < children[k][j].ProcessId
		})
	}

	var roots []ProcessInfoModel
	for _, p := range processes {
		_, parentExists := byPID[p.ParentProcessId]
		if p.ParentProcessId == 0 || !parentExists || p.ParentProcessId == p.ProcessId {
			roots = append(roots, p)
		}
	}
	sort.SliceStable(roots, func(i, j int) bool { return roots[i].ProcessId < roots[j].ProcessId })

	var buildNode func(p ProcessInfoModel, depth int) ProcessTreeNode
	buildNode = func(p ProcessInfoModel, depth int) ProcessTreeNode {
		node := ProcessTreeNode{
			ProcessId:       p.ProcessId,
			ParentProcessId: p.ParentProcessId,
			ImageName:       p.ImageName,
			ThreadCount:     p.ThreadCount,
			WorkingSetSize:  p.WorkingSetSize,
		}
		if depth >= 64 {
			return node
		}
		for _, child := range children[p.ProcessId] {
			if child.ProcessId == p.ProcessId {
				continue
			}
			node.Children = append(node.Children, buildNode(child, depth+1))
		}
		return node
	}

	out := make([]ProcessTreeNode, 0, len(roots))
	for _, p := range roots {
		out = append(out, buildNode(p, 0))
	}
	return out
}

func collectSubtreeOrder(processes []ProcessInfoModel, pid uint32, includeRoot bool, leavesFirst bool) []uint32 {
	children := make(map[uint32][]uint32, len(processes))
	for _, p := range processes {
		children[p.ParentProcessId] = append(children[p.ParentProcessId], p.ProcessId)
	}
	for k := range children {
		sort.SliceStable(children[k], func(i, j int) bool { return children[k][i] < children[k][j] })
	}

	type item struct {
		pid   uint32
		depth int
	}
	queue := []item{{pid: pid, depth: 0}}
	collected := make([]item, 0, 64)
	seen := make(map[uint32]struct{}, 64)

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, ok := seen[cur.pid]; ok {
			continue
		}
		seen[cur.pid] = struct{}{}
		collected = append(collected, cur)
		for _, ch := range children[cur.pid] {
			if ch == cur.pid {
				continue
			}
			queue = append(queue, item{pid: ch, depth: cur.depth + 1})
		}
	}

	filtered := make([]item, 0, len(collected))
	for _, it := range collected {
		if !includeRoot && it.pid == pid {
			continue
		}
		filtered = append(filtered, it)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if leavesFirst {
			if filtered[i].depth != filtered[j].depth {
				return filtered[i].depth > filtered[j].depth
			}
		} else {
			if filtered[i].depth != filtered[j].depth {
				return filtered[i].depth < filtered[j].depth
			}
		}
		return filtered[i].pid < filtered[j].pid
	})

	out := make([]uint32, 0, len(filtered))
	for _, it := range filtered {
		out = append(out, it.pid)
	}
	return out
}
