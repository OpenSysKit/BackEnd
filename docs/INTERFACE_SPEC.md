# OpenSysKit 接口文档（Electron 直连 Pipe）

版本：`v2`  
状态：`Draft / 可直接实现`  
适用前端：`Electron (JS)`

---

## 1. 总体方案

采用 **Electron 主进程（Main Process）直接连接命名管道**：

- Pipe: `\\.\pipe\OpenSysKit`
- 协议: `JSON-RPC 2.0`（每条请求/响应一行，`\n` 分隔）

调用链：

`Renderer -> preload -> Electron main -> \\.\pipe\OpenSysKit`

说明：
- Renderer 不直接访问管道
- Node `net.connect('\\\\.\\pipe\\OpenSysKit')` 仅在 main 侧使用

---

## 2. 传输协议

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
    "message": "xxx"
  }
}
```

### 2.3 连接策略

1. 支持短连接：每次请求单独连接管道。  
2. 支持长连接：一个连接发送多次请求。  
3. 前端建议默认短连接，逻辑更简单、故障隔离更清晰。

---

## 3. 方法定义

### 3.1 `Toolkit.Ping`

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

请求：

```json
{
  "id": 2,
  "method": "Toolkit.EnumProcesses",
  "params": [{}]
}
```

成功结果：

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
- `2049 == 0x801`，即 `PROCESS_TERMINATE(0x1) + PROCESS_SUSPEND_RESUME(0x800)`。

成功结果：

```json
{
  "success": true
}
```

---

## 4. Electron 最小实现（JS）

### 4.1 Main 进程（Pipe 客户端）

```js
// main/pipeClient.js
const net = require("net");

const PIPE_NAME = "\\\\.\\pipe\\OpenSysKit";

function callRpc(method, params = {}) {
  return new Promise((resolve, reject) => {
    const id = Date.now();
    const req = JSON.stringify({ id, method, params: [params] }) + "\n";

    const client = net.connect(PIPE_NAME, () => client.write(req));

    let buf = "";
    client.on("data", chunk => {
      buf += chunk.toString("utf8");
      const idx = buf.indexOf("\n");
      if (idx >= 0) {
        const line = buf.slice(0, idx);
        client.end();
        try {
          const resp = JSON.parse(line);
          if (resp.error) {
            reject(new Error(resp.error.message || "RPC error"));
            return;
          }
          resolve(resp.result ?? {});
        } catch (e) {
          reject(e);
        }
      }
    });

    client.on("error", reject);
  });
}

module.exports = { callRpc };
```

### 4.2 IPC 暴露给 Renderer

```js
// main/main.js
const { ipcMain } = require("electron");
const { callRpc } = require("./pipeClient");

ipcMain.handle("osk:protect", (_, pid) =>
  callRpc("Toolkit.ProtectProcess", { process_id: pid })
);
ipcMain.handle("osk:unprotect", (_, pid) =>
  callRpc("Toolkit.UnprotectProcess", { process_id: pid })
);
ipcMain.handle("osk:kill", (_, pid) =>
  callRpc("Toolkit.KillProcess", { process_id: pid })
);
ipcMain.handle("osk:enum", () =>
  callRpc("Toolkit.EnumProcesses", {})
);
ipcMain.handle("osk:ping", () =>
  callRpc("Toolkit.Ping", {})
);
```

### 4.3 Preload 白名单 API

```js
// preload.js
const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("osk", {
  ping: () => ipcRenderer.invoke("osk:ping"),
  enumProcesses: () => ipcRenderer.invoke("osk:enum"),
  protect: pid => ipcRenderer.invoke("osk:protect", pid),
  unprotect: pid => ipcRenderer.invoke("osk:unprotect", pid),
  kill: pid => ipcRenderer.invoke("osk:kill", pid),
});
```

---

## 5. 错误处理建议

| 场景 | 前端提示 |
|---|---|
| Pipe 连接失败 | `无法连接 OpenSysKit 后端，请确认 OpenSysKit.exe 已启动` |
| RPC error != null | `内核调用失败：{message}` |
| 参数非法 | `参数错误，请检查 PID 或策略值` |
| 请求超时 | `请求超时，请重试` |

---

## 6. 安全要求（Electron）

1. `nodeIntegration = false`。  
2. `contextIsolation = true`。  
3. Renderer 只能通过 `preload` 暴露的白名单方法访问能力。  
4. 不在 Renderer 拼接任意 method 字符串，method 固定映射。  
5. 所有危险动作（`kill/protect/unprotect`）必须二次确认。

