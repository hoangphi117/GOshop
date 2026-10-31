package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
)

func EncryptAES(plainText, key []byte) (string, error) {
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

	cipherText := gcm.Seal(nonce, nonce, plainText, nil)

	return base64.URLEncoding.EncodeToString(cipherText), nil
}

func DecryptAES(cipherBase64 string, key []byte) ([]byte, error) {

	cipherText, err := base64.URLEncoding.DecodeString(cipherBase64)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]

	return gcm.Open(nil, nonce, cipherText, nil)
}
