import { useEffect, useState } from 'react'

import { getMe, getPlan } from '../lib/api'
import type { MeResponse, PlanResponse } from '../types/api'

export function AccountPlanPage() {
  const [me, setMe] = useState<MeResponse | null>(null)
  const [planData, setPlanData] = useState<PlanResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true

    Promise.all([getMe(), getPlan()])
      .then(([meResponse, planResponse]) => {
        if (active) {
          setMe(meResponse)
          setPlanData(planResponse)
        }
      })
      .catch((requestError: Error) => {
        if (active) {
          setError(requestError.message)
        }
      })

    return () => {
      active = false
    }
  }, [])

  if (error) {
    return <section className="panel-card">Failed to load account data: {error}</section>
  }

  if (!me || !planData) {
    return (
      <div className="page-grid">
        <div className="account-overview-grid">
          <div className="panel-card dash-skel" style={{ minHeight: 160 }} />
          <div className="panel-card dash-skel" style={{ minHeight: 160 }} />
        </div>
        <div className="panel-card dash-skel" style={{ minHeight: 180 }} />
      </div>
    )
  }

  const currentPlanCode = planData.plan.code
  const isPaidPlan = currentPlanCode !== 'free'
  const adminIdLimit = planData.plan.maxAdminSteamIds ?? planData.plan.maxAdmins
  const modSlotLimit = planData.plan.maxUserConfigurableMods

  return (
    <div className="page-grid">
      <div className="account-overview-grid">
        <section className="panel-card account-overview-card">
          <div className="account-overview-card__header">
            <div>
              <h3>{me.user.displayName}</h3>
            </div>
            {me.user.role === 'admin' ? <span className="pill pill--accent">Admin</span> : null}
          </div>
          <div className="account-plan-meta">
            <div>
              <span className="topbar__label">Email</span>
              <strong>{me.user.email}</strong>
            </div>
          </div>
        </section>

        <section className="panel-card account-overview-card account-overview-card--primary">
          <div className="account-overview-card__header">
            <div>
              <h3>{planData.plan.name}</h3>
            </div>
            <span className={isPaidPlan ? 'pill pill--accent' : 'pill'}>{isPaidPlan ? 'Active' : 'Free'}</span>
          </div>
        </section>
      </div>

      <div className="account-overview-grid">
        <section className="panel-card account-overview-card">
          <div className="account-overview-card__header">
            <div>
              <span className="eyebrow">Usage</span>
            </div>
          </div>

          <div className="usage-bars-list">
            <div className="usage-bar">
              <div className="usage-bar__label-row">
                <span className="usage-bar__label">Server slots</span>
                <span className="usage-bar__value">{planData.usage.serverCount} / {planData.plan.maxServers}</span>
              </div>
              <div className="usage-bar__track">
                <div
                  className={`usage-bar__fill${
                    planData.usage.serverCount / planData.plan.maxServers >= 1
                      ? ' usage-bar__fill--danger'
                      : planData.usage.serverCount / planData.plan.maxServers >= 0.75
                        ? ' usage-bar__fill--warning'
                        : ''
                  }`}
                  style={{ width: `${Math.min(100, Math.round((planData.usage.serverCount / planData.plan.maxServers) * 100))}%` }}
                />
              </div>
            </div>

            <div className="usage-bar">
              <div className="usage-bar__label-row">
                <span className="usage-bar__label">Tick rate ceiling</span>
                <span className="usage-bar__value">{planData.plan.maxTickRate} Hz</span>
              </div>
              <div className="usage-bar__track">
                <div
                  className="usage-bar__fill"
                  style={{ width: `${Math.min(100, Math.round((planData.plan.maxTickRate / 128) * 100))}%` }}
                />
              </div>
            </div>

            <div className="account-usage-grid">
              <article className="account-usage-stat">
                <span className="account-usage-stat__label">Admin IDs</span>
                <strong className="account-usage-stat__value">{adminIdLimit}</strong>
              </article>
              <article className="account-usage-stat">
                <span className="account-usage-stat__label">Mod slots</span>
                <strong className="account-usage-stat__value">{modSlotLimit}</strong>
              </article>
            </div>
          </div>
        </section>

        <section className="panel-card account-overview-card">
          <div className="account-overview-card__header">
            <div>
              <span className="eyebrow">Limits</span>
            </div>
          </div>
          <div className="account-limit-list">
            <div className="account-limit-list__row">
              <span>Advanced config</span>
              <strong>{planData.plan.allowAdvancedConfig ? 'Enabled' : 'Locked'}</strong>
            </div>
            <div className="account-limit-list__row">
              <span>Custom mods</span>
              <strong>{planData.plan.allowCustomMods ? 'Allowed' : 'Unavailable'}</strong>
            </div>
            <div className="account-limit-list__row">
              <span>SpeedRankeds</span>
              <strong>{planData.plan.allowSpeedRankeds ? 'Available' : 'Unavailable'}</strong>
            </div>
            <div className="account-limit-list__row">
              <span>Premium feature access</span>
              <strong>{planData.plan.premiumFeatureAccess ? 'Included' : 'Not included'}</strong>
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}
