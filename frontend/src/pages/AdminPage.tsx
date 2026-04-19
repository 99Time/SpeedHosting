import { useEffect, useState } from 'react'

import { StatusBadge } from '../components/common/StatusBadge'
import { StatCard } from '../components/common/StatCard'
import { deleteAdminServer, getAdminOverview, runAdminServerAction, updateAdminServerConfig, updateUserPlan } from '../lib/api'
import type { AdminOverviewResponse, AdminServerSummary, Server, ServerConfig, ServerConfigMod } from '../types/api'

const booleanFields: Array<{ key: keyof ServerConfig; label: string }> = [
  { key: 'isPublic', label: 'Is Public' },
  { key: 'reloadBannedIDs', label: 'Reload Banned IDs' },
  { key: 'usePuckBannedIDs', label: 'Use Puck Banned IDs' },
  { key: 'printMetrics', label: 'Print Metrics' },
  { key: 'startPaused', label: 'Start Paused' },
  { key: 'allowVoting', label: 'Allow Voting' },
]

const timeoutFields: Array<{ key: keyof ServerConfig; label: string }> = [
  { key: 'kickTimeout', label: 'Kick Timeout' },
  { key: 'sleepTimeout', label: 'Sleep Timeout' },
  { key: 'joinMidMatchDelay', label: 'Join Mid-Match Delay' },
  { key: 'targetFrameRate', label: 'Target Frame Rate' },
  { key: 'serverTickRate', label: 'Server Tick Rate' },
  { key: 'clientTickRate', label: 'Client Tick Rate' },
]

const phaseFields: Array<{ key: keyof ServerConfig; label: string }> = [
  { key: 'warmup', label: 'Warmup' },
  { key: 'faceOff', label: 'FaceOff' },
  { key: 'playing', label: 'Playing' },
  { key: 'blueScore', label: 'BlueScore' },
  { key: 'redScore', label: 'RedScore' },
  { key: 'replay', label: 'Replay' },
  { key: 'periodOver', label: 'PeriodOver' },
  { key: 'gameOver', label: 'GameOver' },
]

function parseAdminSteamIds(value: string) {
  return value
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean)
}

function adminSteamIdsToText(values: string[]) {
  return values.join(', ')
}

function buildDraftMap(servers: AdminServerSummary[]) {
  return servers.reduce<Record<number, ServerConfig>>((accumulator, server) => {
    accumulator[server.id] = {
      ...server.config,
      adminSteamIds: [...server.config.adminSteamIds],
      mods: [...server.config.mods],
    }
    return accumulator
  }, {})
}

function TrashIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
      <path d="M4 7h16" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      <path d="M10 11v6" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      <path d="M14 11v6" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      <path d="M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12" stroke="currentColor" strokeWidth="1.8" />
      <path d="M9 4h6a1 1 0 0 1 1 1v2H8V5a1 1 0 0 1 1-1Z" stroke="currentColor" strokeWidth="1.8" />
    </svg>
  )
}

export function AdminPage() {
  const [data, setData] = useState<AdminOverviewResponse | null>(null)
  const [configDrafts, setConfigDrafts] = useState<Record<number, ServerConfig>>({})
  const [showAdvanced, setShowAdvanced] = useState<Record<number, boolean>>({})
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [updatingUserId, setUpdatingUserId] = useState<number | null>(null)
  const [busyAction, setBusyAction] = useState<string | null>(null)
  const [busyDeleteId, setBusyDeleteId] = useState<number | null>(null)
  const [savingConfigId, setSavingConfigId] = useState<number | null>(null)

  useEffect(() => {
    let active = true

    getAdminOverview()
      .then((response) => {
        if (active) {
          setData(response)
          setConfigDrafts(buildDraftMap(response.servers))
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

  function syncManagedServer(nextServer: Server) {
    setData((current) =>
      current
        ? {
            ...current,
            servers: current.servers.map((server) =>
              server.id === nextServer.id
                ? {
                    ...server,
                    ...nextServer,
                    tickRate: nextServer.config.serverTickRate || nextServer.desiredTickRate,
                  }
                : server,
            ),
          }
        : current,
    )

    setConfigDrafts((currentDrafts) => ({
      ...currentDrafts,
      [nextServer.id]: {
        ...nextServer.config,
        adminSteamIds: [...nextServer.config.adminSteamIds],
        mods: [...nextServer.config.mods],
      },
    }))
  }

  function updateDraft(serverId: number, key: keyof ServerConfig, value: ServerConfig[keyof ServerConfig]) {
    setConfigDrafts((currentDrafts) => ({
      ...currentDrafts,
      [serverId]: {
        ...currentDrafts[serverId],
        [key]: value,
      },
    }))
  }

  function updateMod(serverId: number, index: number, key: keyof ServerConfigMod, value: string | boolean) {
    setConfigDrafts((currentDrafts) => {
      const nextMods = [...(currentDrafts[serverId]?.mods ?? [])]
      nextMods[index] = {
        ...nextMods[index],
        [key]: value,
      }

      return {
        ...currentDrafts,
        [serverId]: {
          ...currentDrafts[serverId],
          mods: nextMods,
        },
      }
    })
  }

  function addMod(serverId: number) {
    setConfigDrafts((currentDrafts) => ({
      ...currentDrafts,
      [serverId]: {
        ...currentDrafts[serverId],
        mods: [...(currentDrafts[serverId]?.mods ?? []), { workshopId: '', enabled: true, clientRequired: false }],
      },
    }))
  }

  function removeMod(serverId: number, index: number) {
    setConfigDrafts((currentDrafts) => ({
      ...currentDrafts,
      [serverId]: {
        ...currentDrafts[serverId],
        mods: (currentDrafts[serverId]?.mods ?? []).filter((_, currentIndex) => currentIndex !== index),
      },
    }))
  }

  async function handlePlanChange(userId: number, planCode: string) {
    setUpdatingUserId(userId)
    setError(null)
    setMessage(null)

    try {
      const response = await updateUserPlan(userId, planCode)
      setData((current) =>
        current
          ? {
              ...current,
              users: current.users.map((user) => (user.id === userId ? response.user : user)),
              servers: current.servers.map((server) =>
                server.ownerId === userId
                  ? {
                      ...server,
                      planCode,
                    }
                  : server,
              ),
            }
          : current,
      )
      setMessage(`Plan for user ${response.user.displayName} updated to ${planCode.toUpperCase()}.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Plan update failed')
    } finally {
      setUpdatingUserId(null)
    }
  }

  async function handleServerAction(serverId: number, action: 'start' | 'stop' | 'restart') {
    setBusyAction(`${serverId}:${action}`)
    setError(null)
    setMessage(null)

    try {
      const response = await runAdminServerAction(serverId, action)
      syncManagedServer(response.server)
      setMessage(`Server ${response.server.name} ${action} completed.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Server action failed')
    } finally {
      setBusyAction(null)
    }
  }

  async function handleSaveConfig(serverId: number) {
    const draft = configDrafts[serverId]
    if (!draft) {
      return
    }

    setSavingConfigId(serverId)
    setError(null)
    setMessage(null)

    try {
      const response = await updateAdminServerConfig(serverId, draft)
      syncManagedServer(response.server)
      setMessage(`Configuration for ${response.server.name} was saved.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Config update failed')
    } finally {
      setSavingConfigId(null)
    }
  }

  async function handleDelete(serverId: number, serverName: string) {
    if (!window.confirm(`Delete ${serverName}? This will stop the service, disable it, and remove the JSON config.`)) {
      return
    }

    setBusyDeleteId(serverId)
    setError(null)
    setMessage(null)

    try {
      await deleteAdminServer(serverId)
      setData((current) =>
        current
          ? {
              ...current,
              servers: current.servers.filter((server) => server.id !== serverId),
              users: current.users.map((user) => ({
                ...user,
                serverCount: current.servers.some((server) => server.id === serverId && server.ownerId === user.id)
                  ? Math.max(0, user.serverCount - 1)
                  : user.serverCount,
              })),
            }
          : current,
      )
      setConfigDrafts((currentDrafts) => {
        const nextDrafts = { ...currentDrafts }
        delete nextDrafts[serverId]
        return nextDrafts
      })
      setShowAdvanced((currentState) => {
        const nextState = { ...currentState }
        delete nextState[serverId]
        return nextState
      })
      setMessage(`Server ${serverName} was deleted.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Delete failed')
    } finally {
      setBusyDeleteId(null)
    }
  }

  if (error && !data) {
    return <section className="panel-card">Failed to load admin overview: {error}</section>
  }

  if (!data) {
    return (
      <div className="page-grid">
        <div className="admin-analytics">
          {[0, 1, 2, 3].map((i) => (
            <div className="stat-card dash-skel" key={i} style={{ minHeight: 100 }} />
          ))}
        </div>
        <div className="panel-card dash-skel" style={{ minHeight: 200 }} />
        <div className="panel-card dash-skel" style={{ minHeight: 300 }} />
      </div>
    )
  }

  const adminCount = data.users.filter((user) => user.role === 'admin').length
  const runningServers = data.servers.filter((server) => server.status === 'running').length

  return (
    <div className="page-grid">
      <div className="admin-analytics">
        <StatCard label="Users" value={data.users.length} hint="registered" />
        <StatCard label="Admins" value={adminCount} hint="with panel access" />
        <StatCard label="Fleet" value={data.servers.length} hint="provisioned servers" />
        <StatCard label="Running" value={runningServers} hint="currently active" />
      </div>

      <section className="panel-card">
        <div className="panel-card__header">
          <div>
            <span className="eyebrow">Attribution</span>
            <h3>Acquisition by source</h3>
          </div>
        </div>

        <div className="attribution-report-grid">
          {data.attributionSummary.length === 0 ? (
            <article className="feature-card">
              <h3>No attribution data yet</h3>
              <p>Once traffic starts flowing through /puck, this report will show which in-game placements actually convert.</p>
            </article>
          ) : (
            data.attributionSummary.map((report) => (
              <article className="feature-card attribution-report-card" key={report.source}>
                <div className="panel-card__header panel-card__header--tight">
                  <div>
                    <span className="topbar__label">Source</span>
                    <h4>{report.source}</h4>
                  </div>
                  <span className="pill">{report.landingViews} views</span>
                </div>

                <div className="details-grid details-grid--compact">
                  <div>
                    <span className="topbar__label">Signups</span>
                    <strong>{report.registerSuccesses}</strong>
                  </div>
                  <div>
                    <span className="topbar__label">First servers</span>
                    <strong>{report.firstServerCreated}</strong>
                  </div>
                  <div>
                    <span className="topbar__label">Paid upgrades</span>
                    <strong>{report.proUpgradeSuccesses}</strong>
                  </div>
                  <div>
                    <span className="topbar__label">CTA clicks</span>
                    <strong>{report.ctaClicks}</strong>
                  </div>
                </div>

                <div className="details-grid details-grid--compact">
                  <div>
                    <span className="topbar__label">Signup rate</span>
                    <strong>{report.registerConversionPct.toFixed(1)}%</strong>
                  </div>
                  <div>
                    <span className="topbar__label">Server rate</span>
                    <strong>{report.serverConversionPct.toFixed(1)}%</strong>
                  </div>
                  <div>
                    <span className="topbar__label">Pro rate</span>
                    <strong>{report.proConversionPct.toFixed(1)}%</strong>
                  </div>
                  <div>
                    <span className="topbar__label">Upgrade clicks</span>
                    <strong>{report.proUpgradeClicks}</strong>
                  </div>
                </div>
              </article>
            ))
          )}
        </div>
      </section>

      <div className="admin-grid">
        <section className="panel-card">
          <div className="panel-card__header">
            <div>
              <span className="eyebrow">Accounts</span>
              <h3>Users</h3>
            </div>
          </div>

          <div className="admin-users">
            {data.users.map((user) => (
              <article className="admin-row" key={user.id}>
                <div className="admin-row__header">
                  <div className="admin-row__meta">
                    <strong>{user.displayName}</strong>
                    <span>{user.email}</span>
                  </div>
                  <span className="pill pill--accent">{user.role}</span>
                </div>

                <div className="details-grid details-grid--compact">
                  <div>
                    <span className="topbar__label">Servers</span>
                    <strong>{user.serverCount}</strong>
                  </div>
                  <div>
                    <span className="topbar__label">Plan</span>
                    <strong>{user.planCode.toUpperCase()}</strong>
                  </div>
                  <div>
                    <span className="topbar__label">First source</span>
                    <strong>{user.firstAcquisitionSource || 'direct'}</strong>
                  </div>
                  <div>
                    <span className="topbar__label">Latest source</span>
                    <strong>{user.latestAcquisitionSource || 'direct'}</strong>
                  </div>
                </div>

                <div className="admin-row__footer">
                  <label>
                    <span className="topbar__label">Change plan</span>
                    <select
                      value={user.planCode}
                      onChange={(event) => handlePlanChange(user.id, event.target.value)}
                      disabled={updatingUserId === user.id}
                    >
                      {data.plans.map((plan) => (
                        <option key={plan.code} value={plan.code}>
                          {plan.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  {updatingUserId === user.id ? <span className="muted-copy">Updating...</span> : null}
                </div>
              </article>
            ))}
          </div>
        </section>

        <section className="panel-card">
          <div className="panel-card__header">
            <div>
              <span className="eyebrow">Fleet</span>
              <h3>Servers</h3>
            </div>
          </div>

          <div className="stack-grid">
            {data.servers.length === 0 ? (
              <div className="empty-state">
                <h4>No servers yet</h4>
              </div>
            ) : (
              data.servers.map((server) => (
                <article className="server-card" key={server.id}>
                  <div className="panel-card__header panel-card__header--tight">
                    <div>
                      <span className="topbar__label">
                        {server.ownerName} · {server.ownerEmail}
                      </span>
                      <h4>{server.name}</h4>
                    </div>
                    <div className="server-card__toolbar">
                      <StatusBadge status={server.status} />
                      <button
                        aria-label={`Delete ${server.name}`}
                        className="icon-button icon-button--danger"
                        disabled={busyDeleteId !== null || busyAction !== null || savingConfigId !== null}
                        onClick={() => handleDelete(server.id, server.name)}
                        type="button"
                      >
                        <TrashIcon />
                      </button>
                    </div>
                  </div>

                  <div className="details-grid details-grid--compact">
                    <div>
                      <span className="topbar__label">Plan</span>
                      <strong>{server.planCode.toUpperCase()}</strong>
                    </div>
                    <div>
                      <span className="topbar__label">Players</span>
                      <strong>
                        {server.playerCount}/{server.maxPlayers}
                      </strong>
                    </div>
                    <div>
                      <span className="topbar__label">Tick rate</span>
                      <strong>{server.tickRate}</strong>
                    </div>
                    <div>
                      <span className="topbar__label">Process</span>
                      <strong>{server.processState}</strong>
                    </div>
                    <div>
                      <span className="topbar__label">Service</span>
                      <strong>{server.serviceName}</strong>
                    </div>
                    <div>
                      <span className="topbar__label">Config file</span>
                      <strong>{server.configFilePath}</strong>
                    </div>
                  </div>

                  <div className="action-row action-row--compact">
                    <button
                      className="button button--primary"
                      onClick={() => handleServerAction(server.id, 'start')}
                      disabled={busyAction !== null}
                      type="button"
                    >
                      {busyAction === `${server.id}:start` ? 'Starting...' : 'Start'}
                    </button>
                    <button
                      className="button button--ghost"
                      onClick={() => handleServerAction(server.id, 'restart')}
                      disabled={busyAction !== null}
                      type="button"
                    >
                      {busyAction === `${server.id}:restart` ? 'Restarting...' : 'Restart'}
                    </button>
                    <button
                      className="button button--ghost"
                      onClick={() => handleServerAction(server.id, 'stop')}
                      disabled={busyAction !== null}
                      type="button"
                    >
                      {busyAction === `${server.id}:stop` ? 'Stopping...' : 'Stop'}
                    </button>
                  </div>

                  {server.lastActionError ? <p className="error-copy">Last runtime error: {server.lastActionError}</p> : null}

                  <section className="server-card__config">
                    <div className="panel-card__header panel-card__header--tight">
                      <div>
                        <span className="eyebrow">Admin override</span>
                        <h4>Customer server config</h4>
                      </div>
                      <button
                        className="button button--ghost button--small"
                        disabled={savingConfigId !== null || busyDeleteId !== null}
                        onClick={() => handleSaveConfig(server.id)}
                        type="button"
                      >
                        {savingConfigId === server.id ? 'Saving...' : 'Save config'}
                      </button>
                    </div>

                    <div className="details-grid details-grid--compact">
                      <label>
                        <span>Max Players</span>
                        <input
                          min={2}
                          max={64}
                          type="number"
                          value={configDrafts[server.id]?.maxPlayers ?? server.config.maxPlayers}
                          onChange={(event) => updateDraft(server.id, 'maxPlayers', Number(event.target.value))}
                        />
                      </label>
                      <label>
                        <span>Password</span>
                        <input
                          placeholder="Optional server password"
                          type="text"
                          value={configDrafts[server.id]?.password ?? ''}
                          onChange={(event) => updateDraft(server.id, 'password', event.target.value)}
                        />
                      </label>
                    </div>

                    <label>
                      <span>Admin Steam IDs (comma separated)</span>
                      <input
                        placeholder="7656119..."
                        type="text"
                        value={adminSteamIdsToText(configDrafts[server.id]?.adminSteamIds ?? [])}
                        onChange={(event) => updateDraft(server.id, 'adminSteamIds', parseAdminSteamIds(event.target.value))}
                      />
                    </label>
                    <p className="muted-copy">Admin save rewrites the runtime JSON with only the canonical adminSteamIds key.</p>

                    <div className="advanced-config">
                      <label className="toggle-inline">
                        <input
                          checked={showAdvanced[server.id] ?? false}
                          onChange={(event) =>
                            setShowAdvanced((currentState) => ({
                              ...currentState,
                              [server.id]: event.target.checked,
                            }))
                          }
                          type="checkbox"
                        />
                        <span>Show full parameter editor</span>
                      </label>

                      {showAdvanced[server.id] ? (
                        <>
                          <section className="advanced-config__section">
                            <span className="eyebrow">Server Behavior</span>
                            <div className="toggle-grid">
                              {booleanFields.map((field) => (
                                <label className="toggle-field" key={field.key}>
                                  <span>{field.label}</span>
                                  <input
                                    checked={Boolean(configDrafts[server.id]?.[field.key])}
                                    onChange={(event) => updateDraft(server.id, field.key, event.target.checked)}
                                    type="checkbox"
                                  />
                                </label>
                              ))}
                            </div>
                          </section>

                          <section className="advanced-config__section">
                            <span className="eyebrow">Timeouts and Ticks</span>
                            <div className="details-grid details-grid--compact">
                              {timeoutFields.map((field) => (
                                <label key={field.key}>
                                  <span>{field.label}</span>
                                  <input
                                    min={0}
                                    type="number"
                                    value={Number(configDrafts[server.id]?.[field.key] ?? 0)}
                                    onChange={(event) => updateDraft(server.id, field.key, Number(event.target.value))}
                                  />
                                </label>
                              ))}
                            </div>
                          </section>

                          <section className="advanced-config__section">
                            <span className="eyebrow">Phase Durations</span>
                            <div className="details-grid details-grid--compact">
                              {phaseFields.map((field) => (
                                <label key={field.key}>
                                  <span>{field.label}</span>
                                  <input
                                    min={0}
                                    type="number"
                                    value={Number(configDrafts[server.id]?.[field.key] ?? 0)}
                                    onChange={(event) => updateDraft(server.id, field.key, Number(event.target.value))}
                                  />
                                </label>
                              ))}
                            </div>
                          </section>

                          <section className="advanced-config__section">
                            <span className="eyebrow">Voice</span>
                            <label>
                              <span>VOIP Enabled</span>
                              <div className="segmented-control" role="group" aria-label="VOIP Enabled">
                                <button
                                  className={
                                    configDrafts[server.id]?.voipEnabled
                                      ? 'segmented-control__button segmented-control__button--active'
                                      : 'segmented-control__button'
                                  }
                                  onClick={() => updateDraft(server.id, 'voipEnabled', true)}
                                  type="button"
                                >
                                  Enabled
                                </button>
                                <button
                                  className={
                                    configDrafts[server.id]?.voipEnabled === false
                                      ? 'segmented-control__button segmented-control__button--active'
                                      : 'segmented-control__button'
                                  }
                                  onClick={() => updateDraft(server.id, 'voipEnabled', false)}
                                  type="button"
                                >
                                  Disabled
                                </button>
                              </div>
                            </label>
                          </section>

                          <section className="advanced-config__section">
                            <div className="panel-card__header panel-card__header--tight">
                              <div>
                                <span className="eyebrow">Mods</span>
                                <h4>Workshop configuration</h4>
                              </div>
                              <button className="button button--ghost button--small" onClick={() => addMod(server.id)} type="button">
                                Add mod
                              </button>
                            </div>

                            <div className="mods-list">
                              {(configDrafts[server.id]?.mods ?? []).map((mod, index) => (
                                <article className="mod-row" key={`${server.id}-mod-${index}`}>
                                  <input
                                    placeholder="Workshop ID"
                                    type="text"
                                    value={mod.workshopId}
                                    onChange={(event) => updateMod(server.id, index, 'workshopId', event.target.value)}
                                  />
                                  <label className="toggle-inline">
                                    <input
                                      checked={mod.enabled}
                                      onChange={(event) => updateMod(server.id, index, 'enabled', event.target.checked)}
                                      type="checkbox"
                                    />
                                    <span>Enabled</span>
                                  </label>
                                  <label className="toggle-inline">
                                    <input
                                      checked={mod.clientRequired}
                                      onChange={(event) => updateMod(server.id, index, 'clientRequired', event.target.checked)}
                                      type="checkbox"
                                    />
                                    <span>Client Req.</span>
                                  </label>
                                  <button
                                    className="icon-button icon-button--danger"
                                    onClick={() => removeMod(server.id, index)}
                                    type="button"
                                  >
                                    <TrashIcon />
                                  </button>
                                </article>
                              ))}
                            </div>
                          </section>
                        </>
                      ) : null}
                    </div>
                  </section>
                </article>
              ))
            )}
          </div>
        </section>
      </div>

      {message ? <p className="success-copy">{message}</p> : null}
      {error ? <p className="error-copy">{error}</p> : null}
    </div>
  )
}