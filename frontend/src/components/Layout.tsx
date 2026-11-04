import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth'

export function Layout() {
  const { usuario, sair } = useAuth()
  const navegar = useNavigate()
  const admin = usuario?.papel === 'admin'
  const iniciais = (usuario?.nome ?? '?')
    .split(' ')
    .map((p) => p[0])
    .slice(0, 2)
    .join('')
    .toUpperCase()

  return (
    <>
      <div className="faixa-topo" />
      <header className="app">
        <div className="header-conteudo">
          <div>
            <h1>Formulário de Diagnóstico Escolar</h1>
            <p className="sub">Servidores · Infraestrutura · Projetos · Ofertas de cursos</p>
          </div>
          <div className="header-acoes">
            {admin && (
              <>
                <button className="btn-suave btn-mini" onClick={() => navegar('/escolas')}>
                  Gerenciar escolas
                </button>
                <button className="btn-suave btn-mini" onClick={() => navegar('/usuarios')}>
                  Usuários
                </button>
              </>
            )}
            <div className="usuario-chip">
              <div className="avatar">{iniciais}</div>
              <span title={usuario?.escola_nome || undefined}>{usuario?.nome}</span>
              {usuario?.papel === 'admin' ? (
                <span className="papel-tag">Admin</span>
              ) : usuario?.escola_nome ? (
                <span className="papel-tag" title={usuario.escola_nome}>
                  Responsável
                </span>
              ) : (
                <span className="papel-tag">Rede</span>
              )}
            </div>
            <button className="btn-suave btn-mini" onClick={() => sair()} title="Sair">
              Sair
            </button>
          </div>
        </div>
      </header>
      <main className="pagina">
        <Outlet />
      </main>
    </>
  )
}
