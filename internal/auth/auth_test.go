package auth_test

import (
	"testing"
	"time"

	"github.com/statix/statix/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordRoundTrip(t *testing.T) {
	password := "SecretPassword123!"

	hash, err := auth.HashPassword(password)
	require.NoError(t, err)
	assert.Contains(t, hash, "$argon2id$")

	valid, err := auth.VerifyPassword(password, hash)
	require.NoError(t, err)
	assert.True(t, valid)

	invalid, err := auth.VerifyPassword("WrongPassword!", hash)
	require.NoError(t, err)
	assert.False(t, invalid)
}

func TestSessionStoreExpiryAndPurge(t *testing.T) {
	ttl := 50 * time.Millisecond
	store := auth.NewSessionStore(ttl)

	sess, err := store.Create("admin")
	require.NoError(t, err)
	assert.NotEmpty(t, sess.ID)

	got, ok := store.Get(sess.ID)
	assert.True(t, ok)
	assert.Equal(t, "admin", got.Username)

	time.Sleep(100 * time.Millisecond)

	_, ok = store.Get(sess.ID)
	assert.False(t, ok)

	// Create another session and test Purge
	sess2, _ := store.Create("admin2")
	time.Sleep(100 * time.Millisecond)
	store.Purge()
	_, ok = store.Get(sess2.ID)
	assert.False(t, ok)
}

func TestRateLimiter(t *testing.T) {
	limit := 5
	window := 1 * time.Second
	lockout := 200 * time.Millisecond

	rl := auth.NewRateLimiter(limit, window, lockout)
	ip := "127.0.0.1"
	user := "admin"

	// 4 failures -> allowed
	for i := 0; i < 4; i++ {
		allowed, _ := rl.Allow(ip, user)
		assert.True(t, allowed)
		rl.Record(ip, user)
	}

	// 5th failure -> locked
	rl.Record(ip, user)
	allowed, retryAfter := rl.Allow(ip, user)
	assert.False(t, allowed)
	assert.Greater(t, retryAfter, time.Duration(0))

	// Wait for lockout TTL
	time.Sleep(250 * time.Millisecond)
	allowed, _ = rl.Allow(ip, user)
	assert.True(t, allowed)
}

func TestCSRFValidation(t *testing.T) {
	token, err := auth.GenerateCSRFToken()
	require.NoError(t, err)
	assert.Len(t, token, 64) // 32 bytes hex = 64 chars

	assert.True(t, auth.ValidateCSRFToken(token, token))
	assert.False(t, auth.ValidateCSRFToken(token, "invalid_token"))
	assert.False(t, auth.ValidateCSRFToken("", token))
	assert.False(t, auth.ValidateCSRFToken(token, ""))
}

func TestVerifyPasswordBoundsDoS(t *testing.T) {
	// Memory exceeds 256MB limit (e.g. 512MB)
	hugeMemHash := "$argon2id$v=19$m=524288,t=3,p=2$c29tZXNhbHQ$c29tZWhhc2g"
	valid, err := auth.VerifyPassword("password", hugeMemHash)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "bounds")

	// Iterations exceeds 10
	hugeIterHash := "$argon2id$v=19$m=65536,t=100,p=2$c29tZXNhbHQ$c29tZWhhc2g"
	valid, err = auth.VerifyPassword("password", hugeIterHash)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "bounds")

	// Parallelism exceeds 16
	hugeParallelHash := "$argon2id$v=19$m=65536,t=3,p=32$c29tZXNhbHQ$c29tZWhhc2g"
	valid, err = auth.VerifyPassword("password", hugeParallelHash)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "bounds")

	// Zero values
	zeroHash := "$argon2id$v=19$m=0,t=0,p=0$c29tZXNhbHQ$c29tZWhhc2g"
	valid, err = auth.VerifyPassword("password", zeroHash)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "bounds")
}

func TestRateLimiterCleanup(t *testing.T) {
	window := 50 * time.Millisecond
	lockout := 50 * time.Millisecond
	rl := auth.NewRateLimiter(2, window, lockout)

	rl.Record("1.1.1.1", "user1")
	rl.Record("2.2.2.2", "user2")
	assert.Equal(t, 2, rl.Size())

	// Wait for window to expire
	time.Sleep(75 * time.Millisecond)
	rl.Cleanup()
	assert.Equal(t, 0, rl.Size())

	// Test locked entry expiration
	rl.Record("3.3.3.3", "user3")
	rl.Record("3.3.3.3", "user3") // reaches limit (2) -> locked
	assert.Equal(t, 1, rl.Size())

	time.Sleep(75 * time.Millisecond)
	rl.Cleanup()
	assert.Equal(t, 0, rl.Size())
}
