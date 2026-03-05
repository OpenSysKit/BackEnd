# OpenSysKit BackEnd 接口文档（与当前代码一致）

更新时间：2026-03-05  
适用范围：`BackEnd` 当前 `main` 分支

- 速查版： [INTERFACE_QUICK_REF.md](./INTERFACE_QUICK_REF.md)

---

## 1. 传输与协议

- 传输：Windows 命名管道 `\\.\pipe\OpenSysKit`
- RPC：Go 标准库 `net/rpc/jsonrpc`（JSON-RPC over stream）
- 服务名：`Toolkit`
- 方法名格式：`Toolkit.<Method>`

请求格式（真实）：

```json
{
  "id": 1,
  "method": "Toolkit.Ping",
  "params": [
    {}
  ]
}
```

说明：

- `params` 必须是数组，服务端按第一个对象反序列化参数。
- 缺少 `params` 时会返回：`jsonrpc: request body missing params`。

---

## 2. 响应格式（真实）

### 2.1 成功

```json
{
  "id": 1,
  "result": {"status": "ok"},
  "error": null
}
```

### 2.2 失败

```json
{
  "id": 1,
  "result": null,
  "error": "驱动未加载"
}
```

说明：

- 该实现里 `error` 是字符串，不是 `{code,message}` 对象。
- `error` 文本通常是方法里 `fmt.Errorf(...)` 的结果，可能包含底层错误拼接。

---

## 3. 接口清单（逐接口真实成功/错误返回）

## 3.1 `Toolkit.Ping`

参数：`{}`

成功返回：

```json
{
  "id": 1,
  "result": {"status": "ok"},
  "error": null
}
```

错误返回（示例，缺少 params）：

```json
{
  "id": 1,
  "result": null,
  "error": "jsonrpc: request body missing params"
}
```

## 3.2 `Toolkit.EnumProcesses`

参数：`{}`

成功返回：

```json
{
  "id": 2,
  "result": {
    "processes": [
      {
        "process_id": 5388,
        "parent_process_id": 1234,
        "thread_count": 10,
        "working_set_size": 12345678,
        "image_name": "TestTool.exe"
      }
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 2,
  "result": null,
  "error": "驱动未加载"
}
```

常见错误文本：

- `驱动未加载`
- `枚举进程失败: ...`
- `返回数据过小`
- `解析头部失败: ...`

## 3.3 `Toolkit.KillProcess`

参数：`{"process_id": <uint32>}`

成功返回：

```json
{
  "id": 3,
  "result": {"success": true},
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 3,
  "result": null,
  "error": "结束进程失败: ..."
}
```

## 3.4 `Toolkit.ProtectProcess`

参数：`{"process_id": <uint32>}`

成功返回：

```json
{
  "id": 4,
  "result": {"success": true},
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 4,
  "result": null,
  "error": "WinDrive 未加载"
}
```

## 3.5 `Toolkit.UnprotectProcess`

参数：`{"process_id": <uint32>}`

成功返回：

```json
{
  "id": 5,
  "result": {"success": true},
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 5,
  "result": null,
  "error": "取消保护进程失败: ..."
}
```

## 3.6 `Toolkit.SetProtectPolicy`

参数：

```json
{"version": 1, "deny_access_mask": 2049}
```

成功返回：

```json
{
  "id": 6,
  "result": {"success": true},
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 6,
  "result": null,
  "error": "设置保护策略失败: ..."
}
```

## 3.7 `Toolkit.ListDirectory`

参数：

```json
{"path": "C:\\"}
```

`path` 为空时默认系统盘根目录。

成功返回：

```json
{
  "id": 7,
  "result": {
    "current_path": "C:\\",
    "parent_path": "",
    "entries": [
      {
        "name": "Windows",
        "path": "C:\\Windows",
        "is_dir": true,
        "size": 0,
        "mod_time": "2026-03-05T10:00:00+08:00"
      }
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 7,
  "result": null,
  "error": "读取目录失败: ..."
}
```

## 3.8 `Toolkit.DeleteFileKernel`

参数：

```json
{"path": "C:\\Temp\\test.txt"}
```

成功返回：

```json
{
  "id": 8,
  "result": {"success": true},
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 8,
  "result": null,
  "error": "path 不能为空"
}
```

常见错误文本：

- `驱动未加载`
- `path 不能为空`
- `路径编码失败: ...`
- `路径过长，最大支持 N UTF-16 字符`
- `内核删除文件失败: ...`

## 3.9 `Toolkit.KillFileLockingProcesses`

参数：

```json
{"path": "C:\\Temp\\test.txt"}
```

成功返回（注意：即使部分 PID 失败，整体仍可能是成功响应，失败写在 `results` 内）：

```json
{
  "id": 9,
  "result": {
    "found_pids": [5388, 9524],
    "results": [
      {"process_id": 5388, "success": true},
      {"process_id": 9524, "success": false, "error": "Access is denied."}
    ]
  },
  "error": null
}
```

错误返回（示例，参数问题）：

```json
{
  "id": 9,
  "result": null,
  "error": "path 不能为空"
}
```

## 3.10 `Toolkit.EnumProcessModules`

参数：`{"process_id": <uint32>}`

成功返回：

```json
{
  "id": 10,
  "result": {
    "process_id": 5388,
    "modules": [
      {
        "process_id": 5388,
        "module_name": "kernel32.dll",
        "base_address": 140709826207744,
        "size": 770048,
        "path": "C:\\Windows\\System32\\kernel32.dll"
      }
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 10,
  "result": null,
  "error": "process_id must be > 0"
}
```

## 3.11 `Toolkit.EnumNetworkConnections`

参数：`{"protocol": "all|tcp|udp"}`（空值默认 `all`）

成功返回：

```json
{
  "id": 11,
  "result": {
    "protocol": "all",
    "connections": [
      {
        "protocol": "tcp",
        "local_ip": "127.0.0.1",
        "local_port": 19090,
        "remote_ip": "0.0.0.0",
        "remote_port": 0,
        "state": "listen",
        "process_id": 1234,
        "process_name": "TestTool.exe"
      }
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 11,
  "result": null,
  "error": "protocol 仅支持 all/tcp/udp"
}
```

## 3.12 `Toolkit.HealthCheck`

参数：`{}`

成功返回：

```json
{
  "id": 12,
  "result": {
    "overall_status": "degraded",
    "generated_at": "2026-03-05T10:00:00+08:00",
    "components": [
      {"name": "backend", "status": "ok", "message": "rpc service running"},
      {"name": "opensyskit_driver", "status": "ok", "message": "ioctl enum_processes ok"},
      {"name": "windrive_driver", "status": "degraded", "message": "windrive not connected"}
    ]
  },
  "error": null
}
```

错误返回（示例，通常仅协议层参数错误）：

```json
{
  "id": 12,
  "result": null,
  "error": "jsonrpc: request body missing params"
}
```

## 3.13 `Toolkit.GetProcessTree`

参数：`{}`

成功返回：

```json
{
  "id": 13,
  "result": {
    "total": 2,
    "roots": [
      {
        "process_id": 1,
        "parent_process_id": 0,
        "image_name": "System",
        "thread_count": 100,
        "working_set_size": 0,
        "children": [
          {
            "process_id": 5388,
            "parent_process_id": 1,
            "image_name": "TestTool.exe",
            "thread_count": 10,
            "working_set_size": 123456,
            "children": []
          }
        ]
      }
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 13,
  "result": null,
  "error": "驱动未加载"
}
```

## 3.14 `Toolkit.KillProcessTree`

参数：

```json
{
  "process_id": 5388,
  "include_root": true,
  "leaves_first": true,
  "strict_errors": false
}
```

成功返回（`strict_errors=false` 时允许部分失败）：

```json
{
  "id": 14,
  "result": {
    "target_process_id": 5388,
    "ordered_pids": [9524, 5388],
    "results": [
      {"process_id": 9524, "success": true},
      {"process_id": 5388, "success": false, "error": "Access is denied."}
    ]
  },
  "error": null
}
```

错误返回（示例，`strict_errors=true` 且中途失败）：

```json
{
  "id": 14,
  "result": null,
  "error": "结束子树进程失败(pid=5388): ..."
}
```

## 3.15 `Toolkit.EnumThreads`

参数：`{"process_id": <uint32>}`

成功返回：

```json
{
  "id": 15,
  "result": {
    "process_id": 5388,
    "threads": [
      {
        "thread_id": 12000,
        "owner_process_id": 5388,
        "base_priority": 8,
        "delta_priority": 0
      }
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 15,
  "result": null,
  "error": "process_id must be > 0"
}
```

## 3.16 `Toolkit.EnumHandles`

参数：`{"process_id": <uint32>}`

成功返回：

```json
{
  "id": 16,
  "result": {
    "process_id": 5388,
    "total_handles": 421,
    "types": [
      {"type_index": 7, "type_name": "Process", "count": 96}
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 16,
  "result": null,
  "error": "枚举句柄失败: ..."
}
```

## 3.17 `Toolkit.WatchHandleStats`

参数：

```json
{
  "process_id": 5388,
  "sample_count": 6,
  "interval_ms": 5000,
  "top_n": 5
}
```

约束与默认：

- `sample_count`: 默认 6，范围 1~60
- `interval_ms`: 默认 5000，范围 500~10000
- `top_n`: 默认 5，范围 1~20

成功返回：

```json
{
  "id": 17,
  "result": {
    "process_id": 5388,
    "samples": [
      {
        "timestamp": "2026-03-05T10:00:00+08:00",
        "total_handles": 421,
        "top_types": [
          {"type_index": 7, "type_name": "Process", "count": 96}
        ]
      }
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 17,
  "result": null,
  "error": "句柄采样失败(第 1 次): ..."
}
```

## 3.18 `Toolkit.ResolvePortConflict`

参数：

```json
{"port": 8080, "protocol": "all", "action": "kill"}
```

成功返回（注意：结果里可含部分失败项）：

```json
{
  "id": 18,
  "result": {
    "port": 8080,
    "protocol": "all",
    "action": "kill",
    "summary": "匹配连接 2 条，成功处置 1 项",
    "matches": [
      {
        "protocol": "tcp",
        "local_ip": "0.0.0.0",
        "local_port": 8080,
        "remote_ip": "0.0.0.0",
        "remote_port": 0,
        "state": "listen",
        "process_id": 1234,
        "process_name": "TestTool.exe"
      }
    ],
    "results": [
      {"process_id": 1234, "method": "kill_process", "success": true}
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 18,
  "result": null,
  "error": "action 仅支持 kill/disconnect"
}
```

常见错误文本：

- `port must be > 0`
- `protocol 仅支持 all/tcp/udp`
- `action 仅支持 kill/disconnect`
- `驱动未加载，无法执行 kill`
- `disconnect 暂仅支持 TCP`

## 3.19 `Toolkit.SuspendThread`

参数：`{"thread_id": <uint32>}`

成功返回：

```json
{
  "id": 19,
  "result": {"success": true, "suspend_count": 1},
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 19,
  "result": null,
  "error": "thread_id must be > 0"
}
```

## 3.20 `Toolkit.ResumeThread`

参数：`{"thread_id": <uint32>}`

成功返回：

```json
{
  "id": 20,
  "result": {"success": true, "suspend_count": 0},
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 20,
  "result": null,
  "error": "恢复线程失败: ..."
}
```

## 3.21 `Toolkit.ListServices`

参数：`{"name_like": ""}`（可选过滤）

成功返回：

```json
{
  "id": 21,
  "result": {
    "services": [
      {
        "name": "WinDefend",
        "display_name": "Microsoft Defender Antivirus Service",
        "state": "running",
        "start_type": "auto"
      }
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 21,
  "result": null,
  "error": "枚举服务失败: ..."
}
```

## 3.22 `Toolkit.ListStartupEntries`

参数：

```json
{"category": "all", "name_like": ""}
```

`category` 仅支持：`all/services/tasks`，空值默认 `all`。

成功返回：

```json
{
  "id": 22,
  "result": {
    "category": "all",
    "entries": [
      {
        "source": "service",
        "name": "WinDefend",
        "display_name": "Microsoft Defender Antivirus Service",
        "state": "running",
        "run_as": "LocalSystem",
        "command": "\"C:\\Program Files\\Windows Defender\\MsMpEng.exe\"",
        "trigger": "auto_start",
        "detail": ""
      }
    ]
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 22,
  "result": null,
  "error": "category 仅支持 all/services/tasks"
}
```

## 3.23 `Toolkit.StartService`

参数：`{"name": "<service_name>"}`

成功返回：

```json
{
  "id": 23,
  "result": {"success": true},
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 23,
  "result": null,
  "error": "name 不能为空"
}
```

## 3.24 `Toolkit.StopService`

参数：`{"name": "<service_name>"}`

成功返回：

```json
{
  "id": 24,
  "result": {"success": true},
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 24,
  "result": null,
  "error": "停止服务失败: ..."
}
```

## 3.25 `Toolkit.SetServiceStartType`

参数：

```json
{"name": "<service_name>", "start_type": "auto|manual|disabled"}
```

成功返回：

```json
{
  "id": 25,
  "result": {"success": true},
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 25,
  "result": null,
  "error": "修改服务启动类型失败: ..."
}
```

## 3.26 `Toolkit.ApplyProtectTemplate`

参数：`{"template": "low|medium|high"}`（空值默认 `medium`）

成功返回：

```json
{
  "id": 26,
  "result": {
    "success": true,
    "template": "medium",
    "version": 1,
    "deny_access_mask": 2049
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 26,
  "result": null,
  "error": "template 仅支持 low/medium/high"
}
```

## 3.27 `Toolkit.GetAuditLogs`

参数：`{"limit": 100}`（`<=0` 默认 100）

成功返回：

```json
{
  "id": 27,
  "result": {
    "total": 1,
    "entries": [
      {
        "id": 9,
        "timestamp": "2026-03-05T10:00:00+08:00",
        "action": "kill_process",
        "params": {"process_id": 5388},
        "success": true
      }
    ]
  },
  "error": null
}
```

错误返回（示例，协议层）：

```json
{
  "id": 27,
  "result": null,
  "error": "jsonrpc: request body missing params"
}
```

## 3.28 `Toolkit.ExportReport`

参数：

```json
{"path": "", "include_audit": true, "audit_limit": 200}
```

说明：`path` 为空时自动输出到 `OpenSysKit.exe` 同级 `reports/`。

成功返回：

```json
{
  "id": 28,
  "result": {
    "success": true,
    "path": "E:\\OpenSysKit\\BackEnd\\bin\\reports\\20260305-100000.json",
    "size": 5821
  },
  "error": null
}
```

错误返回（示例）：

```json
{
  "id": 28,
  "result": null,
  "error": "收集进程列表失败: 驱动未加载"
}
```

常见错误文本：

- `收集健康检查失败: ...`
- `收集进程列表失败: ...`
- `收集服务列表失败: ...`
- `收集网络连接失败: ...`
- `创建报告目录失败: ...`
- `序列化报告失败: ...`
- `写入报告失败: ...`

---

## 4. 开发建议

- 每次请求都带独立 `id`，便于并发对齐响应。
- 对 `error != null` 直接按字符串展示，不要按 `error.code` 解析（本实现无 code 字段）。
- 对“部分失败但整体成功”的接口（`KillFileLockingProcesses`、`KillProcessTree`、`ResolvePortConflict`），要额外检查 `result.results` 里的每一项 `success`。
