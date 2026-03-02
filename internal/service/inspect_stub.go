//go:build !windows

package service

import "fmt"

func enumProcessModules(_ uint32) ([]ProcessModuleModel, error) {
	return nil, fmt.Errorf("仅支持 Windows")
}

func enumNetworkConnections(_ string) ([]NetworkConnectionModel, error) {
	return nil, fmt.Errorf("仅支持 Windows")
}

type tcpDisconnectResult struct {
	ProcessId uint32
	Success   bool
	Error     string
}

func disconnectTCPByLocalPort(_ uint16, _ map[uint32]struct{}) ([]tcpDisconnectResult, error) {
	return nil, fmt.Errorf("仅支持 Windows")
}
