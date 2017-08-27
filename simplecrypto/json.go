package simplecrypto

import (
	"encoding/json"
	"io"
)

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

	return CheckAndDecrypt(key, m.Ciphertext, m.HMAC)
}

// DecodeJSONReader performs the same task as DecodeJSON, but reads from
// a Reader.
func DecodeJSONReader(key []byte, r io.Reader) ([]byte, error) {
	var m JSONMessage
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		return nil, err
	}

	return CheckAndDecrypt(key, m.Ciphertext, m.HMAC)
}

// EncodeJSON encrypts a payload using a key, then encodes it as a JSON
// object, which includes the ciphertext and its HMAC.
func EncodeJSON(key, payload []byte) ([]byte, error) {
	ciphertext, hmac, err := EncryptAndHMAC(key, payload)
	if err != nil {
		return nil, err
	}

	m := JSONMessage{
		Ciphertext: ciphertext,
		HMAC:       hmac,
	}

	return json.Marshal(m)
}

// EncodeJSONWriter performs the same task as EncodeJSON, but writes to
// a Writer.
func EncodeJSONWriter(key, payload []byte, w io.Writer) error {
	ciphertext, hmac, err := EncryptAndHMAC(key, payload)
	if err != nil {
		return err
	}

	m := JSONMessage{
		Ciphertext: ciphertext,
		HMAC:       hmac,
	}

	return json.NewEncoder(w).Encode(m)
}
