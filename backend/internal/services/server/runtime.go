package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"speedhosting/backend/internal/models"
	"speedhosting/backend/internal/planrules"
	"speedhosting/backend/internal/servermode"
)

var fileNamePattern = regexp.MustCompile(`[^a-z0-9]+`)

const runtimeCommandTimeout = 20 * time.Second

const hostedTargetFrameRate = 244

type runtimeServerSpec struct {
	ConfigFilePath string
	ServiceName    string
	ConfigJSON     string
	Config         models.ServerConfig
	TemplatePath   string
	AdminMerge     adminSteamIDMergeResult
}

type adminSteamIDMergeResult struct {
	TemplateAdminSteamIDs      []string
	RequestedUserAdminSteamIDs []string
	FinalAdminSteamIDs         []string
	DroppedUserAdminSteamIDs   []string
	DroppedDueToPlanLimit      bool
}

func (s *Service) prepareRuntimeServer(ctx context.Context, input CreateInput, plan models.Plan) (runtimeServerSpec, error) {
	fileSegment := sanitizeFileSegment(input.Name)
	if fileSegment == "" {
		return runtimeServerSpec{}, fmt.Errorf("%w: server name must contain letters or numbers", ErrInvalidServerInput)
	}

	fileName := fmt.Sprintf("server_%s.json", fileSegment)
	configFilePath := filepath.Join(s.cfg.PuckConfigDir, fileName)
	serviceName := s.resolveServiceName(plan.Code, configFilePath, "")

	if _, err := os.Stat(configFilePath); err == nil {
		return runtimeServerSpec{}, fmt.Errorf("%w: %s already exists", ErrDuplicateServerFile, fileName)
	}

	templateConfig, err := s.loadTemplateConfig()
	if err != nil {
		return runtimeServerSpec{}, err
	}

	port, pingPort, err := s.allocatePorts(ctx)
	if err != nil {
		return runtimeServerSpec{}, err
	}

	currentConfig := extractServerConfig(templateConfig)
	desiredConfig := currentConfig
	desiredConfig.MaxPlayers = input.MaxPlayers
	desiredConfig.ServerTickRate = input.DesiredTickRate
	desiredConfig.ClientTickRate = input.DesiredTickRate
	desiredConfig.Password = input.Password
	desiredConfig.AdminSteamIDs = append([]string(nil), input.AdminSteamIDs...)
	desiredConfig.Mods = append([]models.ServerConfigMod(nil), input.Mods...)

	templateAdminSteamIDs := mandatoryTemplateAdminSteamIDs(templateConfig)
	templateBaseMods := normalizeMods(extractServerConfig(templateConfig).Mods)
	normalizedConfig, adminMerge, err := s.applyHostedServerConfig(templateConfig, currentConfig, desiredConfig, plan, templateAdminSteamIDs, templateBaseMods)
	if err != nil {
		return runtimeServerSpec{}, err
	}

	templateConfig["name"] = strings.TrimSpace(input.Name)
	templateConfig["port"] = port
	templateConfig["pingPort"] = pingPort

	renderedConfig, err := renderRuntimeConfig(templateConfig)
	if err != nil {
		return runtimeServerSpec{}, err
	}

	return runtimeServerSpec{
		ConfigFilePath: configFilePath,
		ServiceName:    serviceName,
		ConfigJSON:     renderedConfig,
		Config:         normalizedConfig,
		TemplatePath:   s.cfg.PuckTemplateConfig,
		AdminMerge:     adminMerge,
	}, nil
}

func (s *Service) writeRuntimeConfigFile(configFilePath string, configJSON string) error {
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0o755); err != nil {
		return fmt.Errorf("create puck config directory: %w", err)
	}

	file, err := os.OpenFile(configFilePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%w: %s already exists", ErrDuplicateServerFile, filepath.Base(configFilePath))
		}

		return fmt.Errorf("write runtime config file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(configJSON + "\n"); err != nil {
		return fmt.Errorf("persist runtime config file: %w", err)
	}

	return nil
}

func (s *Service) deleteRuntimeConfigFile(configFilePath string) error {
	if configFilePath == "" {
		return nil
	}

	if err := os.Remove(configFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove runtime config file: %w", err)
	}

	return nil
}

func (s *Service) rewriteRuntimeConfigFile(configFilePath string, configJSON string) error {
	if err := os.WriteFile(configFilePath, []byte(strings.TrimSpace(configJSON)+"\n"), 0o644); err != nil {
		return fmt.Errorf("rewrite runtime config file: %w", err)
	}

	return nil
}

func (s *Service) loadTemplateConfig() (map[string]any, error) {
	templatePath := s.cfg.PuckTemplateConfig
	if _, err := os.Stat(templatePath); err != nil {
		return nil, fmt.Errorf("%w: template config %s is not available", ErrRuntimeTemplate, templatePath)
	}

	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("%w: read template config: %v", ErrRuntimeTemplate, err)
	}

	var template map[string]any
	if err := json.Unmarshal(content, &template); err != nil {
		return nil, fmt.Errorf("%w: parse template config: %v", ErrRuntimeTemplate, err)
	}

	clone, err := cloneTemplateMap(template)
	if err != nil {
		return nil, fmt.Errorf("%w: clone template config: %v", ErrRuntimeTemplate, err)
	}

	return clone, nil
}

func cloneTemplateMap(source map[string]any) (map[string]any, error) {
	encoded, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}

	var clone map[string]any
	if err := json.Unmarshal(encoded, &clone); err != nil {
		return nil, err
	}

	return clone, nil
}

func renderRuntimeConfig(payload map[string]any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return "", fmt.Errorf("encode runtime config: %w", err)
	}

	return strings.TrimSpace(buffer.String()), nil
}

func parseRuntimeConfig(configJSON string) (map[string]any, error) {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON == "" {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(configJSON), &payload); err != nil {
		return nil, fmt.Errorf("parse runtime config: %w", err)
	}

	return payload, nil
}

func extractServerConfig(payload map[string]any) models.ServerConfig {
	resolvedServerMode := resolveServerModeFromRuntimePayload(payload)
	isPublic := resolvedServerMode == servermode.Public

	return models.ServerConfig{
		MaxPlayers:        intFromAnyOrDefault(payload["maxPlayers"], 10),
		Password:          stringFromAny(payload["password"]),
		VOIPEnabled:       boolFromAny(payload["voipEnabled"]),
		AdminSteamIDs:     mergedAdminSteamIDs(payload),
		IsPublic:          isPublic,
		ServerMode:        resolvedServerMode,
		ReloadBannedIDs:   boolFromAny(payload["reloadBannedIDs"]),
		UsePuckBannedIDs:  boolFromAny(payload["usePuckBannedIDs"]),
		PrintMetrics:      boolFromAny(payload["printMetrics"]),
		StartPaused:       boolFromAny(payload["startPaused"]),
		AllowVoting:       boolFromAny(payload["allowVoting"]),
		KickTimeout:       intFromAnyOrDefault(payload["kickTimeout"], 1800),
		SleepTimeout:      intFromAnyOrDefault(payload["sleepTimeout"], 900),
		JoinMidMatchDelay: intFromAnyOrDefault(payload["joinMidMatchDelay"], 5),
		TargetFrameRate:   intFromAnyOrDefault(payload["targetFrameRate"], 244),
		ServerTickRate:    intFromAnyOrDefault(payload["serverTickRate"], 120),
		ClientTickRate:    intFromAnyOrDefault(payload["clientTickRate"], 120),
		Warmup:            intFromAnyOrDefault(payload["warmup"], 600),
		FaceOff:           intFromAnyOrDefault(payload["faceOff"], 3),
		Playing:           intFromAnyOrDefault(payload["playing"], 200),
		BlueScore:         intFromAnyOrDefault(payload["blueScore"], 5),
		RedScore:          intFromAnyOrDefault(payload["redScore"], 5),
		Replay:            intFromAnyOrDefault(payload["replay"], 10),
		PeriodOver:        intFromAnyOrDefault(payload["periodOver"], 15),
		GameOver:          intFromAnyOrDefault(payload["gameOver"], 15),
		Mods:              modsFromAny(payload["mods"]),
	}
}

func mandatoryTemplateAdminSteamIDs(payload map[string]any) []string {
	values := make([]string, 0)
	seen := make(map[string]struct{})
	for _, value := range mergedAdminSteamIDs(payload) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		values = append(values, trimmed)
	}

	return values
}

func mergedAdminSteamIDs(payload map[string]any) []string {
	values := make([]string, 0)
	seen := make(map[string]struct{})
	for _, key := range []string{"adminSteamIds", "adminSteamIDs"} {
		for _, value := range stringListFromAny(payload[key]) {
			if _, exists := seen[value]; exists {
				continue
			}

			seen[value] = struct{}{}
			values = append(values, value)
		}
	}

	return values
}

func (s *Service) applyHostedServerConfig(payload map[string]any, currentConfig models.ServerConfig, desiredConfig models.ServerConfig, plan models.Plan, templateAdminSteamIDs []string, templateBaseMods []models.ServerConfigMod) (models.ServerConfig, adminSteamIDMergeResult, error) {
	normalizedConfig := currentConfig
	planCode := strings.ToLower(strings.TrimSpace(plan.Code))
	advancedSettingsAllowed := plan.AllowAdvancedConfig
	requestedTickRate := desiredConfig.ServerTickRate
	requestedAdminSteamIDs := sanitizeRequestedAdminSteamIDs(desiredConfig.AdminSteamIDs)
	requestedModIDs := workshopIDs(normalizeMods(desiredConfig.Mods))

	if desiredConfig.MaxPlayers > 0 {
		normalizedConfig.MaxPlayers = desiredConfig.MaxPlayers
	}
	if normalizedConfig.MaxPlayers <= 0 {
		normalizedConfig.MaxPlayers = 10
	}

	normalizedConfig.Password = strings.TrimSpace(desiredConfig.Password)

	requestedUserAdminSteamIDs := filterUserAdminSteamIDs(desiredConfig.AdminSteamIDs, templateAdminSteamIDs)
	currentUserAdminSteamIDs := filterUserAdminSteamIDs(currentConfig.AdminSteamIDs, templateAdminSteamIDs)
	if len(requestedUserAdminSteamIDs) > plan.MaxAdmins {
		if sameStringSlices(requestedUserAdminSteamIDs, currentUserAdminSteamIDs) {
			requestedUserAdminSteamIDs = requestedUserAdminSteamIDs[:plan.MaxAdmins]
		} else {
			reason := fmt.Sprintf("max admin steam ids is %d", plan.MaxAdmins)
			s.logf("[plan-enforcement] plan_code=%s requested_tick_rate=%d requested_admin_steam_ids=%v requested_mod_ids=%v advanced_settings_allowed=%t result=rejected reason=%q", planCode, requestedTickRate, requestedAdminSteamIDs, requestedModIDs, advancedSettingsAllowed, reason)
			return models.ServerConfig{}, adminSteamIDMergeResult{}, fmt.Errorf("%w: %s", ErrPlanLimitReached, reason)
		}
	}

	adminMerge := mergeAdminSteamIDs(templateAdminSteamIDs, requestedUserAdminSteamIDs, plan.MaxAdmins)
	normalizedConfig.AdminSteamIDs = adminMerge.FinalAdminSteamIDs

	if plan.AllowAdvancedConfig {
		normalizedConfig.VOIPEnabled = desiredConfig.VOIPEnabled
		normalizedConfig.ServerMode = normalizeHostedServerMode(desiredConfig)
		normalizedConfig.IsPublic = normalizedConfig.ServerMode == servermode.Public
		normalizedConfig.ReloadBannedIDs = desiredConfig.ReloadBannedIDs
		normalizedConfig.UsePuckBannedIDs = desiredConfig.UsePuckBannedIDs
		normalizedConfig.PrintMetrics = desiredConfig.PrintMetrics
		normalizedConfig.StartPaused = desiredConfig.StartPaused
		normalizedConfig.AllowVoting = desiredConfig.AllowVoting
		normalizedConfig.KickTimeout = desiredConfig.KickTimeout
		normalizedConfig.SleepTimeout = desiredConfig.SleepTimeout
		normalizedConfig.JoinMidMatchDelay = desiredConfig.JoinMidMatchDelay
		normalizedConfig.ServerTickRate = desiredConfig.ServerTickRate
		normalizedConfig.ClientTickRate = desiredConfig.ClientTickRate
		normalizedConfig.Warmup = desiredConfig.Warmup
		normalizedConfig.FaceOff = desiredConfig.FaceOff
		normalizedConfig.Playing = desiredConfig.Playing
		normalizedConfig.BlueScore = desiredConfig.BlueScore
		normalizedConfig.RedScore = desiredConfig.RedScore
		normalizedConfig.Replay = desiredConfig.Replay
		normalizedConfig.PeriodOver = desiredConfig.PeriodOver
		normalizedConfig.GameOver = desiredConfig.GameOver
	}

	normalizedConfig.ServerMode = normalizeHostedServerMode(normalizedConfig)
	normalizedConfig.IsPublic = normalizedConfig.ServerMode == servermode.Public

	switch planCode {
	case "free":
		normalizedConfig.TargetFrameRate = plan.MaxTickRate
		normalizedConfig.ServerTickRate = plan.MaxTickRate
		normalizedConfig.ClientTickRate = plan.MaxTickRate
	case "pro", "premium":
		normalizedConfig.TargetFrameRate = hostedTargetFrameRate
	}

	if planCode == "free" {
		normalizedConfig.ServerTickRate = plan.MaxTickRate
		normalizedConfig.ClientTickRate = plan.MaxTickRate
	} else {
		if normalizedConfig.ServerTickRate <= 0 {
			normalizedConfig.ServerTickRate = plan.MaxTickRate
		}
		if normalizedConfig.ClientTickRate <= 0 {
			normalizedConfig.ClientTickRate = normalizedConfig.ServerTickRate
		}
	}

	if normalizedConfig.ServerTickRate > plan.MaxTickRate || normalizedConfig.ClientTickRate > plan.MaxTickRate {
		reason := fmt.Sprintf("max tick rate is %d", plan.MaxTickRate)
		s.logf("[plan-enforcement] plan_code=%s requested_tick_rate=%d requested_admin_steam_ids=%v requested_mod_ids=%v advanced_settings_allowed=%t result=rejected reason=%q", planCode, requestedTickRate, requestedAdminSteamIDs, requestedModIDs, advancedSettingsAllowed, reason)
		return models.ServerConfig{}, adminMerge, fmt.Errorf("%w: %s", ErrPlanLimitReached, reason)
	}

	baseTemplateMods := sanitizeBaseTemplateModsForPlan(templateBaseMods, plan)
	requestedUserMods := filterUserConfigurableMods(desiredConfig.Mods, baseTemplateMods)
	currentUserMods := filterUserConfigurableMods(currentConfig.Mods, baseTemplateMods)
	strippedSpeedRankeds := containsWorkshopID(templateBaseMods, planrules.SpeedRankedsWorkshopID) && !plan.AllowSpeedRankeds
	rejectedModIDs := make([]string, 0)

	if !plan.AllowSpeedRankeds && containsWorkshopID(requestedUserMods, planrules.SpeedRankedsWorkshopID) && !sameModSlices(requestedUserMods, currentUserMods) {
		reason := fmt.Sprintf("mod %s is not available on %s", planrules.SpeedRankedsWorkshopID, strings.Title(planCode))
		rejectedModIDs = append(rejectedModIDs, planrules.SpeedRankedsWorkshopID)
		s.logf("[plan-enforcement] plan_code=%s requested_tick_rate=%d requested_admin_steam_ids=%v requested_mod_ids=%v rejected_mod_ids=%v stripped_speedrankeds=%t advanced_settings_allowed=%t result=rejected reason=%q", planCode, requestedTickRate, requestedAdminSteamIDs, requestedModIDs, rejectedModIDs, strippedSpeedRankeds, advancedSettingsAllowed, reason)
		return models.ServerConfig{}, adminMerge, fmt.Errorf("%w: %s", ErrPlanLimitReached, reason)
	}

	finalUserMods := requestedUserMods
	if !plan.AllowCustomMods {
		if len(requestedUserMods) > 0 && !sameModSlices(requestedUserMods, currentUserMods) {
			rejectedModIDs = workshopIDs(requestedUserMods)
			reason := "custom mods are not available on this plan"
			s.logf("[plan-enforcement] plan_code=%s requested_tick_rate=%d requested_admin_steam_ids=%v requested_mod_ids=%v rejected_mod_ids=%v stripped_speedrankeds=%t advanced_settings_allowed=%t result=rejected reason=%q", planCode, requestedTickRate, requestedAdminSteamIDs, requestedModIDs, rejectedModIDs, strippedSpeedRankeds, advancedSettingsAllowed, reason)
			return models.ServerConfig{}, adminMerge, fmt.Errorf("%w: %s", ErrPlanLimitReached, reason)
		}
		finalUserMods = nil
	} else if len(finalUserMods) > plan.MaxUserConfigurableMods {
		if sameModSlices(finalUserMods, currentUserMods) {
			rejectedModIDs = workshopIDs(finalUserMods[plan.MaxUserConfigurableMods:])
			finalUserMods = finalUserMods[:plan.MaxUserConfigurableMods]
		} else {
			reason := fmt.Sprintf("max configurable mods is %d", plan.MaxUserConfigurableMods)
			rejectedModIDs = workshopIDs(finalUserMods[plan.MaxUserConfigurableMods:])
			s.logf("[plan-enforcement] plan_code=%s requested_tick_rate=%d requested_admin_steam_ids=%v requested_mod_ids=%v rejected_mod_ids=%v stripped_speedrankeds=%t advanced_settings_allowed=%t result=rejected reason=%q", planCode, requestedTickRate, requestedAdminSteamIDs, requestedModIDs, rejectedModIDs, strippedSpeedRankeds, advancedSettingsAllowed, reason)
			return models.ServerConfig{}, adminMerge, fmt.Errorf("%w: %s", ErrPlanLimitReached, reason)
		}
	}

	normalizedConfig.Mods = mergePlanMods(baseTemplateMods, finalUserMods)

	payload["maxPlayers"] = normalizedConfig.MaxPlayers
	payload["password"] = normalizedConfig.Password
	delete(payload, "adminSteamIDs")
	payload["adminSteamIds"] = normalizedConfig.AdminSteamIDs
	payload["targetFrameRate"] = normalizedConfig.TargetFrameRate
	payload["serverTickRate"] = normalizedConfig.ServerTickRate
	payload["clientTickRate"] = normalizedConfig.ClientTickRate

	if plan.AllowAdvancedConfig {
		payload["voipEnabled"] = normalizedConfig.VOIPEnabled
		payload["isPublic"] = normalizedConfig.IsPublic
		payload["serverMode"] = normalizedConfig.ServerMode
		payload["reloadBannedIDs"] = normalizedConfig.ReloadBannedIDs
		payload["usePuckBannedIDs"] = normalizedConfig.UsePuckBannedIDs
		payload["printMetrics"] = normalizedConfig.PrintMetrics
		payload["startPaused"] = normalizedConfig.StartPaused
		payload["allowVoting"] = normalizedConfig.AllowVoting
		payload["kickTimeout"] = normalizedConfig.KickTimeout
		payload["sleepTimeout"] = normalizedConfig.SleepTimeout
		payload["joinMidMatchDelay"] = normalizedConfig.JoinMidMatchDelay
		payload["warmup"] = normalizedConfig.Warmup
		payload["faceOff"] = normalizedConfig.FaceOff
		payload["playing"] = normalizedConfig.Playing
		payload["blueScore"] = normalizedConfig.BlueScore
		payload["redScore"] = normalizedConfig.RedScore
		payload["replay"] = normalizedConfig.Replay
		payload["periodOver"] = normalizedConfig.PeriodOver
		payload["gameOver"] = normalizedConfig.GameOver
	}
	if _, exists := payload["serverMode"]; !exists {
		payload["serverMode"] = normalizedConfig.ServerMode
	}

	payload["mods"] = modsToAny(normalizedConfig.Mods)
	payload["speedRankedsEnabled"] = containsWorkshopID(normalizedConfig.Mods, planrules.SpeedRankedsWorkshopID)

	s.logf("[plan-enforcement] plan_code=%s requested_tick_rate=%d requested_admin_steam_ids=%v requested_mod_ids=%v final_tick_rate=%d final_admin_steam_ids=%v final_mod_ids=%v rejected_mod_ids=%v stripped_speedrankeds=%t advanced_settings_allowed=%t result=ok", planCode, requestedTickRate, requestedAdminSteamIDs, requestedModIDs, normalizedConfig.ServerTickRate, normalizedConfig.AdminSteamIDs, workshopIDs(normalizedConfig.Mods), rejectedModIDs, strippedSpeedRankeds, advancedSettingsAllowed)

	return normalizedConfig, adminMerge, nil
}

func sanitizeFileSegment(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = fileNamePattern.ReplaceAllString(normalized, "_")
	normalized = strings.Trim(normalized, "_")
	return normalized
}

func (s *Service) allocatePorts(_ context.Context) (int, int, error) {
	usedPorts := make(map[int]bool)
	for _, reservedPort := range s.cfg.PuckReservedPorts {
		usedPorts[reservedPort] = true
	}

	configFiles, err := filepath.Glob(filepath.Join(s.cfg.PuckConfigDir, "server*.json"))
	if err != nil {
		return 0, 0, fmt.Errorf("inspect puck configs: %w", err)
	}

	for _, configFile := range configFiles {
		content, err := os.ReadFile(configFile)
		if err != nil {
			continue
		}

		var payload map[string]any
		if err := json.Unmarshal(content, &payload); err != nil {
			continue
		}

		if port, ok := intFromAny(payload["port"]); ok {
			usedPorts[port] = true
		}

		if pingPort, ok := intFromAny(payload["pingPort"]); ok {
			usedPorts[pingPort] = true
		}
	}

	port := s.cfg.PuckBasePort
	for usedPorts[port] || usedPorts[port+1] {
		port += 2
	}

	return port, port + 1, nil
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed, true
		}
	}

	return 0, false
}

func intFromAnyOrDefault(value any, fallback int) int {
	if parsed, ok := intFromAny(value); ok {
		return parsed
	}

	return fallback
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		return normalized == "true" || normalized == "1" || normalized == "yes" || normalized == "on"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	}

	return false
}

func stringFromAny(value any) string {
	if typed, ok := value.(string); ok {
		return strings.TrimSpace(typed)
	}

	return ""
}

func resolveServerModeFromRuntimePayload(payload map[string]any) string {
	legacyIsPublic := legacyIsPublicFromRuntimePayload(payload)
	resolution := servermode.Resolve(firstStringFromAny(payload["serverMode"], payload["ServerMode"], payload["server_mode"]), legacyIsPublic)
	return resolution.Normalized
}

func normalizeHostedServerMode(config models.ServerConfig) string {
	legacyIsPublic := config.IsPublic
	resolution := servermode.Resolve(config.ServerMode, &legacyIsPublic)
	return resolution.Normalized
}

func legacyIsPublicFromRuntimePayload(payload map[string]any) *bool {
	for _, key := range []string{"isPublic", "IsPublic", "is_public"} {
		value, exists := payload[key]
		if !exists {
			continue
		}
		resolved := boolFromAny(value)
		return &resolved
	}

	return nil
}

func firstStringFromAny(values ...any) string {
	for _, value := range values {
		text := stringFromAny(value)
		if text != "" {
			return text
		}
	}

	return ""
}

func stringListFromAny(value any) []string {
	items := make([]string, 0)
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
	case []any:
		for _, item := range typed {
			trimmed := strings.TrimSpace(fmt.Sprint(item))
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
	case string:
		for _, part := range strings.Split(typed, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
	}

	return items
}

func modsFromAny(value any) []models.ServerConfigMod {
	entries, ok := value.([]any)
	if !ok {
		return nil
	}

	mods := make([]models.ServerConfigMod, 0, len(entries))
	for _, entry := range entries {
		payload, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		workshopID := strings.TrimSpace(stringFromAny(payload["workshopId"]))
		if workshopID == "" {
			workshopID = strings.TrimSpace(stringFromAny(payload["id"]))
		}
		if workshopID == "" {
			continue
		}

		mods = append(mods, models.ServerConfigMod{
			WorkshopID:     workshopID,
			Enabled:        boolFromAny(payload["enabled"]),
			ClientRequired: boolFromAny(payload["clientRequired"]),
		})
	}

	return mods
}

func modsToAny(mods []models.ServerConfigMod) []map[string]any {
	result := make([]map[string]any, 0, len(mods))
	for _, mod := range mods {
		if strings.TrimSpace(mod.WorkshopID) == "" {
			continue
		}

		result = append(result, map[string]any{
			"workshopId":     strings.TrimSpace(mod.WorkshopID),
			"enabled":        mod.Enabled,
			"clientRequired": mod.ClientRequired,
		})
	}

	return result
}

func mergeAdminSteamIDs(templateAdminSteamIDs []string, requestedUserAdminSteamIDs []string, maxExtraAdmins int) adminSteamIDMergeResult {
	if maxExtraAdmins < 0 {
		maxExtraAdmins = 0
	}

	result := adminSteamIDMergeResult{
		TemplateAdminSteamIDs:      append([]string(nil), templateAdminSteamIDs...),
		RequestedUserAdminSteamIDs: sanitizeRequestedAdminSteamIDs(requestedUserAdminSteamIDs),
		FinalAdminSteamIDs:         make([]string, 0, len(templateAdminSteamIDs)+len(requestedUserAdminSteamIDs)),
		DroppedUserAdminSteamIDs:   make([]string, 0),
	}

	seen := make(map[string]struct{}, len(templateAdminSteamIDs)+len(requestedUserAdminSteamIDs))
	for _, value := range result.TemplateAdminSteamIDs {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result.FinalAdminSteamIDs = append(result.FinalAdminSteamIDs, trimmed)
	}

	extraCount := 0
	for _, value := range result.RequestedUserAdminSteamIDs {
		if _, exists := seen[value]; exists {
			continue
		}
		if extraCount >= maxExtraAdmins {
			result.DroppedDueToPlanLimit = true
			result.DroppedUserAdminSteamIDs = append(result.DroppedUserAdminSteamIDs, value)
			continue
		}

		seen[value] = struct{}{}
		result.FinalAdminSteamIDs = append(result.FinalAdminSteamIDs, value)
		extraCount++
	}

	return result
}

func sanitizeRequestedAdminSteamIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := normalizeAdminSteamID(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}

func normalizeAdminSteamID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	buffer := strings.Builder{}
	for _, character := range trimmed {
		if character >= '0' && character <= '9' {
			buffer.WriteRune(character)
		}
	}

	normalized := buffer.String()
	if len(normalized) < 5 || len(normalized) > 32 {
		return ""
	}

	return normalized
}

func normalizeMods(mods []models.ServerConfigMod) []models.ServerConfigMod {
	result := make([]models.ServerConfigMod, 0, len(mods))
	seen := make(map[string]struct{}, len(mods))
	for _, mod := range mods {
		workshopID := strings.TrimSpace(mod.WorkshopID)
		if workshopID == "" {
			continue
		}
		if _, exists := seen[workshopID]; exists {
			continue
		}
		seen[workshopID] = struct{}{}

		result = append(result, models.ServerConfigMod{
			WorkshopID:     workshopID,
			Enabled:        mod.Enabled,
			ClientRequired: mod.ClientRequired,
		})
	}

	return result
}

func filterUserAdminSteamIDs(values []string, templateAdminSteamIDs []string) []string {
	templateSet := make(map[string]struct{}, len(templateAdminSteamIDs))
	for _, value := range sanitizeRequestedAdminSteamIDs(templateAdminSteamIDs) {
		templateSet[value] = struct{}{}
	}

	filtered := make([]string, 0, len(values))
	for _, value := range sanitizeRequestedAdminSteamIDs(values) {
		if _, exists := templateSet[value]; exists {
			continue
		}
		filtered = append(filtered, value)
	}

	return filtered
}

func sanitizeBaseTemplateModsForPlan(templateBaseMods []models.ServerConfigMod, plan models.Plan) []models.ServerConfigMod {
	filtered := make([]models.ServerConfigMod, 0, len(templateBaseMods))
	for _, mod := range normalizeMods(templateBaseMods) {
		if !plan.AllowSpeedRankeds && mod.WorkshopID == planrules.SpeedRankedsWorkshopID {
			continue
		}
		filtered = append(filtered, mod)
	}
	return filtered
}

func filterUserConfigurableMods(mods []models.ServerConfigMod, baseTemplateMods []models.ServerConfigMod) []models.ServerConfigMod {
	baseIDs := make(map[string]struct{}, len(baseTemplateMods))
	for _, mod := range normalizeMods(baseTemplateMods) {
		baseIDs[mod.WorkshopID] = struct{}{}
	}

	filtered := make([]models.ServerConfigMod, 0, len(mods))
	for _, mod := range normalizeMods(mods) {
		if _, exists := baseIDs[mod.WorkshopID]; exists {
			continue
		}
		filtered = append(filtered, mod)
	}

	return filtered
}

func mergePlanMods(baseTemplateMods []models.ServerConfigMod, userMods []models.ServerConfigMod) []models.ServerConfigMod {
	merged := make([]models.ServerConfigMod, 0, len(baseTemplateMods)+len(userMods))
	merged = append(merged, normalizeMods(baseTemplateMods)...)
	merged = append(merged, filterUserConfigurableMods(userMods, baseTemplateMods)...)
	return normalizeMods(merged)
}

func sameModSlices(left []models.ServerConfigMod, right []models.ServerConfigMod) bool {
	left = normalizeMods(left)
	right = normalizeMods(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].WorkshopID != right[index].WorkshopID {
			return false
		}
	}
	return true
}

func sameStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if strings.TrimSpace(left[index]) != strings.TrimSpace(right[index]) {
			return false
		}
	}
	return true
}

func workshopIDs(mods []models.ServerConfigMod) []string {
	ids := make([]string, 0, len(mods))
	for _, mod := range normalizeMods(mods) {
		ids = append(ids, mod.WorkshopID)
	}
	return ids
}

func containsWorkshopID(mods []models.ServerConfigMod, workshopID string) bool {
	workshopID = strings.TrimSpace(workshopID)
	if workshopID == "" {
		return false
	}
	for _, mod := range normalizeMods(mods) {
		if mod.WorkshopID == workshopID {
			return true
		}
	}
	return false
}

func (s *Service) performRuntimeAction(ctx context.Context, action string, server models.Server, planCode string, serviceName string) error {
	if server.ConfigFilePath == "" || serviceName == "" {
		s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=%s result=error error=%q", server.ID, server.Name, planCode, serviceName, action, "missing runtime metadata for this server")
		return fmt.Errorf("missing runtime metadata for this server")
	}

	if _, err := os.Stat(server.ConfigFilePath); err != nil {
		s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=%s result=error error=%q", server.ID, server.Name, planCode, serviceName, action, fmt.Sprintf("runtime config file is missing: %s", server.ConfigFilePath))
		return fmt.Errorf("runtime config file is missing: %s", server.ConfigFilePath)
	}

	s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=%s result=starting", server.ID, server.Name, planCode, serviceName, action)

	commandCtx, cancel := context.WithTimeout(ctx, runtimeCommandTimeout)
	defer cancel()

	command := exec.CommandContext(commandCtx, s.cfg.PuckSystemctlPath, action, serviceName)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=%s result=error error=%q", server.ID, server.Name, planCode, serviceName, action, message)
		return fmt.Errorf("%s %s failed: %s", action, serviceName, message)
	}

	s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=%s result=ok", server.ID, server.Name, planCode, serviceName, action)

	return nil
}

func (s *Service) cleanupRuntimeServer(ctx context.Context, server models.Server, planCode string, serviceName string) error {
	serviceNames := uniqueServiceNames(serviceName, server.ServiceName)
	for _, candidate := range serviceNames {
		if _, err := s.runSystemctl(ctx, "stop", candidate); err != nil && !isIgnorableSystemctlError(err) {
			return fmt.Errorf("stop %s before delete: %w", candidate, err)
		}

		_, _ = s.runSystemctl(ctx, "disable", candidate)
		_, _ = s.runSystemctl(ctx, "reset-failed", candidate)
	}

	s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=cleanup result=ok", server.ID, server.Name, planCode, serviceName)

	if err := s.deleteRuntimeConfigFile(server.ConfigFilePath); err != nil {
		s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=cleanup result=error error=%q", server.ID, server.Name, planCode, serviceName, err.Error())
		return err
	}

	return nil
}

func (s *Service) runSystemctl(ctx context.Context, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, runtimeCommandTimeout)
	defer cancel()

	command := exec.CommandContext(commandCtx, s.cfg.PuckSystemctlPath, args...)
	output, err := command.CombinedOutput()
	message := strings.TrimSpace(string(output))
	if err != nil {
		if message == "" {
			message = err.Error()
		}
		return message, fmt.Errorf("%s", message)
	}

	return message, nil
}

func isIgnorableSystemctlError(err error) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "not loaded") ||
		strings.Contains(message, "could not be found") ||
		strings.Contains(message, "no such file") ||
		strings.Contains(message, "not be found")
}

func (s *Service) currentServerStatus(ctx context.Context, server models.Server, planCode string, serviceName string) string {
	if serviceName == "" {
		s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=status result=unknown", server.ID, server.Name, planCode, serviceName)
		return "unknown"
	}

	status := s.readSystemctlStatus(ctx, serviceName)
	if status == "stopped" {
		for _, legacyServiceName := range uniqueServiceNames(server.ServiceName) {
			if legacyServiceName == serviceName {
				continue
			}

			legacyStatus := s.readSystemctlStatus(ctx, legacyServiceName)
			if legacyStatus == "running" || legacyStatus == "error" {
				s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=status result=%s", server.ID, server.Name, planCode, legacyServiceName, legacyStatus)
				return legacyStatus
			}
		}
	}

	s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=status result=%s", server.ID, server.Name, planCode, serviceName, status)
	return status
}

func (s *Service) readSystemctlStatus(ctx context.Context, serviceName string) string {
	if serviceName == "" {
		return "unknown"
	}

	commandCtx, cancel := context.WithTimeout(ctx, runtimeCommandTimeout)
	defer cancel()

	command := exec.CommandContext(commandCtx, s.cfg.PuckSystemctlPath, "is-active", serviceName)
	output, err := command.Output()
	if err != nil {
		return "stopped"
	}

	status := strings.ToLower(strings.TrimSpace(string(output)))
	switch status {
	case "active", "activating", "reloading":
		return "running"
	case "failed":
		return "error"
	default:
		return "stopped"
	}
}

func (s *Service) resolveServiceName(planCode string, configFilePath string, storedServiceName string) string {
	instance := runtimeServiceInstance(configFilePath, storedServiceName)
	if instance == "" {
		return strings.TrimSpace(storedServiceName)
	}

	switch strings.ToLower(strings.TrimSpace(planCode)) {
	case "free":
		return "puck-free@" + instance
	case "pro":
		return "puck-pro@" + instance
	case "premium":
		return "puck-premium@" + instance
	default:
		storedServiceName = strings.TrimSpace(storedServiceName)
		if storedServiceName != "" {
			return storedServiceName
		}

		prefix := strings.TrimSpace(s.cfg.PuckServicePrefix)
		if prefix == "" {
			prefix = "puck@"
		}

		return prefix + instance
	}
}

func runtimeServiceInstance(configFilePath string, storedServiceName string) string {
	configFilePath = strings.TrimSpace(configFilePath)
	if configFilePath != "" {
		fileName := strings.TrimSpace(filepath.Base(configFilePath))
		if fileName != "" {
			extension := filepath.Ext(fileName)
			return strings.TrimSuffix(fileName, extension)
		}
	}

	storedServiceName = strings.TrimSpace(storedServiceName)
	storedServiceName = strings.TrimSuffix(storedServiceName, ".service")
	if atIndex := strings.Index(storedServiceName, "@"); atIndex >= 0 && atIndex < len(storedServiceName)-1 {
		return storedServiceName[atIndex+1:]
	}

	return storedServiceName
}

func uniqueServiceNames(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}

	return result
}
