package ipc

import (
	"log"
	"net"

	"github.com/Microsoft/go-winio"
)

const PipeName = `\\.\pipe\OpenSysKit`

// Listen 创建 Windows 命名管道监听器。
func Listen() (net.Listener, error) {
	cfg := &winio.PipeConfig{
		// WARNING: "D:P(A;;GA;;;WD)" 赋予 Everyone 完全访问权限，仅用于开发阶段。
		// 生产环境应改为管理员组限定，如 "D:P(A;;GA;;;BA)" (仅 Builtin Administrators)。
		SecurityDescriptor: "D:P(A;;GA;;;WD)",
		MessageMode:        false,
		InputBufferSize:    65536,
		OutputBufferSize:   65536,
	}
	ln, err := winio.ListenPipe(PipeName, cfg)
	if err != nil {
		return nil, err
	}
	log.Printf("[ipc] 正在监听命名管道: %s", PipeName)
	return ln, nil
}
