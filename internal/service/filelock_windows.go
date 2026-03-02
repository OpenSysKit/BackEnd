//go:build windows

package service

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	rmAppNameLen       = 255
	rmServiceNameLen   = 63
	rmSessionKeyLen    = 32
	rmErrorSuccess     = 0
	rmRebootReasonNone = 0
)

type rmUniqueProcess struct {
	ProcessID        uint32
	ProcessStartTime windows.Filetime
}

type rmProcessInfo struct {
	Process          rmUniqueProcess
	AppName          [rmAppNameLen + 1]uint16
	ServiceShortName [rmServiceNameLen + 1]uint16
	ApplicationType  uint32
	AppStatus        uint32
	TSSessionID      uint32
	Restartable      int32
}

var (
	modRstrtMgr             = windows.NewLazySystemDLL("rstrtmgr.dll")
	procRmStartSession      = modRstrtMgr.NewProc("RmStartSession")
	procRmRegisterResources = modRstrtMgr.NewProc("RmRegisterResources")
	procRmGetList           = modRstrtMgr.NewProc("RmGetList")
	procRmEndSession        = modRstrtMgr.NewProc("RmEndSession")
)

func findLockingProcessIDs(path string) ([]uint32, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("路径编码失败: %w", err)
	}

	var session uint32
	var sessionKey [rmSessionKeyLen + 1]uint16
	if code, callErr := rmStartSession(&session, &sessionKey[0]); code != rmErrorSuccess {
		return nil, fmt.Errorf("RmStartSession 失败: code=%d err=%v", code, callErr)
	}
	defer rmEndSession(session)

	if code, callErr := rmRegisterFile(session, pathPtr); code != rmErrorSuccess {
		return nil, fmt.Errorf("RmRegisterResources 失败: code=%d err=%v", code, callErr)
	}

	infos, code, callErr := rmGetList(session)
	if code != rmErrorSuccess {
		return nil, fmt.Errorf("RmGetList 失败: code=%d err=%v", code, callErr)
	}

	pidMap := make(map[uint32]struct{}, len(infos))
	result := make([]uint32, 0, len(infos))
	for _, info := range infos {
		pid := info.Process.ProcessID
		if pid == 0 {
			continue
		}
		if _, exists := pidMap[pid]; exists {
			continue
		}
		pidMap[pid] = struct{}{}
		result = append(result, pid)
	}

	return result, nil
}

func rmStartSession(session *uint32, sessionKey *uint16) (uint32, error) {
	r1, _, e1 := procRmStartSession.Call(
		uintptr(unsafe.Pointer(session)),
		0,
		uintptr(unsafe.Pointer(sessionKey)),
	)
	if e1 != windows.ERROR_SUCCESS && e1 != nil {
		return uint32(r1), e1
	}
	return uint32(r1), nil
}

func rmRegisterFile(session uint32, filePath *uint16) (uint32, error) {
	files := []*uint16{filePath}
	r1, _, e1 := procRmRegisterResources.Call(
		uintptr(session),
		uintptr(uint32(len(files))),
		uintptr(unsafe.Pointer(&files[0])),
		0,
		0,
		0,
		0,
	)
	if e1 != windows.ERROR_SUCCESS && e1 != nil {
		return uint32(r1), e1
	}
	return uint32(r1), nil
}

func rmGetList(session uint32) ([]rmProcessInfo, uint32, error) {
	var needed uint32
	var count uint32
	rebootReasons := uint32(rmRebootReasonNone)

	r1, _, e1 := procRmGetList.Call(
		uintptr(session),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&count)),
		0,
		uintptr(unsafe.Pointer(&rebootReasons)),
	)
	code := uint32(r1)
	if code != rmErrorSuccess && code != uint32(windows.ERROR_MORE_DATA) {
		if e1 != windows.ERROR_SUCCESS && e1 != nil {
			return nil, code, e1
		}
		return nil, code, nil
	}

	if needed == 0 {
		return nil, rmErrorSuccess, nil
	}

	infos := make([]rmProcessInfo, needed)
	count = needed
	r1, _, e1 = procRmGetList.Call(
		uintptr(session),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&infos[0])),
		uintptr(unsafe.Pointer(&rebootReasons)),
	)
	code = uint32(r1)
	if code != rmErrorSuccess {
		if e1 != windows.ERROR_SUCCESS && e1 != nil {
			return nil, code, e1
		}
		return nil, code, nil
	}

	return infos[:count], rmErrorSuccess, nil
}

func rmEndSession(session uint32) {
	_, _, _ = procRmEndSession.Call(uintptr(session))
}
