package metrics

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type cpuRaw struct {
	Core    int
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	Iowait  uint64
	Irq     uint64
	Softirq uint64
	Steal   uint64
}

func (c cpuRaw) total() uint64 {
	return c.User + c.Nice + c.System + c.Idle + c.Iowait + c.Irq + c.Softirq + c.Steal
}

func (c cpuRaw) idleTotal() uint64 {
	return c.Idle + c.Iowait
}

func parseCPUStat(r io.Reader) ([]cpuRaw, error) {
	scanner := bufio.NewScanner(r)
	var stats []cpuRaw

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "cpu") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		cpuLabel := fields[0]
		var core int
		if cpuLabel == "cpu" {
			core = -1
		} else {
			cNum, err := strconv.Atoi(strings.TrimPrefix(cpuLabel, "cpu"))
			if err != nil {
				continue
			}
			core = cNum
		}

		// parse up to 8 numeric jiffie values
		var vals [8]uint64
		for i := 0; i < 8 && i+1 < len(fields); i++ {
			v, err := strconv.ParseUint(fields[i+1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("metrics: invalid cpu value %s in line %q: %w", fields[i+1], line, err)
			}
			vals[i] = v
		}

		stats = append(stats, cpuRaw{
			Core:    core,
			User:    vals[0],
			Nice:    vals[1],
			System:  vals[2],
			Idle:    vals[3],
			Iowait:  vals[4],
			Irq:     vals[5],
			Softirq: vals[6],
			Steal:   vals[7],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("metrics: failed reading /proc/stat: %w", err)
	}

	return stats, nil
}

func computeCPUPercent(prev, curr []cpuRaw) []CPUStat {
	prevMap := make(map[int]cpuRaw, len(prev))
	for _, p := range prev {
		prevMap[p.Core] = p
	}

	result := make([]CPUStat, 0, len(curr))
	for _, c := range curr {
		p, ok := prevMap[c.Core]
		if !ok {
			result = append(result, CPUStat{Core: c.Core, Percent: 0.0})
			continue
		}

		totalPrev := p.total()
		totalCurr := c.total()
		idlePrev := p.idleTotal()
		idleCurr := c.idleTotal()

		if totalCurr <= totalPrev {
			result = append(result, CPUStat{Core: c.Core, Percent: 0.0})
			continue
		}

		deltaTotal := float64(totalCurr - totalPrev)
		deltaIdle := float64(idleCurr - idlePrev)
		if deltaIdle < 0 {
			deltaIdle = 0
		}

		pct := 100.0 * (1.0 - (deltaIdle / deltaTotal))
		if pct < 0.0 {
			pct = 0.0
		} else if pct > 100.0 {
			pct = 100.0
		}

		result = append(result, CPUStat{
			Core:    c.Core,
			Percent: pct,
		})
	}

	return result
}
