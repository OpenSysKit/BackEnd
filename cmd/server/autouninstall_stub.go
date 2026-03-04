//go:build !windows

package main

import (
	"fmt"
	"time"
)

func scheduleSelfUninstall(_ time.Duration, _ []uint64) error {
	return fmt.Errorf("自动卸载仅支持 Windows")
}
