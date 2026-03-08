# OpenSysKit BackEnd 接口速查

更新时间：2026-03-08  
适用范围：`BackEnd` 当前 `main` 分支

- 完整版（含完整 JSON 示例）：[INTERFACE_SPEC.md](./INTERFACE_SPEC.md)
- 本文定位：快速对接时查参数、成功字段、错误字符串

---

## 1. 通用约定

请求包：

```json
{"id": 1, "method": "Toolkit.Ping", "params": [{}]}
```

成功包：

```json
{"id": 1, "result": {...}, "error": null}
```

失败包（真实）：

```json
{"id": 1, "result": null, "error": "错误字符串"}
```

说明：`error` 是字符串，不是 `{code,message}`；流式客户端应同时用 `id` 对齐请求与响应。

---

## 2. 接口速查

## 2.1 `Toolkit.Ping`
- `params`: `{}`
- 成功 `result`: `{"status":"ok"}`
- 错误 `error` 示例: `jsonrpc: request body missing params`

## 2.2 `Toolkit.EnumProcesses`
- `params`: `{}`
- 成功 `result`: `{"processes":[{process_id,parent_process_id,thread_count,working_set_size,image_name}]}`
- 错误 `error` 示例: `驱动未加载` / `枚举进程失败: ...` / `返回数据过小`

## 2.3 `Toolkit.KillProcess`
- `params`: `{"process_id":uint32}`
- 成功 `result`: `{"success":true,"used_method":"psp|zw","nt_status":0}`
- 内核拒绝时 `result`: `{"success":false,"used_method":"none|psp|zw","nt_status":<ntstatus>}`
- 错误 `error` 示例: `驱动未加载` / `结束进程失败: 驱动返回的 Kill 结果过小: ...`

## 2.4 `Toolkit.ProtectProcess`
- `params`: `{"process_id":uint32}`
- 成功 `result`: `{"success":true}`
- 错误 `error` 示例: `WinDrive 未加载` / `保护进程失败: ...`

## 2.5 `Toolkit.UnprotectProcess`
- `params`: `{"process_id":uint32}`
- 成功 `result`: `{"success":true}`
- 错误 `error` 示例: `WinDrive 未加载` / `取消保护进程失败: ...`

## 2.6 `Toolkit.SetProtectPolicy`
- `params`: `{"version":uint32,"deny_access_mask":uint32}`
- 成功 `result`: `{"success":true}`
- 错误 `error` 示例: `WinDrive 未加载` / `设置保护策略失败: ...`

## 2.7 `Toolkit.ListDirectory`
- `params`: `{"path":"绝对路径"}`（空值默认 `C:\\`）
- 成功 `result`: `{"current_path":"...","parent_path":"...","entries":[{name,path,is_dir,size,mod_time}]}`
- 错误 `error` 示例: `路径解析失败: ...` / `读取目录失败: ...`

## 2.8 `Toolkit.DeleteFileKernel`
- `params`: `{"path":"文件路径"}`
- 成功 `result`: `{"success":true}`
- 错误 `error` 示例: `path 不能为空` / `驱动未加载` / `路径编码失败: ...` / `路径过长，最大支持 N UTF-16 字符` / `内核删除文件失败: ...`

## 2.9 `Toolkit.KillFileLockingProcesses`
- `params`: `{"path":"文件路径"}`
- 成功 `result`: `{"found_pids":[...],"results":[{process_id,success,used_method?,nt_status,error?}]}`
- 错误 `error` 示例: `path 不能为空` / `驱动未加载` / `查询占用进程失败: ...`
- 注意: 该接口即使部分 PID 失败，也可能 `error=null`，需检查 `results[].success`。

## 2.10 `Toolkit.EnumProcessModules`
- `params`: `{"process_id":uint32}`
- 成功 `result`: `{"process_id":5388,"modules":[{process_id,module_name,base_address,size,path}]}`
- 错误 `error` 示例: `process_id must be > 0` / `枚举进程模块失败: ...`

## 2.11 `Toolkit.EnumNetworkConnections`
- `params`: `{"protocol":"all|tcp|udp"}`（空值默认 `all`）
- 成功 `result`: `{"protocol":"all","connections":[{protocol,local_ip,local_port,remote_ip,remote_port,state,process_id,process_name}]}`
- 错误 `error` 示例: `protocol 仅支持 all/tcp/udp` / `枚举网络连接失败: ...`

## 2.12 `Toolkit.HealthCheck`
- `params`: `{}`
- 成功 `result`: `{"overall_status":"ok|degraded|down","generated_at":"RFC3339","components":[{name,status,message}]}`
- 错误 `error` 示例: `jsonrpc: request body missing params`

## 2.13 `Toolkit.GetProcessTree`
- `params`: `{}`
- 成功 `result`: `{"total":N,"roots":[{process_id,parent_process_id,image_name,thread_count,working_set_size,children:[]}]}`
- 错误 `error` 示例: `驱动未加载` / `枚举进程失败: ...`

## 2.14 `Toolkit.KillProcessTree`
- `params`: `{"process_id":uint32,"include_root":bool,"leaves_first":bool,"strict_errors":bool}`
- 成功 `result`: `{"target_process_id":...,"ordered_pids":[...],"results":[{process_id,success,used_method?,nt_status,error?}]}`
- 错误 `error` 示例: `process_id must be > 0` / `驱动未加载` / `结束子树进程失败(pid=...): ...`
- 注意: `strict_errors=false` 时，部分失败也可整体成功。

## 2.15 `Toolkit.EnumThreads`
- `params`: `{"process_id":uint32}`
- 成功 `result`: `{"process_id":...,"threads":[{thread_id,owner_process_id,base_priority,delta_priority}]}`
- 错误 `error` 示例: `process_id must be > 0` / `枚举线程失败: ...`

## 2.16 `Toolkit.EnumHandles`
- `params`: `{"process_id":uint32}`
- 成功 `result`: `{"process_id":...,"total_handles":N,"types":[{type_index,type_name,count}]}`
- 错误 `error` 示例: `process_id must be > 0` / `枚举句柄失败: ...`

## 2.17 `Toolkit.WatchHandleStats`
- `params`: `{"process_id":uint32,"sample_count":int,"interval_ms":int,"top_n":int}`
- 成功 `result`: `{"process_id":...,"samples":[{timestamp,total_handles,top_types:[...]}]}`
- 错误 `error` 示例: `process_id must be > 0` / `句柄采样失败(第 1 次): ...`
- 约束: `sample_count 1~60`、`interval_ms 500~10000`、`top_n 1~20`。

## 2.18 `Toolkit.ResolvePortConflict`
- `params`: `{"port":uint16,"protocol":"all|tcp|udp","action":"kill|disconnect"}`
- 成功 `result`: `{"port":...,"protocol":"...","action":"...","summary":"...","matches":[...],"results":[{process_id,method,success,used_method?,nt_status,error?}]}`
- `results[].method` 当前枚举：`kill_process` / `disconnect_tcp`
- 错误 `error` 示例: `port must be > 0` / `protocol 仅支持 all/tcp/udp` / `action 仅支持 kill/disconnect` / `驱动未加载，无法执行 kill` / `disconnect 暂仅支持 TCP` / `断开 TCP 连接失败: ...`
- 注意: `action=kill` 时，高风险系统进程会在 `results` 中返回 `success=false,error="高风险系统进程，拒绝结束"`，但整体仍可能成功。

## 2.19 `Toolkit.SuspendThread`
- `params`: `{"thread_id":uint32}`
- 成功 `result`: `{"success":true,"suspend_count":int}`
- 错误 `error` 示例: `thread_id must be > 0` / `挂起线程失败: ...`

## 2.20 `Toolkit.ResumeThread`
- `params`: `{"thread_id":uint32}`
- 成功 `result`: `{"success":true,"suspend_count":int}`
- 错误 `error` 示例: `thread_id must be > 0` / `恢复线程失败: ...`

## 2.21 `Toolkit.ListServices`
- `params`: `{"name_like":"可选过滤"}`
- 成功 `result`: `{"services":[{name,display_name,state,start_type}]}`
- 错误 `error` 示例: `枚举服务失败: ...`

## 2.22 `Toolkit.ListStartupEntries`
- `params`: `{"category":"all|services|tasks","name_like":"可选过滤"}`（空值默认 `all`）
- 成功 `result`: `{"category":"all","entries":[{source,name,display_name?,state?,run_as?,command?,trigger?,detail?}]}`
- 错误 `error` 示例: `category 仅支持 all/services/tasks` / `枚举自启动项失败: ...`

## 2.23 `Toolkit.StartService`
- `params`: `{"name":"服务名"}`
- 成功 `result`: `{"success":true}`
- 错误 `error` 示例: `name 不能为空` / `启动服务失败: ...`

## 2.24 `Toolkit.StopService`
- `params`: `{"name":"服务名"}`
- 成功 `result`: `{"success":true}`
- 错误 `error` 示例: `name 不能为空` / `停止服务失败: ...`

## 2.25 `Toolkit.SetServiceStartType`
- `params`: `{"name":"服务名","start_type":"auto|manual|disabled"}`
- 成功 `result`: `{"success":true}`
- 错误 `error` 示例: `name 不能为空` / `start_type 仅支持 auto/manual/disabled` / `修改服务启动类型失败: ...`

## 2.26 `Toolkit.ApplyProtectTemplate`
- `params`: `{"template":"low|medium|high"}`（空值默认 `medium`）
- 成功 `result`: `{"success":true,"template":"medium","version":1,"deny_access_mask":2049}`
- 错误 `error` 示例: `WinDrive 未加载` / `template 仅支持 low/medium/high` / `设置保护策略失败: ...`

## 2.27 `Toolkit.GetAuditLogs`
- `params`: `{"limit":int}`（`<=0` 默认 `100`）
- 成功 `result`: `{"total":N,"entries":[{id,timestamp,action,params?,success,error?}]}`
- 错误 `error` 示例: `jsonrpc: request body missing params`

## 2.28 `Toolkit.ExportReport`
- `params`: `{"path":"可空","include_audit":bool,"audit_limit":int}`
- 成功 `result`: `{"success":true,"path":"...\\reports\\*.json","size":5821}`
- 错误 `error` 示例: `收集进程列表失败: 驱动未加载` / `写入报告失败: ...`

---

## 3. 前端对接建议

- 每次请求使用唯一 `id`，并校验响应 `id` 与请求一致。
- 对所有写操作接口，不只看顶层 `error`，还要看 `result` 内的 `success` 或 `results[]`。
- 统一把 `error` 当纯文本提示。
