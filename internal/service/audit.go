package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const maxAuditEntries = 2000

type AuditEntry struct {
	ID        int64          `json:"id"`
	Timestamp string         `json:"timestamp"`
	Action    string         `json:"action"`
	Params    map[string]any `json:"params,omitempty"`
	Success   bool           `json:"success"`
	Error     string         `json:"error,omitempty"`
}

type auditStore struct {
	mu      sync.Mutex
	entries []AuditEntry
}

var (
	globalAuditStore = &auditStore{entries: make([]AuditEntry, 0, maxAuditEntries)}
	auditIDSeq       atomic.Int64
)

func auditWrite(action string, params map[string]any, err error) {
	entry := AuditEntry{
		ID:        auditIDSeq.Add(1),
		Timestamp: time.Now().Format(time.RFC3339),
		Action:    action,
		Params:    params,
		Success:   err == nil,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	globalAuditStore.append(entry)
}

func (s *auditStore) append(e AuditEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	if len(s.entries) > maxAuditEntries {
		s.entries = append([]AuditEntry(nil), s.entries[len(s.entries)-maxAuditEntries:]...)
	}
}

func (s *auditStore) list(limit int) []AuditEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > len(s.entries) {
		limit = len(s.entries)
	}
	out := make([]AuditEntry, 0, limit)
	for i := len(s.entries) - 1; i >= len(s.entries)-limit; i-- {
		out = append(out, s.entries[i])
	}
	return out
}

type GetAuditLogsArgs struct {
	Limit int `json:"limit"`
}

type GetAuditLogsReply struct {
	Total   int          `json:"total"`
	Entries []AuditEntry `json:"entries"`
}

func (t *ToolkitService) GetAuditLogs(args *GetAuditLogsArgs, reply *GetAuditLogsReply) error {
	limit := args.Limit
	if limit <= 0 {
		limit = 100
	}
	entries := globalAuditStore.list(limit)
	reply.Total = len(entries)
	reply.Entries = entries
	return nil
}

type ExportReportArgs struct {
	Path         string `json:"path"`
	IncludeAudit bool   `json:"include_audit"`
	AuditLimit   int    `json:"audit_limit"`
}

type ExportReportReply struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
}

type reportModel struct {
	GeneratedAt string           `json:"generated_at"`
	Health      HealthCheckReply `json:"health"`
	Processes   int              `json:"processes"`
	Services    int              `json:"services"`
	Connections int              `json:"connections"`
	Audit       []AuditEntry     `json:"audit,omitempty"`
}

func (t *ToolkitService) ExportReport(args *ExportReportArgs, reply *ExportReportReply) error {
	var health HealthCheckReply
	if err := t.HealthCheck(&HealthCheckArgs{}, &health); err != nil {
		return fmt.Errorf("收集健康检查失败: %w", err)
	}

	var processes EnumProcessesReply
	if err := t.EnumProcesses(&EnumProcessesArgs{}, &processes); err != nil {
		return fmt.Errorf("收集进程列表失败: %w", err)
	}

	var services ListServicesReply
	if err := t.ListServices(&ListServicesArgs{}, &services); err != nil {
		return fmt.Errorf("收集服务列表失败: %w", err)
	}

	var conns EnumNetworkConnectionsReply
	if err := t.EnumNetworkConnections(&EnumNetworkConnectionsArgs{Protocol: "all"}, &conns); err != nil {
		return fmt.Errorf("收集网络连接失败: %w", err)
	}

	report := reportModel{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Health:      health,
		Processes:   len(processes.Processes),
		Services:    len(services.Services),
		Connections: len(conns.Connections),
	}

	if args.IncludeAudit {
		limit := args.AuditLimit
		if limit <= 0 {
			limit = 200
		}
		report.Audit = globalAuditStore.list(limit)
	}

	outPath := args.Path
	if outPath == "" {
		baseDir := "."
		if exePath, err := os.Executable(); err == nil {
			baseDir = filepath.Dir(exePath)
		}
		outPath = filepath.Join(baseDir, "reports", time.Now().Format("20060102-150405")+".json")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("创建报告目录失败: %w", err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化报告失败: %w", err)
	}
	if err = os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("写入报告失败: %w", err)
	}

	reply.Success = true
	reply.Path = outPath
	reply.Size = int64(len(data))
	auditWrite("export_report", map[string]any{
		"path":          outPath,
		"include_audit": args.IncludeAudit,
		"audit_limit":   args.AuditLimit,
	}, nil)
	return nil
}
