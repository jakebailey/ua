package simplecrypto

import (
	"encoding/json"
	"errors"
)

// ErrHMACDoesNotMatch is returned by DecodeJSON when the provided HMAC does
// not match.
var ErrHMACDoesNotMatch = errors.New("simplecrypto: HMAC does not match ciphertext")

// JSONMessage defines a serialization format for a ciphertext and its HMAC.
// The two fields are encoded by encoding/json as base64 strings.
//
// This type is not intended to be used directly, but is exported to show
// the JSON format.
type JSONMessage struct {
	Ciphertext []byte `json:"ciphertext"`
	HMAC       []byte `json:"hmac"`
}

// DecodeJSON decodes a serialized JSON message (data) using a key,
// and returns the decrypted payload. If the decoded cyphertext does not
// match the decoded HMAC, then an error is returned.
func DecodeJSON(key, data []byte) ([]byte, error) {
	var m JSONMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	if !CheckMAC(key, m.Ciphertext, m.HMAC) {
		return nil, ErrHMACDoesNotMatch
	}

	return Decrypt(key, m.Ciphertext)
}

// EncodeJSON encrypts a payload using a key, then encodes it as a JSON
// object, which includes the ciphertext and its HMAC.
func EncodeJSON(key, payload []byte) ([]byte, error) {
	ciphertext, err := Encrypt(key, payload)
	if err != nil {
		return nil, err
	}

	m := JSONMessage{
		Ciphertext: ciphertext,
		HMAC:       HMAC(key, ciphertext),
	}

	return json.Marshal(m)
}
