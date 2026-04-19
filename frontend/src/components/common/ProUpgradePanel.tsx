import { useEffect } from 'react'

import { useAuth } from '../../auth/AuthContext'
import { getPreferredAttributionSource } from '../../lib/attribution'
import { trackEvent } from '../../lib/analytics'
import { getPublicPlanCard } from '../../lib/planCatalog'
import { PRO_PATREON_URL, SPEEDHOSTING_DISCORD_URL } from '../../lib/proUpgrade'

type ProUpgradePanelProps = {
  planCode?: string
  variant?: 'full' | 'compact'
  title?: string
}

const upgradeHighlights = [
  'Pro unlocks advanced settings and one controlled user mod slot',
  'Premium adds more mod freedom, more admins, and a higher ceiling',
  'Both paid tiers stay above Free while preserving a clear ladder to Premium',
]

export function ProUpgradePanel({ planCode, variant = 'full', title = 'Unlock a paid plan' }: ProUpgradePanelProps) {
  const { user } = useAuth()
  const source = getPreferredAttributionSource(user)
  const currentPlan = getPublicPlanCard(planCode)
  const isPro = currentPlan.code === 'pro'
  const isPremium = currentPlan.code === 'premium'

  useEffect(() => {
    if (isPro || isPremium) {
      return
    }

    void trackEvent('pro_upgrade_view', {
      source,
      route: `${window.location.pathname}${window.location.search}`,
      metadata: {
        variant,
        title,
      },
    })
  }, [isPremium, isPro, source, title, variant])

  if (isPremium) {
    return (
      <section className={`upgrade-panel upgrade-panel--${variant} upgrade-panel--active`}>
        <div className="upgrade-panel__header">
          <div>
            <span className="eyebrow">Premium status</span>
            <h3>Premium is active</h3>
          </div>
          <span className="pill pill--accent">Premium</span>
        </div>

        <p>
          This account already has the highest public plan. Premium limits, premium-only headroom,
          and the strongest hosting flexibility are already unlocked.
        </p>
      </section>
    )
  }

  if (isPro) {
    return (
      <section className={`upgrade-panel upgrade-panel--${variant} upgrade-panel--active`}>
        <div className="upgrade-panel__header">
          <div>
            <span className="eyebrow">Most Popular plan</span>
            <h3>Pro is active</h3>
          </div>
          <span className="pill pill--accent">Pro</span>
        </div>

        <p>
          Pro is active. If your community grows beyond one user mod slot, five admin Steam IDs,
          or the 240 tick ceiling, Premium is the next step.
        </p>

        <div className="upgrade-panel__actions">
          <a className="button button--ghost" href={SPEEDHOSTING_DISCORD_URL} rel="noreferrer noopener" target="_blank">
            Ask About Premium
          </a>
        </div>
      </section>
    )
  }

  const isCompact = variant === 'compact'

  return (
    <section className={`upgrade-panel upgrade-panel--${variant}`}>
      <div className="upgrade-panel__header">
        <div>
          <span className="eyebrow">Paid plans</span>
          <h3>{title}</h3>
        </div>
        <span className="pill pill--accent">Pro from 8 EUR / month</span>
      </div>

      <p>
        Start on Free, move to Pro when you want real control, and step up to Premium when your
        community needs more mods, more admins, and a higher ceiling than Pro should offer.
      </p>

      <div className="upgrade-panel__summary-grid">
        <div className="upgrade-panel__summary-item">
          <span className="topbar__label">Why upgrade</span>
          <strong>Advanced settings, mods, and stronger headroom</strong>
        </div>
        <div className="upgrade-panel__summary-item">
          <span className="topbar__label">Best fit</span>
          <strong>Communities moving beyond a single starter server</strong>
        </div>
      </div>

      <div className="upgrade-panel__highlights">
        {upgradeHighlights.map((item) => (
          <article className="upgrade-panel__highlight" key={item}>
            <strong>{item}</strong>
          </article>
        ))}
      </div>

      <div className="upgrade-panel__actions">
        <a
          className="button button--primary"
          href={PRO_PATREON_URL}
          rel="noreferrer noopener"
          target="_blank"
          onClick={() => {
            void trackEvent('pro_upgrade_click', {
              source,
              route: `${window.location.pathname}${window.location.search}`,
              metadata: {
                variant,
                title,
                price: '8 EUR/month',
              },
            })
          }}
        >
          Upgrade to Pro
        </a>
        <a className="button button--ghost" href={SPEEDHOSTING_DISCORD_URL} rel="noreferrer noopener" target="_blank">
          Ask About Premium
        </a>
      </div>

      {isCompact ? (
        <div className="upgrade-panel__note">
          <span className="topbar__label">Current activation flow</span>
          <p>After subscribing on Patreon, open a Discord ticket, send your SpeedHosting account email, and attach a screenshot showing that you are subscribed.</p>
        </div>
      ) : (
        <section className="upgrade-panel__steps">
          <span className="topbar__label">How activation works</span>
          <div className="upgrade-panel__step-list">
            <article className="upgrade-panel__step">
              <strong>1. Subscribe on Patreon</strong>
              <p>Open Patreon in a new tab and complete the SpeedHosting Pro subscription.</p>
            </article>
            <article className="upgrade-panel__step">
              <strong>2. Open a ticket in Discord</strong>
              <p>Join the SpeedHosting Discord and open a support ticket for Pro activation.</p>
            </article>
            <article className="upgrade-panel__step">
              <strong>3. Send your email and subscription screenshot</strong>
              <p>Include the same SpeedHosting account email you use here and a screenshot proving that you are subscribed on Patreon.</p>
            </article>
            <article className="upgrade-panel__step">
              <strong>4. We activate Pro manually</strong>
              <p>Your account is upgraded after we verify the Discord ticket and Patreon subscription.</p>
            </article>
          </div>
        </section>
      )}
    </section>
  )
}
