import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  api,
  Dados,
  Diagnostico,
  EscolaResumo,
  ErroAPI,
  formatarData,
  fotoMedia,
  tokenAtual,
} from '../api'
import { dadosVazios } from '../api'
import { useAuth } from '../auth'
import { FotoUpload } from '../components/FotoUpload'
import { Bloco, Campo, CampoNumero, CampoTexto, Metrica } from '../components/campos'

const SUGESTOES_ESPACOS = ['Biblioteca', 'Refeitório', 'Área externa', 'Área de lazer']

interface RascunhoLocal {
  // Relógio do navegador: usado apenas para ordenar cópias locais.
  atualizadoEm: number
  // Versão do servidor (atualizado_em) sobre a qual a cópia local foi editada.
  // Permite decidir qual cópia é a mais recente sem depender do relógio do
  // navegador estar sincronizado com o do servidor.
  baseServidorEm?: string | null
  dados: Dados
}

function chaveRascunhoLocal(diagnosticoId: string, usuarioId?: string): string {
  return `diagnostico-rascunho:${usuarioId ?? 'usuario'}:${diagnosticoId}`
}

// serializar gera uma representação estável (chaves ordenadas) dos dados, para
// comparar a cópia local com o que está gravado no servidor.
function serializar(valor: unknown): string {
  if (Array.isArray(valor)) return `[${valor.map(serializar).join(',')}]`
  if (valor && typeof valor === 'object') {
    const objeto = valor as Record<string, unknown>
    const chaves = Object.keys(objeto)
      .filter((k) => objeto[k] !== undefined)
      .sort()
    return `{${chaves.map((k) => `${JSON.stringify(k)}:${serializar(objeto[k])}`).join(',')}}`
  }
  return JSON.stringify(valor ?? null)
}

// copiaLocalDevePrevalecer indica se a cópia guardada no navegador tem edições
// que ainda não chegaram ao servidor (ex.: formulário fechado sem conexão).
function copiaLocalDevePrevalecer(local: RascunhoLocal, servidor: Diagnostico, dadosServidor: Dados): boolean {
  if (!local?.dados || typeof local.dados !== 'object') return false
  // Nada a recuperar quando o servidor já tem exatamente os mesmos dados.
  if (serializar({ ...dadosVazios(), ...local.dados }) === serializar(dadosServidor)) return false
  // Cópia editada sobre a versão que o servidor ainda guarda: as edições locais
  // são mais novas do que o registro gravado, mesmo com relógios diferentes.
  if (local.baseServidorEm && local.baseServidorEm === servidor.atualizado_em) return true
  // Sem versão de referência (cópias antigas) ou registro alterado depois no
  // servidor: decide pelo relógio do navegador, como último recurso.
  const relogioServidor = new Date(servidor.atualizado_em).getTime()
  return Number.isFinite(local.atualizadoEm) && local.atualizadoEm > relogioServidor
}

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
  const [estadoSalvamento, setEstadoSalvamento] = useState<
    'aguardando' | 'pendente' | 'salvando' | 'salvo' | 'erro'
  >('aguardando')
  const [erroSalvamento, setErroSalvamento] = useState('')
  const [pendencias, setPendencias] = useState<string[]>([])
  const [enviadoOk, setEnviadoOk] = useState(false)
  const [novoEspaco, setNovoEspaco] = useState('')
  const [novoEspacoRevitalizacao, setNovoEspacoRevitalizacao] = useState('')
  const [novoCurso, setNovoCurso] = useState('')
  const [novoLab, setNovoLab] = useState('')
  const timer = useRef<number | undefined>(undefined)
  const retryTimer = useRef<number | undefined>(undefined)
  const versaoDados = useRef(0)
  const ultimaVersaoSalva = useRef(0)
  const ultimaVersaoKeepalive = useRef(-1)
  const ultimaAtualizacaoLocal = useRef(0)
  const filaSalvamento = useRef<Promise<void>>(Promise.resolve())
  const dadosAtuais = useRef(dados)
  const erroArmazenamentoLocal = useRef(false)
  // Evita criar dois rascunhos quando a escola é definida por vínculo/URL.
  const criandoRascunho = useRef(false)
  // Última versão (atualizado_em) confirmada pelo servidor.
  const baseServidorEm = useRef<string | null>(null)

  useLayoutEffect(() => {
    dadosAtuais.current = dados
    versaoDados.current += 1
  }, [dados])

  useEffect(() => {
    api<EscolaResumo[]>('/api/escolas').then(setEscolas).catch(() => {})
  }, [])

  // Cria o rascunho assim que a escola está definida — pelo vínculo do
  // responsável (usuario.escola_id) ou pela escola escolhida no painel/URL
  // (?escola=...). Assim o preenchimento nunca começa sem um rascunho salvo.
  useEffect(() => {
    if (!novo || diag) return
    const alvo = escolaId || usuario?.escola_id || ''
    if (alvo) escolherEscola(alvo)
  }, [novo, diag, escolaId, usuario?.escola_id])

  useEffect(() => {
    if (novo) return
    api<Diagnostico>(`/api/diagnosticos/${id}`)
      .then((d) => {
        setDiag(d)
        baseServidorEm.current = d.atualizado_em
        let dadosCarregados = { ...dadosVazios(), ...d.dados }
        if (d.status === 'rascunho') {
          try {
            const chave = chaveRascunhoLocal(d.id, usuario?.id)
            const bruto = localStorage.getItem(chave)
            if (bruto) {
              const local: RascunhoLocal = JSON.parse(bruto)
              if (copiaLocalDevePrevalecer(local, d, dadosCarregados)) {
                dadosCarregados = { ...dadosVazios(), ...local.dados }
                setErroSalvamento('Rascunho local recuperado; salvando no servidor.')
              } else {
                localStorage.removeItem(chave)
              }
            }
          } catch {
            setErroSalvamento('Não foi possível ler a cópia local do rascunho.')
          }
        }
        if (d.status === 'rascunho' && usuario?.nome) {
          dadosCarregados.responsavel = usuario.nome
        } else {
          dadosCarregados.responsavel ||= usuario?.nome ?? ''
        }
        setDados(dadosCarregados)
        if (d.status === 'rascunho') {
          setSalvoEm(new Date(d.atualizado_em).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' }))
          setEstadoSalvamento('pendente')
        }
        setCarregando(false)
      })
      .catch((e) => {
        setErroFatal((e as Error).message)
        setCarregando(false)
      })
  }, [id, novo, usuario?.id, usuario?.nome])

  const enviado = diag?.status === 'enviado'
  const escolaAtualId = diag?.escola_id ?? escolaId
  const nomePreenchedor = enviado
    ? dados.responsavel || diag?.atualizado_por?.nome || usuario?.nome
    : usuario?.nome

  function atualizar(fn: (d: Dados) => Dados) {
    if (enviado) return
    setDados((anterior) => fn(anterior))
  }

  function set<T extends keyof Dados>(campo: T, valor: Dados[T]) {
    atualizar((d) => ({ ...d, [campo]: valor }))
  }

  // Cria (ou retoma) o rascunho da escola escolhida.
  function escolherEscola(eid: string) {
    setEscolaId(eid)
    if (!eid || criandoRascunho.current) return
    criandoRascunho.current = true
    api<Diagnostico>('/api/diagnosticos', {
      method: 'POST',
      body: JSON.stringify({ escola_id: eid }),
    })
      .then((d) => {
        baseServidorEm.current = d.atualizado_em
        navegar(`/formulario/${d.id}`, { replace: true })
      })
      .catch((e) => setErroFatal((e as Error).message))
      .finally(() => {
        criandoRascunho.current = false
      })
  }

  // Repete a criação/retomada do rascunho depois de uma falha (ex.: queda de
  // rede ou erro momentâneo do servidor). Os dados digitados continuam na tela.
  function tentarCriarNovamente() {
    setErroFatal('')
    if (escolaId) escolherEscola(escolaId)
  }

  // ---- apoio ao salvamento do rascunho ----

  // Guarda a cópia de segurança no navegador (usada quando o envio falha).
  function guardarCopiaLocal(diagId: string, snapshot: Dados): number {
    const atualizadoEm = Math.max(Date.now(), ultimaAtualizacaoLocal.current + 1)
    ultimaAtualizacaoLocal.current = atualizadoEm
    try {
      localStorage.setItem(
        chaveRascunhoLocal(diagId, usuario?.id),
        JSON.stringify({
          atualizadoEm,
          baseServidorEm: baseServidorEm.current,
          dados: snapshot,
        } satisfies RascunhoLocal),
      )
      erroArmazenamentoLocal.current = false
    } catch {
      erroArmazenamentoLocal.current = true
    }
    return atualizadoEm
  }

  // Remove a cópia local apenas quando ela corresponde ao que o servidor confirmou.
  function limparCopiaLocal(diagId: string, snapshot: Dados, atualizadoEm: number) {
    try {
      const chave = chaveRascunhoLocal(diagId, usuario?.id)
      const bruto = localStorage.getItem(chave)
      if (!bruto) return
      const local = JSON.parse(bruto) as RascunhoLocal
      if (local.atualizadoEm <= atualizadoEm && serializar(local.dados) === serializar(snapshot)) {
        localStorage.removeItem(chave)
      }
    } catch {
      erroArmazenamentoLocal.current = true
    }
  }

  // Enfileira o envio, garantindo que as versões cheguem ao servidor em ordem.
  function enfileirarSalvamento(diagId: string, snapshot: Dados) {
    const pedido = filaSalvamento.current.then(() =>
      api<{ ok: boolean; atualizado_em?: string }>(`/api/diagnosticos/${diagId}/rascunho`, {
        method: 'PUT',
        body: JSON.stringify({ dados: snapshot }),
      }),
    )
    filaSalvamento.current = pedido.then(() => undefined, () => undefined)
    return pedido
  }

  // Falhas que não adianta repetir (rascunho já enviado, sem permissão, removido).
  function falhaDefinitiva(erro: unknown): boolean {
    return erro instanceof ErroAPI && [403, 404, 409].includes(erro.status ?? 0)
  }

  // salvamento automático do rascunho
  useEffect(() => {
    if (!diag || diag.status !== 'rascunho') return
    const diagId = diag.id
    const versao = versaoDados.current
    window.clearTimeout(timer.current)
    window.clearTimeout(retryTimer.current)
    setEstadoSalvamento('pendente')
    setErroSalvamento('')

    const atualizadoEm = guardarCopiaLocal(diagId, dados)

    function salvar(snapshot: Dados, versaoSnapshot: number) {
      if (versaoSnapshot === versaoDados.current) setEstadoSalvamento('salvando')
      enfileirarSalvamento(diagId, snapshot).then(
        (resposta) => {
          if (resposta?.atualizado_em) baseServidorEm.current = resposta.atualizado_em
          ultimaVersaoSalva.current = Math.max(ultimaVersaoSalva.current, versaoSnapshot)
          if (versaoSnapshot !== versaoDados.current) return
          limparCopiaLocal(diagId, snapshot, atualizadoEm)
          setSalvoEm(new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' }))
          setEstadoSalvamento('salvo')
          setErroSalvamento(
            erroArmazenamentoLocal.current ? 'O servidor salvou, mas a cópia local não pôde ser atualizada.' : '',
          )
        },
        (erro: unknown) => {
          if (falhaDefinitiva(erro)) {
            setEstadoSalvamento('erro')
            setErroSalvamento(
              `${erro instanceof Error ? erro.message : 'Não foi possível salvar o rascunho.'} ` +
                'Recarregue a página para continuar — suas alterações ficam guardadas neste navegador.',
            )
            return
          }
          if (versaoSnapshot !== versaoDados.current) return
          setEstadoSalvamento('erro')
          setErroSalvamento(erro instanceof Error ? erro.message : 'Falha ao salvar o rascunho.')
          retryTimer.current = window.setTimeout(() => salvar(dadosAtuais.current, versaoDados.current), 3000)
        },
      )
    }

    timer.current = window.setTimeout(() => {
      salvar(dados, versao)
    }, 900)
    return () => {
      window.clearTimeout(timer.current)
      window.clearTimeout(retryTimer.current)
    }
  }, [dados, diag, usuario?.id])

  // Envia a versão mais recente mesmo quando a página é ocultada ou encerrada.
  useEffect(() => {
    if (!diag || diag.status !== 'rascunho') return

    function salvarAoSair() {
      const versao = versaoDados.current
      if (versao <= ultimaVersaoSalva.current || versao === ultimaVersaoKeepalive.current) return
      ultimaVersaoKeepalive.current = versao

      const dadosSaida = dadosAtuais.current
      const atualizadoEm = guardarCopiaLocal(diag.id, dadosSaida)

      const token = tokenAtual()
      void fetch(`/api/diagnosticos/${diag.id}/rascunho`, {
        method: 'PUT',
        credentials: 'same-origin',
        keepalive: true,
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ dados: dadosSaida }),
      }).then((resposta) => {
        if (!resposta.ok && versao === versaoDados.current) {
          setEstadoSalvamento('erro')
          setErroSalvamento('Não foi possível confirmar o salvamento antes de sair.')
        } else if (resposta.ok) {
          ultimaVersaoSalva.current = Math.max(ultimaVersaoSalva.current, versao)
          limparCopiaLocal(diag.id, dadosSaida, atualizadoEm)
          if (versao === versaoDados.current) {
            setSalvoEm(new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' }))
            setEstadoSalvamento('salvo')
            setErroSalvamento('')
          }
        }
      }).catch(() => {
        if (versao === versaoDados.current) {
          setEstadoSalvamento('erro')
          setErroSalvamento('Sem conexão para confirmar o salvamento. Tentaremos novamente.')
        }
      })
    }

    function aoMudarVisibilidade() {
      if (document.visibilityState === 'hidden') salvarAoSair()
    }

    window.addEventListener('pagehide', salvarAoSair)
    document.addEventListener('visibilitychange', aoMudarVisibilidade)
    return () => {
      window.removeEventListener('pagehide', salvarAoSair)
      document.removeEventListener('visibilitychange', aoMudarVisibilidade)
      salvarAoSair()
    }
  }, [diag, usuario?.id])

  async function enviar() {
    if (!diag) return
    setPendencias([])
    // Cancela salvamentos agendados para não competirem com o envio.
    window.clearTimeout(timer.current)
    window.clearTimeout(retryTimer.current)
    try {
      const d = await api<Diagnostico>(`/api/diagnosticos/${diag.id}/enviar`, { method: 'POST' })
      setDiag(d)
      setDados({ ...dadosVazios(), ...d.dados })
      // O rascunho virou envio: a cópia de segurança local não é mais necessária.
      try {
        localStorage.removeItem(chaveRascunhoLocal(d.id, usuario?.id))
      } catch {
        /* sem acesso ao armazenamento local */
      }
      setErroSalvamento('')
      setEstadoSalvamento('salvo')
      setSalvoEm(new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' }))
      setEnviadoOk(true)
      window.scrollTo({ top: 0, behavior: 'smooth' })
    } catch (e) {
      const err = e as ErroAPI
      if (err.status === 409) {
        // Já enviado em outra aba/dispositivo: reflete o estado atual do servidor.
        try {
          const atual = await api<Diagnostico>(`/api/diagnosticos/${diag.id}`)
          setDiag(atual)
          setDados({ ...dadosVazios(), ...atual.dados })
        } catch {
          /* mantém o formulário como está */
        }
      }
      setPendencias(err.pendencias && err.pendencias.length > 0 ? err.pendencias : [err.message])
    }
  }

  async function salvarRascunho() {
    if (!diag || enviado) return
    window.clearTimeout(timer.current)
    window.clearTimeout(retryTimer.current)

    const snapshot = dados
    const versao = versaoDados.current
    const atualizadoEm = guardarCopiaLocal(diag.id, snapshot)

    setEstadoSalvamento('salvando')
    setErroSalvamento('')
    try {
      const resposta = await enfileirarSalvamento(diag.id, snapshot)
      if (resposta?.atualizado_em) baseServidorEm.current = resposta.atualizado_em
      ultimaVersaoSalva.current = Math.max(ultimaVersaoSalva.current, versao)
      if (versao !== versaoDados.current) {
        setEstadoSalvamento('pendente')
        return
      }
      limparCopiaLocal(diag.id, snapshot, atualizadoEm)
      setSalvoEm(new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' }))
      setEstadoSalvamento('salvo')
      setErroSalvamento('')
    } catch (erro) {
      setEstadoSalvamento('erro')
      setErroSalvamento(erro instanceof Error ? erro.message : 'Não foi possível salvar o rascunho.')
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

  // Obras e espaços que precisam de revitalização (listas opcionais do formulário).
  const obras = dados.obras ?? []
  const revitalizacao = dados.revitalizacao ?? []
  // Obras com fotos de apenas um dos lados (antes ou depois) ficam em destaque
  // na revisão, para o comparativo do mesmo ângulo não ficar incompleto.
  const obrasSemPar = obras.filter((o) => (o.fotos_antes?.length ?? 0) === 0 || (o.fotos_depois?.length ?? 0) === 0)

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
        {novo && !diag && escolaId && (
          <div className="acoes-linha" style={{ marginTop: 10 }}>
            <button type="button" className="btn-primario btn-mini" onClick={tentarCriarNovamente}>
              Tentar novamente
            </button>
          </div>
        )}
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
    <div className="formulario-pagina">
      <div className="formulario-cabecalho">
        <div className="formulario-cabecalho-principal">
          <span className="formulario-sobretitulo">Diagnóstico escolar</span>
          <h2>{diag ? diag.escola_nome : 'Vamos começar?'}</h2>
          <p>
            {diag
              ? `Atualizado em ${formatarData(diag.atualizado_em)}${diag.atualizado_por ? ` por ${diag.atualizado_por.nome}` : ''}`
              : 'Selecione sua escola e preencha as informações da unidade.'}
          </p>
          {nomePreenchedor && (
            <div className="formulario-identidade">
              <span className="formulario-identidade-icone" aria-hidden="true">✓</span>
              <span>Preenchido por <strong>{nomePreenchedor}</strong></span>
            </div>
          )}
        </div>
        <div className="formulario-cabecalho-status">
          {diag && (
            <span className={'status-chip ' + (enviado ? 'enviado' : 'rascunho')}>
              {enviado ? '✓ Enviado' : 'Em preenchimento'}
            </span>
          )}
          <div className="indicador-salvo">
            {diag && !enviado && (
              estadoSalvamento === 'salvando' || estadoSalvamento === 'pendente' ? (
                <>
                  {estadoSalvamento === 'salvando' ? 'Salvando alterações…' : 'Alterações pendentes de salvamento…'}
                  {salvoEm && <span>Último salvamento: {salvoEm}</span>}
                </>
              ) : estadoSalvamento === 'erro' ? (
                <span className="indicador-salvo-erro" role="status">
                  Falha ao salvar; tentando novamente. {erroSalvamento}
                  {salvoEm && ` Último salvamento: ${salvoEm}.`}
                </span>
              ) : (
                <>
                  <span className="ponto-verde" />
                  {salvoEm ? `Salvo às ${salvoEm}` : 'Salvamento automático ativo'}
                </>
              )
            )}
          </div>
        </div>
      </div>

      {diag && !enviado && (
        <div className="rascunho-barra" role="region" aria-label="Salvar rascunho">
          <div className="rascunho-barra-texto">
            <strong>Continue quando quiser</strong>
            <span>Salve seu rascunho e retome depois pelo botão “Continuar rascunho” no painel.</span>
            {estadoSalvamento === 'erro' && (
              <span className="indicador-salvo-erro" role="alert">{erroSalvamento}</span>
            )}
            {estadoSalvamento === 'salvo' && salvoEm && (
              <span className="rascunho-confirmacao" role="status">Salvo no servidor às {salvoEm}</span>
            )}
          </div>
          <button
            type="button"
            className="btn-primario rascunho-botao"
            disabled={estadoSalvamento === 'salvando'}
            onClick={salvarRascunho}
          >
            {estadoSalvamento === 'salvando' ? 'Salvando…' : 'Salvar rascunho'}
          </button>
        </div>
      )}

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
            ['obras', '4. Obras'],
            ['revitalizacao', '5. Revitalização'],
            ['projetos', '6. Projetos'],
            ['cursos', '7. Cursos'],
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
        <Bloco id="identificacao" titulo="Identificação da unidade" descricao="Confira a escola e as imagens que identificam a unidade.">
          {novo ? (
            <Campo rotulo="Unidade escolar *" dica="Após escolher, o rascunho é criado e o preenchimento começa.">
              <select value={escolaId} onChange={(e) => escolherEscola(e.target.value)}>
                <option value="">Selecione a escola que está sendo diagnosticada…</option>
                {escolasDisponiveis.map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.nome}
                  </option>
                ))}
              </select>
            </Campo>
          ) : (
            <Campo rotulo="Unidade escolar">
              <input value={diag!.escola_nome} readOnly />
            </Campo>
          )}

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
          descricao="Informe o nome e a descrição de cada espaço e adicione até 3 fotos."
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
                    onClick={() => set('espacos', [...dados.espacos, { nome: s, descricao: '', fotos: [] }])}
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
                    set('espacos', [...dados.espacos, { nome: novoEspaco.trim(), descricao: '', fotos: [] }])
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
              <div className="subcartao espaco-editor" key={i}>
                <div className="espaco-editor-cabeca">
                  <h4>Espaço {i + 1}</h4>
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
                <CampoTexto
                  rotulo="Nome do espaço"
                  valor={esp.nome}
                  onChange={(v) => {
                    const espacos = [...dados.espacos]
                    espacos[i] = { ...espacos[i], nome: v }
                    set('espacos', espacos)
                  }}
                  placeholder="Ex.: Biblioteca, quadra, horta…"
                />
                <Campo rotulo="Descrição do espaço" opcional>
                  <textarea
                    value={esp.descricao ?? ''}
                    onChange={(e) => {
                      const espacos = [...dados.espacos]
                      espacos[i] = { ...espacos[i], descricao: e.target.value }
                      set('espacos', espacos)
                    }}
                    placeholder="Descreva as características e condições deste espaço."
                  />
                </Campo>
                <FotoUpload
                  fotos={esp.fotos}
                  escolaId={escolaAtualId}
                  max={3}
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

        {/* ---------- 4. Obras realizadas ---------- */}
        <Bloco
          id="obras"
          numero="4"
          verde
          titulo="Obras realizadas na unidade escolar"
          descricao="Descreva cada obra concluída e envie um par de fotos: antes e depois."
        >
          <div className="aviso" style={{ marginBottom: 14 }}>
            <strong>Orientação para as fotos:</strong> escolha um ponto fixo (portão, esquina, canto da sala) e
            fotografe <b>antes</b> e <b>depois</b> da obra <b>do mesmo ângulo, da mesma distância e na mesma
            direção</b>. Mantenha a mesma altura da câmera e evite zoom ou troca de lado. Assim o antes e o depois
            ficam alinhados e mostram claramente a transformação do espaço.
          </div>

          {obras.length === 0 && (
            <div className="vazio">
              Nenhuma obra registrada ainda. Adicione as obras realizadas na unidade, inclusive as mais simples
              (pintura, troca de piso, cobertura, mobiliário).
            </div>
          )}

          {obras.map((obra, i) => (
            <div className="subcartao" key={i} style={{ marginBottom: 14 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 10, marginBottom: 8 }}>
                <h4 style={{ margin: 0 }}>
                  Obra {i + 1}{' '}
                  <span className="status-chip nenhum" style={{ marginLeft: 8 }}>
                    {obras.length} cadastrada(s)
                  </span>
                </h4>
                {!enviado && (
                  <button
                    type="button"
                    className="btn-perigo btn-mini"
                    onClick={() => set('obras', obras.filter((_, j) => j !== i))}
                  >
                    Remover
                  </button>
                )}
              </div>
              <div style={{ display: 'grid', gap: 12 }}>
                <CampoTexto
                  rotulo="Obra realizada"
                  valor={obra.nome}
                  onChange={(v) => {
                    const lista = [...obras]
                    lista[i] = { ...lista[i], nome: v }
                    set('obras', lista)
                  }}
                  placeholder="Ex.: Reforma do telhado, pintura da quadra, troca do piso da sala 3…"
                />
                <Campo rotulo="Descrição da obra" dica="O que foi feito, quando e qual problema resolveu.">
                  <textarea
                    maxLength={2500}
                    value={obra.descricao}
                    onChange={(e) => {
                      const lista = [...obras]
                      lista[i] = { ...lista[i], descricao: e.target.value }
                      set('obras', lista)
                    }}
                    placeholder="Descreva a intervenção realizada, o período de execução e o resultado para a escola…"
                  />
                  <div style={{ fontSize: 11.5, color: 'var(--cinza-fraco)', textAlign: 'right', marginTop: 3 }}>
                    {obra.descricao.length}/2500
                  </div>
                </Campo>
                <div className="grade2">
                  <div>
                    <label className="rotulo">Fotos — antes da obra</label>
                    <p className="dica">Mesmo ângulo, distância e enquadramento usados no depois.</p>
                    <FotoUpload
                      fotos={obra.fotos_antes ?? []}
                      escolaId={escolaAtualId}
                      max={4}
                      textoBotao="Foto de antes"
                      onChange={(fs) => {
                        const lista = [...obras]
                        lista[i] = { ...lista[i], fotos_antes: fs }
                        set('obras', lista)
                      }}
                    />
                  </div>
                  <div>
                    <label className="rotulo">Fotos — depois da obra</label>
                    <p className="dica">Repita o enquadramento das fotos de antes.</p>
                    <FotoUpload
                      fotos={obra.fotos_depois ?? []}
                      escolaId={escolaAtualId}
                      max={4}
                      textoBotao="Foto de depois"
                      onChange={(fs) => {
                        const lista = [...obras]
                        lista[i] = { ...lista[i], fotos_depois: fs }
                        set('obras', lista)
                      }}
                    />
                  </div>
                </div>
                {((obra.fotos_antes?.length ?? 0) === 0 || (obra.fotos_depois?.length ?? 0) === 0) && (
                  <div className="aviso">
                    Para o comparativo ficar completo, envie pelo menos uma foto de <b>antes</b> e uma de{' '}
                    <b>depois</b> desta obra, tiradas do mesmo ângulo.
                  </div>
                )}
              </div>
            </div>
          ))}

          {!enviado && (
            <button
              type="button"
              className="btn-primario"
              onClick={() =>
                set('obras', [
                  ...obras,
                  { nome: '', descricao: '', fotos_antes: [], fotos_depois: [] },
                ])
              }
            >
              ＋ Adicionar obra
            </button>
          )}
        </Bloco>

        {/* ---------- 5. Espaços que precisam de revitalização ---------- */}
        <Bloco
          id="revitalizacao"
          numero="5"
          verde
          titulo="Espaços que precisam de revitalização"
          descricao="Aponte os espaços que necessitam de intervenção, com descrição e fotos da situação atual."
        >
          <div className="aviso" style={{ marginBottom: 14 }}>
            <strong>Como registrar:</strong> fotografe o espaço exatamente como ele está hoje, de frente e de perto
            do problema (infiltração, piso quebrado, mobiliário danificado, iluminação precária…). Essas imagens
            orientam o planejamento das próximas obras.
          </div>

          {!enviado && (
            <div>
              <p className="dica" style={{ marginBottom: 8 }}>
                Sugestões rápidas — toque para adicionar. <strong>{revitalizacao.length} adicionado(s)</strong>
              </p>
              <div className="chips" style={{ marginBottom: 10 }}>
                {SUGESTOES_ESPACOS.map((s) => (
                  <button
                    key={s}
                    type="button"
                    className="chip-sugestao"
                    disabled={revitalizacao.some((r) => (r.nome ?? '').toLowerCase() === s.toLowerCase())}
                    onClick={() =>
                      set('revitalizacao', [
                        ...revitalizacao,
                        { nome: s, descricao: '', fotos: [] },
                      ])
                    }
                  >
                    ＋ {s}
                  </button>
                ))}
              </div>
            </div>
          )}

          {revitalizacao.length === 0 ? (
            <div className="vazio">
              Nenhum espaço apontado ainda. Use as sugestões acima ou inclua um espaço com outro nome.
            </div>
          ) : (
            revitalizacao.map((espaco, i) => (
              <div className="subcartao espaco-editor" key={i} style={{ marginBottom: 14 }}>
                <div className="espaco-editor-cabeca">
                  <h4>Espaço {i + 1}</h4>
                  {!enviado && (
                    <button
                      type="button"
                      className="btn-perigo btn-mini"
                      onClick={() => set('revitalizacao', revitalizacao.filter((_, j) => j !== i))}
                    >
                      Remover
                    </button>
                  )}
                </div>
                <CampoTexto
                  rotulo="Nome do espaço"
                  valor={espaco.nome}
                  onChange={(v) => {
                    const lista = [...revitalizacao]
                    lista[i] = { ...lista[i], nome: v }
                    set('revitalizacao', lista)
                  }}
                  placeholder="Ex.: Quadra descoberta, banheiro do bloco B, pátio…"
                />
                <Campo rotulo="Descrição da necessidade" dica="O que precisa ser feito e por quê.">
                  <textarea
                    maxLength={2500}
                    value={espaco.descricao}
                    onChange={(e) => {
                      const lista = [...revitalizacao]
                      lista[i] = { ...lista[i], descricao: e.target.value }
                      set('revitalizacao', lista)
                    }}
                    placeholder="Descreva o problema atual e a intervenção necessária…"
                  />
                  <div style={{ fontSize: 11.5, color: 'var(--cinza-fraco)', textAlign: 'right', marginTop: 3 }}>
                    {espaco.descricao.length}/2500
                  </div>
                </Campo>
                <div>
                  <label className="rotulo">Fotos do espaço</label>
                  <p className="dica">Mostre a situação atual do espaço por mais de um ângulo.</p>
                  <FotoUpload
                    fotos={espaco.fotos ?? []}
                    escolaId={escolaAtualId}
                    max={4}
                    textoBotao="Adicionar fotos"
                    onChange={(fs) => {
                      const lista = [...revitalizacao]
                      lista[i] = { ...lista[i], fotos: fs }
                      set('revitalizacao', lista)
                    }}
                  />
                </div>
              </div>
            ))
          )}

          {!enviado && (
            <div style={{ display: 'flex', gap: 10, alignItems: 'flex-end', flexWrap: 'wrap' }}>
              <div style={{ flex: 1, minWidth: 240 }}>
                <CampoTexto
                  rotulo="Adicionar outro espaço"
                  dica="A escola tem outro espaço que precisa de revitalização? Adicione quantos forem necessários."
                  valor={novoEspacoRevitalizacao}
                  onChange={setNovoEspacoRevitalizacao}
                  placeholder="Ex.: Telhado do bloco A, horta, sala de leitura…"
                />
              </div>
              <button
                type="button"
                className="btn-suave"
                disabled={!novoEspacoRevitalizacao.trim()}
                onClick={() => {
                  set('revitalizacao', [
                    ...revitalizacao,
                    { nome: novoEspacoRevitalizacao.trim(), descricao: '', fotos: [] },
                  ])
                  setNovoEspacoRevitalizacao('')
                }}
              >
                ＋ Adicionar
              </button>
            </div>
          )}
        </Bloco>

        {/* ---------- 6. Projetos ---------- */}
        <Bloco
          id="projetos"
          numero="6"
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

        {/* ---------- 7. Cursos ---------- */}
        <Bloco
          id="cursos"
          numero="7"
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
              {(obras.length > 0 || revitalizacao.length > 0) && (
                <div className="revisao-item">
                  <div
                    className="marca ok"
                    style={obrasSemPar.length > 0 ? { background: 'var(--aviso-fundo)', color: '#854d0e' } : {}}
                  >
                    {obrasSemPar.length > 0 ? '!' : '✓'}
                  </div>
                  <span>
                    {obras.length} obra(s) realizada(s) · {revitalizacao.length} espaço(s) que precisam de revitalização
                    {obrasSemPar.length > 0
                      ? ` — ${obrasSemPar.length} obra(s) ainda sem o par de fotos antes/depois tiradas do mesmo ângulo`
                      : ''}
                  </span>
                </div>
              )}
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
                <button
                  type="button"
                  className="btn-suave"
                  disabled={estadoSalvamento === 'salvando'}
                  onClick={salvarRascunho}
                >
                  {estadoSalvamento === 'salvando' ? 'Salvando…' : 'Salvar rascunho'}
                </button>
                {!podeEnviar && (
                  <div className="aviso">
                    Complete os itens marcados com ✕ acima para habilitar o envio. O rascunho é salvo automaticamente.
                  </div>
                )}
                <div>
                  <button
                    type="button"
                    className="btn-verde"
                    disabled={!podeEnviar || estadoSalvamento !== 'salvo'}
                    onClick={enviar}
                  >
                    📨 Enviar formulário
                  </button>
                </div>
              </>
            )}
          </Bloco>
        )}
      </fieldset>
    </div>
  )
}
