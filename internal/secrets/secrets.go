package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/pomelohq/pomelo/internal/paths"
)

var mu sync.Mutex

func keyPath() string { return paths.StatePath("secret.key") }
func storePath(session string) string {
	return paths.StatePath(filepath.Join("secrets", sanitize(session)+".bin"))
}

func sanitize(s string) string {
	if s == "" {
		return "_"
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '/' || r == '\\' || r == '.' || r == ' ' {
			r = '_'
		}
		out = append(out, r)
	}
	return string(out)
}

func loadKey() ([]byte, error) {
	p := keyPath()
	if b, err := os.ReadFile(p); err == nil && len(b) == 32 {
		return b, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func gcm() (cipher.AEAD, error) {
	key, err := loadKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func load(session string) (map[string]string, error) {
	m := map[string]string{}
	data, err := os.ReadFile(storePath(session))
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return m, nil
	}
	aead, err := gcm()
	if err != nil {
		return nil, err
	}
	ns := aead.NonceSize()
	if len(data) < ns {
		return nil, fmt.Errorf("secrets store corrupt")
	}
	plain, err := aead.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("secrets decrypt failed (key changed?): %w", err)
	}
	if err := json.Unmarshal(plain, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func save(session string, m map[string]string) error {
	plain, err := json.Marshal(m)
	if err != nil {
		return err
	}
	aead, err := gcm()
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	sealed := aead.Seal(nonce, nonce, plain, nil)
	p := storePath(session)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, sealed, 0o600)
}

func Get(session, name string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	m, err := load(session)
	if err != nil {
		return "", false
	}
	v, ok := m[name]
	return v, ok
}

func Names(session string) []string {
	mu.Lock()
	defer mu.Unlock()
	m, err := load(session)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(m))
	for k := range m {
		if strings.HasPrefix(k, "__") {
			continue
		}
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func Set(session, name, value string) error {
	mu.Lock()
	defer mu.Unlock()
	m, err := load(session)
	if err != nil {
		return err
	}
	if value == "" {
		delete(m, name)
	} else {
		m[name] = value
	}
	return save(session, m)
}
