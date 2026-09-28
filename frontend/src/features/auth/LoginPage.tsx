import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router'

import { isApiError } from '../../api/client'
import { useLogin } from '../../api/queries'
import logo from '../../assets/logo.png'
import { Button } from '../../components/Button'
import { useAuth } from './authContext'

const inputClass =
  'w-full rounded-lg border border-border bg-bg px-3 py-2 text-sm outline-none placeholder:text-muted focus:border-primary'

export function LoginPage() {
  const { session, signIn } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const login = useLogin()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  const from = (location.state as { from?: string } | null)?.from ?? '/'
  if (session) return <Navigate to={from} replace />

  function submit(e: FormEvent) {
    e.preventDefault()
    login.mutate(
      { username, password },
      {
        onSuccess: (s) => {
          signIn(s)
          navigate(from, { replace: true })
        },
      },
    )
  }

  const error = login.error
    ? isApiError(login.error, 401)
      ? 'Usuario o contraseña incorrectos'
      : login.error.message
    : null

  return (
    <main className="flex min-h-full items-center justify-center p-6">
      <form onSubmit={submit} className="flex w-full max-w-sm flex-col gap-5 rounded-2xl border border-border bg-surface p-8">
        <img src={logo} alt="Bia" className="h-8 self-start" />
        <div>
          <h1 className="text-xl font-semibold">Ingresa a tu cuenta</h1>
          <p className="mt-1 text-sm text-muted">Gestión de energía con IA</p>
        </div>
        <label className="flex flex-col gap-1.5 text-sm">
          Usuario
          <input className={inputClass} value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
        </label>
        <label className="flex flex-col gap-1.5 text-sm">
          Contraseña
          <input className={inputClass} type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" required />
        </label>
        {error && (
          <p role="alert" className="text-sm text-danger">
            {error}
          </p>
        )}
        <Button type="submit" disabled={login.isPending}>
          {login.isPending ? 'Ingresando…' : 'Ingresar'}
        </Button>
      </form>
    </main>
  )
}
