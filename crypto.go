package aibot

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

func DecryptAES(ciphertext []byte, key string) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, ErrDecrypt
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("%w: ciphertext length %d is not a multiple of %d", ErrDecrypt, len(ciphertext), aes.BlockSize)
	}
	keyBytes, err := aesKeyBytes(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, keyBytes[:aes.BlockSize]).CryptBlocks(plain, ciphertext)
	return pkcs7Unpad32(plain)
}

func aesKeyBytes(key string) ([]byte, error) {
	if key == "" {
		return nil, ErrInvalidKey
	}
	raw := []byte(key)
	if len(raw) == 32 {
		return raw, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("%w: key length is %d, not 32, and not valid base64", ErrInvalidKey, len(raw))
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("%w: decoded key length is %d, want 32", ErrInvalidKey, len(decoded))
	}
	return decoded, nil
}

func pkcs7Unpad32(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidPadding
	}
	pad := int(data[len(data)-1])
	if pad < 1 || pad > 32 || pad > len(data) {
		return nil, fmt.Errorf("%w: pad byte %d out of range", ErrInvalidPadding, pad)
	}
	if !bytes.Equal(data[len(data)-pad:], bytes.Repeat([]byte{byte(pad)}, pad)) {
		return nil, ErrInvalidPadding
	}
	return data[:len(data)-pad], nil
}
