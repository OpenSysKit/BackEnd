# OpenSysKit 接口文档（前端对接）

版本：`v1`  
状态：`Draft / 可直接实现`  
适用前端：`Web / Electron / 任意 JS 客户端`

---

## 1. 设计说明

浏览器 JS 不能直接访问 Windows 命名管道 `\\.\pipe\OpenSysKit`。  
因此前端建议对接 **本地 HTTP Bridge**，由 Bridge 再转发到命名管道 JSON-RPC。

调用链：

`Frontend(JS) -> http://127.0.0.1:19090 -> Pipe Bridge -> \\.\pipe\OpenSysKit`

---

## 2. 统一响应格式

成功：

```json
{
  "ok": true,
  "data": {}
}
```

失败：

```json
{
  "ok": false,
  "error": "错误信息"
}
```

---

## 3. HTTP 接口

### 3.1 连通性检查

- Method: `GET`
- Path: `/api/ping`

Response:

```json
{
  "ok": true,
  "data": {
    "rpc": {
      "status": "ok"
    }
  }
}
```

---

### 3.2 枚举进程

- Method: `GET`
- Path: `/api/processes`

Response:

```json
{
  "ok": true,
  "data": {
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
}
```

---

### 3.3 保护进程

- Method: `POST`
- Path: `/api/protect`
- Body:

```json
{
  "process_id": 5388
}
```

Response:

```json
{
  "ok": true,
  "data": {
    "success": true
  }
}
```

---

### 3.4 取消保护进程

- Method: `POST`
- Path: `/api/unprotect`
- Body:

```json
{
  "process_id": 5388
}
```

Response:

```json
{
  "ok": true,
  "data": {
    "success": true
  }
}
```

---

### 3.5 结束进程（内核 kill）

- Method: `POST`
- Path: `/api/kill`
- Body:

```json
{
  "process_id": 5388
}
```

Response:

```json
{
  "ok": true,
  "data": {
    "success": true
  }
}
```

---

### 3.6 设置保护策略

- Method: `POST`
- Path: `/api/policy`
- Body:

```json
{
  "version": 1,
  "deny_access_mask": 2049
}
```

说明：
- `2049 == 0x801`，即 `PROCESS_TERMINATE(0x1) + PROCESS_SUSPEND_RESUME(0x800)`。

Response:

```json
{
  "ok": true,
  "data": {
    "success": true
  }
}
```

---

## 4. 错误码建议（前端提示文案可复用）

| 场景 | 建议文案 |
|---|---|
| OpenSysKit 未启动 | `后端未连接，请先启动 OpenSysKit.exe` |
| 管道连接失败 | `无法连接内核通道，请检查驱动状态` |
| process_id 非法 | `PID 参数不合法` |
| RPC 返回错误 | `内核调用失败：{message}` |

---

## 5. 安全约束（前端必须遵守）

1. 前端仅调用本地回环地址：`127.0.0.1`。  
2. 不在页面保存敏感参数（例如 token/证书路径）。  
3. 所有危险操作（`kill/protect/unprotect`）需要二次确认。  
4. 前端不直接持有驱动句柄，不直接访问管道。  

---

## 6. 最小调用示例（浏览器 JS）

```js
async function protect(pid) {
  const res = await fetch("http://127.0.0.1:19090/api/protect", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ process_id: pid })
  });
  const data = await res.json();
  if (!data.ok) throw new Error(data.error);
  return data.data;
}
```

---

## 7. 与现有 Pipe JSON-RPC 对应关系

| HTTP Bridge | Pipe JSON-RPC |
|---|---|
| `GET /api/ping` | `Toolkit.Ping` |
| `GET /api/processes` | `Toolkit.EnumProcesses` |
| `POST /api/protect` | `Toolkit.ProtectProcess` |
| `POST /api/unprotect` | `Toolkit.UnprotectProcess` |
| `POST /api/kill` | `Toolkit.KillProcess` |
| `POST /api/policy` | `Toolkit.SetProtectPolicy` |

