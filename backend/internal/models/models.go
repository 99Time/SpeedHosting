package models

type AuthenticatedUser struct {
	ID                        int64  `json:"id"`
	Email                     string `json:"email"`
	DisplayName               string `json:"displayName"`
	PlanCode                  string `json:"planCode"`
	Role                      string `json:"role"`
	FirstAcquisitionSource    string `json:"firstAcquisitionSource,omitempty"`
	LatestAcquisitionSource   string `json:"latestAcquisitionSource,omitempty"`
	FirstAcquisitionTimestamp string `json:"firstAcquisitionTimestamp,omitempty"`
}

type AcquisitionAttribution struct {
	Source      string `json:"source"`
	Timestamp   string `json:"timestamp,omitempty"`
	LandingPath string `json:"landingPath,omitempty"`
	FullURL     string `json:"fullUrl,omitempty"`
	SessionID   string `json:"sessionId,omitempty"`
	Route       string `json:"route,omitempty"`
}

type PuckModeration struct {
	Muted      bool    `json:"muted"`
	Banned     bool    `json:"banned"`
	MuteReason *string `json:"muteReason"`
	BanReason  *string `json:"banReason"`
}

type PuckBadge struct {
	Tag      string `json:"tag"`
	Title    string `json:"title"`
	ColorHex string `json:"colorHex"`
}

type RankedTier struct {
	TierKey      string `json:"tierKey"`
	TierName     string `json:"tierName"`
	TierOrder    int    `json:"tierOrder"`
	TierColorHex string `json:"tierColorHex,omitempty"`
	TierTag      string `json:"tierTag,omitempty"`
}

type PuckLinkingState struct {
	Linked                    bool   `json:"linked"`
	DiscordID                 string `json:"discordId,omitempty"`
	LastKnownGameName         string `json:"lastKnownGameName,omitempty"`
	LastKnownGamePlayerNumber string `json:"lastKnownGamePlayerNumber,omitempty"`
}

type PuckPlayerState struct {
	SteamID    string           `json:"steamId"`
	Moderation PuckModeration   `json:"moderation"`
	Badge      PuckBadge        `json:"badge"`
	Linking    PuckLinkingState `json:"linking"`
	Ranked     *PuckRankedState `json:"ranked,omitempty"`
}

type PuckRankedState struct {
	MMR          int        `json:"mmr"`
	RankPosition int        `json:"rankPosition"`
	Tier         RankedTier `json:"tier"`
}

type PuckModerationCommand struct {
	SteamID         string `json:"steamId"`
	DurationSeconds int    `json:"durationSeconds"`
	Reason          string `json:"reason"`
	IssuedBy        string `json:"issuedBy"`
	Source          string `json:"source"`
}

type PuckModerationWriteResult struct {
	OK              bool   `json:"ok"`
	SteamID         string `json:"steamId"`
	PunishmentType  string `json:"punishmentType"`
	DurationSeconds int    `json:"durationSeconds"`
	ExpiresAt       string `json:"expiresAt"`
}

type RankedLeaderboardEntry struct {
	Rank                      int        `json:"rank"`
	SteamID                   string     `json:"steamId"`
	DiscordID                 string     `json:"discordId,omitempty"`
	LinkedDiscordID           string     `json:"linkedDiscordId,omitempty"`
	LinkedDiscordDisplay      string     `json:"linkedDiscordDisplay,omitempty"`
	LastKnownGameName         string     `json:"lastKnownGameName,omitempty"`
	LastKnownGamePlayerNumber string     `json:"lastKnownGamePlayerNumber,omitempty"`
	DisplayName               string     `json:"displayName"`
	MMR                       int        `json:"mmr"`
	Goals                     int        `json:"goals"`
	Assists                   int        `json:"assists"`
	SecondaryAssists          int        `json:"secondaryAssists"`
	Wins                      int        `json:"wins"`
	Losses                    int        `json:"losses"`
	StarPoints                int        `json:"starPoints"`
	WinStreak                 int        `json:"winStreak"`
	Tier                      RankedTier `json:"tier"`
}

type RankedPlayer struct {
	SteamID                   string     `json:"steamId"`
	DiscordID                 string     `json:"discordId,omitempty"`
	LinkedDiscordDisplay      string     `json:"linkedDiscordDisplay,omitempty"`
	LastKnownGameName         string     `json:"lastKnownGameName,omitempty"`
	LastKnownGamePlayerNumber string     `json:"lastKnownGamePlayerNumber,omitempty"`
	DisplayName               string     `json:"displayName"`
	MMR                       int        `json:"mmr"`
	Goals                     int        `json:"goals"`
	Assists                   int        `json:"assists"`
	SecondaryAssists          int        `json:"secondaryAssists"`
	Wins                      int        `json:"wins"`
	Losses                    int        `json:"losses"`
	StarPoints                int        `json:"starPoints"`
	WinStreak                 int        `json:"winStreak"`
	RankPosition              int        `json:"rankPosition"`
	Tier                      RankedTier `json:"tier"`
}

type RankedLinkRequestResult struct {
	OK               bool   `json:"ok"`
	Code             string `json:"code,omitempty"`
	ExpiresInSeconds int    `json:"expiresInSeconds,omitempty"`
	AlreadyOpen      bool   `json:"alreadyOpen,omitempty"`
	AlreadyLinked    bool   `json:"alreadyLinked,omitempty"`
	SteamID          string `json:"steamId,omitempty"`
}

type RankedLinkStatus struct {
	Linked           bool   `json:"linked"`
	SteamID          string `json:"steamId,omitempty"`
	Pending          bool   `json:"pending"`
	Code             string `json:"code,omitempty"`
	ExpiresInSeconds int    `json:"expiresInSeconds,omitempty"`
	ChannelID        string `json:"channelId,omitempty"`
}

type RankedLinkCompleteResult struct {
	OK        bool                  `json:"ok"`
	DiscordID string                `json:"discordId"`
	SteamID   string                `json:"steamId"`
	Identity  *RankedLinkedIdentity `json:"identity,omitempty"`
	Player    *RankedPlayer         `json:"player,omitempty"`
}

type RankedLinkedIdentity struct {
	DiscordID                 string `json:"discordId"`
	SteamID                   string `json:"steamId"`
	DiscordDisplay            string `json:"discordDisplay,omitempty"`
	LastKnownGameName         string `json:"lastKnownGameName,omitempty"`
	LastKnownGamePlayerNumber string `json:"lastKnownGamePlayerNumber,omitempty"`
}

type RankedMatchResult struct {
	MatchID     int64                     `json:"matchId"`
	CreatedAt   string                    `json:"createdAt"`
	ServerName  string                    `json:"serverName,omitempty"`
	ServerMode  string                    `json:"serverMode,omitempty"`
	WinningTeam string                    `json:"winningTeam,omitempty"`
	BlueScore   *int                      `json:"blueScore,omitempty"`
	RedScore    *int                      `json:"redScore,omitempty"`
	Summary     string                    `json:"summary,omitempty"`
	MVP         *RankedMatchResultPlayer  `json:"mvp,omitempty"`
	Players     []RankedMatchResultPlayer `json:"players"`
}

type RankedMatchResultPlayer struct {
	SteamID                   string      `json:"steamId"`
	DiscordID                 string      `json:"discordId,omitempty"`
	LinkedDiscordDisplay      string      `json:"linkedDiscordDisplay,omitempty"`
	LastKnownGameName         string      `json:"lastKnownGameName,omitempty"`
	LastKnownGamePlayerNumber string      `json:"lastKnownGamePlayerNumber,omitempty"`
	Tier                      *RankedTier `json:"tier,omitempty"`
	Goals                     int         `json:"goals"`
	Assists                   int         `json:"assists"`
	SecondaryAssists          *int        `json:"secondaryAssists,omitempty"`
	MMRBefore                 *int        `json:"mmrBefore,omitempty"`
	MMRAfter                  *int        `json:"mmrAfter,omitempty"`
	MMRDelta                  *int        `json:"mmrDelta,omitempty"`
	Team                      string      `json:"team,omitempty"`
	IsMVP                     *bool       `json:"isMvp,omitempty"`
	Won                       *bool       `json:"won,omitempty"`
}

type RankedMatchResultsResponse struct {
	Results []RankedMatchResult `json:"results"`
}

type Plan struct {
	ID                      int64  `json:"id"`
	Code                    string `json:"code"`
	Name                    string `json:"name"`
	MaxServers              int    `json:"maxServers"`
	MaxTickRate             int    `json:"maxTickRate"`
	MaxAdmins               int    `json:"maxAdmins"`
	MaxAdminSteamIDs        int    `json:"maxAdminSteamIds"`
	AllowCustomMods         bool   `json:"allowCustomMods"`
	AllowAdvancedConfig     bool   `json:"allowAdvancedConfig"`
	MaxUserConfigurableMods int    `json:"maxUserConfigurableMods"`
	AllowSpeedRankeds       bool   `json:"allowSpeedRankeds"`
	PremiumFeatureAccess    bool   `json:"premiumFeatureAccess"`
}

type ServerConfigMod struct {
	WorkshopID     string `json:"workshopId"`
	Enabled        bool   `json:"enabled"`
	ClientRequired bool   `json:"clientRequired"`
}

type ServerConfig struct {
	MaxPlayers        int               `json:"maxPlayers"`
	Password          string            `json:"password"`
	VOIPEnabled       bool              `json:"voipEnabled"`
	AdminSteamIDs     []string          `json:"adminSteamIds"`
	IsPublic          bool              `json:"isPublic"`
	ServerMode        string            `json:"serverMode,omitempty"`
	ReloadBannedIDs   bool              `json:"reloadBannedIDs"`
	UsePuckBannedIDs  bool              `json:"usePuckBannedIDs"`
	PrintMetrics      bool              `json:"printMetrics"`
	StartPaused       bool              `json:"startPaused"`
	AllowVoting       bool              `json:"allowVoting"`
	KickTimeout       int               `json:"kickTimeout"`
	SleepTimeout      int               `json:"sleepTimeout"`
	JoinMidMatchDelay int               `json:"joinMidMatchDelay"`
	TargetFrameRate   int               `json:"targetFrameRate"`
	ServerTickRate    int               `json:"serverTickRate"`
	ClientTickRate    int               `json:"clientTickRate"`
	Warmup            int               `json:"warmup"`
	FaceOff           int               `json:"faceOff"`
	Playing           int               `json:"playing"`
	BlueScore         int               `json:"blueScore"`
	RedScore          int               `json:"redScore"`
	Replay            int               `json:"replay"`
	PeriodOver        int               `json:"periodOver"`
	GameOver          int               `json:"gameOver"`
	Mods              []ServerConfigMod `json:"mods"`
}

type Server struct {
	ID              int64        `json:"id"`
	OwnerID         int64        `json:"ownerId"`
	Name            string       `json:"name"`
	Slug            string       `json:"slug"`
	Region          string       `json:"region"`
	ConfigFilePath  string       `json:"configFilePath"`
	ServiceName     string       `json:"serviceName"`
	Status          string       `json:"status"`
	DesiredTickRate int          `json:"desiredTickRate"`
	MaxPlayers      int          `json:"maxPlayers"`
	PlayerCount     int          `json:"playerCount"`
	ProcessState    string       `json:"processState"`
	LastActionError string       `json:"lastActionError,omitempty"`
	Config          ServerConfig `json:"config"`
}

type DashboardSummary struct {
	ServerCount   int `json:"serverCount"`
	ActiveServers int `json:"activeServers"`
	TotalPlayers  int `json:"totalPlayers"`
	MaxServers    int `json:"maxServers"`
	MaxTickRate   int `json:"maxTickRate"`
}

type DashboardOverview struct {
	User    AuthenticatedUser `json:"user"`
	Plan    Plan              `json:"plan"`
	Summary DashboardSummary  `json:"summary"`
	Servers []Server          `json:"servers"`
}

type UpdateEntry struct {
	ID               string `json:"id,omitempty"`
	Title            string `json:"title"`
	ShortDescription string `json:"short_description"`
	Content          string `json:"content"`
	Tag              string `json:"tag"`
	CreatedAt        string `json:"created_at"`
	Icon             string `json:"icon,omitempty"`
}

type UpdatesResponse struct {
	Updates []UpdateEntry `json:"updates"`
}

type AdminUserSummary struct {
	ID                      int64  `json:"id"`
	Email                   string `json:"email"`
	DisplayName             string `json:"displayName"`
	PlanCode                string `json:"planCode"`
	Role                    string `json:"role"`
	ServerCount             int    `json:"serverCount"`
	FirstAcquisitionSource  string `json:"firstAcquisitionSource,omitempty"`
	LatestAcquisitionSource string `json:"latestAcquisitionSource,omitempty"`
}

type AdminServerSummary struct {
	ID              int64        `json:"id"`
	OwnerID         int64        `json:"ownerId"`
	Name            string       `json:"name"`
	OwnerEmail      string       `json:"ownerEmail"`
	OwnerName       string       `json:"ownerName"`
	PlanCode        string       `json:"planCode"`
	Region          string       `json:"region"`
	Status          string       `json:"status"`
	ConfigFilePath  string       `json:"configFilePath"`
	ServiceName     string       `json:"serviceName"`
	DesiredTickRate int          `json:"desiredTickRate"`
	PlayerCount     int          `json:"playerCount"`
	MaxPlayers      int          `json:"maxPlayers"`
	ProcessState    string       `json:"processState"`
	TickRate        int          `json:"tickRate"`
	LastActionError string       `json:"lastActionError,omitempty"`
	Config          ServerConfig `json:"config"`
}

type AdminOverview struct {
	Users              []AdminUserSummary        `json:"users"`
	Servers            []AdminServerSummary      `json:"servers"`
	Plans              []Plan                    `json:"plans"`
	AttributionSummary []AttributionSourceReport `json:"attributionSummary"`
}

type AttributionSourceReport struct {
	Source                string  `json:"source"`
	LandingViews          int     `json:"landingViews"`
	CTAClicks             int     `json:"ctaClicks"`
	RegisterViews         int     `json:"registerViews"`
	RegisterSubmits       int     `json:"registerSubmits"`
	RegisterSuccesses     int     `json:"registerSuccesses"`
	FirstServerClicks     int     `json:"firstServerClicks"`
	FirstServerCreated    int     `json:"firstServerCreated"`
	ProUpgradeViews       int     `json:"proUpgradeViews"`
	ProUpgradeClicks      int     `json:"proUpgradeClicks"`
	ProUpgradeSuccesses   int     `json:"proUpgradeSuccesses"`
	RegisterConversionPct float64 `json:"registerConversionPct"`
	ServerConversionPct   float64 `json:"serverConversionPct"`
	ProConversionPct      float64 `json:"proConversionPct"`
}
