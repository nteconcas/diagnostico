import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, Diagnostico, formatarData, fotoMedia, fotoThumb } from '../api'
import { useAuth } from '../auth'

// Envio mostra o diagnóstico enviado em modo somente leitura com as exportações.
export function Envio() {
  const { usuario } = useAuth()
  const admin = usuario?.papel === 'admin'
  const { id } = useParams<{ id: string }>()
  const [diag, setDiag] = useState<Diagnostico | null>(null)
  const [erro, setErro] = useState('')

  useEffect(() => {
    api<Diagnostico>(`/api/diagnosticos/${id}`)
      .then(setDiag)
      .catch((e) => setErro((e as Error).message))
  }, [id])

  if (erro) {
    return (
      <div className="erro-caixa">
        {erro} — <Link to="/">voltar ao painel</Link>
      </div>
    )
  }
  if (!diag) {
    return (
      <div className="carregando-pagina">
        <div className="girando" /> Carregando envio…
      </div>
    )
  }
  const d = diag.dados
  const totalServ = d.servidores.gestao + d.servidores.efetivos + d.servidores.act + d.servidores.pedagogico
  const totalEquip =
    d.equipamentos.computadores + d.equipamentos.tablets + d.equipamentos.projetores + d.equipamentos.telas

  return (
    <>
      <div className="secao-titulo">
        <div>
          <h2>{diag.escola_nome}</h2>
          <p className="desc">
            <span className={'status-chip ' + (diag.status === 'enviado' ? 'enviado' : 'rascunho')}>
              {diag.status === 'enviado' ? '✓ Enviado' : 'Rascunho'}
            </span>{' '}
            {diag.enviado_em && <>· enviado em {formatarData(diag.enviado_em)}</>}
          </p>
        </div>
        <div className="exportar-grupo">
          <a className="btn-suave btn-mini" href={`/api/diagnosticos/${diag.id}/relatorio.pdf`}>
            ⬇ PDF
          </a>
          <a className="btn-suave btn-mini" href={`/api/diagnosticos/${diag.id}/relatorio.docx`}>
            ⬇ DOCX
          </a>
          <a className="btn-suave btn-mini" href={`/api/diagnosticos/${diag.id}/relatorio.html`} target="_blank" rel="noreferrer">
            ⬇ HTML
          </a>
          {!admin && diag.status === 'rascunho' && (
            <Link className="btn-primario btn-mini" to={`/formulario/${diag.id}`}>
              Continuar preenchimento
            </Link>
          )}
        </div>
      </div>

      <section className="bloco">
        <div className="cartao">
          <div className="bloco-cabeca">
            <div className="bloco-numero">🏫</div>
            <div>
              <h3>Identificação e localização</h3>
            </div>
          </div>
          <div className="bloco-corpo">
            <div className="resumo-grid">
              <div className="resumo-item">
                <b className="rot">Responsável pelo preenchimento</b>
                <div className="val">{d.responsavel || '—'}</div>
              </div>
              <div className="resumo-item">
                <b className="rot">Endereço</b>
                <div className="val">{d.endereco || '—'}</div>
              </div>
              <div className="resumo-item">
                <b className="rot">Localização GPS</b>
                <div className="val">
                  {d.gps.capturado ? (
                    <a href={`https://www.google.com/maps?q=${d.gps.lat},${d.gps.lng}`} target="_blank" rel="noreferrer">
                      {d.gps.lat}, {d.gps.lng} — abrir no mapa
                    </a>
                  ) : (
                    '—'
                  )}
                </div>
              </div>
            </div>
            {(d.contatos.secretaria.telefone || d.contatos.direcao.telefone || d.contatos.pedagogico.telefone) && (
              <div className="tabela-wrap">
                <table className="dados">
                  <thead>
                    <tr>
                      <th>Setor</th>
                      <th>Telefone</th>
                      <th>E-mail</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      <td>Secretaria</td>
                      <td>{d.contatos.secretaria.telefone || '—'}</td>
                      <td>{d.contatos.secretaria.email || '—'}</td>
                    </tr>
                    <tr>
                      <td>Direção</td>
                      <td>{d.contatos.direcao.telefone || '—'}</td>
                      <td>{d.contatos.direcao.email || '—'}</td>
                    </tr>
                    <tr>
                      <td>Pedagógico</td>
                      <td>{d.contatos.pedagogico.telefone || '—'}</td>
                      <td>{d.contatos.pedagogico.email || '—'}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            )}
            <div className="galeria-mini">
              {d.foto_fachada && (
                <figure style={{ margin: 0 }}>
                  <img src={fotoMedia(d.foto_fachada)} alt="Fachada" />
                  <figcaption style={{ fontSize: 11, color: 'var(--cinza)', textAlign: 'center' }}>Fachada (capa do relatório)</figcaption>
                </figure>
              )}
              {d.foto_gestor && (
                <figure style={{ margin: 0 }}>
                  <img src={fotoMedia(d.foto_gestor)} alt="Gestor" />
                  <figcaption style={{ fontSize: 11, color: 'var(--cinza)', textAlign: 'center' }}>Gestor(a)</figcaption>
                </figure>
              )}
            </div>
          </div>
        </div>
      </section>

      <section className="bloco">
        <div className="cartao">
          <div className="bloco-cabeca">
            <div className="bloco-numero">1</div>
            <div>
              <h3>Servidores</h3>
            </div>
          </div>
          <div className="bloco-corpo">
            <div className="resumo-grid">
              <div className="resumo-item">
                <b className="rot">Gestão</b>
                <div className="val">{d.servidores.gestao}</div>
              </div>
              <div className="resumo-item">
                <b className="rot">Pedagógico</b>
                <div className="val">{d.servidores.pedagogico}</div>
              </div>
              <div className="resumo-item">
                <b className="rot">Professores efetivos</b>
                <div className="val">{d.servidores.efetivos}</div>
              </div>
              <div className="resumo-item">
                <b className="rot">Professores ACT</b>
                <div className="val">{d.servidores.act}</div>
              </div>
              <div className="resumo-item">
                <b className="rot">Total de servidores</b>
                <div className="val">
                  <strong>{totalServ}</strong>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="bloco">
        <div className="cartao">
          <div className="bloco-cabeca">
            <div className="bloco-numero">2</div>
            <div>
              <h3>Infraestrutura</h3>
            </div>
          </div>
          <div className="bloco-corpo">
            <div className="resumo-grid">
              <div className="resumo-item">
                <b className="rot">Salas de aula</b>
                <div className="val">
                  {d.salas.total} no total · {d.salas.ocupadas} ocupadas
                </div>
              </div>
              <div className="resumo-item">
                <b className="rot">Equipamentos (total {totalEquip})</b>
                <div className="val">
                  {d.equipamentos.computadores} computadores · {d.equipamentos.tablets} tablets · {d.equipamentos.projetores} projetores ·{' '}
                  {d.equipamentos.telas} telas
                </div>
              </div>
            </div>
            {(d.labs.maker.possui || d.labs.informatica.possui || d.labs.outros.length > 0) && (
              <div className="resumo-grid">
                {d.labs.maker.possui && (
                  <div className="resumo-item">
                    <b className="rot">Laboratório maker</b>
                    <div className="val">{d.labs.maker.descricao || 'Possui'}</div>
                  </div>
                )}
                {d.labs.informatica.possui && (
                  <div className="resumo-item">
                    <b className="rot">Laboratório de informática</b>
                    <div className="val">{d.labs.informatica.descricao || 'Possui'}</div>
                  </div>
                )}
                {d.labs.outros.map((l, i) => (
                  <div className="resumo-item" key={i}>
                    <b className="rot">{l.nome || 'Outro laboratório'}</b>
                    <div className="val">{l.descricao}</div>
                  </div>
                ))}
              </div>
            )}
            <div>
              <b className="rot" style={{ fontSize: 11.5, textTransform: 'uppercase', color: 'var(--cinza)', letterSpacing: 0.4 }}>
                Fotos — antes da gestão 2023/2027
              </b>
              <div className="galeria-mini" style={{ marginTop: 6 }}>
                {d.fotos_antes.map((f) => (
                  <img key={f.id} src={fotoThumb(f)} alt={f.nome} />
                ))}
              </div>
            </div>
            <div>
              <b className="rot" style={{ fontSize: 11.5, textTransform: 'uppercase', color: 'var(--cinza)', letterSpacing: 0.4 }}>
                Fotos — gestão atual 2023/2027
              </b>
              <div className="galeria-mini" style={{ marginTop: 6 }}>
                {d.fotos_atual.map((f) => (
                  <img key={f.id} src={fotoThumb(f)} alt={f.nome} />
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      {d.espacos.length > 0 && (
        <section className="bloco">
          <div className="cartao">
            <div className="bloco-cabeca">
              <div className="bloco-numero">3</div>
              <div>
                <h3>Espaços da escola</h3>
              </div>
            </div>
            <div className="bloco-corpo">
              {d.espacos.map((e, i) => (
                <div className="subcartao" key={i}>
                  <h4>{e.nome}</h4>
                  {e.descricao && <p className="espaco-descricao">{e.descricao}</p>}
                  {e.fotos.length > 0 && (
                    <div className="galeria-mini">
                      {e.fotos.map((f) => (
                        <img key={f.id} src={fotoThumb(f)} alt={f.nome} />
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        </section>
      )}

      {(d.obras ?? []).length > 0 && (
        <section className="bloco">
          <div className="cartao">
            <div className="bloco-cabeca">
              <div className="bloco-numero">4</div>
              <div>
                <h3>Obras realizadas</h3>
              </div>
            </div>
            <div className="bloco-corpo">
              {(d.obras ?? []).map((o, i) => (
                <div className="subcartao" key={i}>
                  <h4>{o.nome || `Obra ${i + 1}`}</h4>
                  {o.descricao && <p style={{ whiteSpace: 'pre-wrap', fontSize: 13.5 }}>{o.descricao}</p>}
                  <div className="grade2" style={{ marginTop: 8 }}>
                    <div>
                      <b
                        className="rot"
                        style={{ fontSize: 11.5, textTransform: 'uppercase', color: 'var(--cinza)', letterSpacing: 0.4 }}
                      >
                        Antes da obra
                      </b>
                      <div className="galeria-mini" style={{ marginTop: 6 }}>
                        {(o.fotos_antes ?? []).length > 0 ? (
                          (o.fotos_antes ?? []).map((f) => <img key={f.id} src={fotoThumb(f)} alt={f.nome} />)
                        ) : (
                          <span className="dica">Nenhuma foto de antes enviada.</span>
                        )}
                      </div>
                    </div>
                    <div>
                      <b
                        className="rot"
                        style={{ fontSize: 11.5, textTransform: 'uppercase', color: 'var(--cinza)', letterSpacing: 0.4 }}
                      >
                        Depois da obra
                      </b>
                      <div className="galeria-mini" style={{ marginTop: 6 }}>
                        {(o.fotos_depois ?? []).length > 0 ? (
                          (o.fotos_depois ?? []).map((f) => <img key={f.id} src={fotoThumb(f)} alt={f.nome} />)
                        ) : (
                          <span className="dica">Nenhuma foto de depois enviada.</span>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>
      )}

      {(d.revitalizacao ?? []).length > 0 && (
        <section className="bloco">
          <div className="cartao">
            <div className="bloco-cabeca">
              <div className="bloco-numero">5</div>
              <div>
                <h3>Espaços que precisam de revitalização</h3>
              </div>
            </div>
            <div className="bloco-corpo">
              {(d.revitalizacao ?? []).map((r, i) => (
                <div className="subcartao" key={i}>
                  <h4>{r.nome || `Espaço ${i + 1}`}</h4>
                  {r.descricao && <p className="espaco-descricao">{r.descricao}</p>}
                  {(r.fotos ?? []).length > 0 && (
                    <div className="galeria-mini" style={{ marginTop: 6 }}>
                      {(r.fotos ?? []).map((f) => (
                        <img key={f.id} src={fotoThumb(f)} alt={f.nome} />
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        </section>
      )}

      {d.projetos.length > 0 && (
        <section className="bloco">
          <div className="cartao">
            <div className="bloco-cabeca">
              <div className="bloco-numero">6</div>
              <div>
                <h3>Projetos</h3>
              </div>
            </div>
            <div className="bloco-corpo">
              {d.projetos.map((p, i) => (
                <div className="subcartao" key={i}>
                  <h4>{p.nome || `Projeto ${i + 1}`}</h4>
                  <p style={{ whiteSpace: 'pre-wrap', fontSize: 13.5 }}>{p.descricao}</p>
                  {p.fotos.length > 0 && (
                    <div className="galeria-mini" style={{ marginTop: 8 }}>
                      {p.fotos.map((f) => (
                        <img key={f.id} src={fotoThumb(f)} alt={f.nome} />
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        </section>
      )}

      {d.cursos.length > 0 && (
        <section className="bloco">
          <div className="cartao">
            <div className="bloco-cabeca">
              <div className="bloco-numero">7</div>
              <div>
                <h3>Ofertas de cursos</h3>
              </div>
            </div>
            <div className="bloco-corpo">
              <div className="chips">
                {d.cursos.map((c) => (
                  <span className="chip" key={c}>
                    {c}
                  </span>
                ))}
              </div>
            </div>
          </div>
        </section>
      )}
    </>
  )
}
