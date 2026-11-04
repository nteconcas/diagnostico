package report

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

// DirFotos é a pasta uploads (definida pelo main na inicialização).
var DirFotos string

// caminhoFoto resolve o arquivo médio no disco a partir do id da foto.
func caminhoFoto(id string) string {
	if id == "" {
		return ""
	}
	return filepath.Join(DirFotos, id+".m.jpg")
}

// b64 devolve o conteúdo do arquivo em base64 (imagens embutidas no HTML).
func b64(caminho string) string {
	boleto, err := os.ReadFile(caminho)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(boleto)
}

// galeriaHTML monta a grade de fotos com legendas.
func galeriaHTML(fotos []FotoRep) template.HTML {
	if len(fotos) == 0 {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString(`<div class="galeria">`)
	for _, f := range fotos {
		fmt.Fprintf(&buf,
			`<figure><img src="data:image/jpeg;base64,%s" alt="%s"><figcaption>%s</figcaption></figure>`,
			b64(f.Caminho), template.HTMLEscapeString(f.Legenda), template.HTMLEscapeString(f.Legenda))
	}
	buf.WriteString(`</div>`)
	return template.HTML(buf.String())
}

var funcMap = template.FuncMap{
	"b64":     b64,
	"galeria": galeriaHTML,
}

// RenderHTML gera o relatório em HTML autossuficiente (imagens embutidas em base64).
func RenderHTML(re Reporte) ([]byte, error) {
	tpl, err := template.New("relatorio").Funcs(funcMap).Parse(tplHTML)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, re); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
