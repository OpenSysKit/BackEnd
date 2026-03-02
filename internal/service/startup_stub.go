//go:build !windows

package service

import "fmt"

func listStartupEntries(_, _ string) ([]StartupEntryModel, error) {
	return nil, fmt.Errorf("仅支持 Windows")
}
