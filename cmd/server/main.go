package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/OpenSysKit/backend/internal/driver"
	"github.com/OpenSysKit/backend/internal/ipc"
	rpcserver "github.com/OpenSysKit/backend/internal/rpc"
)

const devicePath = `\\.\OpenSysKit`

// 由 -ldflags 在编译时注入
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("OpenSysKit 后端服务正在启动... (版本: %s, 构建时间: %s)", version, buildTime)

	// 打开内核驱动设备
	var drv driver.Device
	client, err := driver.Open(devicePath)
	if err == nil {
		drv = client
		defer client.Close()
		log.Println("检测到驱动已加载，直接连接内核驱动设备")
	} else {
		log.Printf("未检测到运行中的驱动 (%v)，尝试通过 DriverLoader 加载...", err)

		// 尝试通过 DriverLoader 手动映射并加载驱动
		loader, loaderErr := driver.NewLoader("DriverLoader.sys")
		if loaderErr != nil {
			log.Printf("警告: 初始化加载器失败: %v", loaderErr)
		} else {
			// 将 loader 的清理工作放到 defer，程序退出时自动卸载目标驱动并停止加载器
			defer loader.Close()

			log.Println("加载器初始化成功，尝试映射 OpenSysKit.sys...")
			if handle, mapErr := loader.MapDriver("OpenSysKit.sys"); mapErr != nil {
				log.Printf("警告: 映射驱动失败: %v", mapErr)
			} else {
				log.Printf("驱动映射成功，句柄: %d", handle)

				// 再次尝试打开内核驱动设备
				client, err = driver.Open(devicePath)
				if err == nil {
					drv = client
					defer client.Close()
					log.Println("已连接内核驱动设备")
				} else {
					log.Printf("警告: 驱动映射成功，但打开设备仍失败: %v", err)
				}
			}
		}
	}

	// 创建 IPC 监听（命名管道）
	ln, err := ipc.Listen()
	if err != nil {
		log.Fatalf("创建 IPC 监听器失败: %v", err)
	}
	defer ln.Close()

	// 创建 JSON-RPC 服务器
	srv, err := rpcserver.NewServer(drv)
	if err != nil {
		log.Fatalf("创建 RPC 服务器失败: %v", err)
	}

	go func() {
		if err := srv.Serve(ln); err != nil {
			log.Printf("RPC 服务器错误: %v", err)
		}
	}()

	log.Println("OpenSysKit 后端服务已启动，等待前端连接...")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("正在关闭服务...")
}
