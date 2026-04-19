import { Navigate, useLocation } from 'react-router-dom'

import { useAuth } from './AuthContext'

function AuthGateMessage({ message }: { message: string }) {
  return (
    <div className="auth-page">
      <section className="auth-card auth-card--compact">
        <span className="pill">SpeedHosting</span>
        <h1>{message}</h1>
      </section>
    </div>
  )
}

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <AuthGateMessage message="Loading your session" />
  }

  if (!user) {
    return <Navigate replace state={{ from: location.pathname }} to="/login" />
  }

  return <>{children}</>
}

export function AdminRoute({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <AuthGateMessage message="Checking admin access" />
  }

  if (!user) {
    return <Navigate replace to="/login" />
  }

  if (user.role !== 'admin') {
    return <Navigate replace to="/app" />
  }

  return <>{children}</>
}

export function PublicOnlyRoute({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <AuthGateMessage message="Preparing SpeedHosting" />
  }

  if (user) {
    const target = (location.state as { from?: string } | null)?.from ?? '/app'
    return <Navigate replace to={target} />
  }

  return <>{children}</>
}