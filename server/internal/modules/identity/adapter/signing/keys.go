// Package signing holds the instance's Ed25519 key (M1/P1 design 3.4): it
// signs the access tokens and, through a key derived from it, tags the
// refresh tokens. The key never leaves this package and is never logged.
package signing

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// macInfo separates the MAC key from the signing key (HKDF info).
const macInfo = "nervewiki refresh-token mac v1"

// Keys are the signing key and the MAC key derived from its seed.
type Keys struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	mac     []byte
}

// ParseKeys reads a PKCS#8 PEM Ed25519 private key, the format of
// `openssl genpkey -algorithm ed25519`. Errors never quote the input.
func ParseKeys(pemData []byte) (*Keys, error) {
	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("not a PEM \"PRIVATE KEY\" block (PKCS#8)")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8: %w", err)
	}
	private, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("the key is %T, want an Ed25519 key", key)
	}
	return newKeys(private)
}

// EphemeralKeys generates a key for this process only: dev and test without
// auth.jwt.private_key_file. Its tokens do not outlive a restart.
func EphemeralKeys() *Keys {
	_, private, _ := ed25519.GenerateKey(nil) // crypto/rand; never fails
	k, _ := newKeys(private)                  // cannot fail for a generated key
	return k
}

func newKeys(private ed25519.PrivateKey) (*Keys, error) {
	mac, err := hkdf.Key(sha256.New, private.Seed(), nil, macInfo, 32)
	if err != nil {
		return nil, fmt.Errorf("derive the MAC key: %w", err)
	}
	return &Keys{private: private, public: private.Public().(ed25519.PublicKey), mac: mac}, nil
}
