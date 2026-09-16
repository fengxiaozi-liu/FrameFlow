package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
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

func OpenVault(ctx context.Context, dir string) (*Vault, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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

func (v *Vault) Put(ctx context.Context, id, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
	if err := ctx.Err(); err != nil {
		return err
	}
	previous, existed := v.items[id]
	v.items[id] = base64.StdEncoding.EncodeToString(sealed)
	if err := v.persist(ctx); err != nil {
		if existed {
			v.items[id] = previous
		} else {
			delete(v.items, id)
		}
		return err
	}
	return nil
}

func (v *Vault) Has(ctx context.Context, id string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	_, ok := v.items[id]
	return ok, nil
}

func (v *Vault) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	previous, existed := v.items[id]
	delete(v.items, id)
	if err := v.persist(ctx); err != nil {
		if existed {
			v.items[id] = previous
		}
		return err
	}
	return nil
}

func (v *Vault) Get(ctx context.Context, id string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	v.mu.RLock()
	encoded, ok := v.items[id]
	v.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !ok {
		return "", fault.ErrNotFound
	}
	sealed, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return "", e
	}
	block, e := aes.NewCipher(v.key)
	if e != nil {
		return "", e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return "", e
	}
	if len(sealed) < gcm.NonceSize() {
		return "", errors.New("invalid credential ciphertext")
	}
	plain, e := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], []byte(id))
	return string(plain), e
}

func (v *Vault) persist(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
