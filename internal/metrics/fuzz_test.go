package metrics

import (
	"strings"
	"testing"
)

func FuzzProcStat(f *testing.F) {
	// Seed corpus
	f.Add("1 (init) S 0 1 1 0 -1 4194560 1200 0 0 0 100 200 0 0 20 0 1 0 500 10000 100")
	f.Add("1234 (statix) R 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22")
	f.Add("broken input without parens")
	f.Add("")

	f.Fuzz(func(t *testing.T, data string) {
		_, _, _, _ = parseProcStat(strings.NewReader(data))
	})
}

func FuzzMemInfo(f *testing.F) {
	f.Add("MemTotal:        16328464 kB\nMemFree:          9182348 kB\nMemAvailable:    12847292 kB\nBuffers:           482916 kB\nCached:           3829104 kB\nSwapTotal:        2097148 kB\nSwapFree:         2097148 kB\n")
	f.Add("invalid: text\nMemTotal: notanumber kB\n")
	f.Add("")

	f.Fuzz(func(t *testing.T, data string) {
		_, _ = parseMemInfo(strings.NewReader(data))
	})
}

func FuzzNetDev(f *testing.F) {
	f.Add("Inter-|   Receive                                                |  Transmit\n face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n  eth0: 12345678   10000    0    0    0     0          0         0  87654321    8000    0    0    0     0       0          0\n")
	f.Add("broken header\n")
	f.Add("")

	f.Fuzz(func(t *testing.T, data string) {
		_, _ = parseNetDev(strings.NewReader(data))
	})
}

func FuzzCPUStat(f *testing.F) {
	f.Add("cpu  100 20 30 500 10 0 5 0 0 0\ncpu0 50 10 15 250 5 0 2 0 0 0\ncpu1 50 10 15 250 5 0 3 0 0 0\n")
	f.Add("cpu invalid line\n")
	f.Add("")

	f.Fuzz(func(t *testing.T, data string) {
		_, _ = parseCPUStat(strings.NewReader(data))
	})
}

func FuzzDiskStats(f *testing.F) {
	f.Add("   8       0 sda 100 20 300 40 500 60 700 80 0 90 100\n   8       1 sda1 10 2 30 4 50 6 70 8 0 9 10\n")
	f.Add("corrupt line\n")
	f.Add("")

	f.Fuzz(func(t *testing.T, data string) {
		_, _ = parseDiskStats(strings.NewReader(data))
	})
}
