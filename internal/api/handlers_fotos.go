package api

import (
	"context"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"diagnostico-escolar/internal/auth"
	"diagnostico-escolar/internal/model"
)

var padraoArquivo = regexp.MustCompile(`^[0-9a-f]{16,64}\.(orig\.(jpg|jpeg|png|gif|webp)|t\.jpg|m\.jpg)$`)

func (s *Servidor) fotoUpload(w http.ResponseWriter, r *http.Request) {
	if !exigirPreenchador(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		erroJSON(w, 400, "Arquivo muito grande ou requisição inválida (máximo: 20 MB por foto)")
		return
	}
	arquivo, cabecalho, err := r.FormFile("arquivo")
	if err != nil {
		erroJSON(w, 400, "Campo 'arquivo' não encontrado no envio")
		return
	}
	defer arquivo.Close()
	u := auth.UsuarioDoCtx(r)
	escolaID := r.FormValue("escola_id")
	if escolaID == "" {
		erroJSON(w, 400, "Informe a escola vinculada à foto")
		return
	}
	if !podeEscola(u, escolaID) {
		negadoEscola(w)
		return
	}
	var nomeEscola string
	if err := s.pool.QueryRow(r.Context(), `SELECT nome FROM escolas WHERE id = $1`, escolaID).Scan(&nomeEscola); err != nil {
		erroJSON(w, 404, "Escola não encontrada")
		return
	}
	foto, tamanho, arquivoR2, err := s.loja.SalvarImagem(r.Context(), arquivo, cabecalho.Filename, nomeEscola)
	if err != nil {
		erroJSON(w, 400, err.Error())
		return
	}
	mime := cabecalho.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/jpeg"
	}
	if _, err := s.pool.Exec(r.Context(),
		`INSERT INTO fotos (id, arquivo, nome_original, mime, bytes, enviado_por)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		foto.ID, arquivoR2, cabecalho.Filename, mime, tamanho, u.ID); err != nil {
		s.loja.Excluir(foto.ID, arquivoR2)
		erroJSON(w, 500, "Erro ao registrar a foto")
		return
	}
	writeJSON(w, 201, foto)
}

func (s *Servidor) fotoExcluir(w http.ResponseWriter, r *http.Request) {
	if !exigirPreenchador(w, r) {
		return
	}
	id := r.PathValue("id")
	var arquivo string
	err := s.pool.QueryRow(r.Context(), `SELECT arquivo FROM fotos WHERE id = $1`, id).Scan(&arquivo)
	if err != nil {
		erroJSON(w, 404, "Foto não encontrada")
		return
	}
	s.loja.Excluir(id, arquivo)
	_, _ = s.pool.Exec(r.Context(), `DELETE FROM fotos WHERE id = $1`, id)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// fotoServir entrega as imagens autenticadas com cache privado.
func (s *Servidor) fotoServir(w http.ResponseWriter, r *http.Request) {
	arquivo := filepath.Base(r.PathValue("arquivo"))
	partes := padraoArquivo.FindStringSubmatch(arquivo)
	if len(partes) == 0 {
		http.NotFound(w, r)
		return
	}
	id := strings.SplitN(arquivo, ".", 2)[0]
	var chaveOriginal string
	if err := s.pool.QueryRow(r.Context(), `SELECT arquivo FROM fotos WHERE id = $1`, id).Scan(&chaveOriginal); err != nil {
		http.NotFound(w, r)
		return
	}
	chave, local := chaveOriginal, id+".orig."+path.Ext(chaveOriginal)[1:]
	if strings.HasSuffix(arquivo, ".t.jpg") {
		chave, local = path.Dir(chaveOriginal)+"/thumb.jpg", id+".t.jpg"
	} else if strings.HasSuffix(arquivo, ".m.jpg") {
		chave, local = path.Dir(chaveOriginal)+"/medium.jpg", id+".m.jpg"
	}
	if err := s.loja.GarantirArquivo(r.Context(), chave, local); err != nil {
		http.Error(w, "Imagem indisponível", http.StatusBadGateway)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, s.loja.Caminho(local))
}

func (s *Servidor) prepararFotosRelatorio(ctx context.Context, dados model.Dados) error {
	fotos := []model.Foto{}
	if dados.FotoGestor != nil {
		fotos = append(fotos, *dados.FotoGestor)
	}
	if dados.FotoFachada != nil {
		fotos = append(fotos, *dados.FotoFachada)
	}
	fotos = append(fotos, dados.FotosAntes...)
	fotos = append(fotos, dados.FotosAtual...)
	for _, lab := range []model.LabFixo{dados.Labs.Maker, dados.Labs.Informatica} {
		fotos = append(fotos, lab.Fotos...)
	}
	for _, lab := range dados.Labs.Outros {
		fotos = append(fotos, lab.Fotos...)
	}
	for _, espaco := range dados.Espacos {
		fotos = append(fotos, espaco.Fotos...)
	}
	for _, projeto := range dados.Projetos {
		fotos = append(fotos, projeto.Fotos...)
	}
	vistos := map[string]bool{}
	for _, foto := range fotos {
		if foto.ID == "" || vistos[foto.ID] {
			continue
		}
		vistos[foto.ID] = true
		var arquivo string
		if err := s.pool.QueryRow(ctx, `SELECT arquivo FROM fotos WHERE id = $1`, foto.ID).Scan(&arquivo); err != nil {
			return err
		}
		if err := s.loja.PrepararRelatorio(ctx, foto.ID, arquivo); err != nil {
			return err
		}
	}
	return nil
}
