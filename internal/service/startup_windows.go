//go:build windows

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/svc/mgr"
)

func listStartupEntries(category string, nameLike string) ([]StartupEntryModel, error) {
	filter := strings.ToLower(strings.TrimSpace(nameLike))
	out := make([]StartupEntryModel, 0, 128)

	if category == "all" || category == "services" {
		services, err := listStartupServices(filter)
		if err != nil {
			return nil, err
		}
		out = append(out, services...)
	}

	if category == "all" || category == "tasks" {
		tasks, err := listStartupTasks(filter)
		if err != nil {
			return nil, err
		}
		out = append(out, tasks...)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func listStartupServices(filter string) ([]StartupEntryModel, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, err
	}
	defer m.Disconnect()

	names, err := m.ListServices()
	if err != nil {
		return nil, err
	}

	out := make([]StartupEntryModel, 0, len(names))
	for _, name := range names {
		svcHandle, openErr := m.OpenService(name)
		if openErr != nil {
			continue
		}

		cfg, cfgErr := svcHandle.Config()
		st, stErr := svcHandle.Query()
		_ = svcHandle.Close()
		if cfgErr != nil || stErr != nil {
			continue
		}

		if cfg.StartType != mgr.StartAutomatic {
			continue
		}

		if filter != "" {
			target := strings.ToLower(name + " " + cfg.DisplayName)
			if !strings.Contains(target, filter) {
				continue
			}
		}

		detail := ""
		if cfg.DelayedAutoStart {
			detail = "delayed-auto"
		}

		out = append(out, StartupEntryModel{
			Source:      "service",
			Name:        name,
			DisplayName: cfg.DisplayName,
			State:       serviceStateToString(st.State),
			RunAs:       cfg.ServiceStartName,
			Command:     cfg.BinaryPathName,
			Trigger:     "auto_start",
			Detail:      detail,
		})
	}
	return out, nil
}

type taskInfo struct {
	FullName     string `json:"FullName"`
	State        string `json:"State"`
	User         string `json:"User"`
	TriggerKinds string `json:"TriggerKinds"`
	Description  string `json:"Description"`
}

func listStartupTasks(filter string) ([]StartupEntryModel, error) {
	const script = `$ErrorActionPreference='SilentlyContinue';
$items = Get-ScheduledTask | Where-Object { $_.Settings.Enabled -eq $true } | ForEach-Object {
  $kinds = @($_.Triggers | ForEach-Object { $_.CimClass.CimClassName })
  if (($kinds -contains 'MSFT_TaskBootTrigger') -or ($kinds -contains 'MSFT_TaskLogonTrigger')) {
    [PSCustomObject]@{
      FullName = [string]("{0}{1}" -f $_.TaskPath, $_.TaskName)
      State = [string]$_.State
      User = [string]$_.Principal.UserId
      TriggerKinds = [string]($kinds -join ';')
      Description = [string]$_.Description
    }
  }
}
$items | ConvertTo-Json -Depth 4 -Compress`

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("读取计划任务失败: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("读取计划任务失败: %w", err)
	}

	items, err := parseTaskJSON(output)
	if err != nil {
		return nil, fmt.Errorf("解析计划任务输出失败: %w", err)
	}

	out := make([]StartupEntryModel, 0, len(items))
	for _, it := range items {
		if filter != "" {
			target := strings.ToLower(it.FullName + " " + it.Description)
			if !strings.Contains(target, filter) {
				continue
			}
		}
		out = append(out, StartupEntryModel{
			Source:  "task",
			Name:    it.FullName,
			State:   strings.ToLower(strings.TrimSpace(it.State)),
			RunAs:   it.User,
			Trigger: it.TriggerKinds,
			Detail:  it.Description,
		})
	}
	return out, nil
}

func parseTaskJSON(raw []byte) ([]taskInfo, error) {
	data := bytes.TrimSpace(raw)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return []taskInfo{}, nil
	}

	if data[0] == '[' {
		var arr []taskInfo
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}

	var one taskInfo
	if err := json.Unmarshal(data, &one); err != nil {
		return nil, err
	}
	return []taskInfo{one}, nil
}
