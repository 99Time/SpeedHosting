import { useEffect, useRef, useState } from 'react'

import { useAuth } from '../../auth/AuthContext'
import { buildAcquisitionPayload, getPreferredAttributionSource } from '../../lib/attribution'
import { trackEvent } from '../../lib/analytics'
import { createServer } from '../../lib/api'
import { SPEEDRANKEDS_WORKSHOP_ID } from '../../lib/mods'
import type { PlanResponse, Server } from '../../types/api'

type CreateServerModalProps = {
  isOpen: boolean
  onClose: () => void
  planData: PlanResponse
  serverCount: number
  onServerCreated: (server: Server) => void
}

const fixedInfrastructure = {
  region: 'Nuremberg',
  country: 'Germany',
  networkZone: 'eu-central',
}

function createInitialForm(planData: PlanResponse) {
  return {
    name: '',
    desiredTickRate: planData.plan.code === 'free' ? planData.plan.maxTickRate : 120,
    maxPlayers: 10,
    password: '',
    adminSteamIds: '',
    enableSpeedRankeds: false,
  }
}

function parseAdminSteamIds(value: string) {
  return value
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean)
}

export function CreateServerModal({
  isOpen,
  onClose,
  planData,
  serverCount,
  onServerCreated,
}: CreateServerModalProps) {
  const { user } = useAuth()
  const source = getPreferredAttributionSource(user)
  const firstInputRef = useRef<HTMLInputElement>(null)

  const [form, setForm] = useState(() => createInitialForm(planData))
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const limitReached = serverCount >= planData.plan.maxServers
  const maxAdminSteamIds = planData.plan.maxAdminSteamIds ?? planData.plan.maxAdmins ?? 1
  const maxUserConfigurableMods =
    planData.plan.maxUserConfigurableMods ?? (planData.plan.allowCustomMods ? 1 : 0)
  const canToggleSpeedRankeds = planData.plan.allowSpeedRankeds

  useEffect(() => {
    if (isOpen) {
      setForm(createInitialForm(planData))
      setError(null)
      setTimeout(() => firstInputRef.current?.focus(), 80)
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }

    return () => {
      document.body.style.overflow = ''
    }
  }, [isOpen, planData])

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === 'Escape' && !submitting) onClose()
    }

    if (isOpen) {
      document.addEventListener('keydown', onKey)
    }

    return () => {
      document.removeEventListener('keydown', onKey)
    }
  }, [isOpen, onClose, submitting])

  function handleOverlayClick(event: React.MouseEvent<HTMLDivElement>) {
    if (event.target === event.currentTarget && !submitting) onClose()
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (limitReached) return
    setSubmitting(true)
    setError(null)

    try {
      if (serverCount === 0) {
        void trackEvent('first_server_create_click', {
          source,
          route: `${window.location.pathname}${window.location.search}`,
          metadata: { origin: 'create_modal' },
        })
      }

      const response = await createServer({
        name: form.name,
        desiredTickRate: form.desiredTickRate,
        maxPlayers: form.maxPlayers,
        password: form.password,
        adminSteamIds: parseAdminSteamIds(form.adminSteamIds),
        mods: form.enableSpeedRankeds
          ? [{ workshopId: SPEEDRANKEDS_WORKSHOP_ID, enabled: true, clientRequired: false }]
          : [],
        acquisition: buildAcquisitionPayload(user, source),
      })

      onServerCreated(response.server)
      onClose()
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Creation failed')
    } finally {
      setSubmitting(false)
    }
  }

  if (!isOpen) return null

  const planLabel = `${planData.plan.name} plan · ${serverCount}/${planData.plan.maxServers} servers used`
  const planDetails =
    planData.plan.code === 'free'
      ? 'Starter tier with a fixed tick ceiling and a single admin slot.'
      : planData.plan.code === 'pro'
        ? `Paid tier with advanced config, ${maxAdminSteamIds} admin IDs, and ${maxUserConfigurableMods} mod slot.`
        : `Top tier with the highest public limits, ${maxAdminSteamIds} admin IDs, and ${maxUserConfigurableMods} mod slots.`

  return (
    <div
      className="modal-overlay"
      onClick={handleOverlayClick}
      role="presentation"
    >
      <div
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="create-modal-title"
        aria-describedby="create-modal-subtitle"
      >
        <div className="modal__header">
          <div>
            <h2 id="create-modal-title">Create Server</h2>
            <p className="modal__subtitle" id="create-modal-subtitle">{planLabel}</p>
            <p className="muted-copy">{planDetails}</p>
          </div>
          <button
            aria-label="Close"
            className="modal__close"
            onClick={onClose}
            type="button"
            disabled={submitting}
          >
            ✕
          </button>
        </div>

        {limitReached ? (
          <div className="modal__body modal__body--notice">
            <p className="error-copy">
              Server limit reached ({serverCount}/{planData.plan.maxServers}). Upgrade your plan
              to create more servers.
            </p>
            <div className="modal__footer">
              <button className="button button--ghost" onClick={onClose} type="button">
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <form className="modal__body" onSubmit={handleSubmit}>
            <section className="modal__section">
              <div className="modal__section-header">
                <h3>Basic</h3>
                <p>Name the server and confirm the fixed deployment region.</p>
              </div>
              <div className="modal__row">
                <label className="modal__field">
                  <span>Server name</span>
                  <input
                    ref={firstInputRef}
                    required
                    value={form.name}
                    onChange={(event) =>
                      setForm((current) => ({ ...current, name: event.target.value }))
                    }
                    placeholder="Weekend Arena"
                  />
                </label>
                <label className="modal__field">
                  <span>Region</span>
                  <input
                    className="modal__readonly"
                    readOnly
                    type="text"
                    value={`${fixedInfrastructure.region}, ${fixedInfrastructure.country}`}
                  />
                </label>
              </div>
              <section className="modal__infra">
                <span className="topbar__label">Network zone</span>
                <strong>{fixedInfrastructure.networkZone}</strong>
                <p>All new servers are currently provisioned in the EU Central deployment footprint.</p>
              </section>
            </section>

            <section className="modal__section">
              <div className="modal__section-header">
                <h3>Gameplay</h3>
                <p>Set the match pacing and player capacity allowed on your current plan.</p>
              </div>
              <div className="modal__row">
                <label className="modal__field">
                  <span>Tick rate</span>
                  <input
                    className={planData.plan.code === 'free' ? 'modal__readonly' : undefined}
                    type="number"
                    min={30}
                    max={planData.plan.maxTickRate}
                    value={form.desiredTickRate}
                    readOnly={planData.plan.code === 'free'}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        desiredTickRate: Number(event.target.value),
                      }))
                    }
                  />
                  {planData.plan.code === 'free' ? (
                    <span className="modal__field-note">Fixed on the Free plan at {planData.plan.maxTickRate} Hz.</span>
                  ) : null}
                </label>
                <label className="modal__field">
                  <span>Max players</span>
                  <input
                    type="number"
                    min={2}
                    max={16}
                    value={form.maxPlayers}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        maxPlayers: Number(event.target.value),
                      }))
                    }
                  />
                </label>
              </div>
            </section>

            <section className="modal__section">
              <div className="modal__section-header">
                <h3>Access</h3>
                <p>Control who can join and who can administer the server once it is live.</p>
              </div>
              <div className="modal__stack">
                <label className="modal__field">
                  <span>Password</span>
                  <input
                    placeholder="Optional server password"
                    type="text"
                    value={form.password}
                    onChange={(event) =>
                      setForm((current) => ({ ...current, password: event.target.value }))
                    }
                  />
                </label>

                {maxAdminSteamIds > 0 ? (
                  <label className="modal__field">
                    <span>Admin IDs</span>
                    <input
                      placeholder="7656119..."
                      type="text"
                      value={form.adminSteamIds}
                      onChange={(event) =>
                        setForm((current) => ({ ...current, adminSteamIds: event.target.value }))
                      }
                    />
                    <span className="modal__field-note">
                      Up to {maxAdminSteamIds} admin Steam ID{maxAdminSteamIds === 1 ? '' : 's'}, comma separated.
                    </span>
                  </label>
                ) : null}
              </div>
            </section>

            {canToggleSpeedRankeds ? (
              <section className="modal__section">
                <div className="modal__section-header">
                  <h3>Mods</h3>
                  <p>Turn on plan-supported built-in modules during provisioning.</p>
                </div>
                <div className="switch-field switch-field--modal">
                  <div className="switch-field__copy">
                    <span className="switch-field__label">Enable SpeedRankeds</span>
                    <p className="switch-field__hint">
                      Adds ranked matchmaking and stat tracking.
                      {planCodeNote(planData.plan.code, maxUserConfigurableMods)}
                    </p>
                  </div>
                  <button
                    aria-checked={form.enableSpeedRankeds}
                    className={
                      form.enableSpeedRankeds
                        ? 'switch-field__control switch-field__control--active'
                        : 'switch-field__control'
                    }
                    onClick={() =>
                      setForm((current) => ({
                        ...current,
                        enableSpeedRankeds: !current.enableSpeedRankeds,
                      }))
                    }
                    role="switch"
                    type="button"
                  >
                    <span className="switch-field__thumb" />
                  </button>
                </div>
              </section>
            ) : null}

            {error ? <p className="error-copy">{error}</p> : null}

            <div className="modal__footer">
              <button
                className="button button--ghost"
                type="button"
                onClick={onClose}
                disabled={submitting}
              >
                Cancel
              </button>
              <button
                className="button button--primary"
                type="submit"
                disabled={submitting}
              >
                {submitting ? 'Creating Server...' : 'Create Server'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

function planCodeNote(planCode: string, maxUserConfigurableMods: number) {
  if (planCode === 'pro') {
    return ' Uses your single Pro mod slot.'
  }

  if (planCode === 'premium') {
    return ` Counts toward your ${maxUserConfigurableMods} Premium mod slots.`
  }

  return ''
}
