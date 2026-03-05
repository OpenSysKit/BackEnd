# OpenSysKit BackEnd

OpenSysKit BackEnd 是 Windows 平台的命名管道 JSON-RPC 服务，负责：

- 与 `OpenSysKit.sys` 交互（进程/文件等内核能力）
- 与 `DriverLoader.sys`(WinDrive) 交互（进程保护策略）
- 向前端暴露 `Toolkit.*` RPC 接口

## 文档入口

- 接口完整文档（逐接口 JSON 示例）：[docs/INTERFACE_SPEC.md](./docs/INTERFACE_SPEC.md)
- 接口速查表（前端对接一页版）：[docs/INTERFACE_QUICK_REF.md](./docs/INTERFACE_QUICK_REF.md)

两份文档均按当前代码实现维护，包含每个接口的真实成功/错误返回。

## 运行

```powershell
cd BackEnd

go build -o bin/OpenSysKit.exe ./cmd/server
.\bin\OpenSysKit.exe
```

## 卸载模式

```powershell
.\bin\OpenSysKit.exe uninstall
```

说明：

- `uninstall` 会先处理映射驱动，再在满足条件时卸载 WinDrive 服务
- 支持按 handle 指定卸载目标（见接口文档）
