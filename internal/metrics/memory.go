package metrics

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type memInfo struct {
	MemTotal     uint64
	MemFree      uint64
	MemAvailable uint64
	Buffers      uint64
	Cached       uint64
	SwapTotal    uint64
	SwapFree     uint64
}

func parseMemInfo(r io.Reader) (memInfo, error) {
	scanner := bufio.NewScanner(r)
	var info memInfo

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		valFields := strings.Fields(parts[1])
		if len(valFields) < 1 {
			continue
		}

		valKB, err := strconv.ParseUint(valFields[0], 10, 64)
		if err != nil {
			return info, fmt.Errorf("metrics: invalid value for %s in /proc/meminfo: %w", key, err)
		}

		valBytes := valKB * 1024

		switch key {
		case "MemTotal":
			info.MemTotal = valBytes
		case "MemFree":
			info.MemFree = valBytes
		case "MemAvailable":
			info.MemAvailable = valBytes
		case "Buffers":
			info.Buffers = valBytes
		case "Cached":
			info.Cached = valBytes
		case "SwapTotal":
			info.SwapTotal = valBytes
		case "SwapFree":
			info.SwapFree = valBytes
		}
	}

	if err := scanner.Err(); err != nil {
		return info, fmt.Errorf("metrics: error reading /proc/meminfo: %w", err)
	}

	return info, nil
}
