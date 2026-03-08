package service

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/OpenSysKit/backend/internal/driver"
)

type killExecutionResult struct {
	Success    bool
	UsedMethod string
	NTStatus   uint32
}

func executeKillProcess(dev driver.Device, processID uint32) (killExecutionResult, error) {
	req := driver.ProcessRequest{ProcessId: processID}
	inBuf := new(bytes.Buffer)
	if err := binary.Write(inBuf, binary.LittleEndian, req); err != nil {
		return killExecutionResult{}, fmt.Errorf("构造请求失败: %w", err)
	}

	outBuf, err := dev.IoControl(driver.IOCTL_KILL_PROCESS, inBuf.Bytes(), uint32(binary.Size(driver.ProcessKillResult{})))
	if err != nil {
		return killExecutionResult{}, err
	}

	if len(outBuf) < binary.Size(driver.ProcessKillResult{}) {
		return killExecutionResult{}, fmt.Errorf("驱动返回的 Kill 结果过小: got=%d want=%d", len(outBuf), binary.Size(driver.ProcessKillResult{}))
	}

	var result driver.ProcessKillResult
	if err := binary.Read(bytes.NewReader(outBuf[:binary.Size(result)]), binary.LittleEndian, &result); err != nil {
		return killExecutionResult{}, fmt.Errorf("解析 Kill 结果失败: %w", err)
	}

	if result.Version != driver.ProcessKillResultVersion {
		return killExecutionResult{}, fmt.Errorf("驱动 Kill 结果版本不匹配: got=%d want=%d", result.Version, driver.ProcessKillResultVersion)
	}

	parsed := killExecutionResult{
		Success:    result.OperationStatus == 0,
		UsedMethod: processKillMethodName(result.Method),
		NTStatus:   result.OperationStatus,
	}
	if !parsed.Success {
		return parsed, fmt.Errorf("内核返回 NTSTATUS=%s (used_method=%s)", formatNTStatus(result.OperationStatus), parsed.UsedMethod)
	}

	return parsed, nil
}

func processKillMethodName(method uint32) string {
	switch method {
	case driver.ProcessKillMethodPsp:
		return "psp"
	case driver.ProcessKillMethodZw:
		return "zw"
	default:
		return "none"
	}
}

func formatNTStatus(status uint32) string {
	return fmt.Sprintf("0x%08X", status)
}
