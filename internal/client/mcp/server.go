package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jtsang4/nettune/internal/client/http"
	"github.com/jtsang4/nettune/internal/client/probe"
	"github.com/jtsang4/nettune/internal/shared/types"
	"github.com/jtsang4/nettune/pkg/version"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

// Server is the MCP stdio server using mcp-go SDK
type Server struct {
	mcpServer  *server.MCPServer
	client     *http.Client
	logger     *zap.Logger
	rttTester  *probe.RTTTester
	tpTester   *probe.ThroughputTester
	loadTester *probe.LatencyLoadTester
}

// NewServer creates a new MCP server using the official mcp-go SDK
func NewServer(serverURL, apiKey string, timeout time.Duration, logger *zap.Logger) *Server {
	client := http.NewClient(serverURL, apiKey, timeout)

	s := &Server{
		client:     client,
		logger:     logger,
		rttTester:  probe.NewRTTTester(client),
		tpTester:   probe.NewThroughputTester(client),
		loadTester: probe.NewLatencyLoadTester(client),
	}

	// Create MCP server with mcp-go SDK
	s.mcpServer = server.NewMCPServer(
		"nettune",
		version.Version,
		server.WithToolCapabilities(true),
	)

	// Register all tools
	s.registerTools()

	return s
}

// Start starts the MCP stdio server
func (s *Server) Start() error {
	s.logger.Info("starting MCP server with mcp-go SDK")
	return server.ServeStdio(s.mcpServer)
}

// registerTools registers all nettune tools with the MCP server
func (s *Server) registerTools() {
	// Tool: nettune.test_rtt
	s.mcpServer.AddTool(
		mcp.NewTool("nettune.test_rtt",
			mcp.WithDescription("Measure RTT (Round-Trip Time) to the server. Returns p50/p90/p99 latencies, jitter, and error rate."),
			mcp.WithNumber("count",
				mcp.Description("Number of echo requests to send (default: 30)"),
			),
			mcp.WithNumber("concurrency",
				mcp.Description("Number of concurrent requests (default: 1)"),
			),
		),
		s.handleTestRTT,
	)

	// Tool: nettune.test_throughput
	s.mcpServer.AddTool(
		mcp.NewTool("nettune.test_throughput",
			mcp.WithDescription("Measure network throughput (upload or download bandwidth) to the server. Use 'iterations' > 1 for more reliable results with statistical analysis."),
			mcp.WithString("direction",
				mcp.Required(),
				mcp.Description("Test direction: 'download' or 'upload'"),
				mcp.Enum("download", "upload"),
			),
			mcp.WithNumber("bytes",
				mcp.Description("Number of bytes to transfer per iteration (default: 100MB, use 500MB+ for more accurate results)"),
			),
			mcp.WithNumber("parallel",
				mcp.Description("Number of parallel connections (default: 1, use 4-8 for saturating high-bandwidth links)"),
			),
			mcp.WithNumber("iterations",
				mcp.Description("Number of test iterations to run and average (default: 1, use 3-5 for reliable results)"),
			),
		),
		s.handleTestThroughput,
	)

	// Tool: nettune.test_latency_under_load
	s.mcpServer.AddTool(
		mcp.NewTool("nettune.test_latency_under_load",
			mcp.WithDescription("Measure latency while under network load to detect bufferbloat. Compares baseline RTT vs RTT during load."),
			mcp.WithNumber("duration",
				mcp.Description("Load duration in seconds (default: 10)"),
			),
			mcp.WithNumber("load_parallel",
				mcp.Description("Number of parallel connections for load generation (default: 4)"),
			),
			mcp.WithNumber("echo_interval",
				mcp.Description("Echo probe interval in milliseconds (default: 100)"),
			),
		),
		s.handleTestLatencyUnderLoad,
	)

	// Tool: nettune.snapshot_server
	s.mcpServer.AddTool(
		mcp.NewTool("nettune.snapshot_server",
			mcp.WithDescription("Create a snapshot of the current server configuration for potential rollback."),
		),
		s.handleSnapshotServer,
	)

	// Tool: nettune.list_profiles
	s.mcpServer.AddTool(
		mcp.NewTool("nettune.list_profiles",
			mcp.WithDescription("List all available configuration profiles that can be applied to optimize network settings."),
		),
		s.handleListProfiles,
	)

	// Tool: nettune.show_profile
	s.mcpServer.AddTool(
		mcp.NewTool("nettune.show_profile",
			mcp.WithDescription("Show detailed information about a specific configuration profile."),
			mcp.WithString("profile_id",
				mcp.Required(),
				mcp.Description("The ID of the profile to show"),
			),
		),
		s.handleShowProfile,
	)

	// Tool: nettune.apply_profile
	s.mcpServer.AddTool(
		mcp.NewTool("nettune.apply_profile",
			mcp.WithDescription("Apply a configuration profile to the server. Use 'dry_run' mode first to preview changes."),
			mcp.WithString("profile_id",
				mcp.Required(),
				mcp.Description("The ID of the profile to apply"),
			),
			mcp.WithString("mode",
				mcp.Required(),
				mcp.Description("Mode: 'dry_run' to preview changes, 'commit' to apply"),
				mcp.Enum("dry_run", "commit"),
			),
			mcp.WithNumber("auto_rollback_seconds",
				mcp.Description("Seconds to wait before auto-rollback if verification fails (default: 60, 0 to disable)"),
			),
		),
		s.handleApplyProfile,
	)

	// Tool: nettune.rollback
	s.mcpServer.AddTool(
		mcp.NewTool("nettune.rollback",
			mcp.WithDescription("Rollback to a previous configuration snapshot."),
			mcp.WithString("snapshot_id",
				mcp.Description("The ID of the snapshot to rollback to"),
			),
			mcp.WithBoolean("rollback_last",
				mcp.Description("If true, rollback to the most recent snapshot"),
			),
		),
		s.handleRollback,
	)

	// Tool: nettune.status
	s.mcpServer.AddTool(
		mcp.NewTool("nettune.status",
			mcp.WithDescription("Get current server status including configuration state, last apply info, and server information."),
		),
		s.handleStatus,
	)

	// Tool: nettune.create_profile
	s.mcpServer.AddTool(
		mcp.NewTool("nettune.create_profile",
			mcp.WithDescription("Create a new configuration profile for network optimization. The profile can then be applied using nettune.apply_profile."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Unique profile ID (alphanumeric with hyphens, e.g., 'my-custom-profile')"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Human-readable profile name (e.g., 'Low Latency Gaming Profile')"),
			),
			mcp.WithString("description",
				mcp.Description("Detailed description of what this profile does and when to use it"),
			),
			mcp.WithString("risk_level",
				mcp.Required(),
				mcp.Description("Risk level of the profile"),
				mcp.Enum("low", "medium", "high"),
			),
			mcp.WithBoolean("requires_reboot",
				mcp.Description("Whether applying this profile requires a system reboot (default: false)"),
			),
			mcp.WithObject("sysctl",
				mcp.Description("Sysctl parameters to set. Keys are sysctl paths (e.g., 'net.core.rmem_max'), values are the desired settings."),
			),
			mcp.WithString("qdisc_type",
				mcp.Description("Queue discipline type for traffic control"),
				mcp.Enum("fq", "fq_codel", "cake", "pfifo_fast"),
			),
			mcp.WithString("qdisc_interfaces",
				mcp.Description("Which interfaces to apply qdisc to"),
				mcp.Enum("default-route", "all"),
			),
			mcp.WithObject("qdisc_params",
				mcp.Description("Additional qdisc parameters (type-specific, e.g., {'flow_limit': '10000'} for fq)"),
			),
			mcp.WithBoolean("systemd_ensure_qdisc_service",
				mcp.Description("Whether to create a systemd service to persist qdisc settings across reboots (default: false)"),
			),
		),
		s.handleCreateProfile,
	)
}

// Tool handlers

func (s *Server) handleTestRTT(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(request.Params.Arguments)
	count := getIntArg(args, "count", 30)
	concurrency := getIntArg(args, "concurrency", 1)

	result, err := s.rttTester.TestRTT(count, concurrency)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	return mcp.NewToolResultText(toJSON(result)), nil
}

func (s *Server) handleTestThroughput(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(request.Params.Arguments)
	direction := getStringArg(args, "direction", "download")
	bytes := getInt64Arg(args, "bytes", 100*1024*1024)
	parallel := getIntArg(args, "parallel", 1)
	iterations := getIntArg(args, "iterations", 1)

	var result *types.ThroughputResult
	var err error

	if direction == "download" {
		result, err = s.tpTester.TestDownloadWithIterations(bytes, parallel, iterations)
	} else {
		result, err = s.tpTester.TestUploadWithIterations(bytes, parallel, iterations)
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	return mcp.NewToolResultText(toJSON(result)), nil
}

func (s *Server) handleTestLatencyUnderLoad(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(request.Params.Arguments)
	duration := getIntArg(args, "duration", 10)
	loadParallel := getIntArg(args, "load_parallel", 4)
	echoInterval := getIntArg(args, "echo_interval", 100)

	result, err := s.loadTester.TestLatencyUnderLoad(duration, loadParallel, echoInterval)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	return mcp.NewToolResultText(toJSON(result)), nil
}

func (s *Server) handleSnapshotServer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	snapshot, err := s.client.CreateSnapshot()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	return mcp.NewToolResultText(toJSON(map[string]interface{}{
		"snapshot_id":   snapshot.ID,
		"current_state": snapshot.State,
	})), nil
}

func (s *Server) handleListProfiles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profiles, err := s.client.ListProfiles()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	return mcp.NewToolResultText(toJSON(map[string]interface{}{
		"profiles": profiles,
	})), nil
}

func (s *Server) handleShowProfile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(request.Params.Arguments)
	profileID := getStringArg(args, "profile_id", "")
	if profileID == "" {
		return mcp.NewToolResultError("Error: profile_id is required"), nil
	}

	profile, err := s.client.GetProfile(profileID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	return mcp.NewToolResultText(toJSON(profile)), nil
}

func (s *Server) handleApplyProfile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(request.Params.Arguments)
	profileID := getStringArg(args, "profile_id", "")
	mode := getStringArg(args, "mode", "dry_run")
	autoRollback := getIntArg(args, "auto_rollback_seconds", 60)

	if profileID == "" {
		return mcp.NewToolResultError("Error: profile_id is required"), nil
	}

	req := &types.ApplyRequest{
		ProfileID:           profileID,
		Mode:                mode,
		AutoRollbackSeconds: autoRollback,
	}

	result, err := s.client.Apply(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	return mcp.NewToolResultText(toJSON(result)), nil
}

func (s *Server) handleRollback(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(request.Params.Arguments)
	snapshotID := getStringArg(args, "snapshot_id", "")
	rollbackLast := getBoolArg(args, "rollback_last", false)

	if snapshotID == "" && !rollbackLast {
		return mcp.NewToolResultError("Error: either snapshot_id or rollback_last is required"), nil
	}

	req := &types.RollbackRequest{
		SnapshotID:   snapshotID,
		RollbackLast: rollbackLast,
	}

	result, err := s.client.Rollback(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	return mcp.NewToolResultText(toJSON(result)), nil
}

func (s *Server) handleStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status, err := s.client.GetStatus()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error getting status: %v", err)), nil
	}

	serverInfo, err := s.client.ProbeInfo()
	if err != nil {
		// Server info is optional, continue with status only
		return mcp.NewToolResultText(toJSON(map[string]interface{}{
			"status": status,
		})), nil
	}

	return mcp.NewToolResultText(toJSON(map[string]interface{}{
		"status":      status,
		"server_info": serverInfo,
	})), nil
}

func (s *Server) handleCreateProfile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(request.Params.Arguments)

	// Required fields
	id := getStringArg(args, "id", "")
	name := getStringArg(args, "name", "")
	riskLevel := getStringArg(args, "risk_level", "")

	if id == "" {
		return mcp.NewToolResultError("Error: id is required"), nil
	}
	if name == "" {
		return mcp.NewToolResultError("Error: name is required"), nil
	}
	if riskLevel == "" {
		return mcp.NewToolResultError("Error: risk_level is required"), nil
	}

	// Optional fields
	description := getStringArg(args, "description", "")
	requiresReboot := getBoolArg(args, "requires_reboot", false)

	// Build profile
	profile := &types.Profile{
		ID:             id,
		Name:           name,
		Description:    description,
		RiskLevel:      riskLevel,
		RequiresReboot: requiresReboot,
	}

	// Parse sysctl (map[string]interface{})
	if sysctl := getMapArg(args, "sysctl"); sysctl != nil {
		profile.Sysctl = sysctl
	}

	// Parse qdisc config
	qdiscType := getStringArg(args, "qdisc_type", "")
	qdiscInterfaces := getStringArg(args, "qdisc_interfaces", "")
	if qdiscType != "" {
		profile.Qdisc = &types.QdiscConfig{
			Type:       qdiscType,
			Interfaces: qdiscInterfaces,
		}
		if qdiscInterfaces == "" {
			profile.Qdisc.Interfaces = "default-route" // default
		}
		if qdiscParams := getMapArg(args, "qdisc_params"); qdiscParams != nil {
			profile.Qdisc.Params = qdiscParams
		}
	}

	// Parse systemd config
	ensureQdiscService := getBoolArg(args, "systemd_ensure_qdisc_service", false)
	if ensureQdiscService {
		profile.Systemd = &types.SystemdConfig{
			EnsureQdiscService: true,
		}
	}

	// Create profile via HTTP client
	result, err := s.client.CreateProfile(profile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error creating profile: %v", err)), nil
	}

	return mcp.NewToolResultText(toJSON(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Profile '%s' created successfully", id),
		"profile": result,
	})), nil
}

// Helper functions for argument parsing

// parseArgs converts the any type arguments to map[string]interface{}
func parseArgs(args any) map[string]interface{} {
	if args == nil {
		return make(map[string]interface{})
	}
	if m, ok := args.(map[string]interface{}); ok {
		return m
	}
	if m, ok := args.(map[string]any); ok {
		result := make(map[string]interface{})
		for k, v := range m {
			result[k] = v
		}
		return result
	}
	return make(map[string]interface{})
}

func getIntArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		case int64:
			return int(val)
		}
	}
	return defaultVal
}

func getInt64Arg(args map[string]interface{}, key string, defaultVal int64) int64 {
	if v, ok := args[key]; ok {
		switch val := v.(type) {
		case float64:
			return int64(val)
		case int64:
			return val
		case int:
			return int64(val)
		}
	}
	return defaultVal
}

func getStringArg(args map[string]interface{}, key string, defaultVal string) string {
	if v, ok := args[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return defaultVal
}

func getBoolArg(args map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

func getMapArg(args map[string]interface{}, key string) map[string]interface{} {
	if v, ok := args[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

func toJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling result: %v", err)
	}
	return string(data)
}
