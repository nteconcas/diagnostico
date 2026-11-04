package api

import (
	"net/http"
	"path/filepath"
	"regexp"

	"diagnostico-escolar/internal/auth"
)

var padraoArquivo = regexp.MustCompile(`^[0-9a-f]{16,64}\.(orig\.(jpg|jpeg|png)|t\.jpg|m\.jpg)$`)

func (s *Servidor) fotoUpload(w http.ResponseWriter, r *http.Request) {
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
	foto, tamanho, err := s.loja.SalvarImagem(arquivo, cabecalho.Filename, u.ID)
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
		foto.ID, foto.ID+".orig.jpg", cabecalho.Filename, mime, tamanho, u.ID); err != nil {
		erroJSON(w, 500, "Erro ao registrar a foto")
		return
	}
	writeJSON(w, 201, foto)
}

func (s *Servidor) fotoExcluir(w http.ResponseWriter, r *http.Request) {
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

// fotoServir entrega os arquivos de imagem com cache longo.
func (s *Servidor) fotoServir(w http.ResponseWriter, r *http.Request) {
	arquivo := filepath.Base(r.PathValue("arquivo"))
	if !padraoArquivo.MatchString(arquivo) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, s.loja.Caminho(arquivo))
}
