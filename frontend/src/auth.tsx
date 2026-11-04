import { createContext, useContext, useEffect, useState } from 'react'
import { api, definirToken, Usuario } from './api'

interface AuthCtx {
  usuario: Usuario | null
  carregando: boolean
  entrar: (username: string, senha: string) => Promise<void>
  sair: () => Promise<void>
}

const Ctx = createContext<AuthCtx>({} as AuthCtx)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [usuario, setUsuario] = useState<Usuario | null>(null)
  const [carregando, setCarregando] = useState(true)

  useEffect(() => {
    api<Usuario>('/api/me')
      .then(setUsuario)
      .catch(() => {})
      .finally(() => setCarregando(false))
  }, [])

  async function entrar(username: string, senha: string) {
    const u = await api<Usuario>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ username, senha }),
    })
    if (u.token) definirToken(u.token)
    setUsuario(u)
  }

  async function sair() {
    try {
      await api('/api/logout', { method: 'POST' })
    } catch {}
    definirToken(null)
    setUsuario(null)
    window.location.href = '/login'
  }

  return <Ctx.Provider value={{ usuario, carregando, entrar, sair }}>{children}</Ctx.Provider>
}

export const useAuth = () => useContext(Ctx)
