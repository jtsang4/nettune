package service

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jtsang4/nettune/internal/server/adapter"
	"github.com/jtsang4/nettune/internal/shared/types"
	"go.uber.org/zap"
)

// ApplyService handles profile application and rollback
type ApplyService struct {
	profileService  *ProfileService
	snapshotService *SnapshotService
	historyService  *HistoryService
	adapter         *adapter.SystemAdapter
	mu              sync.Mutex
	applyLock       bool
	logger          *zap.Logger
}

// NewApplyService creates a new ApplyService
func NewApplyService(
	profileService *ProfileService,
	snapshotService *SnapshotService,
	historyService *HistoryService,
	adapter *adapter.SystemAdapter,
	logger *zap.Logger,
) *ApplyService {
	return &ApplyService{
		profileService:  profileService,
		snapshotService: snapshotService,
		historyService:  historyService,
		adapter:         adapter,
		logger:          logger,
	}
}

// Apply applies a profile
func (s *ApplyService) Apply(req *types.ApplyRequest) (*types.ApplyResult, error) {
	s.mu.Lock()
	if s.applyLock {
		s.mu.Unlock()
		return nil, types.ErrApplyInProgress
	}
	s.applyLock = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.applyLock = false
		s.mu.Unlock()
	}()

	// Get profile
	profile, err := s.profileService.Get(req.ProfileID)
	if err != nil {
		return nil, err
	}

	// Validate profile
	if err := s.profileService.Validate(profile); err != nil {
		return nil, err
	}

	// Get current state for plan generation
	currentState, err := s.snapshotService.GetCurrentState()
	if err != nil {
		return nil, fmt.Errorf("failed to get current state: %w", err)
	}

	// Generate plan
	plan := s.generatePlan(profile, currentState)

	result := &types.ApplyResult{
		Mode:      req.Mode,
		ProfileID: req.ProfileID,
		Plan:      plan,
	}

	// For dry_run, just return the plan
	if req.Mode == "dry_run" {
		result.Success = true
		return result, nil
	}

	// For commit mode, create snapshot first
	snapshot, err := s.snapshotService.Create()
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}
	result.SnapshotID = snapshot.ID

	// Apply changes
	if err := s.applyChanges(profile); err != nil {
		s.logger.Error("failed to apply changes, rolling back",
			zap.String("profile", profile.ID),
			zap.Error(err))

		// Rollback on failure (use internal method since we already hold the lock)
		if rollbackErr := s.rollbackInternal(snapshot.ID); rollbackErr != nil {
			s.logger.Error("rollback failed", zap.Error(rollbackErr))
			result.Errors = append(result.Errors, fmt.Sprintf("apply failed: %v; rollback also failed: %v", err, rollbackErr))
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("apply failed and rolled back: %v", err))
		}
		result.Success = false
		return result, nil
	}

	// Verify changes
	verification := s.verifyChanges(profile)
	result.Verification = verification

	if !verification.SysctlOK || !verification.QdiscOK {
		s.logger.Error("verification failed, rolling back",
			zap.String("profile", profile.ID))

		// Use internal method since we already hold the lock
		if rollbackErr := s.rollbackInternal(snapshot.ID); rollbackErr != nil {
			s.logger.Error("rollback failed", zap.Error(rollbackErr))
			result.Errors = append(result.Errors, fmt.Sprintf("verification failed; rollback also failed: %v", rollbackErr))
		} else {
			result.Errors = append(result.Errors, "verification failed; rolled back")
		}
		result.Success = false
		return result, nil
	}

	result.Success = true
	result.AppliedAt = time.Now()

	// Record in history
	if s.historyService != nil {
		s.historyService.RecordApply(req.ProfileID, snapshot.ID, true)
	}

	s.logger.Info("applied profile successfully",
		zap.String("profile", profile.ID),
		zap.String("snapshot", snapshot.ID))

	return result, nil
}

// Rollback restores a previous snapshot (acquires lock)
func (s *ApplyService) Rollback(snapshotID string) error {
	s.mu.Lock()
	if s.applyLock {
		s.mu.Unlock()
		return types.ErrApplyInProgress
	}
	s.applyLock = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.applyLock = false
		s.mu.Unlock()
	}()

	return s.rollbackInternal(snapshotID)
}

// rollbackInternal performs the rollback without acquiring lock (caller must hold lock)
func (s *ApplyService) rollbackInternal(snapshotID string) error {
	snapshot, err := s.snapshotService.Get(snapshotID)
	if err != nil {
		return err
	}

	// Restore sysctl values
	if snapshot.State.Sysctl != nil {
		if err := s.adapter.Sysctl.SetMultiple(snapshot.State.Sysctl); err != nil {
			s.logger.Error("failed to restore sysctl", zap.Error(err))
		}
	}

	// Restore backed up files
	for path, content := range snapshot.Backups {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			s.logger.Error("failed to restore file",
				zap.String("path", path),
				zap.Error(err))
		}
	}

	// Reload sysctl from restored file
	sysctlFile := "/etc/sysctl.d/99-nettune.conf"
	if _, ok := snapshot.Backups[sysctlFile]; ok {
		s.adapter.Sysctl.LoadFromFile(sysctlFile)
	}

	// Restore qdisc
	for iface, info := range snapshot.State.Qdisc {
		if info != nil {
			if err := s.adapter.Qdisc.Set(iface, info.Type, nil); err != nil {
				s.logger.Error("failed to restore qdisc",
					zap.String("interface", iface),
					zap.Error(err))
			}
		}
	}

	if s.historyService != nil {
		s.historyService.RecordRollback(snapshotID, true)
	}

	s.logger.Info("rolled back to snapshot", zap.String("snapshot", snapshotID))
	return nil
}

// RollbackLast rolls back to the most recent snapshot
func (s *ApplyService) RollbackLast() error {
	snapshot, err := s.snapshotService.GetLatest()
	if err != nil {
		return err
	}
	return s.Rollback(snapshot.ID)
}

// GetStatus returns the current system status
func (s *ApplyService) GetStatus() (*types.SystemStatus, error) {
	currentState, err := s.snapshotService.GetCurrentState()
	if err != nil {
		return nil, err
	}

	snapshots, err := s.snapshotService.List()
	if err != nil {
		return nil, err
	}

	status := &types.SystemStatus{
		CurrentState:   currentState,
		SnapshotsCount: len(snapshots),
	}

	if len(snapshots) > 0 {
		status.LatestSnapshotID = snapshots[0].ID
	}

	// Get last apply info from history
	if s.historyService != nil {
		status.LastApply = s.historyService.GetLastApply()
	}

	return status, nil
}

// generatePlan generates an apply plan based on profile and current state
func (s *ApplyService) generatePlan(profile *types.Profile, currentState *types.SystemState) *types.ApplyPlan {
	plan := &types.ApplyPlan{
		SysctlChanges:  make(map[string]*types.Change),
		QdiscChanges:   make(map[string]*types.Change),
		SystemdChanges: make(map[string]*types.Change),
	}

	// Sysctl changes
	if profile.Sysctl != nil {
		for key, newValue := range profile.Sysctl {
			oldValue := currentState.Sysctl[key]
			newValueStr := formatSysctlValue(newValue)
			// Normalize both values for comparison
			if normalizeSysctlValue(oldValue) != normalizeSysctlValue(newValueStr) {
				plan.SysctlChanges[key] = &types.Change{
					From: oldValue,
					To:   newValueStr,
				}
			}
		}
	}

	// Qdisc changes
	if profile.Qdisc != nil {
		var interfaces []string
		if profile.Qdisc.Interfaces == "default-route" {
			if iface, err := s.adapter.Qdisc.GetDefaultRouteInterface(); err == nil {
				interfaces = []string{iface}
			}
		} else {
			interfaces, _ = s.adapter.Qdisc.ListInterfaces()
		}

		for _, iface := range interfaces {
			currentQdisc := currentState.Qdisc[iface]
			currentType := ""
			if currentQdisc != nil {
				currentType = currentQdisc.Type
			}
			if currentType != profile.Qdisc.Type {
				plan.QdiscChanges[iface] = &types.Change{
					From: currentType,
					To:   profile.Qdisc.Type,
				}
			}
		}
	}

	// Systemd changes
	if profile.Systemd != nil && profile.Systemd.EnsureQdiscService {
		unitActive := currentState.SystemdUnits[adapter.NettuneQdiscServiceName]
		if !unitActive {
			plan.SystemdChanges[adapter.NettuneQdiscServiceName] = &types.Change{
				From: "inactive",
				To:   "active",
			}
		}
	}

	return plan
}

// applyChanges applies the profile changes
func (s *ApplyService) applyChanges(profile *types.Profile) error {
	// Apply sysctl changes
	if profile.Sysctl != nil {
		sysctlValues := make(map[string]string)
		for key, value := range profile.Sysctl {
			sysctlValues[key] = formatSysctlValue(value)
		}

		// Write to persistent file
		if err := s.adapter.Sysctl.WriteToFile("/etc/sysctl.d/99-nettune.conf", sysctlValues); err != nil {
			return fmt.Errorf("failed to write sysctl file: %w", err)
		}

		// Apply immediately
		if err := s.adapter.Sysctl.SetMultiple(sysctlValues); err != nil {
			return fmt.Errorf("failed to apply sysctl: %w", err)
		}
	}

	// Apply qdisc changes
	if profile.Qdisc != nil {
		// Validate qdisc parameters before applying
		if profile.Qdisc.Params != nil {
			if err := s.adapter.Qdisc.ValidateQdiscParams(profile.Qdisc.Type, profile.Qdisc.Params); err != nil {
				return fmt.Errorf("qdisc validation failed: %w", err)
			}
		}

		var interfaces []string
		if profile.Qdisc.Interfaces == "default-route" {
			iface, err := s.adapter.Qdisc.GetDefaultRouteInterface()
			if err != nil {
				return fmt.Errorf("failed to get default route interface: %w", err)
			}
			interfaces = []string{iface}
		} else {
			var err error
			interfaces, err = s.adapter.Qdisc.ListInterfaces()
			if err != nil {
				return fmt.Errorf("failed to list interfaces: %w", err)
			}
		}

		for _, iface := range interfaces {
			if err := s.adapter.Qdisc.Set(iface, profile.Qdisc.Type, profile.Qdisc.Params); err != nil {
				return fmt.Errorf("failed to set qdisc for %s: %w", iface, err)
			}
		}

		// Setup systemd service for persistence if requested
		if profile.Systemd != nil && profile.Systemd.EnsureQdiscService {
			if err := s.ensureQdiscService(profile.Qdisc.Type, interfaces); err != nil {
				s.logger.Warn("failed to setup qdisc service", zap.Error(err))
			}
		}
	}

	return nil
}

// verifyChanges verifies that the changes were applied correctly
func (s *ApplyService) verifyChanges(profile *types.Profile) *types.VerificationResult {
	result := &types.VerificationResult{
		SysctlOK:  true,
		QdiscOK:   true,
		SystemdOK: true,
	}

	// Verify sysctl
	if profile.Sysctl != nil {
		for key, expectedValue := range profile.Sysctl {
			actualValue, err := s.adapter.Sysctl.Get(key)
			if err != nil {
				result.SysctlOK = false
				result.Errors = append(result.Errors, fmt.Sprintf("failed to read sysctl %s: %v", key, err))
				continue
			}

			expectedStr := formatSysctlValue(expectedValue)
			// Normalize both values (handle tab/space differences in tcp_rmem, tcp_wmem, etc.)
			if normalizeSysctlValue(actualValue) != normalizeSysctlValue(expectedStr) {
				result.SysctlOK = false
				result.Errors = append(result.Errors, fmt.Sprintf("sysctl %s: expected %s, got %s", key, expectedStr, actualValue))
			}
		}
	}

	// Verify qdisc
	if profile.Qdisc != nil {
		var interfaces []string
		if profile.Qdisc.Interfaces == "default-route" {
			if iface, err := s.adapter.Qdisc.GetDefaultRouteInterface(); err == nil {
				interfaces = []string{iface}
			}
		} else {
			interfaces, _ = s.adapter.Qdisc.ListInterfaces()
		}

		for _, iface := range interfaces {
			info, err := s.adapter.Qdisc.Get(iface)
			if err != nil {
				result.QdiscOK = false
				result.Errors = append(result.Errors, fmt.Sprintf("failed to read qdisc for %s: %v", iface, err))
				continue
			}

			if info.Type != profile.Qdisc.Type {
				result.QdiscOK = false
				result.Errors = append(result.Errors, fmt.Sprintf("qdisc for %s: expected %s, got %s", iface, profile.Qdisc.Type, info.Type))
			}
		}
	}

	// Verify systemd
	if profile.Systemd != nil && profile.Systemd.EnsureQdiscService {
		active, _ := s.adapter.Systemd.IsActive(adapter.NettuneQdiscServiceName)
		enabled, _ := s.adapter.Systemd.IsEnabled(adapter.NettuneQdiscServiceName)
		if !active || !enabled {
			result.SystemdOK = false
			result.Errors = append(result.Errors, fmt.Sprintf("service %s is not active or enabled", adapter.NettuneQdiscServiceName))
		}
	}

	return result
}

// ensureQdiscService creates and enables the qdisc persistence service
func (s *ApplyService) ensureQdiscService(qdiscType string, interfaces []string) error {
	// Create setup script
	iface := ""
	if len(interfaces) > 0 {
		iface = interfaces[0]
	}
	script := adapter.GenerateQdiscSetupScript(qdiscType, iface)
	if err := os.WriteFile(adapter.NettuneQdiscScriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write qdisc script: %w", err)
	}

	// Create systemd unit
	unit := adapter.GenerateQdiscServiceUnit()
	if err := s.adapter.Systemd.CreateUnit(adapter.NettuneQdiscServiceName, unit); err != nil {
		return fmt.Errorf("failed to create systemd unit: %w", err)
	}

	// Enable and start
	if err := s.adapter.Systemd.Enable(adapter.NettuneQdiscServiceName); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	if err := s.adapter.Systemd.Start(adapter.NettuneQdiscServiceName); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// formatSysctlValue formats a sysctl value to string, handling numeric types to avoid scientific notation
func formatSysctlValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		// Avoid scientific notation for large integers
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	case int, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint32, uint64:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// normalizeSysctlValue normalizes whitespace in sysctl values for comparison
// This handles cases where tcp_rmem/tcp_wmem values use tabs vs spaces
func normalizeSysctlValue(value string) string {
	// Replace tabs with spaces, then normalize multiple spaces to single space
	result := strings.ReplaceAll(value, "\t", " ")
	// Normalize multiple spaces
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return strings.TrimSpace(result)
}
