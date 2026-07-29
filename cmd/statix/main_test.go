package main

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindListenerSuccess(t *testing.T) {
	// Pick a free port dynamically first
	dummyLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	dummyAddr := dummyLn.Addr().String()
	require.NoError(t, dummyLn.Close())

	ln, boundAddr, fallbackUsed, err := bindListener(dummyAddr, false, 5)
	require.NoError(t, err)
	defer ln.Close()

	assert.Equal(t, dummyAddr, boundAddr)
	assert.False(t, fallbackUsed)
}

func TestBindListenerFallbackWhenBusy(t *testing.T) {
	// Bind to a free port to occupy it
	occupantLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer occupantLn.Close()

	occupiedAddr := occupantLn.Addr().String()
	host, portStr, err := net.SplitHostPort(occupiedAddr)
	require.NoError(t, err)

	// Try binding to occupied address with fallback enabled
	ln, boundAddr, fallbackUsed, err := bindListener(occupiedAddr, true, 5)
	require.NoError(t, err)
	defer ln.Close()

	assert.True(t, fallbackUsed)
	assert.NotEqual(t, occupiedAddr, boundAddr)

	_, boundPortStr, err := net.SplitHostPort(boundAddr)
	require.NoError(t, err)
	assert.NotEqual(t, portStr, boundPortStr)
	assert.Equal(t, host, "127.0.0.1")
}

func TestBindListenerNoFallbackWhenExplicit(t *testing.T) {
	// Bind to a free port to occupy it
	occupantLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer occupantLn.Close()

	occupiedAddr := occupantLn.Addr().String()

	// Try binding with allowFallback = false
	ln, _, _, err := bindListener(occupiedAddr, false, 1)
	assert.Error(t, err)
	if ln != nil {
		_ = ln.Close()
	}
}
