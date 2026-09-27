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

	// Verify defensive copying: mutations to returned slices must not affect internal buffer
	s5 := metrics.Snapshot{
		MemTotal:  500,
		CPU:       []metrics.CPUStat{{Core: 0, Percent: 50.0}},
		Disks:     []metrics.DiskStat{{Device: "sda", UsedPct: 40.0}},
		Networks:  []metrics.NetStat{{Interface: "eth0", RXBps: 1000}},
		Processes: []metrics.ProcessStat{{PID: 1, CPUPct: 10.0}},
	}
	rb.Push(s5)

	latest, ok = rb.Latest()
	require.True(t, ok)
	latest.CPU[0].Percent = 99.9
	latest.Disks[0].UsedPct = 88.8
	latest.Networks[0].RXBps = 9999
	latest.Processes[0].CPUPct = 77.7

	latestAgain, _ := rb.Latest()
	assert.Equal(t, 50.0, latestAgain.CPU[0].Percent)
	assert.Equal(t, 40.0, latestAgain.Disks[0].UsedPct)
	assert.Equal(t, 1000.0, latestAgain.Networks[0].RXBps)
	assert.Equal(t, 10.0, latestAgain.Processes[0].CPUPct)

	allSlice := rb.All()
	allSlice[len(allSlice)-1].CPU[0].Percent = 100.0
	latestAfterAll, _ := rb.Latest()
	assert.Equal(t, 50.0, latestAfterAll.CPU[0].Percent)
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

func TestComputeDiskIOGeneralDevices(t *testing.T) {
	fakeStatfs := func(path string) (uint64, uint64, error) {
		return 1000, 200, nil // 80% used
	}

	// Test with various primary devices: vda, xvda, nvme0n1, mmcblk0, dm-0, sdb
	devices := []string{"vda", "xvda", "nvme0n1", "mmcblk0", "dm-0", "sdb"}
	for _, dev := range devices {
		curr := []metrics.DiskRaw{
			{Device: dev, ReadsCompleted: 10, SectorsRead: 100, WritesCompleted: 5, SectorsWritten: 50},
		}
		stats := metrics.ComputeDiskIOForTest(nil, curr, time.Second, fakeStatfs)
		require.Len(t, stats, 1)
		assert.Equal(t, dev, stats[0].Device)
		assert.InDelta(t, 80.0, stats[0].UsedPct, 0.01, "dev %s should report root used pct", dev)
	}
}

func TestProcessCPUUnderflow(t *testing.T) {
	tempDir := t.TempDir()
	pidDir := filepath.Join(tempDir, "123")
	require.NoError(t, os.MkdirAll(pidDir, 0755))

	// Write stat where utime=10, stime=5 (total 15)
	statContent := "123 (testproc) S 1 1 1 0 -1 0 0 0 0 0 10 5 0 0 20 0 1 0 100 1000 100"
	require.NoError(t, os.WriteFile(filepath.Join(pidDir, "stat"), []byte(statContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pidDir, "status"), []byte("Name:\ttestproc\nVmRSS:\t1024 kB\n"), 0644))

	// Provide prevCPU with HIGHER total: utime=20, stime=20 (total 40)
	// currTotal (15) < prevTotal (40) -> would underflow unsigned uint64 without guard
	prev := map[int]metrics.ProcCPURawForTest{
		123: {Utime: 20, Stime: 20, TimeSec: 100.0},
	}

	stats, _, err := metrics.CollectProcessesForTest(tempDir, prev, 0, 101.0, 10)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 0.0, stats[0].CPUPct, "should be 0, not uint64 overflow")
}
