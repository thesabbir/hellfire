import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'
import { useNavigate } from '@tanstack/react-router'
import Cookies from 'js-cookie'
import { postAuthLogin, postAuthLogout, getAuthMe } from './api'
import type { DbUser } from './api/types.gen'
import { AUTH_COOKIE_NAME } from './api-client'

interface AuthContextType {
  user: DbUser | null
  loading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  isAuthenticated: boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<DbUser | null>(null)
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()

  useEffect(() => {
    checkAuth()
  }, [])

  async function checkAuth() {
    try {
      const token = Cookies.get(AUTH_COOKIE_NAME)
      if (!token) {
        setLoading(false)
        return
      }

      const response = await getAuthMe()
      if (response.data) {
        setUser(response.data)
      } else {
        Cookies.remove(AUTH_COOKIE_NAME)
      }
    } catch {
      Cookies.remove(AUTH_COOKIE_NAME)
    } finally {
      setLoading(false)
    }
  }

  async function login(username: string, password: string) {
    const response = await postAuthLogin({
      body: { username, password },
    })

    if (response.error) {
      const errorObj = response.error as { error?: string }
      throw new Error(errorObj?.error || 'Login failed')
    }

    if (response.data?.token) {
      // Set cookie with secure options
      Cookies.set(AUTH_COOKIE_NAME, response.data.token, {
        expires: 7, // 7 days
        sameSite: 'strict',
        secure: window.location.protocol === 'https:',
      })
      setUser(response.data.user || null)
      navigate({ to: '/' })
    }
  }

  async function logout() {
    try {
      await postAuthLogout()
    } catch {
      // Ignore logout errors
    } finally {
      Cookies.remove(AUTH_COOKIE_NAME)
      setUser(null)
      navigate({ to: '/login' })
    }
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        login,
        logout,
        isAuthenticated: !!user,
      }}
    >
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
