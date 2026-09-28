import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useMemo, useState, type ReactNode } from 'react'

import type { Session } from '../../api/types'
import { AuthContext } from './authContext'

export const SESSION_KEY = 'bia.auth'

// Storage can be unavailable (private mode, blocked site data): the session then lives in memory only.
function readSession(): Session | null {
  try {
    const raw = localStorage.getItem(SESSION_KEY)
    return raw ? (JSON.parse(raw) as Session) : null
  } catch {
    return null
  }
}

function writeSession(session: Session | null): void {
  try {
    if (session) localStorage.setItem(SESSION_KEY, JSON.stringify(session))
    else localStorage.removeItem(SESSION_KEY)
  } catch {
    // Keep the in-memory session.
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [session, setSession] = useState<Session | null>(readSession)

  const signIn = useCallback((s: Session) => {
    writeSession(s)
    setSession(s)
  }, [])

  const signOut = useCallback(() => {
    writeSession(null)
    setSession(null)
    queryClient.clear()
  }, [queryClient])

  const value = useMemo(() => ({ session, signIn, signOut }), [session, signIn, signOut])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
