# Formulário de Diagnóstico Escolar

Aplicação web em **Go** (API + frontend React embutidos em um único binário) para coleta do
**Diagnóstico Escolar** da rede: identificação da unidade, contatos e GPS, servidores,
infraestrutura (com registro fotográfico), espaços, projetos e ofertas de cursos — com
**relatórios exportáveis em PDF, DOCX e HTML**.

- **Backend:** Go 1.23+ · API REST (net/http) · PostgreSQL (pgx) · sessões em banco · bcrypt
- **Frontend:** React 18 + Vite (SPA) — compilado e embutido no binário via `go:embed`
- **Fotos:** armazenadas em bucket privado Cloudflare R2, organizadas por slug da escola;
  miniatura, versão média e imagem 4:3 para relatórios são geradas no servidor
- **Relatórios:** PDF (gofpdf), DOCX (OOXML nativo) e HTML autossuficiente (imagens em base64)
- **Acesso:** login por usuário/senha com papéis `admin` (gerencia escolas e usuários) e
  `preenchador` (preenche diagnósticos)

---

## 1. Requisitos

| Componente | Versão |
|---|---|
| Go | 1.23 ou superior |
| Node.js + npm | 18+ (apenas para compilar o frontend) |
| PostgreSQL | 14+ (testado no 16/17) |

## 2. Estrutura

```
diagnostico-escolar/
├── main.go                     # servidor (API + web) e inicialização
├── embed.go                    # serve o frontend compilado (go:embed)
├── internal/
│   ├── model/                  # entidades e o formato dos dados do formulário
│   ├── db/                     # conexão e schema (migrations automáticas)
│   ├── auth/                   # login, sessões, middleware, seed do admin
│   ├── fsstore/                # upload de fotos + miniaturas + EXIF
│   ├── api/                    # handlers REST (escolas, diagnósticos, fotos, usuários)
│   └── report/                 # relatórios: dados, HTML, PDF, DOCX
├── frontend/                   # SPA React (src) e dist/ compilado
├── deploy/
│   ├── diagnostico-escolar.service   # unit systemd
│   └── docker-compose.yml            # PostgreSQL para produção (opcional)
└── diagnostico-escolar         # binário já compilado (linux/amd64)
```

## Configuração de ambiente

Copie `.env.example` para `.env` e preencha as credenciais do PostgreSQL, do administrador
inicial e do Cloudflare R2. O bucket deve permanecer privado; crie uma chave S3 limitada a
esse bucket. O app usa o endpoint padrão `https://<ACCOUNT_ID>.r2.cloudflarestorage.com`;
`R2_ENDPOINT` só é necessário se o endpoint for personalizado. O arquivo `.env` não deve ser
versionado nem exposto publicamente.

O `.env.example` é um modelo: o binário não carrega arquivos `.env` por conta própria. O
Docker Compose e a unit systemd fornecidos carregam as variáveis por configuração. Para
execução manual no Linux, exporte as variáveis antes de iniciar o binário.

## 3. Compilação

```bash
# frontend (uma vez, ou a cada alteração)
cd frontend
npm install
npm run build          # gera frontend/dist

# binário único
cd ..
go build -o diagnostico-escolar .
```

O binário final **não precisa de Node, Go nem arquivos externos** — frontend e migrations
estão embutidos.

O `Dockerfile` faz essas etapas automaticamente em múltiplos estágios. No Dokploy, configure
o diretório raiz como a raiz deste repositório e selecione esse `Dockerfile`; cada build
compilará os fontes do commit implantado, sem usar o binário `diagnostico-escolar` existente
no repositório.

## 4. Banco de dados

Crie o banco e o usuário (em um PostgreSQL já instalado):

```sql
CREATE USER diagnostico WITH PASSWORD 'TROQUE_ESSA_SENHA';
CREATE DATABASE diagnostico OWNER diagnostico;
```

Ou use o Docker Compose incluído:

```bash
docker compose --env-file .env -f deploy/docker-compose.yml up -d
```

As tabelas são criadas automaticamente na primeira execução do app
(`usuarios`, `sessoes`, `escolas`, `fotos`, `diagnosticos`).

Ao iniciar, o app aplica automaticamente as migrações pendentes e registra suas versões em
`schema_migrations`. Alterações futuras no banco devem ser adicionadas como uma nova versão
em `internal/db/db.go`; não basta alterar uma migração que já foi aplicada.

## 5. Execução

```bash
set -a
source .env
set +a

./diagnostico-escolar
```

| Variável | Padrão | Descrição |
|---|---|---|
| `PORT` | `8080` | porta HTTP |
| `DATABASE_URL` | `postgres://diag:diag@localhost:5433/diagnostico?sslmode=disable` | conexão PostgreSQL |
| `DATA_DIR` | `data` | pasta das fotos (`uploads/`) — **inclua no backup** |
| `ADMIN_USUARIO` | `admin` | usuário administrador criado no 1º boot |
| `ADMIN_SENHA` | obrigatória em banco vazio | senha inicial do administrador |
| `R2_ACCOUNT_ID` | — | ID da conta Cloudflare |
| `R2_BUCKET` | — | nome do bucket privado |
| `R2_ACCESS_KEY_ID` | — | access key S3 do R2 |
| `R2_SECRET_ACCESS_KEY` | — | secret key S3 do R2 |
| `R2_ENDPOINT` | endpoint padrão da conta | endpoint S3 personalizado (opcional) |
| `TZ` | do sistema | fuso horário exibido nos relatórios |

**Primeiro acesso:** entre com as credenciais definidas em `ADMIN_USUARIO` e `ADMIN_SENHA`.

## 6. Instalação como serviço (systemd)

```bash
# 1. binário, configuração e dados no servidor
sudo mkdir -p /opt/diagnostico-escolar/data
sudo cp diagnostico-escolar /opt/diagnostico-escolar/
sudo cp .env /opt/diagnostico-escolar/.env
sudo chmod 600 /opt/diagnostico-escolar/.env
sudo useradd -r -s /usr/sbin/nologin diagnostico
sudo chown -R diagnostico:diagnostico /opt/diagnostico-escolar

# 2. instale a unit systemd
sudo cp deploy/diagnostico-escolar.service /etc/systemd/system/

# 3. ative
sudo systemctl daemon-reload
sudo systemctl enable --now diagnostico-escolar
sudo systemctl status diagnostico-escolar
```

Logs: `journalctl -u diagnostico-escolar -f`.

Configure um proxy reverso com HTTPS para encaminhar a aplicação em `127.0.0.1:8080`.

## 7. Backup

```bash
# dados do banco
pg_dump -U diagnostico diagnostico > backup-diagnostico.sql

# cache local opcional; as imagens originais e variantes ficam no bucket R2
tar czf backup-fotos-cache.tgz /opt/diagnostico-escolar/data/uploads
```

Restore: restaure o banco e mantenha o bucket R2; o cache local pode ser reconstruído a partir
dele. Garanta também que as credenciais R2 do `.env` estejam disponíveis.

## 8. Uso no dia a dia

1. **Admin → Gerenciar escolas:** cadastra as unidades (nome + NTE).
2. **Admin → Usuários:** cria contas para os responsáveis por cada escola/NTE.
3. **Novo diagnóstico:** seleciona a escola → o rascunho é criado e **salvo automaticamente**
   a cada alteração (indicador no topo).
4. O formulário segue a estrutura institucional: identificação (escola, responsável, foto do
   gestor e **foto da fachada — capa do relatório**), endereço/contatos por setor, **GPS**
   (botão de captura + link do mapa no relatório), **1. Servidores** (com métricas de total e
   distribuição efetivos × ACT), **2. Infraestrutura** (salas, equipamentos, fotos antes/atual
   da gestão 2023/2027 — mínimo 3 de cada — e laboratórios), **3. Espaços**, **4. Projetos**
   (até 2500 caracteres e 3 fotos por projeto) e **5. Ofertas de cursos**.
5. **Enviar formulário:** só habilita quando os campos obrigatórios estão completos (a lista
   de pendências é mostrada). Depois de enviado, o registro fica somente leitura e o botão de
   exportação oferece **PDF, DOCX e HTML**.
6. O painel mostra o status por escola (não iniciado / rascunho / enviado), quem atualizou e
   permite reabrir um rascunho ou exportar relatórios de qualquer envio.

## 9. Notas técnicas

- Fotos aceitas: JPG, PNG, WEBP e GIF (até 20 MB); convertidas para JPEG no servidor.
- O rascunho é único por escola (índice parcial no PostgreSQL) — abrir o diagnóstico de uma
  escola com rascunho existente continua o mesmo preenchimento.
- Diagnósticos enviados preservam histórico; admins podem descartá-los pela API.
- O relatório HTML é autossuficiente (imagens embutidas) — abre offline em qualquer navegador
  e possui botão "Imprimir / Salvar PDF".
- Sessões expiram em 7 dias e são limpas automaticamente.
