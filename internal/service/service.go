package service

import (
	"github.com/OpenSysKit/backend/internal/driver"
)

// ToolkitService 暴露给前端的 JSON-RPC 服务。
// 所有方法通过 Driver 接口与内核驱动通信。
type ToolkitService struct {
	Driver driver.Device
}

// PingArgs 连通性测试请求参数。
type PingArgs struct{}

// PingReply 连通性测试响应。
type PingReply struct {
	Status string `json:"status"`
}

// Ping 连通性测试，前端可用于检测后端服务是否存活。
func (t *ToolkitService) Ping(_ *PingArgs, reply *PingReply) error {
	reply.Status = "ok"
	return nil
}

// TODO: 根据实际功能需求添加更多 RPC 方法
