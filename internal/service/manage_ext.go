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
		err := fmt.Errorf("驱动未加载")
		auditWrite("kill_process_tree", map[string]any{"process_id": args.ProcessId}, err)
		return err
	}
	if args.ProcessId == 0 {
		err := fmt.Errorf("process_id must be > 0")
		auditWrite("kill_process_tree", map[string]any{"process_id": args.ProcessId}, err)
		return err
	}

	processes, err := t.getProcessList()
	if err != nil {
		auditWrite("kill_process_tree", map[string]any{"process_id": args.ProcessId}, err)
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
				retErr := fmt.Errorf("构造请求失败(pid=%d): %w", pid, err)
				auditWrite("kill_process_tree", map[string]any{"process_id": args.ProcessId, "failed_pid": pid}, retErr)
				return retErr
			}
			continue
		}

		if _, err = t.Driver.IoControl(driver.IOCTL_KILL_PROCESS, inBuf.Bytes(), 0); err != nil {
			kr := KillResult{ProcessId: pid, Success: false, Error: err.Error()}
			reply.Results = append(reply.Results, kr)
			if args.StrictErrors {
				retErr := fmt.Errorf("结束子树进程失败(pid=%d): %w", pid, err)
				auditWrite("kill_process_tree", map[string]any{"process_id": args.ProcessId, "failed_pid": pid}, retErr)
				return retErr
			}
			continue
		}

		reply.Results = append(reply.Results, KillResult{ProcessId: pid, Success: true})
	}

	auditWrite("kill_process_tree", map[string]any{
		"process_id":   args.ProcessId,
		"killed_count": len(reply.Results),
	}, nil)
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

// EnumHandlesArgs 句柄统计请求参数
type EnumHandlesArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// HandleTypeStat 句柄类型统计项
type HandleTypeStat struct {
	TypeIndex uint16 `json:"type_index"`
	TypeName  string `json:"type_name"`
	Count     uint32 `json:"count"`
}

// EnumHandlesReply 句柄统计响应
type EnumHandlesReply struct {
	ProcessId    uint32           `json:"process_id"`
	TotalHandles uint32           `json:"total_handles"`
	Types        []HandleTypeStat `json:"types"`
}

// EnumHandles 按 PID 枚举句柄数量与类型分布
func (t *ToolkitService) EnumHandles(args *EnumHandlesArgs, reply *EnumHandlesReply) error {
	if args.ProcessId == 0 {
		return fmt.Errorf("process_id must be > 0")
	}
	total, stats, err := enumHandleStatsByPID(args.ProcessId)
	if err != nil {
		return fmt.Errorf("枚举句柄失败: %w", err)
	}
	reply.ProcessId = args.ProcessId
	reply.TotalHandles = total
	reply.Types = stats
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
		err := fmt.Errorf("thread_id must be > 0")
		auditWrite("suspend_thread", map[string]any{"thread_id": args.ThreadId}, err)
		return err
	}
	count, err := suspendThread(args.ThreadId)
	if err != nil {
		reply.Success = false
		retErr := fmt.Errorf("挂起线程失败: %w", err)
		auditWrite("suspend_thread", map[string]any{"thread_id": args.ThreadId}, retErr)
		return retErr
	}
	reply.Success = true
	reply.SuspendCount = count
	auditWrite("suspend_thread", map[string]any{"thread_id": args.ThreadId, "suspend_count": count}, nil)
	return nil
}

// ResumeThread 恢复线程
func (t *ToolkitService) ResumeThread(args *ThreadActionArgs, reply *ThreadActionReply) error {
	if args.ThreadId == 0 {
		err := fmt.Errorf("thread_id must be > 0")
		auditWrite("resume_thread", map[string]any{"thread_id": args.ThreadId}, err)
		return err
	}
	count, err := resumeThread(args.ThreadId)
	if err != nil {
		reply.Success = false
		retErr := fmt.Errorf("恢复线程失败: %w", err)
		auditWrite("resume_thread", map[string]any{"thread_id": args.ThreadId}, retErr)
		return retErr
	}
	reply.Success = true
	reply.SuspendCount = count
	auditWrite("resume_thread", map[string]any{"thread_id": args.ThreadId, "suspend_count": count}, nil)
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

// ListStartupEntriesArgs 自启动项枚举请求参数
type ListStartupEntriesArgs struct {
	Category string `json:"category"`  // all/services/tasks
	NameLike string `json:"name_like"` // 可选过滤
}

// StartupEntryModel 自启动项模型
type StartupEntryModel struct {
	Source      string `json:"source"` // service/task
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	State       string `json:"state,omitempty"`
	RunAs       string `json:"run_as,omitempty"`
	Command     string `json:"command,omitempty"`
	Trigger     string `json:"trigger,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

// ListStartupEntriesReply 自启动项枚举响应
type ListStartupEntriesReply struct {
	Category string              `json:"category"`
	Entries  []StartupEntryModel `json:"entries"`
}

// ListStartupEntries 枚举自启动项（服务 + 计划任务）
func (t *ToolkitService) ListStartupEntries(args *ListStartupEntriesArgs, reply *ListStartupEntriesReply) error {
	category := strings.ToLower(strings.TrimSpace(args.Category))
	if category == "" {
		category = "all"
	}
	if category != "all" && category != "services" && category != "tasks" {
		return fmt.Errorf("category 仅支持 all/services/tasks")
	}

	entries, err := listStartupEntries(category, strings.TrimSpace(args.NameLike))
	if err != nil {
		return fmt.Errorf("枚举自启动项失败: %w", err)
	}

	reply.Category = category
	reply.Entries = entries
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
		err := fmt.Errorf("name 不能为空")
		auditWrite("start_service", map[string]any{"name": args.Name}, err)
		return err
	}
	if err := startWindowsService(args.Name); err != nil {
		reply.Success = false
		retErr := fmt.Errorf("启动服务失败: %w", err)
		auditWrite("start_service", map[string]any{"name": args.Name}, retErr)
		return retErr
	}
	reply.Success = true
	auditWrite("start_service", map[string]any{"name": args.Name}, nil)
	return nil
}

// StopService 停止服务
func (t *ToolkitService) StopService(args *ServiceActionArgs, reply *ServiceActionReply) error {
	if strings.TrimSpace(args.Name) == "" {
		err := fmt.Errorf("name 不能为空")
		auditWrite("stop_service", map[string]any{"name": args.Name}, err)
		return err
	}
	if err := stopWindowsService(args.Name); err != nil {
		reply.Success = false
		retErr := fmt.Errorf("停止服务失败: %w", err)
		auditWrite("stop_service", map[string]any{"name": args.Name}, retErr)
		return retErr
	}
	reply.Success = true
	auditWrite("stop_service", map[string]any{"name": args.Name}, nil)
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
		err := fmt.Errorf("name 不能为空")
		auditWrite("set_service_start_type", map[string]any{"name": args.Name, "start_type": args.StartType}, err)
		return err
	}
	if err := setWindowsServiceStartType(args.Name, args.StartType); err != nil {
		reply.Success = false
		retErr := fmt.Errorf("修改服务启动类型失败: %w", err)
		auditWrite("set_service_start_type", map[string]any{"name": args.Name, "start_type": args.StartType}, retErr)
		return retErr
	}
	reply.Success = true
	auditWrite("set_service_start_type", map[string]any{"name": args.Name, "start_type": args.StartType}, nil)
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
		err := fmt.Errorf("WinDrive 未加载")
		auditWrite("apply_protect_template", map[string]any{"template": args.Template}, err)
		return err
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
		err := fmt.Errorf("template 仅支持 low/medium/high")
		auditWrite("apply_protect_template", map[string]any{"template": args.Template}, err)
		return err
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
		retErr := fmt.Errorf("设置保护策略失败: %w", err)
		auditWrite("apply_protect_template", map[string]any{"template": template, "deny_access_mask": mask}, retErr)
		return retErr
	}

	reply.Success = true
	reply.Template = template
	reply.Version = 1
	reply.DenyAccessMask = mask
	auditWrite("apply_protect_template", map[string]any{"template": template, "deny_access_mask": mask}, nil)
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
