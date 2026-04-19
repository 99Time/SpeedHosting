import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { StatCard } from '../components/common/StatCard'
import { StatusBadge } from '../components/common/StatusBadge'
import { getDashboard } from '../lib/api'
import type { DashboardResponse } from '../types/api'

export function DashboardPage() {
  const [data, setData] = useState<DashboardResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true

    getDashboard()
      .then((response) => {
        if (active) {
          setData(response)
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
    return <section className="panel-card">Failed to load dashboard: {error}</section>
  }

  if (!data) {
    return (
      <div className="page-grid">
        <div className="dash-skel dash-skel--banner" />
        <div className="stats-grid">
          {[0, 1, 2, 3].map((i) => (
            <div className="stat-card dash-skel" key={i} style={{ minHeight: 110 }} />
          ))}
        </div>
        <div className="dash-content">
          <div className="panel-card dash-skel" style={{ minHeight: 168 }} />
          <div className="panel-card dash-skel" style={{ minHeight: 210 }} />
        </div>
      </div>
    )
  }

  const isPaid = data.plan.code !== 'free'
  const hasServers = data.servers.length > 0

  return (
    <div className="page-grid">
      {/* Banner */}
      <section className="dash-banner">
        <div className="dash-banner__head">
          <div>
            <h2 className="dash-banner__title">
              {hasServers
                ? `${data.summary.serverCount} server${data.summary.serverCount === 1 ? '' : 's'}`
                : 'No servers yet'}
            </h2>
            <p className="dash-banner__sub">
              {hasServers
                ? `${data.summary.activeServers} of ${data.summary.serverCount} running`
                : 'Ready when you are.'}
            </p>
          </div>
          <div className="dash-banner__actions">
            <span className="dash-banner__plan-pill">{data.plan.name}</span>
            <Link className="button button--primary button--small" to="/app/servers">
              {hasServers ? 'Manage Servers' : 'Create Server'}
            </Link>
          </div>
        </div>
      </section>

      {/* Stats */}
      <div className="stats-grid">
        <StatCard
          label="Servers"
          value={`${data.summary.serverCount}/${data.summary.maxServers}`}
          hint="slots used"
        />
        <StatCard label="Running" value={data.summary.activeServers} hint="currently active" />
        <StatCard label="Players" value={data.summary.totalPlayers} hint="across all servers" />
        <StatCard label="Tick rate" value={`${data.summary.maxTickRate} Hz`} hint="plan ceiling" />
      </div>

      {/* Two-column content */}
      <div className="dash-content">
        {/* Quick actions */}
        <section className="panel-card">
          <div className="dash-actions-grid">
              <Link className="dash-action-btn" to="/app/servers">
                <span className="dash-action-btn__icon">
                  <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
                    <rect height="6" rx="2" stroke="currentColor" strokeWidth="1.8" width="18" x="3" y="4" />
                    <rect height="6" rx="2" stroke="currentColor" strokeWidth="1.8" width="18" x="3" y="14" />
                  </svg>
                </span>
                <span className="dash-action-btn__label">My Servers</span>
              </Link>
              <Link className="dash-action-btn" to="/app/account">
                <span className="dash-action-btn__icon">
                  <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
                    <circle cx="12" cy="8" r="4" stroke="currentColor" strokeWidth="1.8" />
                    <path d="M4 20c0-4 3.58-7 8-7s8 3 8 7" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
                  </svg>
                </span>
                <span className="dash-action-btn__label">Account</span>
              </Link>
              <Link className="dash-action-btn" to="/app/updates">
                <span className="dash-action-btn__icon">
                  <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
                    <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
                  </svg>
                </span>
                <span className="dash-action-btn__label">What&apos;s New</span>
              </Link>
              {isPaid ? (
                <Link className="dash-action-btn" to="/app/servers">
                  <span className="dash-action-btn__icon">
                    <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
                      <path d="M12 3l8 4v5c0 4.4-3.4 8.5-8 9.4C7.4 20.5 4 16.4 4 12V7l8-4Z" stroke="currentColor" strokeWidth="1.8" />
                    </svg>
                  </span>
                  <span className="dash-action-btn__label">Advanced Config</span>
                </Link>
              ) : (
                <Link className="dash-action-btn" to="/app/account">
                  <span className="dash-action-btn__icon">
                    <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
                      <path d="M12 2l2 7h7l-5.5 4 2 7L12 16l-5.5 4 2-7L3 9h7L12 2Z" stroke="currentColor" strokeWidth="1.8" />
                    </svg>
                  </span>
                  <span className="dash-action-btn__label">Upgrade</span>
                </Link>
              )}
          </div>
        </section>

        {/* Server list */}
        <section className="panel-card">
          <div className="panel-card__header--tight">
            <span className="eyebrow">Servers</span>
            <Link className="button button--ghost button--small" to="/app/servers">
              {hasServers ? 'All servers' : 'Create one'}
            </Link>
          </div>
          {!hasServers ? (
            <div className="empty-state">
              <div className="empty-state__icon" aria-hidden="true">
                <svg fill="none" height="24" viewBox="0 0 24 24" width="24">
                  <rect height="6" rx="2" stroke="currentColor" strokeWidth="1.6" width="18" x="3" y="4" />
                  <rect height="6" rx="2" stroke="currentColor" strokeWidth="1.6" width="18" x="3" y="14" />
                </svg>
              </div>
              <h4>No servers yet</h4>
            </div>
          ) : (
            <div className="dash-server-list">
              {data.servers.map((server) => (
                <div className="dash-server-item" key={server.id}>
                  <div className="dash-server-item__name">{server.name}</div>
                  <div className="dash-server-item__meta">
                    <span>{server.playerCount}/{server.maxPlayers} players</span>
                  </div>
                  <StatusBadge status={server.status} />
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}

