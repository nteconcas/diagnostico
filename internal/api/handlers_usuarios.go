package api

import (
        "context"
        "errors"
        "net/http"
        "strings"

        "github.com/jackc/pgx/v5/pgconn"

        "diagnostico-escolar/internal/auth"
        "diagnostico-escolar/internal/model"
)

type corpoUsuario struct {
        Nome     string      `json:"nome"`
        Username string      `json:"username"`
        Senha    string      `json:"senha"`
        Papel    model.Papel `json:"papel"`
        EscolaID string      `json:"escola_id"`
}

// resolverEscola valida o vínculo com a escola: vazio → nil (sem vínculo);
// informado → verifica existência e devolve o ponteiro. Administradores
// nunca ficam vinculados a uma escola.
func (s *Servidor) resolverEscola(w http.ResponseWriter, papel model.Papel, escolaID string) (*string, bool) {
        escolaID = strings.TrimSpace(escolaID)
        if papel == model.PapelAdmin || escolaID == "" {
                return nil, true
        }
        var n int
        if err := s.pool.QueryRow(context.Background(), `SELECT count(*) FROM escolas WHERE id = $1`, escolaID).Scan(&n); err != nil || n == 0 {
                erroJSON(w, 400, "Escola vinculada não encontrada")
                return nil, false
        }
        return &escolaID, true
}

func (s *Servidor) usuariosListar(w http.ResponseWriter, r *http.Request) {
        if !exigirAdmin(w, r) {
                return
        }
        linhas, err := s.pool.Query(r.Context(),
                `SELECT u.id::text, u.username, u.nome, u.papel, u.escola_id::text, COALESCE(e.nome, ''), u.criado_em
                 FROM usuarios u LEFT JOIN escolas e ON e.id = u.escola_id
                 ORDER BY u.nome`)
        if err != nil {
                erroJSON(w, 500, "Erro ao listar usuários")
                return
        }
        defer linhas.Close()
        lista := []model.Usuario{}
        for linhas.Next() {
                var u model.Usuario
                var escolaID *string
                if err := linhas.Scan(&u.ID, &u.Username, &u.Nome, &u.Papel, &escolaID, &u.EscolaNome, &u.CriadoEm); err == nil {
                        u.EscolaID = escolaID
                        lista = append(lista, u)
                }
        }
        writeJSON(w, 200, lista)
}

func validarCorpoUsuario(w http.ResponseWriter, corpo corpoUsuario, senhaObrigatoria bool) (model.Usuario, bool) {
        u := model.Usuario{
                Nome:     strings.TrimSpace(corpo.Nome),
                Username: strings.TrimSpace(corpo.Username),
                Papel:    corpo.Papel,
        }
        if u.Nome == "" || u.Username == "" {
                erroJSON(w, 400, "Informe nome e nome de usuário")
                return u, false
        }
        if u.Papel != model.PapelAdmin && u.Papel != model.PapelPreenchedor {
                erroJSON(w, 400, "Papel inválido (use 'admin' ou 'preenchador')")
                return u, false
        }
        if senhaObrigatoria && len(corpo.Senha) < 4 {
                erroJSON(w, 400, "A senha deve ter pelo menos 4 caracteres")
                return u, false
        }
        return u, true
}

func (s *Servidor) usuarioCriar(w http.ResponseWriter, r *http.Request) {
        if !exigirAdmin(w, r) {
                return
        }
        var corpo corpoUsuario
        if !readJSON(w, r, &corpo) {
                return
        }
        u, ok := validarCorpoUsuario(w, corpo, true)
        if !ok {
                return
        }
        escolaID, ok := s.resolverEscola(w, u.Papel, corpo.EscolaID)
        if !ok {
                return
        }
        hash, err := auth.Hash(corpo.Senha)
        if err != nil {
                erroJSON(w, 500, "Erro ao processar senha")
                return
        }
        var id string
        err = s.pool.QueryRow(r.Context(),
                `INSERT INTO usuarios (username, senha_hash, nome, papel, escola_id) VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
                u.Username, hash, u.Nome, u.Papel, escolaID).Scan(&id)
        if err != nil {
                var pgErr *pgconn.PgError
                if errors.As(err, &pgErr) && pgErr.Code == "23505" {
                        erroJSON(w, 409, "Nome de usuário já existe")
                        return
                }
                erroJSON(w, 500, "Erro ao criar usuário")
                return
        }
        writeJSON(w, 201, map[string]string{"id": id})
}

func (s *Servidor) usuarioAtualizar(w http.ResponseWriter, r *http.Request) {
        if !exigirAdmin(w, r) {
                return
        }
        id := r.PathValue("id")
        var corpo corpoUsuario
        if !readJSON(w, r, &corpo) {
                return
        }
        u, ok := validarCorpoUsuario(w, corpo, false)
        if !ok {
                return
        }
        escolaID, ok := s.resolverEscola(w, u.Papel, corpo.EscolaID)
        if !ok {
                return
        }
        var err error
        if corpo.Senha != "" {
                if len(corpo.Senha) < 4 {
                        erroJSON(w, 400, "A senha deve ter pelo menos 4 caracteres")
                        return
                }
                var hash string
                hash, err = auth.Hash(corpo.Senha)
                if err != nil {
                        erroJSON(w, 500, "Erro ao processar senha")
                        return
                }
                _, err = s.pool.Exec(r.Context(),
                        `UPDATE usuarios SET nome = $2, username = $3, papel = $4, senha_hash = $5, escola_id = $6 WHERE id = $1`,
                        id, u.Nome, u.Username, u.Papel, hash, escolaID)
        } else {
                _, err = s.pool.Exec(r.Context(),
                        `UPDATE usuarios SET nome = $2, username = $3, papel = $4, escola_id = $5 WHERE id = $1`,
                        id, u.Nome, u.Username, u.Papel, escolaID)
        }
        if err != nil {
                var pgErr *pgconn.PgError
                if errors.As(err, &pgErr) && pgErr.Code == "23505" {
                        erroJSON(w, 409, "Nome de usuário já existe")
                        return
                }
                erroJSON(w, 500, "Erro ao atualizar usuário")
                return
        }
        writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Servidor) usuarioExcluir(w http.ResponseWriter, r *http.Request) {
        if !exigirAdmin(w, r) {
                return
        }
        id := r.PathValue("id")
        if auth.UsuarioDoCtx(r).ID == id {
                erroJSON(w, 400, "Você não pode excluir o seu próprio usuário")
                return
        }
        ct, err := s.pool.Exec(r.Context(), `DELETE FROM usuarios WHERE id = $1`, id)
        if err != nil || ct.RowsAffected() == 0 {
                erroJSON(w, 404, "Usuário não encontrado")
                return
        }
        writeJSON(w, 200, map[string]bool{"ok": true})
}
