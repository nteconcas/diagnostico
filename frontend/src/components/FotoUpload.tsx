import { useRef, useState } from 'react'
import { Foto, fotoThumb, uploadFoto } from '../api'

interface Props {
  fotos: Foto[]
  max: number
  min?: number
  onChange: (fotos: Foto[]) => void
  textoBotao?: string
}

// FotoUpload envia as imagens imediatamente e devolve os metadados para o rascunho.
export function FotoUpload({ fotos, max, min, onChange, textoBotao = 'Adicionar' }: Props) {
  const refInput = useRef<HTMLInputElement>(null)
  const [enviando, setEnviando] = useState(0)
  const [erro, setErro] = useState('')

  async function escolher(arquivos: FileList | null) {
    if (!arquivos || arquivos.length === 0) return
    setErro('')
    setEnviando(arquivos.length)
    const novas: Foto[] = []
    for (const arquivo of Array.from(arquivos)) {
      if (fotos.length + novas.length >= max) break
      try {
        novas.push(await uploadFoto(arquivo))
      } catch (e) {
        setErro((e as Error).message)
      }
    }
    setEnviando(0)
    if (novas.length > 0) onChange([...fotos, ...novas].slice(0, max))
    if (refInput.current) refInput.current.value = ''
  }

  function remover(id: string) {
    onChange(fotos.filter((f) => f.id !== id))
  }

  const cheio = fotos.length >= max
  const completo = min ? fotos.length >= min : true

  return (
    <div>
      <div className="upload-grade">
        {fotos.map((f) => (
          <div className="upload-foto" key={f.id}>
            <img src={fotoThumb(f)} alt={f.nome || 'Foto enviada'} />
            <button
              type="button"
              className="remover"
              title="Remover foto"
              onClick={() => remover(f.id)}
            >
              ✕
            </button>
          </div>
        ))}
        {enviando > 0 && (
          <div className="upload-foto" title={`Enviando ${enviando} foto(s)…`}>
            <div
              style={{
                width: 104, height: 78, borderRadius: 10,
                border: '1.5px dashed var(--cinza-fraco)',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                color: 'var(--cinza)', fontSize: 12,
              }}
            >
              enviando…
            </div>
          </div>
        )}
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginTop: 10, flexWrap: 'wrap' }}>
        <input
          ref={refInput}
          type="file"
          accept="image/*"
          multiple
          hidden
          onChange={(e) => escolher(e.target.files)}
        />
        <button
          type="button"
          className="btn-suave btn-mini"
          disabled={cheio || enviando > 0}
          onClick={() => refInput.current?.click()}
        >
          ＋ {textoBotao}
        </button>
        <span className="upload-contador">
          {fotos.length}/{max}
          {min ? (
            completo ? (
              <span className="ok">· mínimo atingido</span>
            ) : (
              <span className="faltando">· mínimo {min}</span>
            )
          ) : null}
        </span>
        {erro && <span className="upload-erro">{erro}</span>}
      </div>
    </div>
  )
}
