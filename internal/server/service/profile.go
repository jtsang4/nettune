// Package service provides business logic services
package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/jtsang4/nettune/internal/shared/types"
	"github.com/jtsang4/nettune/internal/shared/utils"
	"go.uber.org/zap"
)

// ProfileService manages configuration profiles
type ProfileService struct {
	profilesDir string
	cache       map[string]*types.Profile
	mu          sync.RWMutex
	logger      *zap.Logger
}

// NewProfileService creates a new ProfileService
func NewProfileService(profilesDir string, logger *zap.Logger) (*ProfileService, error) {
	s := &ProfileService{
		profilesDir: profilesDir,
		cache:       make(map[string]*types.Profile),
		logger:      logger,
	}

	// Ensure profiles directory exists
	if err := utils.EnsureDir(profilesDir); err != nil {
		return nil, fmt.Errorf("failed to create profiles directory: %w", err)
	}

	// Copy builtin profiles to profiles directory if they don't exist
	if err := s.copyBuiltinProfiles(); err != nil {
		logger.Warn("failed to copy builtin profiles", zap.Error(err))
	}

	// Load profiles
	if err := s.Reload(); err != nil {
		return nil, err
	}

	return s, nil
}

// copyBuiltinProfiles copies embedded builtin profiles to the profiles directory
func (s *ProfileService) copyBuiltinProfiles() error {
	entries, err := builtinProfiles.ReadDir("builtin")
	if err != nil {
		return fmt.Errorf("failed to read builtin profiles: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		targetPath := filepath.Join(s.profilesDir, entry.Name())

		// Skip if file already exists
		if _, err := os.Stat(targetPath); err == nil {
			s.logger.Debug("builtin profile already exists, skipping",
				zap.String("file", entry.Name()))
			continue
		}

		// Read embedded file
		data, err := builtinProfiles.ReadFile("builtin/" + entry.Name())
		if err != nil {
			s.logger.Warn("failed to read builtin profile",
				zap.String("file", entry.Name()),
				zap.Error(err))
			continue
		}

		// Write to profiles directory
		if err := utils.AtomicWriteFile(targetPath, data, 0644); err != nil {
			s.logger.Warn("failed to copy builtin profile",
				zap.String("file", entry.Name()),
				zap.Error(err))
			continue
		}

		s.logger.Info("copied builtin profile",
			zap.String("file", entry.Name()))
	}

	return nil
}

// List returns all available profile metadata
func (s *ProfileService) List() ([]*types.ProfileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var profiles []*types.ProfileMeta
	for _, p := range s.cache {
		profiles = append(profiles, p.ToMeta())
	}
	return profiles, nil
}

// Get returns a profile by ID
func (s *ProfileService) Get(id string) (*types.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.cache[id]
	if !ok {
		return nil, types.ErrProfileNotFound
	}
	return p, nil
}

// Validate validates a profile configuration
func (s *ProfileService) Validate(p *types.Profile) error {
	var errors []string

	// Validate ID format
	if !isValidProfileID(p.ID) {
		errors = append(errors, "invalid profile ID format (must be alphanumeric with hyphens)")
	}

	// Validate name
	if p.Name == "" {
		errors = append(errors, "name is required")
	}

	// Validate risk level
	if p.RiskLevel != "low" && p.RiskLevel != "medium" && p.RiskLevel != "high" {
		errors = append(errors, "risk_level must be 'low', 'medium', or 'high'")
	}

	// Validate sysctl keys
	if p.Sysctl != nil {
		for key := range p.Sysctl {
			if !isValidSysctlKey(key) {
				errors = append(errors, fmt.Sprintf("invalid sysctl key '%s': must be in format like 'net.core.rmem_max' or 'net.ipv4.tcp_rmem'", key))
			}
		}
	}

	// Validate qdisc config
	if p.Qdisc != nil {
		if !isValidQdiscType(p.Qdisc.Type) {
			errors = append(errors, fmt.Sprintf("invalid qdisc type '%s': must be one of 'fq', 'fq_codel', 'cake', or 'pfifo_fast'", p.Qdisc.Type))
		}
		if p.Qdisc.Interfaces != "default-route" && p.Qdisc.Interfaces != "all" {
			errors = append(errors, "qdisc interfaces must be 'default-route' or 'all'")
		}
		// Validate qdisc parameters
		if p.Qdisc.Params != nil && p.Qdisc.Type != "" {
			if err := validateQdiscParams(p.Qdisc.Type, p.Qdisc.Params); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%w: %s", types.ErrValidationFailed, strings.Join(errors, "; "))
	}
	return nil
}

// Reload reloads profiles from disk
func (s *ProfileService) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := utils.ListFiles(s.profilesDir, ".json")
	if err != nil {
		return fmt.Errorf("failed to list profile files: %w", err)
	}

	newCache := make(map[string]*types.Profile)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			s.logger.Warn("failed to read profile file",
				zap.String("file", file),
				zap.Error(err))
			continue
		}

		var profile types.Profile
		if err := json.Unmarshal(data, &profile); err != nil {
			s.logger.Warn("failed to parse profile file",
				zap.String("file", file),
				zap.Error(err))
			continue
		}

		// Basic validation
		if profile.ID == "" {
			s.logger.Warn("profile missing ID",
				zap.String("file", file))
			continue
		}

		newCache[profile.ID] = &profile
		s.logger.Debug("loaded profile",
			zap.String("id", profile.ID),
			zap.String("file", file))
	}

	s.cache = newCache
	s.logger.Info("loaded profiles", zap.Int("count", len(newCache)))
	return nil
}

// Save saves a profile to disk
func (s *ProfileService) Save(p *types.Profile) error {
	if err := s.Validate(p); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	filename := fmt.Sprintf("%s.json", p.ID)
	path := filepath.Join(s.profilesDir, filename)

	if err := utils.AtomicWriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write profile: %w", err)
	}

	s.mu.Lock()
	s.cache[p.ID] = p
	s.mu.Unlock()

	s.logger.Info("saved profile", zap.String("id", p.ID))
	return nil
}

// Helper functions

var profileIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func isValidProfileID(id string) bool {
	if len(id) < 2 {
		return false
	}
	return profileIDRegex.MatchString(id)
}

var sysctlKeyRegex = regexp.MustCompile(`^[a-z][a-z0-9_.]*[a-z0-9]$`)

func isValidSysctlKey(key string) bool {
	return sysctlKeyRegex.MatchString(key)
}

func isValidQdiscType(qdiscType string) bool {
	validTypes := map[string]bool{
		"fq":         true,
		"fq_codel":   true,
		"cake":       true,
		"pfifo_fast": true,
	}
	return validTypes[qdiscType]
}

// validQdiscParams defines valid parameters for each qdisc type
var validQdiscParams = map[string][]string{
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

// validateQdiscParams validates qdisc parameters for a given qdisc type
func validateQdiscParams(qdiscType string, params map[string]interface{}) error {
	validParams, ok := validQdiscParams[qdiscType]
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
		return fmt.Errorf("invalid qdisc parameter(s) for '%s': %v. Valid parameters are: %v",
			qdiscType, invalidParams, validParams)
	}

	return nil
}
