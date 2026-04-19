import { Link } from 'react-router-dom'

import { useAuth } from '../auth/AuthContext'
import { publicPlanCatalog } from '../lib/planCatalog'
import { PRO_PATREON_URL, SPEEDHOSTING_DISCORD_URL } from '../lib/proUpgrade'

export function LandingPage() {
  const { user } = useAuth()

  return (
    <div className="lp-root">
      {/* NAV */}
      <nav className="lp-nav">
        <div className="lp-container lp-nav__inner">
          <div className="lp-nav__brand">
            <span className="lp-eyebrow">PUCK Server Hosting</span>
            <span className="lp-brand-name">SpeedHosting</span>
          </div>
          <div className="lp-nav__actions">
            <Link className="lp-btn lp-btn--ghost" to="/login">Sign in</Link>
            <Link className="lp-btn lp-btn--primary" to={user ? '/app' : '/register'}>
              {user ? 'Open dashboard' : 'Start Free'}
            </Link>
          </div>
        </div>
      </nav>

      {/* HERO */}
      <section className="lp-hero">
        <div className="lp-container lp-hero__inner">
          <span className="lp-tag">Free plan available</span>
          <h1 className="lp-hero__headline">
            Your PUCK server,<br />live in minutes.
          </h1>
          <p className="lp-hero__sub">
            Managed hosting for PUCK communities. Start free, upgrade when you are ready.
          </p>
          <div className="lp-cta-row">
            <Link className="lp-btn lp-btn--primary lp-btn--lg" to={user ? '/app/servers' : '/register'}>
              {user ? 'Deploy Now →' : 'Start Hosting Free →'}
            </Link>
            <a className="lp-btn lp-btn--ghost lp-btn--lg" href="#plans">
              See Pricing
            </a>
          </div>
          <div className="lp-trust-row">
            <span className="lp-trust-chip">Account-owned servers</span>
            <span className="lp-trust-chip">No config files</span>
            <span className="lp-trust-chip">EU Central hosting</span>
          </div>
        </div>
      </section>

      {/* FEATURES */}
      <section className="lp-section">
        <div className="lp-container">
          <div className="lp-section__head">
            <h2>Everything you need to run a PUCK server.</h2>
            <p>From first deploy to advanced configuration, without infrastructure overhead.</p>
          </div>
          <div className="lp-grid lp-grid--3">
            <article className="lp-card">
              <h3>Instant Provisioning</h3>
              <p>Sign up, pick a plan, deploy. No config files, no CLI, no waiting.</p>
            </article>
            <article className="lp-card">
              <h3>Full Runtime Control</h3>
              <p>Start, stop, restart, and configure your server from one dashboard.</p>
            </article>
            <article className="lp-card">
              <h3>A Clear Growth Path</h3>
              <p>Free to Pro to Premium. Upgrade only when your community needs more.</p>
            </article>
          </div>
        </div>
      </section>

      {/* PRICING */}
      <section className="lp-section lp-section--alt" id="plans">
        <div className="lp-container">
          <div className="lp-section__head">
            <h2>Simple, honest pricing.</h2>
            <p>Start for free. No credit card required.</p>
          </div>
          <div className="lp-plans">
            <article className="lp-plan">
              <div className="lp-plan__top">
                <span className="lp-eyebrow">{publicPlanCatalog.free.eyebrow}</span>
                <h3>{publicPlanCatalog.free.title}</h3>
              </div>
              <div className="lp-plan__price">
                <strong>{publicPlanCatalog.free.price}</strong>
                <span>{publicPlanCatalog.free.cadence}</span>
              </div>
              <p className="lp-plan__desc">{publicPlanCatalog.free.summary}</p>
              <ul className="lp-plan__specs">
                <li><span>Servers</span><strong>{publicPlanCatalog.free.serverAllowance}</strong></li>
                <li><span>Tick rate</span><strong>{publicPlanCatalog.free.tickRate}</strong></li>
                <li><span>Mods</span><strong>{publicPlanCatalog.free.mods}</strong></li>
                <li><span>SpeedRankeds</span><strong>{publicPlanCatalog.free.speedRankeds}</strong></li>
              </ul>
              <div className="lp-plan__cta">
                <Link className="lp-btn lp-btn--ghost lp-btn--full" to={user ? '/app/servers' : '/register'}>
                  {user ? 'Launch My Server' : 'Start Free'}
                </Link>
              </div>
            </article>

            <article className="lp-plan lp-plan--pro">
              <div className="lp-plan__top">
                <div className="lp-plan__top-row">
                  <span className="lp-eyebrow">{publicPlanCatalog.pro.eyebrow}</span>
                  <span className="lp-tag lp-tag--accent">Most Popular</span>
                </div>
                <h3>{publicPlanCatalog.pro.title}</h3>
              </div>
              <div className="lp-plan__price">
                <strong>{publicPlanCatalog.pro.price}</strong>
                <span>{publicPlanCatalog.pro.cadence}</span>
              </div>
              <p className="lp-plan__desc">{publicPlanCatalog.pro.summary}</p>
              <ul className="lp-plan__specs">
                <li><span>Servers</span><strong>{publicPlanCatalog.pro.serverAllowance}</strong></li>
                <li><span>Tick rate</span><strong>{publicPlanCatalog.pro.tickRate}</strong></li>
                <li><span>Mods</span><strong>{publicPlanCatalog.pro.mods}</strong></li>
                <li><span>Admins</span><strong>{publicPlanCatalog.pro.admins}</strong></li>
              </ul>
              <div className="lp-plan__cta">
                <a className="lp-btn lp-btn--primary lp-btn--full" href={PRO_PATREON_URL} rel="noreferrer noopener" target="_blank">Upgrade to Pro</a>
                <a className="lp-btn lp-btn--ghost lp-btn--full" href={SPEEDHOSTING_DISCORD_URL} rel="noreferrer noopener" target="_blank">Discord</a>
              </div>
            </article>

            <article className="lp-plan">
              <div className="lp-plan__top">
                <span className="lp-eyebrow">{publicPlanCatalog.premium.eyebrow}</span>
                <h3>{publicPlanCatalog.premium.title}</h3>
              </div>
              <div className="lp-plan__price">
                <strong>{publicPlanCatalog.premium.price}</strong>
                <span>{publicPlanCatalog.premium.cadence}</span>
              </div>
              <p className="lp-plan__desc">{publicPlanCatalog.premium.summary}</p>
              <ul className="lp-plan__specs">
                <li><span>Servers</span><strong>{publicPlanCatalog.premium.serverAllowance}</strong></li>
                <li><span>Tick rate</span><strong>{publicPlanCatalog.premium.tickRate}</strong></li>
                <li><span>Mods</span><strong>{publicPlanCatalog.premium.mods}</strong></li>
                <li><span>Admins</span><strong>{publicPlanCatalog.premium.admins}</strong></li>
              </ul>
              <div className="lp-plan__cta">
                <a className="lp-btn lp-btn--ghost lp-btn--full" href={SPEEDHOSTING_DISCORD_URL} rel="noreferrer noopener" target="_blank">Ask About Premium</a>
              </div>
            </article>
          </div>

          <div className="lp-plan-note">
            <p>After subscribing on Patreon, open a Discord ticket, send your SpeedHosting account email, and attach a screenshot confirming your subscription so we can activate Pro manually.</p>
          </div>
        </div>
      </section>

      {/* FAQ */}
      <section className="lp-section">
        <div className="lp-container">
          <div className="lp-section__head">
            <h2>Common questions.</h2>
          </div>
          <div className="lp-grid lp-grid--2">
            <article className="lp-card">
              <h3>How fast can I launch?</h3>
              <p>Minutes from signup to a live server. No config files or CLI required.</p>
            </article>
            <article className="lp-card">
              <h3>What is included for free?</h3>
              <p>One server, guided setup, full dashboard access, and core runtime controls.</p>
            </article>
            <article className="lp-card">
              <h3>Why upgrade to Pro?</h3>
              <p>More servers, higher tick rates, workshop mods, and up to 5 admin Steam IDs.</p>
            </article>
            <article className="lp-card">
              <h3>Where is it hosted?</h3>
              <p>EU Central, Nuremberg, Germany. More regions are planned.</p>
            </article>
          </div>
        </div>
      </section>

      {/* FOOTER CTA */}
      <footer className="lp-footer">
        <div className="lp-container lp-footer__inner">
          <h2>Start free. Scale with your community.</h2>
          <p>No credit card required. No infrastructure knowledge needed.</p>
          <div className="lp-cta-row">
            <Link className="lp-btn lp-btn--primary lp-btn--lg" to={user ? '/app/servers' : '/register'}>
              {user ? 'Deploy a Server →' : 'Get Started →'}
            </Link>
            <a className="lp-btn lp-btn--ghost lp-btn--lg" href={SPEEDHOSTING_DISCORD_URL} rel="noreferrer noopener" target="_blank">Join Discord</a>
          </div>
        </div>
      </footer>
    </div>
  )
}
