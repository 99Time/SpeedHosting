import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import { useAuth } from '../auth/AuthContext'
import { buildAcquisitionPayload, getPreferredAttributionSource } from '../lib/attribution'
import { trackEvent } from '../lib/analytics'
import { ApiError } from '../lib/api'

export function RegisterPage() {
  const navigate = useNavigate()
  const { register } = useAuth()
  const [form, setForm] = useState({ displayName: '', email: '', password: '' })
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const source = getPreferredAttributionSource()

  useEffect(() => {
    void trackEvent('register_view', {
      source,
      route: `${window.location.pathname}${window.location.search}`,
    })
  }, [source])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setError(null)

    try {
      const acquisition = buildAcquisitionPayload(undefined, source)
      void trackEvent('register_submit', {
        source,
        route: `${window.location.pathname}${window.location.search}`,
      })

      await register({ ...form, acquisition })
      navigate(source === 'hostcommand' ? '/app/servers' : '/app', { replace: true })
    } catch (requestError) {
      if (requestError instanceof ApiError) {
        setError(requestError.message)
      } else {
        setError('Unable to create your account right now')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-sign-page">
      <section className="auth-sign-card">
        <div className="auth-sign-card__brand">
          <div className="app-brand__mark">
            <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
              <path d="M12 2L3 7v5c0 4.8 3.8 9.3 9 10.3 5.2-1 9-5.5 9-10.3V7L12 2Z" fill="currentColor" opacity="0.9" />
            </svg>
          </div>
          <span className="app-brand__name">SpeedHosting</span>
        </div>

        <div className="auth-sign-card__header">
          <h2>Create account</h2>
          <p>Your first server starts on the Free plan. Upgrade later.</p>
        </div>

        <form className="form-grid" onSubmit={handleSubmit}>
          <label>
            <span>Display name</span>
            <input
              autoComplete="nickname"
              onChange={(event) =>
                setForm((current) => ({ ...current, displayName: event.target.value }))
              }
              placeholder="Server owner"
              required
              type="text"
              value={form.displayName}
            />
          </label>
          <label>
            <span>Email</span>
            <input
              autoComplete="email"
              onChange={(event) => setForm((current) => ({ ...current, email: event.target.value }))}
              placeholder="you@example.com"
              required
              type="email"
              value={form.email}
            />
          </label>
          <label>
            <span>Password</span>
            <input
              autoComplete="new-password"
              minLength={8}
              onChange={(event) =>
                setForm((current) => ({ ...current, password: event.target.value }))
              }
              placeholder="Choose a strong password (8+ chars)"
              required
              type="password"
              value={form.password}
            />
          </label>
          <button className="button button--primary button--full" disabled={submitting} type="submit">
            {submitting ? 'Creating account…' : 'Create account'}
          </button>
        </form>

        {error ? <p className="error-copy">{error}</p> : null}

        <div className="auth-sign-card__footer">
          <span>Already registered?</span>
          <Link to="/login">Sign in</Link>
        </div>
      </section>
    </div>
  )
}
