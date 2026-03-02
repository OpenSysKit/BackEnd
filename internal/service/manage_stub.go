//go:build !windows

package service

import "fmt"

func enumThreadsByProcess(_ uint32) ([]ThreadInfoModel, error) {
	return nil, fmt.Errorf("仅支持 Windows")
}

func suspendThread(_ uint32) (int32, error) {
	return -1, fmt.Errorf("仅支持 Windows")
}

func resumeThread(_ uint32) (int32, error) {
	return -1, fmt.Errorf("仅支持 Windows")
}

func listWindowsServices(_ string) ([]ServiceInfoModel, error) {
	return nil, fmt.Errorf("仅支持 Windows")
}

func startWindowsService(_ string) error {
	return fmt.Errorf("仅支持 Windows")
}

func stopWindowsService(_ string) error {
	return fmt.Errorf("仅支持 Windows")
}

func setWindowsServiceStartType(_, _ string) error {
	return fmt.Errorf("仅支持 Windows")
}
