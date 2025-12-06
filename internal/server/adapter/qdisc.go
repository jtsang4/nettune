package adapter

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/jtsang4/nettune/internal/shared/types"
	"go.uber.org/zap"
)

// QdiscManager handles qdisc operations
type QdiscManager struct {
	logger *zap.Logger
}

// NewQdiscManager creates a new QdiscManager
func NewQdiscManager(logger *zap.Logger) *QdiscManager {
	return &QdiscManager{logger: logger}
}

// Get returns qdisc information for an interface
func (m *QdiscManager) Get(iface string) (*types.QdiscInfo, error) {
	cmd := exec.Command("tc", "qdisc", "show", "dev", iface)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get qdisc for %s: %w", iface, err)
	}

	// Parse tc output, e.g., "qdisc fq 8001: root refcnt 2 limit 10000p..."
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("no qdisc output for %s", iface)
	}

	// Parse first line for root qdisc
	for _, line := range lines {
		if strings.Contains(line, "root") {
			return m.parseQdiscLine(line)
		}
	}

	// If no root qdisc, parse first line
	return m.parseQdiscLine(lines[0])
}

// Set sets the root qdisc for an interface
func (m *QdiscManager) Set(iface, qdiscType string, params map[string]interface{}) error {
	// First try to replace existing qdisc
	args := []string{"qdisc", "replace", "dev", iface, "root", qdiscType}

	// Add parameters if any
	for key, value := range params {
		args = append(args, key, fmt.Sprintf("%v", value))
	}

	cmd := exec.Command("tc", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If replace fails, try delete then add
		m.logger.Debug("qdisc replace failed, trying delete+add",
			zap.String("interface", iface),
			zap.Error(err))

		// Delete existing qdisc
		delCmd := exec.Command("tc", "qdisc", "del", "dev", iface, "root")
		delCmd.Run() // Ignore errors as there might not be a root qdisc

		// Add new qdisc
		addArgs := []string{"qdisc", "add", "dev", iface, "root", qdiscType}
		for key, value := range params {
			addArgs = append(addArgs, key, fmt.Sprintf("%v", value))
		}

		addCmd := exec.Command("tc", addArgs...)
		output, err = addCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set qdisc for %s: %w\noutput: %s", iface, err, string(output))
		}
	}

	m.logger.Info("set qdisc successfully",
		zap.String("interface", iface),
		zap.String("type", qdiscType))
	return nil
}

// GetAll returns qdisc information for all interfaces
func (m *QdiscManager) GetAll() (map[string]*types.QdiscInfo, error) {
	ifaces, err := m.ListInterfaces()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*types.QdiscInfo)
	for _, iface := range ifaces {
		info, err := m.Get(iface)
		if err != nil {
			m.logger.Debug("failed to get qdisc for interface",
				zap.String("interface", iface),
				zap.Error(err))
			continue
		}
		result[iface] = info
	}
	return result, nil
}

// GetDefaultRouteInterface returns the interface for the default route
func (m *QdiscManager) GetDefaultRouteInterface() (string, error) {
	// Read /proc/net/route for default route
	file, err := os.Open("/proc/net/route")
	if err != nil {
		// Fallback to ip route command
		return m.getDefaultRouteViaIP()
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Skip header
	scanner.Scan()

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			// Default route has destination "00000000"
			if fields[1] == "00000000" {
				return fields[0], nil
			}
		}
	}

	return "", fmt.Errorf("no default route found")
}

// ListInterfaces returns a list of network interface names
func (m *QdiscManager) ListInterfaces() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	var names []string
	for _, iface := range ifaces {
		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// Skip down interfaces
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		names = append(names, iface.Name)
	}
	return names, nil
}

// parseQdiscLine parses a tc qdisc show line
func (m *QdiscManager) parseQdiscLine(line string) (*types.QdiscInfo, error) {
	// Example: "qdisc fq 8001: root refcnt 2 limit 10000p flow_limit 100p buckets 1024"
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid qdisc line: %s", line)
	}

	if fields[0] != "qdisc" {
		return nil, fmt.Errorf("unexpected qdisc output: %s", line)
	}

	info := &types.QdiscInfo{
		Type:   fields[1],
		Handle: strings.TrimSuffix(fields[2], ":"),
		Params: make(map[string]interface{}),
	}

	// Parse remaining parameters
	for i := 3; i < len(fields)-1; i += 2 {
		key := fields[i]
		if key == "root" || key == "refcnt" {
			continue
		}
		if i+1 < len(fields) {
			info.Params[key] = fields[i+1]
		}
	}

	return info, nil
}

// getDefaultRouteViaIP gets default route interface using ip command
func (m *QdiscManager) getDefaultRouteViaIP() (string, error) {
	cmd := exec.Command("ip", "route", "show", "default")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get default route: %w", err)
	}

	// Parse: "default via 192.168.1.1 dev eth0 ..."
	fields := strings.Fields(string(output))
	for i, field := range fields {
		if field == "dev" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}

	return "", fmt.Errorf("no default route interface found")
}

// GetInterfaceMTU returns the MTU for an interface
func (m *QdiscManager) GetInterfaceMTU(iface string) (int, error) {
	netIface, err := net.InterfaceByName(iface)
	if err != nil {
		return 0, err
	}
	return netIface.MTU, nil
}

// ValidQdiscParams defines valid parameters for each qdisc type
var ValidQdiscParams = map[string][]string{
	"fq": {
		"limit", "flow_limit", "quantum", "initial_quantum",
		"maxrate", "buckets", "pacing", "nopacing", "refill_delay",
		"low_rate_threshold", "orphan_mask", "timer_slack",
		"ce_threshold", "horizon", "horizon_cap", "horizon_drop",
	},
	"fq_codel": {
		"limit", "flows", "target", "interval", "quantum",
		"ecn", "noecn", "ce_threshold", "memory_limit",
	},
	"cake": {
		"bandwidth", "besteffort", "diffserv3", "diffserv4", "diffserv8",
		"flowblind", "srchost", "dsthost", "hosts", "flows",
		"dual-srchost", "dual-dsthost", "nat", "nonat",
		"wash", "nowash", "split-gso", "no-split-gso",
		"ack-filter", "ack-filter-aggressive", "no-ack-filter",
		"memlimit", "fwmark", "atm", "noatm", "ptm", "noptm",
		"overhead", "mpu", "ingress", "egress",
		"rtt", "raw", "conservative",
	},
	"pfifo_fast": {}, // No additional params
}

// ValidateQdiscParams validates qdisc parameters for a given qdisc type
func (m *QdiscManager) ValidateQdiscParams(qdiscType string, params map[string]interface{}) error {
	validParams, ok := ValidQdiscParams[qdiscType]
	if !ok {
		return fmt.Errorf("unknown qdisc type: %s", qdiscType)
	}

	// Create a set for O(1) lookup
	validSet := make(map[string]bool)
	for _, p := range validParams {
		validSet[p] = true
	}

	// Check each provided parameter
	var invalidParams []string
	for key := range params {
		if !validSet[key] {
			invalidParams = append(invalidParams, key)
		}
	}

	if len(invalidParams) > 0 {
		return fmt.Errorf("invalid parameter(s) for qdisc '%s': %v. Valid parameters: %v",
			qdiscType, invalidParams, validParams)
	}

	return nil
}
