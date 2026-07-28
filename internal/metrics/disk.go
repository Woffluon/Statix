package metrics

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type diskRaw struct {
	Device          string
	ReadsCompleted  uint64
	SectorsRead     uint64
	WritesCompleted uint64
	SectorsWritten  uint64
}

type statfsFunc func(path string) (totalBytes, freeBytes uint64, err error)

func parseDiskStats(r io.Reader) ([]diskRaw, error) {
	scanner := bufio.NewScanner(r)
	var stats []diskRaw

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		dev := fields[2]
		if strings.HasPrefix(dev, "loop") || strings.HasPrefix(dev, "ram") {
			continue
		}

		reads, err1 := strconv.ParseUint(fields[3], 10, 64)
		sRead, err2 := strconv.ParseUint(fields[5], 10, 64)
		writes, err3 := strconv.ParseUint(fields[7], 10, 64)
		sWrite, err4 := strconv.ParseUint(fields[9], 10, 64)

		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}

		stats = append(stats, diskRaw{
			Device:          dev,
			ReadsCompleted:  reads,
			SectorsRead:     sRead,
			WritesCompleted: writes,
			SectorsWritten:  sWrite,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("metrics: failed reading /proc/diskstats: %w", err)
	}

	return stats, nil
}

func computeDiskIO(prev, curr []diskRaw, interval time.Duration, sfn statfsFunc) []DiskStat {
	if sfn == nil {
		sfn = defaultStatfs
	}

	prevMap := make(map[string]diskRaw, len(prev))
	for _, p := range prev {
		prevMap[p.Device] = p
	}

	secs := interval.Seconds()
	if secs <= 0 {
		secs = 1.0
	}

	result := make([]DiskStat, 0, len(curr))
	for _, c := range curr {
		var readBps, writeBps float64
		if p, ok := prevMap[c.Device]; ok {
			if c.SectorsRead >= p.SectorsRead {
				readBps = float64((c.SectorsRead - p.SectorsRead) * 512) / secs
			}
			if c.SectorsWritten >= p.SectorsWritten {
				writeBps = float64((c.SectorsWritten - p.SectorsWritten) * 512) / secs
			}
		}

		var usedPct float64
		if c.Device == "sda" || c.Device == "vda" || c.Device == "nvme0n1" {
			total, free, err := sfn("/")
			if err == nil && total > 0 {
				used := total - free
				usedPct = (float64(used) / float64(total)) * 100.0
			}
		}

		result = append(result, DiskStat{
			Device:   c.Device,
			UsedPct:  usedPct,
			ReadBps:  readBps,
			WriteBps: writeBps,
		})
	}

	return result
}
