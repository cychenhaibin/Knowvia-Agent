package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

type fluxACredentialCipher struct {
	aead cipher.AEAD
}

func NewFluxACredentialCipher(key []byte) (FluxACredentialCipher, error) {
	if len(key) != 32 {
		return nil, errors.New("FluxA credentials key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return fluxACredentialCipher{aead: aead}, nil
}

func (c fluxACredentialCipher) Encrypt(plaintext string, additionalData []byte) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plaintext), additionalData)
	return base64.RawStdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func (c fluxACredentialCipher) Decrypt(ciphertext string, additionalData []byte) (string, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(ciphertext)
	if err != nil || len(decoded) < c.aead.NonceSize() {
		return "", errors.New("invalid FluxA credential ciphertext")
	}
	plaintext, err := c.aead.Open(nil, decoded[:c.aead.NonceSize()], decoded[c.aead.NonceSize():], additionalData)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
