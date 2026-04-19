package puck

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"speedhosting/backend/internal/models"
	"speedhosting/backend/internal/ranktier"
	"speedhosting/backend/internal/servermode"
	rankedservice "speedhosting/backend/internal/services/ranked"
)

const maxPunishmentDuration = 365 * 24 * time.Hour

type Service struct {
	db           *sql.DB
	logger       *log.Logger
	ranked       rankedLookup
	mu           sync.RWMutex
	matchReports []json.RawMessage
}

type rankedLookup interface {
	Rank(ctx context.Context, steamID string, discordID string, query string) (models.RankedPlayer, error)
}

type discordLinkProjection struct {
	DiscordID                 string
	DiscordDisplay            string
	LastKnownGameName         string
	LastKnownGamePlayerNumber string
}

type normalizedMatchResult struct {
	ServerName  string
	ServerMode  string
	ModeInput   string
	ModeReason  string
	WinningTeam string
	BlueScore   *int
	RedScore    *int
	Summary     string
	MVPSteamID  string
	Players     []normalizedMatchPlayer
}

type normalizedMatchPlayer struct {
	SteamID                 string
	DisplayName             string
	Team                    string
	ExcludedFromMMR         bool
	Goals                   int
	Assists                 int
	SecondaryAssists        int
	SecondaryAssistsPresent bool
	MMRBefore               *int
	MMRAfter                *int
	MMRDelta                *int
	IsMVP                   bool
	IsMVPPresent            bool
	Won                     bool
	ResultKnown             bool
	ShotsProvided           bool
	SavesProvided           bool
}

func NewService(db *sql.DB, logger *log.Logger, ranked rankedLookup) *Service {
	return &Service{
		db:           db,
		logger:       logger,
		ranked:       ranked,
		matchReports: make([]json.RawMessage, 0, 16),
	}
}

func (s *Service) GetPlayerState(ctx context.Context, steamID string) (models.PuckPlayerState, error) {
	steamID = normalizeSteamID(steamID)
	if err := validateSteamID(steamID); err != nil {
		return models.PuckPlayerState{}, fmt.Errorf("steam id is required")
	}

	if err := s.expirePunishments(ctx, steamID, time.Now().UTC()); err != nil {
		return models.PuckPlayerState{}, err
	}

	state := defaultPlayerState(steamID)

	rows, err := s.db.QueryContext(ctx, `
		SELECT punishment_type, COALESCE(reason, '')
		FROM puck_punishments
		WHERE steam_id = ? AND active = 1 AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY created_at DESC, id DESC`, steamID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return models.PuckPlayerState{}, fmt.Errorf("query active puck punishments: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var punishmentType string
		var reason string
		if err := rows.Scan(&punishmentType, &reason); err != nil {
			return models.PuckPlayerState{}, fmt.Errorf("scan active puck punishment: %w", err)
		}

		reason = strings.TrimSpace(reason)
		switch strings.TrimSpace(punishmentType) {
		case "mute":
			if !state.Moderation.Muted {
				state.Moderation.Muted = true
				if reason != "" {
					reasonCopy := reason
					state.Moderation.MuteReason = &reasonCopy
				}
			}
		case "tempban", "ban":
			if !state.Moderation.Banned {
				state.Moderation.Banned = true
				if reason != "" {
					reasonCopy := reason
					state.Moderation.BanReason = &reasonCopy
				}
			}
		}
	}

	if err := rows.Err(); err != nil {
		return models.PuckPlayerState{}, fmt.Errorf("iterate active puck punishments: %w", err)
	}

	s.logf("[puck] player-state link lookup steam=%s", maskIdentifier(steamID))
	link, linked, err := s.lookupDiscordLinkBySteamID(ctx, steamID)
	if err != nil {
		return models.PuckPlayerState{}, err
	}
	s.logf("[puck] player-state permanent link steam=%s linked=%t", maskIdentifier(steamID), linked)
	state.Linking.Linked = linked
	if linked {
		state.Linking.DiscordID = link.DiscordID
		state.Linking.LastKnownGameName = link.LastKnownGameName
		state.Linking.LastKnownGamePlayerNumber = link.LastKnownGamePlayerNumber
	}
	if s.ranked != nil {
		rankedPlayer, rankedErr := s.ranked.Rank(ctx, steamID, "", "")
		switch {
		case rankedErr == nil:
			state.Ranked = &models.PuckRankedState{
				MMR:          rankedPlayer.MMR,
				RankPosition: rankedPlayer.RankPosition,
				Tier:         rankedPlayer.Tier,
			}
			s.logf("[puck] player-state ranked steam=%s rank=%d mmr=%d tier=%s color=%s tag=%s", maskIdentifier(steamID), rankedPlayer.RankPosition, rankedPlayer.MMR, rankedPlayer.Tier.TierKey, rankedPlayer.Tier.TierColorHex, rankedPlayer.Tier.TierTag)
		case rankedservice.IsNotFound(rankedErr):
			s.logf("[puck] player-state ranked steam=%s result=not_found", maskIdentifier(steamID))
		case errors.Is(rankedErr, sql.ErrNoRows):
			s.logf("[puck] player-state ranked steam=%s result=not_found", maskIdentifier(steamID))
		default:
			s.logf("[puck] player-state ranked steam=%s err=%v", maskIdentifier(steamID), rankedErr)
			return models.PuckPlayerState{}, rankedErr
		}
	}
	if state.Ranked != nil {
		s.logf("[puck] player-state projection steam=%s linked=%t ranked=true mmr=%d tier=%s color=%s tag=%s api_game_player_number=%q", maskIdentifier(steamID), state.Linking.Linked, state.Ranked.MMR, state.Ranked.Tier.TierKey, state.Ranked.Tier.TierColorHex, state.Ranked.Tier.TierTag, state.Linking.LastKnownGamePlayerNumber)
	} else {
		s.logf("[puck] player-state projection steam=%s linked=%t ranked=false api_game_player_number=%q", maskIdentifier(steamID), state.Linking.Linked, state.Linking.LastKnownGamePlayerNumber)
	}

	return state, nil
}

func (s *Service) Mute(ctx context.Context, input models.PuckModerationCommand) (models.PuckModerationWriteResult, error) {
	return s.applyTimedPunishment(ctx, "mute", input)
}

func (s *Service) TempBan(ctx context.Context, input models.PuckModerationCommand) (models.PuckModerationWriteResult, error) {
	return s.applyTimedPunishment(ctx, "tempban", input)
}

func (s *Service) ReportMatch(ctx context.Context, payload json.RawMessage) error {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	cloned := append(json.RawMessage(nil), payload...)

	s.mu.Lock()
	s.matchReports = append(s.matchReports, cloned)
	if len(s.matchReports) > 100 {
		s.matchReports = append([]json.RawMessage(nil), s.matchReports[len(s.matchReports)-100:]...)
	}
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Printf("[puck] received match result payload: %s", string(cloned))
	}

	normalizedMatch, err := normalizeMatchReportPayload(cloned)
	if err != nil {
		return err
	}
	s.logf("[puck] match ingest mode incoming_server_mode=%q normalized_server_mode=%s reason=%s", normalizedMatch.ModeInput, normalizedMatch.ServerMode, normalizedMatch.ModeReason)
	if !servermode.IsCompetitive(normalizedMatch.ServerMode) {
		s.logf("[puck] match ingest decision official_competitive=false final_action=skip_official_progression final_persistence=skip_official_match_result server_mode=%s reason=%s server_name=%q", normalizedMatch.ServerMode, normalizedMatch.ModeReason, normalizedMatch.ServerName)
		return nil
	}
	s.logf("[puck] match ingest decision official_competitive=true final_action=apply_official_progression final_persistence=store_official_match_result server_mode=%s reason=%s server_name=%q", normalizedMatch.ServerMode, normalizedMatch.ModeReason, normalizedMatch.ServerName)
	if len(normalizedMatch.Players) == 0 {
		s.logf("[puck] match ingest players=0 result=no_authoritative_player_stats")
		return nil
	}

	for _, player := range normalizedMatch.Players {
		s.logf("[puck] match ingest normalized steam=%s goals=%d assists=%d secondary_assists=%d secondary_present=%t incoming_is_mvp_present=%t incoming_is_mvp=%t result_known=%t won=%t shots_ignored=%t saves_ignored=%t", maskIdentifier(player.SteamID), player.Goals, player.Assists, player.SecondaryAssists, player.SecondaryAssistsPresent, player.IsMVPPresent, player.IsMVP, player.ResultKnown, player.Won, player.ShotsProvided, player.SavesProvided)
	}
	s.logf("[puck] match ingest summary incoming_server_name=%q winning_team=%q incoming_blue_score=%s incoming_red_score=%s mvp=%s summary=%q", normalizedMatch.ServerName, normalizedMatch.WinningTeam, optionalIntLog(normalizedMatch.BlueScore), optionalIntLog(normalizedMatch.RedScore), maskIdentifier(normalizedMatch.MVPSteamID), normalizedMatch.Summary)

	if err := s.persistMatchArtifacts(ctx, normalizedMatch); err != nil {
		return err
	}

	return nil
}

func (s *Service) RecentMatchResults(ctx context.Context, limit int) ([]models.RankedMatchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT payload_json
		FROM ranked_match_results
		WHERE server_mode = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?`, servermode.Competitive, limit)
	if err != nil {
		return nil, fmt.Errorf("query ranked match results: %w", err)
	}
	defer rows.Close()

	results := make([]models.RankedMatchResult, 0, limit)
	for rows.Next() {
		var payloadText string
		if err := rows.Scan(&payloadText); err != nil {
			return nil, fmt.Errorf("scan ranked match result: %w", err)
		}

		var result models.RankedMatchResult
		if err := json.Unmarshal([]byte(payloadText), &result); err != nil {
			return nil, fmt.Errorf("decode ranked match result payload: %w", err)
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ranked match results: %w", err)
	}

	return results, nil
}

func (s *Service) LatestMatchResult(ctx context.Context) (models.RankedMatchResult, error) {
	s.logf("[puck] latest match lookup branch=start")

	var payloadText string
	err := s.db.QueryRowContext(ctx, `
		SELECT payload_json
		FROM ranked_match_results
		WHERE server_mode = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, servermode.Competitive).Scan(&payloadText)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logf("[puck] latest match lookup branch=no_match_found")
			return models.RankedMatchResult{}, sql.ErrNoRows
		}
		return models.RankedMatchResult{}, fmt.Errorf("query latest ranked match result: %w", err)
	}

	var result models.RankedMatchResult
	if err := json.Unmarshal([]byte(payloadText), &result); err != nil {
		return models.RankedMatchResult{}, fmt.Errorf("decode latest ranked match result payload: %w", err)
	}

	s.logf("[puck] latest endpoint payload_shape match_id=%d server_name=%q winning_team=%q blue_score=%s red_score=%s players=%d mvp=%s player_mvp_values=%s", result.MatchID, result.ServerName, result.WinningTeam, optionalIntLog(result.BlueScore), optionalIntLog(result.RedScore), len(result.Players), matchResultMVPLog(result.MVP), matchResultPlayerMVPLog(result.Players))
	s.logf("[puck] latest match lookup branch=returned_latest match_id=%d", result.MatchID)
	return result, nil
}

func (s *Service) persistMatchArtifacts(ctx context.Context, match normalizedMatchResult) error {
	if len(match.Players) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ranked match artifact tx: %w", err)
	}
	defer tx.Rollback()

	for _, player := range match.Players {
		if err := ensureRankedProfileForMatchTx(ctx, tx, player); err != nil {
			return err
		}
		if err := incrementRankedProfileStatsTx(ctx, tx, player); err != nil {
			return err
		}

		persistedGoals, persistedAssists, persistedSecondaryAssists, persistedWins, persistedLosses, loadErr := loadPersistedRankedStatTotalsTx(ctx, tx, player.SteamID)
		if loadErr != nil {
			return loadErr
		}
		s.logf("[puck] match ingest persisted steam=%s goals=%d assists=%d secondary_assists=%d wins=%d losses=%d", maskIdentifier(player.SteamID), persistedGoals, persistedAssists, persistedSecondaryAssists, persistedWins, persistedLosses)
	}

	result, err := s.buildEnrichedMatchResultTx(ctx, tx, match)
	if err != nil {
		return err
	}
	matchID, err := storeMatchResultTx(ctx, tx, result)
	if err != nil {
		return err
	}
	result.MatchID = matchID
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal enriched ranked match result for log: %w", err)
	}
	s.logf("[puck] match ingest enriched match_id=%d payload=%s", matchID, string(resultJSON))

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ranked match artifact tx: %w", err)
	}

	return nil
}

func (s *Service) applyTimedPunishment(ctx context.Context, punishmentType string, input models.PuckModerationCommand) (models.PuckModerationWriteResult, error) {
	steamID := normalizeSteamID(input.SteamID)
	if err := validateSteamID(steamID); err != nil {
		return models.PuckModerationWriteResult{}, fmt.Errorf("steam id is required")
	}

	duration := time.Duration(input.DurationSeconds) * time.Second
	if input.DurationSeconds <= 0 {
		return models.PuckModerationWriteResult{}, fmt.Errorf("durationSeconds must be greater than zero")
	}
	if duration > maxPunishmentDuration {
		return models.PuckModerationWriteResult{}, fmt.Errorf("durationSeconds exceeds maximum allowed duration")
	}

	reason := sanitizeReason(input.Reason)
	issuedBy := sanitizeActor(input.IssuedBy, "server_admin")
	source := sanitizeActor(input.Source, "puck_mod")
	now := time.Now().UTC()
	expiresAt := now.Add(duration)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.PuckModerationWriteResult{}, fmt.Errorf("begin puck punishment tx: %w", err)
	}
	defer tx.Rollback()

	if err := expirePunishmentsTx(ctx, tx, steamID, now); err != nil {
		return models.PuckModerationWriteResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE puck_punishments
		SET active = 0
		WHERE steam_id = ? AND punishment_type = ? AND active = 1`, steamID, punishmentType); err != nil {
		return models.PuckModerationWriteResult{}, fmt.Errorf("deactivate active puck punishment: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO puck_punishments (steam_id, punishment_type, reason, issued_by, source, created_at, expires_at, active)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)`,
		steamID,
		punishmentType,
		nullableString(reason),
		nullableString(issuedBy),
		nullableString(source),
		now.Format(time.RFC3339),
		expiresAt.Format(time.RFC3339),
	); err != nil {
		return models.PuckModerationWriteResult{}, fmt.Errorf("insert puck punishment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.PuckModerationWriteResult{}, fmt.Errorf("commit puck punishment: %w", err)
	}

	s.logf("[puck] moderation applied issuer=%s target=%s type=%s duration=%ds result=ok", issuedBy, steamID, punishmentType, input.DurationSeconds)

	return models.PuckModerationWriteResult{
		OK:              true,
		SteamID:         steamID,
		PunishmentType:  punishmentType,
		DurationSeconds: input.DurationSeconds,
		ExpiresAt:       expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *Service) expirePunishments(ctx context.Context, steamID string, now time.Time) error {
	return expirePunishmentsDB(ctx, s.db, steamID, now)
}

func expirePunishmentsDB(ctx context.Context, db execContext, steamID string, now time.Time) error {
	if err := expirePunishmentsExec(ctx, db, steamID, now); err != nil {
		return err
	}

	return nil
}

func expirePunishmentsTx(ctx context.Context, tx *sql.Tx, steamID string, now time.Time) error {
	return expirePunishmentsExec(ctx, tx, steamID, now)
}

type execContext interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func expirePunishmentsExec(ctx context.Context, executor execContext, steamID string, now time.Time) error {
	if _, err := executor.ExecContext(ctx, `
		UPDATE puck_punishments
		SET active = 0
		WHERE steam_id = ? AND active = 1 AND expires_at IS NOT NULL AND expires_at <= ?`, steamID, now.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("expire puck punishments: %w", err)
	}

	return nil
}

func validateSteamID(steamID string) error {
	if steamID == "" {
		return fmt.Errorf("steam id is required")
	}
	if len(steamID) < 5 || len(steamID) > 32 {
		return fmt.Errorf("steam id must be between 5 and 32 digits")
	}
	return nil
}

func normalizeSteamID(value string) string {
	value = strings.TrimSpace(value)
	buffer := strings.Builder{}
	for _, character := range value {
		if character >= '0' && character <= '9' {
			buffer.WriteRune(character)
		}
	}
	return buffer.String()
}

func sanitizeReason(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 256 {
		value = value[:256]
	}
	return value
}

func sanitizeActor(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func sanitizePlayerDisplayName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func normalizeMatchReportPayload(payload json.RawMessage) (normalizedMatchResult, error) {
	var document map[string]any
	if err := json.Unmarshal(payload, &document); err != nil {
		return normalizedMatchResult{}, fmt.Errorf("parse puck match result payload: %w", err)
	}
	legacyIsPublic, legacyIsPublicPresent := firstBoolValue(document["isPublic"], document["IsPublic"], document["is_public"])
	var legacyIsPublicPointer *bool
	if legacyIsPublicPresent {
		legacyIsPublicPointer = &legacyIsPublic
	}
	modeResolution := servermode.Resolve(firstString(document["serverMode"], document["ServerMode"], document["server_mode"]), legacyIsPublicPointer)

	normalized := normalizedMatchResult{
		ServerName:  sanitizeServerName(firstString(document["serverName"], document["ServerName"], document["server_name"], document["Server"])),
		ServerMode:  modeResolution.Normalized,
		ModeInput:   modeResolution.Input,
		ModeReason:  modeResolution.Reason,
		WinningTeam: normalizeMatchTeam(firstString(document["winningTeam"], document["WinningTeam"], document["winner"], document["Winner"], document["winning_team"], document["winningSide"], document["WinningSide"])),
		Summary:     sanitizeMatchSummary(firstString(document["summary"], document["Summary"], document["matchSummary"], document["MatchSummary"], document["headline"], document["Headline"])),
		MVPSteamID:  firstNormalizedSteamID(document["mvpSteamId"], document["mvpSteamID"], document["MvpSteamId"], document["MVPSteamID"], document["mvpPlayerSteamId"], document["MvpPlayerSteamId"], document["mvpPlayerId"], document["MvpPlayerId"]),
	}
	if blueScore, ok := firstPresentIntValue(document["blueScore"], document["BlueScore"], document["blue_score"]); ok {
		normalized.BlueScore = intPointer(maxInt(blueScore, 0))
	}
	if redScore, ok := firstPresentIntValue(document["redScore"], document["RedScore"], document["red_score"]); ok {
		normalized.RedScore = intPointer(maxInt(redScore, 0))
	}
	if scoreMap, ok := firstMap(document["score"], document["Score"]); ok {
		if normalized.BlueScore == nil {
			if blueScore, ok := firstPresentIntValue(scoreMap["blue"], scoreMap["Blue"], scoreMap["home"], scoreMap["Home"], scoreMap["team1"], scoreMap["Team1"]); ok {
				normalized.BlueScore = intPointer(maxInt(blueScore, 0))
			}
		}
		if normalized.RedScore == nil {
			if redScore, ok := firstPresentIntValue(scoreMap["red"], scoreMap["Red"], scoreMap["away"], scoreMap["Away"], scoreMap["team2"], scoreMap["Team2"]); ok {
				normalized.RedScore = intPointer(maxInt(redScore, 0))
			}
		}
	}

	playerItems := extractMatchPlayerItems(document)
	normalized.Players = make([]normalizedMatchPlayer, 0, len(playerItems))
	for _, item := range playerItems {
		playerMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		player, ok := normalizeMatchPlayer(playerMap)
		if !ok {
			continue
		}
		if !player.ResultKnown && normalized.WinningTeam != "" && player.Team != "" {
			player.ResultKnown = true
			player.Won = player.Team == normalized.WinningTeam
		}
		if normalized.MVPSteamID == "" && player.IsMVP {
			normalized.MVPSteamID = player.SteamID
		}
		normalized.Players = append(normalized.Players, player)
	}

	if normalized.WinningTeam == "" && normalized.BlueScore != nil && normalized.RedScore != nil {
		switch {
		case *normalized.BlueScore > *normalized.RedScore:
			normalized.WinningTeam = "blue"
		case *normalized.RedScore > *normalized.BlueScore:
			normalized.WinningTeam = "red"
		}
	}

	return normalized, nil
}

func extractMatchPlayerItems(document map[string]any) []any {
	for _, key := range []string{"players", "Players", "playerStats", "PlayerStats", "player_stats"} {
		if values, ok := anySlice(document[key]); ok {
			return values
		}
	}
	if stats, ok := firstMap(document["stats"], document["Stats"]); ok {
		for _, key := range []string{"players", "Players", "playerStats", "PlayerStats", "player_stats"} {
			if values, ok := anySlice(stats[key]); ok {
				return values
			}
		}
	}
	return nil
}

func normalizeMatchPlayer(player map[string]any) (normalizedMatchPlayer, bool) {
	excludedFromMMR, excludedPresent := firstBoolValue(
		player["excludedFromMmr"],
		player["ExcludedFromMmr"],
		player["excluded_from_mmr"],
		player["excludeFromMmr"],
		player["ExcludeFromMmr"],
	)
	if excludedPresent && excludedFromMMR {
		return normalizedMatchPlayer{}, false
	}

	steamID := firstNormalizedSteamID(player["steamId"], player["SteamId"], player["steamID"], player["SteamID"], player["playerId"], player["PlayerId"], player["playerID"], player["PlayerID"], player["id"], player["Id"])
	if steamID == "" {
		return normalizedMatchPlayer{}, false
	}

	goals, _ := intFromAny(firstValue(player, "goals", "Goals"))
	assists, _ := intFromAny(firstValue(player, "assists", "Assists"))
	secondaryAssists, secondaryPresent := firstPresentInt(player, "secondaryAssists", "SecondaryAssists", "secondary_assists", "secondaryAssist", "SecondaryAssist", "secondAssist", "SecondAssist", "secondAssists", "SecondAssists", "secondaryAssistCount", "SecondaryAssistCount")
	mmrBefore, mmrBeforePresent := firstPresentInt(player, "mmrBefore", "MmrBefore", "mmr_before", "beforeMMR", "BeforeMMR", "preMatchMMR", "PreMatchMMR", "pre_match_mmr")
	mmrAfter, mmrAfterPresent := firstPresentInt(player, "mmrAfter", "MmrAfter", "mmr_after", "afterMMR", "AfterMMR", "postMatchMMR", "PostMatchMMR", "post_match_mmr")
	mmrDelta, mmrDeltaPresent := firstPresentInt(player, "mmrDelta", "MmrDelta", "mmr_delta", "deltaMMR", "DeltaMMR", "mmrChange", "MmrChange", "mmr_change")
	_, shotsProvided := firstPresentInt(player, "shots", "Shots", "shotCount", "ShotCount")
	_, savesProvided := firstPresentInt(player, "saves", "Saves", "saveCount", "SaveCount")
	resultWon, resultKnown := matchResultFromPlayer(player)
	if !mmrDeltaPresent && mmrBeforePresent && mmrAfterPresent {
		mmrDelta = mmrAfter - mmrBefore
		mmrDeltaPresent = true
	}

	isMVP, isMVPPresent := firstBoolValueFromMap(player, "mvp", "MVP", "isMVP", "IsMVP", "is_mvp")

	return normalizedMatchPlayer{
		SteamID:                 steamID,
		DisplayName:             sanitizePlayerDisplayName(firstString(player["displayName"], player["DisplayName"], player["playerName"], player["PlayerName"], player["name"], player["Name"])),
		Team:                    normalizeMatchTeam(firstString(player["team"], player["Team"], player["side"], player["Side"], player["squad"], player["Squad"])),
		ExcludedFromMMR:         excludedFromMMR,
		Goals:                   maxInt(goals, 0),
		Assists:                 maxInt(assists, 0),
		SecondaryAssists:        maxInt(secondaryAssists, 0),
		SecondaryAssistsPresent: secondaryPresent,
		MMRBefore:               optionalIntPointer(mmrBefore, mmrBeforePresent),
		MMRAfter:                optionalIntPointer(mmrAfter, mmrAfterPresent),
		MMRDelta:                optionalIntPointer(mmrDelta, mmrDeltaPresent),
		IsMVP:                   isMVP,
		IsMVPPresent:            isMVPPresent,
		Won:                     resultWon,
		ResultKnown:             resultKnown,
		ShotsProvided:           shotsProvided,
		SavesProvided:           savesProvided,
	}, true
}

func ensureRankedProfileForMatchTx(ctx context.Context, tx *sql.Tx, player normalizedMatchPlayer) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO ranked_profiles (steam_id, display_name, mmr, goals, assists, secondary_assists, wins, losses, star_points, win_streak, created_at, updated_at)
		VALUES (?, ?, ?, 0, 0, 0, 0, 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, player.SteamID, nullableString(player.DisplayName), ranktier.InitialMMR); err != nil {
		return fmt.Errorf("ensure ranked profile for match stats: %w", err)
	}
	return nil
}

func incrementRankedProfileStatsTx(ctx context.Context, tx *sql.Tx, player normalizedMatchPlayer) error {
	winIncrement := 0
	lossIncrement := 0
	if player.ResultKnown {
		if player.Won {
			winIncrement = 1
		} else {
			lossIncrement = 1
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE ranked_profiles
		SET display_name = CASE WHEN ? <> '' THEN ? ELSE display_name END,
		    goals = goals + ?,
		    assists = assists + ?,
		    secondary_assists = secondary_assists + ?,
		    wins = wins + ?,
		    losses = losses + ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE steam_id = ?`,
		player.DisplayName,
		player.DisplayName,
		player.Goals,
		player.Assists,
		player.SecondaryAssists,
		winIncrement,
		lossIncrement,
		player.SteamID,
	); err != nil {
		return fmt.Errorf("increment ranked profile stats: %w", err)
	}

	return nil
}

func loadPersistedRankedStatTotalsTx(ctx context.Context, tx *sql.Tx, steamID string) (int, int, int, int, int, error) {
	var goals int
	var assists int
	var secondaryAssists int
	var wins int
	var losses int
	if err := tx.QueryRowContext(ctx, `
		SELECT goals, assists, secondary_assists, wins, losses
		FROM ranked_profiles
		WHERE steam_id = ?`, steamID).Scan(&goals, &assists, &secondaryAssists, &wins, &losses); err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("load persisted ranked stat totals: %w", err)
	}

	return goals, assists, secondaryAssists, wins, losses, nil
}

func (s *Service) buildEnrichedMatchResultTx(ctx context.Context, tx *sql.Tx, match normalizedMatchResult) (models.RankedMatchResult, error) {
	createdAt := time.Now().UTC().Format(time.RFC3339)
	result := models.RankedMatchResult{
		CreatedAt:   createdAt,
		ServerName:  match.ServerName,
		ServerMode:  match.ServerMode,
		WinningTeam: match.WinningTeam,
		BlueScore:   cloneIntPointer(match.BlueScore),
		RedScore:    cloneIntPointer(match.RedScore),
		Summary:     match.Summary,
		Players:     make([]models.RankedMatchResultPlayer, 0, len(match.Players)),
	}

	for _, player := range match.Players {
		link, linked, err := lookupDiscordLinkBySteamIDTx(ctx, tx, player.SteamID)
		if err != nil {
			return models.RankedMatchResult{}, err
		}

		tier, err := resolveMatchPlayerTierTx(ctx, tx, player)
		if err != nil {
			return models.RankedMatchResult{}, err
		}

		resultPlayer := models.RankedMatchResultPlayer{
			SteamID:                   player.SteamID,
			DiscordID:                 link.DiscordID,
			LinkedDiscordDisplay:      safeLinkedDiscordDisplay(link),
			LastKnownGameName:         link.LastKnownGameName,
			LastKnownGamePlayerNumber: link.LastKnownGamePlayerNumber,
			Tier:                      tier,
			Goals:                     player.Goals,
			Assists:                   player.Assists,
			SecondaryAssists:          optionalIntPointer(player.SecondaryAssists, player.SecondaryAssistsPresent),
			MMRBefore:                 cloneIntPointer(player.MMRBefore),
			MMRAfter:                  cloneIntPointer(player.MMRAfter),
			MMRDelta:                  cloneIntPointer(player.MMRDelta),
			Team:                      player.Team,
			IsMVP:                     optionalBoolPointer(player.IsMVP, player.IsMVPPresent),
			Won:                       optionalBoolPointer(player.Won, player.ResultKnown),
		}
		if resultPlayer.LinkedDiscordDisplay == "" && linked {
			resultPlayer.LinkedDiscordDisplay = maskIdentifier(link.DiscordID)
		}

		s.logf("[puck] match result identity steam=%s linked=%t discord=%s game_name=%q game_player_number=%q tier=%s mmr_after=%s final_is_mvp=%t", maskIdentifier(player.SteamID), linked, maskIdentifier(link.DiscordID), link.LastKnownGameName, link.LastKnownGamePlayerNumber, tierKeyForLog(resultPlayer.Tier), optionalIntLog(resultPlayer.MMRAfter), boolPointerValue(resultPlayer.IsMVP))

		result.Players = append(result.Players, resultPlayer)
		if match.MVPSteamID != "" && player.SteamID == match.MVPSteamID {
			mvpCopy := resultPlayer
			result.MVP = &mvpCopy
		}
	}

	s.logf("[puck] match result enriched final_server_name=%q final_blue_score=%s final_red_score=%s final_winning_team=%q final_mvp=%s player_mvp_values=%s", result.ServerName, optionalIntLog(result.BlueScore), optionalIntLog(result.RedScore), result.WinningTeam, matchResultMVPLog(result.MVP), matchResultPlayerMVPLog(result.Players))

	return result, nil
}

func storeMatchResultTx(ctx context.Context, tx *sql.Tx, result models.RankedMatchResult) (int64, error) {
	payloadJSON, err := json.Marshal(result)
	if err != nil {
		return 0, fmt.Errorf("marshal ranked match result: %w", err)
	}

	insertResult, err := tx.ExecContext(ctx, `
		INSERT INTO ranked_match_results (server_mode, winning_team, payload_json, created_at)
		VALUES (?, ?, ?, ?)`, result.ServerMode, nullableString(result.WinningTeam), string(payloadJSON), result.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert ranked match result: %w", err)
	}

	matchID, err := insertResult.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("resolve ranked match result id: %w", err)
	}

	updatedResult := result
	updatedResult.MatchID = matchID
	updatedPayloadJSON, err := json.Marshal(updatedResult)
	if err != nil {
		return 0, fmt.Errorf("marshal ranked match result with id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE ranked_match_results
		SET payload_json = ?
		WHERE id = ?`, string(updatedPayloadJSON), matchID); err != nil {
		return 0, fmt.Errorf("update ranked match result payload id: %w", err)
	}

	return matchID, nil
}

func resolveMatchPlayerTierTx(ctx context.Context, tx *sql.Tx, player normalizedMatchPlayer) (*models.RankedTier, error) {
	if player.MMRAfter != nil {
		tier := ranktier.Resolve(*player.MMRAfter)
		return &tier, nil
	}

	var currentMMR int
	if err := tx.QueryRowContext(ctx, `
		SELECT mmr
		FROM ranked_profiles
		WHERE steam_id = ?`, player.SteamID).Scan(&currentMMR); err != nil {
		return nil, fmt.Errorf("load ranked profile mmr for match result: %w", err)
	}

	tier := ranktier.Resolve(currentMMR)
	return &tier, nil
}

func firstNormalizedSteamID(values ...any) string {
	for _, value := range values {
		text := strings.TrimSpace(fmt.Sprint(value))
		text = normalizeSteamID(text)
		if text != "" {
			return text
		}
	}
	return ""
}

func firstString(values ...any) string {
	for _, value := range values {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func anySlice(value any) ([]any, bool) {
	values, ok := value.([]any)
	return values, ok
}

func firstMap(values ...any) (map[string]any, bool) {
	for _, value := range values {
		typed, ok := value.(map[string]any)
		if ok {
			return typed, true
		}
	}
	return nil, false
}

func firstValue(player map[string]any, keys ...string) any {
	for _, key := range keys {
		value, ok := player[key]
		if ok {
			return value
		}
	}
	return nil
}

func firstPresentIntValue(values ...any) (int, bool) {
	for _, value := range values {
		parsed, ok := intFromAny(value)
		if ok {
			return parsed, true
		}
	}
	return 0, false
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed), true
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		var parsed int
		_, err := fmt.Sscanf(trimmed, "%d", &parsed)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func maxInt(value int, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}

func intPointer(value int) *int {
	copyValue := value
	return &copyValue
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func optionalIntPointer(value int, present bool) *int {
	if !present {
		return nil
	}
	return intPointer(value)
}

func optionalBoolPointer(value bool, present bool) *bool {
	if !present {
		return nil
	}
	copyValue := value
	return &copyValue
}

func firstPresentInt(player map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := player[key]
		if !ok {
			continue
		}
		parsed, parsedOK := intFromAny(value)
		if !parsedOK {
			return 0, true
		}
		return parsed, true
	}
	return 0, false
}

func firstBool(player map[string]any, keys ...string) bool {
	value, _ := firstBoolValueFromMap(player, keys...)
	return value
}

func firstBoolValue(values ...any) (bool, bool) {
	for _, value := range values {
		switch typed := value.(type) {
		case bool:
			return typed, true
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "true", "1", "yes", "y":
				return true, true
			case "false", "0", "no", "n":
				return false, true
			}
		}
	}
	return false, false
}

func firstBoolValueFromMap(player map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		value, ok := player[key]
		if !ok {
			continue
		}
		parsed, parsedOK := firstBoolValue(value)
		if parsedOK {
			return parsed, true
		}
	}
	return false, false
}

func matchResultFromPlayer(player map[string]any) (bool, bool) {
	if value, ok := firstBoolValueFromMap(player, "won", "Won", "isWinner", "IsWinner", "win", "Win"); ok {
		return value, true
	}
	for _, key := range []string{"result", "Result", "matchResult", "MatchResult"} {
		value := strings.ToLower(strings.TrimSpace(fmt.Sprint(player[key])))
		switch value {
		case "win", "won", "winner", "victory":
			return true, true
		case "loss", "lost", "lose", "defeat":
			return false, true
		}
	}
	return false, false
}

func normalizeMatchTeam(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "blue", "home", "team1", "1":
		return "blue"
	case "red", "away", "team2", "2":
		return "red"
	default:
		return value
	}
}

func sanitizeMatchSummary(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > 512 {
		return value[:512]
	}
	return value
}

func sanitizeServerName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > 128 {
		return value[:128]
	}
	return value
}

func (s *Service) lookupDiscordLinkBySteamID(ctx context.Context, steamID string) (discordLinkProjection, bool, error) {
	return lookupDiscordLinkBySteamIDTx(ctx, s.db, steamID)
}

func lookupDiscordLinkBySteamIDTx(ctx context.Context, queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, steamID string) (discordLinkProjection, bool, error) {
	var link discordLinkProjection
	err := queryer.QueryRowContext(ctx, `
		SELECT discord_id, COALESCE(discord_display, ''), COALESCE(last_known_game_name, ''), COALESCE(last_known_game_player_number, '')
		FROM discord_links
		WHERE steam_id = ?`, steamID).Scan(&link.DiscordID, &link.DiscordDisplay, &link.LastKnownGameName, &link.LastKnownGamePlayerNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			return discordLinkProjection{}, false, nil
		}

		return discordLinkProjection{}, false, fmt.Errorf("lookup discord link by steam id: %w", err)
	}

	link.DiscordID = strings.TrimSpace(link.DiscordID)
	link.DiscordDisplay = strings.TrimSpace(link.DiscordDisplay)
	link.LastKnownGameName = strings.TrimSpace(link.LastKnownGameName)
	link.LastKnownGamePlayerNumber = strings.TrimSpace(link.LastKnownGamePlayerNumber)
	if link.DiscordID == "" {
		return discordLinkProjection{}, false, nil
	}

	return link, true, nil
}

func safeLinkedDiscordDisplay(link discordLinkProjection) string {
	if link.DiscordDisplay != "" {
		return link.DiscordDisplay
	}
	return ""
}

func optionalIntLog(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func tierKeyForLog(tier *models.RankedTier) string {
	if tier == nil {
		return ""
	}
	return tier.TierKey
}

func boolPointerValue(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func matchResultMVPLog(mvp *models.RankedMatchResultPlayer) string {
	if mvp == nil {
		return ""
	}
	return maskIdentifier(mvp.SteamID)
}

func matchResultPlayerMVPLog(players []models.RankedMatchResultPlayer) string {
	if len(players) == 0 {
		return ""
	}
	values := make([]string, 0, len(players))
	for _, player := range players {
		if player.IsMVP == nil {
			continue
		}
		values = append(values, fmt.Sprintf("%s:%t", maskIdentifier(player.SteamID), *player.IsMVP))
	}
	return strings.Join(values, ",")
}

func maskIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4 {
		return value
	}
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}

func defaultPlayerState(steamID string) models.PuckPlayerState {
	return models.PuckPlayerState{
		SteamID: steamID,
		Moderation: models.PuckModeration{
			Muted:      false,
			Banned:     false,
			MuteReason: nil,
			BanReason:  nil,
		},
		Badge: models.PuckBadge{
			Tag:      "",
			Title:    "",
			ColorHex: "#ffffff",
		},
		Linking: models.PuckLinkingState{
			Linked: false,
		},
	}
}
