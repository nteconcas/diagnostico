// Package api expõe a API REST do formulário de diagnóstico.
package api

import (
        "encoding/json"
        "log"
        "net/http"
        "time"

        "github.com/jackc/pgx/v5/pgxpool"

        "diagnostico-escolar/internal/auth"
        "diagnostico-escolar/internal/fsstore"
)

// Servidor concentra os handlers da API.
type Servidor struct {
        pool *pgxpool.Pool
        loja *fsstore.Loja
}

// Novo monta o roteador completo (público + protegido por sessão).
func Novo(pool *pgxpool.Pool, loja *fsstore.Loja) *http.ServeMux {
        s := &Servidor{pool: pool, loja: loja}
        mux := http.NewServeMux()

        mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
                writeJSON(w, 200, map[string]string{"status": "ok"})
        })
        mux.HandleFunc("POST /api/login", s.login)

        prot := http.NewServeMux()
        prot.HandleFunc("POST /api/logout", s.logout)
        prot.HandleFunc("GET /api/me", s.me)

        prot.HandleFunc("GET /api/escolas", s.escolasListar)
        prot.HandleFunc("POST /api/escolas", s.escolaCriar)
        prot.HandleFunc("PUT /api/escolas/{id}", s.escolaAtualizar)
        prot.HandleFunc("DELETE /api/escolas/{id}", s.escolaExcluir)

        prot.HandleFunc("GET /api/diagnosticos", s.diagListar)
        prot.HandleFunc("POST /api/diagnosticos", s.diagCriar)
        prot.HandleFunc("GET /api/diagnosticos/{id}", s.diagObter)
        prot.HandleFunc("PUT /api/diagnosticos/{id}/rascunho", s.diagSalvarRascunho)
        prot.HandleFunc("POST /api/diagnosticos/{id}/enviar", s.diagEnviar)
        prot.HandleFunc("DELETE /api/diagnosticos/{id}", s.diagExcluir)
        prot.HandleFunc("GET /api/diagnosticos/{id}/relatorio.pdf", s.relatorioPDF)
        prot.HandleFunc("GET /api/diagnosticos/{id}/relatorio.docx", s.relatorioDOCX)
        prot.HandleFunc("GET /api/diagnosticos/{id}/relatorio.html", s.relatorioHTML)

        prot.HandleFunc("POST /api/fotos", s.fotoUpload)
        prot.HandleFunc("POST /api/fotos/{id}/excluir", s.fotoExcluir)

        prot.HandleFunc("GET /api/usuarios", s.usuariosListar)
        prot.HandleFunc("POST /api/usuarios", s.usuarioCriar)
        prot.HandleFunc("PUT /api/usuarios/{id}", s.usuarioAtualizar)
        prot.HandleFunc("DELETE /api/usuarios/{id}", s.usuarioExcluir)

        mux.Handle("/api/", auth.MiddlewareAuth(pool)(prot))
        mux.Handle("GET /fotos/{arquivo}", auth.MiddlewareAuth(pool)(http.HandlerFunc(s.fotoServir)))
        return mux
}

// logMiddleware registra método, caminho e duração (usado pelo main).
func LogMiddleware(proximo http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                inicio := time.Now()
                proximo.ServeHTTP(w, r)
                log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(inicio).Round(time.Millisecond))
        })
}

// writeJSON responde com JSON e o status informado.
func writeJSON(w http.ResponseWriter, status int, corpo any) {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(status)
        _ = json.NewEncoder(w).Encode(corpo)
}

// erroJSON responde com {"erro": "..."} e status apropriado.
func erroJSON(w http.ResponseWriter, status int, mensagem string) {
        writeJSON(w, status, map[string]string{"erro": mensagem})
}

func erroPendencias(w http.ResponseWriter, pendencias []string) {
        writeJSON(w, 400, map[string]any{"erro": "Formulário incompleto", "pendencias": pendencias})
}

// readJSON decodifica o corpo JSON da requisição.
func readJSON(w http.ResponseWriter, r *http.Request, destino any) bool {
        defer r.Body.Close()
        if err := json.NewDecoder(r.Body).Decode(destino); err != nil {
                erroJSON(w, 400, "Corpo da requisição inválido")
                return false
        }
        return true
}

func exigirAdmin(w http.ResponseWriter, r *http.Request) bool {
        u := auth.UsuarioDoCtx(r)
        if u == nil || u.Papel != "admin" {
                erroJSON(w, 403, "Apenas administradores podem executar esta ação")
                return false
        }
        return true
}
