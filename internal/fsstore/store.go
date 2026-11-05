// Package fsstore grava fotos no Cloudflare R2 e mantém um cache local de variantes.
package fsstore

import (
        "bytes"
        "context"
        "crypto/rand"
        "encoding/hex"
        "fmt"
        "image"
        "image/color"
        _ "image/gif"
        "image/jpeg"
        _ "image/png"
        "io"
        "os"
        "path"
        "path/filepath"
        "regexp"
        "strings"
        "time"

        "github.com/rwcarlsen/goexif/exif"
        xdraw "golang.org/x/image/draw"
        _ "golang.org/x/image/webp"

        "diagnostico-escolar/internal/model"
        "diagnostico-escolar/internal/r2store"
)

// Loja é o diretório base de dados do app.
type Loja struct {
        Dir string
        R2  *r2store.Cliente
}

// UploadsDir devolve a pasta de arquivos enviados.
func (l *Loja) UploadsDir() string {
        dir := filepath.Join(l.Dir, "uploads")
        _ = os.MkdirAll(dir, 0o755)
        return dir
}

// Caminho devolve o caminho absoluto de um arquivo dentro de uploads.
func (l *Loja) Caminho(arquivo string) string {
        return filepath.Join(l.UploadsDir(), arquivo)
}

const (
        largThumb  = 480
        largMedia  = 1100
        largRel    = 1200
        altRel     = 900
        qualidade  = 82
        maxIMO     = 20 << 20 // 20 MB por foto
)

var caracteresSlug = regexp.MustCompile(`[^a-z0-9]+`)

// SalvarImagem corrige a orientação EXIF, gera variantes e envia os objetos ao R2.
func (l *Loja) SalvarImagem(ctx context.Context, r io.Reader, nomeOriginal, nomeEscola string) (model.Foto, int64, string, error) {
        if l.R2 == nil {
                return model.Foto{}, 0, "", fmt.Errorf("armazenamento R2 não configurado")
        }
        bruto, err := io.ReadAll(io.LimitReader(r, maxIMO+1))
        if err != nil {
                return model.Foto{}, 0, "", fmt.Errorf("não foi possível ler o arquivo: %w", err)
        }
        if len(bruto) > maxIMO {
                return model.Foto{}, 0, "", fmt.Errorf("arquivo maior que 20 MB")
        }
        config, _, err := image.DecodeConfig(bytes.NewReader(bruto))
        if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width) > 60000000/int64(config.Height) {
                return model.Foto{}, 0, "", fmt.Errorf("imagem inválida ou com dimensões muito grandes")
        }
        src, formato, err := image.Decode(bytes.NewReader(bruto))
        if err != nil {
                return model.Foto{}, 0, "", fmt.Errorf("arquivo de imagem inválido (use JPG, PNG ou WEBP)")
        }
        src = corrigirOrientacao(bruto, src)

        id := novoID()
        ext := extensaoImagem(formato)
        prefixo := slugEscola(nomeEscola) + "/" + id
        arquivo := prefixo + "/original." + ext
        miniatura, err := codificarJPEG(escalar(src, largThumb), qualidade)
        if err != nil {
                return model.Foto{}, 0, "", err
        }
        media, err := codificarJPEG(escalar(src, largMedia), qualidade)
        if err != nil {
                return model.Foto{}, 0, "", err
        }
        relatorio, err := codificarJPEG(enquadrarRelatorio(src), qualidade)
        if err != nil {
                return model.Foto{}, 0, "", err
        }

        objetos := []struct {
                chave string
                local string
                data  []byte
                tipo  string
        }{
                {arquivo, id + ".orig." + ext, bruto, tipoImagem(formato)},
                {prefixo + "/thumb.jpg", id + ".t.jpg", miniatura, "image/jpeg"},
                {prefixo + "/medium.jpg", id + ".m.jpg", media, "image/jpeg"},
                {prefixo + "/report.jpg", id + ".r.jpg", relatorio, "image/jpeg"},
        }
        for _, objeto := range objetos {
                if err := l.R2.Enviar(ctx, objeto.chave, objeto.data, objeto.tipo); err != nil {
                        for _, enviado := range objetos {
                                if enviado.chave == objeto.chave {
                                        break
                                }
                                _ = l.R2.Excluir(context.Background(), enviado.chave)
                        }
                        return model.Foto{}, 0, "", err
                }
                if err := os.WriteFile(l.Caminho(objeto.local), objeto.data, 0o644); err != nil {
                        for _, enviado := range objetos {
                                _ = l.R2.Excluir(context.Background(), enviado.chave)
                        }
                        return model.Foto{}, 0, "", err
                }
        }
        foto := model.Foto{ID: id, Nome: nomeOriginal}
        return foto, int64(len(bruto)), arquivo, nil
}

// GarantirArquivo baixa do R2 somente quando o cache local não existe.
func (l *Loja) GarantirArquivo(ctx context.Context, chave, local string) error {
        caminho := l.Caminho(local)
        if _, err := os.Stat(caminho); err == nil {
                return nil
        }
        if l.R2 == nil {
                return fmt.Errorf("armazenamento R2 não configurado")
        }
        conteudo, err := l.R2.Obter(ctx, chave)
        if err != nil {
                return err
        }
        return os.WriteFile(caminho, conteudo, 0o644)
}

// PrepararRelatorio garante uma imagem 4:3 para os renderizadores de relatório.
func (l *Loja) PrepararRelatorio(ctx context.Context, id, arquivo string) error {
        local := id + ".r.jpg"
        if _, err := os.Stat(l.Caminho(local)); err == nil {
                return nil
        }
        prefixo := path.Dir(arquivo)
        if prefixo != "." {
                return l.GarantirArquivo(ctx, prefixo+"/report.jpg", local)
        }

        f, err := os.Open(l.Caminho(id + ".m.jpg"))
        if err != nil {
                return err
        }
        src, _, err := image.Decode(f)
        _ = f.Close()
        if err != nil {
                return err
        }
        conteudo, err := codificarJPEG(enquadrarRelatorio(src), qualidade)
        if err != nil {
                return err
        }
        return os.WriteFile(l.Caminho(local), conteudo, 0o644)
}

// Excluir remove o cache local e as variantes armazenadas no bucket.
func (l *Loja) Excluir(id, arquivo string) {
        _ = os.Remove(l.Caminho(arquivo))
        _ = os.Remove(l.Caminho(id+".t.jpg"))
        _ = os.Remove(l.Caminho(id+".m.jpg"))
        _ = os.Remove(l.Caminho(id+".r.jpg"))
        if l.R2 != nil {
                prefixo := filepath.ToSlash(filepath.Dir(arquivo))
                for _, chave := range []string{arquivo, prefixo + "/thumb.jpg", prefixo + "/medium.jpg", prefixo + "/report.jpg"} {
                        _ = l.R2.Excluir(context.Background(), chave)
                }
        }
}

func codificarJPEG(img image.Image, q int) ([]byte, error) {
        var b bytes.Buffer
        if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: q}); err != nil {
                return nil, err
        }
        return b.Bytes(), nil
}

func slugEscola(nome string) string {
        transliteracao := strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "õ", "o", "ô", "o", "ú", "u", "ü", "u", "ç", "c")
        slug := strings.Trim(caracteresSlug.ReplaceAllString(transliteracao.Replace(strings.ToLower(nome)), "-"), "-")
        if slug == "" {
                return "escola"
        }
        return slug
}

func extensaoImagem(formato string) string {
        if formato == "jpeg" {
                return "jpg"
        }
        return formato
}

func tipoImagem(formato string) string {
        if formato == "jpeg" {
                return "image/jpeg"
        }
        return "image/" + formato
}

func enquadrarRelatorio(src image.Image) image.Image {
        b := src.Bounds()
        if b.Dx() <= 0 || b.Dy() <= 0 {
                return image.NewRGBA(image.Rect(0, 0, largRel, altRel))
        }
        escala := min(float64(largRel)/float64(b.Dx()), float64(altRel)/float64(b.Dy()))
        larg := max(1, int(float64(b.Dx())*escala))
        alt := max(1, int(float64(b.Dy())*escala))
        x := (largRel - larg) / 2
        y := (altRel - alt) / 2
        dst := image.NewRGBA(image.Rect(0, 0, largRel, altRel))
        xdraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, xdraw.Src)
        xdraw.CatmullRom.Scale(dst, image.Rect(x, y, x+larg, y+alt), src, b, xdraw.Src, nil)
        return dst
}

// novoID gera um identificador único com crypto/rand.
func novoID() string {
        b := make([]byte, 16)
        if _, err := rand.Read(b); err != nil {
                return fmt.Sprintf("%d", time.Now().UnixNano())
        }
        return hex.EncodeToString(b)
}

// escalar reduz a imagem para caber em largura máxima, sem upscale.
func escalar(src image.Image, largMax int) image.Image {
        b := src.Bounds()
        if b.Dx() <= largMax {
                return src
        }
        nAlt := b.Dy() * largMax / b.Dx()
        dst := image.NewRGBA(image.Rect(0, 0, largMax, nAlt))
        xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
        return dst
}

// sobreBranco achata a transparência sobre fundo branco (JPEG não tem alfa).
func sobreBranco(src image.Image) *image.RGBA {
        b := src.Bounds()
        dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
        xdraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, xdraw.Src)
        xdraw.Draw(dst, dst.Bounds(), src, b.Min, xdraw.Over)
        return dst
}

// corrigirOrientacao aplica a rotação indicada pelo EXIF da câmera.
func corrigirOrientacao(bruto []byte, src image.Image) image.Image {
        var orient = 1
        if x, err := exif.Decode(bytes.NewReader(bruto)); err == nil {
                if tag, err := x.Get(exif.Orientation); err == nil {
                        if n, err := tag.Int(0); err == nil {
                                orient = n
                        }
                }
        }
        switch orient {
        case 3, 4:
                src = girar180(src)
        case 5, 6:
                src = girar90(src)
        case 7, 8:
                src = girar270(src)
        }
        return sobreBranco(src)
}

// girar90 rotaciona 90° no sentido horário.
func girar90(src image.Image) image.Image {
        b := src.Bounds()
        w, h := b.Dx(), b.Dy()
        dst := image.NewRGBA(image.Rect(0, 0, h, w))
        for y := 0; y < h; y++ {
                for x := 0; x < w; x++ {
                        dst.Set(h-1-y, x, src.At(b.Min.X+x, b.Min.Y+y))
                }
        }
        return dst
}

func girar180(src image.Image) image.Image {
        b := src.Bounds()
        w, h := b.Dx(), b.Dy()
        dst := image.NewRGBA(image.Rect(0, 0, w, h))
        for y := 0; y < h; y++ {
                for x := 0; x < w; x++ {
                        dst.Set(w-1-x, h-1-y, src.At(b.Min.X+x, b.Min.Y+y))
                }
        }
        return dst
}

// girar270 rotaciona 270° no sentido horário (90° anti-horário).
func girar270(src image.Image) image.Image {
        b := src.Bounds()
        w, h := b.Dx(), b.Dy()
        dst := image.NewRGBA(image.Rect(0, 0, h, w))
        for y := 0; y < h; y++ {
                for x := 0; x < w; x++ {
                        dst.Set(x, w-1-x, src.At(b.Min.X+x, b.Min.Y+y))
                }
        }
        return dst
}
