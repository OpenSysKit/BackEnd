//go:build !windows

package main

import "fmt"

func runAutoUninstallMode() error {
	return fmt.Errorf("自动卸载模式仅支持 Windows")
}
