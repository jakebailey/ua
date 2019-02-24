package simplecrypto

import (
	"bytes"
	"crypto/aes"
	"crypto/rand"
	"fmt"
	"testing"
)

func makeKey(size int) []byte {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func TestEncryptDecrypt(t *testing.T) {
	data := []byte("Hello, World!")
	keySizes := []int{128, 192, 256}

	for _, keySize := range keySizes {
		keySize := keySize
		t.Run(fmt.Sprintf("%d-bit", keySize), func(t *testing.T) {
			key := makeKey(keySize / 8)

			ciphertext, err := Encrypt(key, data)
			if err != nil {
				t.Fatalf("expected nil error on Encrypt, got %s", err.Error())
			}

			orig, err := Decrypt(key, ciphertext)
			if err != nil {
				t.Fatalf("expected nil error on Decrypt, got %s", err.Error())
			}

			if !bytes.Equal(data, orig) {
				t.Fatalf("decrypted data differs from original payload")
			}
		})
	}
}

func TestCiphertextTooShort(t *testing.T) {
	key := makeKey(128 / 8)
	var ciphertext []byte

	if _, err := Decrypt(key, ciphertext); err != ErrCiphertextTooShort {
		t.Fatalf("expected ErrCiphertextTooShort, got %s", err.Error())
	}

	ciphertext = make([]byte, aes.BlockSize-1)

	if _, err := Decrypt(key, ciphertext); err != ErrCiphertextTooShort {
		t.Fatalf("expected ErrCiphertextTooShort, got %s", err.Error())
	}
}

func TestInvalidKeySize(t *testing.T) {
	key := makeKey(42)

	if _, err := Encrypt(key, nil); err != aes.KeySizeError(42) {
		t.Fatalf("expected KeySizeError(42) on Encrypt, got %s", err.Error())
	}

	if _, err := Decrypt(key, nil); err != aes.KeySizeError(42) {
		t.Fatalf("expected KeySizeError(42) on Decrypt, got %s", err.Error())
	}
}
