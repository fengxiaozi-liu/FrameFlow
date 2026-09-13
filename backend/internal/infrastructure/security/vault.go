package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type Vault struct {
	mu    sync.RWMutex
	key   []byte
	path  string
	items map[string]string
}

func OpenVault(dir string) (*Vault, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	keyPath := filepath.Join(dir, "vault.key")
	key, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = io.ReadFull(rand.Reader, key); err != nil {
			return nil, err
		}
		if err = os.WriteFile(keyPath, key, 0600); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("vault key must be 32 bytes")
	}
	v := &Vault{key: key, path: filepath.Join(dir, "credentials.json"), items: map[string]string{}}
	if data, readErr := os.ReadFile(v.path); readErr == nil {
		if err = json.Unmarshal(data, &v.items); err != nil {
			return nil, err
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return nil, readErr
	}
	return v, nil
}
func (v *Vault) Put(id, value string) error {
	block, e := aes.NewCipher(v.key)
	if e != nil {
		return e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return e
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, e = io.ReadFull(rand.Reader, nonce); e != nil {
		return e
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), []byte(id))
	v.mu.Lock()
	defer v.mu.Unlock()
	v.items[id] = base64.StdEncoding.EncodeToString(sealed)
	return v.persist()
}
func (v *Vault) Has(id string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	_, ok := v.items[id]
	return ok
}
func (v *Vault) Delete(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.items, id)
	return v.persist()
}
func (v *Vault) Get(id string) (string, bool) {
	v.mu.RLock()
	encoded, ok := v.items[id]
	v.mu.RUnlock()
	if !ok {
		return "", false
	}
	sealed, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return "", false
	}
	block, e := aes.NewCipher(v.key)
	if e != nil {
		return "", false
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil || len(sealed) < gcm.NonceSize() {
		return "", false
	}
	plain, e := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], []byte(id))
	return string(plain), e == nil
}
func (v *Vault) persist() error {
	data, e := json.Marshal(v.items)
	if e != nil {
		return e
	}
	tmp := v.path + ".tmp"
	if e = os.WriteFile(tmp, data, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, v.path)
}
