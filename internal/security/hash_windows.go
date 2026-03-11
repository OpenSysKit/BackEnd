//go:build windows

package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

var (
	trustedHash   string
	trustedHashMu sync.RWMutex
	hashSkip      bool
)

// SetTrustedFrontendHash 设置受信任的前端 EXE SHA256（小写 hex）。
// 由 main 在启动时调用，传入 CI 通过 ldflags 注入的值。
// hash 为空表示 dev 构建，跳过校验。
func SetTrustedFrontendHash(sha256Hex string) {
	trustedHashMu.Lock()
	defer trustedHashMu.Unlock()
	trustedHash = strings.ToLower(strings.TrimSpace(sha256Hex))

	skip := strings.TrimSpace(os.Getenv("OPENSYSKIT_SKIP_HASH_CHECK"))
	hashSkip = skip == "1" || strings.EqualFold(skip, "true")

	if trustedHash == "" {
		log.Println("[integrity] 前端 hash 未注入（dev 构建），管道 hash 校验将跳过")
	} else if hashSkip {
		log.Println("[integrity] OPENSYSKIT_SKIP_HASH_CHECK=1，管道 hash 校验将跳过")
	} else {
		log.Printf("[integrity] 受信任前端 hash: %s", trustedHash)
	}
}

// ValidateProcessHash 校验指定 PID 对应 EXE 的 SHA256 是否与受信任值一致。
func ValidateProcessHash(pid uint32) error {
	trustedHashMu.RLock()
	expected := trustedHash
	skip := hashSkip
	trustedHashMu.RUnlock()

	if expected == "" || skip {
		return nil
	}

	imagePath, err := processImagePath(pid)
	if err != nil {
		return fmt.Errorf("读取进程路径失败(pid=%d): %w", pid, err)
	}

	actual, err := fileSHA256(imagePath)
	if err != nil {
		return fmt.Errorf("计算进程 hash 失败(pid=%d, path=%s): %w", pid, imagePath, err)
	}

	if actual != expected {
		return fmt.Errorf("前端 hash 不匹配(pid=%d)\n  期望: %s\n  实际: %s", pid, expected, actual)
	}

	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
