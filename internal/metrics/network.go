package metrics

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type netRaw struct {
	Interface string
	RXBytes   uint64
	RXPkts    uint64
	TXBytes   uint64
	TXPkts    uint64
}

func parseNetDev(r io.Reader) ([]netRaw, error) {
	scanner := bufio.NewScanner(r)
	var stats []netRaw
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= 2 {
			continue // skip header lines
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])

		// RX bytes = fields[0], RX packets = fields[1]
		// TX bytes = fields[8], TX packets = fields[9]
		if len(fields) < 10 {
			continue
		}

		rxBytes, err1 := strconv.ParseUint(fields[0], 10, 64)
		rxPkts, err2 := strconv.ParseUint(fields[1], 10, 64)
		txBytes, err3 := strconv.ParseUint(fields[8], 10, 64)
		txPkts, err4 := strconv.ParseUint(fields[9], 10, 64)

		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}

		stats = append(stats, netRaw{
			Interface: iface,
			RXBytes:   rxBytes,
			RXPkts:    rxPkts,
			TXBytes:   txBytes,
			TXPkts:    txPkts,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("metrics: failed reading /proc/net/dev: %w", err)
	}

	return stats, nil
}

func computeNetIO(prev, curr []netRaw, interval time.Duration) []NetStat {
	prevMap := make(map[string]netRaw, len(prev))
	for _, p := range prev {
		prevMap[p.Interface] = p
	}

	secs := interval.Seconds()
	if secs <= 0 {
		secs = 1.0
	}

	result := make([]NetStat, 0, len(curr))
	for _, c := range curr {
		var rxBps, txBps float64
		if p, ok := prevMap[c.Interface]; ok {
			if c.RXBytes >= p.RXBytes {
				rxBps = float64(c.RXBytes-p.RXBytes) / secs
			}
			if c.TXBytes >= p.TXBytes {
				txBps = float64(c.TXBytes-p.TXBytes) / secs
			}
		}

		result = append(result, NetStat{
			Interface: c.Interface,
			RXBps:     rxBps,
			TXBps:     txBps,
			RXPkts:    c.RXPkts,
			TXPkts:    c.TXPkts,
		})
	}

	return result
}
