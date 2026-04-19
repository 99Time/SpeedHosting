import { useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'

import { useAuth } from '../../auth/AuthContext'

function DashIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
      <rect height="7" rx="2" stroke="currentColor" strokeWidth="1.8" width="7" x="3" y="3" />
      <rect height="7" rx="2" stroke="currentColor" strokeWidth="1.8" width="7" x="14" y="3" />
      <rect height="7" rx="2" stroke="currentColor" strokeWidth="1.8" width="7" x="3" y="14" />
      <rect height="7" rx="2" stroke="currentColor" strokeWidth="1.8" width="7" x="14" y="14" />
    </svg>
  )
}

function ServersIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
      <rect height="6" rx="2" stroke="currentColor" strokeWidth="1.8" width="18" x="3" y="4" />
      <rect height="6" rx="2" stroke="currentColor" strokeWidth="1.8" width="18" x="3" y="14" />
      <circle cx="7.5" cy="7" fill="currentColor" r="1" />
      <circle cx="7.5" cy="17" fill="currentColor" r="1" />
    </svg>
  )
}

function UpdatesIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
      <circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="1.8" />
      <path d="M12 2v3M12 19v3M4.22 4.22l2.12 2.12M17.66 17.66l2.12 2.12M2 12h3M19 12h3M4.22 19.78l2.12-2.12M17.66 6.34l2.12-2.12" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
    </svg>
  )
}

function AccountIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
      <circle cx="12" cy="8" r="4" stroke="currentColor" strokeWidth="1.8" />
      <path d="M4 20c0-4 3.58-7 8-7s8 3 8 7" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
    </svg>
  )
}

function AdminIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
      <path d="M12 3l8 4v5c0 4.4-3.4 8.5-8 9.4C7.4 20.5 4 16.4 4 12V7l8-4Z" stroke="currentColor" strokeWidth="1.8" />
    </svg>
  )
}

function AnalyticsIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="14" viewBox="0 0 24 24" width="14">
      <path d="M3 17l6-6 4 4 8-8" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" />
    </svg>
  )
}

function ChevronRightIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="12" viewBox="0 0 24 24" width="12">
      <path d="M9 6l6 6-6 6" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
    </svg>
  )
}

function HomeIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="14" viewBox="0 0 24 24" width="14">
      <path d="M4 10.5 12 4l8 6.5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.9" />
      <path d="M6.5 9.5V20h11V9.5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.9" />
    </svg>
  )
}

function SignOutIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="14" viewBox="0 0 24 24" width="14">
      <path d="M17 8l4 4-4 4" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.9" />
      <path d="M21 12H9" stroke="currentColor" strokeLinecap="round" strokeWidth="1.9" />
      <path d="M9 4H4a1 1 0 0 0-1 1v14a1 1 0 0 0 1 1h5" stroke="currentColor" strokeLinecap="round" strokeWidth="1.9" />
    </svg>
  )
}

function BrandMark() {
  return (
    <div className="app-brand__mark">
      <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
        <path d="M12 2L3 7v5c0 4.8 3.8 9.3 9 10.3 5.2-1 9-5.5 9-10.3V7L12 2Z" fill="currentColor" opacity="0.9" />
      </svg>
    </div>
  )
}

function pageTitle(pathname: string) {
  if (pathname.startsWith('/app/updates')) return "What's New"
  if (pathname.startsWith('/app/servers')) return 'My Servers'
  if (pathname.startsWith('/app/admin')) return 'Admin Panel'
  if (pathname.startsWith('/app/account')) return 'Account'
  return 'Dashboard'
}

export function AppShell() {
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useAuth()
  const [isSigningOut, setIsSigningOut] = useState(false)
  const isAdmin = user?.role === 'admin'
  const [adminExpanded, setAdminExpanded] = useState(
    location.pathname.startsWith('/app/admin'),
  )

  const initials = user?.displayName
    ? user.displayName.slice(0, 2).toUpperCase()
    : (user?.email?.slice(0, 2).toUpperCase() ?? '??')

  async function handleSignOut() {
    setIsSigningOut(true)
    try {
      await logout()
      navigate('/login')
    } finally {
      setIsSigningOut(false)
    }
  }

  const title = pageTitle(location.pathname)

  return (
    <div className="shell">
      <aside className="shell__sidebar">
        <div>
          {/* Compact brand */}
          <div className="app-brand">
            <BrandMark />
            <div>
              <div className="app-brand__name">SpeedHosting</div>
              <div className="app-brand__sub">PUCK server platform</div>
            </div>
          </div>

          {/* Navigation */}
          <nav className="sidebar-nav" aria-label="Primary">
            <NavLink
              end
              to="/app"
              className={({ isActive }) =>
                isActive ? 'sidebar-nav__link sidebar-nav__link--active' : 'sidebar-nav__link'
              }
            >
              <span className="sidebar-nav__icon"><DashIcon /></span>
              <span className="sidebar-nav__text">Dashboard</span>
            </NavLink>

            <NavLink
              to="/app/servers"
              className={({ isActive }) =>
                isActive ? 'sidebar-nav__link sidebar-nav__link--active' : 'sidebar-nav__link'
              }
            >
              <span className="sidebar-nav__icon"><ServersIcon /></span>
              <span className="sidebar-nav__text">My Servers</span>
            </NavLink>

            <NavLink
              to="/app/updates"
              className={({ isActive }) =>
                isActive ? 'sidebar-nav__link sidebar-nav__link--active' : 'sidebar-nav__link'
              }
            >
              <span className="sidebar-nav__icon"><UpdatesIcon /></span>
              <span className="sidebar-nav__text">What's New</span>
              <span className="sidebar-nav__badge">NEW</span>
            </NavLink>

            <NavLink
              to="/app/account"
              className={({ isActive }) =>
                isActive ? 'sidebar-nav__link sidebar-nav__link--active' : 'sidebar-nav__link'
              }
            >
              <span className="sidebar-nav__icon"><AccountIcon /></span>
              <span className="sidebar-nav__text">Account</span>
            </NavLink>

            {isAdmin ? (
              <div className="sidebar-nav__group">
                <button
                  aria-expanded={adminExpanded}
                  className={`sidebar-nav__parent ${location.pathname.startsWith('/app/admin') ? 'sidebar-nav__parent--active' : ''}`}
                  onClick={() => setAdminExpanded((prev) => !prev)}
                  type="button"
                >
                  <span className="sidebar-nav__icon"><AdminIcon /></span>
                  <span className="sidebar-nav__text">Admin</span>
                  <span className={`sidebar-nav__chevron ${adminExpanded ? 'sidebar-nav__chevron--open' : ''}`}>
                    <ChevronRightIcon />
                  </span>
                </button>
                <div className={`sidebar-nav__sub ${adminExpanded ? 'sidebar-nav__sub--open' : ''}`}>
                  <NavLink
                    end
                    to="/app/admin"
                    className={({ isActive }) =>
                      isActive ? 'sidebar-nav__sub-link sidebar-nav__sub-link--active' : 'sidebar-nav__sub-link'
                    }
                  >
                    <AdminIcon />
                    <span>Panel</span>
                  </NavLink>
                  <NavLink
                    to="/app/admin/analytics"
                    className={({ isActive }) =>
                      isActive ? 'sidebar-nav__sub-link sidebar-nav__sub-link--active' : 'sidebar-nav__sub-link'
                    }
                  >
                    <AnalyticsIcon />
                    <span>Analytics</span>
                  </NavLink>
                </div>
              </div>
            ) : null}
          </nav>
        </div>

        {/* User card */}
        <div className="sidebar-user">
          <div className="sidebar-user__avatar">{initials}</div>
          <div className="sidebar-user__info">
            <div className="sidebar-user__name">{user?.displayName ?? user?.email}</div>
            <div className="sidebar-user__role">{user?.planCode?.toUpperCase()} plan</div>
          </div>
          <button
            className="sidebar-user__sign-out"
            onClick={handleSignOut}
            title={isSigningOut ? 'Signing out…' : 'Sign out'}
            type="button"
          >
            <SignOutIcon />
          </button>
        </div>
      </aside>

      <div className="shell__main">
        <header className="topbar">
          <div>
            <Link className="topbar-home-link" to="/">
              <HomeIcon />
              <span>Home</span>
            </Link>
            <h2>{title}</h2>
          </div>
          <div className="topbar__actions">
            <div className="topbar__meta">
              <div>
                <span className="topbar__label">Signed in as</span>
                <strong>{user?.email}</strong>
              </div>
              <div>
                <span className="topbar__label">Current plan</span>
                <strong>{user?.planCode?.toUpperCase()}</strong>
              </div>
            </div>
          </div>
        </header>

        <main className="page-content">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
