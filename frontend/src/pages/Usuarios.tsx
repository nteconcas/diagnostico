import { FormEvent, useEffect, useState } from 'react'
import { api, EscolaResumo, Usuario } from '../api'
import { useAuth } from '../auth'

interface CorpoUsuario {
  id?: string
  nome: string
  username: string
  senha: string
  papel: 'admin' | 'preenchador'
  escola_id: string
}

const vazio: CorpoUsuario = { nome: '', username: '', senha: '', papel: 'preenchador', escola_id: '' }

export function Usuarios() {
  const [usuarios, setUsuarios] = useState<Usuario[]>([])
  const [escolas, setEscolas] = useState<EscolaResumo[]>([])
  const [corpo, setCorpo] = useState<CorpoUsuario>(vazio)
  const [editando, setEditando] = useState(false)
  const [erro, setErro] = useState('')
  const [ok, setOk] = useState('')
  const { usuario } = useAuth()

  function carregar() {
    api<Usuario[]>('/api/usuarios').then(setUsuarios).catch((e) => setErro((e as Error).message))
  }
  useEffect(carregar, [])
  useEffect(() => {
    api<EscolaResumo[]>('/api/escolas').then(setEscolas).catch(() => {})
  }, [])

  async function salvar(e: FormEvent) {
    e.preventDefault()
    setErro('')
    setOk('')
    try {
      if (editando && corpo.id) {
        await api(`/api/usuarios/${corpo.id}`, { method: 'PUT', body: JSON.stringify(corpo) })
        setOk('Usuário atualizado.')
      } else {
        await api('/api/usuarios', { method: 'POST', body: JSON.stringify(corpo) })
        setOk('Usuário criado.')
      }
      setCorpo(vazio)
      setEditando(false)
      carregar()
    } catch (err) {
      setErro((err as Error).message)
    }
  }

  async function excluir(u: Usuario) {
    if (!confirm(`Excluir o usuário "${u.nome}"?`)) return
    setErro('')
    try {
      await api(`/api/usuarios/${u.id}`, { method: 'DELETE' })
      setOk('Usuário excluído.')
      carregar()
    } catch (err) {
      setErro((err as Error).message)
    }
  }

  function editar(u: Usuario) {
    setEditando(true)
    setCorpo({ id: u.id, nome: u.nome, username: u.username, senha: '', papel: u.papel, escola_id: u.escola_id ?? '' })
    setOk('')
    setErro('')
  }

  return (
    <>
      <div className="secao-titulo">
        <div>
          <h2>Usuários</h2>
          <p className="desc">Contas com acesso ao formulário e o papel de cada uma.</p>
        </div>
      </div>

      {(erro || ok) && (
        <div style={{ marginBottom: 16 }}>
          {erro && <div className="erro-caixa">{erro}</div>}
          {ok && !erro && <div className="sucesso">{ok}</div>}
        </div>
      )}

      <div className="cartao" style={{ padding: 20, marginBottom: 20 }}>
        <h4 style={{ marginBottom: 10 }}>{editando ? 'Editar usuário' : 'Novo usuário'}</h4>
        <form onSubmit={salvar} style={{ display: 'grid', gridTemplateColumns: '1.4fr 1fr 1fr auto auto', gap: 12 }}>
          <div>
            <label className="rotulo">Nome *</label>
            <input value={corpo.nome} onChange={(e) => setCorpo({ ...corpo, nome: e.target.value })} required />
          </div>
          <div>
            <label className="rotulo">Usuário *</label>
            <input value={corpo.username} onChange={(e) => setCorpo({ ...corpo, username: e.target.value })} required />
          </div>
          <div>
            <label className="rotulo">Senha {editando && <span className="opcional">(deixe vazio p/ manter)</span>}</label>
            <input
              type="password"
              value={corpo.senha}
              onChange={(e) => setCorpo({ ...corpo, senha: e.target.value })}
              required={!editando}
            />
          </div>
          <div>
            <label className="rotulo">Papel</label>
            <select value={corpo.papel} onChange={(e) => setCorpo({ ...corpo, papel: e.target.value as any, escola_id: e.target.value === 'admin' ? '' : corpo.escola_id })}>
              <option value="preenchador">Preenchedor</option>
              <option value="admin">Administrador</option>
            </select>
          </div>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 8 }}>
            <button className="btn-primario" type="submit">
              {editando ? 'Salvar' : 'Criar'}
            </button>
            {editando && (
              <button
                type="button"
                className="btn-suave"
                onClick={() => {
                  setEditando(false)
                  setCorpo(vazio)
                }}
              >
                Cancelar
              </button>
            )}
          </div>
        </form>
        {corpo.papel === 'preenchador' && (
          <div style={{ marginTop: 12 }}>
            <label className="rotulo">Escola do responsável</label>
            <select value={corpo.escola_id} onChange={(e) => setCorpo({ ...corpo, escola_id: e.target.value })}>
              <option value="">— Sem vínculo (acessa todas as escolas) —</option>
              {escolas.map((esc) => (
                <option key={esc.id} value={esc.id}>
                  {esc.nome}
                </option>
              ))}
            </select>
            <p className="desc" style={{ marginTop: 6 }}>
              Com vínculo, o usuário só vê e preenche o diagnóstico da própria escola.
            </p>
          </div>
        )}
      </div>

      <div className="cartao tabela-wrap">
        <table className="dados">
          <thead>
            <tr>
              <th>Nome</th>
              <th>Usuário</th>
              <th>Papel</th>
              <th>Escola</th>
              <th style={{ width: 170 }}>Ações</th>
            </tr>
          </thead>
          <tbody>
            {usuarios.map((u) => (
              <tr key={u.id}>
                <td>
                  <strong>{u.nome}</strong>
                  {usuario?.id === u.id && <span style={{ color: 'var(--cinza)', fontSize: 12 }}> (você)</span>}
                </td>
                <td>{u.username}</td>
                <td>
                  <span className="papel-tag">{u.papel === 'admin' ? 'Administrador' : 'Preenchedor'}</span>
                </td>
                <td>{u.escola_nome || '—'}</td>
                <td>
                  <div className="acoes-linha">
                    <button className="btn-suave btn-mini" onClick={() => editar(u)}>
                      Editar
                    </button>
                    {usuario?.id !== u.id && (
                      <button className="btn-perigo btn-mini" onClick={() => excluir(u)}>
                        Excluir
                      </button>
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
