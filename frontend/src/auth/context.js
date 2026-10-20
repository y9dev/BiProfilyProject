// Auth context object + hook (provider lives in AuthContext.jsx).
import { createContext, useContext } from 'react'

export const AuthContext = createContext(null)

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}

export const MANAGER_ROLES = ['manager', 'admin']
