package rpc

import (
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"sync/atomic"

	"github.com/OpenSysKit/backend/internal/driver"
	"github.com/OpenSysKit/backend/internal/security"
	"github.com/OpenSysKit/backend/internal/service"
)

// Server 封装 JSON-RPC 服务器。
type Server struct {
	rpcServer *rpc.Server
	claimed   atomic.Bool
	done      chan struct{}
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

	return &Server{
		rpcServer: s,
		done:      make(chan struct{}),
	}, nil
}

// Done returns a channel that is closed when the exclusive connection ends.
func (s *Server) Done() <-chan struct{} {
	return s.done
}

// Serve 接受连接并使用 JSON-RPC 处理请求。
// 严格独占模式：仅接受第一个通过验证的连接，此后所有连接均被拒绝。
// 当该连接断开后，监听器关闭，服务器退出。
func (s *Server) Serve(ln net.Listener) error {
	log.Println("[rpc] JSON-RPC 服务器已就绪 (严格独占模式)")
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
			}
			return err
		}

		if s.claimed.Load() {
			log.Printf("[rpc] 拒绝连接: 管道已被独占，不允许新连接")
			conn.Close()
			continue
		}

		if err := security.ValidatePipeClient(conn); err != nil {
			log.Printf("[rpc] 客户端验证失败，拒绝连接: %v", err)
			conn.Close()
			continue
		}

		if !s.claimed.CompareAndSwap(false, true) {
			log.Printf("[rpc] 拒绝连接: 管道已被独占 (竞争)")
			conn.Close()
			continue
		}

		log.Printf("[rpc] 接受独占连接: %s", conn.RemoteAddr())

		go func() {
			defer func() {
				conn.Close()
				log.Println("[rpc] 独占连接已断开，关闭监听器，后端将退出")
				ln.Close()
				close(s.done)
			}()
			s.rpcServer.ServeCodec(jsonrpc.NewServerCodec(conn))
		}()
	}
}
