package report

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"os"
	"strings"
)

// RenderDOCX gera o relatório como .docx (OOXML mínimo, sem dependências).
func RenderDOCX(re Reporte) ([]byte, error) {
	d := &docxBuilder{}
	d.montar(re)
	return d.zip()
}

type docxMedia struct {
	rid     string
	nome    string
	caminho string
}

type docxBuilder struct {
	media []docxMedia
	corpo strings.Builder
}

func esc(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\n':
			b.WriteString(" ")
		default:
			if r >= 0x20 || r == 0x09 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func (d *docxBuilder) run(rpr, texto string) string {
	return fmt.Sprintf(`<w:r><w:rPr>%s</w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, rpr, esc(texto))
}

func (d *docxBuilder) pPPr(jc string, depois int) string {
	return fmt.Sprintf(`<w:pPr><w:spacing w:after="%d"/>%s</w:pPr>`, depois, func() string {
		if jc != "" {
			return fmt.Sprintf(`<w:jc w:val="%s"/>`, jc)
		}
		return ""
	}())
}

func (d *docxBuilder) pTitulo(texto string) {
	d.corpo.WriteString("<w:p>" + d.pPPr("center", 160) + d.run(`<w:b/><w:color w:val="1E3A8A"/><w:sz w:val="40"/>`, texto) + "</w:p>")
}

func (d *docxBuilder) pNome(texto string) {
	d.corpo.WriteString("<w:p>" + d.pPPr("center", 120) + d.run(`<w:b/><w:color w:val="1D4ED8"/><w:sz w:val="30"/>`, texto) + "</w:p>")
}

func (d *docxBuilder) pSub(texto string) {
	d.corpo.WriteString("<w:p>" + d.pPPr("center", 100) + d.run(`<w:color w:val="475569"/><w:sz w:val="20"/>`, texto) + "</w:p>")
}

func (d *docxBuilder) pSecao(sub, titulo string) {
	d.corpo.WriteString("<w:p>" + d.pPPr("", 40) + d.run(`<w:b/><w:color w:val="1D4ED8"/><w:sz w:val="28"/>`, titulo) + "</w:p>")
	if sub != "" {
		d.corpo.WriteString("<w:p>" + d.pPPr("", 140) + d.run(`<w:b/><w:color w:val="15803D"/><w:sz w:val="16"/>`, esc(sub)) + "</w:p>")
	}
}

func (d *docxBuilder) pTexto(texto string) {
	if texto == "" {
		return
	}
	d.corpo.WriteString("<w:p>" + d.pPPr("", 100) + d.run(`<w:sz w:val="22"/>`, texto) + "</w:p>")
}

func (d *docxBuilder) pRotuloValor(rotulo, valor string) {
	if valor == "" {
		return
	}
	d.corpo.WriteString("<w:p>" + d.pPPr("", 60) +
		d.run(`<w:b/><w:color w:val="475569"/><w:sz w:val="22"/>`, rotulo+": ") +
		d.run(`<w:sz w:val="22"/>`, valor) + "</w:p>")
}

func (d *docxBuilder) pItem(texto string) {
	d.corpo.WriteString("<w:p>" + d.pPPr("", 40) + d.run(`<w:sz w:val="22"/>`, "•  "+texto) + "</w:p>")
}

func (d *docxBuilder) pEspaco() {
	d.corpo.WriteString(`<w:p/>`)
}

func (d *docxBuilder) pQuebra() {
	d.corpo.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
}

func (d *docxBuilder) tabela(cab []string, linhas [][]string, larguras []int) {
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="9360" w:type="dxa"/><w:tblBorders>`)
	for _, lado := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		fmt.Fprintf(&b, `<w:%s w:val="single" w:sz="4" w:space="0" w:color="E2E8F0"/>`, lado)
	}
	b.WriteString(`</w:tblBorders><w:tblLayout w:type="fixed"/></w:tblPr><w:tblGrid>`)
	for _, w := range larguras {
		fmt.Fprintf(&b, `<w:gridCol w:w="%d"/>`, w)
	}
	b.WriteString(`</w:tblGrid>`)
	cel := func(w int, texto string, cabecalho bool) string {
		shd, rpr := "", `<w:sz w:val="20"/>`
		if cabecalho {
			shd = `<w:shd w:val="clear" w:fill="DBEAFE"/>`
			rpr = `<w:b/><w:color w:val="1E3A8A"/><w:sz w:val="20"/>`
		}
		return fmt.Sprintf(`<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>%s</w:tcPr><w:p>%s</w:p></w:tc>`,
			w, shd, d.run(rpr, texto))
	}
	b.WriteString(`<w:tr>`)
	for i, h := range cab {
		b.WriteString(cel(larguras[i], h, true))
	}
	b.WriteString(`</w:tr>`)
	for _, linha := range linhas {
		b.WriteString(`<w:tr>`)
		for i, c := range linha {
			b.WriteString(cel(larguras[i], c, false))
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl><w:p/>`)
	d.corpo.WriteString(b.String())
}

func (d *docxBuilder) pImagem(caminho string, legenda string) {
	if caminho == "" {
		return
	}
	f, err := os.Open(caminho)
	if err != nil {
		return
	}
	cfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil || cfg.Width <= 0 {
		return
	}
	emuW := cfg.Width * 9525
	emuH := cfg.Height * 9525
	if emuW > 5486400 { // 6 polegadas
		emuH = emuH * 5486400 / emuW
		emuW = 5486400
	}
	rid := fmt.Sprintf("rId%d", 100+len(d.media))
	nome := fmt.Sprintf("img%d.jpeg", len(d.media)+1)
	d.media = append(d.media, docxMedia{rid: rid, nome: nome, caminho: caminho})
	id := len(d.media)
	d.corpo.WriteString(fmt.Sprintf(
		`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="%d" cy="%d"/><wp:docPr id="%d" name="%s"/><wp:cNvGraphicFramePr><a:graphicFrameLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" noChangeAspect="1"/></wp:cNvGraphicFramePr><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`,
		emuW, emuH, id, nome, id, nome, rid, emuW, emuH))
	if legenda != "" {
		d.corpo.WriteString("<w:p>" + d.pPPr("center", 120) + d.run(`<w:color w:val="64748B"/><w:sz w:val="16"/>`, legenda) + "</w:p>")
	}
}

func (d *docxBuilder) galeria(fotos []FotoRep) {
	for _, foto := range fotos {
		d.pImagem(foto.Caminho, foto.Legenda)
	}
}

func (d *docxBuilder) montar(re Reporte) {
	// capa
	d.pEspaco()
	d.pTitulo("FORMULÁRIO DE DIAGNÓSTICO ESCOLAR")
	d.pSub("Servidores · Infraestrutura · Obras · Espaços · Projetos · Ofertas de cursos")
	d.pEspaco()
	if re.Fachada != nil {
		d.pImagem(re.Fachada.Caminho, "Foto da fachada — capa do relatório de diagnóstico")
	}
	d.pEspaco()
	d.pNome(re.EscolaNome)
	sub := ""
	if re.Responsavel != "" {
		sub = "Responsável pelo preenchimento: " + re.Responsavel
	}
	if sub != "" {
		d.pSub(sub)
	}
	d.pTexto(re.Endereco)
	if re.Status == "enviado" {
		d.pSub("Enviado em " + re.EnviadoEm)
	}
	d.pSub("Relatório gerado em " + re.GeradoEm + " por " + re.GeradoPor)
	d.pQuebra()

	// identificação
	d.pSecao("Identificação e localização", "Endereço, contatos e GPS")
	d.pRotuloValor("Endereço", re.Endereco)
	if re.GPS.Capturado {
		d.pRotuloValor("Localização GPS", fmt.Sprintf("%.6f, %.6f", re.GPS.Lat, re.GPS.Lng))
		d.pRotuloValor("Link do mapa", re.GPSLink)
	}
	if re.GestorFoto != nil {
		d.pImagem(re.GestorFoto.Caminho, "Foto do(a) gestor(a)")
	}
	if len(re.Contatos) > 0 {
		linhas := [][]string{}
		for _, c := range re.Contatos {
			linhas = append(linhas, []string{c.Setor, c.Telefone, c.Email})
		}
		d.tabela([]string{"Setor", "Telefone", "E-mail"}, linhas, []int{2340, 3260, 3760})
	}

	// servidores
	d.pSecao("Seção 1", "Servidores")
	d.tabela([]string{"Categoria", "Quantidade"}, [][]string{
		{"Servidores na gestão (direção, vice-direção, secretaria e afins)", fmt.Sprintf("%d", re.Servidores.Gestao)},
		{"Servidores no pedagógico (coordenação, supervisão, orientação e apoio)", fmt.Sprintf("%d", re.Servidores.Pedagogico)},
		{"Professores efetivos (concurso / quadro permanente)", fmt.Sprintf("%d", re.Servidores.Efetivos)},
		{"Professores ACT (admitidos em caráter temporário)", fmt.Sprintf("%d", re.Servidores.ACT)},
		{"Total de servidores", fmt.Sprintf("%d", re.TotalServ)},
	}, []int{7260, 2100})
	d.pTexto(fmt.Sprintf("Distribuição dos docentes: %.0f%% efetivos (%d) · %.0f%% ACT (%d)",
		re.PctEfetivos, re.Servidores.Efetivos, re.PctACT, re.Servidores.ACT))

	// infraestrutura
	d.pSecao("Seção 2", "Infraestrutura")
	d.pRotuloValor("Salas de aula", fmt.Sprintf("%d no total · %d ocupadas (%.0f%% de ocupação)", re.Salas.Total, re.Salas.Ocupadas, re.OcupacaoPct))
	d.pRotuloValor("Equipamentos tecnológicos", fmt.Sprintf("Total: %d — %d computadores, %d tablets, %d projetores, %d telas interativas",
		re.TotalEquip, re.Equip.Computadores, re.Equip.Tablets, re.Equip.Projetores, re.Equip.Telas))
	d.pRotuloValor("Outros equipamentos", re.Equip.Outros)
	for _, lab := range re.LabsFixos {
		d.labDocx(lab)
	}
	for _, lab := range re.LabsOutros {
		d.labDocx(lab)
	}
	if len(re.FotosAntes) > 0 {
		d.pTexto("Registro fotográfico — antes da gestão 2023/2027")
		d.galeria(re.FotosAntes)
	}
	if len(re.FotosAtual) > 0 {
		d.pTexto("Registro fotográfico — gestão atual 2023/2027")
		d.galeria(re.FotosAtual)
	}

	// obras realizadas
	if len(re.Obras) > 0 {
		d.pSecao("Seção 3", "Obras realizadas")
		d.pTexto("Cada obra é registrada com fotos de antes e depois tiradas do mesmo ângulo e da mesma distância.")
		for _, o := range re.Obras {
			d.corpo.WriteString("<w:p>" + d.pPPr("", 40) + d.run(`<w:b/><w:color w:val="15803D"/><w:sz w:val="24"/>`, o.Nome) + "</w:p>")
			d.pTexto(o.Descricao)
			if len(o.Antes) > 0 {
				d.pTexto("Antes da obra")
				d.galeria(o.Antes)
			}
			if len(o.Depois) > 0 {
				d.pTexto("Depois da obra")
				d.galeria(o.Depois)
			}
		}
	}

	// espaços que precisam de revitalização
	if len(re.Revitalizacao) > 0 {
		d.pSecao("Seção 4", "Espaços que precisam de revitalização")
		for _, r := range re.Revitalizacao {
			d.corpo.WriteString("<w:p>" + d.pPPr("", 40) + d.run(`<w:b/><w:color w:val="15803D"/><w:sz w:val="24"/>`, r.Nome) + "</w:p>")
			d.pTexto(r.Descricao)
			d.galeria(r.Fotos)
		}
	}

	// espaços
	if len(re.Espacos) > 0 {
		d.pSecao("Seção 5", "Espaços da escola")
		for _, e := range re.Espacos {
			d.corpo.WriteString("<w:p>" + d.pPPr("", 40) + d.run(`<w:b/><w:color w:val="15803D"/><w:sz w:val="24"/>`, e.Nome) + "</w:p>")
			d.pTexto(e.Descricao)
			d.galeria(e.Fotos)
		}
	}

	// projetos
	if len(re.Projetos) > 0 {
		d.pSecao("Seção 6", "Projetos")
		for _, pr := range re.Projetos {
			d.corpo.WriteString("<w:p>" + d.pPPr("", 40) + d.run(`<w:b/><w:color w:val="15803D"/><w:sz w:val="24"/>`, pr.Nome) + "</w:p>")
			d.pTexto(pr.Descricao)
			d.galeria(pr.Fotos)
		}
	}

	// cursos
	if len(re.Cursos) > 0 {
		d.pSecao("Seção 7", "Ofertas de cursos")
		for _, c := range re.Cursos {
			d.pItem(c)
		}
	}
}

func (d *docxBuilder) labDocx(lab LabRep) {
	d.corpo.WriteString("<w:p>" + d.pPPr("", 40) + d.run(`<w:b/><w:color w:val="15803D"/><w:sz w:val="24"/>`, lab.Nome) + "</w:p>")
	d.pTexto(lab.Descricao)
	d.galeria(lab.Fotos)
}

const docxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Calibri" w:cs="Calibri"/><w:sz w:val="22"/></w:rPr></w:rPrDefault><w:pPrDefault/></w:docDefaults>
<w:style w:type="paragraph" w:styleId="Normal" w:default="1"><w:name w:val="Normal"/></w:style>
</w:styles>`

func (d *docxBuilder) zip() ([]byte, error) {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	add := func(nome, conteudo string) error {
		f, err := zw.Create(nome)
		if err != nil {
			return err
		}
		_, err = f.Write([]byte(conteudo))
		return err
	}

	var rels strings.Builder
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, m := range d.media {
		fmt.Fprintf(&rels, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/%s"/>`, m.rid, m.nome)
		f, err := os.Open(m.caminho)
		if err != nil {
			continue
		}
		zf, err := zw.Create("word/media/" + m.nome)
		if err == nil {
			_, _ = io.Copy(zf, f)
		}
		f.Close()
	}
	rels.WriteString(`</Relationships>`)

	documento := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><w:body>` +
		d.corpo.String() +
		`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/></w:sectPr></w:body></w:document>`

	_ = add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Default Extension="jpeg" ContentType="image/jpeg"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`)
	_ = add("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`)
	_ = add("word/document.xml", documento)
	_ = add("word/styles.xml", docxStyles)
	_ = add("word/_rels/document.xml.rels", rels.String())

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
