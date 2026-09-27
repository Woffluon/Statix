package metrics

import "time"

type DiskRaw = diskRaw

type ProcCPURawForTest struct {
	Utime     uint64
	Stime     uint64
	Starttime uint64
	TimeSec   float64
}

func ComputeDiskIOForTest(prev, curr []DiskRaw, interval time.Duration, sfn statfsFunc) []DiskStat {
	return computeDiskIO(prev, curr, interval, sfn)
}

func CollectProcessesForTest(procRoot string, prevCPU map[int]ProcCPURawForTest, totalCPUDelta float64, nowSec float64, topN int) ([]ProcessStat, map[int]ProcCPURawForTest, error) {
	internalPrev := make(map[int]procCPURaw, len(prevCPU))
	for k, v := range prevCPU {
		internalPrev[k] = procCPURaw{
			utime:     v.Utime,
			stime:     v.Stime,
			starttime: v.Starttime,
			timeSec:   v.TimeSec,
		}
	}
	stats, nextCPU, err := collectProcesses(procRoot, internalPrev, totalCPUDelta, nowSec, topN)
	if err != nil {
		return nil, nil, err
	}
	outNext := make(map[int]ProcCPURawForTest, len(nextCPU))
	for k, v := range nextCPU {
		outNext[k] = ProcCPURawForTest{
			Utime:     v.utime,
			Stime:     v.stime,
			Starttime: v.starttime,
			TimeSec:   v.timeSec,
		}
	}
	return stats, outNext, nil
}
