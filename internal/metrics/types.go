package metrics

import "time"

type CPUStat struct {
	Core    int     `json:"core"`    // -1 = total, 0..N = per-core
	Percent float64 `json:"percent"` // 0–100
}

type DiskStat struct {
	Device   string  `json:"device"`
	UsedPct  float64 `json:"used_pct"`
	ReadBps  float64 `json:"read_bps"`
	WriteBps float64 `json:"write_bps"`
}

type NetStat struct {
	Interface string  `json:"interface"`
	RXBps     float64 `json:"rx_bps"`
	TXBps     float64 `json:"tx_bps"`
	RXPkts    uint64  `json:"rx_pkts"`
	TXPkts    uint64  `json:"tx_pkts"`
}

type ProcessStat struct {
	PID    int     `json:"pid"`
	Name   string  `json:"name"`
	CPUPct float64 `json:"cpu_pct"`
	RSSKB  uint64  `json:"rss_kb"`
	State  string  `json:"state"`
}

type Snapshot struct {
	CollectedAt time.Time     `json:"collected_at"`
	CPU         []CPUStat     `json:"cpu"`
	LoadAvg     [3]float64    `json:"load_avg"`
	MemTotal    uint64        `json:"mem_total"`
	MemUsed     uint64        `json:"mem_used"`
	MemAvail    uint64        `json:"mem_avail"`
	MemCached   uint64        `json:"mem_cached"`
	SwapTotal   uint64        `json:"swap_total"`
	SwapUsed    uint64        `json:"swap_used"`
	Disks       []DiskStat    `json:"disks"`
	Networks    []NetStat     `json:"networks"`
	Processes   []ProcessStat `json:"processes"`
}

// Clone returns a deep defensive copy of Snapshot and its nested slices.
func (s Snapshot) Clone() Snapshot {
	out := s
	if s.CPU != nil {
		out.CPU = make([]CPUStat, len(s.CPU))
		copy(out.CPU, s.CPU)
	}
	if s.Disks != nil {
		out.Disks = make([]DiskStat, len(s.Disks))
		copy(out.Disks, s.Disks)
	}
	if s.Networks != nil {
		out.Networks = make([]NetStat, len(s.Networks))
		copy(out.Networks, s.Networks)
	}
	if s.Processes != nil {
		out.Processes = make([]ProcessStat, len(s.Processes))
		copy(out.Processes, s.Processes)
	}
	return out
}
