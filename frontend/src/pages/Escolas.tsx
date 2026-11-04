import { FormEvent, useEffect, useState } from 'react'
import { api, baixarArquivo, EscolaResumo, formatarData } from '../api'

export function Escolas() {
  const [escolas, setEscolas] = useState<EscolaResumo[]>([])
  const [baixando, setBaixando] = useState('')
  const [nome, setNome] = useState('')
  const [nte, setNte] = useState('')
  const [respNome, setRespNome] = useState('')
  const [respUsuario, setRespUsuario] = useState('')
  const [respSenha, setRespSenha] = useState('')
  const [editando, setEditando] = useState<{ id: string; nome: string; nte: string } | null>(null)
  const [erro, setErro] = useState('')
  const [ok, setOk] = useState('')

  function carregar() {
    api<EscolaResumo[]>('/api/escolas').then(setEscolas).catch(() => {})
  }
  useEffect(carregar, [])

  async function criar(e: FormEvent) {
    e.preventDefault()
    setErro('')
    setOk('')
    try {
      const temResponsavel = respNome.trim() || respUsuario.trim() || respSenha
      const corpo: Record<string, unknown> = { nome, nte }
      if (temResponsavel) {
        corpo.responsavel = { nome: respNome, username: respUsuario, senha: respSenha }
      }
      const r = await api<{ responsavel_username?: string }>('/api/escolas', {
        method: 'POST',
        body: JSON.stringify(corpo),
      })
      setNome('')
      setNte('')
      setRespNome('')
      setRespUsuario('')
      setRespSenha('')
      setOk(
        r.responsavel_username
          ? `Escola cadastrada! O responsável entra com o usuário "${r.responsavel_username}" e a senha definida.`
          : 'Escola cadastrada com sucesso.',
      )
      carregar()
    } catch (err) {
      setErro((err as Error).message)
    }
  }

  async function salvarEdicao() {
    if (!editando) return
    setErro('')
    try {
      await api(`/api/escolas/${editando.id}`, {
        method: 'PUT',
        body: JSON.stringify({ nome: editando.nome, nte: editando.nte }),
      })
      setEditando(null)
      setOk('Escola atualizada.')
      carregar()
    } catch (err) {
      setErro((err as Error).message)
    }
  }

  async function baixar(e: EscolaResumo, formato: 'pdf' | 'docx' | 'html') {
    if (!e.ultimo_diagnostico_id) return
    const chave = `${e.id}-${formato}`
    setErro('')
    setOk('')
    setBaixando(chave)
    try {
      await baixarArquivo(
        `/api/diagnosticos/${e.ultimo_diagnostico_id}/relatorio.${formato}`,
        `diagnostico-${e.id}.${formato}`,
      )
    } catch (err) {
      setErro(`Falha ao gerar o relatório ${formato.toUpperCase()}: ${(err as Error).message}`)
    } finally {
      setBaixando('')
    }
  }

  async function excluir(e: EscolaResumo) {
    if (!confirm(`Excluir a escola "${e.nome}"? Esta ação não pode ser desfeita.`)) return
    setErro('')
    setOk('')
    try {
      await api(`/api/escolas/${e.id}`, { method: 'DELETE' })
      setOk('Escola excluída.')
      carregar()
    } catch (err) {
      setErro((err as Error).message)
    }
  }

  return (
    <>
      <div className="secao-titulo">
        <div>
          <h2>Gerenciar escolas</h2>
          <p className="desc">Cadastro das unidades escolares que serão diagnosticadas.</p>
        </div>
      </div>

      {(erro || ok) && (
        <div style={{ marginBottom: 16 }}>
          {erro && <div className="erro-caixa">{erro}</div>}
          {ok && !erro && <div className="sucesso">{ok}</div>}
        </div>
      )}

      <div className="cartao" style={{ padding: 20, marginBottom: 20 }}>
        <h4 style={{ marginBottom: 4 }}>Nova escola</h4>
        <p className="desc" style={{ marginBottom: 12 }}>
          Preencha o usuário responsável para criar automaticamente o acesso dele — quem vai
          preencher o diagnóstico da escola com a senha definida aqui.
        </p>
        <form onSubmit={criar}>
          <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr auto', gap: 12 }}>
            <div>
              <label className="rotulo">Nome da unidade escolar *</label>
              <input value={nome} onChange={(e) => setNome(e.target.value)} placeholder="Ex.: C. E. Tancredo Neves" required />
            </div>
            <div>
              <label className="rotulo">NTE</label>
              <input value={nte} onChange={(e) => setNte(e.target.value)} placeholder="Ex.: NTE 3" />
            </div>
            <div style={{ display: 'flex', alignItems: 'flex-end' }} />
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr 1fr', gap: 12, marginTop: 12 }}>
            <div>
              <label className="rotulo">Responsável pelo diagnóstico <span className="opcional">(opcional)</span></label>
              <input value={respNome} onChange={(e) => setRespNome(e.target.value)} placeholder="Nome completo" />
            </div>
            <div>
              <label className="rotulo">Usuário de acesso</label>
              <input value={respUsuario} onChange={(e) => setRespUsuario(e.target.value)} placeholder="Ex.: maria.tancredo" />
            </div>
            <div>
              <label className="rotulo">Senha <span className="opcional">(mín. 4)</span></label>
              <input type="text" value={respSenha} onChange={(e) => setRespSenha(e.target.value)} placeholder="Defina e repasse ao responsável" />
            </div>
          </div>
          <div style={{ marginTop: 14 }}>
            <button className="btn-primario" type="submit">
              Cadastrar escola
            </button>
          </div>
        </form>
      </div>

      <div className="cartao tabela-wrap">
        <table className="dados">
          <thead>
            <tr>
              <th>Unidade escolar</th>
              <th>NTE</th>
              <th>Responsável(is)</th>
              <th>Diagnóstico</th>
              <th style={{ width: 210 }}>Relatórios</th>
              <th style={{ width: 150 }}>Ações</th>
            </tr>
          </thead>
          <tbody>
            {escolas.length === 0 && (
              <tr>
                <td colSpan={6}>
                  <div className="vazio">Nenhuma escola cadastrada.</div>
                </td>
              </tr>
            )}
            {escolas.map((e) => (
              <tr key={e.id}>
                <td>
                  {editando?.id === e.id ? (
                    <input value={editando.nome} onChange={(ev) => setEditando({ ...editando, nome: ev.target.value })} />
                  ) : (
                    <strong>{e.nome}</strong>
                  )}
                </td>
                <td>
                  {editando?.id === e.id ? (
                    <input value={editando.nte} onChange={(ev) => setEditando({ ...editando, nte: ev.target.value })} />
                  ) : (
                    e.nte || '—'
                  )}
                </td>
                <td>
                  {e.responsaveis && e.responsaveis.length > 0 ? (
                    <div>
                      {e.responsaveis.map((r) => (
                        <div key={r.id} style={{ fontSize: 13 }}>
                          {r.nome} <span style={{ color: 'var(--cinza)' }}>({r.username})</span>
                        </div>
                      ))}
                    </div>
                  ) : (
                    '—'
                  )}
                </td>
                <td>
                  {e.status === 'enviado' ? (
                    <div>
                      <span className="status-chip enviado">✓ Encerrado</span>
                      <div style={{ fontSize: 12, color: 'var(--cinza)', marginTop: 4 }}>
                        Entregue em {formatarData(e.atualizado_em)}
                      </div>
                    </div>
                  ) : e.status === 'rascunho' ? (
                    <div>
                      <span className="status-chip rascunho">Rascunho</span>
                      <div style={{ fontSize: 12, color: 'var(--cinza)', marginTop: 4 }}>
                        Atualizado em {formatarData(e.atualizado_em)}
                      </div>
                    </div>
                  ) : (
                    <span className="status-chip nenhum">Não iniciado</span>
                  )}
                </td>
                <td>
                  {e.ultimo_diagnostico_id ? (
                    <div className="acoes-linha">
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
                    </div>
                  ) : (
                    <span style={{ color: 'var(--cinza)', fontSize: 13 }}>—</span>
                  )}
                </td>
                <td>
                  <div className="acoes-linha">
                    {editando?.id === e.id ? (
                      <>
                        <button className="btn-verde btn-mini" onClick={salvarEdicao}>
                          Salvar
                        </button>
                        <button className="btn-suave btn-mini" onClick={() => setEditando(null)}>
                          Cancelar
                        </button>
                      </>
                    ) : (
                      <>
                        <button
                          className="btn-suave btn-mini"
                          onClick={() => setEditando({ id: e.id, nome: e.nome, nte: e.nte })}
                        >
                          Editar
                        </button>
                        <button className="btn-perigo btn-mini" onClick={() => excluir(e)}>
                          Excluir
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
    </>
  )
}
