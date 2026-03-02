package rpc

import (
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/OpenSysKit/backend/internal/driver"
	"github.com/OpenSysKit/backend/internal/service"
)

// Server 封装 JSON-RPC 服务器。
type Server struct {
	rpcServer *rpc.Server
}

// NewServer 创建 JSON-RPC 服务器并注册服务。
// 传入 driver.Device 接口使 RPC 方法可以与内核驱动通信，可为 nil（驱动未加载时）。
func NewServer(drv driver.Device, winDrive driver.Device) (*Server, error) {
	s := rpc.NewServer()

	toolkit := &service.ToolkitService{
		Driver:         drv,
		WinDriveDriver: winDrive,
	}
	if err := s.RegisterName("Toolkit", toolkit); err != nil {
		return nil, err
	}

	return &Server{rpcServer: s}, nil
}

// Serve 接受连接并使用 JSON-RPC 处理请求。
func (s *Server) Serve(ln net.Listener) error {
	log.Println("[rpc] JSON-RPC 服务器已就绪")
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	log.Printf("[rpc] 新连接: %s", conn.RemoteAddr())
	s.rpcServer.ServeCodec(jsonrpc.NewServerCodec(conn))
	log.Printf("[rpc] 连接已关闭: %s", conn.RemoteAddr())
}
