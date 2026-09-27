package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func BenchmarkRingBufferPush(b *testing.B) {
	rb := NewRingBuffer(10800)
	s := Snapshot{
		CPU:      make([]CPUStat, 8),
		LoadAvg:  [3]float64{1.0, 2.0, 3.0},
		MemTotal: 16 * 1024 * 1024 * 1024,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rb.Push(s)
	}
}

func BenchmarkRingBufferLatest(b *testing.B) {
	rb := NewRingBuffer(10800)
	s := Snapshot{
		CPU:       make([]CPUStat, 8),
		Disks:     make([]DiskStat, 2),
		Networks:  make([]NetStat, 2),
		Processes: make([]ProcessStat, 10),
		LoadAvg:   [3]float64{1.0, 2.0, 3.0},
		MemTotal:  16 * 1024 * 1024 * 1024,
	}
	rb.Push(s)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = rb.Latest()
	}
}

func BenchmarkRingBufferAll(b *testing.B) {
	rb := NewRingBuffer(100)
	s := Snapshot{
		CPU:       make([]CPUStat, 4),
		Disks:     make([]DiskStat, 1),
		Networks:  make([]NetStat, 1),
		Processes: make([]ProcessStat, 5),
		LoadAvg:   [3]float64{1.0, 2.0, 3.0},
		MemTotal:  16 * 1024 * 1024 * 1024,
	}
	for i := 0; i < 100; i++ {
		rb.Push(s)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rb.All()
	}
}

func BenchmarkParseCPUStat(b *testing.B) {
	content := "cpu  100 20 30 500 10 0 5 0 0 0\ncpu0 50 10 15 250 5 0 2 0 0 0\ncpu1 50 10 15 250 5 0 3 0 0 0\n"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = parseCPUStat(strings.NewReader(content))
	}
}

func BenchmarkParseMemInfo(b *testing.B) {
	content := "MemTotal:        16328464 kB\nMemFree:          9182348 kB\nMemAvailable:    12847292 kB\nBuffers:           482916 kB\nCached:           3829104 kB\nSwapTotal:        2097148 kB\nSwapFree:         2097148 kB\n"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = parseMemInfo(strings.NewReader(content))
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

	buf := NewRingBuffer(10)
	col := New(CollectorConfig{
		Interval: 1 * time.Second,
		ProcRoot: procDir,
	}, buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = col
	}
}
