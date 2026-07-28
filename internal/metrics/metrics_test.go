package metrics_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/statix/statix/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRingBufferOperations(t *testing.T) {
	rb := metrics.NewRingBuffer(3)
	assert.Equal(t, 3, rb.Capacity())
	assert.Equal(t, 0, rb.Size())

	_, ok := rb.Latest()
	assert.False(t, ok)
	assert.Nil(t, rb.All())

	s1 := metrics.Snapshot{MemTotal: 100}
	s2 := metrics.Snapshot{MemTotal: 200}
	s3 := metrics.Snapshot{MemTotal: 300}
	s4 := metrics.Snapshot{MemTotal: 400}

	rb.Push(s1)
	assert.Equal(t, 1, rb.Size())
	latest, ok := rb.Latest()
	assert.True(t, ok)
	assert.Equal(t, uint64(100), latest.MemTotal)

	rb.Push(s2)
	rb.Push(s3)
	assert.Equal(t, 3, rb.Size())

	all := rb.All()
	require.Len(t, all, 3)
	assert.Equal(t, uint64(100), all[0].MemTotal)
	assert.Equal(t, uint64(200), all[1].MemTotal)
	assert.Equal(t, uint64(300), all[2].MemTotal)

	// Overwrite oldest (s1) with s4
	rb.Push(s4)
	assert.Equal(t, 3, rb.Size())
	latest, ok = rb.Latest()
	assert.True(t, ok)
	assert.Equal(t, uint64(400), latest.MemTotal)

	allOverwritten := rb.All()
	require.Len(t, allOverwritten, 3)
	assert.Equal(t, uint64(200), allOverwritten[0].MemTotal)
	assert.Equal(t, uint64(300), allOverwritten[1].MemTotal)
	assert.Equal(t, uint64(400), allOverwritten[2].MemTotal)
}

func TestCollectorRunCancelClean(t *testing.T) {
	tempDir := t.TempDir()
	procDir := filepath.Join(tempDir, "proc")
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net"), 0755))

	// Write mock proc files
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte("cpu 100 0 100 1000 0 0 0 0\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "meminfo"), []byte("MemTotal: 1000 kB\nMemAvailable: 500 kB\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "loadavg"), []byte("0.10 0.20 0.30 1/100 1234\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "diskstats"), []byte("8 0 sda 10 0 100 0 10 0 100 0 0 0 0\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/dev"), []byte("Inter-| Receive | Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n eth0: 100 1 0 0 0 0 0 0 200 2 0 0 0 0 0 0\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "uptime"), []byte("1000.0 900.0\n"), 0644))

	buf := metrics.NewRingBuffer(10)
	col := metrics.New(metrics.CollectorConfig{
		Interval:     50 * time.Millisecond,
		TopProcesses: 5,
		ProcRoot:     procDir,
	}, buf)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- col.Run(ctx)
	}()

	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine leak: Run did not return after context cancel")
	}

	assert.GreaterOrEqual(t, buf.Size(), 1)
}

func BenchmarkRingBufferPush(b *testing.B) {
	rb := metrics.NewRingBuffer(10800)
	s := metrics.Snapshot{
		CPU:      make([]metrics.CPUStat, 8),
		LoadAvg:  [3]float64{1.0, 2.0, 3.0},
		MemTotal: 16 * 1024 * 1024 * 1024,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rb.Push(s)
	}
}

func BenchmarkCollectSnapshot(b *testing.B) {
	tempDir := b.TempDir()
	procDir := filepath.Join(tempDir, "proc")
	_ = os.MkdirAll(filepath.Join(procDir, "net"), 0755)

	_ = os.WriteFile(filepath.Join(procDir, "stat"), []byte("cpu 100 0 100 1000 0 0 0 0\n"), 0644)
	_ = os.WriteFile(filepath.Join(procDir, "meminfo"), []byte("MemTotal: 1000 kB\nMemAvailable: 500 kB\n"), 0644)
	_ = os.WriteFile(filepath.Join(procDir, "loadavg"), []byte("0.10 0.20 0.30 1/100 1234\n"), 0644)
	_ = os.WriteFile(filepath.Join(procDir, "diskstats"), []byte("8 0 sda 10 0 100 0 10 0 100 0 0 0 0\n"), 0644)
	_ = os.WriteFile(filepath.Join(procDir, "net/dev"), []byte("Inter-| Receive | Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n eth0: 100 1 0 0 0 0 0 0 200 2 0 0 0 0 0 0\n"), 0644)
	_ = os.WriteFile(filepath.Join(procDir, "uptime"), []byte("1000.0 900.0\n"), 0644)

	buf := metrics.NewRingBuffer(10)
	col := metrics.New(metrics.CollectorConfig{
		Interval: 1 * time.Second,
		ProcRoot: procDir,
	}, buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = col
	}
}
