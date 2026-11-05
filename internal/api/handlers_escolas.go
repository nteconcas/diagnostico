package api

import (
        "context"
        "errors"
        "net/http"
        "strings"
        "time"

        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgconn"

        "diagnostico-escolar/internal/auth"
        "diagnostico-escolar/internal/model"
)

type corpoResponsavel struct {
        Nome     string `json:"nome"`
        Username string `json:"username"`
        Senha    string `json:"senha"`
}

type escolaComStatus struct {
        ID            string                `json:"id"`
        Nome          string                `json:"nome"`
        CriadoEm      time.Time             `json:"criado_em"`
        UltimoID      *string               `json:"ultimo_diagnostico_id"`
        Status        *string               `json:"status"`
        AtualizadoEm  *time.Time            `json:"atualizado_em"`
        AtualizadoPor *string               `json:"atualizado_por"`
        Responsaveis  []model.UsuarioResumo `json:"responsaveis"`
}

func (s *Servidor) escolasListar(w http.ResponseWriter, r *http.Request) {
        u := auth.UsuarioDoCtx(r)
        consulta := `
                SELECT e.id::text, e.nome, e.criado_em,
                       d.id::text, d.status, d.atualizado_em, u2.nome
                FROM escolas e
                LEFT JOIN LATERAL (
                        SELECT id, status, atualizado_em, atualizado_por
                        FROM diagnosticos x WHERE x.escola_id = e.id
                        ORDER BY criado_em DESC LIMIT 1
                ) d ON true
                LEFT JOIN usuarios u2 ON u2.id = d.atualizado_por`
        argumentos := []any{}
        // Responsável vinculado enxerga apenas a sua escola.
        if u != nil && u.EscolaID != nil {
                consulta += ` WHERE e.id = $1`
                argumentos = append(argumentos, *u.EscolaID)
        }
        consulta += ` ORDER BY e.nome`
        linhas, err := s.pool.Query(r.Context(), consulta, argumentos...)
        if err != nil {
                erroJSON(w, 500, "Erro ao listar escolas")
                return
        }
        defer linhas.Close()
        lista := []escolaComStatus{}
        for linhas.Next() {
                var e escolaComStatus
                if err := linhas.Scan(&e.ID, &e.Nome, &e.CriadoEm, &e.UltimoID, &e.Status, &e.AtualizadoEm, &e.AtualizadoPor); err == nil {
                        e.Responsaveis = []model.UsuarioResumo{}
                        lista = append(lista, e)
                }
        }
        // Anexa os usuários responsáveis de cada escola.
        resp, err := s.pool.Query(r.Context(),
                `SELECT id::text, nome, username, escola_id::text FROM usuarios WHERE escola_id IS NOT NULL ORDER BY nome`)
        if err == nil {
                defer resp.Close()
                indice := map[string]int{}
                for i := range lista {
                        indice[lista[i].ID] = i
                }
                for resp.Next() {
                        var ur model.UsuarioResumo
                        var escolaID string
                        if err := resp.Scan(&ur.ID, &ur.Nome, &ur.Username, &escolaID); err == nil {
                                if i, ok := indice[escolaID]; ok {
                                        lista[i].Responsaveis = append(lista[i].Responsaveis, ur)
                                }
                        }
                }
        }
        writeJSON(w, 200, lista)
}

func (s *Servidor) escolaCriar(w http.ResponseWriter, r *http.Request) {
        if !exigirAdmin(w, r) {
                return
        }
        var corpo struct {
                Nome        string             `json:"nome"`
                Responsavel *corpoResponsavel  `json:"responsavel"`
        }
        if !readJSON(w, r, &corpo) {
                return
        }
        if corpo.Nome == "" {
                erroJSON(w, 400, "Informe o nome da escola")
                return
        }
        // Valida o responsável (opcional) antes de gravar qualquer coisa.
        var respNome, respUser, respSenha string
        if corpo.Responsavel != nil {
                respNome = strings.TrimSpace(corpo.Responsavel.Nome)
                respUser = strings.TrimSpace(corpo.Responsavel.Username)
                respSenha = corpo.Responsavel.Senha
                if respNome == "" || respUser == "" || len(respSenha) < 4 {
                        erroJSON(w, 400, "Para o responsável, informe nome, usuário e senha (mínimo de 4 caracteres)")
                        return
                }
        }

        tx, err := s.pool.Begin(r.Context())
        if err != nil {
                erroJSON(w, 500, "Erro ao criar escola")
                return
        }
        defer tx.Rollback(r.Context())

        var id string
        err = tx.QueryRow(r.Context(),
                `INSERT INTO escolas (nome) VALUES ($1) RETURNING id::text`,
                corpo.Nome).Scan(&id)
        if err != nil {
                erroJSON(w, 500, "Erro ao criar escola")
                return
        }

        resposta := map[string]string{"id": id}
        if corpo.Responsavel != nil {
                hash, errH := auth.Hash(respSenha)
                if errH != nil {
                        erroJSON(w, 500, "Erro ao processar senha do responsável")
                        return
                }
                var idUsuario string
                errU := tx.QueryRow(r.Context(),
                        `INSERT INTO usuarios (username, senha_hash, nome, papel, escola_id)
                         VALUES ($1, $2, $3, 'preenchador', $4) RETURNING id::text`,
                        respUser, hash, respNome, id).Scan(&idUsuario)
                if errU != nil {
                        var pgErr *pgconn.PgError
                        if errors.As(errU, &pgErr) && pgErr.Code == "23505" {
                                erroJSON(w, 409, "Nome de usuário do responsável já existe")
                                return
                        }
                        erroJSON(w, 500, "Erro ao criar o usuário responsável")
                        return
                }
                resposta["responsavel_id"] = idUsuario
                resposta["responsavel_username"] = respUser
        }
        if err := tx.Commit(r.Context()); err != nil {
                erroJSON(w, 500, "Erro ao concluir o cadastro")
                return
        }
        writeJSON(w, 201, resposta)
}

func (s *Servidor) escolaAtualizar(w http.ResponseWriter, r *http.Request) {
        if !exigirAdmin(w, r) {
                return
        }
        id := r.PathValue("id")
        var corpo model.Escola
        if !readJSON(w, r, &corpo) {
                return
        }
        if corpo.Nome == "" {
                erroJSON(w, 400, "Informe o nome da escola")
                return
        }
        ct, err := s.pool.Exec(r.Context(),
                `UPDATE escolas SET nome = $2 WHERE id = $1`, id, corpo.Nome)
        if err != nil || ct.RowsAffected() == 0 {
                erroJSON(w, 404, "Escola não encontrada")
                return
        }
        writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Servidor) escolaExcluir(w http.ResponseWriter, r *http.Request) {
        if !exigirAdmin(w, r) {
                return
        }
        id := r.PathValue("id")
        var n int
        _ = s.pool.QueryRow(r.Context(),
                `SELECT count(*) FROM diagnosticos WHERE escola_id = $1`, id).Scan(&n)
        if n > 0 {
                erroJSON(w, 409, "A escola possui diagnósticos registrados — exclua-os primeiro")
                return
        }
        ct, err := s.pool.Exec(r.Context(), `DELETE FROM escolas WHERE id = $1`, id)
        if err != nil || ct.RowsAffected() == 0 {
                erroJSON(w, 404, "Escola não encontrada")
                return
        }
        writeJSON(w, 200, map[string]bool{"ok": true})
}

var errNaoEncontrado = errors.New("não encontrado")

// carregarDiagnostico busca o diagnóstico completo com nome da escola.
func (s *Servidor) carregarDiagnostico(ctx context.Context, id string) (model.Diagnostico, error) {
        var d model.Diagnostico
        var dadosBrutos []byte
        var criadoNome, atualizadoNome *string
        var criadoUser, atualizadoUser *string
        var criadoID, atualizadoID *string
        err := s.pool.QueryRow(ctx, `
                SELECT d.id::text, d.escola_id::text, d.status, d.dados, d.criado_em, d.atualizado_em, d.enviado_em,
                       e.nome,
                       cu.id::text, cu.nome, cu.username,
                       au.id::text, au.nome, au.username
                FROM diagnosticos d
                JOIN escolas e ON e.id = d.escola_id
                LEFT JOIN usuarios cu ON cu.id = d.criado_por
                LEFT JOIN usuarios au ON au.id = d.atualizado_por
                WHERE d.id = $1`, id).
                Scan(&d.ID, &d.EscolaID, &d.Status, &dadosBrutos, &d.CriadoEm, &d.AtualizadoEm, &d.EnviadoEm,
                        &d.EscolaNome,
                        &criadoID, &criadoNome, &criadoUser,
                        &atualizadoID, &atualizadoNome, &atualizadoUser)
        if errors.Is(err, pgx.ErrNoRows) {
                return d, errNaoEncontrado
        }
        if err != nil {
                return d, err
        }
        if err := parseDados(dadosBrutos, &d.Dados); err != nil {
                return d, err
        }
        if criadoID != nil {
                d.CriadoPor = &model.UsuarioResumo{ID: *criadoID, Nome: valor(criadoNome), Username: valor(criadoUser)}
        }
        if atualizadoID != nil {
                d.AtualizadoPor = &model.UsuarioResumo{ID: *atualizadoID, Nome: valor(atualizadoNome), Username: valor(atualizadoUser)}
        }
        return d, nil
}

func valor(s *string) string {
        if s == nil {
                return ""
        }
        return *s
}
