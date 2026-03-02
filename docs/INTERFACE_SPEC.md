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

