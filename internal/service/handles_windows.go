//go:build windows

package service

import (
	"encoding/binary"
	"fmt"
	"sort"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	systemExtendedHandleInformationClass = 64
	objectTypeInformationClass           = 2
	statusInfoLengthMismatch             = 0xC0000004
	statusBufferTooSmall                 = 0xC0000023
)

var (
	modNtdll                     = windows.NewLazySystemDLL("ntdll.dll")
	procNtQuerySystemInformation = modNtdll.NewProc("NtQuerySystemInformation")
	procNtQueryObject            = modNtdll.NewProc("NtQueryObject")
)

type rawHandleEntry struct {
	processID uint32
	typeIndex uint16
	handle    uintptr
}

type unicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        *uint16
}

func enumHandleStatsByPID(pid uint32) (uint32, []HandleTypeStat, error) {
	entries, err := querySystemHandles()
	if err != nil {
		return 0, nil, err
	}

	total := uint32(0)
	counts := make(map[uint16]uint32)
	rep := make(map[uint16]uintptr)
	for _, e := range entries {
		if e.processID != pid {
			continue
		}
		total++
		counts[e.typeIndex]++
		if _, ok := rep[e.typeIndex]; !ok {
			rep[e.typeIndex] = e.handle
		}
	}

	if total == 0 {
		return 0, []HandleTypeStat{}, nil
	}

	names := resolveTypeNames(pid, rep)
	stats := make([]HandleTypeStat, 0, len(counts))
	for idx, c := range counts {
		name := names[idx]
		if name == "" {
			name = fmt.Sprintf("type#%d", idx)
		}
		stats = append(stats, HandleTypeStat{
			TypeIndex: idx,
			TypeName:  name,
			Count:     c,
		})
	}

	sort.SliceStable(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].TypeIndex < stats[j].TypeIndex
	})
	return total, stats, nil
}

func resolveTypeNames(pid uint32, reps map[uint16]uintptr) map[uint16]string {
	names := make(map[uint16]string, len(reps))
	if len(reps) == 0 {
		return names
	}

	proc, err := windows.OpenProcess(windows.PROCESS_DUP_HANDLE, false, pid)
	if err != nil {
		return names
	}
	defer windows.CloseHandle(proc)

	for idx, hv := range reps {
		var dup windows.Handle
		if err = windows.DuplicateHandle(proc, windows.Handle(hv), windows.CurrentProcess(), &dup, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
			continue
		}
		name, qErr := queryObjectTypeName(dup)
		_ = windows.CloseHandle(dup)
		if qErr != nil || name == "" {
			continue
		}
		names[idx] = name
	}
	return names
}

func queryObjectTypeName(h windows.Handle) (string, error) {
	size := uint32(512)
	ptrSize := int(unsafe.Sizeof(uintptr(0)))
	minHeader := 4 + ptrSize

	for i := 0; i < 8; i++ {
		buf := make([]byte, size)
		var retLen uint32
		r1, _, _ := procNtQueryObject.Call(
			uintptr(h),
			uintptr(objectTypeInformationClass),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(size),
			uintptr(unsafe.Pointer(&retLen)),
		)

		status := uint32(r1)
		if status == statusInfoLengthMismatch || status == statusBufferTooSmall {
			if retLen <= size {
				size *= 2
			} else {
				size = retLen + 128
			}
			continue
		}
		if status != 0 {
			return "", fmt.Errorf("NtQueryObject failed: 0x%08X", status)
		}

		if len(buf) < minHeader {
			return "", nil
		}
		us := (*unicodeString)(unsafe.Pointer(&buf[0]))
		if us.Buffer == nil || us.Length == 0 {
			return "", nil
		}
		u16 := unsafe.Slice(us.Buffer, int(us.Length/2))
		return windows.UTF16ToString(u16), nil
	}
	return "", fmt.Errorf("NtQueryObject retry exceeded")
}

func querySystemHandles() ([]rawHandleEntry, error) {
	size := uint32(1 << 20)
	ptrSize := int(unsafe.Sizeof(uintptr(0)))
	headerSize := ptrSize * 2
	entrySize := ptrSize*3 + 16

	for i := 0; i < 10; i++ {
		buf := make([]byte, size)
		var retLen uint32
		r1, _, _ := procNtQuerySystemInformation.Call(
			uintptr(systemExtendedHandleInformationClass),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(size),
			uintptr(unsafe.Pointer(&retLen)),
		)

		status := uint32(r1)
		if status == statusInfoLengthMismatch || status == statusBufferTooSmall {
			if retLen <= size {
				size *= 2
			} else {
				size = retLen + 4096
			}
			continue
		}
		if status != 0 {
			return nil, fmt.Errorf("NtQuerySystemInformation failed: 0x%08X", status)
		}

		if len(buf) < headerSize {
			return []rawHandleEntry{}, nil
		}

		count, ok := readUintPtr(buf[:ptrSize], ptrSize)
		if !ok {
			return nil, fmt.Errorf("invalid handle table header")
		}

		out := make([]rawHandleEntry, 0, count)
		offset := headerSize
		typeOffset := ptrSize*3 + 6
		for n := uintptr(0); n < count; n++ {
			if offset+entrySize > len(buf) {
				break
			}
			pidVal, ok1 := readUintPtr(buf[offset+ptrSize:offset+ptrSize*2], ptrSize)
			hv, ok2 := readUintPtr(buf[offset+ptrSize*2:offset+ptrSize*3], ptrSize)
			if !ok1 || !ok2 {
				break
			}
			typeIndex := binary.LittleEndian.Uint16(buf[offset+typeOffset : offset+typeOffset+2])
			out = append(out, rawHandleEntry{
				processID: uint32(pidVal),
				typeIndex: typeIndex,
				handle:    hv,
			})
			offset += entrySize
		}
		return out, nil
	}
	return nil, fmt.Errorf("query system handles retry exceeded")
}

func readUintPtr(b []byte, ptrSize int) (uintptr, bool) {
	if ptrSize == 8 {
		if len(b) < 8 {
			return 0, false
		}
		return uintptr(binary.LittleEndian.Uint64(b[:8])), true
	}
	if ptrSize == 4 {
		if len(b) < 4 {
			return 0, false
		}
		return uintptr(binary.LittleEndian.Uint32(b[:4])), true
	}
	return 0, false
}
