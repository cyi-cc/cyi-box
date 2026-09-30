// Package config 进程配置：全部走环境变量，零配置文件。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cyi-cc/cyi-box/backend/internal/vault"
)

type Config struct {
	Port          uint16   // CYIBOX_PORT，默认 8890
	DBPath        string   // CYIBOX_DB，默认 ./data/cyibox.db
	AdminName     string   // CYIBOX_ADMIN_NAME，默认 admin（首启种子账号）
	AdminPassword string   // CYIBOX_ADMIN_PASSWORD，默认 admin123（首启种子密码）
	CorsOrigins   []string // CYIBOX_CORS 逗号分隔白名单；开发走 vite 代理可留空
	VaultKey      []byte   // 密码箱字段加密密钥（env CYIBOX_VAULT_KEY 或自动生成落盘）
	DiskDir       string   // 网盘文件落盘目录，默认 <db目录>/files
}

func (c *Config) New() error {
	c.Port = uint16(envInt("CYIBOX_PORT", 8890))
	c.DBPath = envStr("CYIBOX_DB", "./data/cyibox.db")
	c.AdminName = envStr("CYIBOX_ADMIN_NAME", "admin")
	c.AdminPassword = envStr("CYIBOX_ADMIN_PASSWORD", "admin123")
	if v := strings.TrimSpace(os.Getenv("CYIBOX_CORS")); v != "" {
		for _, o := range strings.Split(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				c.CorsOrigins = append(c.CorsOrigins, o)
			}
		}
	}
	if c.Port == 0 {
		return fmt.Errorf("config: invalid CYIBOX_PORT")
	}
	if dir := filepath.Dir(c.DBPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: create db dir failed: %w", err)
		}
	}
	key, err := loadVaultKey(filepath.Dir(c.DBPath))
	if err != nil {
		return err
	}
	c.VaultKey = key
	c.DiskDir = envStr("CYIBOX_DISK_DIR", filepath.Join(filepath.Dir(c.DBPath), "files"))
	if err := os.MkdirAll(c.DiskDir, 0o755); err != nil {
		return fmt.Errorf("config: create disk dir failed: %w", err)
	}
	return nil
}

// loadVaultKey CYIBOX_VAULT_KEY 优先；否则 <db目录>/vault.key 存在即读，
// 不存在则随机生成并以 0600 落盘——密钥独立于 db 文件，db 被拷走也解不开密文
func loadVaultKey(dir string) ([]byte, error) {
	if v := strings.TrimSpace(os.Getenv("CYIBOX_VAULT_KEY")); v != "" {
		return vault.DeriveKey(v), nil
	}
	path := filepath.Join(dir, "vault.key")
	if raw, err := os.ReadFile(path); err == nil {
		return vault.DeriveKey(string(raw)), nil
	}
	material, err := vault.RandomKey()
	if err != nil {
		return nil, fmt.Errorf("config: generate vault key failed: %w", err)
	}
	if err := os.WriteFile(path, []byte(material), 0o600); err != nil {
		return nil, fmt.Errorf("config: write vault key failed: %w", err)
	}
	return vault.DeriveKey(material), nil
}

func envStr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
