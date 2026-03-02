//go:build windows

package service

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	tcpTableOwnerPIDAll = 5
	udpTableOwnerPID    = 1
	afInet              = 2
)

var (
	modIphlpapi                           = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable               = modIphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable               = modIphlpapi.NewProc("GetExtendedUdpTable")
	errNoMoreFiles          syscall.Errno = syscall.ERROR_NO_MORE_FILES
)

func enumProcessModules(pid uint32) ([]ProcessModuleModel, error) {
	flags := uint32(windows.TH32CS_SNAPMODULE | windows.TH32CS_SNAPMODULE32)
	snap, err := windows.CreateToolhelp32Snapshot(flags, pid)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)

	var entry windows.ModuleEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err = windows.Module32First(snap, &entry); err != nil {
		return nil, err
	}

	out := make([]ProcessModuleModel, 0, 64)
	seen := make(map[string]struct{})
	for {
		module := ProcessModuleModel{
			ProcessId:   pid,
			ModuleName:  windows.UTF16ToString(entry.Module[:]),
			BaseAddress: uint64(entry.ModBaseAddr),
			Size:        entry.ModBaseSize,
			Path:        windows.UTF16ToString(entry.ExePath[:]),
		}
		key := fmt.Sprintf("%s:%d", module.Path, module.BaseAddress)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			out = append(out, module)
		}

		if err = windows.Module32Next(snap, &entry); err != nil {
			if errors.Is(err, errNoMoreFiles) {
				break
			}
			return nil, err
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].BaseAddress != out[j].BaseAddress {
			return out[i].BaseAddress < out[j].BaseAddress
		}
		return out[i].ModuleName < out[j].ModuleName
	})

	return out, nil
}

func enumNetworkConnections(protocol string) ([]NetworkConnectionModel, error) {
	names, err := processNameMap()
	if err != nil {
		return nil, err
	}

	results := make([]NetworkConnectionModel, 0, 512)

	if protocol == "all" || protocol == "tcp" {
		tcpRows, err := queryTCPv4()
		if err != nil {
			return nil, err
		}
		for _, row := range tcpRows {
			results = append(results, NetworkConnectionModel{
				Protocol:    "tcp",
				LocalIP:     ipv4FromDWORD(row.localAddr),
				LocalPort:   ntohs(row.localPort),
				RemoteIP:    ipv4FromDWORD(row.remoteAddr),
				RemotePort:  ntohs(row.remotePort),
				State:       tcpStateToString(row.state),
				ProcessId:   row.pid,
				ProcessName: names[row.pid],
			})
		}
	}

	if protocol == "all" || protocol == "udp" {
		udpRows, err := queryUDPv4()
		if err != nil {
			return nil, err
		}
		for _, row := range udpRows {
			results = append(results, NetworkConnectionModel{
				Protocol:    "udp",
				LocalIP:     ipv4FromDWORD(row.localAddr),
				LocalPort:   ntohs(row.localPort),
				ProcessId:   row.pid,
				ProcessName: names[row.pid],
			})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].ProcessId != results[j].ProcessId {
			return results[i].ProcessId < results[j].ProcessId
		}
		if results[i].Protocol != results[j].Protocol {
			return results[i].Protocol < results[j].Protocol
		}
		if results[i].LocalIP != results[j].LocalIP {
			return results[i].LocalIP < results[j].LocalIP
		}
		return results[i].LocalPort < results[j].LocalPort
	})

	return results, nil
}

type tcpRow struct {
	state      uint32
	localAddr  uint32
	localPort  uint32
	remoteAddr uint32
	remotePort uint32
	pid        uint32
}

type udpRow struct {
	localAddr uint32
	localPort uint32
	pid       uint32
}

func queryTCPv4() ([]tcpRow, error) {
	buf, err := queryIPHelperTable(procGetExtendedTcpTable, tcpTableOwnerPIDAll)
	if err != nil {
		return nil, err
	}
	if len(buf) < 4 {
		return []tcpRow{}, nil
	}

	count := binary.LittleEndian.Uint32(buf[:4])
	rows := make([]tcpRow, 0, count)
	offset := 4
	const rowSize = 24
	for i := uint32(0); i < count && offset+rowSize <= len(buf); i++ {
		row := tcpRow{
			state:      binary.LittleEndian.Uint32(buf[offset : offset+4]),
			localAddr:  binary.LittleEndian.Uint32(buf[offset+4 : offset+8]),
			localPort:  binary.LittleEndian.Uint32(buf[offset+8 : offset+12]),
			remoteAddr: binary.LittleEndian.Uint32(buf[offset+12 : offset+16]),
			remotePort: binary.LittleEndian.Uint32(buf[offset+16 : offset+20]),
			pid:        binary.LittleEndian.Uint32(buf[offset+20 : offset+24]),
		}
		rows = append(rows, row)
		offset += rowSize
	}
	return rows, nil
}

func queryUDPv4() ([]udpRow, error) {
	buf, err := queryIPHelperTable(procGetExtendedUdpTable, udpTableOwnerPID)
	if err != nil {
		return nil, err
	}
	if len(buf) < 4 {
		return []udpRow{}, nil
	}

	count := binary.LittleEndian.Uint32(buf[:4])
	rows := make([]udpRow, 0, count)
	offset := 4
	const rowSize = 12
	for i := uint32(0); i < count && offset+rowSize <= len(buf); i++ {
		row := udpRow{
			localAddr: binary.LittleEndian.Uint32(buf[offset : offset+4]),
			localPort: binary.LittleEndian.Uint32(buf[offset+4 : offset+8]),
			pid:       binary.LittleEndian.Uint32(buf[offset+8 : offset+12]),
		}
		rows = append(rows, row)
		offset += rowSize
	}
	return rows, nil
}

func queryIPHelperTable(proc *windows.LazyProc, tableClass uint32) ([]byte, error) {
	var size uint32
	r1, _, _ := proc.Call(
		0,
		uintptr(unsafe.Pointer(&size)),
		0,
		uintptr(afInet),
		uintptr(tableClass),
		0,
	)
	if r1 != 0 && syscall.Errno(r1) != windows.ERROR_INSUFFICIENT_BUFFER {
		return nil, syscall.Errno(r1)
	}
	if size == 0 {
		return []byte{}, nil
	}

	buf := make([]byte, size)
	r1, _, _ = proc.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		uintptr(afInet),
		uintptr(tableClass),
		0,
	)
	if r1 != 0 {
		return nil, syscall.Errno(r1)
	}
	return buf[:size], nil
}

func processNameMap() (map[uint32]string, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)

	out := make(map[uint32]string, 1024)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err = windows.Process32First(snap, &entry); err != nil {
		return nil, err
	}

	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		out[entry.ProcessID] = name
		if err = windows.Process32Next(snap, &entry); err != nil {
			if errors.Is(err, errNoMoreFiles) {
				break
			}
			return nil, err
		}
	}

	return out, nil
}

func ipv4FromDWORD(v uint32) string {
	ip := net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
	return ip.String()
}

func ntohs(v uint32) uint16 {
	p := uint16(v & 0xFFFF)
	return (p<<8)&0xFF00 | p>>8
}

func tcpStateToString(state uint32) string {
	switch state {
	case 1:
		return "closed"
	case 2:
		return "listen"
	case 3:
		return "syn_sent"
	case 4:
		return "syn_received"
	case 5:
		return "established"
	case 6:
		return "fin_wait_1"
	case 7:
		return "fin_wait_2"
	case 8:
		return "close_wait"
	case 9:
		return "closing"
	case 10:
		return "last_ack"
	case 11:
		return "time_wait"
	case 12:
		return "delete_tcb"
	default:
		return "unknown"
	}
}
