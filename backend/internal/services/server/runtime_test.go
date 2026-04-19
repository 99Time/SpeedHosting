package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"speedhosting/backend/internal/config"
	"speedhosting/backend/internal/models"
	"speedhosting/backend/internal/planrules"
)

func TestResolveServiceNameUsesHostedPlanPrefixes(t *testing.T) {
	service := &Service{cfg: config.Config{PuckServicePrefix: "puck@"}}

	testCases := []struct {
		name     string
		planCode string
		want     string
	}{
		{name: "free", planCode: "free", want: "puck-free@server_weekend_arena"},
		{name: "pro", planCode: "pro", want: "puck-pro@server_weekend_arena"},
		{name: "premium", planCode: "premium", want: "puck-premium@server_weekend_arena"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := service.resolveServiceName(testCase.planCode, "/srv/puckserver/server_weekend_arena.json", "")
			if got != testCase.want {
				t.Fatalf("resolveServiceName(%q) = %q, want %q", testCase.planCode, got, testCase.want)
			}
		})
	}
}

func TestResolveServiceNameKeepsPersonalFallbackUntouched(t *testing.T) {
	service := &Service{cfg: config.Config{PuckServicePrefix: "puck@"}}

	if got := service.resolveServiceName("unknown", "/srv/puckserver/server_private_scrim.json", ""); got != "puck@server_private_scrim" {
		t.Fatalf("expected personal fallback to remain on puck@, got %q", got)
	}

	if got := service.resolveServiceName("unknown", "/srv/puckserver/server_private_scrim.json", "puck@legacy-private"); got != "puck@legacy-private" {
		t.Fatalf("expected stored personal service name to be preserved, got %q", got)
	}
}

func TestApplyHostedServerConfigFreePlanStripsTemplateSpeedRankeds(t *testing.T) {
	service := &Service{}
	plan := planrules.Apply(models.Plan{Code: "free"})
	baseMods := []models.ServerConfigMod{
		{WorkshopID: "1111111111", Enabled: true},
		{WorkshopID: planrules.SpeedRankedsWorkshopID, Enabled: true},
	}
	currentConfig := models.ServerConfig{
		MaxPlayers:      10,
		ServerTickRate:  120,
		ClientTickRate:  120,
		TargetFrameRate: 120,
		Mods:            baseMods,
	}
	desiredConfig := currentConfig
	payload := map[string]any{}

	normalized, _, err := service.applyHostedServerConfig(payload, currentConfig, desiredConfig, plan, nil, baseMods)
	if err != nil {
		t.Fatalf("applyHostedServerConfig returned error: %v", err)
	}

	if containsWorkshopID(normalized.Mods, planrules.SpeedRankedsWorkshopID) {
		t.Fatalf("expected free plan to strip SpeedRankeds from template mods, got %v", workshopIDs(normalized.Mods))
	}

	if !containsWorkshopID(normalized.Mods, "1111111111") {
		t.Fatalf("expected non-restricted template mod to remain, got %v", workshopIDs(normalized.Mods))
	}
	if normalized.ServerTickRate != 120 || normalized.ClientTickRate != 120 {
		t.Fatalf("expected free plan tick rates to stay at 120, got server=%d client=%d", normalized.ServerTickRate, normalized.ClientTickRate)
	}
	if enabled, ok := payload["speedRankedsEnabled"].(bool); !ok || enabled {
		t.Fatalf("expected payload speedRankedsEnabled=false, got %#v", payload["speedRankedsEnabled"])
	}
}

func TestApplyHostedServerConfigFreePlanRejectsUserMods(t *testing.T) {
	service := &Service{}
	plan := planrules.Apply(models.Plan{Code: "free"})
	currentConfig := models.ServerConfig{MaxPlayers: 10, ServerTickRate: 120, ClientTickRate: 120}
	desiredConfig := currentConfig
	desiredConfig.Mods = []models.ServerConfigMod{{WorkshopID: "2222222222", Enabled: true}}

	_, _, err := service.applyHostedServerConfig(map[string]any{}, currentConfig, desiredConfig, plan, nil, nil)
	if !errors.Is(err, ErrPlanLimitReached) {
		t.Fatalf("expected ErrPlanLimitReached, got %v", err)
	}
}

func TestApplyHostedServerConfigProPlanRejectsTooManyMods(t *testing.T) {
	service := &Service{}
	plan := planrules.Apply(models.Plan{Code: "pro"})
	currentConfig := models.ServerConfig{MaxPlayers: 10, ServerTickRate: 240, ClientTickRate: 240}
	desiredConfig := currentConfig
	desiredConfig.Mods = []models.ServerConfigMod{
		{WorkshopID: "2222222222", Enabled: true},
		{WorkshopID: "3333333333", Enabled: true},
	}

	_, _, err := service.applyHostedServerConfig(map[string]any{}, currentConfig, desiredConfig, plan, nil, nil)
	if !errors.Is(err, ErrPlanLimitReached) {
		t.Fatalf("expected ErrPlanLimitReached, got %v", err)
	}
}

func TestApplyHostedServerConfigProPlanRejectsTooManyAdmins(t *testing.T) {
	service := &Service{}
	plan := planrules.Apply(models.Plan{Code: "pro"})
	currentConfig := models.ServerConfig{MaxPlayers: 10, ServerTickRate: 240, ClientTickRate: 240}
	desiredConfig := currentConfig
	desiredConfig.AdminSteamIDs = []string{
		"76561198000000001",
		"76561198000000002",
		"76561198000000003",
		"76561198000000004",
		"76561198000000005",
		"76561198000000006",
	}

	_, _, err := service.applyHostedServerConfig(map[string]any{}, currentConfig, desiredConfig, plan, nil, nil)
	if !errors.Is(err, ErrPlanLimitReached) {
		t.Fatalf("expected ErrPlanLimitReached, got %v", err)
	}
}

func TestApplyHostedServerConfigProPlanRejectsTickRateAboveLimit(t *testing.T) {
	service := &Service{}
	plan := planrules.Apply(models.Plan{Code: "pro"})
	currentConfig := models.ServerConfig{MaxPlayers: 10, ServerTickRate: 240, ClientTickRate: 240}
	desiredConfig := currentConfig
	desiredConfig.ServerTickRate = 300
	desiredConfig.ClientTickRate = 300

	_, _, err := service.applyHostedServerConfig(map[string]any{}, currentConfig, desiredConfig, plan, nil, nil)
	if !errors.Is(err, ErrPlanLimitReached) {
		t.Fatalf("expected ErrPlanLimitReached, got %v", err)
	}
}

func TestApplyHostedServerConfigPremiumPlanAllowsExpandedLimits(t *testing.T) {
	service := &Service{}
	plan := planrules.Apply(models.Plan{Code: "premium"})
	currentConfig := models.ServerConfig{MaxPlayers: 10, ServerTickRate: 240, ClientTickRate: 240}
	desiredConfig := currentConfig
	desiredConfig.ServerTickRate = 360
	desiredConfig.ClientTickRate = 360
	desiredConfig.AdminSteamIDs = []string{
		"76561198000000001",
		"76561198000000002",
		"76561198000000003",
		"76561198000000004",
	}
	desiredConfig.Mods = []models.ServerConfigMod{
		{WorkshopID: planrules.SpeedRankedsWorkshopID, Enabled: true},
		{WorkshopID: "2222222222", Enabled: true},
		{WorkshopID: "3333333333", Enabled: true},
		{WorkshopID: "4444444444", Enabled: true},
	}

	payload := map[string]any{}
	normalized, merge, err := service.applyHostedServerConfig(payload, currentConfig, desiredConfig, plan, nil, nil)
	if err != nil {
		t.Fatalf("applyHostedServerConfig returned error: %v", err)
	}
	if merge.DroppedDueToPlanLimit {
		t.Fatalf("expected premium admin merge to fit within limits")
	}
	if normalized.ServerTickRate != 360 || normalized.ClientTickRate != 360 {
		t.Fatalf("expected premium plan to allow 360 tick rate, got server=%d client=%d", normalized.ServerTickRate, normalized.ClientTickRate)
	}
	if len(normalized.AdminSteamIDs) != 4 {
		t.Fatalf("expected 4 admin Steam IDs, got %v", normalized.AdminSteamIDs)
	}
	if len(normalized.Mods) != 4 {
		t.Fatalf("expected 4 mods, got %v", workshopIDs(normalized.Mods))
	}
	if !containsWorkshopID(normalized.Mods, planrules.SpeedRankedsWorkshopID) {
		t.Fatalf("expected premium plan to allow SpeedRankeds, got %v", workshopIDs(normalized.Mods))
	}
	if enabled, ok := payload["speedRankedsEnabled"].(bool); !ok || !enabled {
		t.Fatalf("expected payload speedRankedsEnabled=true, got %#v", payload["speedRankedsEnabled"])
	}
	if mode, ok := payload["serverMode"].(string); !ok || mode != "competitive" {
		t.Fatalf("expected payload serverMode=competitive, got %#v", payload["serverMode"])
	}
}

func TestApplyHostedServerConfigNormalizesPublicServerMode(t *testing.T) {
	service := &Service{}
	plan := planrules.Apply(models.Plan{Code: "premium"})
	currentConfig := models.ServerConfig{MaxPlayers: 10, ServerTickRate: 240, ClientTickRate: 240, ServerMode: "competitive"}
	desiredConfig := currentConfig
	desiredConfig.ServerMode = "public"
	desiredConfig.IsPublic = false

	payload := map[string]any{}
	normalized, _, err := service.applyHostedServerConfig(payload, currentConfig, desiredConfig, plan, nil, nil)
	if err != nil {
		t.Fatalf("applyHostedServerConfig returned error: %v", err)
	}
	if normalized.ServerMode != "public" {
		t.Fatalf("expected normalized server mode public, got %q", normalized.ServerMode)
	}
	if normalized.IsPublic {
		t.Fatalf("expected normalized isPublic to remain false, got true")
	}
	if mode, ok := payload["serverMode"].(string); !ok || mode != "public" {
		t.Fatalf("expected payload serverMode=public, got %#v", payload["serverMode"])
	}
	if isPublic, ok := payload["isPublic"].(bool); !ok || isPublic {
		t.Fatalf("expected payload isPublic=false, got %#v", payload["isPublic"])
	}
}

func TestApplyHostedServerConfigNormalizesTrainingServerMode(t *testing.T) {
	service := &Service{}
	plan := planrules.Apply(models.Plan{Code: "premium"})
	currentConfig := models.ServerConfig{MaxPlayers: 10, ServerTickRate: 240, ClientTickRate: 240, ServerMode: "competitive"}
	desiredConfig := currentConfig
	desiredConfig.ServerMode = "training"
	desiredConfig.IsPublic = true

	payload := map[string]any{}
	normalized, _, err := service.applyHostedServerConfig(payload, currentConfig, desiredConfig, plan, nil, nil)
	if err != nil {
		t.Fatalf("applyHostedServerConfig returned error: %v", err)
	}
	if normalized.ServerMode != "training" {
		t.Fatalf("expected normalized server mode training, got %q", normalized.ServerMode)
	}
	if !normalized.IsPublic {
		t.Fatalf("expected normalized isPublic to remain true, got false")
	}
	if mode, ok := payload["serverMode"].(string); !ok || mode != "training" {
		t.Fatalf("expected payload serverMode=training, got %#v", payload["serverMode"])
	}
	if isPublic, ok := payload["isPublic"].(bool); !ok || !isPublic {
		t.Fatalf("expected payload isPublic=true, got %#v", payload["isPublic"])
	}
}

func TestApplyHostedServerConfigKeepsCurrentServerModeWhenUpdateOmitsIt(t *testing.T) {
	service := &Service{}
	plan := planrules.Apply(models.Plan{Code: "premium"})
	currentConfig := models.ServerConfig{MaxPlayers: 10, ServerTickRate: 240, ClientTickRate: 240, ServerMode: "public", IsPublic: true}
	desiredConfig := currentConfig
	desiredConfig.ServerMode = ""
	desiredConfig.IsPublic = false

	payload := map[string]any{}
	normalized, _, err := service.applyHostedServerConfig(payload, currentConfig, desiredConfig, plan, nil, nil)
	if err != nil {
		t.Fatalf("applyHostedServerConfig returned error: %v", err)
	}
	if normalized.ServerMode != "public" {
		t.Fatalf("expected current server mode to be preserved, got %q", normalized.ServerMode)
	}
	if normalized.IsPublic {
		t.Fatalf("expected isPublic=false from desired config, got true")
	}
	if mode, ok := payload["serverMode"].(string); !ok || mode != "public" {
		t.Fatalf("expected payload serverMode=public, got %#v", payload["serverMode"])
	}
	if isPublic, ok := payload["isPublic"].(bool); !ok || isPublic {
		t.Fatalf("expected payload isPublic=false, got %#v", payload["isPublic"])
	}
}

func TestExtractServerConfigKeepsIsPublicIndependentFromServerMode(t *testing.T) {
	config := extractServerConfig(map[string]any{
		"isPublic":   true,
		"serverMode": "competitive",
	})
	if !config.IsPublic {
		t.Fatalf("expected isPublic=true to be preserved")
	}
	if config.ServerMode != "competitive" {
		t.Fatalf("expected serverMode=competitive, got %q", config.ServerMode)
	}

	config = extractServerConfig(map[string]any{
		"isPublic":   false,
		"serverMode": "public",
	})
	if config.IsPublic {
		t.Fatalf("expected isPublic=false to be preserved")
	}
	if config.ServerMode != "public" {
		t.Fatalf("expected serverMode=public, got %q", config.ServerMode)
	}
}

func TestPrepareRuntimeServerWritesServerModePerInstance(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "server_template.json")
	template := map[string]any{
		"name":            "Template",
		"maxPlayers":      10,
		"serverTickRate":  120,
		"clientTickRate":  120,
		"targetFrameRate": 120,
		"isPublic":        true,
	}
	templateJSON, err := json.Marshal(template)
	if err != nil {
		t.Fatalf("marshal template: %v", err)
	}
	if err := os.WriteFile(templatePath, templateJSON, 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	service := &Service{cfg: config.Config{PuckConfigDir: tempDir, PuckTemplateConfig: templatePath, PuckServicePrefix: "puck@", PuckBasePort: 7777}}
	plan := planrules.Apply(models.Plan{Code: "premium"})
	spec, err := service.prepareRuntimeServer(ctx, CreateInput{Name: "Weekend Arena", DesiredTickRate: 120, MaxPlayers: 10, ServerMode: "public"}, plan)
	if err != nil {
		t.Fatalf("prepareRuntimeServer returned error: %v", err)
	}
	if spec.Config.ServerMode != "public" {
		t.Fatalf("expected runtime spec server mode public, got %q", spec.Config.ServerMode)
	}
	payload, err := parseRuntimeConfig(spec.ConfigJSON)
	if err != nil {
		t.Fatalf("parse runtime config: %v", err)
	}
	if mode, ok := payload["serverMode"].(string); !ok || mode != "public" {
		t.Fatalf("expected rendered runtime payload serverMode=public, got %#v", payload["serverMode"])
	}
}

func TestPrepareRuntimeServerWritesTrainingServerModePerInstance(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "server_template.json")
	template := map[string]any{
		"name":            "Template",
		"maxPlayers":      10,
		"serverTickRate":  120,
		"clientTickRate":  120,
		"targetFrameRate": 120,
		"isPublic":        false,
	}
	templateJSON, err := json.Marshal(template)
	if err != nil {
		t.Fatalf("marshal template: %v", err)
	}
	if err := os.WriteFile(templatePath, templateJSON, 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	service := &Service{cfg: config.Config{PuckConfigDir: tempDir, PuckTemplateConfig: templatePath, PuckServicePrefix: "puck@", PuckBasePort: 7777}}
	plan := planrules.Apply(models.Plan{Code: "premium"})
	spec, err := service.prepareRuntimeServer(ctx, CreateInput{Name: "Training Arena", DesiredTickRate: 120, MaxPlayers: 10, ServerMode: "training"}, plan)
	if err != nil {
		t.Fatalf("prepareRuntimeServer returned error: %v", err)
	}
	if spec.Config.ServerMode != "training" {
		t.Fatalf("expected runtime spec server mode training, got %q", spec.Config.ServerMode)
	}
	payload, err := parseRuntimeConfig(spec.ConfigJSON)
	if err != nil {
		t.Fatalf("parse runtime config: %v", err)
	}
	if mode, ok := payload["serverMode"].(string); !ok || mode != "training" {
		t.Fatalf("expected rendered runtime payload serverMode=training, got %#v", payload["serverMode"])
	}
}
