export type PublicPlanCode = 'free' | 'pro' | 'premium'

export type PublicPlanCard = {
  code: PublicPlanCode
  name: string
  eyebrow: string
  title: string
  price: string
  cadence: string
  summary: string
  serverAllowance: string
  tickRate: string
  admins: string
  mods: string
  speedRankeds: string
}

export const publicPlanCatalog: Record<PublicPlanCode, PublicPlanCard> = {
  free: {
    code: 'free',
    name: 'Free',
    eyebrow: 'Starter',
    title: 'Launch your first server',
    price: '0 EUR',
    cadence: 'to start',
    summary: 'Best for trying SpeedHosting, learning the dashboard, and launching one restricted starter server.',
    serverAllowance: '1 server',
    tickRate: 'Up to 120 on the starter profile',
    admins: '1 admin Steam ID',
    mods: 'No user-configurable mods',
    speedRankeds: 'Unavailable on Free',
  },
  pro: {
    code: 'pro',
    name: 'Pro',
    eyebrow: 'Most Popular',
    title: 'For growing communities',
    price: '8 EUR',
    cadence: '/ month',
    summary: 'Best for communities that want advanced settings, higher limits, and one controlled user mod slot.',
    serverAllowance: 'Up to 3 servers',
    tickRate: 'Up to 240 with advanced tuning',
    admins: 'Up to 5 admin Steam IDs',
    mods: 'Built-in mods plus 1 user-configurable mod slot',
    speedRankeds: 'Available',
  },
  premium: {
    code: 'premium',
    name: 'Premium',
    eyebrow: 'Recommended',
    title: 'Advanced hosting freedom',
    price: 'Advanced',
    cadence: 'for serious communities',
    summary: 'For serious communities that need more freedom, more mods, more admins, higher ceilings, and room for future premium-only features.',
    serverAllowance: 'Up to 8 servers',
    tickRate: 'Up to 360 with the highest ceiling',
    admins: 'Up to 12 admin Steam IDs',
    mods: 'Built-in mods plus 4 user-configurable mod slots',
    speedRankeds: 'Available with premium feature headroom',
  },
}

export function getPublicPlanCard(code: string | undefined): PublicPlanCard {
  const normalized = (code ?? 'free').toLowerCase() as PublicPlanCode
  return publicPlanCatalog[normalized] ?? publicPlanCatalog.free
}