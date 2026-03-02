# OpenSysKit 接口教学文档（框架无关）

版本：`v3`  
定位：`只讲接口，不绑定前端框架`

---

## 1. 接口总览

OpenSysKit 后端通过 Windows 命名管道提供 JSON-RPC 服务。

- Pipe 名称：`\\.\pipe\OpenSysKit`
- 协议：`JSON-RPC 2.0`
- 数据边界：一条请求一行（`\n` 结尾）

你可以用任意前端/客户端实现，只要能完成：

1. 连接命名管道  
2. 发送 JSON-RPC 请求行  
3. 读取 JSON-RPC 响应行

---

## 2. 请求与响应格式

### 2.1 请求格式

```json
{
  "id": 1,
  "method": "Toolkit.ProtectProcess",
  "params": [
    {
      "process_id": 5388
    }
  ]
}
```

字段说明：

- `id`：请求 ID，前后对应即可（数字或字符串都可，建议数字）。
- `method`：方法名，见下文方法清单。
- `params`：固定为数组，当前服务使用数组第一项对象作为参数。

### 2.2 响应格式

成功：

```json
{
  "id": 1,
  "result": {
    "success": true
  },
  "error": null
}
```

失败：

```json
{
  "id": 1,
  "result": null,
  "error": {
    "code": -32000,
    "message": "错误描述"
  }
}
```

---

## 3. 方法清单

### 3.1 `Toolkit.Ping`

作用：检查链路是否连通。  
参数：空对象。

请求：

```json
{
  "id": 1,
  "method": "Toolkit.Ping",
  "params": [{}]
}
```

成功结果：

```json
{
  "status": "ok"
}
```

---

### 3.2 `Toolkit.EnumProcesses`

作用：枚举进程列表。  
参数：空对象。

请求：

```json
{
  "id": 2,
  "method": "Toolkit.EnumProcesses",
  "params": [{}]
}
```

成功结果示例：

```json
{
  "processes": [
    {
      "process_id": 5388,
      "parent_process_id": 1234,
      "thread_count": 10,
      "working_set_size": 12345678,
      "image_name": "CCleanerPerformanceOptimizerService"
    }
  ]
}
```

---

### 3.3 `Toolkit.ProtectProcess`

作用：将指定 PID 加入保护列表。  
参数：`process_id`（uint32）。

请求：

```json
{
  "id": 3,
  "method": "Toolkit.ProtectProcess",
  "params": [
    {
      "process_id": 5388
    }
  ]
}
```

成功结果：

```json
{
  "success": true
}
```

---

### 3.4 `Toolkit.UnprotectProcess`

作用：将指定 PID 从保护列表移除。  
参数：`process_id`（uint32）。

请求：

```json
{
  "id": 4,
  "method": "Toolkit.UnprotectProcess",
  "params": [
    {
      "process_id": 5388
    }
  ]
}
```

成功结果：

```json
{
  "success": true
}
```

---

### 3.5 `Toolkit.KillProcess`

作用：结束指定 PID。  
参数：`process_id`（uint32）。

请求：

```json
{
  "id": 5,
  "method": "Toolkit.KillProcess",
  "params": [
    {
      "process_id": 5388
    }
  ]
}
```

成功结果：

```json
{
  "success": true
}
```

---

### 3.6 `Toolkit.SetProtectPolicy`

作用：设置保护策略掩码。  
参数：

- `version`：当前使用 `1`
- `deny_access_mask`：访问掩码（uint32）

请求：

```json
{
  "id": 6,
  "method": "Toolkit.SetProtectPolicy",
  "params": [
    {
      "version": 1,
      "deny_access_mask": 2049
    }
  ]
}
```

说明：

- `2049 == 0x801`
- 默认表示：拦截 `PROCESS_TERMINATE(0x1)` + `PROCESS_SUSPEND_RESUME(0x800)`

成功结果：

```json
{
  "success": true
}
```

---

### 3.7 `Toolkit.ListDirectory`

作用：列出目录项（目录优先 + 名称排序）。  
参数：`path`（字符串，目录绝对路径）。

请求：

```json
{
  "id": 7,
  "method": "Toolkit.ListDirectory",
  "params": [
    {
      "path": "C:\\"
    }
  ]
}
```

成功结果示例：

```json
{
  "current_path": "C:\\",
  "parent_path": "",
  "entries": [
    {
      "name": "Windows",
      "path": "C:\\Windows",
      "is_dir": true,
      "size": 0,
      "mod_time": "2026-03-01T10:00:00+08:00"
    }
  ]
}
```

---

### 3.8 `Toolkit.DeleteFileKernel`

作用：通过 OpenSysKit 驱动执行内核文件删除。  
参数：`path`（字符串，文件绝对路径）。

请求：

```json
{
  "id": 8,
  "method": "Toolkit.DeleteFileKernel",
  "params": [
    {
      "path": "C:\\Temp\\test.txt"
    }
  ]
}
```

成功结果：

```json
{
  "success": true
}
```

---

### 3.9 `Toolkit.KillFileLockingProcesses`

作用：查询占用指定文件的进程，并调用内核 `KillProcess` 执行结束。  
参数：`path`（字符串，文件绝对路径）。

请求：

```json
{
  "id": 9,
  "method": "Toolkit.KillFileLockingProcesses",
  "params": [
    {
      "path": "C:\\Temp\\test.txt"
    }
  ]
}
```

成功结果示例：

```json
{
  "found_pids": [5388, 9524],
  "results": [
    { "process_id": 5388, "success": true },
    { "process_id": 9524, "success": false, "error": "..." }
  ]
}
```

---

### 3.10 `Toolkit.HealthCheck`

作用：健康检查面板数据源，检查后端链路与关键能力。  
参数：空对象。

请求：

```json
{
  "id": 10,
  "method": "Toolkit.HealthCheck",
  "params": [{}]
}
```

成功结果示例：

```json
{
  "overall_status": "degraded",
  "generated_at": "2026-03-02T13:00:00+08:00",
  "components": [
    { "name": "backend", "status": "ok", "message": "rpc service running" },
    { "name": "opensyskit_driver", "status": "ok", "message": "ioctl enum_processes ok" },
    { "name": "windrive_driver", "status": "degraded", "message": "windrive not connected" }
  ]
}
```

---

### 3.11 `Toolkit.EnumProcessModules`

作用：按 PID 枚举模块（模块名/基址/大小/路径）。  
参数：`process_id`（uint32）。

请求：

```json
{
  "id": 11,
  "method": "Toolkit.EnumProcessModules",
  "params": [
    {
      "process_id": 5388
    }
  ]
}
```

成功结果示例：

```json
{
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
}
```

---

### 3.12 `Toolkit.EnumNetworkConnections`

作用：枚举 TCP/UDP 连接与 PID 关联。  
参数：

- `protocol`：`all | tcp | udp`

请求：

```json
{
  "id": 12,
  "method": "Toolkit.EnumNetworkConnections",
  "params": [
    {
      "protocol": "all"
    }
  ]
}
```

成功结果示例：

```json
{
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
}
```

---

### 3.13 `Toolkit.GetProcessTree`

作用：返回当前系统进程树（按 PID 升序，含 children）。  
参数：空对象。

请求：

```json
{
  "id": 13,
  "method": "Toolkit.GetProcessTree",
  "params": [{}]
}
```

---

### 3.14 `Toolkit.KillProcessTree`

作用：按子树结束进程（默认叶子优先）。  
参数：

- `process_id`：目标 PID
- `include_root`：是否包含根进程
- `leaves_first`：是否叶子优先
- `strict_errors`：遇错即停

请求：

```json
{
  "id": 14,
  "method": "Toolkit.KillProcessTree",
  "params": [
    {
      "process_id": 5388,
      "include_root": true,
      "leaves_first": true,
      "strict_errors": false
    }
  ]
}
```

---

### 3.15 `Toolkit.EnumThreads`

作用：按 PID 枚举线程。  
参数：`process_id`（uint32）。

---

### 3.16 `Toolkit.SuspendThread`

作用：挂起指定线程。  
参数：`thread_id`（uint32）。

---

### 3.17 `Toolkit.ResumeThread`

作用：恢复指定线程。  
参数：`thread_id`（uint32）。

---

### 3.18 `Toolkit.ListServices`

作用：服务枚举（名称、显示名、状态、启动类型）。  
参数：`name_like`（可选，名称过滤）。

---

### 3.19 `Toolkit.StartService` / `Toolkit.StopService`

作用：启动或停止服务。  
参数：`name`（服务名）。

---

### 3.20 `Toolkit.SetServiceStartType`

作用：设置服务启动类型。  
参数：

- `name`：服务名
- `start_type`：`auto | manual | disabled`

---

### 3.21 `Toolkit.ApplyProtectTemplate`

作用：一键切换 WinDrive 保护模板。  
参数：`template`：`low | medium | high`。

说明：

- `low`：仅拦截 `PROCESS_TERMINATE`
- `medium`：拦截 `TERMINATE + SUSPEND_RESUME`
- `high`：拦截 `TERMINATE + VM_WRITE + SET_INFORMATION + SUSPEND_RESUME`

---

## 4. 实现注意事项

1. 每条请求末尾必须有换行 `\n`，否则服务端会一直等待。  
2. 建议每次请求使用独立 `id`。  
3. 建议客户端做超时控制（例如 3~5 秒）。  
4. 收到 `error != null` 时，应以 `error.message` 为准提示。

---

## 5. 常见错误与定位

| 现象 | 可能原因 | 处理建议 |
|---|---|---|
| 连接管道失败 | `OpenSysKit.exe` 未运行 | 先启动后端 |
| 请求无响应 | 未发送换行 | 检查是否以 `\n` 结束 |
| 返回 RPC 错误 | 参数非法或驱动状态异常 | 先 `Ping` 再重试 |
| `Protect` 成功但行为不符 | 目标进程句柄已提前获取 | 重新按“先保护再新开句柄”验证 |

