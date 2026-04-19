package planrules

import (
	"strings"

	"speedhosting/backend/internal/models"
)

const SpeedRankedsWorkshopID = "3691658485"

type Capability struct {
	Code                     string
	Name                     string
	MaxServers               int
	MaxTickRate              int
	MaxAdminSteamIDs         int
	AllowCustomMods          bool
	AdvancedSettingsUnlocked bool
	MaxUserConfigurableMods  int
	AllowSpeedRankeds        bool
	PremiumFeatureAccess     bool
}

var catalog = map[string]Capability{
	"free": {
		Code:                     "free",
		Name:                     "Free",
		MaxServers:               1,
		MaxTickRate:              120,
		MaxAdminSteamIDs:         1,
		AllowCustomMods:          false,
		AdvancedSettingsUnlocked: false,
		MaxUserConfigurableMods:  0,
		AllowSpeedRankeds:        false,
		PremiumFeatureAccess:     false,
	},
	"pro": {
		Code:                     "pro",
		Name:                     "Pro",
		MaxServers:               3,
		MaxTickRate:              240,
		MaxAdminSteamIDs:         5,
		AllowCustomMods:          true,
		AdvancedSettingsUnlocked: true,
		MaxUserConfigurableMods:  1,
		AllowSpeedRankeds:        true,
		PremiumFeatureAccess:     false,
	},
	"premium": {
		Code:                     "premium",
		Name:                     "Premium",
		MaxServers:               8,
		MaxTickRate:              360,
		MaxAdminSteamIDs:         12,
		AllowCustomMods:          true,
		AdvancedSettingsUnlocked: true,
		MaxUserConfigurableMods:  4,
		AllowSpeedRankeds:        true,
		PremiumFeatureAccess:     true,
	},
}

func ByCode(code string) (Capability, bool) {
	capability, ok := catalog[strings.ToLower(strings.TrimSpace(code))]
	return capability, ok
}

func PublicCatalog() []Capability {
	return []Capability{
		catalog["free"],
		catalog["pro"],
		catalog["premium"],
	}
}

func Apply(plan models.Plan) models.Plan {
	capability, ok := ByCode(plan.Code)
	if !ok {
		plan.MaxAdminSteamIDs = plan.MaxAdmins
		return plan
	}

	plan.Code = capability.Code
	plan.Name = capability.Name
	plan.MaxServers = capability.MaxServers
	plan.MaxTickRate = capability.MaxTickRate
	plan.MaxAdmins = capability.MaxAdminSteamIDs
	plan.MaxAdminSteamIDs = capability.MaxAdminSteamIDs
	plan.AllowCustomMods = capability.AllowCustomMods
	plan.AllowAdvancedConfig = capability.AdvancedSettingsUnlocked
	plan.MaxUserConfigurableMods = capability.MaxUserConfigurableMods
	plan.AllowSpeedRankeds = capability.AllowSpeedRankeds
	plan.PremiumFeatureAccess = capability.PremiumFeatureAccess

	return plan
}
