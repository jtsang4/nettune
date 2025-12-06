package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jtsang4/nettune/internal/shared/types"
	"go.uber.org/zap"
)

func TestNewProfileService(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	svc, err := NewProfileService(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewProfileService failed: %v", err)
	}

	if svc == nil {
		t.Fatal("ProfileService should not be nil")
	}
}

func TestProfileServiceValidate(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	svc, err := NewProfileService(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewProfileService failed: %v", err)
	}

	tests := []struct {
		name    string
		profile *types.Profile
		wantErr bool
	}{
		{
			name: "valid profile",
			profile: &types.Profile{
				ID:        "test-profile",
				Name:      "Test Profile",
				RiskLevel: "low",
			},
			wantErr: false,
		},
		{
			name: "invalid ID format",
			profile: &types.Profile{
				ID:        "x", // too short
				Name:      "Test",
				RiskLevel: "low",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			profile: &types.Profile{
				ID:        "test-profile",
				Name:      "",
				RiskLevel: "low",
			},
			wantErr: true,
		},
		{
			name: "invalid risk level",
			profile: &types.Profile{
				ID:        "test-profile",
				Name:      "Test",
				RiskLevel: "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid with sysctl",
			profile: &types.Profile{
				ID:        "bbr-test",
				Name:      "BBR Test",
				RiskLevel: "low",
				Sysctl: map[string]interface{}{
					"net.core.default_qdisc":          "fq",
					"net.ipv4.tcp_congestion_control": "bbr",
				},
			},
			wantErr: false,
		},
		{
			name: "valid with qdisc",
			profile: &types.Profile{
				ID:        "fq-test",
				Name:      "FQ Test",
				RiskLevel: "medium",
				Qdisc: &types.QdiscConfig{
					Type:       "fq",
					Interfaces: "default-route",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid qdisc type",
			profile: &types.Profile{
				ID:        "bad-qdisc",
				Name:      "Bad Qdisc",
				RiskLevel: "low",
				Qdisc: &types.QdiscConfig{
					Type:       "invalid_qdisc",
					Interfaces: "default-route",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Validate(tt.profile)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProfileServiceSaveAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	svc, err := NewProfileService(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewProfileService failed: %v", err)
	}

	profile := &types.Profile{
		ID:          "save-test",
		Name:        "Save Test Profile",
		Description: "Testing save functionality",
		RiskLevel:   "low",
		Sysctl: map[string]interface{}{
			"net.core.default_qdisc": "fq",
		},
	}

	// Save profile
	if err := svc.Save(profile); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tmpDir, "save-test.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Profile file should exist after save")
	}

	// Get profile
	retrieved, err := svc.Get("save-test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != profile.ID {
		t.Errorf("Retrieved ID = %q, want %q", retrieved.ID, profile.ID)
	}

	if retrieved.Name != profile.Name {
		t.Errorf("Retrieved Name = %q, want %q", retrieved.Name, profile.Name)
	}
}

func TestProfileServiceGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	svc, err := NewProfileService(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewProfileService failed: %v", err)
	}

	_, err = svc.Get("nonexistent")
	if err == nil {
		t.Error("Get should return error for nonexistent profile")
	}
}

func TestProfileServiceList(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	svc, err := NewProfileService(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewProfileService failed: %v", err)
	}

	// Get initial count (includes builtin profiles)
	initialProfiles, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	initialCount := len(initialProfiles)

	// Add profiles
	for i := 0; i < 3; i++ {
		profile := &types.Profile{
			ID:        "profile-" + string(rune('a'+i)) + string(rune('a'+i)),
			Name:      "Profile " + string(rune('A'+i)),
			RiskLevel: "low",
		}
		if err := svc.Save(profile); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List again - should have 3 more than initial
	profiles, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	expectedCount := initialCount + 3
	if len(profiles) != expectedCount {
		t.Errorf("Expected %d profiles, got %d", expectedCount, len(profiles))
	}
}

func TestIsValidProfileID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"bbr-fq", true},
		{"test-profile-123", true},
		{"ab", true},
		{"a", false},            // too short
		{"", false},             // empty
		{"a-", false},           // ends with dash
		{"-a", false},           // starts with dash
		{"Test", false},         // uppercase
		{"test_profile", false}, // underscore not allowed
	}

	for _, tt := range tests {
		result := isValidProfileID(tt.id)
		if result != tt.valid {
			t.Errorf("isValidProfileID(%q) = %v, want %v", tt.id, result, tt.valid)
		}
	}
}

func TestIsValidQdiscType(t *testing.T) {
	valid := []string{"fq", "fq_codel", "cake", "pfifo_fast"}
	invalid := []string{"", "invalid", "noqueue", "htb"}

	for _, qdiscType := range valid {
		if !isValidQdiscType(qdiscType) {
			t.Errorf("isValidQdiscType(%q) should be true", qdiscType)
		}
	}

	for _, qdiscType := range invalid {
		if isValidQdiscType(qdiscType) {
			t.Errorf("isValidQdiscType(%q) should be false", qdiscType)
		}
	}
}
