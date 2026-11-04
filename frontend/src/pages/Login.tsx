import { FormEvent, useState } from 'react'
import { useAuth } from '../auth'

export function Login() {
  const { entrar } = useAuth()
  const [username, setUsername] = useState('')
  const [senha, setSenha] = useState('')
  const [erro, setErro] = useState('')
  const [enviando, setEnviando] = useState(false)

  async function submeter(e: FormEvent) {
    e.preventDefault()
    setErro('')
    setEnviando(true)
    try {
      await entrar(username.trim(), senha)
      window.location.href = '/'
    } catch (err) {
      setErro((err as Error).message)
      setEnviando(false)
    }
  }

  return (
    <div className="pagina-login">
      <div className="login-cartao">
        <div className="logo">🎓</div>
        <h1>Formulário de Diagnóstico Escolar</h1>
        <p className="sub">
          Servidores · Infraestrutura · Projetos · Ofertas de cursos — uso institucional da rede educacional
        </p>
        <form onSubmit={submeter}>
          {erro && <div className="erro-caixa">{erro}</div>}
          <div>
            <label className="rotulo">Usuário</label>
            <input
              type="text"
              value={username}
              autoFocus
              onChange={(e) => setUsername(e.target.value)}
              placeholder="seu.usuario"
            />
          </div>
          <div>
            <label className="rotulo">Senha</label>
            <input
              type="password"
              value={senha}
              onChange={(e) => setSenha(e.target.value)}
              placeholder="••••••••"
            />
          </div>
          <button className="btn-primario" type="submit" disabled={enviando}>
            {enviando ? 'Entrando…' : 'Entrar'}
          </button>
        </form>
        <p className="login-rodape">Acesso restrito a servidores autorizados da rede.</p>
      </div>
    </div>
  )
}
