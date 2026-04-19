import { createContext, useContext, useEffect, useState } from 'react'

import { ApiError, getMe, loginUser, logoutUser, registerUser } from '../lib/api'
import type { AcquisitionPayload, User } from '../types/api'

type AuthContextValue = {
  user: User | null
  isLoading: boolean
  login: (input: { email: string; password: string; acquisition?: AcquisitionPayload }) => Promise<User>
  register: (input: { displayName: string; email: string; password: string; acquisition?: AcquisitionPayload }) => Promise<User>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    let active = true

    getMe()
      .then((response) => {
        if (active) {
          setUser(response.user)
        }
      })
      .catch((error: unknown) => {
        if (!active) {
          return
        }

        if (error instanceof ApiError && error.status === 401) {
          setUser(null)
          return
        }

        setUser(null)
      })
      .finally(() => {
        if (active) {
          setIsLoading(false)
        }
      })

    return () => {
      active = false
    }
  }, [])

  async function login(input: { email: string; password: string; acquisition?: AcquisitionPayload }) {
    const response = await loginUser(input)
    setUser(response.user)
    return response.user
  }

  async function register(input: { displayName: string; email: string; password: string; acquisition?: AcquisitionPayload }) {
    const response = await registerUser(input)
    setUser(response.user)
    return response.user
  }

  async function logout() {
    try {
      await logoutUser()
    } finally {
      setUser(null)
    }
  }

  return (
    <AuthContext.Provider value={{ user, isLoading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }

  return context
}