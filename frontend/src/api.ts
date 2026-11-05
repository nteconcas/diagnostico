// Tipos e helpers de comunicação com a API Go.

export type Papel = 'admin' | 'preenchador'

export interface Usuario {
  id: string
  username: string
  nome: string
  papel: Papel
  escola_id?: string | null
  escola_nome?: string
  token?: string
}

// Sessão via token Bearer: essencial quando o app roda em iframes
// (ex.: preview do chat), onde navegadores bloqueiam cookies de terceiros.
const CHAVE_TOKEN = 'diag_token'

export function tokenAtual(): string | null {
  try {
    return localStorage.getItem(CHAVE_TOKEN)
  } catch {
    return null
  }
}

export function definirToken(t: string | null): void {
  try {
    if (t) localStorage.setItem(CHAVE_TOKEN, t)
    else localStorage.removeItem(CHAVE_TOKEN)
  } catch {}
}

export interface Foto {
  id: string
  nome: string
}

export const fotoThumb = (f: Foto) => `/fotos/${f.id}.t.jpg`
export const fotoMedia = (f: Foto) => `/fotos/${f.id}.m.jpg`

export interface Contato {
  telefone: string
  email: string
}

export interface LabFixo {
  possui: boolean
  descricao: string
  fotos: Foto[]
}

export interface LabOutro {
  nome: string
  descricao: string
  fotos: Foto[]
}

export interface Espaco {
  nome: string
  descricao: string
  fotos: Foto[]
}

export interface Projeto {
  nome: string
  descricao: string
  fotos: Foto[]
}

export interface Dados {
  responsavel: string
  foto_gestor: Foto | null
  foto_fachada: Foto | null
  endereco: string
  contatos: { secretaria: Contato; direcao: Contato; pedagogico: Contato }
  gps: { lat: number; lng: number; capturado: boolean }
  servidores: { gestao: number; efetivos: number; act: number; pedagogico: number }
  salas: { total: number; ocupadas: number }
  equipamentos: { computadores: number; tablets: number; projetores: number; telas: number; outros: string }
  fotos_antes: Foto[]
  fotos_atual: Foto[]
  labs: { maker: LabFixo; informatica: LabFixo; outros: LabOutro[] }
  espacos: Espaco[]
  projetos: Projeto[]
  cursos: string[]
}

export function dadosVazios(): Dados {
  return {
    responsavel: '',
    foto_gestor: null,
    foto_fachada: null,
    endereco: '',
    contatos: {
      secretaria: { telefone: '', email: '' },
      direcao: { telefone: '', email: '' },
      pedagogico: { telefone: '', email: '' },
    },
    gps: { lat: 0, lng: 0, capturado: false },
    servidores: { gestao: 0, efetivos: 0, act: 0, pedagogico: 0 },
    salas: { total: 0, ocupadas: 0 },
    equipamentos: { computadores: 0, tablets: 0, projetores: 0, telas: 0, outros: '' },
    fotos_antes: [],
    fotos_atual: [],
    labs: {
      maker: { possui: false, descricao: '', fotos: [] },
      informatica: { possui: false, descricao: '', fotos: [] },
      outros: [],
    },
    espacos: [],
    projetos: [],
    cursos: [],
  }
}

export interface UsuarioResumo {
  id: string
  nome: string
  username: string
}

export interface Diagnostico {
  id: string
  escola_id: string
  status: 'rascunho' | 'enviado'
  dados: Dados
  criado_por: UsuarioResumo | null
  atualizado_por: UsuarioResumo | null
  criado_em: string
  atualizado_em: string
  enviado_em: string | null
  escola_nome: string
}

export interface ResponsavelResumo {
  id: string
  nome: string
  username: string
}

export interface EscolaResumo {
  id: string
  nome: string
  criado_em: string
  ultimo_diagnostico_id: string | null
  status: 'rascunho' | 'enviado' | null
  atualizado_em: string | null
  atualizado_por: string | null
  responsaveis?: ResponsavelResumo[]
}

export interface ListaDiag {
  id: string
  escola_id: string
  escola_nome: string
  status: string
  criado_em: string
  atualizado_em: string
  enviado_em: string | null
  criado_por: string | null
  atualizado_por: string | null
}

export class ErroAPI extends Error {
  pendencias?: string[]
  constructor(mensagem: string, pendencias?: string[]) {
    super(mensagem)
    this.pendencias = pendencias
  }
}

export async function api<T>(caminho: string, opcoes?: RequestInit): Promise<T> {
  const token = tokenAtual()
  const resposta = await fetch(caminho, {
    credentials: 'same-origin',
    ...opcoes,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(opcoes?.headers ?? {}),
    },
  })
  if (resposta.status === 401 && window.location.pathname !== '/login') {
    definirToken(null)
    window.location.href = '/login'
    throw new ErroAPI('Sessão expirada')
  }
  if (!resposta.ok) {
    const corpo = await resposta.json().catch(() => ({ erro: 'Erro inesperado' }))
    throw new ErroAPI(corpo.erro ?? 'Erro inesperado', corpo.pendencias)
  }
  return resposta.json()
}

export function uploadFoto(arquivo: File, escolaId: string): Promise<Foto> {
  return new Promise((ok, falhou) => {
    const fd = new FormData()
    fd.append('arquivo', arquivo)
    fd.append('escola_id', escolaId)
    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/api/fotos')
    const t = tokenAtual()
    if (t) xhr.setRequestHeader('Authorization', `Bearer ${t}`)
    xhr.onload = () => {
      if (xhr.status === 201 || xhr.status === 200) {
        ok(JSON.parse(xhr.responseText))
      } else {
        let msg = 'Falha no envio da foto'
        try {
          msg = JSON.parse(xhr.responseText).erro ?? msg
        } catch {}
        falhou(new ErroAPI(msg))
      }
    }
    xhr.onerror = () => falhou(new ErroAPI('Falha de rede no envio da foto'))
    xhr.send(fd)
  })
}

// Baixa um arquivo autenticado (ex.: relatórios) respeitando o token Bearer,
// e dispara o salvamento no navegador com o nome vindo do servidor.
export async function baixarArquivo(caminho: string, nomePadrao: string): Promise<void> {
  const token = tokenAtual()
  const resposta = await fetch(caminho, {
    credentials: 'same-origin',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (resposta.status === 401 && window.location.pathname !== '/login') {
    definirToken(null)
    window.location.href = '/login'
    throw new ErroAPI('Sessão expirada')
  }
  if (!resposta.ok) {
    const corpo = await resposta.json().catch(() => ({ erro: 'Erro inesperado' }))
    throw new ErroAPI(corpo.erro ?? 'Erro inesperado')
  }
  const blob = await resposta.blob()
  const disposicao = resposta.headers.get('Content-Disposition') ?? ''
  const m = /filename="?([^";]+)"?/i.exec(disposicao)
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = m?.[1] ?? nomePadrao
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 5000)
}

export function formatarData(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleString('pt-BR', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}
