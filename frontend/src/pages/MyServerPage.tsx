import { useEffect, useState } from 'react'

import { useAuth } from '../auth/AuthContext'
import { CreateServerModal } from '../components/common/CreateServerModal'
import { StatusBadge } from '../components/common/StatusBadge'
import { deleteServer, getPlan, getServers, runServerAction, updateServerConfig } from '../lib/api'
import { hasWorkshopMod, SPEEDRANKEDS_WORKSHOP_ID, toggleWorkshopMod } from '../lib/mods'
import type { PlanResponse, Server, ServerConfig, ServerConfigMod } from '../types/api'

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

function adminSteamIdsToText(values: string[]) {
  return values.join(', ')
}

function buildDraftMap(servers: Server[]) {
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

function ServerStackIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="28" viewBox="0 0 24 24" width="28">
      <rect height="5" rx="1.5" stroke="currentColor" strokeWidth="1.7" width="16" x="4" y="4" />
      <rect height="5" rx="1.5" stroke="currentColor" strokeWidth="1.7" width="16" x="4" y="10" />
      <rect height="5" rx="1.5" stroke="currentColor" strokeWidth="1.7" width="16" x="4" y="16" />
      <path d="M8 6.5h.01M8 12.5h.01M8 18.5h.01" stroke="currentColor" strokeLinecap="round" strokeWidth="2.2" />
    </svg>
  )
}

export function MyServerPage() {
  const { user } = useAuth()
  const [planData, setPlanData] = useState<PlanResponse | null>(null)
  const [servers, setServers] = useState<Server[]>([])
  const [error, setError] = useState<string | null>(null)
  const [busyAction, setBusyAction] = useState<string | null>(null)
  const [busyDeleteId, setBusyDeleteId] = useState<number | null>(null)
  const [savingConfigId, setSavingConfigId] = useState<number | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [loaded, setLoaded] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState<Record<number, boolean>>({})
  const [configDrafts, setConfigDrafts] = useState<Record<number, ServerConfig>>({})
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)

  useEffect(() => {
    let active = true

    Promise.all([getServers(), getPlan()])
      .then(([serversResponse, planResponse]) => {
        if (active) {
          setServers(serversResponse.servers)
          setConfigDrafts(buildDraftMap(serversResponse.servers))
          setPlanData(planResponse)
        }
      })
      .catch((requestError: Error) => {
        if (active) {
          setError(requestError.message)
        }
      })
      .finally(() => {
        if (active) {
          setLoaded(true)
        }
      })

    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    if (!message) {
      return
    }

    const timeoutId = window.setTimeout(() => {
      setMessage(null)
    }, 4200)

    return () => {
      window.clearTimeout(timeoutId)
    }
  }, [message])

  const limitReached = planData !== null && planData.usage.serverCount >= planData.plan.maxServers
  const maxAdminSteamIds = planData?.plan.maxAdminSteamIds ?? planData?.plan.maxAdmins ?? 1
  const maxUserConfigurableMods = planData?.plan.maxUserConfigurableMods ?? (planData?.plan.allowCustomMods ? 1 : 0)

  function syncServer(nextServer: Server) {
    setServers((currentServers) =>
      currentServers.map((currentServer) => (currentServer.id === nextServer.id ? nextServer : currentServer)),
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

  function toggleSpeedRankeds(serverId: number, enabled: boolean) {
    setConfigDrafts((currentDrafts) => ({
      ...currentDrafts,
      [serverId]: {
        ...currentDrafts[serverId],
        mods: toggleWorkshopMod(currentDrafts[serverId]?.mods ?? [], SPEEDRANKEDS_WORKSHOP_ID, enabled),
      },
    }))
  }

  function handleServerCreated(newServer: Server) {
    setServers((current) => [newServer, ...current])
    setConfigDrafts((current) => ({
      ...current,
      [newServer.id]: {
        ...newServer.config,
        adminSteamIds: [...newServer.config.adminSteamIds],
        mods: [...newServer.config.mods],
      },
    }))
    setPlanData((current) =>
      current
        ? { ...current, usage: { serverCount: current.usage.serverCount + 1 } }
        : current,
    )
    setMessage(`${newServer.name} is provisioned and ready to start.`)
  }

  async function handleAction(serverId: number, action: 'start' | 'stop' | 'restart') {
    setBusyAction(`${serverId}:${action}`)
    setError(null)

    try {
      const response = await runServerAction(serverId, action)
      syncServer(response.server)
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : 'Action failed'
      setError(message)
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
      const response = await updateServerConfig(serverId, draft)
      syncServer(response.server)
      setMessage(`Configuration for ${response.server.name} was saved.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Config update failed')
    } finally {
      setSavingConfigId(null)
    }
  }

  async function handleDelete(serverId: number, serverName: string) {
    if (!window.confirm(`Delete ${serverName}? This will stop the service and remove its JSON config.`)) {
      return
    }

    setBusyDeleteId(serverId)
    setError(null)
    setMessage(null)

    try {
      await deleteServer(serverId)
      setServers((currentServers) => currentServers.filter((server) => server.id !== serverId))
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
      setPlanData((current) =>
        current
          ? {
              ...current,
              usage: {
                serverCount: Math.max(0, current.usage.serverCount - 1),
              },
            }
          : current,
      )
      setMessage(`Server ${serverName} was deleted.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Delete failed')
    } finally {
      setBusyDeleteId(null)
    }
  }

  if (!loaded && !error) {
    return (
      <div className="page-grid">
        <div className="panel-card dash-skel" style={{ minHeight: 64 }} />
        {[0, 1].map((i) => (
          <div className="panel-card dash-skel" key={i} style={{ minHeight: 180 }} />
        ))}
      </div>
    )
  }

  return (
    <>
      {message ? (
        <div className="toast-stack" aria-live="polite">
          <div className="toast toast--success" role="status">
            <span className="toast__icon" aria-hidden="true">●</span>
            <div>
              <strong>Success</strong>
              <p>{message}</p>
            </div>
          </div>
        </div>
      ) : null}

      <div className="page-grid">
        <section className="panel-card">
          <div className="panel-card__header--tight">
            <span className="servers-header-title">Servers
              {planData ? (
                <span className="servers-counter">{planData.usage.serverCount}/{planData.plan.maxServers}</span>
              ) : null}
            </span>
            <button
              className="button button--primary button--small"
              disabled={limitReached || !planData}
              onClick={() => setIsCreateModalOpen(true)}
              type="button"
            >
              New Server
            </button>
          </div>

          <div className="stack-grid">
            {servers.length === 0 ? (
              <div className="empty-state">
                <div className="empty-state__icon" aria-hidden="true">
                  <ServerStackIcon />
                </div>
                <h4>No servers yet</h4>
                <button
                  className="button button--primary"
                  disabled={!planData || limitReached}
                  onClick={() => setIsCreateModalOpen(true)}
                  type="button"
                >
                  New Server
                </button>
              </div>
            ) : (
              servers.map((server) => (
                <article className="server-card" key={server.id}>
                  <div className="panel-card__header panel-card__header--tight">
                    <div>
                      <span className="topbar__label">{server.region} · eu-central</span>
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
                      <span className="topbar__label">Tick rate</span>
                      <strong>{server.desiredTickRate}</strong>
                    </div>
                    <div>
                      <span className="topbar__label">Players</span>
                      <strong>
                        {server.playerCount}/{server.maxPlayers}
                      </strong>
                    </div>
                    {user?.role === 'admin' && server.configFilePath ? (
                      <>
                        <div>
                          <span className="topbar__label">Config file</span>
                          <strong>{server.configFilePath}</strong>
                        </div>
                        <div>
                          <span className="topbar__label">Service unit</span>
                          <strong>{server.serviceName}</strong>
                        </div>
                      </>
                    ) : null}
                  </div>

                  <div className="action-row action-row--compact">
                    <button
                      className="button button--primary"
                      onClick={() => handleAction(server.id, 'start')}
                      disabled={busyAction !== null}
                      type="button"
                    >
                      {busyAction === `${server.id}:start` ? 'Starting...' : 'Start'}
                    </button>
                    <button
                      className="button button--ghost"
                      onClick={() => handleAction(server.id, 'restart')}
                      disabled={busyAction !== null}
                      type="button"
                    >
                      {busyAction === `${server.id}:restart` ? 'Restarting...' : 'Restart'}
                    </button>
                    <button
                      className="button button--ghost"
                      onClick={() => handleAction(server.id, 'stop')}
                      disabled={busyAction !== null}
                      type="button"
                    >
                      {busyAction === `${server.id}:stop` ? 'Stopping...' : 'Stop'}
                    </button>
                  </div>

                  {server.lastActionError ? (
                    <p className="error-copy">Last runtime error: {server.lastActionError}</p>
                  ) : null}

                  <section className="server-card__config">
                    <div className="panel-card__header panel-card__header--tight">
                      <div>
                        <span className="eyebrow">Access</span>
                        <h4>Server access and config</h4>
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
                        max={16}
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
                      onChange={(event) =>
                        updateDraft(server.id, 'adminSteamIds', event.target.value.split(',').map((s) => s.trim()).filter(Boolean))
                      }
                    />
                  </label>
                  <p className="muted-copy">
                    Your current plan allows up to {maxAdminSteamIds} admin Steam ID{maxAdminSteamIds > 1 ? 's' : ''}.
                  </p>

                    {planData?.plan.allowAdvancedConfig ? (
                      <div className="advanced-config">
                        <label className="toggle-inline toggle-inline--checkbox toggle-inline--panel">
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
                          <span className="toggle-inline__label">Show advanced settings</span>
                        </label>

                        {showAdvanced[server.id] ? (
                          <>
                            <section className="advanced-config__section">
                              <span className="eyebrow">Server Behavior</span>
                              <div className="toggle-grid">
                                {booleanFields.map((field) => (
                                  <label className="toggle-field" key={field.key}>
                                    <input
                                      checked={Boolean(configDrafts[server.id]?.[field.key])}
                                      onChange={(event) => updateDraft(server.id, field.key, event.target.checked)}
                                      type="checkbox"
                                    />
                                    <span className="toggle-field__label">{field.label}</span>
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

                          {planData?.plan.allowCustomMods ? (
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

                              <p className="muted-copy">
                                Built-in system mods stay managed by SpeedHosting and do not count toward your {maxUserConfigurableMods} user-configurable mod slot{maxUserConfigurableMods === 1 ? '' : 's'}.
                              </p>

                              {planData?.plan.allowSpeedRankeds ? (
                                <div className="switch-field switch-field--inline">
                                  <div className="switch-field__copy">
                                    <span className="switch-field__label">Enable SpeedRankeds</span>
                                    <p className="switch-field__hint">Adds ranked matchmaking and stat tracking.</p>
                                  </div>
                                  <button
                                    aria-checked={hasWorkshopMod(configDrafts[server.id]?.mods ?? [], SPEEDRANKEDS_WORKSHOP_ID)}
                                    className={
                                      hasWorkshopMod(configDrafts[server.id]?.mods ?? [], SPEEDRANKEDS_WORKSHOP_ID)
                                        ? 'switch-field__control switch-field__control--active'
                                        : 'switch-field__control'
                                    }
                                    onClick={() =>
                                      toggleSpeedRankeds(
                                        server.id,
                                        !hasWorkshopMod(configDrafts[server.id]?.mods ?? [], SPEEDRANKEDS_WORKSHOP_ID),
                                      )
                                    }
                                    role="switch"
                                    type="button"
                                  >
                                    <span className="switch-field__thumb" />
                                  </button>
                                </div>
                              ) : (
                                <p className="muted-copy">SpeedRankeds is unavailable on this plan.</p>
                              )}

                              <div className="mods-list">
                                {(configDrafts[server.id]?.mods ?? []).map((mod, index) => (
                                  <article className="mod-row" key={`${server.id}-mod-${index}`}>
                                    <input
                                      placeholder="Workshop ID"
                                      type="text"
                                      disabled={mod.workshopId === SPEEDRANKEDS_WORKSHOP_ID}
                                      value={mod.workshopId}
                                      onChange={(event) => updateMod(server.id, index, 'workshopId', event.target.value)}
                                    />
                                    <label className="toggle-inline toggle-inline--checkbox">
                                      <input
                                        checked={mod.enabled}
                                        onChange={(event) => updateMod(server.id, index, 'enabled', event.target.checked)}
                                        type="checkbox"
                                      />
                                      <span className="toggle-inline__label">Enabled</span>
                                    </label>
                                    <label className="toggle-inline toggle-inline--checkbox">
                                      <input
                                        checked={mod.clientRequired}
                                        onChange={(event) =>
                                          updateMod(server.id, index, 'clientRequired', event.target.checked)
                                        }
                                        type="checkbox"
                                      />
                                      <span className="toggle-inline__label">Client Req.</span>
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
                          ) : null}
                          </>
                        ) : null}
                      </div>
                    ) : null}
                  </section>
              </article>
            ))
          )}
        </div>

        {error ? <p className="error-copy">{error}</p> : null}
      </section>
      </div>

    {planData ? (
      <CreateServerModal
        isOpen={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        planData={planData}
        serverCount={servers.length}
        onServerCreated={handleServerCreated}
      />
    ) : null}
    </>
  )
}
