package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"github.com/fogleman/gg"
	qrcode "github.com/skip2/go-qrcode"
)

// KTAData holds all the member data needed to render the card.
type KTAData struct {
	NomorKTA    string // 16-digit member number
	NamaAnggota string // member name (empty = blank line for handwriting)
	Kecamatan   string // e.g. "Kecamatan Kelapa Gading"
	Kota        string // e.g. "KOTA JAKARTA UTARA"
	Provinsi    string // e.g. "DKI Jakarta"
	LogoPath    string // path to the Perindo eagle logo PNG
	OutputPath  string // destination file path for the generated PNG
}

// Card dimensions — CR80 physical size at 300 dpi
// 3.375" × 2.125" → 1013 × 638 px
const (
	cardW  = 1013
	cardH  = 638
	radius = 32.0

	// Layout constants
	marginL  = 64.0  // left margin for all body content
	marginR  = 64.0  // right margin
	headerH  = 220.0 // total header height (navy + red strip)
	redStripH = 78.0  // height of the red portion of header
	qrSize   = 175.0 // QR code pixel size
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

// font paths — falls back gracefully
var (
	fontRegular = "C:/Windows/Fonts/arial.ttf"
	fontBold    = "C:/Windows/Fonts/arialbd.ttf"
	fontItalic  = "C:/Windows/Fonts/ariali.ttf"
)

// GenerateKTACard renders a KTA Partai Perindo card and saves it as PNG.
func GenerateKTACard(data KTAData) error {
	dc := gg.NewContext(cardW, cardH)

	// ── 1. White card background ──────────────────────────────────────────────
	drawRoundedRect(dc, 0, 0, float64(cardW), float64(cardH), radius, colorBgCard)

	// ── 2. Navy header (rounded top corners only) ─────────────────────────────
	navyH := headerH - redStripH
	drawRoundedRectTop(dc, 0, 0, float64(cardW), navyH+4, radius, colorNavy)

	// Gold separator between navy and red
	dc.SetColor(colorGold)
	dc.DrawRectangle(0, navyH, float64(cardW), 4)
	dc.Fill()

	// Red strip
	dc.SetColor(colorRed)
	dc.DrawRectangle(0, navyH+4, float64(cardW), redStripH)
	dc.Fill()

	// Gold line at bottom of red strip
	dc.SetColor(colorGold)
	dc.DrawRectangle(0, headerH, float64(cardW), 4)
	dc.Fill()

	// ── 3. Logo centered in navy area ─────────────────────────────────────────
	if data.LogoPath != "" {
		logoY := navyH / 2
		if err := drawLogo(dc, data.LogoPath, float64(cardW)/2, logoY, 140); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load logo: %v\n", err)
		}
	}

	// ── 4. Header text in red strip ───────────────────────────────────────────
	redCenterY := navyH + 4 + redStripH/2

	// "KARTU TANDA ANGGOTA" — spaced small text
	loadFont(dc, fontRegular, 19)
	dc.SetColor(colorWhite)
	drawTrackedText(dc, "KARTU TANDA ANGGOTA", float64(cardW)/2, redCenterY-20, 4)

	// "PARTAI PERINDO" — bold large text
	loadFont(dc, fontBold, 40)
	dc.SetColor(colorWhite)
	drawTrackedText(dc, "PARTAI PERINDO", float64(cardW)/2, redCenterY+22, 5)

	// ── 5. Body layout (below header) ─────────────────────────────────────────
	bodyTop := headerH + 4 // just below gold line
	contentX := marginL
	rightEdge := float64(cardW) - marginR

	// Pre-calculate vertical positions so we can center the QR
	numY := bodyTop + 72
	div1Y := numY + 46
	labelY := div1Y + 30
	nameY := labelY + 48
	div2Y := nameY + 52

	// QR: right-aligned, vertically centered between div1 and div2
	// Clamp to qrSize max so it never exceeds the constant
	qrPad := 10.0
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
	loadFont(dc, fontBold, 52)
	dc.SetColor(colorBlack)
	dc.DrawStringAnchored(data.NomorKTA, contentX, numY, 0, 0.5)

	// ── 7. Divider (full width) ───────────────────────────────────────────────
	drawHLine(dc, contentX, rightEdge, div1Y, colorGold, 1.5)

	// ── 8. NAMA ANGGOTA section ───────────────────────────────────────────────
	drawSectionLabel(dc, "NAMA ANGGOTA", contentX, labelY)

	if data.NamaAnggota != "" {
		loadFont(dc, fontItalic, 34)
		dc.SetColor(colorBlack)
		dc.DrawStringAnchored(data.NamaAnggota, contentX, nameY, 0, 0.5)
	}
	// dashed underline — stops before QR
	drawDashedLine(dc, contentX, nameY+24, textMaxX, nameY+24, colorGold)

	// ── 9. Divider (full width) ───────────────────────────────────────────────
	drawHLine(dc, contentX, rightEdge, div2Y, colorGold, 1.5)

	// ── 10. WILAYAH section ───────────────────────────────────────────────────
	wilayahLabelY := div2Y + 28
	drawSectionLabel(dc, "WILAYAH", contentX, wilayahLabelY)

	loadFont(dc, fontRegular, 24)
	dc.SetColor(colorBlack)
	dc.DrawStringAnchored(data.Kecamatan, contentX, wilayahLabelY+42, 0, 0.5)
	dc.DrawStringAnchored(data.Kota+", "+data.Provinsi, contentX, wilayahLabelY+72, 0, 0.5)

	// ── 11. Save with rounded clip ────────────────────────────────────────────
	return saveWithRoundedClip(dc, data.OutputPath, radius)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// loadFont tries the primary path, falls back to Verdana variants silently.
func loadFont(dc *gg.Context, path string, size float64) {
	if err := dc.LoadFontFace(path, size); err != nil {
		switch path {
		case fontBold:
			dc.LoadFontFace("C:/Windows/Fonts/verdanab.ttf", size) //nolint
		case fontItalic:
			dc.LoadFontFace("C:/Windows/Fonts/verdanai.ttf", size) //nolint
		default:
			dc.LoadFontFace("C:/Windows/Fonts/verdana.ttf", size) //nolint
		}
	}
}

// drawSectionLabel draws a small all-caps tracked label in gray.
func drawSectionLabel(dc *gg.Context, text string, x, y float64) {
	loadFont(dc, fontRegular, 17)
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

// drawLogo loads a PNG logo and draws it centered at (cx, cy), scaled to maxW.
func drawLogo(dc *gg.Context, path string, cx, cy, maxW float64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return err
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
		dc.DrawImage(qr.Image(int(size)), int(x), int(y))
		return nil
	}
	dc.DrawImage(img, int(x), int(y))
	return nil
}

// saveWithRoundedClip clips the context to rounded corners and saves as PNG.
func saveWithRoundedClip(dc *gg.Context, outputPath string, r float64) error {
	w := float64(dc.Width())
	h := float64(dc.Height())

	final := gg.NewContext(int(w), int(h))
	final.DrawRoundedRectangle(0, 0, w, h, r)
	final.Clip()
	final.DrawImage(dc.Image(), 0, 0)

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	return png.Encode(f, final.Image())
}
