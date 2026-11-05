// Package model define as entidades e o formato dos dados do diagnóstico.
package model

import (
        "fmt"
        "strings"
        "time"
)

// Papel de um usuário do sistema.
type Papel string

const (
        PapelAdmin        Papel = "admin"
        PapelPreenchedor  Papel = "preenchador"
)

// Usuario é uma conta com acesso ao formulário. Se EscolaID estiver preenchido
// (papel preenchador), o usuário é o responsável pela escola e só enxerga/edita
// os diagnósticos dela.
type Usuario struct {
        ID        string    `json:"id"`
        Username  string    `json:"username"`
        SenhaHash string    `json:"-"`
        Nome      string    `json:"nome"`
        Papel     Papel     `json:"papel"`
        EscolaID  *string   `json:"escola_id,omitempty"`
        EscolaNome string   `json:"escola_nome,omitempty"`
        CriadoEm  time.Time `json:"criado_em"`
}

// UsuarioResumo aparece nos registros de quem criou/atualizou um diagnóstico.
type UsuarioResumo struct {
        ID       string `json:"id"`
        Nome     string `json:"nome"`
        Username string `json:"username"`
}

// Escola é uma unidade escolar da rede.
type Escola struct {
        ID       string    `json:"id"`
        Nome     string    `json:"nome"`
        CriadoEm time.Time `json:"criado_em"`
}

// Foto referenciada pelo diagnóstico (apenas id + nome; as URLs são derivadas).
type Foto struct {
        ID   string `json:"id"`
        Nome string `json:"nome"`
}

// ThumbURL aponta para a miniatura (grade da interface).
func (f Foto) ThumbURL() string {
        if f.ID == "" {
                return ""
        }
        return "/fotos/" + f.ID + ".t.jpg"
}

// MediaURL aponta para a versão média (relatórios e visualização).
func (f Foto) MediaURL() string {
        if f.ID == "" {
                return ""
        }
        return "/fotos/" + f.ID + ".m.jpg"
}

// Contato de um setor da escola.
type Contato struct {
        Telefone string `json:"telefone"`
        Email    string `json:"email"`
}

type Contatos struct {
        Secretaria  Contato `json:"secretaria"`
        Direcao     Contato `json:"direcao"`
        Pedagogico  Contato `json:"pedagogico"`
}

type GPS struct {
        Lat       float64 `json:"lat"`
        Lng       float64 `json:"lng"`
        Capturado bool    `json:"capturado"`
}

type Servidores struct {
        Gestao     int `json:"gestao"`
        Efetivos   int `json:"efetivos"`
        ACT        int `json:"act"`
        Pedagogico int `json:"pedagogico"`
}

type Salas struct {
        Total    int `json:"total"`
        Ocupadas int `json:"ocupadas"`
}

type Equipamentos struct {
        Computadores int    `json:"computadores"`
        Tablets      int    `json:"tablets"`
        Projetores   int    `json:"projetores"`
        Telas        int    `json:"telas"`
        Outros       string `json:"outros"`
}

// LabFixo é um laboratório pré-definido (maker ou informática).
type LabFixo struct {
        Possui    bool   `json:"possui"`
        Descricao string `json:"descricao"`
        Fotos     []Foto `json:"fotos"`
}

// LabOutro é um laboratório com nome livre.
type LabOutro struct {
        Nome      string `json:"nome"`
        Descricao string `json:"descricao"`
        Fotos     []Foto `json:"fotos"`
}

type Labs struct {
        Maker       LabFixo    `json:"maker"`
        Informatica LabFixo    `json:"informatica"`
        Outros      []LabOutro `json:"outros"`
}

type Espaco struct {
        Nome      string `json:"nome"`
        Descricao string `json:"descricao"`
        Fotos     []Foto `json:"fotos"`
}

type Projeto struct {
        Nome      string `json:"nome"`
        Descricao string `json:"descricao"`
        Fotos     []Foto `json:"fotos"`
}

// Obra é uma obra já realizada na unidade escolar. O registro fotográfico é
// feito em pares: as fotos de "antes" e de "depois" devem ser tiradas do mesmo
// ângulo e da mesma distância, para evidenciar a transformação do espaço.
type Obra struct {
        Nome        string `json:"nome"`
        Descricao   string `json:"descricao"`
        FotosAntes  []Foto `json:"fotos_antes"`
        FotosDepois []Foto `json:"fotos_depois"`
}

// EspacoRevitalizacao é um espaço da escola que ainda precisa de
// revitalização, registrado com descrição e fotos da situação atual.
type EspacoRevitalizacao struct {
        Nome      string `json:"nome"`
        Descricao string `json:"descricao"`
        Fotos     []Foto `json:"fotos"`
}

// Dados é o conteúdo completo do formulário de diagnóstico.
type Dados struct {
        Responsavel   string                `json:"responsavel"`
        FotoGestor    *Foto                 `json:"foto_gestor,omitempty"`
        FotoFachada   *Foto                 `json:"foto_fachada,omitempty"`
        Endereco      string                `json:"endereco"`
        Contatos      Contatos              `json:"contatos"`
        GPS           GPS                   `json:"gps"`
        Servidores    Servidores            `json:"servidores"`
        Salas         Salas                 `json:"salas"`
        Equipamentos  Equipamentos          `json:"equipamentos"`
        FotosAntes    []Foto                `json:"fotos_antes"`
        FotosAtual    []Foto                `json:"fotos_atual"`
        Labs          Labs                  `json:"labs"`
        Espacos       []Espaco              `json:"espacos"`
        Obras         []Obra                `json:"obras"`
        Revitalizacao []EspacoRevitalizacao `json:"revitalizacao"`
        Projetos      []Projeto             `json:"projetos"`
        Cursos        []string              `json:"cursos"`
}

func nonNeg(n int) int {
        if n < 0 {
                return 0
        }
        return n
}

func limiteFotos(fs []Foto, max int) []Foto {
        if len(fs) > max {
                fs = fs[:max]
        }
        out := make([]Foto, 0, len(fs))
        for _, f := range fs {
                if f.ID != "" {
                        out = append(out, f)
                }
        }
        return out
}

// Normalizar limpa e limita os dados recebidos do cliente.
func (d *Dados) Normalizar() {
        d.Responsavel = strings.TrimSpace(d.Responsavel)
        d.Endereco = strings.TrimSpace(d.Endereco)
        d.Servidores.Gestao = nonNeg(d.Servidores.Gestao)
        d.Servidores.Efetivos = nonNeg(d.Servidores.Efetivos)
        d.Servidores.ACT = nonNeg(d.Servidores.ACT)
        d.Servidores.Pedagogico = nonNeg(d.Servidores.Pedagogico)
        d.Salas.Total = nonNeg(d.Salas.Total)
        d.Salas.Ocupadas = nonNeg(d.Salas.Ocupadas)
        if d.Salas.Ocupadas > d.Salas.Total {
                d.Salas.Ocupadas = d.Salas.Total
        }
        d.Equipamentos.Computadores = nonNeg(d.Equipamentos.Computadores)
        d.Equipamentos.Tablets = nonNeg(d.Equipamentos.Tablets)
        d.Equipamentos.Projetores = nonNeg(d.Equipamentos.Projetores)
        d.Equipamentos.Telas = nonNeg(d.Equipamentos.Telas)
        d.Equipamentos.Outros = strings.TrimSpace(d.Equipamentos.Outros)
        if len(d.FotosAntes) > 6 {
                d.FotosAntes = d.FotosAntes[:6]
        }
        if len(d.FotosAtual) > 6 {
                d.FotosAtual = d.FotosAtual[:6]
        }
        d.FotosAntes = limiteFotos(d.FotosAntes, 6)
        d.FotosAtual = limiteFotos(d.FotosAtual, 6)
        if d.FotoGestor != nil && d.FotoGestor.ID == "" {
                d.FotoGestor = nil
        }
        if d.FotoFachada != nil && d.FotoFachada.ID == "" {
                d.FotoFachada = nil
        }
        for _, c := range []*Contato{&d.Contatos.Secretaria, &d.Contatos.Direcao, &d.Contatos.Pedagogico} {
                c.Telefone = strings.TrimSpace(c.Telefone)
                c.Email = strings.TrimSpace(c.Email)
        }
        labs := []LabOutro{}
        for _, lo := range d.Labs.Outros {
                lo.Nome = strings.TrimSpace(lo.Nome)
                lo.Descricao = strings.TrimSpace(lo.Descricao)
                lo.Fotos = limiteFotos(lo.Fotos, 6)
                if lo.Nome == "" && len(lo.Fotos) == 0 {
                        continue
                }
                labs = append(labs, lo)
        }
        d.Labs.Outros = labs
        d.Labs.Maker.Descricao = strings.TrimSpace(d.Labs.Maker.Descricao)
        d.Labs.Maker.Fotos = limiteFotos(d.Labs.Maker.Fotos, 6)
        d.Labs.Informatica.Descricao = strings.TrimSpace(d.Labs.Informatica.Descricao)
        d.Labs.Informatica.Fotos = limiteFotos(d.Labs.Informatica.Fotos, 6)
        espacos := []Espaco{}
        vistos := map[string]bool{}
        for _, e := range d.Espacos {
                e.Nome = strings.TrimSpace(e.Nome)
                e.Descricao = strings.TrimSpace(e.Descricao)
                if e.Nome == "" || vistos[strings.ToLower(e.Nome)] {
                        continue
                }
                vistos[strings.ToLower(e.Nome)] = true
                e.Fotos = limiteFotos(e.Fotos, 3)
                espacos = append(espacos, e)
        }
        d.Espacos = espacos
        // Obras realizadas: descrição + pares de fotos antes/depois.
        obras := []Obra{}
        for _, o := range d.Obras {
                o.Nome = strings.TrimSpace(o.Nome)
                o.Descricao = strings.TrimSpace(o.Descricao)
                if len(o.Descricao) > 2500 {
                        o.Descricao = o.Descricao[:2500]
                }
                o.FotosAntes = limiteFotos(o.FotosAntes, 4)
                o.FotosDepois = limiteFotos(o.FotosDepois, 4)
                if o.Nome == "" && o.Descricao == "" && len(o.FotosAntes) == 0 && len(o.FotosDepois) == 0 {
                        continue
                }
                obras = append(obras, o)
        }
        d.Obras = obras
        // Espaços que precisam de revitalização: nome, descrição e fotos.
        revitalizacao := []EspacoRevitalizacao{}
        vistosRevitalizacao := map[string]bool{}
        for _, r := range d.Revitalizacao {
                r.Nome = strings.TrimSpace(r.Nome)
                r.Descricao = strings.TrimSpace(r.Descricao)
                if len(r.Descricao) > 2500 {
                        r.Descricao = r.Descricao[:2500]
                }
                r.Fotos = limiteFotos(r.Fotos, 4)
                if r.Nome == "" && r.Descricao == "" && len(r.Fotos) == 0 {
                        continue
                }
                if chave := strings.ToLower(r.Nome); chave != "" {
                        if vistosRevitalizacao[chave] {
                                continue
                        }
                        vistosRevitalizacao[chave] = true
                }
                revitalizacao = append(revitalizacao, r)
        }
        d.Revitalizacao = revitalizacao
        projetos := []Projeto{}
        for _, p := range d.Projetos {
                p.Nome = strings.TrimSpace(p.Nome)
                p.Descricao = strings.TrimSpace(p.Descricao)
                if len(p.Descricao) > 2500 {
                        p.Descricao = p.Descricao[:2500]
                }
                p.Fotos = limiteFotos(p.Fotos, 3)
                if p.Nome == "" && p.Descricao == "" && len(p.Fotos) == 0 {
                        continue
                }
                projetos = append(projetos, p)
        }
        d.Projetos = projetos
        cursos := []string{}
        vistosCurso := map[string]bool{}
        for _, c := range d.Cursos {
                c = strings.TrimSpace(c)
                if c == "" || vistosCurso[strings.ToLower(c)] {
                        continue
                }
                vistosCurso[strings.ToLower(c)] = true
                cursos = append(cursos, c)
        }
        d.Cursos = cursos
}

// TotalServidores soma gestão, pedagógico e docentes.
func (d Dados) TotalServidores() int {
        s := d.Servidores
        return s.Gestao + s.Efetivos + s.ACT + s.Pedagogico
}

func pct(part, todo int) float64 {
        if todo <= 0 {
                return 0
        }
        return float64(part) / float64(todo) * 100
}

// PctEfetivos é a fatia de professores efetivos entre os docentes.
func (d Dados) PctEfetivos() float64 {
        return pct(d.Servidores.Efetivos, d.Servidores.Efetivos+d.Servidores.ACT)
}

// PctACT é a fatia de professores temporários entre os docentes.
func (d Dados) PctACT() float64 {
        return pct(d.Servidores.ACT, d.Servidores.Efetivos+d.Servidores.ACT)
}

// OcupacaoPct é a razão entre salas ocupadas e o total.
func (d Dados) OcupacaoPct() float64 {
        return pct(d.Salas.Ocupadas, d.Salas.Total)
}

// TotalEquipamentos soma os equipamentos tecnológicos quantificados.
func (d Dados) TotalEquipamentos() int {
        e := d.Equipamentos
        return e.Computadores + e.Tablets + e.Projetores + e.Telas
}

// GPSLink devolve o link do mapa para as coordenadas capturadas.
func (d Dados) GPSLink() string {
        if !d.GPS.Capturado {
                return ""
        }
        return fmt.Sprintf("https://www.google.com/maps?q=%.6f,%.6f", d.GPS.Lat, d.GPS.Lng)
}

// Pendencias lista os campos obrigatórios ainda não preenchidos.
func (d Dados) Pendencias() []string {
        p := []string{}
        if d.FotoFachada == nil {
                p = append(p, "Foto da fachada da escola")
        }
        if d.Endereco == "" {
                p = append(p, "Endereço da escola")
        }
        if len(d.FotosAntes) < 3 {
                p = append(p, fmt.Sprintf("Fotos antes da gestão 2023/2027 — mínimo de 3 (enviadas: %d)", len(d.FotosAntes)))
        }
        if len(d.FotosAtual) < 3 {
                p = append(p, fmt.Sprintf("Fotos da gestão atual 2023/2027 — mínimo de 3 (enviadas: %d)", len(d.FotosAtual)))
        }
        return p
}

// Diagnostico é um preenchimento (rascunho ou enviado) para uma escola.
type Diagnostico struct {
        ID           string         `json:"id"`
        EscolaID     string         `json:"escola_id"`
        Status       string         `json:"status"` // rascunho | enviado
        Dados        Dados          `json:"dados"`
        CriadoPor    *UsuarioResumo `json:"criado_por"`
        AtualizadoPor *UsuarioResumo `json:"atualizado_por"`
        CriadoEm     time.Time      `json:"criado_em"`
        AtualizadoEm time.Time      `json:"atualizado_em"`
        EnviadoEm    *time.Time     `json:"enviado_em"`
        // Preenchidos nas consultas com JOIN:
        EscolaNome string `json:"escola_nome"`
}
