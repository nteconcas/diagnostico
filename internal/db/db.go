// Package db gerencia a conexão e o schema no PostgreSQL.
package db

import (
        "context"
        "fmt"
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

const schemaEscolas = `
CREATE TABLE IF NOT EXISTS escolas (
                id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                nome      TEXT NOT NULL,
                nte       TEXT NOT NULL DEFAULT '',
                criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

const schema = `
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_diag_rascunho_unico
        ON diagnosticos (escola_id) WHERE status = 'rascunho';

CREATE INDEX IF NOT EXISTS idx_diag_escola ON diagnosticos (escola_id);
CREATE INDEX IF NOT EXISTS idx_sessoes_expira ON sessoes (expira_em);

-- Migração para bases antigas: vínculo do usuário responsável com a escola.
ALTER TABLE usuarios ADD COLUMN IF NOT EXISTS escola_id UUID REFERENCES escolas(id) ON DELETE SET NULL;
`

type migration struct {
        version int
        schema  string
}

var migrations = []migration{
        {version: 1, schema: schemaEscolas},
        // Repara bancos que registraram a versão 1 quando ela ainda continha todo o schema.
        {version: 2, schema: schemaEscolas},
        {version: 3, schema: schema},
}

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

        // escolas is a prerequisite for the remaining schema, so restore it even
        // when migration metadata claims that its migration has already run.
        if _, err := tx.Exec(ctx, schemaEscolas); err != nil {
                return fmt.Errorf("garantir tabela escolas: %w", err)
        }

        var aplicada int
        if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&aplicada); err != nil {
                return err
        }
        for _, m := range migrations {
                if m.version <= aplicada {
                        continue
                }
                if _, err := tx.Exec(ctx, m.schema); err != nil {
                        return fmt.Errorf("migração de schema %d: %w", m.version, err)
                }
                if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version); err != nil {
                        return err
                }
        }
        return tx.Commit(ctx)
}
