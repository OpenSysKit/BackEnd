//go:build windows

package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

var (
	modKernel32       = windows.NewLazySystemDLL("kernel32.dll")
	procSuspendThread = modKernel32.NewProc("SuspendThread")
	procResumeThread  = modKernel32.NewProc("ResumeThread")
)

func enumThreadsByProcess(pid uint32) ([]ThreadInfoModel, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)

	var entry windows.ThreadEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err = windows.Thread32First(snap, &entry); err != nil {
		return nil, err
	}

	out := make([]ThreadInfoModel, 0, 64)
	for {
		if entry.OwnerProcessID == pid {
			out = append(out, ThreadInfoModel{
				ThreadId:      entry.ThreadID,
				OwnerProcess:  entry.OwnerProcessID,
				BasePriority:  int32(entry.BasePri),
				DeltaPriority: int32(entry.DeltaPri),
			})
		}

		err = windows.Thread32Next(snap, &entry)
		if err != nil {
			if errors.Is(err, syscall.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, err
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].ThreadId < out[j].ThreadId })
	return out, nil
}

func suspendThread(threadID uint32) (int32, error) {
	h, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, threadID)
	if err != nil {
		return -1, err
	}
	defer windows.CloseHandle(h)

	r1, _, callErr := procSuspendThread.Call(uintptr(h))
	if r1 == 0xFFFFFFFF {
		if callErr != syscall.Errno(0) {
			return -1, callErr
		}
		return -1, windows.GetLastError()
	}
	return int32(r1), nil
}

func resumeThread(threadID uint32) (int32, error) {
	h, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, threadID)
	if err != nil {
		return -1, err
	}
	defer windows.CloseHandle(h)

	r1, _, callErr := procResumeThread.Call(uintptr(h))
	if r1 == 0xFFFFFFFF {
		if callErr != syscall.Errno(0) {
			return -1, callErr
		}
		return -1, windows.GetLastError()
	}
	return int32(r1), nil
}

func listWindowsServices(nameLike string) ([]ServiceInfoModel, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, err
	}
	defer m.Disconnect()

	names, err := m.ListServices()
	if err != nil {
		return nil, err
	}

	filter := strings.ToLower(strings.TrimSpace(nameLike))
	out := make([]ServiceInfoModel, 0, len(names))

	for _, name := range names {
		if filter != "" && !strings.Contains(strings.ToLower(name), filter) {
			continue
		}
		s, err := m.OpenService(name)
		if err != nil {
			continue
		}

		cfg, cfgErr := s.Config()
		st, stErr := s.Query()
		_ = s.Close()
		if cfgErr != nil || stErr != nil {
			continue
		}

		out = append(out, ServiceInfoModel{
			Name:        name,
			DisplayName: cfg.DisplayName,
			State:       serviceStateToString(st.State),
			StartType:   serviceStartTypeToString(cfg.StartType),
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func startWindowsService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()

	if err = s.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return err
	}
	return nil
}

func stopWindowsService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()

	_, err = s.Control(svc.Stop)
	if err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return err
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st, qErr := s.Query()
		if qErr != nil {
			return qErr
		}
		if st.State == svc.Stopped {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("停止服务超时")
}

func setWindowsServiceStartType(name string, startType string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()

	cfg, err := s.Config()
	if err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(startType)) {
	case "auto":
		cfg.StartType = mgr.StartAutomatic
	case "manual":
		cfg.StartType = mgr.StartManual
	case "disabled":
		cfg.StartType = mgr.StartDisabled
	default:
		return fmt.Errorf("start_type 仅支持 auto/manual/disabled")
	}

	return s.UpdateConfig(cfg)
}

func serviceStateToString(st svc.State) string {
	switch st {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start_pending"
	case svc.StopPending:
		return "stop_pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue_pending"
	case svc.PausePending:
		return "pause_pending"
	case svc.Paused:
		return "paused"
	default:
		return "unknown"
	}
}

func serviceStartTypeToString(st uint32) string {
	switch st {
	case uint32(mgr.StartAutomatic):
		return "auto"
	case uint32(mgr.StartManual):
		return "manual"
	case uint32(mgr.StartDisabled):
		return "disabled"
	default:
		return "unknown"
	}
}
