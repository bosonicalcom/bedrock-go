package pagex_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"

	"github.com/bosonicalcom/bedrock-go/persistence/pagex"
)

type cursorToken struct {
	ID     string
	Offset int64
}

// newKeySet generates a fresh, independent cleartext Tink keyset JSON string, distinct from
// pagex's built-in default, for tests that need a second/custom keyset.
func newKeySet(t *testing.T) string {
	t.Helper()

	handle, err := keyset.NewHandle(aead.AES256GCMKeyTemplate())
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, insecurecleartextkeyset.Write(handle, keyset.NewJSONWriter(&buf)))
	return buf.String()
}

func TestNewTokenParseToken_RoundTrip(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		token, err := pagex.NewToken[string]("hello-world")
		require.NoError(t, err)

		got, err := pagex.ParseToken[string](token)
		require.NoError(t, err)
		assert.Equal(t, "hello-world", got)
	})

	t.Run("int64", func(t *testing.T) {
		token, err := pagex.NewToken[int64](42)
		require.NoError(t, err)

		got, err := pagex.ParseToken[int64](token)
		require.NoError(t, err)
		assert.Equal(t, int64(42), got)
	})

	t.Run("struct", func(t *testing.T) {
		want := cursorToken{ID: "abc123", Offset: 100}

		token, err := pagex.NewToken[cursorToken](want)
		require.NoError(t, err)

		got, err := pagex.ParseToken[cursorToken](token)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("slice", func(t *testing.T) {
		want := []string{"a", "b", "c"}

		token, err := pagex.NewToken[[]string](want)
		require.NoError(t, err)

		got, err := pagex.ParseToken[[]string](token)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("map", func(t *testing.T) {
		want := map[string]int{"a": 1, "b": 2}

		token, err := pagex.NewToken[map[string]int](want)
		require.NoError(t, err)

		got, err := pagex.ParseToken[map[string]int](token)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}

func TestNewTokenParseToken_RoundTrip_CustomKeySet(t *testing.T) {
	ks := newKeySet(t)
	want := cursorToken{ID: "xyz", Offset: 7}

	token, err := pagex.NewToken[cursorToken](want, pagex.WithEncryptKeySet(ks))
	require.NoError(t, err)

	got, err := pagex.ParseToken[cursorToken](token, pagex.WithEncryptKeySet(ks))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestParseToken_InvalidEncodedToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{name: "not base64", token: "!!!not-base64!!!"},
		{name: "empty string", token: ""},
		{name: "corrupted ciphertext", token: "YWJjZGVmZ2hpams="}, // valid base64, not a valid AEAD ciphertext
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pagex.ParseToken[string](tt.token)
			assert.Error(t, err)
			assert.Empty(t, got)
		})
	}
}

func TestNewTokenParseToken_KeySetMismatch(t *testing.T) {
	customKeySet := newKeySet(t)

	t.Run("encrypted with default, parsed with custom", func(t *testing.T) {
		token, err := pagex.NewToken[string]("hello")
		require.NoError(t, err)

		got, err := pagex.ParseToken[string](token, pagex.WithEncryptKeySet(customKeySet))
		assert.Error(t, err)
		assert.Empty(t, got)
	})

	t.Run("encrypted with custom, parsed with default", func(t *testing.T) {
		token, err := pagex.NewToken[string]("hello", pagex.WithEncryptKeySet(customKeySet))
		require.NoError(t, err)

		got, err := pagex.ParseToken[string](token)
		assert.Error(t, err)
		assert.Empty(t, got)
	})
}

func TestNewTokenParseToken_InvalidKeySetJSON(t *testing.T) {
	const invalidKeySet = "not valid json"

	t.Run("NewToken", func(t *testing.T) {
		token, err := pagex.NewToken[string]("hello", pagex.WithEncryptKeySet(invalidKeySet))
		assert.Error(t, err)
		assert.Empty(t, token)
	})

	t.Run("ParseToken", func(t *testing.T) {
		got, err := pagex.ParseToken[string]("does-not-matter", pagex.WithEncryptKeySet(invalidKeySet))
		assert.Error(t, err)
		assert.Empty(t, got)
	})
}

func TestNewToken_MarshalError(t *testing.T) {
	// channels are not encodable by msgpack, so this exercises the marshal error branch.
	_, err := pagex.NewToken[chan int](make(chan int))
	assert.Error(t, err)
}

func TestParseToken_UnmarshalError(t *testing.T) {
	// a slice payload cannot be unmarshaled into an int64, so this exercises the
	// unmarshal error branch after a successful decrypt.
	token, err := pagex.NewToken[[]string]([]string{"a", "b"})
	require.NoError(t, err)

	got, err := pagex.ParseToken[int64](token)
	assert.Error(t, err)
	assert.Zero(t, got)
}
