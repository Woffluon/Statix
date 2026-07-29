package metrics

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type procCPURaw struct {
	utime     uint64
	stime     uint64
	starttime uint64
	timeSec   float64
}

func parseProcStat(r io.Reader) (utime, stime, starttime uint64, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, 0, 0, err
	}
	content := string(data)

	// Process name in stat is enclosed in parentheses, e.g. 1234 (nginx) S 1 ...
	lastParen := strings.LastIndex(content, ")")
	if lastParen == -1 || lastParen+2 >= len(content) {
		return 0, 0, 0, fmt.Errorf("invalid stat format")
	}

	afterParen := content[lastParen+2:]
	fields := strings.Fields(afterParen)

	// After ')', fields are:
	// 0: state
	// 1: ppid ...
	// 11: utime (field 14 of full stat)
	// 12: stime (field 15 of full stat)
	// 19: starttime (field 22 of full stat)
	if len(fields) < 20 {
		return 0, 0, 0, fmt.Errorf("stat fields too short")
	}

	utime, err1 := strconv.ParseUint(fields[11], 10, 64)
	stime, err2 := strconv.ParseUint(fields[12], 10, 64)
	starttime, err3 := strconv.ParseUint(fields[19], 10, 64)

	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse stat fields")
	}

	return utime, stime, starttime, nil
}

func parseProcStatus(r io.Reader) (name string, rssKB uint64, state string, err error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "Name":
			name = val
		case "State":
			fields := strings.Fields(val)
			if len(fields) > 0 {
				state = fields[0]
			}
		case "VmRSS":
			fields := strings.Fields(val)
			if len(fields) > 0 {
				v, parseErr := strconv.ParseUint(fields[0], 10, 64)
				if parseErr == nil {
					rssKB = v
				}
			}
		}
	}
	return name, rssKB, state, scanner.Err()
}

func parseUptime(r io.Reader) (float64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty uptime")
	}
	return strconv.ParseFloat(fields[0], 64)
}

func collectProcesses(procRoot string, prevCPU map[int]procCPURaw, _ float64, nowSec float64, topN int) ([]ProcessStat, map[int]procCPURaw, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("metrics: read proc dir %s: %w", procRoot, err)
	}

	var stats []ProcessStat
	nextCPU := make(map[int]procCPURaw)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue // skip non-PID dirs
		}

		pidDir := filepath.Join(procRoot, entry.Name())

		// Read stat
		statFile, err := os.Open(filepath.Join(pidDir, "stat"))
		if err != nil {
			continue
		}
		utime, stime, starttime, err := parseProcStat(statFile)
		_ = statFile.Close()
		if err != nil {
			continue
		}

		// Read status
		statusFile, err := os.Open(filepath.Join(pidDir, "status"))
		var name, state string
		var rssKB uint64
		if err == nil {
			name, rssKB, state, _ = parseProcStatus(statusFile)
			_ = statusFile.Close()
		}
		if name == "" {
			name = entry.Name()
		}

		nextCPU[pid] = procCPURaw{
			utime:     utime,
			stime:     stime,
			starttime: starttime,
			timeSec:   nowSec,
		}

		var cpuPct float64
		if prev, ok := prevCPU[pid]; ok {
			deltaTime := nowSec - prev.timeSec
			if deltaTime > 0 {
				deltaTicks := float64((utime + stime) - (prev.utime + prev.stime))
				// 100.0 clock ticks per second standard
				cpuPct = (deltaTicks / 100.0) / deltaTime * 100.0
			}
		}

		if cpuPct < 0 {
			cpuPct = 0
		}

		stats = append(stats, ProcessStat{
			PID:    pid,
			Name:   name,
			CPUPct: cpuPct,
			RSSKB:  rssKB,
			State:  state,
		})
	}

	// Sort descending by CPUPct, tie-break by RSSKB
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].CPUPct == stats[j].CPUPct {
			return stats[i].RSSKB > stats[j].RSSKB
		}
		return stats[i].CPUPct > stats[j].CPUPct
	})

	if len(stats) > topN {
		stats = stats[:topN]
	}

	return stats, nextCPU, nil
}
