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
