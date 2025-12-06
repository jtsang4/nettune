package types

// LatencyStats represents latency statistics
type LatencyStats struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Mean float64 `json:"mean"`
	P50  float64 `json:"p50"`
	P90  float64 `json:"p90"`
	P99  float64 `json:"p99"`
}

// RTTResult represents RTT test results
type RTTResult struct {
	Count      int           `json:"count"`
	Successful int           `json:"successful"`
	Failed     int           `json:"failed"`
	RTT        *LatencyStats `json:"rtt"`    // in milliseconds
	Jitter     float64       `json:"jitter"` // in milliseconds
	Errors     []string      `json:"errors,omitempty"`
}

// ThroughputResult represents throughput test results
type ThroughputResult struct {
	Direction      string    `json:"direction"` // "download" | "upload"
	Bytes          int64     `json:"bytes"`
	DurationMs     int64     `json:"duration_ms"`
	ThroughputMbps float64   `json:"throughput_mbps"`
	Parallel       int       `json:"parallel"`
	Iterations     int       `json:"iterations,omitempty"`  // number of test iterations
	AllResults     []float64 `json:"all_results,omitempty"` // throughput of each iteration (Mbps)
	StdDev         float64   `json:"std_dev,omitempty"`     // standard deviation (Mbps)
	Errors         []string  `json:"errors,omitempty"`
}

// LatencyUnderLoadResult represents latency under load test results
type LatencyUnderLoadResult struct {
	Baseline       *LatencyStats `json:"baseline"`      // latency without load
	UnderLoad      *LatencyStats `json:"under_load"`    // latency under load
	InflationP50   float64       `json:"inflation_p50"` // p50 inflation factor
	InflationP99   float64       `json:"inflation_p99"` // p99 inflation factor
	LoadDurationMs int64         `json:"load_duration_ms"`
	LoadMbps       float64       `json:"load_mbps"`
}

// EchoResponse represents the /probe/echo response
type EchoResponse struct {
	Ts int64 `json:"ts"`
	Ok bool  `json:"ok"`
}

// UploadResponse represents the /probe/upload response
type UploadResponse struct {
	ReceivedBytes int64 `json:"received_bytes"`
	DurationMs    int64 `json:"duration_ms"`
}

// ServerInfo represents server information from /probe/info
type ServerInfo struct {
	Hostname          string            `json:"hostname"`
	KernelVersion     string            `json:"kernel_version"`
	Distribution      string            `json:"distribution"`
	CongestionControl string            `json:"congestion_control"`
	DefaultQdisc      string            `json:"default_qdisc"`
	DefaultInterface  string            `json:"default_interface"`
	InterfaceMTU      int               `json:"interface_mtu"`
	InterfaceSpeed    string            `json:"interface_speed,omitempty"`
	InterfaceStats    *InterfaceStats   `json:"interface_stats,omitempty"`
	AvailableCCs      []string          `json:"available_ccs"`
	Dependencies      map[string]string `json:"dependencies"` // dependency name -> status
}

// InterfaceStats represents network interface statistics
type InterfaceStats struct {
	RxPackets int64 `json:"rx_packets"`
	TxPackets int64 `json:"tx_packets"`
	RxDropped int64 `json:"rx_dropped"`
	TxDropped int64 `json:"tx_dropped"`
	RxErrors  int64 `json:"rx_errors"`
	TxErrors  int64 `json:"tx_errors"`
}
