// Package vault 密码箱原语：AES-256-GCM 字段加密 + RFC 6238 TOTP + otpauth:// 解析。
// 不引入第三方依赖，全部标准库实现。
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// DeriveKey 任意密钥材料规整为 32 字节 AES key（hex/base64 解码失败则取 sha256）
func DeriveKey(material string) []byte {
	material = strings.TrimSpace(material)
	if b, err := hex.DecodeString(material); err == nil && len(b) >= 16 {
		sum := sha256.Sum256(b)
		return sum[:]
	}
	if b, err := base64.StdEncoding.DecodeString(material); err == nil && len(b) >= 16 {
		sum := sha256.Sum256(b)
		return sum[:]
	}
	sum := sha256.Sum256([]byte(material))
	return sum[:]
}

// RandomKey 生成 32 字节随机 key 的 hex 表示
func RandomKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Encrypt AES-256-GCM → base64(nonce|ct)
func Encrypt(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt 解密 Encrypt 产物
func Decrypt(key []byte, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("vault: ciphertext too short")
	}
	pt, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("vault: decrypt failed")
	}
	return string(pt), nil
}

// ParseSecret 接受裸 base32 密钥或 otpauth://totp/...?secret=... 链接，统一返回 base32 密钥。
// 输入非法返回 error，服务层转成业务错误码。
func ParseSecret(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", errors.New("empty secret")
	}
	if strings.HasPrefix(strings.ToLower(s), "otpauth://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", fmt.Errorf("vault: bad otpauth uri: %w", err)
		}
		secret := u.Query().Get("secret")
		if secret == "" {
			return "", errors.New("vault: otpauth uri missing secret")
		}
		s = secret
	}
	// 容错：去空格、横线，转大写；无 padding 形式也接受
	clean := strings.NewReplacer(" ", "", "-", "").Replace(s)
	clean = strings.ToUpper(clean)
	if _, err := decodeBase32(clean); err != nil {
		return "", fmt.Errorf("vault: invalid base32 secret: %w", err)
	}
	return clean, nil
}

func decodeBase32(s string) ([]byte, error) {
	if b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s); err == nil {
		return b, nil
	}
	return base32.StdEncoding.DecodeString(s)
}

// TOTP 计算当前验证码（30s 步长，6 位，SHA1），同时返回本码剩余秒数
func TOTP(secret string, t time.Time) (string, int64, error) {
	key, err := decodeBase32(secret)
	if err != nil {
		return "", 0, fmt.Errorf("vault: invalid totp secret: %w", err)
	}
	counter := uint64(t.Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset])&0x7f)<<24 |
		(uint32(sum[offset+1]))<<16 |
		(uint32(sum[offset+2]))<<8 |
		uint32(sum[offset+3])
	secondsLeft := 30 - (t.Unix() % 30)
	return fmt.Sprintf("%06d", code%1000000), secondsLeft, nil
}
