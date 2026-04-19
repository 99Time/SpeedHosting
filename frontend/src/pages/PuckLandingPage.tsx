import { useEffect } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'

import { useAuth } from '../auth/AuthContext'
import { capturePuckAttribution } from '../lib/attribution'
import { trackEvent } from '../lib/analytics'
import { publicPlanCatalog } from '../lib/planCatalog'
import { getPuckSourceConfig, resolvePuckSource } from '../lib/puckSources'
import { PRO_PATREON_URL, SPEEDHOSTING_DISCORD_URL } from '../lib/proUpgrade'

export function PuckLandingPage() {
  const { user } = useAuth()
  const { sourceAlias } = useParams()
  const [searchParams] = useSearchParams()
  const { source, isExplicitSource } = resolvePuckSource(searchParams.get('src'), sourceAlias)
  const copy = getPuckSourceConfig(source)

  useEffect(() => {
    const currentPath = `${window.location.pathname}${window.location.search}`
    const attribution = capturePuckAttribution(source, currentPath, window.location.href, isExplicitSource)

    void trackEvent('puck_landing_view', {
      source,
      route: currentPath,
      acquisition: {
        source,
        timestamp: attribution.latestTimestamp,
        landingPath: attribution.landingPath,
        fullUrl: attribution.fullUrl,
        sessionId: attribution.sessionId,
        route: currentPath,
      },
      metadata: {
        variant: source,
        explicitSource: isExplicitSource,
      },
    })
  }, [isExplicitSource, source])

  function handleCTAClick(kind: 'primary' | 'secondary') {
    void trackEvent('puck_cta_click', {
      source,
      route: `${window.location.pathname}${window.location.search}`,
      metadata: {
        kind,
        variant: source,
      },
    })
  }

  return (
    <div className="marketing-page marketing-page--puck">
      <header className="marketing-header marketing-header--puck">
        <div className="marketing-nav">
          <div>
            <span className="brand-block__eyebrow">From inside PUCK</span>
            <h2 className="marketing-brand">SpeedHosting</h2>
          </div>

          <div className="marketing-actions">
            <Link className="button button--ghost" to="/login">
              Sign in
            </Link>
            <Link className="button button--primary" onClick={() => handleCTAClick('primary')} to={user ? '/app/servers' : '/register'}>
              {user ? 'Launch My Server' : copy.primaryCta}
            </Link>
          </div>
        </div>

        <div className="marketing-hero">
          <div className="marketing-hero__copy">
            <span className="pill">Free plan available</span>
            <h1>{copy.headline}</h1>
            <p>{copy.subheadline}</p>

            <div className="marketing-hero__actions">
              <Link className="button button--primary" onClick={() => handleCTAClick('primary')} to={user ? '/app/servers' : '/register'}>
                {user ? 'Launch My Server' : copy.primaryCta}
              </Link>
              <a className="button button--ghost" href="#puck-plans" onClick={() => handleCTAClick('secondary')}>
                {copy.secondaryCta}
              </a>
            </div>

            <div className="marketing-highlight-row">
              <span className="marketing-highlight-chip">{copy.contextLine}</span>
              <span className="marketing-highlight-chip">{copy.trustLine}</span>
            </div>

            <div className="marketing-kpis">
              <article className="feature-card">
                <h3>{publicPlanCatalog.free.eyebrow}</h3>
                <p>{publicPlanCatalog.free.summary}</p>
              </article>
              <article className="feature-card feature-card--pro-callout">
                <h3>{publicPlanCatalog.pro.name} is {publicPlanCatalog.pro.price}{publicPlanCatalog.pro.cadence}</h3>
                <p>{publicPlanCatalog.pro.summary}</p>
                <div className="button-group button-group--compact">
                  <a className="button button--primary button--small" href={PRO_PATREON_URL} rel="noreferrer noopener" target="_blank">
                    Upgrade to Pro
                  </a>
                  <a className="button button--ghost button--small" href={SPEEDHOSTING_DISCORD_URL} rel="noreferrer noopener" target="_blank">
                    Discord
                  </a>
                </div>
              </article>
              <article className="feature-card">
                <h3>{publicPlanCatalog.premium.name}</h3>
                <p>{publicPlanCatalog.premium.summary}</p>
              </article>
            </div>

            <div className="trust-strip">
              <span className="trust-pill">Built for in-game traffic</span>
              <span className="trust-pill">From click to live server</span>
              <span className="trust-pill">Free first, paid later</span>
            </div>
          </div>

          <aside className="marketing-hero__panel puck-landing-panel">
            <div className="marketing-scope-intro">
              <span className="eyebrow">Why players click through</span>
              <h3>One clean dashboard for launching, managing, and upgrading your own PUCK server.</h3>
            </div>
            <div className="marketing-scope-list">
              <article className="marketing-scope-item">
                <span className="topbar__label">Built for PUCK</span>
                <strong>Focused on players, server owners, and communities instead of generic hosting buyers.</strong>
              </article>
              <article className="marketing-scope-item">
                <span className="topbar__label">Low friction</span>
                <strong>Start free, move to Pro for real control, and keep Premium as the advanced ceiling for serious communities.</strong>
              </article>
              <article className="marketing-scope-item">
                <span className="topbar__label">Current region</span>
                <strong>Hosted in EU Central, based in Nuremberg, Germany.</strong>
              </article>
            </div>

            <div className="product-preview">
              <div className="product-preview__header">
                <div className="product-preview__subheader">
                  <span className="topbar__label">What the flow feels like</span>
                  <strong>Signup, create, start, invite.</strong>
                </div>
                <span className="pill pill--accent">Player-first</span>
              </div>

              <div className="product-preview__status-list">
                <div className="product-preview__status-row product-preview__status-row--success">
                  <span>Signup</span>
                  <strong>Free account in minutes</strong>
                </div>
                <div className="product-preview__status-row">
                  <span>Create server</span>
                  <strong>Basic setup from one form</strong>
                </div>
                <div className="product-preview__status-row product-preview__status-row--warning">
                  <span>Upgrade later</span>
                  <strong>Only when Pro or Premium actually matter</strong>
                </div>
              </div>
            </div>
          </aside>
        </div>
      </header>

      <section className="panel-card landing-section">
        <div className="landing-section__header">
          <div>
            <span className="eyebrow">How it works</span>
            <h3>Go from a PUCK player to a server owner in three steps.</h3>
          </div>
        </div>
        <div className="landing-grid landing-grid--three">
          <article className="feature-card">
            <h3>1. Create your account</h3>
            <p>Use the Free plan to get started without any commitment.</p>
          </article>
          <article className="feature-card">
            <h3>2. Launch your first server</h3>
            <p>Pick the basics, create the server, and manage it from one dashboard.</p>
          </article>
          <article className="feature-card">
            <h3>3. Upgrade only when needed</h3>
            <p>Pro is the paid step for advanced settings and one user mod slot. Premium is there when Pro becomes too tight.</p>
          </article>
        </div>
      </section>

      <section className="panel-card landing-section" id="puck-plans">
        <div className="landing-section__header">
          <div>
            <span className="eyebrow">Plans</span>
            <h3>Clear Free, Pro, and Premium choices for PUCK communities.</h3>
          </div>
        </div>

        <div className="plan-comparison-grid">
          <article className="plan-card plan-card--current">
            <div className="plan-card__header">
              <div>
                <span className="topbar__label">{publicPlanCatalog.free.eyebrow}</span>
                <h4>{publicPlanCatalog.free.title}</h4>
              </div>
              <div className="plan-card__price">
                <strong>{publicPlanCatalog.free.price}</strong>
                <span>{publicPlanCatalog.free.cadence}</span>
              </div>
            </div>
            <p>{publicPlanCatalog.free.summary}</p>
            <div className="plan-card__list">
              <div>
                <span className="topbar__label">Included</span>
                <strong>{publicPlanCatalog.free.serverAllowance}</strong>
              </div>
              <div>
                <span className="topbar__label">Tick rate</span>
                <strong>{publicPlanCatalog.free.tickRate}</strong>
              </div>
              <div>
                <span className="topbar__label">Mods</span>
                <strong>{publicPlanCatalog.free.mods}</strong>
              </div>
              <div>
                <span className="topbar__label">SpeedRankeds</span>
                <strong>{publicPlanCatalog.free.speedRankeds}</strong>
              </div>
            </div>
          </article>

          <article className="plan-card plan-card--pro">
            <div className="plan-card__header">
              <div>
                <span className="topbar__label">{publicPlanCatalog.pro.eyebrow}</span>
                <h4>{publicPlanCatalog.pro.title}</h4>
              </div>
              <div className="plan-card__price">
                <strong>{publicPlanCatalog.pro.price}</strong>
                <span>{publicPlanCatalog.pro.cadence}</span>
              </div>
            </div>
            <p>{publicPlanCatalog.pro.summary}</p>
            <div className="plan-card__list">
              <div>
                <span className="topbar__label">Servers</span>
                <strong>{publicPlanCatalog.pro.serverAllowance}</strong>
              </div>
              <div>
                <span className="topbar__label">Tick rate</span>
                <strong>{publicPlanCatalog.pro.tickRate}</strong>
              </div>
              <div>
                <span className="topbar__label">Mods</span>
                <strong>{publicPlanCatalog.pro.mods}</strong>
              </div>
              <div>
                <span className="topbar__label">Admins</span>
                <strong>{publicPlanCatalog.pro.admins}</strong>
              </div>
            </div>
          </article>

          <article className="plan-card">
            <div className="plan-card__header">
              <div>
                <span className="topbar__label">{publicPlanCatalog.premium.eyebrow}</span>
                <h4>{publicPlanCatalog.premium.title}</h4>
              </div>
              <div className="plan-card__price">
                <strong>{publicPlanCatalog.premium.price}</strong>
                <span>{publicPlanCatalog.premium.cadence}</span>
              </div>
            </div>
            <p>{publicPlanCatalog.premium.summary}</p>
            <div className="plan-card__list">
              <div>
                <span className="topbar__label">Servers</span>
                <strong>{publicPlanCatalog.premium.serverAllowance}</strong>
              </div>
              <div>
                <span className="topbar__label">Tick rate</span>
                <strong>{publicPlanCatalog.premium.tickRate}</strong>
              </div>
              <div>
                <span className="topbar__label">Mods</span>
                <strong>{publicPlanCatalog.premium.mods}</strong>
              </div>
              <div>
                <span className="topbar__label">Admins</span>
                <strong>{publicPlanCatalog.premium.admins}</strong>
              </div>
            </div>
          </article>
        </div>
      </section>

      <section className="site-footer">
        <div className="site-footer__grid">
          <div>
            <span className="eyebrow">From PUCK to hosting</span>
            <h3>{copy.primaryCta} and keep everything in one clean dashboard.</h3>
            <p>{copy.trustLine}</p>
          </div>

          <div className="site-footer__cta">
            <Link className="button button--primary" onClick={() => handleCTAClick('primary')} to={user ? '/app/servers' : '/register'}>
              {user ? 'Open My Servers' : copy.primaryCta}
            </Link>
            <a className="button button--ghost" href={SPEEDHOSTING_DISCORD_URL} rel="noreferrer noopener" target="_blank">
              Join Discord
            </a>
          </div>
        </div>
      </section>
    </div>
  )
}