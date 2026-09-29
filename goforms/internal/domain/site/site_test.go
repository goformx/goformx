package site

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOrigin(t *testing.T) {
	for _, test := range []struct{ input, expected string }{
		{"https://Example.COM:443", "https://example.com"},
		{"https://Example.COM:0443", "https://example.com"},
		{"https://Example.COM:008443", "https://example.com:8443"},
		{"https://Example.COM.:8443", "https://example.com:8443"},
		{"http://LOCALHOST:80", "http://localhost"},
		{"http://127.0.0.1:3000", "http://127.0.0.1:3000"},
	} {
		actual, err := NormalizeOrigin(test.input)
		require.NoError(t, err, test.input)
		require.Equal(t, test.expected, actual)
	}
	for _, value := range []string{
		"http://example.com", "https://example.com/path", "https://user@example.com",
		"https://example.com?x=1", "https://example.com#fragment", "https://example.com/",
		"https://example.com:99999", "https://example.com ", "ftp://example.com",
	} {
		_, err := NormalizeOrigin(value)
		require.ErrorIs(t, err, ErrInvalid, value)
	}
}
