package api

import (
        "net/http"
        "time"

        "diagnostico-escolar/internal/auth"
        "diagnostico-escolar/internal/model"
)

type corpoLogin struct {
        Username string `json:"username"`
        Senha    string `json:"senha"`
}

// respostaLogin devolve o usuário junto com o token de sessão. O token
// também é entregue no corpo porque o app pode rodar em contextos onde o
// cookie de sessão é bloqueado (iframes de preview), e o frontend o guarda
// em localStorage para enviar como Authorization: Bearer.
type respostaLogin struct {
        model.Usuario
        Token string `json:"token"`
}

func (s *Servidor) login(w http.ResponseWriter, r *http.Request) {
        var corpo corpoLogin
        if !readJSON(w, r, &corpo) {
                return
        }
        if corpo.Username == "" || corpo.Senha == "" {
                erroJSON(w, 400, "Informe usuário e senha")
                return
        }
        usuario, token, err := auth.Login(s.pool, corpo.Username, corpo.Senha)
        if err != nil {
                erroJSON(w, 401, "Usuário ou senha inválidos")
                return
        }
        http.SetCookie(w, &http.Cookie{
                Name:     "diag_sessao",
                Value:    token,
                Path:     "/",
                HttpOnly: true,
                SameSite: http.SameSiteLaxMode,
                MaxAge:   int((7 * 24 * time.Hour).Seconds()),
        })
        writeJSON(w, 200, respostaLogin{Usuario: usuario, Token: token})
}

func (s *Servidor) logout(w http.ResponseWriter, r *http.Request) {
        if token := auth.TokenDaRequisicao(r); token != "" {
                auth.Logout(s.pool, token)
        }
        http.SetCookie(w, &http.Cookie{Name: "diag_sessao", Value: "", Path: "/", MaxAge: -1})
        writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Servidor) me(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, 200, auth.UsuarioDoCtx(r))
}
