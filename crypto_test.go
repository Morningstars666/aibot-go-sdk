package aibot

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func encryptForTest(t *testing.T, plain []byte, key []byte) []byte {
	t.Helper()
	pad := 32 - len(plain)%32
	padded := append(append([]byte{}, plain...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:16]).CryptBlocks(ct, padded)
	return ct
}

func TestDecryptAESRoundTrip(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	plain := []byte("hello wecom aibot sdk 长连接")
	ct := encryptForTest(t, plain, []byte(key))
	got, err := DecryptAES(ct, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("got %q, want %q", got, plain)
	}
}

func TestDecryptAESBase64Key(t *testing.T) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand: %v", err)
	}
	b64 := base64.StdEncoding.EncodeToString(raw)
	plain := []byte("base64 key payload")
	ct := encryptForTest(t, plain, raw)
	got, err := DecryptAES(ct, b64)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("got %q, want %q", got, plain)
	}
}

func TestDecryptAESExactBlockAligned(t *testing.T) {
	key := "abcdefghijklmnopqrstuvwxyzabcdef"
	plain := bytes.Repeat([]byte("A"), 64)
	ct := encryptForTest(t, plain, []byte(key))
	got, err := DecryptAES(ct, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("got %q, want %q", got, plain)
	}
}

func TestDecryptAESErrors(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	if _, err := DecryptAES(nil, key); err == nil {
		t.Fatal("expected error for empty ciphertext")
	}
	if _, err := DecryptAES([]byte("short"), key); err == nil {
		t.Fatal("expected error for non-block-aligned ciphertext")
	}
	if _, err := DecryptAES(make([]byte, 32), "shortkey"); err == nil {
		t.Fatal("expected error for invalid key")
	}
	if _, err := DecryptAES(make([]byte, 32), "!!!!invalid-base64!!!!"); err == nil {
		t.Fatal("expected error for non-base64 invalid key")
	}
	badPad := make([]byte, 32)
	if _, err := DecryptAES(badPad, key); err == nil {
		t.Fatal("expected error for invalid padding")
	}
	if _, err := DecryptAES(make([]byte, 32), ""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestAESKeyBytes(t *testing.T) {
	raw32 := "0123456789abcdef0123456789abcdef"
	k, err := aesKeyBytes(raw32)
	if err != nil || string(k) != raw32 {
		t.Fatalf("raw key: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(raw32))
	k, err = aesKeyBytes(encoded)
	if err != nil || string(k) != raw32 {
		t.Fatalf("base64 key: %v", err)
	}
	if _, err := aesKeyBytes("tooshort"); err == nil {
		t.Fatal("expected error for short key")
	}
}
