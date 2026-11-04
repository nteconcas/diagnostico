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

	"diagnostico-escolar/internal/api"
	"diagnostico-escolar/internal/auth"
	"diagnostico-escolar/internal/db"
	"diagnostico-escolar/internal/fsstore"
)

func ambiente(chave, padrao string) string {
	if v := os.Getenv(chave); v != "" {
		return v
	}
	return padrao
}

func main() {
	porta := ambiente("PORT", "8080")
	dsn := ambiente("DATABASE_URL", "postgres://diag:diag@localhost:5433/diagnostico?sslmode=disable")
	dirDados := ambiente("DATA_DIR", "data")

	pool, err := db.Conectar(dsn)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao PostgreSQL: %v", err)
	}
	if err := db.Migrar(pool); err != nil {
		log.Fatalf("Erro ao aplicar o schema: %v", err)
	}
	if err := auth.SeedAdmin(pool, ambiente("ADMIN_USUARIO", "admin"), ambiente("ADMIN_SENHA", "admin123")); err != nil {
		log.Fatalf("Erro ao criar o usuário administrador inicial: %v", err)
	}

	loja := &fsstore.Loja{Dir: dirDados}
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
