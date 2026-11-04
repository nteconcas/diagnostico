import { ReactNode } from 'react'

// Campos reutilizados por todo o formulário.

export function Campo({
  rotulo,
  opcional,
  dica,
  children,
}: {
  rotulo: string
  opcional?: boolean
  dica?: string
  children: ReactNode
}) {
  return (
    <div>
      <label className="rotulo">
        {rotulo} {opcional && <span className="opcional">(opcional)</span>}
      </label>
      {dica && <p className="dica">{dica}</p>}
      {children}
    </div>
  )
}

export function CampoTexto(props: {
  rotulo: string
  valor: string
  onChange: (v: string) => void
  opcional?: boolean
  dica?: string
  placeholder?: string
  tipo?: string
}) {
  return (
    <Campo rotulo={props.rotulo} opcional={props.opcional} dica={props.dica}>
      <input
        type={props.tipo ?? 'text'}
        value={props.valor}
        placeholder={props.placeholder}
        onChange={(e) => props.onChange(e.target.value)}
      />
    </Campo>
  )
}

export function CampoNumero(props: {
  rotulo: string
  valor: number
  onChange: (v: number) => void
  dica?: string
  min?: number
}) {
  return (
    <Campo rotulo={props.rotulo} dica={props.dica}>
      <input
        type="number"
        min={props.min ?? 0}
        value={Number.isFinite(props.valor) ? props.valor : 0}
        onChange={(e) => props.onChange(Math.max(props.min ?? 0, parseInt(e.target.value || '0', 10) || 0))}
      />
    </Campo>
  )
}

export function Metrica({ valor, rotulo, verde }: { valor: string | number; rotulo: string; verde?: boolean }) {
  return (
    <div className={'metrica' + (verde ? ' verde' : '')}>
      <div className="valor">{valor}</div>
      <div className="rot">{rotulo}</div>
    </div>
  )
}

export function Bloco({
  id,
  numero,
  titulo,
  descricao,
  verde,
  children,
}: {
  id: string
  numero?: string
  titulo: string
  descricao?: string
  verde?: boolean
  children: ReactNode
}) {
  return (
    <section className="bloco" id={id}>
      <div className="cartao">
        <div className="bloco-cabeca">
          {numero && <div className={'bloco-numero' + (verde ? ' verde' : '')}>{numero}</div>}
          <div>
            <h3>{titulo}</h3>
            {descricao && <p>{descricao}</p>}
          </div>
        </div>
        <div className="bloco-corpo">{children}</div>
      </div>
    </section>
  )
}
