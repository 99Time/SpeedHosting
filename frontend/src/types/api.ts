export type User = {
  id: number
  email: string
  displayName: string
  planCode: string
  role: string
  firstAcquisitionSource?: string
  latestAcquisitionSource?: string
  firstAcquisitionTimestamp?: string
}

export type AcquisitionPayload = {
  source: string
  timestamp?: string
  landingPath?: string
  fullUrl?: string
  sessionId?: string
  route?: string
}

export type Plan = {
  id: number
  code: string
  name: string
  maxServers: number
  maxTickRate: number
  maxAdmins: number
  maxAdminSteamIds: number
  allowCustomMods: boolean
  allowAdvancedConfig: boolean
  maxUserConfigurableMods: number
  allowSpeedRankeds: boolean
  premiumFeatureAccess: boolean
}

export type ServerConfigMod = {
  workshopId: string
  enabled: boolean
  clientRequired: boolean
}

export type ServerConfig = {
  maxPlayers: number
  password: string
  voipEnabled: boolean
  adminSteamIds: string[]
  isPublic: boolean
  reloadBannedIDs: boolean
  usePuckBannedIDs: boolean
  printMetrics: boolean
  startPaused: boolean
  allowVoting: boolean
  kickTimeout: number
  sleepTimeout: number
  joinMidMatchDelay: number
  targetFrameRate: number
  serverTickRate: number
  clientTickRate: number
  warmup: number
  faceOff: number
  playing: number
  blueScore: number
  redScore: number
  replay: number
  periodOver: number
  gameOver: number
  mods: ServerConfigMod[]
}

export type Server = {
  id: number
  ownerId: number
  name: string
  slug: string
  region: string
  configFilePath: string
  serviceName: string
  status: string
  desiredTickRate: number
  maxPlayers: number
  playerCount: number
  processState: string
  lastActionError: string
  config: ServerConfig
}

export type DashboardSummary = {
  serverCount: number
  activeServers: number
  totalPlayers: number
  maxServers: number
  maxTickRate: number
}

export type DashboardResponse = {
  user: User
  plan: Plan
  summary: DashboardSummary
  servers: Server[]
}

export type UpdateEntry = {
  id?: string
  title: string
  short_description: string
  content: string
  tag: string
  created_at: string
  icon?: string
}

export type UpdatesResponse = {
  updates: UpdateEntry[]
}

export type MeResponse = {
  user: User
}

export type AuthResponse = {
  user: User
}

export type PlanResponse = {
  plan: Plan
  usage: {
    serverCount: number
  }
}

export type ServersResponse = {
  servers: Server[]
}

export type ServerResponse = {
  server: Server
}

export type LogoutResponse = {
  ok: boolean
}

export type AdminUserSummary = {
  id: number
  email: string
  displayName: string
  planCode: string
  role: string
  serverCount: number
  firstAcquisitionSource?: string
  latestAcquisitionSource?: string
}

export type AttributionSourceReport = {
  source: string
  landingViews: number
  ctaClicks: number
  registerViews: number
  registerSubmits: number
  registerSuccesses: number
  firstServerClicks: number
  firstServerCreated: number
  proUpgradeViews: number
  proUpgradeClicks: number
  proUpgradeSuccesses: number
  registerConversionPct: number
  serverConversionPct: number
  proConversionPct: number
}

export type AdminServerSummary = {
  id: number
  ownerId: number
  name: string
  ownerEmail: string
  ownerName: string
  planCode: string
  region: string
  status: string
  configFilePath: string
  serviceName: string
  desiredTickRate: number
  playerCount: number
  maxPlayers: number
  processState: string
  tickRate: number
  lastActionError: string
  config: ServerConfig
}

export type AdminOverviewResponse = {
  users: AdminUserSummary[]
  servers: AdminServerSummary[]
  plans: Plan[]
  attributionSummary: AttributionSourceReport[]
}

export type AdminUserResponse = {
  user: AdminUserSummary
}
