package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/OpenSysKit/backend/internal/driver"
	"github.com/OpenSysKit/backend/internal/ipc"
	rpcserver "github.com/OpenSysKit/backend/internal/rpc"
	"github.com/OpenSysKit/backend/internal/security"
)

const devicePath = `\\.\OpenSysKit`

// 由 -ldflags 在编译时注入
var (
	version        = "dev"
	buildTime      = "unknown"
	frontendSHA256 = "" // CI 构建时注入前端 EXE 的 SHA256 hex，为空则跳过校验
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	hideConsoleWindow()
	logFile, logErr := setupLogFile()
	if logErr != nil {
		log.Printf("警告: 日志文件初始化失败: %v", logErr)
	} else {
		defer logFile.Close()
	}

	if shouldEnterAutoUninstallMode() {
		if err := runAutoUninstallMode(); err != nil {
			log.Fatalf("自动卸载失败: %v", err)
		}
		return
	}

	if shouldEnterUninstallMode() {
		if err := runUninstallMode(); err != nil {
			log.Fatalf("卸载失败: %v", err)
		}
		return
	}

	log.Printf("OpenSysKit 后端服务正在启动... (版本: %s, 构建时间: %s)", version, buildTime)

	security.SetTrustedFrontendHash(frontendSHA256)

	// 打开内核驱动设备
	var drv driver.Device
	mappedHandles := make([]uint64, 0, 1)
	mappedByThisProcess := false
	var loader *driver.Loader
	var err error

	loader, err = driver.NewLoader("DriverLoader.sys")
	if err != nil {
		log.Printf("警告: 初始化加载器失败: %v", err)
	} else {
		log.Println("加载器初始化成功")
	}

	client, err := driver.Open(devicePath)
	if err == nil {
		drv = client
		defer client.Close()
		log.Println("检测到驱动已加载，直接连接内核驱动设备")
	} else {
		log.Printf("未检测到运行中的驱动 (%v)，尝试通过 DriverLoader 加载...", err)

		// 尝试通过 DriverLoader 手动映射并加载驱动
		if loader == nil {
			log.Printf("警告: 加载器不可用，无法映射驱动")
		} else {
			log.Println("尝试映射 OpenSysKit.sys...")
			if handle, mapErr := loader.MapDriver("OpenSysKit.sys"); mapErr != nil {
				log.Printf("警告: 映射驱动失败: %v", mapErr)
			} else {
				mappedHandles = append(mappedHandles, handle)
				mappedByThisProcess = true
				log.Printf("驱动映射成功，句柄: %d", handle)

				// 设备/符号链接注册需要时间，带退避重试
				var openErr error
				for i := 0; i < 10; i++ {
					client, openErr = driver.Open(devicePath)
					if openErr == nil {
						drv = client
						defer client.Close()
						log.Println("已连接内核驱动设备")
						break
					}
					time.Sleep(200 * time.Millisecond)
				}
				if openErr != nil {
					log.Printf("警告: 驱动映射成功，但打开设备仍失败(重试后): %v", openErr)
				}
			}
		}
	}

	// WinDrive 仅作为驱动加载器使用，不再打开其设备句柄
	// 进程保护功能已迁移到 OpenSysKit 驱动的 PPL 实现

	// 创建 IPC 监听（命名管道）
	ln, err := ipc.Listen()
	if err != nil {
		log.Fatalf("创建 IPC 监听器失败: %v", err)
	}
	defer ln.Close()

	// 创建 JSON-RPC 服务器（不再传递 WinDrive 设备）
	srv, err := rpcserver.NewServer(drv, nil)
	if err != nil {
		log.Fatalf("创建 RPC 服务器失败: %v", err)
	}

	go func() {
		if err := srv.Serve(ln); err != nil {
			log.Printf("RPC 服务器错误: %v", err)
		}
	}()

	log.Println("OpenSysKit 后端服务已启动，等待前端连接...")

	// 启动前端进程（与后端 exe 同目录的 OpenSysKit.UI.exe）
	guard, guardErr := newFrontendGuard()
	if guardErr != nil {
		log.Printf("警告: 无法初始化前端守护 (%v)，继续以无头模式运行", guardErr)
	} else {
		// 等待命名管道就绪，再拉起前端，避免前端连接时管道还没 Listen
		if pipeErr := waitForPipe(`\\.\pipe\OpenSysKit`, 5*time.Second); pipeErr != nil {
			log.Printf("警告: %v，仍尝试启动前端", pipeErr)
		}
		if startErr := guard.Start(); startErr != nil {
			log.Printf("警告: 启动前端失败 (%v)，继续以无头模式运行", startErr)
		} else {
			log.Printf("前端守护已激活，前端 PID = %d", guard.pidOf())
			// 自保护暂时禁用
			_ = guard.pidOf()
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// 等待：前端退出 OR 管道独占连接断开 OR 系统信号
	if guard != nil && guardErr == nil {
		select {
		case <-guard.Done():
			log.Println("前端已退出，后端随之退出")
		case <-srv.Done():
			log.Println("管道独占连接已断开，后端退出")
			guard.Kill()
		case <-sig:
			log.Println("收到退出信号，正在关闭前端...")
			guard.Kill()
		}
	} else {
		select {
		case <-srv.Done():
			log.Println("管道独占连接已断开，后端退出")
		case <-sig:
		}
	}

	// 不在退出时调用 sp.cleanup()（IOCTL_UNPROTECT_PROCESS），
	// 避免在进程退出临界区向 WinDrive 发 IOCTL 触发蓝屏。
	// 保护的 PID 在进程终止后自然失效，驱动卸载时会反注册所有 callback。

	log.Println("正在关闭服务...")

	// 显式关闭设备句柄，确保在 TerminateProcess 前释放
	if drv != nil {
		if c, ok := drv.(*driver.Client); ok {
			log.Println("关闭 OpenSysKit 设备句柄")
			c.Close()
			drv = nil
		}
	}

	// 确保 Loader 句柄也被关闭
	if loader != nil {
		log.Println("关闭 DriverLoader 句柄")
		loader.Close()
		loader = nil
	}

	// 卸载不能再放在主进程退出阶段自动执行：
	// 在当前系统环境下会触发 KiSystemServiceExitPico / APC_INDEX_MISMATCH 蓝屏。
	// 因此这里只释放本进程句柄，并明确提示使用显式 uninstall 命令完成完整卸载链路。
	if mappedByThisProcess && len(mappedHandles) > 0 {
		log.Printf("已跳过退出阶段自动卸载以避免蓝屏: mapped_by_this_process=%t, handles=%s，请稍后手动执行 OpenSysKit.exe uninstall --handles=%s",
			mappedByThisProcess,
			formatHandleList(mappedHandles),
			formatHandleList(mappedHandles),
		)
	} else {
		log.Printf("退出时无需卸载映射驱动: mapped_by_this_process=%t, handles=%s", mappedByThisProcess, formatHandleList(mappedHandles))
	}

	log.Println("主进程已完成资源释放，正常退出")
	return
}

func shouldEnterUninstallMode() bool {
	if len(os.Args) < 2 {
		return false
	}
	arg := os.Args[1]
	return arg == "uninstall" || arg == "--uninstall"
}

func shouldEnterAutoUninstallMode() bool {
	if len(os.Args) < 2 {
		return false
	}
	arg := os.Args[1]
	return arg == "autouninstall" || arg == "--autouninstall"
}

func autoUninstallEnabled() bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("OPENSYSKIT_AUTO_UNINSTALL")))
	if raw == "" {
		return true
	}
	switch raw {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}
