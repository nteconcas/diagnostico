package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"diagnostico-escolar/internal/report"
)

//go:embed all:frontend/dist
var arquivosEmbed embed.FS

// RegistrarWeb serve o frontend React compilado (com fallback SPA para o index.html)
// e define a pasta de fotos usada pelos relatórios.
func RegistrarWeb(mux *http.ServeMux, uploadsDir string) {
	conteudo, err := fs.Sub(arquivosEmbed, "frontend/dist")
	if err != nil {
		panic(err)
	}
	report.DirFotos = uploadsDir
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caminho := strings.TrimPrefix(r.URL.Path, "/")
		if caminho != "" {
			if f, err := conteudo.Open(caminho); err == nil {
				f.Close()
				http.FileServerFS(conteudo).ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(conteudo, "index.html")
		if err != nil {
			http.Error(w, "Frontend não compilado: execute 'npm install && npm run build' dentro de frontend/", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	}))
}
