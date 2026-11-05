package api

import (
        "context"
        "encoding/json"
        "errors"
        "log"
        "net/http"
        "strings"
        "time"

        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgconn"

        "diagnostico-escolar/internal/auth"
        "diagnostico-escolar/internal/model"
        "diagnostico-escolar/internal/report"
)

func parseDados(brutos []byte, destino *model.Dados) error {
        *destino = model.Dados{}
        if len(brutos) == 0 {
                return nil
        }
        if err := json.Unmarshal(brutos, destino); err != nil {
                return err
        }
        destino.Normalizar()
        return nil
}

type listaDiag struct {
        ID            string     `json:"id"`
        EscolaID      string     `json:"escola_id"`
        EscolaNome    string     `json:"escola_nome"`
        Status        string     `json:"status"`
        CriadoEm      time.Time  `json:"criado_em"`
        AtualizadoEm  time.Time  `json:"atualizado_em"`
        EnviadoEm     *time.Time `json:"enviado_em"`
        CriadoPor     *string    `json:"criado_por"`
        AtualizadoPor *string    `json:"atualizado_por"`
}

// podeEscola indica se o preenchidor pode manipular diagnósticos da escola.
// Responsáveis vinculados ficam restritos à própria escola.
func podeEscola(u *model.Usuario, escolaID string) bool {
        return u == nil || u.EscolaID == nil || *u.EscolaID == escolaID
}

func negadoEscola(w http.ResponseWriter) {
        erroJSON(w, 403, "Acesso restrito aos diagnósticos da sua escola")
}

func (s *Servidor) diagListar(w http.ResponseWriter, r *http.Request) {
        u := auth.UsuarioDoCtx(r)
        consulta := `
                SELECT d.id::text, d.escola_id::text, e.nome, d.status,
                       d.criado_em, d.atualizado_em, d.enviado_em, cu.nome, au.nome
                FROM diagnosticos d
                JOIN escolas e ON e.id = d.escola_id
                LEFT JOIN usuarios cu ON cu.id = d.criado_por
                LEFT JOIN usuarios au ON au.id = d.atualizado_por`
        argumentos := []any{}
        if u != nil && u.EscolaID != nil {
                consulta += ` WHERE d.escola_id = $1`
                argumentos = append(argumentos, *u.EscolaID)
        }
        consulta += ` ORDER BY d.atualizado_em DESC`
        linhas, err := s.pool.Query(r.Context(), consulta, argumentos...)
        if err != nil {
                erroJSON(w, 500, "Erro ao listar diagnósticos")
                return
        }
        defer linhas.Close()
        lista := []listaDiag{}
        for linhas.Next() {
                var l listaDiag
                if err := linhas.Scan(&l.ID, &l.EscolaID, &l.EscolaNome, &l.Status,
                        &l.CriadoEm, &l.AtualizadoEm, &l.EnviadoEm, &l.CriadoPor, &l.AtualizadoPor); err == nil {
                        lista = append(lista, l)
                }
        }
        writeJSON(w, 200, lista)
}

func (s *Servidor) diagCriar(w http.ResponseWriter, r *http.Request) {
        if !exigirPreenchador(w, r) {
                return
        }
        var corpo struct {
                EscolaID string `json:"escola_id"`
        }
        if !readJSON(w, r, &corpo) {
                return
        }
        if corpo.EscolaID == "" {
                erroJSON(w, 400, "Selecione a escola que está sendo diagnosticada")
                return
        }
        var nome string
        err := s.pool.QueryRow(r.Context(),
                `SELECT nome FROM escolas WHERE id = $1`, corpo.EscolaID).Scan(&nome)
        if errors.Is(err, pgx.ErrNoRows) {
                erroJSON(w, 404, "Escola não encontrada")
                return
        }
        if err != nil {
                erroJSON(w, 500, "Erro ao verificar escola")
                return
        }
        u := auth.UsuarioDoCtx(r)
        if !podeEscola(u, corpo.EscolaID) {
                negadoEscola(w)
                return
        }
        // Se já existe rascunho para a escola, devolve o existente para continuar
        // o preenchimento em vez de criar um registro novo.
        existente, err := s.rascunhoDaEscola(r.Context(), corpo.EscolaID)
        if err == nil {
                writeJSON(w, 200, existente)
                return
        }
        if !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, errNaoEncontrado) {
                log.Printf("buscar rascunho da escola %s: %v", corpo.EscolaID, err)
                erroJSON(w, 500, "Erro ao verificar rascunho existente")
                return
        }
        // O cast ::text é obrigatório: jsonb_build_object é variádico ("any") e o
        // Postgres não infere o tipo de um parâmetro sem tipo — sem o cast o INSERT
        // falha com "could not determine data type of parameter $3".
        var id string
        err = s.pool.QueryRow(r.Context(), `
                INSERT INTO diagnosticos (escola_id, criado_por, atualizado_por, dados)
                VALUES ($1, $2, $2, jsonb_build_object('responsavel', $3::text)) RETURNING id::text`,
                corpo.EscolaID, u.ID, u.Nome).Scan(&id)
        if err != nil {
                // Outra aba/dispositivo pode ter criado o rascunho entre a consulta
                // acima e este INSERT: o índice único por escola (código 23505) barra
                // a duplicata e aqui devolvemos o rascunho que venceu a corrida.
                var pgErr *pgconn.PgError
                if errors.As(err, &pgErr) && pgErr.Code == "23505" {
                        if corrida, busca := s.rascunhoDaEscola(r.Context(), corpo.EscolaID); busca == nil {
                                writeJSON(w, 200, corrida)
                                return
                        }
                }
                log.Printf("criar diagnóstico (escola %s, usuário %s): %v", corpo.EscolaID, u.ID, err)
                erroJSON(w, 500, "Erro ao criar diagnóstico")
                return
        }
        d, err := s.carregarDiagnostico(r.Context(), id)
        if err != nil {
                log.Printf("carregar diagnóstico %s: %v", id, err)
                erroJSON(w, 500, "Erro ao carregar diagnóstico")
                return
        }
        writeJSON(w, 201, d)
}

// rascunhoDaEscola devolve o rascunho mais recente da escola. Retorna
// pgx.ErrNoRows quando a escola ainda não tem rascunho em preenchimento.
func (s *Servidor) rascunhoDaEscola(ctx context.Context, escolaID string) (model.Diagnostico, error) {
        var id string
        err := s.pool.QueryRow(ctx,
                `SELECT id::text FROM diagnosticos
                 WHERE escola_id = $1 AND status = 'rascunho'
                 ORDER BY atualizado_em DESC
                 LIMIT 1`,
                escolaID).Scan(&id)
        if err != nil {
                return model.Diagnostico{}, err
        }
        return s.carregarDiagnostico(ctx, id)
}

func (s *Servidor) diagObter(w http.ResponseWriter, r *http.Request) {
        d, err := s.carregarDiagnostico(r.Context(), r.PathValue("id"))
        if errors.Is(err, errNaoEncontrado) {
                erroJSON(w, 404, "Diagnóstico não encontrado")
                return
        }
        if err != nil {
                erroJSON(w, 500, "Erro ao carregar diagnóstico")
                return
        }
        u := auth.UsuarioDoCtx(r)
        if u.Papel == "admin" && d.Status != "enviado" {
                erroJSON(w, 403, "Administradores não podem acessar formulários em preenchimento")
                return
        }
        if !podeEscola(u, d.EscolaID) {
                negadoEscola(w)
                return
        }
        writeJSON(w, 200, d)
}

func (s *Servidor) diagSalvarRascunho(w http.ResponseWriter, r *http.Request) {
        if !exigirPreenchador(w, r) {
                return
        }
        id := r.PathValue("id")
        var corpo struct {
                Dados model.Dados `json:"dados"`
        }
        if !readJSON(w, r, &corpo) {
                return
        }
        var escolaID string
        if err := s.pool.QueryRow(r.Context(), `SELECT escola_id::text FROM diagnosticos WHERE id = $1`, id).Scan(&escolaID); err != nil {
                if errors.Is(err, pgx.ErrNoRows) {
                        erroJSON(w, 404, "Diagnóstico não encontrado")
                        return
                }
                erroJSON(w, 500, "Erro ao verificar diagnóstico")
                return
        }
        if !podeEscola(auth.UsuarioDoCtx(r), escolaID) {
                negadoEscola(w)
                return
        }
        corpo.Dados.Normalizar()
        u := auth.UsuarioDoCtx(r)
        corpo.Dados.Responsavel = u.Nome
        brutos, err := json.Marshal(corpo.Dados)
        if err != nil {
                erroJSON(w, 500, "Erro ao processar dados")
                return
        }
        var atualizadoEm time.Time
        err = s.pool.QueryRow(r.Context(), `
                UPDATE diagnosticos
                SET dados = $2, atualizado_por = $3, atualizado_em = now()
                WHERE id = $1 AND status = 'rascunho'
                RETURNING atualizado_em`, id, brutos, u.ID).Scan(&atualizadoEm)
        if errors.Is(err, pgx.ErrNoRows) {
                // Nenhuma linha alterada: o diagnóstico já foi enviado ou não existe.
                var status string
                errStatus := s.pool.QueryRow(r.Context(),
                        `SELECT status FROM diagnosticos WHERE id = $1`, id).Scan(&status)
                if errors.Is(errStatus, pgx.ErrNoRows) {
                        erroJSON(w, 404, "Diagnóstico não encontrado")
                        return
                }
                if errStatus != nil {
                        erroJSON(w, 500, "Erro ao verificar diagnóstico")
                        return
                }
                erroJSON(w, 409, "Este diagnóstico já foi enviado e não pode mais ser alterado")
                return
        }
        if err != nil {
                erroJSON(w, 500, "Erro ao salvar rascunho")
                return
        }
        // atualizado_em volta para o cliente acompanhar a versão gravada.
        writeJSON(w, 200, map[string]any{"ok": true, "atualizado_em": atualizadoEm})
}

func (s *Servidor) diagEnviar(w http.ResponseWriter, r *http.Request) {
        if !exigirPreenchador(w, r) {
                return
        }
        id := r.PathValue("id")
        d, err := s.carregarDiagnostico(r.Context(), id)
        if errors.Is(err, errNaoEncontrado) {
                erroJSON(w, 404, "Diagnóstico não encontrado")
                return
        }
        if err != nil {
                erroJSON(w, 500, "Erro ao carregar diagnóstico")
                return
        }
        if d.Status == "enviado" {
                erroJSON(w, 409, "Este diagnóstico já foi enviado")
                return
        }
        if !podeEscola(auth.UsuarioDoCtx(r), d.EscolaID) {
                negadoEscola(w)
                return
        }
        if pend := d.Dados.Pendencias(); len(pend) > 0 {
                erroPendencias(w, pend)
                return
        }
        u := auth.UsuarioDoCtx(r)
        enviadoEm := time.Now()
        _, err = s.pool.Exec(r.Context(), `
                UPDATE diagnosticos
                SET status = 'enviado', enviado_em = $2, atualizado_por = $3, atualizado_em = now()
                WHERE id = $1 AND status = 'rascunho'`, id, enviadoEm, u.ID)
        if err != nil {
                erroJSON(w, 500, "Erro ao enviar diagnóstico")
                return
        }
        d2, err := s.carregarDiagnostico(r.Context(), id)
        if err != nil {
                erroJSON(w, 500, "Erro ao recarregar diagnóstico")
                return
        }
        writeJSON(w, 200, d2)
}

func (s *Servidor) diagExcluir(w http.ResponseWriter, r *http.Request) {
        if !exigirAdmin(w, r) {
                return
        }
        ct, err := s.pool.Exec(r.Context(), `DELETE FROM diagnosticos WHERE id = $1`, r.PathValue("id"))
        if err != nil || ct.RowsAffected() == 0 {
                erroJSON(w, 404, "Diagnóstico não encontrado")
                return
        }
        writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---- relatórios ----

func nomeArquivo(re report.Reporte, formato string) string {
        return "diagnostico-" + slug(re.EscolaNome) + "-" + time.Now().Format("2006-01-02") + "." + formato
}

func slug(s string) string {
        acentos := map[rune]rune{
                'á': 'a', 'à': 'a', 'ã': 'a', 'â': 'a', 'é': 'e', 'ê': 'e', 'í': 'i',
                'ó': 'o', 'õ': 'o', 'ô': 'o', 'ú': 'u', 'ü': 'u', 'ç': 'c',
        }
        var b strings.Builder
        for _, r := range strings.ToLower(s) {
                if rr, ok := acentos[r]; ok {
                        r = rr
                }
                switch {
                case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
                        b.WriteRune(r)
                default:
                        b.WriteRune('-')
                }
        }
        return strings.Trim(b.String(), "-")
}

func (s *Servidor) enviarRelatorio(w http.ResponseWriter, r *http.Request, formato string) {
        d, err := s.carregarDiagnostico(r.Context(), r.PathValue("id"))
        if errors.Is(err, errNaoEncontrado) {
                erroJSON(w, 404, "Diagnóstico não encontrado")
                return
        }
        if err != nil {
                erroJSON(w, 500, "Erro ao carregar diagnóstico")
                return
        }
        if !podeEscola(auth.UsuarioDoCtx(r), d.EscolaID) {
                negadoEscola(w)
                return
        }
        if err := s.prepararFotosRelatorio(r.Context(), d.Dados); err != nil {
                erroJSON(w, 500, "Erro ao carregar imagens do relatório")
                return
        }
        u := auth.UsuarioDoCtx(r)
        re := report.Montar(report.Entrada{Diag: d, GeradoPor: u.Nome})
        var conteudo []byte
        var mime string
        switch formato {
        case "pdf":
                conteudo, err = report.RenderPDF(re)
                mime = "application/pdf"
        case "docx":
                conteudo, err = report.RenderDOCX(re)
                mime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
        default:
                conteudo, err = report.RenderHTML(re)
                mime = "text/html; charset=utf-8"
        }
        if err != nil {
                erroJSON(w, 500, "Erro ao gerar relatório: "+err.Error())
                return
        }
        disposicao := "attachment"
        if formato == "html" {
                disposicao = "inline"
        }
        w.Header().Set("Content-Type", mime)
        w.Header().Set("Content-Disposition", disposicao+`; filename="`+nomeArquivo(re, formato)+`"`)
        _, _ = w.Write(conteudo)
}

func (s *Servidor) relatorioPDF(w http.ResponseWriter, r *http.Request) {
        s.enviarRelatorio(w, r, "pdf")
}

func (s *Servidor) relatorioDOCX(w http.ResponseWriter, r *http.Request) {
        s.enviarRelatorio(w, r, "docx")
}

func (s *Servidor) relatorioHTML(w http.ResponseWriter, r *http.Request) {
        s.enviarRelatorio(w, r, "html")
}
