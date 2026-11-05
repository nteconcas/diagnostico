// Package report monta e renderiza o relatório de diagnóstico em HTML, PDF e DOCX.
package report

import (
        "fmt"
        "time"

        "diagnostico-escolar/internal/model"
)

// Entrada reúne o que é preciso para montar o relatório.
type Entrada struct {
        Diag      model.Diagnostico
        GeradoPor string
}

type FotoRep struct {
        Caminho string // caminho no disco (leitura server-side)
        URL     string // URL pública (HTML usa data-URI, mas fica para referência)
        Legenda string
}

type ContatoRep struct {
        Setor    string
        Telefone string
        Email    string
}

type LabRep struct {
        Nome      string
        Descricao string
        Fotos     []FotoRep
}

type EspacoRep struct {
        Nome  string
        Fotos []FotoRep
}

type ProjetoRep struct {
        Nome      string
        Descricao string
        Fotos     []FotoRep
}

// Reporte é o modelo pronto para os três formatos de saída.
type Reporte struct {
        EscolaNome  string
        Status      string
        Responsavel string
        Fachada     *FotoRep
        GestorFoto  *FotoRep
        Endereco    string
        Contatos    []ContatoRep
        GPS         model.GPS
        GPSLink     string

        Servidores    model.Servidores
        TotalServ     int
        PctEfetivos   float64
        PctACT        float64
        Salas         model.Salas
        OcupacaoPct   float64
        Equip         model.Equipamentos
        TotalEquip    int
        FotosAntes    []FotoRep
        FotosAtual    []FotoRep
        LabsFixos     []LabRep
        LabsOutros    []LabRep
        Espacos       []EspacoRep
        Projetos      []ProjetoRep
        Cursos        []string
        EnviadoEm     string
        AtualizadoEm  string
        GeradoEm      string
        GeradoPor     string
}

func paraFotos(fs []model.Foto) []FotoRep {
        out := make([]FotoRep, 0, len(fs))
        for i, f := range fs {
                out = append(out, FotoRep{
                        Caminho: caminhoFoto(f.ID),
                        URL:     f.MediaURL(),
                        Legenda: fmt.Sprintf("Foto %d", i+1),
                })
        }
        return out
}

func paraFotosComLegenda(fs []model.Foto, prefixo string) []FotoRep {
        out := make([]FotoRep, 0, len(fs))
        for i, f := range fs {
                leg := f.Nome
                if leg == "" {
                        leg = fmt.Sprintf("%s — foto %d", prefixo, i+1)
                }
                out = append(out, FotoRep{Caminho: caminhoFoto(f.ID), URL: f.MediaURL(), Legenda: leg})
        }
        return out
}

// Montar converte o diagnóstico em Reporte.
func Montar(e Entrada) Reporte {
        d := e.Diag.Dados
        re := Reporte{
                EscolaNome:  e.Diag.EscolaNome,
                Status:      e.Diag.Status,
                Responsavel: d.Responsavel,
                Endereco:    d.Endereco,
                GPS:         d.GPS,
                GPSLink:     d.GPSLink(),
                Servidores:  d.Servidores,
                TotalServ:   d.TotalServidores(),
                PctEfetivos: d.PctEfetivos(),
                PctACT:      d.PctACT(),
                Salas:       d.Salas,
                OcupacaoPct: d.OcupacaoPct(),
                Equip:       d.Equipamentos,
                TotalEquip:  d.TotalEquipamentos(),
                FotosAntes:  paraFotosComLegenda(d.FotosAntes, "Antes da gestão"),
                FotosAtual:  paraFotosComLegenda(d.FotosAtual, "Gestão atual"),
                Cursos:      d.Cursos,
                GeradoEm:    time.Now().Format("02/01/2006 15:04"),
                GeradoPor:   e.GeradoPor,
        }
        if d.FotoFachada != nil {
                re.Fachada = &FotoRep{Caminho: caminhoFoto(d.FotoFachada.ID), URL: d.FotoFachada.MediaURL(), Legenda: "Fachada da escola"}
        }
        if d.FotoGestor != nil {
                re.GestorFoto = &FotoRep{Caminho: caminhoFoto(d.FotoGestor.ID), URL: d.FotoGestor.MediaURL(), Legenda: "Gestor(a)"}
        }
        if t := d.Contatos.Secretaria; t.Telefone != "" || t.Email != "" {
                re.Contatos = append(re.Contatos, ContatoRep{"Secretaria", t.Telefone, t.Email})
        }
        if t := d.Contatos.Direcao; t.Telefone != "" || t.Email != "" {
                re.Contatos = append(re.Contatos, ContatoRep{"Direção", t.Telefone, t.Email})
        }
        if t := d.Contatos.Pedagogico; t.Telefone != "" || t.Email != "" {
                re.Contatos = append(re.Contatos, ContatoRep{"Pedagógico", t.Telefone, t.Email})
        }
        if d.Labs.Maker.Possui {
                re.LabsFixos = append(re.LabsFixos, LabRep{"Laboratório maker", d.Labs.Maker.Descricao, paraFotos(d.Labs.Maker.Fotos)})
        }
        if d.Labs.Informatica.Possui {
                re.LabsFixos = append(re.LabsFixos, LabRep{"Laboratório de informática", d.Labs.Informatica.Descricao, paraFotos(d.Labs.Informatica.Fotos)})
        }
        for _, lo := range d.Labs.Outros {
                re.LabsOutros = append(re.LabsOutros, LabRep{lo.Nome, lo.Descricao, paraFotos(lo.Fotos)})
        }
        for _, esp := range d.Espacos {
                re.Espacos = append(re.Espacos, EspacoRep{esp.Nome, paraFotos(esp.Fotos)})
        }
        for _, p := range d.Projetos {
                re.Projetos = append(re.Projetos, ProjetoRep{p.Nome, p.Descricao, paraFotos(p.Fotos)})
        }
        if e.Diag.EnviadoEm != nil {
                re.EnviadoEm = e.Diag.EnviadoEm.Format("02/01/2006 15:04")
        }
        re.AtualizadoEm = e.Diag.AtualizadoEm.Format("02/01/2006 15:04")
        return re
}
