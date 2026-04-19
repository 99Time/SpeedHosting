import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'

import { AuthProvider } from '../auth/AuthContext'
import { AdminRoute, ProtectedRoute, PublicOnlyRoute } from '../auth/RouteGuards'
import { AppShell } from '../components/layout/AppShell'
import { AccountPlanPage } from '../pages/AccountPlanPage'
import { AdminPage } from '../pages/AdminPage'
import { DashboardPage } from '../pages/DashboardPage'
import { LandingPage } from '../pages/LandingPage'
import { LoginPage } from '../pages/LoginPage'
import { MyServerPage } from '../pages/MyServerPage'
import { PuckLandingPage } from '../pages/PuckLandingPage'
import { RegisterPage } from '../pages/RegisterPage'
import { UpdatesPage } from '../pages/UpdatesPage'

export function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<LandingPage />} />
          <Route path="/puck" element={<PuckLandingPage />} />
          <Route path="/puck/:sourceAlias" element={<PuckLandingPage />} />
          <Route
            path="/login"
            element={
              <PublicOnlyRoute>
                <LoginPage />
              </PublicOnlyRoute>
            }
          />
          <Route
            path="/register"
            element={
              <PublicOnlyRoute>
                <RegisterPage />
              </PublicOnlyRoute>
            }
          />

          <Route
            path="/updates"
            element={
              <ProtectedRoute>
                <Navigate replace to="/app/updates" />
              </ProtectedRoute>
            }
          />
          <Route
            path="/changelog"
            element={
              <ProtectedRoute>
                <Navigate replace to="/app/updates" />
              </ProtectedRoute>
            }
          />

          <Route
            path="/app"
            element={
              <ProtectedRoute>
                <AppShell />
              </ProtectedRoute>
            }
          >
            <Route index element={<DashboardPage />} />
            <Route path="servers" element={<MyServerPage />} />
            <Route path="updates" element={<UpdatesPage />} />
            <Route path="account" element={<AccountPlanPage />} />
            <Route
              path="admin"
              element={
                <AdminRoute>
                  <AdminPage />
                </AdminRoute>
              }
            />
          </Route>

          <Route path="*" element={<Navigate replace to="/" />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
