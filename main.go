// Formulário de Diagnóstico Escolar — servidor único em Go (API + frontend embutido).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"diagnostico-escolar/internal/api"
	"diagnostico-escolar/internal/auth"
	"diagnostico-escolar/internal/db"
	"diagnostico-escolar/internal/fsstore"
	"diagnostico-escolar/internal/r2store"
)

func ambiente(chave, padrao string) string {
	if v := os.Getenv(chave); v != "" {
		return v
	}
	return padrao
}

// abrirBanco conecta e aplica o schema, repetindo a operação enquanto o
// PostgreSQL não estiver pronto. Em um deploy o container da aplicação pode
// subir antes do banco aceitar conexões; assim o schema é ajustado com
// segurança em vez de o processo encerrar na primeira tentativa.
func abrirBanco(dsn string) (*pgxpool.Pool, error) {
	const tentativas = 12
	var ultimo error
	for tentativa := 1; tentativa <= tentativas; tentativa++ {
		pool, err := db.Conectar(dsn)
		if err == nil {
			if err = db.Migrar(pool); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		ultimo = err
		if tentativa < tentativas {
			log.Printf("Banco indisponível (tentativa %d/%d): %v — nova tentativa em 5s", tentativa, tentativas, err)
			time.Sleep(5 * time.Second)
		}
	}
	return nil, ultimo
}

func main() {
	porta := ambiente("PORT", "8080")
	dsn := ambiente("DATABASE_URL", "postgres://diag:diag@localhost:5433/diagnostico?sslmode=disable")
	dirDados := ambiente("DATA_DIR", "data")
	clienteR2, err := r2store.NovoDoAmbiente()
	if err != nil {
		log.Fatalf("Configuração do Cloudflare R2 inválida: %v", err)
	}

	pool, err := abrirBanco(dsn)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao PostgreSQL e ajustar o schema: %v", err)
	}
	if err := auth.SeedAdmin(pool, ambiente("ADMIN_USUARIO", "admin"), os.Getenv("ADMIN_SENHA")); err != nil {
		log.Fatalf("Erro ao criar o usuário administrador inicial: %v", err)
	}

	loja := &fsstore.Loja{Dir: dirDados, R2: clienteR2}
	if err := os.MkdirAll(loja.UploadsDir(), 0o755); err != nil {
		log.Fatalf("Erro ao criar a pasta de uploads: %v", err)
	}

	mux := api.Novo(pool, loja)
	RegistrarWeb(mux, loja.UploadsDir())
	go auth.LimpezaSessoes(pool)

	servidor := &http.Server{
		Addr:              ":" + porta,
		Handler:           api.LogMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("┌──────────────────────────────────────────────┐")
		log.Printf("│  Formulário de Diagnóstico Escolar           │")
		log.Printf("│  Servidor em http://localhost:%s            │", porta)
		log.Printf("└──────────────────────────────────────────────┘")
		if err := servidor.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro no servidor: %v", err)
		}
	}()

	parar := make(chan os.Signal, 1)
	signal.Notify(parar, os.Interrupt, syscall.SIGTERM)
	<-parar
	log.Println("Encerrando o servidor...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = servidor.Shutdown(ctx)
}
