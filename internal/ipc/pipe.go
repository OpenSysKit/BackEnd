package ipc

import (
	"log"
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/OpenSysKit/backend/internal/security"
)

const PipeName = `\\.\pipe\OpenSysKit`

// Listen 创建 Windows 命名管道监听器。
func Listen() (net.Listener, error) {
	sddl, err := security.BuildPipeSecurityDescriptor()
	if err != nil {
		log.Printf("[ipc] 警告: 生成Pipe SDDL失败，回退到基础ACL: %v", err)
	}

	cfg := &winio.PipeConfig{
		// 默认仅允许 SYSTEM + Administrators + 当前用户 SID。
		SecurityDescriptor: sddl,
		MessageMode:        false,
		InputBufferSize:    65536,
		OutputBufferSize:   65536,
	}
	ln, err := winio.ListenPipe(PipeName, cfg)
	if err != nil {
		return nil, err
	}
	log.Printf("[ipc] 正在监听命名管道: %s (SDDL=%s)", PipeName, sddl)
	return ln, nil
}
