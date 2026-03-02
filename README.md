# OpenSysKit

> Windows 内核级系统工具箱

OpenSysKit 是一款 Windows 平台的内核级系统管理工具，提供进程管理、驱动管理等底层系统操作能力。本仓库为后端服务，负责与内核驱动通信并向前端暴露 JSON-RPC 接口。

## 快速开始

### 命令行卸载模式

用于手动清理驱动链路（先卸载 OpenSysKit，再在无其他映射驱动时卸载 WinDrive）：

```powershell
.\bin\OpenSysKit.exe uninstall
```

行为约束：

- 若 WinDrive 映射驱动数量为 `0`：直接进入 WinDrive 卸载流程。
- 若映射驱动数量为 `1`：按 OpenSysKit 处理并卸载后继续。
- 若映射驱动数量 `>1`：拒绝自动卸载，提示先手动处理，避免误卸载其他驱动。

## 许可证

待定
