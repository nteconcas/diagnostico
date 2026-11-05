package report

import (
        "bytes"
        "fmt"
        "image"
        _ "image/jpeg"
        "os"
        "strconv"
        "strings"

        "github.com/go-pdf/fpdf"
)

type pdfDoc struct {
        f  *fpdf.Fpdf
        tr func(string) string
}

var (
        corAzul   = []int{29, 78, 216}
        corAzulE  = []int{30, 58, 138}
        corVerde  = []int{21, 128, 61}
        corTinta  = []int{15, 23, 42}
        corCinza  = []int{71, 85, 105}
        corLinha  = []int{226, 232, 240}
        corFundo  = []int{248, 250, 252}
        corAzulCl = []int{219, 234, 254}
)

func novoPDF() *pdfDoc {
        f := fpdf.New("P", "mm", "A4", "")
        f.SetMargins(15, 16, 15)
        f.SetAutoPageBreak(true, 20)
        d := &pdfDoc{f: f, tr: f.UnicodeTranslatorFromDescriptor("")}
        f.SetFooterFunc(func() {
                f.SetY(-14)
                f.SetFont("Helvetica", "", 8)
                f.SetTextColor(148, 163, 184)
                f.CellFormat(0, 8, d.tr("Formulário de Diagnóstico Escolar · Uso institucional da rede educacional"), "", 0, "C", false, 0, "")
        })
        return d
}

func (p *pdfDoc) faixaTitulo() {
        p.f.SetFillColor(corAzul[0], corAzul[1], corAzul[2])
        p.f.Rect(0, 0, 210, 30, "F")
        p.f.SetFillColor(corVerde[0], corVerde[1], corVerde[2])
        p.f.Rect(0, 30, 210, 2.6, "F")
        p.f.SetTextColor(255, 255, 255)
        p.f.SetFont("Helvetica", "B", 17)
        p.f.SetXY(15, 9)
        p.f.CellFormat(180, 8, p.tr("FORMULÁRIO DE DIAGNÓSTICO ESCOLAR"), "", 0, "L", false, 0, "")
        p.f.SetFont("Helvetica", "", 8.5)
        p.f.SetXY(15, 19)
        p.f.CellFormat(180, 5, p.tr("Servidores · Infraestrutura · Obras · Espaços · Projetos · Ofertas de cursos"), "", 0, "L", false, 0, "")
}

func (p *pdfDoc) secao(sub, titulo string) {
        if p.f.GetY() > 232 {
                p.f.AddPage()
        }
        p.f.Ln(1)
        y := p.f.GetY()
        p.f.SetFillColor(corAzul[0], corAzul[1], corAzul[2])
        p.f.Rect(15, y+0.6, 3.2, 7.6, "F")
        p.f.SetXY(21, y)
        p.f.SetFont("Helvetica", "B", 13)
        p.f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
        p.f.CellFormat(174, 7.6, p.tr(titulo), "", 1, "L", false, 0, "")
        if sub != "" {
                p.f.SetX(21)
                p.f.SetFont("Helvetica", "B", 7.5)
                p.f.SetTextColor(corVerde[0], corVerde[1], corVerde[2])
                p.f.CellFormat(174, 3.8, p.tr(strings.ToUpper(sub)), "", 1, "L", false, 0, "")
        }
        p.f.Ln(1.4)
}

func (p *pdfDoc) kv(rotulo, valor string) {
        if valor == "" {
                return
        }
        p.f.SetFont("Helvetica", "B", 9.5)
        p.f.SetTextColor(corCinza[0], corCinza[1], corCinza[2])
        p.f.SetX(15)
        p.f.CellFormat(52, 5.6, p.tr(rotulo), "", 0, "L", false, 0, "")
        p.f.SetFont("Helvetica", "", 9.5)
        p.f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
        p.f.MultiCell(128, 5.6, p.tr(valor), "", "L", false)
        p.f.Ln(0.6)
}

func (p *pdfDoc) tabela(larguras []float64, cab []string, linhas [][]string) {
        alt := 7.0
        cabecalho := func() {
                p.f.SetFont("Helvetica", "B", 9.5)
                p.f.SetFillColor(corAzulCl[0], corAzulCl[1], corAzulCl[2])
                p.f.SetTextColor(corAzulE[0], corAzulE[1], corAzulE[2])
                p.f.SetDrawColor(corLinha[0], corLinha[1], corLinha[2])
                p.f.SetX(15)
                for i, h := range cab {
                        p.f.CellFormat(larguras[i], alt, p.tr(h), "1", 0, "L", true, 0, "")
                }
                p.f.Ln(-1)
        }
        cabecalho()
        p.f.SetFont("Helvetica", "", 9.5)
        p.f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
        for r, linha := range linhas {
                if p.f.GetY() > 268 {
                        p.f.AddPage()
                        cabecalho()
                        p.f.SetFont("Helvetica", "", 9.5)
                        p.f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
                }
                fill := r%2 == 1
                if fill {
                        p.f.SetFillColor(corFundo[0], corFundo[1], corFundo[2])
                }
                p.f.SetX(15)
                for i, c := range linha {
                        p.f.CellFormat(larguras[i], alt, p.tr(c), "1", 0, "L", fill, 0, "")
                }
                p.f.Ln(-1)
        }
        p.f.Ln(2.5)
}

func (p *pdfDoc) barraDupla(pctA, pctB float64, legA, legB string) {
        x, y := 15.0, p.f.GetY()+1
        w, h := 180.0, 5.2
        p.f.SetFillColor(corLinha[0], corLinha[1], corLinha[2])
        p.f.Rect(x, y, w, h, "F")
        if pctA > 0 {
                p.f.SetFillColor(corAzul[0], corAzul[1], corAzul[2])
                p.f.Rect(x, y, w*pctA/100, h, "F")
        }
        if pctB > 0 {
                p.f.SetFillColor(corVerde[0], corVerde[1], corVerde[2])
                p.f.Rect(x+w*pctA/100, y, w*pctB/100, h, "F")
        }
        p.f.SetY(y + h + 1.5)
        p.f.SetFont("Helvetica", "", 8.5)
        p.f.SetTextColor(corCinza[0], corCinza[1], corCinza[2])
        p.f.CellFormat(0, 5, p.tr(fmt.Sprintf("%.0f%% %s · %.0f%% %s", pctA, legA, pctB, legB)), "", 1, "L", false, 0, "")
}

func decodeConfig(caminho string) (int, int, error) {
        f, err := os.Open(caminho)
        if err != nil {
                return 0, 0, err
        }
        defer f.Close()
        cfg, _, err := image.DecodeConfig(f)
        if err != nil {
                return 0, 0, err
        }
        return cfg.Width, cfg.Height, nil
}

func (p *pdfDoc) imagemAjustada(caminho string, x, y, maxW, maxH float64) float64 {
        wPx, hPx, err := decodeConfig(caminho)
        if err != nil || wPx <= 0 {
                return 0
        }
        ratio := float64(hPx) / float64(wPx)
        w := maxW
        h := w * ratio
        if h > maxH {
                h = maxH
                w = h / ratio
        }
        f, err := os.Open(caminho)
        if err != nil {
                return 0
        }
        defer f.Close()
        p.f.RegisterImageOptionsReader(caminho, fpdf.ImageOptions{ImageType: "jpg"}, f)
        p.f.Image(caminho, x+(maxW-w)/2, y, w, h, false, "jpg", 0, "")
        return h
}

func (p *pdfDoc) galeria(fotos []FotoRep, titulo string) {
        if len(fotos) == 0 {
                return
        }
        if titulo != "" {
                if p.f.GetY() > 262 {
                        p.f.AddPage()
                }
                p.f.SetFont("Helvetica", "B", 9.5)
                p.f.SetTextColor(corCinza[0], corCinza[1], corCinza[2])
                p.f.SetX(15)
                p.f.CellFormat(0, 6, p.tr(titulo), "", 1, "L", false, 0, "")
        }
        celW, celH, gap := 84.0, 62.0, 6.0
        col := 0
        y := p.f.GetY()
        for _, foto := range fotos {
                if col == 0 {
                        if y+celH+10 > 272 {
                                p.f.AddPage()
                                y = p.f.GetY()
                        } else {
                                p.f.SetY(y)
                        }
                }
                x := 15.0 + float64(col)*(celW+gap)
                h := p.imagemAjustada(foto.Caminho, x, y, celW, celH-7)
                p.f.SetXY(x, y+h+1)
                p.f.SetFont("Helvetica", "", 7.5)
                p.f.SetTextColor(100, 116, 139)
                p.f.CellFormat(celW, 4, p.tr(foto.Legenda), "", 0, "C", false, 0, "")
                col++
                if col == 2 {
                        col = 0
                        y += celH + 6
                }
        }
        if col != 0 {
                y += celH + 6
        }
        p.f.SetY(y)
}

func inteiro(n int) string { return strconv.Itoa(n) }

// RenderPDF gera o relatório em PDF.
func RenderPDF(re Reporte) ([]byte, error) {
        p := novoPDF()
        f := p.f

        // ---- capa ----
        f.AddPage()
        p.faixaTitulo()
        f.SetTextColor(corAzulE[0], corAzulE[1], corAzulE[2])
        f.SetFont("Helvetica", "B", 19)
        f.SetXY(15, 42)
        f.CellFormat(180, 9, p.tr(re.EscolaNome), "", 0, "C", false, 0, "")
        f.SetFont("Helvetica", "", 10.5)
        f.SetTextColor(corCinza[0], corCinza[1], corCinza[2])
        f.SetXY(15, 52)
        sub := ""
        if re.Responsavel != "" {
                sub = p.tr("Responsável pelo preenchimento: " + re.Responsavel)
        }
        f.CellFormat(180, 6, sub, "", 0, "C", false, 0, "")
        if re.Fachada != nil {
                h := p.imagemAjustada(re.Fachada.Caminho, 30, 64, 150, 105)
                f.SetY(64 + h + 6)
        } else {
                f.SetY(180)
        }
        f.SetFont("Helvetica", "", 10)
        f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
        f.SetX(15)
        f.CellFormat(180, 6, p.tr(re.Endereco), "", 0, "C", false, 0, "")
        f.Ln(8)
        f.SetFont("Helvetica", "", 9.5)
        f.SetTextColor(corCinza[0], corCinza[1], corCinza[2])
        if re.Status == "enviado" {
                f.SetX(15)
                f.CellFormat(180, 5.5, p.tr("Enviado em "+re.EnviadoEm), "", 1, "C", false, 0, "")
        }
        f.SetX(15)
        f.CellFormat(180, 5.5, p.tr("Relatório gerado em "+re.GeradoEm+" por "+re.GeradoPor), "", 1, "C", false, 0, "")

        // ---- identificação e contatos ----
        f.AddPage()
        p.faixaTitulo()
        f.SetY(40)
        p.secao("Identificação e localização", "Endereço, contatos e GPS")
        p.kv("Endereço", re.Endereco)
        if re.GPS.Capturado {
                p.kv("Localização GPS", fmt.Sprintf("%.6f, %.6f", re.GPS.Lat, re.GPS.Lng))
                p.kv("Link do mapa", re.GPSLink)
        }
        if re.GestorFoto != nil {
                y := f.GetY()
                h := p.imagemAjustada(re.GestorFoto.Caminho, 15, y, 26, 26)
                f.SetXY(45, y)
                f.SetFont("Helvetica", "", 9.5)
                f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
                f.CellFormat(100, 6, p.tr("Foto do(a) gestor(a)"), "", 1, "L", false, 0, "")
                f.SetY(y + h + 3)
        }
        if len(re.Contatos) > 0 {
                linhas := [][]string{}
                for _, c := range re.Contatos {
                        linhas = append(linhas, []string{c.Setor, c.Telefone, c.Email})
                }
                p.tabela([]float64{40, 55, 85}, []string{"Setor", "Telefone", "E-mail"}, linhas)
        }

        // ---- servidores ----
        p.secao("Seção 1", "Servidores")
        linhas := [][]string{
                {"Servidores na gestão (direção, vice-direção, secretaria e afins)", inteiro(re.Servidores.Gestao)},
                {"Servidores no pedagógico (coordenação, supervisão, orientação e apoio)", inteiro(re.Servidores.Pedagogico)},
                {"Professores efetivos (concurso / quadro permanente)", inteiro(re.Servidores.Efetivos)},
                {"Professores ACT (admitidos em caráter temporário)", inteiro(re.Servidores.ACT)},
                {"Total de servidores", inteiro(re.TotalServ)},
        }
        p.tabela([]float64{140, 40}, []string{"Categoria", "Quantidade"}, linhas)
        p.barraDupla(re.PctEfetivos, re.PctACT, "efetivos ("+inteiro(re.Servidores.Efetivos)+")", "ACT ("+inteiro(re.Servidores.ACT)+")")

        // ---- infraestrutura ----
        p.secao("Seção 2", "Infraestrutura")
        p.kv("Salas de aula", fmt.Sprintf("%s no total · %s ocupadas (%.0f%% de ocupação)", inteiro(re.Salas.Total), inteiro(re.Salas.Ocupadas), re.OcupacaoPct))
        p.kv("Equipamentos tecnológicos", fmt.Sprintf("Total: %s — %s computadores, %s tablets, %s projetores, %s telas interativas",
                inteiro(re.TotalEquip), inteiro(re.Equip.Computadores), inteiro(re.Equip.Tablets), inteiro(re.Equip.Projetores), inteiro(re.Equip.Telas)))
        p.kv("Outros equipamentos", re.Equip.Outros)
        f.Ln(1)
        for _, lab := range re.LabsFixos {
                p.blocoLab(lab)
        }
        for _, lab := range re.LabsOutros {
                p.blocoLab(lab)
        }
        p.galeria(re.FotosAntes, "Registro fotográfico — antes da gestão 2023/2027")
        p.galeria(re.FotosAtual, "Registro fotográfico — gestão atual 2023/2027")

        // ---- obras realizadas ----
        if len(re.Obras) > 0 {
                p.secao("Seção 3", "Obras realizadas")
                f.SetFont("Helvetica", "", 9)
                f.SetTextColor(corCinza[0], corCinza[1], corCinza[2])
                f.SetX(15)
                f.MultiCell(180, 4.8, p.tr("Cada obra é registrada com fotos de antes e depois tiradas do mesmo ângulo e da mesma distância."), "", "L", false)
                f.Ln(1.6)
                for _, o := range re.Obras {
                        if f.GetY() > 240 {
                                f.AddPage()
                        }
                        f.SetFont("Helvetica", "B", 11)
                        f.SetTextColor(corVerde[0], corVerde[1], corVerde[2])
                        f.SetX(15)
                        f.CellFormat(0, 6.5, p.tr(o.Nome), "", 1, "L", false, 0, "")
                        if o.Descricao != "" {
                                f.SetFont("Helvetica", "", 9.5)
                                f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
                                f.SetX(15)
                                f.MultiCell(180, 5.2, p.tr(o.Descricao), "", "L", false)
                        }
                        p.galeria(o.Antes, "Antes da obra")
                        p.galeria(o.Depois, "Depois da obra")
                        f.Ln(2)
                }
        }

        // ---- espaços que precisam de revitalização ----
        if len(re.Revitalizacao) > 0 {
                p.secao("Seção 4", "Espaços que precisam de revitalização")
                for _, r := range re.Revitalizacao {
                        if f.GetY() > 240 {
                                f.AddPage()
                        }
                        f.SetFont("Helvetica", "B", 11)
                        f.SetTextColor(corVerde[0], corVerde[1], corVerde[2])
                        f.SetX(15)
                        f.CellFormat(0, 6.5, p.tr(r.Nome), "", 1, "L", false, 0, "")
                        if r.Descricao != "" {
                                f.SetFont("Helvetica", "", 9.5)
                                f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
                                f.SetX(15)
                                f.MultiCell(180, 5.2, p.tr(r.Descricao), "", "L", false)
                        }
                        p.galeria(r.Fotos, "")
                        f.Ln(2)
                }
        }

        // ---- espaços ----
        if len(re.Espacos) > 0 {
                p.secao("Seção 5", "Espaços da escola")
                for _, e := range re.Espacos {
                        p.f.SetFont("Helvetica", "B", 10.5)
                        p.f.SetTextColor(corVerde[0], corVerde[1], corVerde[2])
                        p.f.SetX(15)
                        p.f.CellFormat(0, 6, p.tr(e.Nome), "", 1, "L", false, 0, "")
                        if e.Descricao != "" {
                                p.f.SetFont("Helvetica", "", 9.5)
                                p.f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
                                p.f.SetX(15)
                                p.f.MultiCell(180, 5.2, p.tr(e.Descricao), "", "L", false)
                        }
                        p.galeria(e.Fotos, "")
                }
        }

        // ---- projetos ----
        if len(re.Projetos) > 0 {
                p.secao("Seção 6", "Projetos")
                for _, pr := range re.Projetos {
                        if f.GetY() > 240 {
                                f.AddPage()
                        }
                        f.SetFont("Helvetica", "B", 11)
                        f.SetTextColor(corVerde[0], corVerde[1], corVerde[2])
                        f.SetX(15)
                        f.CellFormat(0, 6.5, p.tr(pr.Nome), "", 1, "L", false, 0, "")
                        if pr.Descricao != "" {
                                f.SetFont("Helvetica", "", 9.5)
                                f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
                                f.SetX(15)
                                f.MultiCell(180, 5.2, p.tr(pr.Descricao), "", "L", false)
                        }
                        p.galeria(pr.Fotos, "")
                        f.Ln(2)
                }
        }

        // ---- cursos ----
        if len(re.Cursos) > 0 {
                p.secao("Seção 7", "Ofertas de cursos")
                f.SetFont("Helvetica", "", 9.5)
                f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
                for _, c := range re.Cursos {
                        f.SetX(18)
                        f.CellFormat(0, 5.8, p.tr("•  "+c), "", 1, "L", false, 0, "")
                }
        }

        var buf bytes.Buffer
        if err := f.Output(&buf); err != nil {
                return nil, err
        }
        return buf.Bytes(), nil
}

func (p *pdfDoc) blocoLab(lab LabRep) {
        f := p.f
        if f.GetY() > 250 {
                f.AddPage()
        }
        f.SetFont("Helvetica", "B", 10.5)
        f.SetTextColor(corVerde[0], corVerde[1], corVerde[2])
        f.SetX(15)
        f.CellFormat(0, 6, p.tr(lab.Nome), "", 1, "L", false, 0, "")
        if lab.Descricao != "" {
                f.SetFont("Helvetica", "", 9.5)
                f.SetTextColor(corTinta[0], corTinta[1], corTinta[2])
                f.SetX(15)
                f.MultiCell(180, 5.2, p.tr(lab.Descricao), "", "L", false)
        }
        p.galeria(lab.Fotos, "")
        f.Ln(1.5)
}
