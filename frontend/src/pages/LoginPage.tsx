import { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'

import { useAuth } from '../auth/AuthContext'
import { buildAcquisitionPayload, getPreferredAttributionSource } from '../lib/attribution'
import { trackEvent } from '../lib/analytics'
import { ApiError } from '../lib/api'

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { login } = useAuth()
  const [form, setForm] = useState({ email: '', password: '' })
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const source = getPreferredAttributionSource()

  const target =
    (location.state as { from?: string } | null)?.from ??
    (source === 'hostcommand' ? '/app/servers' : '/app')

  useEffect(() => {
    void trackEvent('login_view', {
      source,
      route: `${location.pathname}${location.search}`,
    })
  }, [location.pathname, location.search, source])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setError(null)

    try {
      await login({
        ...form,
        acquisition: buildAcquisitionPayload(undefined, source),
      })
      navigate(target, { replace: true })
    } catch (requestError) {
      if (requestError instanceof ApiError) {
        setError(requestError.message)
      } else {
        setError('Unable to sign in right now')
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
          <h2>Welcome back</h2>
          <p>Sign in to manage your PUCK servers.</p>
        </div>

        <form className="form-grid" onSubmit={handleSubmit}>
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
              autoComplete="current-password"
              onChange={(event) => setForm((current) => ({ ...current, password: event.target.value }))}
              placeholder="Enter your password"
              required
              type="password"
              value={form.password}
            />
          </label>
          <button className="button button--primary button--full" disabled={submitting} type="submit">
            {submitting ? 'Signing in…' : 'Sign in'}
          </button>
        </form>

        {error ? <p className="error-copy">{error}</p> : null}

        <div className="auth-sign-card__footer">
          <span>Need an account?</span>
          <Link to="/register">Create one</Link>
        </div>
      </section>
    </div>
  )
}
