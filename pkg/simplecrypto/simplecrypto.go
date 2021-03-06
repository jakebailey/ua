// Package simplecrypto implements a simplified interface for encryption and
// decryption using AES encryption in CFB mode.
//
// It is implemented using examples given in crypto/cipher and crypto/hmac.
package simplecrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

var (
	// ErrCiphertextTooShort is returned when the provided ciphertext is too
	// short to contain a valid IV.
	ErrCiphertextTooShort = errors.New("simplecrypto: ciphertext too short")

	// ErrHMACDoesNotMatch is returned when the provided HMAC does not match.
	ErrHMACDoesNotMatch = errors.New("simplecrypto: HMAC does not match ciphertext")
)

// Encrypt encrypts a payload using the given key. It returns a byte
// slice with the IV as the first aes.BlockSize bytes, followed by
// the encrypted payload.
//
// Note: Ciphertexts must be authenticated as well as encrypted in order to
// be secure. Be sure to calculate the ciphertext's HMAC to send with it.
// This library provides shorthand for calculating the HMAC of a ciphertext.
func Encrypt(key, payload []byte) (ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext = make([]byte, aes.BlockSize+len(payload))

	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], payload)

	return ciphertext, nil
}

// Decrypt decrypts a ciphertext using the given key. It returns a byte
// slice of the encrypted payload.
//
// This function assumes the ciphertext is in the format generated by the
// Encrypt function, i.e. the IV followed by the encrypted payload.
//
// Note: Ciphertexts must be authenticated as well as encrypted in order to
// be secure. Be sure to check the ciphertext's HMAC before decrypting it.
// This library provides shorthand for checking the HMAC of a ciphertext.
func Decrypt(key, ciphertext []byte) (payload []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, ErrCiphertextTooShort
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}

// HMAC calculates the HMAC of the message using the given key using the
// SHA256 has function.
func HMAC(key, message []byte) []byte {
	mac := hmac.New(sha256.New, key)

	if _, err := mac.Write(message); err != nil {
		// A write to sha256 hasher can never fail.
		panic(err)
	}

	return mac.Sum(nil)
}

// CheckMAC reports whether messageMAC is a valid HMAC tag for message.
//
// This implementation is given in the documentation for crypto/hmac.
func CheckMAC(key, message, messageMAC []byte) bool {
	expectedMAC := HMAC(key, message)
	return hmac.Equal(messageMAC, expectedMAC)
}

// EncryptAndHMAC encrypts the payload using Encrypt, then calculates the
// ciphertext's HMAC.
func EncryptAndHMAC(key, payload []byte) (ciphertext, hmac []byte, err error) {
	ciphertext, err = Encrypt(key, payload)
	if err != nil {
		return nil, nil, err
	}

	return ciphertext, HMAC(key, ciphertext), nil
}

// CheckAndDecrypt checks the HMAC of the chiphertext, and decrypts it if the
// HMAC matches.
func CheckAndDecrypt(key, ciphertext, hmac []byte) (payload []byte, err error) {
	if !CheckMAC(key, ciphertext, hmac) {
		return nil, ErrHMACDoesNotMatch
	}

	return Decrypt(key, ciphertext)
}
