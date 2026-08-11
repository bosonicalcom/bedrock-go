package pagex

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"fmt"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	// todo: remove this and use the keyset from the environment/remote secret store.
	// this is just for testing purposes.
	_pageTokenEncryptionKeyset = `{
				"key": [{
						"keyData": {
								"keyMaterialType":
										"SYMMETRIC",
								"typeUrl":
										"type.googleapis.com/google.crypto.tink.AesGcmKey",
								"value":
										"GiBWyUfGgYk3RTRhj/LIUzSudIWlyjCftCOypTr0jCNSLg=="
						},
						"keyId": 294406504,
						"outputPrefixType": "TINK",
						"status": "ENABLED"
				}],
				"primaryKeyId": 294406504
		}`
)

type tokenOptions struct {
	encryptKeySet string
}

type TokenOpt func(*tokenOptions)

// NewToken encrypts and encodes a page token.
//
// Safe to share to external callers through public network layers.
//
// You may use the WithEncryptKeySet option to specify a custom keyset to use for encryption (BYOK; bring your own key).
// Beware, default encryption keyset is not safe for production use.
func NewToken[T any](token T, opts ...TokenOpt) (string, error) {
	options := &tokenOptions{}
	for _, opt := range opts {
		opt(options)
	}

	fmtToken, err := msgpack.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("bedrock.pagex: cannot marshal page token: %w", err)
	}

	keysetStr := cmp.Or(options.encryptKeySet, _pageTokenEncryptionKeyset)
	keysetHandle, err := insecurecleartextkeyset.Read(
		keyset.NewJSONReader(bytes.NewBufferString(keysetStr)))
	if err != nil {
		return "", fmt.Errorf("bedrock.pagex: cannot read page token keyset: %w", err)
	}

	// Retrieve the AEAD primitive we want to use from the keyset handle.
	primitive, err := aead.New(keysetHandle)
	if err != nil {
		return "", fmt.Errorf("bedrock.pagex: cannot create page token aead primitive: %w", err)
	}

	// Use the primitive to encrypt a message. In this case the primary key of the
	// keyset will be used (which is also the only key in this example).
	// This doesnt need assoc data.
	ciphertext, err := primitive.Encrypt(fmtToken, nil)
	if err != nil {
		return "", fmt.Errorf("bedrock.pagex: cannot encrypt page token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// ParseToken parses a page token of type T from the given encoded token.
func ParseToken[T any](encodedToken string, opts ...TokenOpt) (T, error) {
	var options tokenOptions
	for _, opt := range opts {
		opt(&options)
	}

	ciphertext, err := base64.URLEncoding.DecodeString(encodedToken)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("bedrock.pagex: cannot decode page token: %w", err)
	}

	keysetStr := cmp.Or(options.encryptKeySet, _pageTokenEncryptionKeyset)
	keysetHandle, err := insecurecleartextkeyset.Read(
		keyset.NewJSONReader(bytes.NewBufferString(keysetStr)))
	if err != nil {
		var zero T
		return zero, fmt.Errorf("bedrock.pagex: cannot read page token keyset: %w", err)
	}

	primitive, err := aead.New(keysetHandle)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("bedrock.pagex: cannot create page token aead primitive: %w", err)
	}

	plaintext, err := primitive.Decrypt(ciphertext, nil)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("bedrock.pagex: cannot decrypt page token: %w", err)
	}

	var token T
	if err := msgpack.Unmarshal(plaintext, &token); err != nil {
		return token, fmt.Errorf("bedrock.pagex: cannot unmarshal page token: %w", err)
	}
	return token, nil
}

// - Options -

// WithEncryptKeySet sets the encryption keyset to use for the token.
func WithEncryptKeySet(keyset string) TokenOpt {
	return func(o *tokenOptions) {
		o.encryptKeySet = keyset
	}
}
