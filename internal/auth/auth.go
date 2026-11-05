// Package auth implementa login, sessões em banco e o middleware de autenticação.
package auth

import (
        "context"
        "crypto/rand"
        "encoding/hex"
        "fmt"
        "log"
        "net/http"
        "strings"
        "time"

        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgxpool"
        "golang.org/x/crypto/bcrypt"

        "diagnostico-escolar/internal/model"
)

const nomeCookie = "diag_sessao"
const duracaoSessao = 7 * 24 * time.Hour

type chaveCtx struct{}

// UsuarioDoCtx devolve o usuário autenticado da requisição.
func UsuarioDoCtx(r *http.Request) *model.Usuario {
        u, _ := r.Context().Value(chaveCtx{}).(*model.Usuario)
        return u
}

// Hash gera o hash bcrypt de uma senha.
func Hash(senha string) (string, error) {
        b, err := bcrypt.GenerateFromPassword([]byte(senha), bcrypt.DefaultCost)
        return string(b), err
}

func novoToken() string {
        b := make([]byte, 32)
        if _, err := rand.Read(b); err != nil {
                panic(err)
        }
        return hex.EncodeToString(b)
}

// Login valida as credenciais e cria uma nova sessão.
func Login(pool *pgxpool.Pool, username, senha string) (model.Usuario, string, error) {
        var u model.Usuario
        var escolaID *string
        err := pool.QueryRow(context.Background(),
                `SELECT u.id::text, u.username, u.senha_hash, u.nome, u.papel, u.escola_id::text, COALESCE(e.nome, ''), u.criado_em
                 FROM usuarios u LEFT JOIN escolas e ON e.id = u.escola_id
                 WHERE u.username = $1`,
                username).Scan(&u.ID, &u.Username, &u.SenhaHash, &u.Nome, &u.Papel, &escolaID, &u.EscolaNome, &u.CriadoEm)
        if err == pgx.ErrNoRows {
                return u, "", errCredenciais
        }
        if err != nil {
                return u, "", err
        }
        if bcrypt.CompareHashAndPassword([]byte(u.SenhaHash), []byte(senha)) != nil {
                return u, "", errCredenciais
        }
        token := novoToken()
        _, err = pool.Exec(context.Background(),
                `INSERT INTO sessoes (token, usuario_id, expira_em) VALUES ($1, $2, $3)`,
                token, u.ID, time.Now().Add(duracaoSessao))
        if err != nil {
                return u, "", err
        }
        u.EscolaID = escolaID
        return u, token, nil
}

var errCredenciais = erroLogin{}

type erroLogin struct{}

func (erroLogin) Error() string { return "credenciais inválidas" }

// CredenciaisInvalidas informa se o erro veio de usuário/senha errados.
func CredenciaisInvalidas(err error) bool {
        _, ok := err.(erroLogin)
        return ok
}

// Logout encerra a sessão atual.
func Logout(pool *pgxpool.Pool, token string) {
        _, _ = pool.Exec(context.Background(), `DELETE FROM sessoes WHERE token = $1`, token)
}

// TokenDoCookie extrai o token de sessão do cookie.
func TokenDoCookie(r *http.Request) string {
        c, err := r.Cookie(nomeCookie)
        if err != nil {
                return ""
        }
        return c.Value
}

// TokenDaRequisição extrai o token do cabeçalho Authorization (Bearer)
// ou, na ausência dele, do cookie de sessão. O Bearer é essencial quando
// o app roda embutido em iframes, onde navegadores bloqueiam cookies de
// terceiros (ex.: painéis de preview).
func TokenDaRequisicao(r *http.Request) string {
        if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
                if t := strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")); t != "" {
                        return t
                }
        }
        return TokenDoCookie(r)
}

// MiddlewareAuth exige sessão válida e injeta o usuário no contexto.
func MiddlewareAuth(pool *pgxpool.Pool) func(http.Handler) http.Handler {
        return func(proximo http.Handler) http.Handler {
                return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                        token := TokenDaRequisicao(r)
                        if token == "" {
                                http.Error(w, `{"erro":"Não autenticado"}`, http.StatusUnauthorized)
                                return
                        }
                        var u model.Usuario
                        var escolaID *string
                        err := pool.QueryRow(r.Context(),
                                `SELECT u.id::text, u.username, u.nome, u.papel, u.escola_id::text, COALESCE(e.nome, ''), u.criado_em
                                 FROM sessoes s
                                 JOIN usuarios u ON u.id = s.usuario_id
                                 LEFT JOIN escolas e ON e.id = u.escola_id
                                 WHERE s.token = $1 AND s.expira_em > now()`,
                                token).Scan(&u.ID, &u.Username, &u.Nome, &u.Papel, &escolaID, &u.EscolaNome, &u.CriadoEm)
                        if err != nil {
                                http.Error(w, `{"erro":"Sessão expirada ou inválida"}`, http.StatusUnauthorized)
                                return
                        }
                        u.EscolaID = escolaID
                        proximo.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), chaveCtx{}, &u)))
                })
        }
}

// LimpezaSessoes remove periodicamente sessões expiradas.
func LimpezaSessoes(pool *pgxpool.Pool) {
        ticker := time.NewTicker(1 * time.Hour)
        for range ticker.C {
                _, err := pool.Exec(context.Background(), `DELETE FROM sessoes WHERE expira_em < now()`)
                if err != nil {
                        log.Printf("limpeza de sessões: %v", err)
                }
        }
}

// SeedAdmin cria o usuário administrador inicial se a tabela estiver vazia.
func SeedAdmin(pool *pgxpool.Pool, username, senha string) error {
        var n int
        if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM usuarios`).Scan(&n); err != nil {
                return err
        }
        if n > 0 {
                return nil
        }
        if senha == "" {
                return fmt.Errorf("defina ADMIN_SENHA antes de iniciar com o banco vazio")
        }
        hash, err := Hash(senha)
        if err != nil {
                return err
        }
        _, err = pool.Exec(context.Background(),
                `INSERT INTO usuarios (username, senha_hash, nome, papel) VALUES ($1, $2, $3, 'admin')`,
                username, hash, "Administrador")
        if err != nil {
                return err
        }
        log.Printf("Usuário administrador inicial criado: %s", username)
        return nil
}
