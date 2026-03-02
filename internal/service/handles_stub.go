//go:build !windows

package service

import "fmt"

func enumHandleStatsByPID(_ uint32) (uint32, []HandleTypeStat, error) {
	return 0, nil, fmt.Errorf("仅支持 Windows")
}
