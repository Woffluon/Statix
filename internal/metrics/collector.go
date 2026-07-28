package metrics

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type CollectorConfig struct {
	Interval     time.Duration
	TopProcesses int
	ProcRoot     string
}

type Collector struct {
	cfg       CollectorConfig
	buf       *RingBuffer
	Snapshots <-chan Snapshot
	snapshots chan Snapshot

	// state across ticks for deltas
	prevCPU      []cpuRaw
	prevDisk     []diskRaw
	prevNet      []netRaw
	prevProc     map[int]procCPURaw
	lastTickTime time.Time
}

func New(cfg CollectorConfig, buf *RingBuffer) *Collector {
	if cfg.Interval <= 0 {
		cfg.Interval = 2 * time.Second
	}
	if cfg.TopProcesses <= 0 {
		cfg.TopProcesses = 15
	}
	if cfg.ProcRoot == "" {
		cfg.ProcRoot = "/proc"
	}

	snapChan := make(chan Snapshot, 1)

	return &Collector{
		cfg:       cfg,
		buf:       buf,
		Snapshots: snapChan,
		snapshots: snapChan,
		prevProc:  make(map[int]procCPURaw),
	}
}

func (c *Collector) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()

	// Initial immediate collection tick
	c.collectTick()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.collectTick()
		}
	}
}

func (c *Collector) collectTick() {
	now := time.Now()
	interval := c.cfg.Interval
	if !c.lastTickTime.IsZero() {
		interval = now.Sub(c.lastTickTime)
	}

	snapshot := Snapshot{
		CollectedAt: now,
	}

	// 1. CPU
	procStatPath := filepath.Join(c.cfg.ProcRoot, "stat")
	if f, err := os.Open(procStatPath); err == nil {
		if currCPU, err := parseCPUStat(f); err == nil {
			if len(c.prevCPU) > 0 {
				snapshot.CPU = computeCPUPercent(c.prevCPU, currCPU)
			}
			c.prevCPU = currCPU
		}
		_ = f.Close()
	}

	// 2. LoadAvg
	loadAvgPath := filepath.Join(c.cfg.ProcRoot, "loadavg")
	if f, err := os.Open(loadAvgPath); err == nil {
		if data, err := io.ReadAll(f); err == nil {
			var l1, l5, l15 float64
			_, _ = fmtSscanf(string(data), &l1, &l5, &l15)
			snapshot.LoadAvg = [3]float64{l1, l5, l15}
		}
		_ = f.Close()
	}

	// 3. Memory
	memInfoPath := filepath.Join(c.cfg.ProcRoot, "meminfo")
	if f, err := os.Open(memInfoPath); err == nil {
		if mem, err := parseMemInfo(f); err == nil {
			snapshot.MemTotal = mem.MemTotal
			snapshot.MemAvail = mem.MemAvailable
			if mem.MemTotal >= mem.MemAvailable {
				snapshot.MemUsed = mem.MemTotal - mem.MemAvailable
			}
			snapshot.MemCached = mem.Cached
			snapshot.SwapTotal = mem.SwapTotal
			if mem.SwapTotal >= mem.SwapFree {
				snapshot.SwapUsed = mem.SwapTotal - mem.SwapFree
			}
		}
		_ = f.Close()
	}

	// 4. Disk
	diskStatsPath := filepath.Join(c.cfg.ProcRoot, "diskstats")
	if f, err := os.Open(diskStatsPath); err == nil {
		if currDisk, err := parseDiskStats(f); err == nil {
			snapshot.Disks = computeDiskIO(c.prevDisk, currDisk, interval, nil)
			c.prevDisk = currDisk
		}
		_ = f.Close()
	}

	// 5. Network
	netDevPath := filepath.Join(c.cfg.ProcRoot, "net/dev")
	if f, err := os.Open(netDevPath); err == nil {
		if currNet, err := parseNetDev(f); err == nil {
			snapshot.Networks = computeNetIO(c.prevNet, currNet, interval)
			c.prevNet = currNet
		}
		_ = f.Close()
	}

	// 6. Processes
	uptimePath := filepath.Join(c.cfg.ProcRoot, "uptime")
	var uptime float64
	if f, err := os.Open(uptimePath); err == nil {
		uptime, _ = parseUptime(f)
		_ = f.Close()
	}
	nowSec := float64(now.UnixNano()) / 1e9
	procs, nextProc, err := collectProcesses(c.cfg.ProcRoot, c.prevProc, uptime, nowSec, c.cfg.TopProcesses)
	if err == nil {
		snapshot.Processes = procs
		c.prevProc = nextProc
	}

	c.lastTickTime = now

	// Save to ring buffer
	if c.buf != nil {
		c.buf.Push(snapshot)
	}

	// Broadcast non-blocking
	select {
	case c.snapshots <- snapshot:
	default:
		// Drain old snapshot if unread and send newest
		select {
		case <-c.snapshots:
		default:
		}
		select {
		case c.snapshots <- snapshot:
		default:
		}
	}
}

func fmtSscanf(str string, a ...any) (n int, err error) {
	var f1, f2, f3 float64
	fields := strings.Fields(str)
	if len(fields) >= 3 {
		f1, _ = strconv.ParseFloat(fields[0], 64)
		f2, _ = strconv.ParseFloat(fields[1], 64)
		f3, _ = strconv.ParseFloat(fields[2], 64)
		if ptr, ok := a[0].(*float64); ok {
			*ptr = f1
		}
		if ptr, ok := a[1].(*float64); ok {
			*ptr = f2
		}
		if ptr, ok := a[2].(*float64); ok {
			*ptr = f3
		}
		return 3, nil
	}
	return 0, io.EOF
}
