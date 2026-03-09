package service

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strings"
	"syscall"

	"github.com/OpenSysKit/backend/internal/driver"
)

const (
	driverEnumModulesOutSize       uint32 = 512 * 1024
	driverEnumThreadsOutSize       uint32 = 256 * 1024
	driverEnumKernelModulesOutSize uint32 = 512 * 1024
	driverEnumHandlesOutSize       uint32 = 8 * 1024 * 1024
	driverEnumConnectionsOutSize   uint32 = 2 * 1024 * 1024
)

// HandleEntryModel 句柄明细。
type HandleEntryModel struct {
	ProcessId       uint32 `json:"process_id"`
	Handle          uint64 `json:"handle"`
	ObjectTypeIndex uint32 `json:"object_type_index"`
	GrantedAccess   uint32 `json:"granted_access"`
	ObjectAddress   uint64 `json:"object_address"`
	TypeName        string `json:"type_name"`
	ObjectName      string `json:"object_name"`
}

// ListHandlesArgs 句柄明细请求参数。
type ListHandlesArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// ListHandlesReply 句柄明细响应。
type ListHandlesReply struct {
	ProcessId uint32             `json:"process_id"`
	Handles   []HandleEntryModel `json:"handles"`
}

// KernelModuleModel 内核模块信息。
type KernelModuleModel struct {
	BaseAddress uint64 `json:"base_address"`
	Size        uint32 `json:"size"`
	ModuleName  string `json:"module_name"`
	Path        string `json:"path"`
}

// EnumKernelModulesArgs 内核模块枚举请求参数。
type EnumKernelModulesArgs struct{}

// EnumKernelModulesReply 内核模块枚举响应。
type EnumKernelModulesReply struct {
	Modules []KernelModuleModel `json:"modules"`
}

// FreezeProcessArgs 冻结进程请求参数。
type FreezeProcessArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// FreezeProcessReply 冻结进程响应。
type FreezeProcessReply struct {
	Success bool `json:"success"`
}

// UnfreezeProcessArgs 解冻进程请求参数。
type UnfreezeProcessArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// UnfreezeProcessReply 解冻进程响应。
type UnfreezeProcessReply struct {
	Success bool `json:"success"`
}

// HideProcessArgs 隐藏进程请求参数。
type HideProcessArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// HideProcessReply 隐藏进程响应。
type HideProcessReply struct {
	Success bool `json:"success"`
}

// UnhideProcessArgs 恢复隐藏进程请求参数。
type UnhideProcessArgs struct {
	ProcessId uint32 `json:"process_id"`
}

// UnhideProcessReply 恢复隐藏进程响应。
type UnhideProcessReply struct {
	Success bool `json:"success"`
}

// InjectDllArgs DLL 注入请求参数。
type InjectDllArgs struct {
	ProcessId uint32 `json:"process_id"`
	DllPath   string `json:"dll_path"`
}

// InjectDllReply DLL 注入响应。
type InjectDllReply struct {
	Success bool `json:"success"`
}

// CloseHandleArgs 强制关闭句柄请求参数。
type CloseHandleArgs struct {
	ProcessId uint32 `json:"process_id"`
	Handle    uint64 `json:"handle"`
}

// CloseHandleReply 强制关闭句柄响应。
type CloseHandleReply struct {
	Success bool `json:"success"`
}

// UnloadDriverArgs 卸载驱动请求参数。
type UnloadDriverArgs struct {
	ServiceName string `json:"service_name"`
}

// UnloadDriverReply 卸载驱动响应。
type UnloadDriverReply struct {
	Success bool `json:"success"`
}

func encodeBinary(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeUTF16Fixed(src []uint16) string {
	return syscall.UTF16ToString(src)
}

func copyUTF16Fixed(dst []uint16, value string, fieldName string) error {
	utf16Value, err := syscall.UTF16FromString(value)
	if err != nil {
		return fmt.Errorf("%s 编码失败: %w", fieldName, err)
	}
	if len(utf16Value) > len(dst) {
		return fmt.Errorf("%s 过长，最大支持 %d UTF-16 字符", fieldName, len(dst)-1)
	}
	copy(dst, utf16Value)
	return nil
}

func parseDriverProcessList(outBuf []byte) ([]ProcessInfoModel, error) {
	headerSize := binary.Size(driver.ProcessListHeader{})
	if len(outBuf) < headerSize {
		return nil, fmt.Errorf("驱动返回的进程列表过小")
	}

	var header driver.ProcessListHeader
	if err := binary.Read(bytes.NewReader(outBuf[:headerSize]), binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	entrySize := binary.Size(driver.ProcessInfo{})
	offset := headerSize
	processes := make([]ProcessInfoModel, 0, header.Count)
	for i := uint32(0); i < header.Count && offset+entrySize <= len(outBuf); i++ {
		var info driver.ProcessInfo
		if err := binary.Read(bytes.NewReader(outBuf[offset:offset+entrySize]), binary.LittleEndian, &info); err != nil {
			return nil, err
		}
		processes = append(processes, ProcessInfoModel{
			ProcessId:       info.ProcessId,
			ParentProcessId: info.ParentProcessId,
			ThreadCount:     info.ThreadCount,
			WorkingSetSize:  info.WorkingSetSize,
			ImageName:       decodeUTF16Fixed(info.ImageName[:]),
		})
		offset += entrySize
	}
	return processes, nil
}

func processNameMapViaDriver(dev driver.Device) (map[uint32]string, error) {
	outBuf, err := dev.IoControl(driver.IOCTL_ENUM_PROCESSES, nil, 1024*1024)
	if err != nil {
		return nil, err
	}
	processes, err := parseDriverProcessList(outBuf)
	if err != nil {
		return nil, err
	}
	result := make(map[uint32]string, len(processes))
	for _, p := range processes {
		result[p.ProcessId] = p.ImageName
	}
	return result, nil
}

func enumProcessModulesViaDriver(dev driver.Device, pid uint32) ([]ProcessModuleModel, error) {
	inBuf, err := encodeBinary(driver.ProcessRequest{ProcessId: pid})
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}

	outBuf, err := dev.IoControl(driver.IOCTL_ENUM_MODULES, inBuf, driverEnumModulesOutSize)
	if err != nil {
		return nil, err
	}

	headerSize := binary.Size(driver.ModuleListHeader{})
	if len(outBuf) < headerSize {
		return nil, fmt.Errorf("驱动返回的模块列表过小")
	}

	var header driver.ModuleListHeader
	if err := binary.Read(bytes.NewReader(outBuf[:headerSize]), binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	entrySize := binary.Size(driver.ModuleInfo{})
	offset := headerSize
	modules := make([]ProcessModuleModel, 0, header.Count)
	for i := uint32(0); i < header.Count && offset+entrySize <= len(outBuf); i++ {
		var info driver.ModuleInfo
		if err := binary.Read(bytes.NewReader(outBuf[offset:offset+entrySize]), binary.LittleEndian, &info); err != nil {
			return nil, err
		}
		modules = append(modules, ProcessModuleModel{
			ProcessId:   pid,
			ModuleName:  decodeUTF16Fixed(info.BaseName[:]),
			BaseAddress: info.BaseAddress,
			Size:        info.SizeOfImage,
			Path:        decodeUTF16Fixed(info.FullPath[:]),
		})
		offset += entrySize
	}

	sort.SliceStable(modules, func(i, j int) bool {
		if modules[i].BaseAddress != modules[j].BaseAddress {
			return modules[i].BaseAddress < modules[j].BaseAddress
		}
		return modules[i].ModuleName < modules[j].ModuleName
	})
	return modules, nil
}

func enumThreadsViaDriver(dev driver.Device, pid uint32) ([]ThreadInfoModel, error) {
	inBuf, err := encodeBinary(driver.ProcessRequest{ProcessId: pid})
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}

	outBuf, err := dev.IoControl(driver.IOCTL_ENUM_THREADS, inBuf, driverEnumThreadsOutSize)
	if err != nil {
		return nil, err
	}

	headerSize := binary.Size(driver.ThreadListHeader{})
	if len(outBuf) < headerSize {
		return nil, fmt.Errorf("驱动返回的线程列表过小")
	}

	var header driver.ThreadListHeader
	if err := binary.Read(bytes.NewReader(outBuf[:headerSize]), binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	entrySize := binary.Size(driver.ThreadInfo{})
	offset := headerSize
	threads := make([]ThreadInfoModel, 0, header.Count)
	for i := uint32(0); i < header.Count && offset+entrySize <= len(outBuf); i++ {
		var info driver.ThreadInfo
		if err := binary.Read(bytes.NewReader(outBuf[offset:offset+entrySize]), binary.LittleEndian, &info); err != nil {
			return nil, err
		}
		threads = append(threads, ThreadInfoModel{
			ThreadId:      info.ThreadId,
			OwnerProcess:  info.ProcessId,
			BasePriority:  info.Priority,
			DeltaPriority: 0,
			StartAddress:  info.StartAddress,
			IsTerminating: info.IsTerminating != 0,
		})
		offset += entrySize
	}

	sort.SliceStable(threads, func(i, j int) bool { return threads[i].ThreadId < threads[j].ThreadId })
	return threads, nil
}

func listHandlesViaDriver(dev driver.Device, pid uint32) ([]HandleEntryModel, error) {
	inBuf, err := encodeBinary(driver.HandleEnumRequest{ProcessId: pid})
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}

	outBuf, err := dev.IoControl(driver.IOCTL_ENUM_HANDLES, inBuf, driverEnumHandlesOutSize)
	if err != nil {
		return nil, err
	}

	headerSize := binary.Size(driver.HandleListHeader{})
	if len(outBuf) < headerSize {
		return nil, fmt.Errorf("驱动返回的句柄列表过小")
	}

	var header driver.HandleListHeader
	if err := binary.Read(bytes.NewReader(outBuf[:headerSize]), binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	entrySize := binary.Size(driver.HandleInfo{})
	offset := headerSize
	handles := make([]HandleEntryModel, 0, header.Count)
	for i := uint32(0); i < header.Count && offset+entrySize <= len(outBuf); i++ {
		var info driver.HandleInfo
		if err := binary.Read(bytes.NewReader(outBuf[offset:offset+entrySize]), binary.LittleEndian, &info); err != nil {
			return nil, err
		}
		handles = append(handles, HandleEntryModel{
			ProcessId:       info.ProcessId,
			Handle:          info.Handle,
			ObjectTypeIndex: info.ObjectTypeIndex,
			GrantedAccess:   info.GrantedAccess,
			ObjectAddress:   info.ObjectAddress,
			TypeName:        decodeUTF16Fixed(info.TypeName[:]),
			ObjectName:      decodeUTF16Fixed(info.ObjectName[:]),
		})
		offset += entrySize
	}

	sort.SliceStable(handles, func(i, j int) bool {
		if handles[i].ProcessId != handles[j].ProcessId {
			return handles[i].ProcessId < handles[j].ProcessId
		}
		return handles[i].Handle < handles[j].Handle
	})
	return handles, nil
}

func buildHandleStats(entries []HandleEntryModel) (uint32, []HandleTypeStat) {
	counts := make(map[uint32]HandleTypeStat)
	for _, entry := range entries {
		stat := counts[entry.ObjectTypeIndex]
		stat.TypeIndex = uint16(entry.ObjectTypeIndex)
		if stat.TypeName == "" {
			stat.TypeName = entry.TypeName
		}
		if stat.TypeName == "" {
			stat.TypeName = fmt.Sprintf("type#%d", entry.ObjectTypeIndex)
		}
		stat.Count++
		counts[entry.ObjectTypeIndex] = stat
	}

	stats := make([]HandleTypeStat, 0, len(counts))
	for _, stat := range counts {
		stats = append(stats, stat)
	}

	sort.SliceStable(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].TypeIndex < stats[j].TypeIndex
	})
	return uint32(len(entries)), stats
}

func enumNetworkConnectionsViaDriver(dev driver.Device, protocol string) ([]NetworkConnectionModel, error) {
	outBuf, err := dev.IoControl(driver.IOCTL_ENUM_CONNECTIONS, nil, driverEnumConnectionsOutSize)
	if err != nil {
		return nil, err
	}

	headerSize := binary.Size(driver.ConnectionListHeader{})
	if len(outBuf) < headerSize {
		return nil, fmt.Errorf("驱动返回的连接列表过小")
	}

	var header driver.ConnectionListHeader
	if err := binary.Read(bytes.NewReader(outBuf[:headerSize]), binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	names, _ := processNameMapViaDriver(dev)
	entrySize := binary.Size(driver.ConnectionInfo{})
	offset := headerSize
	conns := make([]NetworkConnectionModel, 0, header.Count)
	for i := uint32(0); i < header.Count && offset+entrySize <= len(outBuf); i++ {
		var info driver.ConnectionInfo
		if err := binary.Read(bytes.NewReader(outBuf[offset:offset+entrySize]), binary.LittleEndian, &info); err != nil {
			return nil, err
		}

		protoName := ""
		switch info.Protocol {
		case driver.ConnectionProtoTCP:
			protoName = "tcp"
		case driver.ConnectionProtoUDP:
			protoName = "udp"
		default:
			offset += entrySize
			continue
		}
		if protocol != "all" && protocol != protoName {
			offset += entrySize
			continue
		}

		conn := NetworkConnectionModel{
			Protocol:    protoName,
			LocalIP:     driverIPString(info.LocalAddr[:], info.IsIPv6 != 0),
			LocalPort:   info.LocalPort,
			ProcessId:   info.ProcessId,
			ProcessName: names[info.ProcessId],
		}
		if protoName == "tcp" {
			conn.RemoteIP = driverIPString(info.RemoteAddr[:], info.IsIPv6 != 0)
			conn.RemotePort = info.RemotePort
			conn.State = tcpStateToString(info.State)
		}
		conns = append(conns, conn)
		offset += entrySize
	}

	sort.SliceStable(conns, func(i, j int) bool {
		if conns[i].ProcessId != conns[j].ProcessId {
			return conns[i].ProcessId < conns[j].ProcessId
		}
		if conns[i].Protocol != conns[j].Protocol {
			return conns[i].Protocol < conns[j].Protocol
		}
		if conns[i].LocalIP != conns[j].LocalIP {
			return conns[i].LocalIP < conns[j].LocalIP
		}
		return conns[i].LocalPort < conns[j].LocalPort
	})
	return conns, nil
}

func enumKernelModulesViaDriver(dev driver.Device) ([]KernelModuleModel, error) {
	outBuf, err := dev.IoControl(driver.IOCTL_ENUM_KERNEL_MODULES, nil, driverEnumKernelModulesOutSize)
	if err != nil {
		return nil, err
	}

	headerSize := binary.Size(driver.KernelModuleListHeader{})
	if len(outBuf) < headerSize {
		return nil, fmt.Errorf("驱动返回的内核模块列表过小")
	}

	var header driver.KernelModuleListHeader
	if err := binary.Read(bytes.NewReader(outBuf[:headerSize]), binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	entrySize := binary.Size(driver.KernelModuleInfo{})
	offset := headerSize
	modules := make([]KernelModuleModel, 0, header.Count)
	for i := uint32(0); i < header.Count && offset+entrySize <= len(outBuf); i++ {
		var info driver.KernelModuleInfo
		if err := binary.Read(bytes.NewReader(outBuf[offset:offset+entrySize]), binary.LittleEndian, &info); err != nil {
			return nil, err
		}
		modules = append(modules, KernelModuleModel{
			BaseAddress: info.BaseAddress,
			Size:        info.SizeOfImage,
			ModuleName:  decodeUTF16Fixed(info.BaseName[:]),
			Path:        decodeUTF16Fixed(info.FullPath[:]),
		})
		offset += entrySize
	}

	sort.SliceStable(modules, func(i, j int) bool {
		if modules[i].BaseAddress != modules[j].BaseAddress {
			return modules[i].BaseAddress < modules[j].BaseAddress
		}
		return modules[i].ModuleName < modules[j].ModuleName
	})
	return modules, nil
}

func driverIPString(raw []byte, isIPv6 bool) string {
	if isIPv6 {
		return net.IP(raw).String()
	}
	if len(raw) < 4 {
		return ""
	}
	return net.IP(raw[:4]).String()
}

func (t *ToolkitService) FreezeProcess(args *FreezeProcessArgs, reply *FreezeProcessReply) error {
	if t.Driver == nil {
		err := fmt.Errorf("驱动未加载")
		auditWrite("freeze_process", map[string]any{"process_id": args.ProcessId}, err)
		return err
	}

	inBuf, err := encodeBinary(driver.ProcessRequest{ProcessId: args.ProcessId})
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	if _, err = t.Driver.IoControl(driver.IOCTL_FREEZE_PROCESS, inBuf, 0); err != nil {
		reply.Success = false
		retErr := fmt.Errorf("冻结进程失败: %w", err)
		auditWrite("freeze_process", map[string]any{"process_id": args.ProcessId}, retErr)
		return retErr
	}

	reply.Success = true
	auditWrite("freeze_process", map[string]any{"process_id": args.ProcessId}, nil)
	return nil
}

func (t *ToolkitService) UnfreezeProcess(args *UnfreezeProcessArgs, reply *UnfreezeProcessReply) error {
	if t.Driver == nil {
		err := fmt.Errorf("驱动未加载")
		auditWrite("unfreeze_process", map[string]any{"process_id": args.ProcessId}, err)
		return err
	}

	inBuf, err := encodeBinary(driver.ProcessRequest{ProcessId: args.ProcessId})
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	if _, err = t.Driver.IoControl(driver.IOCTL_UNFREEZE_PROCESS, inBuf, 0); err != nil {
		reply.Success = false
		retErr := fmt.Errorf("解冻进程失败: %w", err)
		auditWrite("unfreeze_process", map[string]any{"process_id": args.ProcessId}, retErr)
		return retErr
	}

	reply.Success = true
	auditWrite("unfreeze_process", map[string]any{"process_id": args.ProcessId}, nil)
	return nil
}

func (t *ToolkitService) HideProcess(args *HideProcessArgs, reply *HideProcessReply) error {
	if t.Driver == nil {
		err := fmt.Errorf("驱动未加载")
		auditWrite("hide_process", map[string]any{"process_id": args.ProcessId}, err)
		return err
	}

	inBuf, err := encodeBinary(driver.ProcessRequest{ProcessId: args.ProcessId})
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	if _, err = t.Driver.IoControl(driver.IOCTL_HIDE_PROCESS, inBuf, 0); err != nil {
		reply.Success = false
		retErr := fmt.Errorf("隐藏进程失败: %w", err)
		auditWrite("hide_process", map[string]any{"process_id": args.ProcessId}, retErr)
		return retErr
	}

	reply.Success = true
	auditWrite("hide_process", map[string]any{"process_id": args.ProcessId}, nil)
	return nil
}

func (t *ToolkitService) UnhideProcess(args *UnhideProcessArgs, reply *UnhideProcessReply) error {
	if t.Driver == nil {
		err := fmt.Errorf("驱动未加载")
		auditWrite("unhide_process", map[string]any{"process_id": args.ProcessId}, err)
		return err
	}

	inBuf, err := encodeBinary(driver.ProcessRequest{ProcessId: args.ProcessId})
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	if _, err = t.Driver.IoControl(driver.IOCTL_UNHIDE_PROCESS, inBuf, 0); err != nil {
		reply.Success = false
		retErr := fmt.Errorf("恢复隐藏进程失败: %w", err)
		auditWrite("unhide_process", map[string]any{"process_id": args.ProcessId}, retErr)
		return retErr
	}

	reply.Success = true
	auditWrite("unhide_process", map[string]any{"process_id": args.ProcessId}, nil)
	return nil
}

func (t *ToolkitService) InjectDll(args *InjectDllArgs, reply *InjectDllReply) error {
	if t.Driver == nil {
		err := fmt.Errorf("驱动未加载")
		auditWrite("inject_dll", map[string]any{"process_id": args.ProcessId, "dll_path": args.DllPath}, err)
		return err
	}
	if strings.TrimSpace(args.DllPath) == "" {
		err := fmt.Errorf("dll_path 不能为空")
		auditWrite("inject_dll", map[string]any{"process_id": args.ProcessId, "dll_path": args.DllPath}, err)
		return err
	}

	var req driver.InjectDllRequest
	req.ProcessId = args.ProcessId
	if err := copyUTF16Fixed(req.DllPath[:], args.DllPath, "dll_path"); err != nil {
		return err
	}

	inBuf, err := encodeBinary(req)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	if _, err = t.Driver.IoControl(driver.IOCTL_INJECT_DLL, inBuf, 0); err != nil {
		reply.Success = false
		retErr := fmt.Errorf("注入 DLL 失败: %w", err)
		auditWrite("inject_dll", map[string]any{"process_id": args.ProcessId, "dll_path": args.DllPath}, retErr)
		return retErr
	}

	reply.Success = true
	auditWrite("inject_dll", map[string]any{"process_id": args.ProcessId, "dll_path": args.DllPath}, nil)
	return nil
}

func (t *ToolkitService) ListHandles(args *ListHandlesArgs, reply *ListHandlesReply) error {
	if t.Driver == nil {
		err := fmt.Errorf("驱动未加载")
		auditWrite("list_handles", map[string]any{"process_id": args.ProcessId}, err)
		return err
	}

	handles, err := listHandlesViaDriver(t.Driver, args.ProcessId)
	if err != nil {
		retErr := fmt.Errorf("枚举句柄明细失败: %w", err)
		auditWrite("list_handles", map[string]any{"process_id": args.ProcessId}, retErr)
		return retErr
	}

	reply.ProcessId = args.ProcessId
	reply.Handles = handles
	auditWrite("list_handles", map[string]any{"process_id": args.ProcessId, "count": len(handles)}, nil)
	return nil
}

func (t *ToolkitService) EnumKernelModules(_ *EnumKernelModulesArgs, reply *EnumKernelModulesReply) error {
	if t.Driver == nil {
		err := fmt.Errorf("驱动未加载")
		auditWrite("enum_kernel_modules", nil, err)
		return err
	}

	modules, err := enumKernelModulesViaDriver(t.Driver)
	if err != nil {
		retErr := fmt.Errorf("枚举内核模块失败: %w", err)
		auditWrite("enum_kernel_modules", nil, retErr)
		return retErr
	}

	reply.Modules = modules
	auditWrite("enum_kernel_modules", map[string]any{"count": len(modules)}, nil)
	return nil
}

func (t *ToolkitService) CloseHandle(args *CloseHandleArgs, reply *CloseHandleReply) error {
	if t.Driver == nil {
		err := fmt.Errorf("驱动未加载")
		auditWrite("close_handle", map[string]any{"process_id": args.ProcessId, "handle": args.Handle}, err)
		return err
	}

	inBuf, err := encodeBinary(driver.CloseHandleRequest{ProcessId: args.ProcessId, Handle: args.Handle})
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	if _, err = t.Driver.IoControl(driver.IOCTL_CLOSE_HANDLE, inBuf, 0); err != nil {
		reply.Success = false
		retErr := fmt.Errorf("关闭句柄失败: %w", err)
		auditWrite("close_handle", map[string]any{"process_id": args.ProcessId, "handle": args.Handle}, retErr)
		return retErr
	}

	reply.Success = true
	auditWrite("close_handle", map[string]any{"process_id": args.ProcessId, "handle": args.Handle}, nil)
	return nil
}

func (t *ToolkitService) UnloadDriver(args *UnloadDriverArgs, reply *UnloadDriverReply) error {
	if t.Driver == nil {
		err := fmt.Errorf("驱动未加载")
		auditWrite("unload_driver", map[string]any{"service_name": args.ServiceName}, err)
		return err
	}
	if strings.TrimSpace(args.ServiceName) == "" {
		err := fmt.Errorf("service_name 不能为空")
		auditWrite("unload_driver", map[string]any{"service_name": args.ServiceName}, err)
		return err
	}

	var req driver.DriverServiceRequest
	if err := copyUTF16Fixed(req.ServiceName[:], args.ServiceName, "service_name"); err != nil {
		return err
	}

	inBuf, err := encodeBinary(req)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	if _, err = t.Driver.IoControl(driver.IOCTL_UNLOAD_DRIVER, inBuf, 0); err != nil {
		reply.Success = false
		retErr := fmt.Errorf("卸载驱动失败: %w", err)
		auditWrite("unload_driver", map[string]any{"service_name": args.ServiceName}, retErr)
		return retErr
	}

	reply.Success = true
	auditWrite("unload_driver", map[string]any{"service_name": args.ServiceName}, nil)
	return nil
}
