package ciphers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
)

const (
	keySize    = 32
	encKeySize = 256
)

func ReadPublicKey(filepath string) (*rsa.PublicKey, error) {
	p, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(p)
	if block == nil {
		return nil, errors.New("error decoding pem block")
	}

	k, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return k.(*rsa.PublicKey), nil
}

func ReadPrivateKey(filepath string) (*rsa.PrivateKey, error) {
	p, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(p)
	if block == nil {
		return nil, errors.New("error decoding pem block")
	}

	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return k.(*rsa.PrivateKey), nil
}

func EncryptMsg(publicKey *rsa.PublicKey, message []byte) (encryptedMsg []byte, err error) {
	aesKey := make([]byte, keySize)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	encryptedMsg = gcm.Seal(nonce, nonce, message, nil)

	hash := sha256.New()
	encryptedKey, err := rsa.EncryptOAEP(hash, rand.Reader, publicKey, aesKey, nil)
	if err != nil {
		return nil, err
	}

	return slices.Concat(encryptedKey, encryptedMsg), nil
}

func DecryptMsg(privateKey *rsa.PrivateKey, encMsg []byte) ([]byte, error) {
	hash := sha256.New()
	fmt.Println(len(encMsg))
	encKey := encMsg[:encKeySize]
	encMsg = encMsg[encKeySize:]

	aesKey, err := rsa.DecryptOAEP(hash, rand.Reader, privateKey, encKey, nil)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encMsg) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := encMsg[:nonceSize], encMsg[nonceSize:]
	msg, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
