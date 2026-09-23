package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"strings"
)

// Secrets at rest are sealed with AES-256-GCM when ENCRYPTION_KEY is set. The
// key is any string; it is hashed with SHA-256 to 32 bytes. Values are stored as
// "enc:v1:<base64(nonce||ciphertext)>"; plaintext values load unchanged and are
// sealed on the next save, so enabling the key migrates existing data lazily.
const encPrefix = "enc:v1:"

var encKey []byte

func initCrypto() {
	if k := os.Getenv("ENCRYPTION_KEY"); k != "" {
		sum := sha256.Sum256([]byte(k))
		encKey = sum[:]
	} else {
		encKey = nil
	}
}

func seal(plain string) string {
	if encKey == nil || plain == "" || strings.HasPrefix(plain, encPrefix) {
		return plain
	}
	block, _ := aes.NewCipher(encKey)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	_, _ = rand.Read(nonce)
	return encPrefix + base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plain), nil))
}

func open(stored string) (string, error) {
	if !strings.HasPrefix(stored, encPrefix) {
		return stored, nil
	}
	if encKey == nil {
		return "", errors.New("stored secrets are encrypted but ENCRYPTION_KEY is not set")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, encPrefix))
	if err != nil {
		return "", err
	}
	block, _ := aes.NewCipher(encKey)
	gcm, _ := cipher.NewGCM(block)
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("encrypted value too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("failed to decrypt stored secret (wrong ENCRYPTION_KEY?)")
	}
	return string(plain), nil
}

// secretFields lists every string field that is sealed at rest.
func secretFields(c *Config) []*string {
	fields := []*string{&c.Password, &c.RelaySecret, &c.ApiKey}
	for i := range c.Accounts {
		a := &c.Accounts[i]
		fields = append(fields, &a.AccessToken, &a.RefreshToken, &a.ClientSecret, &a.RelaySecret, &a.CompatAPIKey)
	}
	for i := range c.ApiKeys {
		fields = append(fields, &c.ApiKeys[i].Key)
	}
	return fields
}

// sealConfig returns a copy of c with secrets encrypted; c itself is untouched.
func sealConfig(c *Config) *Config {
	if encKey == nil || c == nil {
		return c
	}
	cp := *c
	cp.Accounts = append([]Account(nil), c.Accounts...)
	cp.ApiKeys = append([]ApiKeyEntry(nil), c.ApiKeys...)
	for _, f := range secretFields(&cp) {
		*f = seal(*f)
	}
	return &cp
}

// openConfig decrypts secrets in place.
func openConfig(c *Config) error {
	if c == nil {
		return nil
	}
	for _, f := range secretFields(c) {
		v, err := open(*f)
		if err != nil {
			return err
		}
		*f = v
	}
	return nil
}
