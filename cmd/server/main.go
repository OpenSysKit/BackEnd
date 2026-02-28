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
	if client, err := driver.Open(devicePath); err != nil {
		// 驱动未加载时仅警告，不阻塞启动，方便开发调试
		log.Printf("警告: 打开驱动设备失败（驱动可能未加载）: %v", err)
	} else {
		drv = client
		defer client.Close()
		log.Println("已连接内核驱动设备")
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
