// Package db gerencia a conexão e o schema no PostgreSQL.
package db

import (
        "context"
        "fmt"
        "log"
        "time"

        "github.com/jackc/pgx/v5/pgxpool"
)

// Conectar abre um pool de conexões com o PostgreSQL.
func Conectar(dsn string) (*pgxpool.Pool, error) {
        cfg, err := pgxpool.ParseConfig(dsn)
        if err != nil {
                return nil, err
        }
        cfg.MaxConns = 10
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        pool, err := pgxpool.NewWithConfig(ctx, cfg)
        if err != nil {
                return nil, err
        }
        if err := pool.Ping(ctx); err != nil {
                return nil, err
        }
        return pool, nil
}

// schemaBase reúne todo o DDL do aplicativo em formato idempotente
// (CREATE ... IF NOT EXISTS / ALTER ... ADD COLUMN IF NOT EXISTS). Ele é
// reaplicado em toda inicialização do binário, de modo que cada deploy ajuste o
// banco que já está em produção: tabelas, colunas e índices ausentes são
// recriados sem apagar nenhum dado existente.
const schemaBase = `
CREATE TABLE IF NOT EXISTS escolas (
        id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        nome      TEXT NOT NULL,
        nte       TEXT NOT NULL DEFAULT '',
        criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS usuarios (
        id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        username    TEXT NOT NULL UNIQUE,
        senha_hash  TEXT NOT NULL,
        nome        TEXT NOT NULL,
        papel       TEXT NOT NULL CHECK (papel IN ('admin','preenchador')),
        escola_id   UUID REFERENCES escolas(id) ON DELETE SET NULL,
        criado_em   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessoes (
        token      TEXT PRIMARY KEY,
        usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
        criada_em  TIMESTAMPTZ NOT NULL DEFAULT now(),
        expira_em  TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS fotos (
        id            TEXT PRIMARY KEY,
        arquivo       TEXT NOT NULL,
        nome_original TEXT NOT NULL,
        mime          TEXT NOT NULL,
        bytes         BIGINT NOT NULL,
        enviado_por   UUID REFERENCES usuarios(id) ON DELETE SET NULL,
        criado_em     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS diagnosticos (
        id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        escola_id      UUID NOT NULL REFERENCES escolas(id) ON DELETE CASCADE,
        status         TEXT NOT NULL DEFAULT 'rascunho' CHECK (status IN ('rascunho','enviado')),
        dados          JSONB NOT NULL DEFAULT '{}'::jsonb,
        criado_por     UUID REFERENCES usuarios(id) ON DELETE SET NULL,
        atualizado_por UUID REFERENCES usuarios(id) ON DELETE SET NULL,
        criado_em      TIMESTAMPTZ NOT NULL DEFAULT now(),
        atualizado_em  TIMESTAMPTZ NOT NULL DEFAULT now(),
        enviado_em     TIMESTAMPTZ
);

-- Bases antigas: garante que toda coluna esperada pelo binário exista.
ALTER TABLE escolas ADD COLUMN IF NOT EXISTS nte TEXT NOT NULL DEFAULT '';
ALTER TABLE escolas ADD COLUMN IF NOT EXISTS criado_em TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE usuarios ADD COLUMN IF NOT EXISTS escola_id UUID REFERENCES escolas(id) ON DELETE SET NULL;
ALTER TABLE usuarios ADD COLUMN IF NOT EXISTS criado_em TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE diagnosticos ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'rascunho';
ALTER TABLE diagnosticos ADD COLUMN IF NOT EXISTS dados JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE diagnosticos ADD COLUMN IF NOT EXISTS criado_por UUID REFERENCES usuarios(id) ON DELETE SET NULL;
ALTER TABLE diagnosticos ADD COLUMN IF NOT EXISTS atualizado_por UUID REFERENCES usuarios(id) ON DELETE SET NULL;
ALTER TABLE diagnosticos ADD COLUMN IF NOT EXISTS criado_em TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE diagnosticos ADD COLUMN IF NOT EXISTS atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE diagnosticos ADD COLUMN IF NOT EXISTS enviado_em TIMESTAMPTZ;
ALTER TABLE fotos ADD COLUMN IF NOT EXISTS enviado_por UUID REFERENCES usuarios(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_diag_rascunho_unico
        ON diagnosticos (escola_id) WHERE status = 'rascunho';

CREATE INDEX IF NOT EXISTS idx_diag_escola ON diagnosticos (escola_id);
CREATE INDEX IF NOT EXISTS idx_sessoes_expira ON sessoes (expira_em);
`

// versaoSchema identifica o schema publicado neste binário. O valor é gravado em
// schema_migrations a cada deploy para auditoria das bases em produção.
const versaoSchema = 4

type migracao struct {
        version int
        sql     string
}

// migracoes guarda passos que vão além de DDL idempotente (por exemplo,
// transformar dados existentes), aplicados em ordem de versão. O schema em si
// vive em schemaBase e é reaplicado em todo boot.
var migracoes = []migracao{}

// Migrar aplica, em ordem, as versões de schema ainda não registradas.
func Migrar(pool *pgxpool.Pool) error {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
        defer cancel()

        tx, err := pool.Begin(ctx)
        if err != nil {
                return err
        }
        defer tx.Rollback(ctx)

        if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(641928301)`); err != nil {
                return err
        }
        if _, err := tx.Exec(ctx, `
                CREATE TABLE IF NOT EXISTS schema_migrations (
                        version INTEGER PRIMARY KEY,
                        applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
                )`); err != nil {
                return err
        }

        // O schema completo (idempotente) é reaplicado em toda inicialização:
        // assim o banco do ambiente de deploy sempre fica alinhado ao binário
        // publicado, mesmo que uma versão anterior tenha falhado no meio.
        if _, err := tx.Exec(ctx, schemaBase); err != nil {
                return fmt.Errorf("garantir schema base: %w", err)
        }

        var aplicada int
        if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&aplicada); err != nil {
                return err
        }
        for _, m := range migracoes {
                if m.version <= aplicada {
                        continue
                }
                if _, err := tx.Exec(ctx, m.sql); err != nil {
                        return fmt.Errorf("migração %d: %w", m.version, err)
                }
                if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version); err != nil {
                        return err
                }
                aplicada = m.version
        }
        if aplicada < versaoSchema {
                if _, err := tx.Exec(ctx, `
                        INSERT INTO schema_migrations (version) VALUES ($1)
                        ON CONFLICT (version) DO NOTHING`, versaoSchema); err != nil {
                        return err
                }
                aplicada = versaoSchema
        }
        if err := tx.Commit(ctx); err != nil {
                return err
        }
        log.Printf("Banco de dados ajustado (schema na versão %d).", aplicada)
        return nil
}
