// Package fsstore grava as fotos enviadas no disco (pasta data/uploads),
// gerando miniaturas e versões médias em JPEG para interface e relatórios.
package fsstore

import (
        "bytes"
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
        "path/filepath"
        "time"

        "github.com/rwcarlsen/goexif/exif"
        xdraw "golang.org/x/image/draw"
        _ "golang.org/x/image/webp"

        "diagnostico-escolar/internal/model"
)

// Loja é o diretório base de dados do app.
type Loja struct {
        Dir string
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
        qualidade  = 82
        maxIMO     = 20 << 20 // 20 MB por foto
)

// SalvarImagem decodifica a imagem enviada, corrige a orientação EXIF,
// gera miniatura (480px) e versão média (1100px) em JPEG e grava tudo em uploads.
func (l *Loja) SalvarImagem(r io.Reader, nomeOriginal string, enviadoPor string) (model.Foto, int64, error) {
        bruto, err := io.ReadAll(io.LimitReader(r, maxIMO+1))
        if err != nil {
                return model.Foto{}, 0, fmt.Errorf("não foi possível ler o arquivo: %w", err)
        }
        if len(bruto) > maxIMO {
                return model.Foto{}, 0, fmt.Errorf("arquivo maior que 20 MB")
        }
        src, _, err := image.Decode(bytes.NewReader(bruto))
        if err != nil {
                return model.Foto{}, 0, fmt.Errorf("arquivo de imagem inválido (use JPG, PNG ou WEBP)")
        }
        src = corrigirOrientacao(bruto, src)

        id := novoID()
        if err := gravarJPEG(l.Caminho(id+".t.jpg"), escalar(src, largThumb), qualidade); err != nil {
                return model.Foto{}, 0, err
        }
        if err := gravarJPEG(l.Caminho(id+".m.jpg"), escalar(src, largMedia), qualidade); err != nil {
                return model.Foto{}, 0, err
        }
        // mantém o original para auditoria
        ext := "jpg"
        if len(bruto) > 3 && bruto[0] == 0x89 && bruto[1] == 'P' {
                ext = "png"
        }
        if err := os.WriteFile(l.Caminho(id+".orig."+ext), bruto, 0o644); err != nil {
                return model.Foto{}, 0, err
        }
        foto := model.Foto{ID: id, Nome: nomeOriginal}
        return foto, int64(len(bruto)), nil
}

// Excluir remove os arquivos de uma foto (best effort).
func (l *Loja) Excluir(id, arquivo string) {
        _ = os.Remove(l.Caminho(arquivo))
        _ = os.Remove(l.Caminho(id+".t.jpg"))
        _ = os.Remove(l.Caminho(id+".m.jpg"))
}

func gravarJPEG(caminho string, img image.Image, q int) error {
        f, err := os.Create(caminho)
        if err != nil {
                return err
        }
        defer f.Close()
        return jpeg.Encode(f, img, &jpeg.Options{Quality: q})
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
