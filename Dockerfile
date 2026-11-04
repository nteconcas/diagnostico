# Dockerfile — Formulário de Diagnóstico Escolar
# Constrói a imagem do app a partir do binário Linux já compilado neste pacote.
FROM debian:bookworm-slim

WORKDIR /app
COPY diagnostico-escolar /app/diagnostico-escolar
RUN chmod +x /app/diagnostico-escolar && mkdir -p /dados

ENV PORT=8080 \
    DATA_DIR=/dados \
    TZ=America/Sao_Paulo

VOLUME /dados
EXPOSE 8080

ENTRYPOINT ["/app/diagnostico-escolar"]
