package configbundle

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/scrypt"
)

var magic = []byte("POMBUNDLE1\n")

type payload struct {
	Config  string            `json:"config"`
	Secrets map[string]string `json:"secrets,omitempty"`
}

func BuildPlain(configYAML string) []byte { return []byte(configYAML) }

func BuildEncrypted(configYAML string, secrets map[string]string, password string) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("password required to include secrets")
	}
	plain, err := json.Marshal(payload{Config: configYAML, Secrets: secrets})
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key, err := deriveKey(password, salt)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plain, nil)

	var out bytes.Buffer
	out.Write(magic)
	out.Write(salt)
	out.Write(nonce)
	out.Write(ct)
	return out.Bytes(), nil
}

func IsEncrypted(data []byte) bool { return bytes.HasPrefix(data, magic) }

func Open(data []byte, password string) (string, map[string]string, error) {
	if !IsEncrypted(data) {
		return string(data), nil, nil
	}
	body := data[len(magic):]
	if len(body) < 16+12 {
		return "", nil, fmt.Errorf("bundle truncated")
	}
	salt, rest := body[:16], body[16:]
	key, err := deriveKey(password, salt)
	if err != nil {
		return "", nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", nil, err
	}
	ns := gcm.NonceSize()
	if len(rest) < ns {
		return "", nil, fmt.Errorf("bundle truncated")
	}
	nonce, ct := rest[:ns], rest[ns:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", nil, fmt.Errorf("wrong password or corrupt bundle")
	}
	var p payload
	if err := json.Unmarshal(plain, &p); err != nil {
		return "", nil, err
	}
	return p.Config, p.Secrets, nil
}

func deriveKey(password string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(password), salt, 1<<15, 8, 1, 32)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
