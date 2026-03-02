//go:build !windows

package service

import "fmt"

func enumProcessModules(_ uint32) ([]ProcessModuleModel, error) {
	return nil, fmt.Errorf("仅支持 Windows")
}

func enumNetworkConnections(_ string) ([]NetworkConnectionModel, error) {
	return nil, fmt.Errorf("仅支持 Windows")
}
