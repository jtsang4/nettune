package handlers

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jtsang4/nettune/internal/server/service"
	"github.com/jtsang4/nettune/internal/shared/types"
)

// ProfileHandler handles profile-related HTTP endpoints
type ProfileHandler struct {
	profileService *service.ProfileService
}

// NewProfileHandler creates a new ProfileHandler
func NewProfileHandler(profileService *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
	}
}

// List handles GET /profiles
func (h *ProfileHandler) List(c *gin.Context) {
	profiles, err := h.profileService.List()
	if err != nil {
		internalError(c, err.Error())
		return
	}

	success(c, gin.H{
		"profiles": profiles,
	})
}

// Get handles GET /profiles/:id
func (h *ProfileHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "profile id is required")
		return
	}

	profile, err := h.profileService.Get(id)
	if err != nil {
		if errors.Is(err, types.ErrProfileNotFound) {
			notFound(c, "profile not found")
			return
		}
		internalError(c, err.Error())
		return
	}

	success(c, profile)
}

// CreateProfileRequest represents a request to create a new profile
type CreateProfileRequest struct {
	ID             string                 `json:"id" binding:"required"`
	Name           string                 `json:"name" binding:"required"`
	Description    string                 `json:"description,omitempty"`
	RiskLevel      string                 `json:"risk_level" binding:"required,oneof=low medium high"`
	RequiresReboot bool                   `json:"requires_reboot,omitempty"`
	Sysctl         map[string]interface{} `json:"sysctl,omitempty"`
	Qdisc          *types.QdiscConfig     `json:"qdisc,omitempty"`
	Systemd        *types.SystemdConfig   `json:"systemd,omitempty"`
}

// Create handles POST /profiles
func (h *ProfileHandler) Create(c *gin.Context) {
	var req CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	// Convert request to Profile
	profile := &types.Profile{
		ID:             req.ID,
		Name:           req.Name,
		Description:    req.Description,
		RiskLevel:      req.RiskLevel,
		RequiresReboot: req.RequiresReboot,
		Sysctl:         req.Sysctl,
		Qdisc:          req.Qdisc,
		Systemd:        req.Systemd,
	}

	// Save profile (validation happens inside Save)
	if err := h.profileService.Save(profile); err != nil {
		if errors.Is(err, types.ErrValidationFailed) {
			badRequest(c, err.Error())
			return
		}
		internalError(c, err.Error())
		return
	}

	success(c, profile.ToMeta())
}

func notFound(c *gin.Context, message string) {
	c.JSON(404, gin.H{"success": false, "error": gin.H{"code": "NOT_FOUND", "message": message}})
}
