package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/redis/go-redis/v9"
	qrcode "github.com/skip2/go-qrcode"
)

// KTAData holds all the member data needed to render the card.
type KTAData struct {
	NomorKTA         string // 16-digit member number
	NamaAnggota      string // member name (empty = blank line for handwriting)
	Kecamatan        string // e.g. "Kecamatan Kelapa Gading"
	Kota             string // e.g. "KOTA JAKARTA UTARA"
	Provinsi         string // e.g. "DKI Jakarta"
	LogoS3Bucket     string // S3 bucket containing the Perindo eagle logo PNG
	LogoS3Key        string // S3 object key for the logo PNG
	FontS3Bucket     string // S3 bucket containing the font files
	FontRegularS3Key string // S3 object key for the regular font
	FontBoldS3Key    string // S3 object key for the bold font
	FontBlackS3Key   string // S3 object key for the heaviest font
	FontItalicS3Key  string // S3 object key for the italic font
	S3Bucket         string // destination S3 bucket
	S3Key            string // destination object key
}

// Card dimensions — CR80 physical size at 300 dpi
// 3.375" × 2.125" → 1013 × 638 px
const (
	cardW  = 1013
	cardH  = 638
	radius = 32.0

	// Layout constants
	marginL   = 64.0  // left margin for all body content
	marginR   = 64.0  // right margin
	headerH   = 230.0 // total header height (navy + red strip)
	redStripH = 120.0 // height of the red portion of header
	qrSize    = 175.0 // QR code pixel size
)

// Brand colours
var (
	colorNavy   = color.RGBA{R: 18, G: 34, B: 102, A: 255}
	colorRed    = color.RGBA{R: 196, G: 30, B: 58, A: 255}
	colorGold   = color.RGBA{R: 198, G: 160, B: 80, A: 255}
	colorWhite  = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	colorBlack  = color.RGBA{R: 20, G: 20, B: 20, A: 255}
	colorGray   = color.RGBA{R: 130, G: 130, B: 130, A: 255}
	colorBgCard = color.RGBA{R: 248, G: 248, B: 248, A: 255}
)

type cardFonts struct {
	regular *truetype.Font
	bold    *truetype.Font
	black   *truetype.Font
	italic  *truetype.Font
}

// GenerateKTACard renders a KTA Partai Perindo card and uploads it as PNG to S3.
func GenerateKTACard(client *s3.Client, cache *redis.Client, data KTAData) error {
	fonts, err := loadCardFonts(cache, client, data.FontS3Bucket, data.FontRegularS3Key, data.FontBoldS3Key, data.FontBlackS3Key, data.FontItalicS3Key)
	if err != nil {
		return err
	}

	dc := gg.NewContext(cardW, cardH)

	// ── 1. White card background ──────────────────────────────────────────────
	drawRoundedRect(dc, 0, 0, float64(cardW), float64(cardH), radius, colorBgCard)

	// ── 2. Navy header top (solid, rounded top corners) ──────────────────────
	navyH := headerH - redStripH
	// drawRoundedRectTop(dc, 0, 0, float64(cardW), navyH, radius, colorNavy)

	// ── 3. Red strip with left→right gradient (#232e6f → #bf2135) ────────────
	gradLeft := color.RGBA{R: 0x23, G: 0x2e, B: 0x6f, A: 255}
	gradRight := color.RGBA{R: 0xbf, G: 0x21, B: 0x35, A: 255}
	drawHGradientRect(dc, 0, navyH, float64(cardW), redStripH, gradLeft, gradRight)

	// Gold line at bottom of red strip
	dc.SetColor(colorGold)
	dc.DrawRectangle(0, headerH, float64(cardW), 4)
	dc.Fill()

	// ── 4. Logo centered in navy area ─────────────────────────────────────────
	if data.LogoS3Bucket != "" || data.LogoS3Key != "" {
		logoY := navyH / 2
		if err := drawLogo(dc, cache, client, data.LogoS3Bucket, data.LogoS3Key, float64(cardW)/2, logoY, 140); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load logo: %v\n", err)
		}
	}

	// ── 5. Header text in red strip ───────────────────────────────────────────
	redCenterY := navyH + redStripH/2

	// "KARTU TANDA ANGGOTA" — spaced small text
	loadFont(dc, fonts.bold, 19)
	dc.SetColor(colorWhite)
	drawTrackedText(dc, "KARTU TANDA ANGGOTA", float64(cardW)/2, redCenterY-22, 8)

	// "PARTAI PERINDO" — bold large text
	loadFont(dc, fonts.black, 40)
	dc.SetColor(colorWhite)
	drawTrackedText(dc, "PARTAI PERINDO", float64(cardW)/2, redCenterY+16, 5)

	// ── 6. Body layout (below header) ─────────────────────────────────────────
	bodyTop := headerH // just below gold line
	contentX := marginL
	rightEdge := float64(cardW) - marginR

	// Pre-calculate vertical positions so we can center the QR
	numY := bodyTop + 64
	div1Y := numY + 36
	labelY := div1Y + 30
	nameY := labelY + 60
	div2Y := nameY + 68

	// QR: right-aligned, vertically centered between div1 and div2
	// Clamp to qrSize max so it never exceeds the constant
	qrPad := -30.0
	qrZoneH := div2Y - div1Y - qrPad*2
	qrSize2 := qrZoneH
	if qrSize2 > qrSize {
		qrSize2 = qrSize
	}
	qrX := rightEdge - qrSize2
	qrY := div1Y + qrPad // sit just below div1 with padding
	// Left text area stops before the QR with a gap
	textMaxX := qrX - 20

	qrContent := data.NomorKTA
	if qrContent == "" {
		qrContent = "PERINDO"
	}
	if err := drawQRCode(dc, qrContent, qrX, qrY, qrSize2); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not generate QR: %v\n", err)
	}

	// ── 6. Member number ──────────────────────────────────────────────────────
	loadFont(dc, fonts.black, 48)
	dc.SetColor(colorBlack)
	dc.DrawStringAnchored(data.NomorKTA, contentX, numY, 0, 0.5)

	// ── 7. Divider (full width) ───────────────────────────────────────────────
	// drawHLine(dc, contentX, rightEdge, div1Y, colorGold, 1.5)

	// ── 8. NAMA ANGGOTA section ───────────────────────────────────────────────
	drawSectionLabel(dc, fonts, "NAMA ANGGOTA", contentX, labelY)

	if data.NamaAnggota != "" {
		loadFont(dc, fonts.italic, 34)
		dc.SetColor(colorBlack)
		dc.DrawStringAnchored(data.NamaAnggota, contentX, nameY, 0, 0.5)
	}
	// dashed underline — stops before QR
	drawDashedLine(dc, contentX, nameY+24, textMaxX, nameY+24, colorGold)

	// ── 9. Divider (full width) ───────────────────────────────────────────────
	// drawHLine(dc, contentX, rightEdge, div2Y, colorGold, 1.5)

	// ── 10. WILAYAH section ───────────────────────────────────────────────────
	wilayahLabelY := div2Y + 14
	drawSectionLabel(dc, fonts, "WILAYAH", contentX, wilayahLabelY)

	loadFont(dc, fonts.regular, 24)
	dc.SetColor(colorBlack)
	dc.DrawStringAnchored(data.Kecamatan, contentX, wilayahLabelY+42, 0, 0.5)
	dc.DrawStringAnchored(data.Kota+", "+data.Provinsi, contentX, wilayahLabelY+72, 0, 0.5)

	// ── 11. Save with rounded clip ────────────────────────────────────────────
	return saveWithRoundedClip(dc, client, data.S3Bucket, data.S3Key, radius)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func loadCardFonts(cache *redis.Client, client *s3.Client, bucket, regularKey, boldKey, blackKey, italicKey string) (cardFonts, error) {
	if bucket == "" {
		return cardFonts{}, fmt.Errorf("font S3 bucket is required")
	}

	regular, err := loadFontFromS3(cache, client, bucket, regularKey)
	if err != nil {
		return cardFonts{}, fmt.Errorf("load regular font: %w", err)
	}
	bold, err := loadFontFromS3(cache, client, bucket, boldKey)
	if err != nil {
		return cardFonts{}, fmt.Errorf("load bold font: %w", err)
	}
	black, err := loadFontFromS3(cache, client, bucket, blackKey)
	if err != nil {
		return cardFonts{}, fmt.Errorf("load black font: %w", err)
	}
	italic, err := loadFontFromS3(cache, client, bucket, italicKey)
	if err != nil {
		return cardFonts{}, fmt.Errorf("load italic font: %w", err)
	}

	return cardFonts{regular: regular, bold: bold, black: black, italic: italic}, nil
}

func loadFontFromS3(cache *redis.Client, client *s3.Client, bucket, key string) (*truetype.Font, error) {
	if key == "" {
		return nil, fmt.Errorf("font S3 key is required")
	}
	fontData, err := loadS3Asset(context.Background(), cache, client, bucket, key)
	if err != nil {
		return nil, err
	}
	font, err := truetype.Parse(fontData)
	if err != nil {
		return nil, fmt.Errorf("parse s3://%s/%s: %w", bucket, key, err)
	}
	return font, nil
}

func loadFont(dc *gg.Context, font *truetype.Font, size float64) {
	face := truetype.NewFace(font, &truetype.Options{Size: size})
	dc.SetFontFace(face)
}

// drawSectionLabel draws a small all-caps tracked label in gray.
func drawSectionLabel(dc *gg.Context, fonts cardFonts, text string, x, y float64) {
	loadFont(dc, fonts.regular, 17)
	dc.SetColor(colorGray)
	// left-aligned tracked text
	drawTrackedTextLeft(dc, text, x, y, 3)
}

// drawTrackedText draws centered text with extra letter-spacing.
func drawTrackedText(dc *gg.Context, text string, cx, y, spacing float64) {
	runes := []rune(text)
	totalW := 0.0
	for i, ch := range runes {
		w, _ := dc.MeasureString(string(ch))
		totalW += w
		if i < len(runes)-1 {
			totalW += spacing
		}
	}
	x := cx - totalW/2
	for i, ch := range runes {
		dc.DrawStringAnchored(string(ch), x, y, 0, 0.5)
		w, _ := dc.MeasureString(string(ch))
		x += w
		if i < len(runes)-1 {
			x += spacing
		}
	}
}

// drawTrackedTextLeft draws left-aligned text with extra letter-spacing.
func drawTrackedTextLeft(dc *gg.Context, text string, x, y, spacing float64) {
	cx := x
	for i, ch := range []rune(text) {
		dc.DrawStringAnchored(string(ch), cx, y, 0, 0.5)
		w, _ := dc.MeasureString(string(ch))
		cx += w
		if i < len([]rune(text))-1 {
			cx += spacing
		}
	}
}

// drawRoundedRect draws a filled rounded rectangle.
func drawRoundedRect(dc *gg.Context, x, y, w, h, r float64, c color.Color) {
	dc.SetColor(c)
	dc.DrawRoundedRectangle(x, y, w, h, r)
	dc.Fill()
}

// drawRoundedRectTop draws a rectangle with rounded top corners only.
func drawRoundedRectTop(dc *gg.Context, x, y, w, h, r float64, c color.Color) {
	dc.SetColor(c)
	dc.NewSubPath()
	dc.MoveTo(x+r, y)
	dc.LineTo(x+w-r, y)
	dc.DrawArc(x+w-r, y+r, r, -math.Pi/2, 0)
	dc.LineTo(x+w, y+h)
	dc.LineTo(x, y+h)
	dc.DrawArc(x+r, y+r, r, math.Pi, 3*math.Pi/2)
	dc.ClosePath()
	dc.Fill()
}

// drawHLine draws a solid horizontal line.
func drawHLine(dc *gg.Context, x1, x2, y float64, c color.Color, lineW float64) {
	dc.SetColor(c)
	dc.DrawRectangle(x1, y, x2-x1, lineW)
	dc.Fill()
}

// drawDashedLine draws a horizontal dashed line.
func drawDashedLine(dc *gg.Context, x1, y1, x2, y2 float64, c color.Color) {
	dc.SetColor(c)
	dc.SetLineWidth(1.5)
	dashLen, gapLen := 8.0, 5.0
	cx := x1
	for cx < x2 {
		end := cx + dashLen
		if end > x2 {
			end = x2
		}
		dc.DrawLine(cx, y1, end, y2)
		dc.Stroke()
		cx += dashLen + gapLen
	}
}

// drawLogo downloads a PNG logo from S3 and draws it centered at (cx, cy), scaled to maxW.
func drawLogo(dc *gg.Context, cache *redis.Client, client *s3.Client, bucket, key string, cx, cy, maxW float64) error {
	if bucket == "" || key == "" {
		return fmt.Errorf("logo S3 bucket and key are required")
	}
	logoData, err := loadS3Asset(context.Background(), cache, client, bucket, key)
	if err != nil {
		return err
	}
	img, err := png.Decode(bytes.NewReader(logoData))
	if err != nil {
		return fmt.Errorf("decode logo s3://%s/%s: %w", bucket, key, err)
	}

	b := img.Bounds()
	scale := maxW / float64(b.Dx())
	dstW := int(maxW)
	dstH := int(float64(b.Dy()) * scale)

	scaled := gg.NewContext(dstW, dstH)
	scaled.Scale(scale, scale)
	scaled.DrawImage(img, 0, 0)

	dc.DrawImageAnchored(scaled.Image(), int(cx), int(cy), 0.5, 0.5)
	return nil
}

// drawQRCode renders a QR code and draws it at top-left position (x, y).
// The white QR background is replaced with the card background color.
func drawQRCode(dc *gg.Context, content string, x, y, size float64) error {
	qr, err := qrcode.New(content, qrcode.High)
	if err != nil {
		return err
	}
	qr.DisableBorder = true
	qr.BackgroundColor = colorBgCard
	qr.ForegroundColor = colorBlack

	pngBytes, err := qr.PNG(int(size))
	if err != nil {
		return err
	}

	img, _, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		dc.DrawImage(qr.Image(int(size)), int(x), int(y-20))
		return nil
	}
	dc.DrawImage(img, int(x), int(y-20))
	return nil
}

// saveWithRoundedClip clips the context to rounded corners and uploads it as PNG to S3.
func saveWithRoundedClip(dc *gg.Context, client *s3.Client, bucket, key string, r float64) error {
	if bucket == "" {
		return fmt.Errorf("S3 bucket is required")
	}
	if key == "" {
		return fmt.Errorf("S3 object key is required")
	}

	w := float64(dc.Width())
	h := float64(dc.Height())

	final := gg.NewContext(int(w), int(h))
	final.DrawRoundedRectangle(0, 0, w, h, r)
	final.Clip()
	final.DrawImage(dc.Image(), 0, 0)

	var imageData bytes.Buffer
	if err := png.Encode(&imageData, final.Image()); err != nil {
		return fmt.Errorf("encode PNG: %w", err)
	}

	_, err := client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(imageData.Bytes()),
		ContentType: aws.String("image/png"),
	})
	if err != nil {
		return fmt.Errorf("upload PNG to s3://%s/%s: %w", bucket, key, err)
	}
	return nil
}

// drawHGradientRect fills a rectangle with a horizontal linear gradient,
// lerping from colorL on the left to colorR on the right.
func drawHGradientRect(dc *gg.Context, x, y, w, h float64, colorL, colorR color.RGBA) {
	for col := 0; col < int(w); col++ {
		t := float64(col) / float64(int(w)-1)
		cr := uint8(float64(colorL.R) + t*(float64(colorR.R)-float64(colorL.R)))
		cg := uint8(float64(colorL.G) + t*(float64(colorR.G)-float64(colorL.G)))
		cb := uint8(float64(colorL.B) + t*(float64(colorR.B)-float64(colorL.B)))
		dc.SetColor(color.RGBA{R: cr, G: cg, B: cb, A: 255})
		dc.DrawRectangle(x+float64(col), y, 1, h)
		dc.Fill()
	}
}
