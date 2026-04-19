const knownSourceConfigs = {
  welcome: {
    headline: 'Host your own PUCK server in minutes.',
    subheadline: 'You just found SpeedHosting through a live server. Start free and launch your own without setup friction.',
    contextLine: 'Built for PUCK players and communities who want a smooth first step into hosting.',
    trustLine: 'Simple dashboard, fast setup, account-owned servers.',
    urgency: 'Start free today and upgrade later when you want more control.',
    primaryCta: 'Start Free',
    secondaryCta: 'See Plans',
    registerTitle: 'You are one step away from your own PUCK server.',
    registerSubtitle: 'Start free and get your first server online quickly.',
    loginTitle: 'Get back to your PUCK server fast.',
    loginSubtitle: 'Manage your servers, plan, and next session from one place.',
    dashboardEmptyTitle: 'Create your first PUCK server now.',
    dashboardEmptyCopy: 'Free is the fastest way to go from player to server owner.',
  },
  postmatch: {
    headline: 'Enjoyed this server? Launch your own next.',
    subheadline: 'You already played on a live PUCK server. Use the same polished hosting flow for your own matches, scrims, or community nights.',
    contextLine: 'Free gets you started. Pro unlocks mods, advanced settings, and more control for 8 EUR/month.',
    trustLine: 'Built for communities that care about server quality and clean admin control.',
    urgency: 'The fastest path from a good match to your own server is already here.',
    primaryCta: 'Host My PUCK Server',
    secondaryCta: 'How It Works',
    registerTitle: 'Turn that good match into your own server.',
    registerSubtitle: 'Create your account, start free, and host your next session your way.',
    loginTitle: 'Come back and launch your own.',
    loginSubtitle: 'Your dashboard is ready for your next PUCK server.',
    dashboardEmptyTitle: 'Launch the server you wanted after that match.',
    dashboardEmptyCopy: 'Use Free to get started, then move to Pro for mods and deeper control.',
  },
  chat: {
    headline: 'Want your own PUCK server?',
    subheadline: 'SpeedHosting makes it easy to start free, launch fast, and manage everything from one polished dashboard.',
    contextLine: 'Low-friction hosting for PUCK players who just want their own server.',
    trustLine: 'Fast setup, simple controls, and a clean upgrade path when you need more.',
    urgency: 'Start free now and decide later if you want Pro.',
    primaryCta: 'Start Free',
    secondaryCta: 'See Plans',
    registerTitle: 'Create your account and launch fast.',
    registerSubtitle: 'Start with Free and build from there.',
    loginTitle: 'Sign in and open your hosting dashboard.',
    loginSubtitle: 'Your PUCK server tools are waiting.',
    dashboardEmptyTitle: 'Create a server in a few clicks.',
    dashboardEmptyCopy: 'The Free plan is enough to get you live quickly.',
  },
  hostcommand: {
    headline: 'Launch your PUCK server now.',
    subheadline: 'You used the host command, so we will keep this simple: create your account, launch your server, and upgrade later only if you need more control.',
    contextLine: 'Highest-intent path: direct, fast, and built for people ready to host.',
    trustLine: 'Built for PUCK server owners who want the shortest path to a live server.',
    urgency: 'Start free now. Pro is 8 EUR/month when you want mods and advanced settings.',
    primaryCta: 'Launch My Server',
    secondaryCta: 'See Plans',
    registerTitle: 'Create your account and go straight to server setup.',
    registerSubtitle: 'This path is designed for players ready to host right now.',
    loginTitle: 'Sign in and go straight to My Servers.',
    loginSubtitle: 'The shortest path to a live PUCK server starts there.',
    dashboardEmptyTitle: 'Create your server right now.',
    dashboardEmptyCopy: 'You came here to host. Start with Free, then upgrade only when your community needs more.',
  },
} as const

const aliasMap: Record<string, string> = {
  welcome: 'welcome',
  postmatch: 'postmatch',
  'post-match': 'postmatch',
  chat: 'chat',
  hostcommand: 'hostcommand',
  'host-command': 'hostcommand',
  host: 'hostcommand',
}

const genericConfig = {
  headline: 'Host your own PUCK server in minutes.',
  subheadline: 'SpeedHosting is built for PUCK players and server owners who want a clean dashboard, a free starting point, and a better upgrade path when the community grows.',
  contextLine: 'Built for PUCK players and communities.',
  trustLine: 'Start free, launch fast, and upgrade when you need more control.',
  urgency: 'Free gets you started. Pro is 8 EUR/month for mods, advanced settings, and more room to grow.',
  primaryCta: 'Start Free',
  secondaryCta: 'See Plans',
  registerTitle: 'You are one step away from launching your PUCK server.',
  registerSubtitle: 'Create your account, start free, and launch fast.',
  loginTitle: 'Get back to your PUCK server fast.',
  loginSubtitle: 'Manage your server, plan, and next session from one place.',
  dashboardEmptyTitle: 'Create your first PUCK server now.',
  dashboardEmptyCopy: 'Start on Free and upgrade later if you want mods or advanced settings.',
}

function sanitizeSourceValue(value: string | null | undefined) {
  if (!value) {
    return ''
  }

  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '')
}

export function normalizePuckSource(value: string | null | undefined) {
  const sanitized = sanitizeSourceValue(value)
  if (!sanitized) {
    return null
  }

  return aliasMap[sanitized] ?? sanitized
}

export function resolvePuckSource(sourceParam: string | null | undefined, aliasSource: string | undefined) {
  const querySource = normalizePuckSource(sourceParam)
  const alias = normalizePuckSource(aliasSource)
  const source = querySource ?? alias ?? 'direct'

  return {
    source,
    isExplicitSource: Boolean(querySource ?? alias),
  }
}

export function getPuckSourceConfig(source: string | null | undefined) {
  const normalized = normalizePuckSource(source)
  if (normalized && normalized in knownSourceConfigs) {
    return {
      source: normalized,
      ...knownSourceConfigs[normalized as keyof typeof knownSourceConfigs],
    }
  }

  return {
    source: normalized ?? 'direct',
    ...genericConfig,
  }
}