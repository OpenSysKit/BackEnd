//go:build windows

package security

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const basePipeSDDL = "D:P(A;;GA;;;SY)(A;;GA;;;BA)"

var (
	modKernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	procGetNamedPipeClientProcessId = modKernel32.NewProc("GetNamedPipeClientProcessId")
)

// BuildPipeSecurityDescriptor 返回命名管道 SDDL:
// SYSTEM + Administrators + 当前进程用户 SID。
func BuildPipeSecurityDescriptor() (string, error) {
	sid, err := CurrentUserSIDString()
	if err != nil {
		return basePipeSDDL, err
	}
	return basePipeSDDL + "(A;;GA;;;" + sid + ")", nil
}

// ValidatePipeClient 校验命名管道客户端是否可信：
// 1) 客户端进程用户 SID 必须与当前进程用户 SID 一致
// 2) 可选: OPENSYSKIT_PIPE_ALLOWED_IMAGES 白名单限制可执行文件名（分号分隔）
func ValidatePipeClient(conn net.Conn) error {
	pid, err := getNamedPipeClientPID(conn)
	if err != nil {
		return fmt.Errorf("读取管道客户端 PID 失败: %w", err)
	}

	clientSID, err := processUserSIDString(pid)
	if err != nil {
		return fmt.Errorf("读取客户端SID失败(pid=%d): %w", pid, err)
	}

	currentSID, err := CurrentUserSIDString()
	if err != nil {
		return fmt.Errorf("读取当前进程SID失败: %w", err)
	}

	if !strings.EqualFold(clientSID, currentSID) {
		return fmt.Errorf("客户端SID不匹配(pid=%d, sid=%s)", pid, clientSID)
	}

	if err := validateAllowedClientImage(pid); err != nil {
		return err
	}

	return nil
}

func CurrentUserSIDString() (string, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer token.Close()

	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

func getNamedPipeClientPID(conn net.Conn) (uint32, error) {
	fd, ok := conn.(interface{ Fd() uintptr })
	if !ok {
		return 0, fmt.Errorf("连接对象不支持 Fd")
	}

	var pid uint32
	r1, _, e1 := procGetNamedPipeClientProcessId.Call(fd.Fd(), uintptr(unsafe.Pointer(&pid)))
	if r1 == 0 {
		if e1 != windows.ERROR_SUCCESS && e1 != nil {
			return 0, error(e1)
		}
		return 0, fmt.Errorf("GetNamedPipeClientProcessId 调用失败")
	}
	if pid == 0 {
		return 0, fmt.Errorf("客户端PID为0")
	}
	return pid, nil
}

func processUserSIDString(pid uint32) (string, error) {
	hProc, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(hProc)

	var token windows.Token
	if err := windows.OpenProcessToken(hProc, windows.TOKEN_QUERY, &token); err != nil {
		return "", err
	}
	defer token.Close()

	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

func processImagePath(pid uint32) (string, error) {
	hProc, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(hProc)

	size := uint32(1024)
	buf := make([]uint16, size)
	if err := windows.QueryFullProcessImageName(hProc, 0, &buf[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:size]), nil
}

func validateAllowedClientImage(pid uint32) error {
	raw := strings.TrimSpace(os.Getenv("OPENSYSKIT_PIPE_ALLOWED_IMAGES"))
	if raw == "" {
		return nil
	}

	allowed := make(map[string]struct{})
	for _, part := range strings.Split(raw, ";") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name != "" {
			allowed[name] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil
	}

	imagePath, err := processImagePath(pid)
	if err != nil {
		return fmt.Errorf("读取客户端进程路径失败(pid=%d): %w", pid, err)
	}

	base := strings.ToLower(filepath.Base(imagePath))
	if _, ok := allowed[base]; !ok {
		return fmt.Errorf("客户端进程不在白名单(pid=%d, image=%s)", pid, base)
	}
	return nil
}
