package httpext

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ToURL must always hand out a *url.URL owned by the caller: an http.url value
// is reused by every request that references it, so a request clearing the
// user info for digest auth (#6397) must never mutate the shared URL.
func TestToURLReturnsOwnedURLCopy(t *testing.T) {
	t.Parallel()

	shared, err := NewURL("http://user:secret@example.com/path", "name") // trufflehog:ignore
	require.NoError(t, err)

	first, err := ToURL(shared)
	require.NoError(t, err)
	second, err := ToURL(shared)
	require.NoError(t, err)

	// Each conversion returns its own copy, not the shared pointer.
	require.NotSame(t, shared.GetURL(), first.GetURL())
	require.NotSame(t, first.GetURL(), second.GetURL())

	// Mutating one request's URL (as MakeRequest does for digest auth) leaves
	// the other requests and the shared http.url value untouched.
	first.GetURL().User = nil
	require.Nil(t, first.GetURL().User)

	sharedUser, hasUser := second.GetURL().User.Password()
	require.Equal(t, "secret", sharedUser)
	require.True(t, hasUser, "clearing one copy must not leak into the shared URL")
	require.Equal(t, "secret", passwordOf(t, shared))
}

// A URL without credentials belongs to the same equivalence class for the
// ownership contract: copies must still be independent, and stay credential-less.
func TestToURLReturnsOwnedURLCopyWithoutUser(t *testing.T) {
	t.Parallel()

	shared, err := NewURL("http://example.com/path", "name")
	require.NoError(t, err)

	first, err := ToURL(shared)
	require.NoError(t, err)
	second, err := ToURL(shared)
	require.NoError(t, err)

	require.NotSame(t, first.GetURL(), second.GetURL())
	require.Nil(t, first.GetURL().User)
	require.Nil(t, second.GetURL().User)
}

// The string branch must keep the same ownership guarantee: each conversion
// parses a fresh *url.URL, so mutations cannot cross requests either.
func TestToURLStringBranchIsNotShared(t *testing.T) {
	t.Parallel()

	first, err := ToURL("http://user:secret@example.com/") // trufflehog:ignore
	require.NoError(t, err)
	second, err := ToURL("http://user:secret@example.com/") // trufflehog:ignore
	require.NoError(t, err)

	require.NotSame(t, first.GetURL(), second.GetURL())

	first.GetURL().User = nil
	require.Nil(t, first.GetURL().User)
	require.NotNil(t, second.GetURL().User)
}

// A zero-value URL (no parsed *url.URL) must pass through without panicking;
// the guard in ToURL keeps the copy conditional.
func TestToURLZeroValueDoesNotPanic(t *testing.T) {
	t.Parallel()

	result, err := ToURL(URL{})
	require.NoError(t, err)
	require.Nil(t, result.GetURL())
}

// Non-URL, non-string input remains a client error.
func TestToURLInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := ToURL(42)
	require.Error(t, err)
}

// Clean must never leak any part of the credentials, including when the
// userinfo contains percent-encoded characters (a password with a '@', ':',
// space, etc.), where the decoded username/password lengths no longer match the
// bytes in the raw URL.
func TestURLCleanMasksCredentials(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		rawURL   string
		expected string
	}{
		{
			name:     "plain credentials",
			rawURL:   "http://user:pass@example.com/path", // trufflehog:ignore
			expected: "http://****:****@example.com/path",
		},
		{
			name:     "username only",
			rawURL:   "http://user@example.com/path",
			expected: "http://****@example.com/path",
		},
		{
			name:     "encoded char in password",
			rawURL:   "http://user:p%40ss@example.com/path", // trufflehog:ignore
			expected: "http://****:****@example.com/path",
		},
		{
			name:     "space in password",
			rawURL:   "http://user:sp%20ace@example.com/", // trufflehog:ignore
			expected: "http://****:****@example.com/",
		},
		{
			name:     "encoded char in username",
			rawURL:   "http://us%40er:pass@example.com/", // trufflehog:ignore
			expected: "http://****:****@example.com/",
		},
		{
			name:     "no path",
			rawURL:   "https://user:pa%3Ass@example.com", // trufflehog:ignore
			expected: "https://****:****@example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u, err := NewURL(tc.rawURL, "name")
			require.NoError(t, err)
			require.Equal(t, tc.expected, u.Clean())
			require.Equal(t, tc.expected, u.CleanURL)
		})
	}
}

func passwordOf(t *testing.T, u URL) string {
	t.Helper()
	password, ok := u.GetURL().User.Password()
	require.True(t, ok)
	return password
}
