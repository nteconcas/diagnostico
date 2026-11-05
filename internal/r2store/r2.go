package r2store

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const regiao = "auto"

// Cliente acessa um bucket R2 pela API S3.
type Cliente struct {
	endpoint  *url.URL
	bucket    string
	accessKey string
	secretKey string
	http      *http.Client
}

func Novo(accountID, bucket, accessKey, secretKey, endpoint string) (*Cliente, error) {
	accountID = strings.TrimSpace(accountID)
	bucket = strings.TrimSpace(bucket)
	accessKey = strings.TrimSpace(accessKey)
	secretKey = strings.TrimSpace(secretKey)
	endpoint = strings.TrimSpace(endpoint)
	var faltando []string
	for _, variavel := range []struct {
		nome  string
		valor string
	}{
		{"R2_ACCOUNT_ID", accountID},
		{"R2_BUCKET", bucket},
		{"R2_ACCESS_KEY_ID", accessKey},
		{"R2_SECRET_ACCESS_KEY", secretKey},
	} {
		if variavel.valor == "" {
			faltando = append(faltando, variavel.nome)
		}
	}
	if len(faltando) > 0 {
		return nil, fmt.Errorf("variáveis ausentes no ambiente do app: %s", strings.Join(faltando, ", "))
	}
	if endpoint == "" {
		endpoint = "https://" + accountID + ".r2.cloudflarestorage.com"
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.Scheme != "https" {
		return nil, fmt.Errorf("R2_ENDPOINT inválido")
	}
	return &Cliente{
		endpoint:  u,
		bucket:    bucket,
		accessKey: accessKey,
		secretKey: secretKey,
		http:      &http.Client{Timeout: 2 * time.Minute},
	}, nil
}

func NovoDoAmbiente() (*Cliente, error) {
	bucket := os.Getenv("R2_BUCKET")
	if strings.TrimSpace(bucket) == "" {
		bucket = os.Getenv("R2_BUCKET_NAME")
	}
	return Novo(
		os.Getenv("R2_ACCOUNT_ID"),
		bucket,
		os.Getenv("R2_ACCESS_KEY_ID"),
		os.Getenv("R2_SECRET_ACCESS_KEY"),
		os.Getenv("R2_ENDPOINT"),
	)
}

func (c *Cliente) Enviar(ctx context.Context, chave string, conteudo []byte, tipo string) error {
	resp, err := c.requisicao(ctx, http.MethodPut, chave, conteudo, tipo)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return validarResposta(resp)
}

func (c *Cliente) Obter(ctx context.Context, chave string) ([]byte, error) {
	resp, err := c.requisicao(ctx, http.MethodGet, chave, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := validarResposta(resp); err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

func (c *Cliente) Excluir(ctx context.Context, chave string) error {
	resp, err := c.requisicao(ctx, http.MethodDelete, chave, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return validarResposta(resp)
}

func (c *Cliente) requisicao(ctx context.Context, metodo, chave string, corpo []byte, tipo string) (*http.Response, error) {
	u := *c.endpoint
	u.Path = strings.TrimRight(u.Path, "/") + "/" + c.bucket + "/" + strings.TrimLeft(chave, "/")
	u.RawPath = ""
	if u.RawQuery != "" {
		return nil, fmt.Errorf("R2_ENDPOINT não pode conter query string")
	}

	hashCorpo := sha256.Sum256(corpo)
	hashHex := hex.EncodeToString(hashCorpo[:])
	agora := time.Now().UTC()
	data := agora.Format("20060102")
	dataHora := agora.Format("20060102T150405Z")
	req, err := http.NewRequestWithContext(ctx, metodo, u.String(), bytes.NewReader(corpo))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Amz-Content-Sha256", hashHex)
	req.Header.Set("X-Amz-Date", dataHora)
	if tipo != "" {
		req.Header.Set("Content-Type", tipo)
	}

	const cabecalhosAssinados = "host;x-amz-content-sha256;x-amz-date"
	cabecalhosCanonicos := "host:" + req.URL.Host + "\n" +
		"x-amz-content-sha256:" + hashHex + "\n" +
		"x-amz-date:" + dataHora + "\n"
	requisicaoCanonica := metodo + "\n" + req.URL.EscapedPath() + "\n" + req.URL.RawQuery + "\n" +
		cabecalhosCanonicos + "\n" + cabecalhosAssinados + "\n" + hashHex
	escopo := data + "/" + regiao + "/s3/aws4_request"
	hashRequisicao := sha256.Sum256([]byte(requisicaoCanonica))
	textoAssinado := "AWS4-HMAC-SHA256\n" + dataHora + "\n" + escopo + "\n" + hex.EncodeToString(hashRequisicao[:])
	chaveData := hmacSHA256([]byte("AWS4"+c.secretKey), data)
	chaveRegiao := hmacSHA256(chaveData, regiao)
	chaveServico := hmacSHA256(chaveRegiao, "s3")
	chaveAssinatura := hmacSHA256(chaveServico, "aws4_request")
	assinatura := hex.EncodeToString(hmacSHA256(chaveAssinatura, textoAssinado))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+c.accessKey+"/"+escopo+
		", SignedHeaders="+cabecalhosAssinados+", Signature="+assinatura)
	return c.http.Do(req)
}

func hmacSHA256(chave []byte, valor string) []byte {
	h := hmac.New(sha256.New, chave)
	_, _ = h.Write([]byte(valor))
	return h.Sum(nil)
}

func validarResposta(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	detalhe, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("R2 respondeu HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(detalhe)))
}