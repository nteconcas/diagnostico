import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  api,
  Dados,
  Diagnostico,
  EscolaResumo,
  ErroAPI,
  formatarData,
  fotoMedia,
} from '../api'
import { dadosVazios } from '../api'
import { useAuth } from '../auth'
import { FotoUpload } from '../components/FotoUpload'
import { Bloco, Campo, CampoNumero, CampoTexto, Metrica } from '../components/campos'

const SUGESTOES_ESPACOS = ['Biblioteca', 'Refeitório', 'Área externa', 'Área de lazer']

export function Formulario() {
  const { id } = useParams<{ id: string }>()
  const navegar = useNavigate()
  const [params] = useSearchParams()
  const { usuario } = useAuth()
  const novo = id === 'novo' || !id

  const [escolas, setEscolas] = useState<EscolaResumo[]>([])
  const [escolaId, setEscolaId] = useState(params.get('escola') ?? '')
  const [diag, setDiag] = useState<Diagnostico | null>(null)
  const [dados, setDados] = useState<Dados>(dadosVazios())
  const [carregando, setCarregando] = useState(!novo)
  const [erroFatal, setErroFatal] = useState('')
  const [salvoEm, setSalvoEm] = useState('')
  const [salvando, setSalvando] = useState(false)
  const [pendencias, setPendencias] = useState<string[]>([])
  const [enviadoOk, setEnviadoOk] = useState(false)
  const [novoEspaco, setNovoEspaco] = useState('')
  const [novoCurso, setNovoCurso] = useState('')
  const [novoLab, setNovoLab] = useState('')
  const timer = useRef<number | undefined>(undefined)

  useEffect(() => {
    api<EscolaResumo[]>('/api/escolas').then(setEscolas).catch(() => {})
  }, [])

  // Responsável vinculado à escola abre direto o diagnóstico dela, sem escolher.
  useEffect(() => {
    if (novo && !escolaId && usuario?.escola_id) escolherEscola(usuario.escola_id)
  }, [novo, escolaId, usuario?.escola_id])

  useEffect(() => {
    if (novo) return
    api<Diagnostico>(`/api/diagnosticos/${id}`)
      .then((d) => {
        setDiag(d)
        setDados({ ...dadosVazios(), ...d.dados })
        setCarregando(false)
      })
      .catch((e) => {
        setErroFatal((e as Error).message)
        setCarregando(false)
      })
  }, [id, novo])

  const enviado = diag?.status === 'enviado'
  const escolaAtualId = diag?.escola_id ?? escolaId

  function atualizar(fn: (d: Dados) => Dados) {
    if (enviado) return
    setDados((anterior) => fn(anterior))
  }

  function set<T extends keyof Dados>(campo: T, valor: Dados[T]) {
    atualizar((d) => ({ ...d, [campo]: valor }))
  }

  // cria o rascunho quando a escola é escolhida no modo novo
  function escolherEscola(eid: string) {
    setEscolaId(eid)
    if (!eid) return
    api<Diagnostico>('/api/diagnosticos', {
      method: 'POST',
      body: JSON.stringify({ escola_id: eid }),
    })
      .then((d) => navegar(`/formulario/${d.id}`, { replace: true }))
      .catch((e) => setErroFatal((e as Error).message))
  }

  // salvamento automático do rascunho
  useEffect(() => {
    if (!diag || diag.status !== 'rascunho') return
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => {
      setSalvando(true)
      api(`/api/diagnosticos/${diag.id}/rascunho`, {
        method: 'PUT',
        body: JSON.stringify({ dados }),
      })
        .then(() => setSalvoEm(new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })))
        .catch(() => {})
        .finally(() => setSalvando(false))
    }, 900)
    return () => window.clearTimeout(timer.current)
  }, [dados, diag])

  async function enviar() {
    if (!diag) return
    setPendencias([])
    try {
      const d = await api<Diagnostico>(`/api/diagnosticos/${diag.id}/enviar`, { method: 'POST' })
      setDiag(d)
      setDados({ ...dadosVazios(), ...d.dados })
      setEnviadoOk(true)
      window.scrollTo({ top: 0, behavior: 'smooth' })
    } catch (e) {
      const err = e as ErroAPI
      setPendencias(err.pendencias && err.pendencias.length > 0 ? err.pendencias : [err.message])
    }
  }

  function capturarGPS() {
    if (!navigator.geolocation) {
      alert('Este navegador não suporta captura de localização.')
      return
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        atualizar((d) => ({
          ...d,
          gps: {
            lat: Number(pos.coords.latitude.toFixed(6)),
            lng: Number(pos.coords.longitude.toFixed(6)),
            capturado: true,
          },
        }))
      },
      (err) => alert('Não foi possível capturar a localização: ' + err.message),
      { enableHighAccuracy: true, timeout: 12000 },
    )
  }

  // ---- cálculos das métricas ----
  const serv = dados.servidores
  const totalServ = serv.gestao + serv.efetivos + serv.act + serv.pedagogico
  const docentes = serv.efetivos + serv.act
  const pctEfet = docentes > 0 ? (serv.efetivos / docentes) * 100 : 0
  const pctAct = docentes > 0 ? (serv.act / docentes) * 100 : 0
  const totalEquip =
    dados.equipamentos.computadores +
    dados.equipamentos.tablets +
    dados.equipamentos.projetores +
    dados.equipamentos.telas
  const ocupacao = dados.salas.total > 0 ? (dados.salas.ocupadas / dados.salas.total) * 100 : -1

  const checagens = useMemo(
    () => [
      { ok: !!dados.foto_fachada, texto: 'Foto da fachada da escola (capa do relatório)' },
      { ok: dados.endereco.trim() !== '', texto: 'Endereço da escola' },
      { ok: dados.fotos_antes.length >= 3, texto: `Fotos antes da gestão — mínimo 3 (${dados.fotos_antes.length})` },
      { ok: dados.fotos_atual.length >= 3, texto: `Fotos da gestão atual — mínimo 3 (${dados.fotos_atual.length})` },
    ],
    [dados],
  )
  const podeEnviar = checagens.every((c) => c.ok)

  if (erroFatal) {
    return (
      <div className="erro-caixa">
        {erroFatal} — <a href="/">voltar ao painel</a>
      </div>
    )
  }
  if (carregando || (!novo && !diag)) {
    return (
      <div className="carregando-pagina">
        <div className="girando" /> Carregando formulário…
      </div>
    )
  }

  const escolasComRascunho = new Set(escolas.filter((e) => e.status === 'rascunho').map((e) => e.id))
  const escolasDisponiveis = escolas.filter((e) => !escolasComRascunho.has(e.id) || e.id === escolaId)

  return (
    <>
      <div className="secao-titulo">
        <div>
          <h2>{diag ? diag.escola_nome : 'Formulário de Diagnóstico'}</h2>
          <p className="desc">
            {diag ? (
              <>
                NTE {diag.nte || '—'} ·{' '}
                <span className={'status-chip ' + (enviado ? 'enviado' : 'rascunho')}>
                  {enviado ? '✓ Enviado' : 'Rascunho'}
                </span>{' '}
                · atualizado em {formatarData(diag.atualizado_em)}
                {diag.atualizado_por && <> por {diag.atualizado_por.nome}</>}
              </>
            ) : (
              'Escolha a unidade escolar que está sendo diagnosticada para começar.'
            )}
          </p>
        </div>
        <div className="indicador-salvo">
          {enviado ? (
            <span className="status-chip enviado">✓ Registrado na base da rede</span>
          ) : diag ? (
            salvando ? (
              <>salvando…</>
            ) : (
              <>
                <span className="ponto-verde" /> {salvoEm ? `rascunho salvo às ${salvoEm}` : 'salvamento automático ativo'}
              </>
            )
          ) : null}
        </div>
      </div>

      {enviadoOk && enviado && diag && (
        <div className="sucesso" style={{ marginBottom: 18 }}>
          <strong>✓ Formulário enviado com sucesso!</strong>
          <span>As informações foram registradas na base da rede. Exporte o relatório nos formatos abaixo.</span>
          <div className="exportar-grupo">
            <a className="btn-primario btn-mini" href={`/api/diagnosticos/${diag.id}/relatorio.pdf`}>
              ⬇ Exportar PDF
            </a>
            <a className="btn-suave btn-mini" href={`/api/diagnosticos/${diag.id}/relatorio.docx`}>
              ⬇ Exportar DOCX
            </a>
            <a className="btn-suave btn-mini" href={`/api/diagnosticos/${diag.id}/relatorio.html`} target="_blank" rel="noreferrer">
              ⬇ Exportar HTML
            </a>
            <button className="btn-verde btn-mini" onClick={() => navegar(`/envios/${diag.id}`)}>
              Ver envio
            </button>
          </div>
        </div>
      )}

      {pendencias.length > 0 && (
        <div className="erro-caixa" style={{ marginBottom: 18 }}>
          <strong>Formulário incompleto:</strong>
          <ul>
            {pendencias.map((p, i) => (
              <li key={i}>{p}</li>
            ))}
          </ul>
        </div>
      )}

      {!novo && !enviado && (
        <nav className="secoes">
          {[
            ['identificacao', 'Identificação'],
            ['endereco', 'Endereço e contatos'],
            ['servidores', '1. Servidores'],
            ['infraestrutura', '2. Infraestrutura'],
            ['espacos', '3. Espaços'],
            ['projetos', '4. Projetos'],
            ['cursos', '5. Cursos'],
            ['revisao', 'Revisão e envio'],
          ].map(([ancora, titulo]) => (
            <a key={ancora} href={`#${ancora}`}>
              {titulo}
            </a>
          ))}
        </nav>
      )}

      <fieldset disabled={enviado}>
        {/* ---------- Identificação ---------- */}
        <Bloco id="identificacao" titulo="Identificação da unidade" descricao="Selecione a escola que está sendo diagnosticada.">
          {novo ? (
            <Campo rotulo="Unidade escolar *" dica="Após escolher, o rascunho é criado e o preenchimento começa.">
              <select value={escolaId} onChange={(e) => escolherEscola(e.target.value)}>
                <option value="">Selecione a escola que está sendo diagnosticada…</option>
                {escolasDisponiveis.map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.nome} {e.nte ? `— ${e.nte}` : ''}
                  </option>
                ))}
              </select>
            </Campo>
          ) : (
            <Campo rotulo="Unidade escolar">
              <input value={`${diag!.escola_nome}${diag!.nte ? ` — ${diag!.nte}` : ''}`} readOnly />
            </Campo>
          )}

          <CampoTexto
            rotulo="Nome do(a) gestor(a) ou responsável"
            opcional
            valor={dados.responsavel}
            onChange={(v) => set('responsavel', v)}
            placeholder="Nome de quem responde pelo preenchimento"
          />

          <div className="grade2">
            <div className="subcartao">
              <h4>Foto do gestor</h4>
              <p className="desc">Retrato do(a) gestor(a) — aparece na identificação do relatório.</p>
              {dados.foto_gestor ? (
                <div className="foto-unica" style={{ position: 'relative' }}>
                  <img src={fotoMedia(dados.foto_gestor)} alt="Foto do gestor" />
                  {!enviado && (
                    <button
                      type="button"
                      className="upload-foto remover"
                      style={{ position: 'absolute', top: -7, right: -7 }}
                      onClick={() => set('foto_gestor', null)}
                    >
                      ✕
                    </button>
                  )}
                </div>
              ) : (
                <FotoUpload
                  fotos={[]}
                  escolaId={escolaAtualId}
                  max={1}
                  textoBotao="Enviar foto"
                  onChange={(fs) => set('foto_gestor', fs.length > 0 ? fs[0] : null)}
                />
              )}
            </div>
            <div className="subcartao" style={{ borderColor: dados.foto_fachada ? 'var(--verde)' : '#fca5a5' }}>
              <h4>Foto da fachada da escola *</h4>
              <p className="desc">Frente do prédio, portão ou placa da unidade — registrada como capa do relatório de diagnóstico.</p>
              {dados.foto_fachada ? (
                <div className="foto-unica" style={{ position: 'relative' }}>
                  <img src={fotoMedia(dados.foto_fachada)} alt="Fachada da escola" />
                  {!enviado && (
                    <button
                      type="button"
                      className="upload-foto remover"
                      style={{ position: 'absolute', top: -7, right: -7 }}
                      onClick={() => set('foto_fachada', null)}
                    >
                      ✕
                    </button>
                  )}
                </div>
              ) : (
                <FotoUpload
                  fotos={[]}
                  escolaId={escolaAtualId}
                  max={1}
                  textoBotao="Clique para enviar a foto da fachada"
                  onChange={(fs) => set('foto_fachada', fs.length > 0 ? fs[0] : null)}
                />
              )}
            </div>
          </div>
        </Bloco>

        {/* ---------- Endereço e contatos ---------- */}
        <Bloco
          id="endereco"
          titulo="Endereço, contatos e localização"
          descricao="Onde a unidade está e como falar com cada setor."
        >
          <CampoTexto
            rotulo="Endereço da escola *"
            dica="Rua, número, bairro, cidade — CEP"
            valor={dados.endereco}
            onChange={(v) => set('endereco', v)}
            placeholder="Endereço completo da unidade escolar"
          />

          <div className="grade3">
            {(
              [
                ['Secretaria', 'secretaria'],
                ['Direção', 'direcao'],
                ['Pedagógico', 'pedagogico'],
              ] as const
            ).map(([titulo, chave]) => (
              <div className="subcartao" key={chave}>
                <h4>{titulo}</h4>
                <p className="desc">Telefone e e-mail do setor (opcional).</p>
                <div style={{ display: 'grid', gap: 8 }}>
                  <div>
                    <label className="rotulo">Telefone</label>
                    <input
                      type="tel"
                      value={dados.contatos[chave].telefone}
                      placeholder="(21) 0000-0000"
                      onChange={(e) =>
                        set('contatos', {
                          ...dados.contatos,
                          [chave]: { ...dados.contatos[chave], telefone: e.target.value },
                        })
                      }
                    />
                  </div>
                  <div>
                    <label className="rotulo">E-mail</label>
                    <input
                      type="email"
                      value={dados.contatos[chave].email}
                      placeholder="contato@escola..."
                      onChange={(e) =>
                        set('contatos', {
                          ...dados.contatos,
                          [chave]: { ...dados.contatos[chave], email: e.target.value },
                        })
                      }
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>

          <div className="subcartao">
            <h4>Localização GPS</h4>
            <p className="desc">
              Capture as coordenadas da unidade (opcional) — aparecem no relatório com link para o mapa.
            </p>
            <div style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
              <div style={{ maxWidth: 220 }}>
                <label className="rotulo">Latitude</label>
                <input
                  value={dados.gps.capturado ? dados.gps.lat : ''}
                  readOnly
                  placeholder="—"
                />
              </div>
              <div style={{ maxWidth: 220 }}>
                <label className="rotulo">Longitude</label>
                <input
                  value={dados.gps.capturado ? dados.gps.lng : ''}
                  readOnly
                  placeholder="—"
                />
              </div>
              {!enviado && (
                <button type="button" className="btn-primario" onClick={capturarGPS}>
                  📍 Capturar localização
                </button>
              )}
              {dados.gps.capturado && (
                <a
                  className="btn-suave"
                  href={`https://www.google.com/maps?q=${dados.gps.lat},${dados.gps.lng}`}
                  target="_blank"
                  rel="noreferrer"
                >
                  Ver no mapa
                </a>
              )}
            </div>
          </div>
        </Bloco>

        {/* ---------- 1. Servidores ---------- */}
        <Bloco
          id="servidores"
          numero="1"
          titulo="Servidores"
          descricao="Quadro de pessoal: gestão, pedagógico e docentes."
        >
          <div className="grade4">
            <CampoNumero
              rotulo="Servidores na gestão"
              dica="Direção, vice-direção, secretaria e afins"
              valor={serv.gestao}
              onChange={(v) => set('servidores', { ...serv, gestao: v })}
            />
            <CampoNumero
              rotulo="Servidores no pedagógico"
              dica="Coordenação, supervisão, orientação e apoio"
              valor={serv.pedagogico}
              onChange={(v) => set('servidores', { ...serv, pedagogico: v })}
            />
            <CampoNumero
              rotulo="Professores efetivos"
              dica="Concurso / quadro permanente"
              valor={serv.efetivos}
              onChange={(v) => set('servidores', { ...serv, efetivos: v })}
            />
            <CampoNumero
              rotulo="Professores ACT"
              dica="Admitidos em caráter temporário"
              valor={serv.act}
              onChange={(v) => set('servidores', { ...serv, act: v })}
            />
          </div>
          <div className="subcartao">
            <h4>Métricas do quadro de servidores</h4>
            <div className="metrica-grade">
              <Metrica valor={totalServ} rotulo="Total de servidores" />
              <Metrica valor={serv.gestao} rotulo="Na gestão" />
              <Metrica valor={serv.pedagogico} rotulo="No pedagógico" verde />
            </div>
            {docentes > 0 ? (
              <>
                <div className="barra-externa" style={{ marginTop: 14 }}>
                  <div className="fatia azul" style={{ width: `${pctEfet}%` }} />
                  <div className="fatia verde" style={{ width: `${pctAct}%` }} />
                </div>
                <div className="legenda-barra">
                  <span>
                    <b>{pctEfet.toFixed(0)}%</b> Efetivos ({serv.efetivos})
                  </span>
                  <span>
                    <b>{pctAct.toFixed(0)}%</b> ACT ({serv.act})
                  </span>
                </div>
              </>
            ) : (
              <p className="dica" style={{ marginTop: 10 }}>
                Informe os professores para ver a distribuição efetivos × ACT.
              </p>
            )}
          </div>
        </Bloco>

        {/* ---------- 2. Infraestrutura ---------- */}
        <Bloco
          id="infraestrutura"
          numero="2"
          titulo="Infraestrutura"
          descricao="Salas de aula, equipamentos tecnológicos e registro fotográfico."
        >
          <div className="grade2">
            <CampoNumero
              rotulo="Quantidade total de salas"
              valor={dados.salas.total}
              onChange={(v) => set('salas', { ...dados.salas, total: v })}
            />
            <CampoNumero
              rotulo="Salas ocupadas"
              dica="Salas em uso neste período letivo"
              valor={dados.salas.ocupadas}
              onChange={(v) => set('salas', { ...dados.salas, ocupadas: v })}
            />
          </div>
          <div className="subcartao">
            <h4>
              Ocupação das salas{' '}
              {ocupacao >= 0 && (
                <span style={{ color: 'var(--verde)' }}>
                  — {ocupacao.toFixed(0)}%
                </span>
              )}
            </h4>
            {ocupacao >= 0 ? (
              <div className="barra-externa">
                <div className="fatia azul" style={{ width: `${ocupacao}%` }} />
              </div>
            ) : (
              <p className="dica">Informe o total de salas para calcular.</p>
            )}
          </div>

          <div className="subcartao">
            <h4>
              Equipamentos tecnológicos{' '}
              <span className="status-chip enviado" style={{ marginLeft: 8 }}>
                Total: {totalEquip}
              </span>
            </h4>
            <div className="grade4">
              <CampoNumero
                rotulo="Computadores"
                valor={dados.equipamentos.computadores}
                onChange={(v) => set('equipamentos', { ...dados.equipamentos, computadores: v })}
              />
              <CampoNumero
                rotulo="Tablets"
                valor={dados.equipamentos.tablets}
                onChange={(v) => set('equipamentos', { ...dados.equipamentos, tablets: v })}
              />
              <CampoNumero
                rotulo="Projetores"
                valor={dados.equipamentos.projetores}
                onChange={(v) => set('equipamentos', { ...dados.equipamentos, projetores: v })}
              />
              <CampoNumero
                rotulo="Telas interativas"
                valor={dados.equipamentos.telas}
                onChange={(v) => set('equipamentos', { ...dados.equipamentos, telas: v })}
              />
            </div>
            <div style={{ marginTop: 12 }}>
              <CampoTexto
                rotulo="Outros equipamentos"
                opcional
                valor={dados.equipamentos.outros}
                onChange={(v) => set('equipamentos', { ...dados.equipamentos, outros: v })}
                placeholder="Ex.: impressoras, lousas digitais, notebooks do professor…"
              />
            </div>
          </div>

          <div className="subcartao">
            <h4>Registro fotográfico — antes da gestão 2023/2027 *</h4>
            <p className="desc">
              Envie pelo menos 3 fotos da estrutura física da unidade antes da gestão atual (fachada, salas, quadras
              etc.).
            </p>
            <FotoUpload
              fotos={dados.fotos_antes}
              escolaId={escolaAtualId}
              max={6}
              min={3}
              onChange={(fs) => set('fotos_antes', fs)}
              textoBotao="Adicionar"
            />
          </div>

          <div className="subcartao">
            <h4>Registro fotográfico — gestão atual 2023/2027 *</h4>
            <p className="desc">
              Envie pelo menos 3 fotos da estrutura física da unidade na gestão atual (fachada, salas, quadras etc.).
            </p>
            <FotoUpload
              fotos={dados.fotos_atual}
              escolaId={escolaAtualId}
              max={6}
              min={3}
              onChange={(fs) => set('fotos_atual', fs)}
              textoBotao="Adicionar"
            />
          </div>

          {/* laboratórios */}
          <div className="subcartao">
            <h4>Laboratórios</h4>
            <p className="desc">
              Marque se a escola possui o laboratório — os campos de descrição e fotos serão abertos.
            </p>
            <div style={{ display: 'grid', gap: 12 }}>
              {(
                [
                  ['maker', 'Laboratório maker', 'Espaço de mão na massa: robótica, impressão 3D, eletrônica, artes e prototipagem.'],
                  ['informatica', 'Laboratório de informática', 'Sala com computadores para aulas de tecnologia, pesquisa e atividades digitais.'],
                ] as const
              ).map(([chave, titulo, desc]) => {
                const lab = dados.labs[chave]
                return (
                  <div
                    key={chave}
                    className={'subcartao lab-cartao' + (lab.possui ? ' ativo' : '')}
                    style={{ background: '#fff' }}
                    onClick={() => {
                      if (enviado) return
                      set('labs', { ...dados.labs, [chave]: { ...lab, possui: !lab.possui } })
                    }}
                  >
                    <div className="lab-topo">
                      <div className="lab-check">{lab.possui ? '✓' : ''}</div>
                      <div>
                        <h4 style={{ margin: 0 }}>{titulo}</h4>
                        <p className="desc" style={{ margin: 0 }}>
                          {desc}
                        </p>
                      </div>
                    </div>
                    {lab.possui && (
                      <div className="lab-conteudo" onClick={(e) => e.stopPropagation()}>
                        <CampoTexto
                          rotulo="Descrição"
                          opcional
                          valor={lab.descricao}
                          onChange={(v) => set('labs', { ...dados.labs, [chave]: { ...lab, descricao: v } })}
                          placeholder="Estrutura, capacidade, projetos desenvolvidos…"
                        />
                        <FotoUpload
                          fotos={lab.fotos}
                          escolaId={escolaAtualId}
                          max={6}
                          onChange={(fs) => set('labs', { ...dados.labs, [chave]: { ...lab, fotos: fs } })}
                        />
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
            <div style={{ marginTop: 8, fontSize: 12.5, color: 'var(--cinza)' }}>
              {([dados.labs.maker.possui, dados.labs.informatica.possui].filter(Boolean).length + dados.labs.outros.length)}{' '}
              marcado(s)
            </div>

            {/* outros laboratórios */}
            {dados.labs.outros.map((lo, i) => (
              <div className="subcartao" key={i} style={{ marginTop: 12, background: '#fff' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <h4 style={{ margin: 0 }}>Outro laboratório {i + 1}</h4>
                  {!enviado && (
                    <button
                      type="button"
                      className="btn-perigo btn-mini"
                      onClick={() =>
                        set('labs', { ...dados.labs, outros: dados.labs.outros.filter((_, j) => j !== i) })
                      }
                    >
                      Remover
                    </button>
                  )}
                </div>
                <div style={{ display: 'grid', gap: 10 }}>
                  <CampoTexto
                    rotulo="Nome"
                    valor={lo.nome}
                    onChange={(v) => {
                      const outros = [...dados.labs.outros]
                      outros[i] = { ...outros[i], nome: v }
                      set('labs', { ...dados.labs, outros })
                    }}
                    placeholder="Ex.: Laboratório de ciências, cozinha-escola…"
                  />
                  <CampoTexto
                    rotulo="Descrição"
                    opcional
                    valor={lo.descricao}
                    onChange={(v) => {
                      const outros = [...dados.labs.outros]
                      outros[i] = { ...outros[i], descricao: v }
                      set('labs', { ...dados.labs, outros })
                    }}
                  />
                  <FotoUpload
                    fotos={lo.fotos}
                    escolaId={escolaAtualId}
                    max={6}
                    onChange={(fs) => {
                      const outros = [...dados.labs.outros]
                      outros[i] = { ...outros[i], fotos: fs }
                      set('labs', { ...dados.labs, outros })
                    }}
                  />
                </div>
              </div>
            ))}

            {!enviado && (
              <div style={{ display: 'flex', gap: 10, marginTop: 14, alignItems: 'flex-end', flexWrap: 'wrap' }}>
                <div style={{ flex: 1, minWidth: 240 }}>
                  <CampoTexto
                    rotulo="Adicionar outro laboratório"
                    dica="A escola possui outro laboratório? Adicione quantos forem necessários."
                    valor={novoLab}
                    onChange={setNovoLab}
                    placeholder="Nome do laboratório"
                  />
                </div>
                <button
                  type="button"
                  className="btn-suave"
                  disabled={!novoLab.trim()}
                  onClick={() => {
                    set('labs', {
                      ...dados.labs,
                      outros: [...dados.labs.outros, { nome: novoLab.trim(), descricao: '', fotos: [] }],
                    })
                    setNovoLab('')
                  }}
                >
                  ＋ Adicionar
                </button>
              </div>
            )}
          </div>
        </Bloco>

        {/* ---------- 3. Espaços ---------- */}
        <Bloco
          id="espacos"
          numero="3"
          verde
          titulo="Espaços da escola"
          descricao="Biblioteca, refeitório, áreas externas e outros espaços, com fotos."
        >
          {!enviado && (
            <div>
              <p className="dica" style={{ marginBottom: 8 }}>
                Registre os espaços físicos que a escola possui. Toque em uma sugestão para adicionar rapidamente ou
                inclua um espaço com outro nome.{' '}
                <strong>{dados.espacos.length} adicionado(s)</strong>
              </p>
              <div className="chips" style={{ marginBottom: 10 }}>
                {SUGESTOES_ESPACOS.map((s) => (
                  <button
                    key={s}
                    type="button"
                    className="chip-sugestao"
                    disabled={dados.espacos.some((e) => e.nome.toLowerCase() === s.toLowerCase())}
                    onClick={() => set('espacos', [...dados.espacos, { nome: s, fotos: [] }])}
                  >
                    ＋ {s}
                  </button>
                ))}
              </div>
              <div style={{ display: 'flex', gap: 10, alignItems: 'flex-end', flexWrap: 'wrap' }}>
                <div style={{ flex: 1, minWidth: 240 }}>
                  <CampoTexto
                    rotulo="Adicionar outro espaço"
                    dica="A escola tem outro espaço? Adicione quantos forem necessários."
                    valor={novoEspaco}
                    onChange={setNovoEspaco}
                    placeholder="Ex.: Quadra coberta, sala de leitura, horta…"
                  />
                </div>
                <button
                  type="button"
                  className="btn-suave"
                  disabled={!novoEspaco.trim()}
                  onClick={() => {
                    set('espacos', [...dados.espacos, { nome: novoEspaco.trim(), fotos: [] }])
                    setNovoEspaco('')
                  }}
                >
                  ＋ Adicionar
                </button>
              </div>
            </div>
          )}

          {dados.espacos.length === 0 ? (
            <div className="vazio">Nenhum espaço adicionado ainda. Use as sugestões acima ou inclua outro espaço.</div>
          ) : (
            dados.espacos.map((esp, i) => (
              <div className="subcartao" key={i}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <h4 style={{ margin: 0 }}>{esp.nome}</h4>
                  {!enviado && (
                    <button
                      type="button"
                      className="btn-perigo btn-mini"
                      onClick={() => set('espacos', dados.espacos.filter((_, j) => j !== i))}
                    >
                      Remover
                    </button>
                  )}
                </div>
                <FotoUpload
                  fotos={esp.fotos}
                  escolaId={escolaAtualId}
                  max={6}
                  textoBotao="Adicionar fotos"
                  onChange={(fs) => {
                    const espacos = [...dados.espacos]
                    espacos[i] = { ...espacos[i], fotos: fs }
                    set('espacos', espacos)
                  }}
                />
              </div>
            ))
          )}
        </Bloco>

        {/* ---------- 4. Projetos ---------- */}
        <Bloco
          id="projetos"
          numero="4"
          verde
          titulo="Projetos"
          descricao="Projetos da escola, com descrição e fotos — adicione quantos existirem."
        >
          {dados.projetos.length === 0 && (
            <div className="vazio">
              Nenhum projeto cadastrado — este campo é opcional, mas recomendado.
            </div>
          )}
          {dados.projetos.map((proj, i) => (
            <div className="subcartao" key={i}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <h4 style={{ margin: 0 }}>
                  Projeto {i + 1} <span className="status-chip nenhum" style={{ marginLeft: 8 }}>{dados.projetos.length} cadastrado(s)</span>
                </h4>
                {!enviado && (
                  <button
                    type="button"
                    className="btn-perigo btn-mini"
                    onClick={() => set('projetos', dados.projetos.filter((_, j) => j !== i))}
                  >
                    Remover
                  </button>
                )}
              </div>
              <div style={{ display: 'grid', gap: 10 }}>
                <CampoTexto
                  rotulo="Nome do projeto"
                  valor={proj.nome}
                  onChange={(v) => {
                    const projetos = [...dados.projetos]
                    projetos[i] = { ...projetos[i], nome: v }
                    set('projetos', projetos)
                  }}
                  placeholder="Ex.: Hortas escolares, Robótica educacional…"
                />
                <Campo
                  rotulo="Descrição"
                  opcional
                >
                  <textarea
                    maxLength={2500}
                    value={proj.descricao}
                    onChange={(e) => {
                      const projetos = [...dados.projetos]
                      projetos[i] = { ...projetos[i], descricao: e.target.value }
                      set('projetos', projetos)
                    }}
                    placeholder="Descreva os objetivos, público atendido, atividades e resultados do projeto…"
                  />
                  <div style={{ fontSize: 11.5, color: 'var(--cinza-fraco)', textAlign: 'right', marginTop: 3 }}>
                    {proj.descricao.length}/2500
                  </div>
                </Campo>
                <div>
                  <label className="rotulo">Fotos do projeto {i + 1}</label>
                  <FotoUpload
                    fotos={proj.fotos}
                    escolaId={escolaAtualId}
                    max={3}
                    textoBotao="Adicionar"
                    onChange={(fs) => {
                      const projetos = [...dados.projetos]
                      projetos[i] = { ...projetos[i], fotos: fs }
                      set('projetos', projetos)
                    }}
                  />
                </div>
              </div>
            </div>
          ))}
          {!enviado && (
            <button
              type="button"
              className="btn-primario"
              onClick={() => set('projetos', [...dados.projetos, { nome: '', descricao: '', fotos: [] }])}
            >
              ＋ Adicionar projeto
            </button>
          )}
        </Bloco>

        {/* ---------- 5. Cursos ---------- */}
        <Bloco
          id="cursos"
          numero="5"
          verde
          titulo="Ofertas de cursos"
          descricao="Cursos disponibilizados pela escola à comunidade."
        >
          {!enviado && (
            <div style={{ display: 'flex', gap: 10, alignItems: 'flex-end', flexWrap: 'wrap' }}>
              <div style={{ flex: 1, minWidth: 260 }}>
                <Campo rotulo="Curso" dica="Digite o nome e pressione Enter ou clique em Adicionar.">
                  <input
                    value={novoCurso}
                    onChange={(e) => setNovoCurso(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && novoCurso.trim()) {
                        e.preventDefault()
                        set('cursos', [...dados.cursos, novoCurso.trim()])
                        setNovoCurso('')
                      }
                    }}
                    placeholder="Ex.: Informática básica, EJA, Reforço escolar…"
                  />
                </Campo>
              </div>
              <button
                type="button"
                className="btn-primario"
                disabled={!novoCurso.trim()}
                onClick={() => {
                  set('cursos', [...dados.cursos, novoCurso.trim()])
                  setNovoCurso('')
                }}
              >
                ＋ Adicionar
              </button>
            </div>
          )}
          {dados.cursos.length === 0 ? (
            <div className="vazio">Nenhum curso adicionado ainda.</div>
          ) : (
            <div className="chips">
              {dados.cursos.map((c, i) => (
                <span className="chip azul" key={i}>
                  {c}
                  {!enviado && (
                    <button type="button" title="Remover" onClick={() => set('cursos', dados.cursos.filter((_, j) => j !== i))}>
                      ✕
                    </button>
                  )}
                </span>
              ))}
            </div>
          )}
        </Bloco>

        {/* ---------- Revisão ---------- */}
        {!novo && (
          <Bloco id="revisao" numero="✓" verde titulo="Revisão e envio" descricao="Revise os dados antes de enviar. As informações são registradas na base da rede.">
            <div className="subcartao">
              {checagens.map((c, i) => (
                <div className="revisao-item" key={i}>
                  <div className={'marca ' + (c.ok ? 'ok' : 'falta')}>{c.ok ? '✓' : '✕'}</div>
                  <span>{c.texto}</span>
                </div>
              ))}
              <div className="revisao-item">
                <div className={'marca ' + (totalServ > 0 ? 'ok' : 'ok')} style={totalServ === 0 ? { background: 'var(--aviso-fundo)', color: '#854d0e' } : {}}>
                  {totalServ > 0 ? '✓' : '!'}
                </div>
                <span>{totalServ > 0 ? `Quadro de servidores informado (${totalServ} no total)` : 'Quadro de servidores não informado (opcional)'}</span>
              </div>
            </div>

            {enviado ? (
              <div className="sucesso">
                <strong>✓ Este diagnóstico já foi enviado em {diag?.enviado_em ? formatarData(diag.enviado_em) : '—'}.</strong>
                <div className="exportar-grupo">
                  <a className="btn-primario btn-mini" href={`/api/diagnosticos/${diag!.id}/relatorio.pdf`}>
                    ⬇ Exportar PDF
                  </a>
                  <a className="btn-suave btn-mini" href={`/api/diagnosticos/${diag!.id}/relatorio.docx`}>
                    ⬇ Exportar DOCX
                  </a>
                  <a className="btn-suave btn-mini" href={`/api/diagnosticos/${diag!.id}/relatorio.html`} target="_blank" rel="noreferrer">
                    ⬇ Exportar HTML
                  </a>
                </div>
              </div>
            ) : (
              <>
                {!podeEnviar && (
                  <div className="aviso">
                    Complete os itens marcados com ✕ acima para habilitar o envio. O rascunho é salvo automaticamente.
                  </div>
                )}
                <div>
                  <button type="button" className="btn-verde" disabled={!podeEnviar || salvando} onClick={enviar}>
                    📨 Enviar formulário
                  </button>
                </div>
              </>
            )}
          </Bloco>
        )}
      </fieldset>
    </>
  )
}
