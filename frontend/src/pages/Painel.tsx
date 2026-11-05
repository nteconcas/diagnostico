import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, baixarArquivo, EscolaResumo, formatarData } from '../api'
import { useAuth } from '../auth'

export function Painel() {
  const { usuario } = useAuth()
  const admin = usuario?.papel === 'admin'
  const [escolas, setEscolas] = useState<EscolaResumo[]>([])
  const [carregando, setCarregando] = useState(true)
  const [baixando, setBaixando] = useState('')
  const navegar = useNavigate()

  useEffect(() => {
    api<EscolaResumo[]>('/api/escolas')
      .then(setEscolas)
      .catch(() => {})
      .finally(() => setCarregando(false))
  }, [])

  const stats = useMemo(() => {
    const enviados = escolas.filter((e) => e.status === 'enviado').length
    const rascunhos = escolas.filter((e) => e.status === 'rascunho').length
    return { total: escolas.length, enviados, rascunhos }
  }, [escolas])

  async function baixar(e: EscolaResumo, formato: 'pdf' | 'docx' | 'html') {
    if (!e.ultimo_diagnostico_id) return
    const chave = `${e.id}-${formato}`
    setBaixando(chave)
    try {
      await baixarArquivo(
        `/api/diagnosticos/${e.ultimo_diagnostico_id}/relatorio.${formato}`,
        `diagnostico-${e.id}.${formato}`,
      )
    } catch {
      /* falha silenciosa: o navegador não inicia o download */
    } finally {
      setBaixando('')
    }
  }

  return (
    <>
      <div className="secao-titulo">
        <div>
          <h2>Painel de diagnósticos</h2>
          <p className="desc">
            {admin
              ? 'Acompanhe os diagnósticos das unidades escolares e consulte os enviados.'
              : 'Acompanhe o preenchimento por unidade escolar e exporte os relatórios.'}
          </p>
        </div>
        {!admin && (
          <button className="btn-primario" onClick={() => navegar('/formulario/novo')}>
            ＋ Novo diagnóstico
          </button>
        )}
      </div>

      <div className="estatisticas">
        <div className="cartao estatistica">
          <div className="icone" style={{ background: 'var(--azul-claro)' }}>🏫</div>
          <div>
            <div className="valor">{stats.total}</div>
            <div className="rot">Escolas na rede</div>
          </div>
        </div>
        <div className="cartao estatistica">
          <div className="icone" style={{ background: 'var(--aviso-fundo)' }}>✏️</div>
          <div>
            <div className="valor">{stats.rascunhos}</div>
            <div className="rot">Rascunhos em andamento</div>
          </div>
        </div>
        <div className="cartao estatistica">
          <div className="icone" style={{ background: 'var(--verde-claro)' }}>✅</div>
          <div>
            <div className="valor">{stats.enviados}</div>
            <div className="rot">Diagnósticos enviados</div>
          </div>
        </div>
      </div>

      {carregando ? (
        <div className="carregando-pagina">
          <div className="girando" /> Carregando escolas…
        </div>
      ) : escolas.length === 0 ? (
        <div className="cartao" style={{ padding: 26 }}>
          <div className="vazio">
            Nenhuma escola cadastrada ainda. Administradores podem incluir unidades em{' '}
            <Link to="/escolas">Gerenciar escolas</Link>.
          </div>
        </div>
      ) : (
        <div className="cartao tabela-wrap">
          <table className="dados">
            <thead>
              <tr>
                <th>Unidade escolar</th>
                <th>Último diagnóstico</th>
                <th>Atualizado</th>
                <th>Ações</th>
              </tr>
            </thead>
            <tbody>
              {escolas.map((e) => (
                <tr key={e.id}>
                  <td>
                    <strong>{e.nome}</strong>
                  </td>
                  <td>
                    {e.status === 'enviado' && <span className="status-chip enviado">✓ Encerrado</span>}
                    {e.status === 'rascunho' && <span className="status-chip rascunho">Rascunho</span>}
                    {!e.status && <span className="status-chip nenhum">Não iniciado</span>}
                  </td>
                  <td>
                    {e.atualizado_em ? (
                      <>
                        {formatarData(e.atualizado_em)}
                        {e.atualizado_por && <div style={{ fontSize: 12, color: 'var(--cinza)' }}>por {e.atualizado_por}</div>}
                      </>
                    ) : (
                      '—'
                    )}
                  </td>
                  <td>
                    <div className="acoes-linha">
                      {!admin && e.status === 'rascunho' && e.ultimo_diagnostico_id && (
                        <Link className="btn-suave btn-mini" to={`/formulario/${e.ultimo_diagnostico_id}`}>
                          Continuar
                        </Link>
                      )}
                      {!admin && !e.status && (
                        <Link className="btn-primario btn-mini" to={`/formulario/novo?escola=${e.id}`}>
                          Preencher
                        </Link>
                      )}
                      {e.status === 'enviado' && e.ultimo_diagnostico_id && (
                        <Link className="btn-suave btn-mini" to={`/envios/${e.ultimo_diagnostico_id}`}>
                          Ver envio
                        </Link>
                      )}
                      {e.status === 'enviado' && e.ultimo_diagnostico_id && (
                        <>
                          <button
                            className="btn-suave btn-mini"
                            disabled={baixando === `${e.id}-pdf`}
                            onClick={() => baixar(e, 'pdf')}
                          >
                            {baixando === `${e.id}-pdf` ? '…' : 'PDF'}
                          </button>
                          <button
                            className="btn-suave btn-mini"
                            disabled={baixando === `${e.id}-docx`}
                            onClick={() => baixar(e, 'docx')}
                          >
                            {baixando === `${e.id}-docx` ? '…' : 'DOCX'}
                          </button>
                          <button
                            className="btn-suave btn-mini"
                            disabled={baixando === `${e.id}-html`}
                            onClick={() => baixar(e, 'html')}
                          >
                            {baixando === `${e.id}-html` ? '…' : 'HTML'}
                          </button>
                        </>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}
