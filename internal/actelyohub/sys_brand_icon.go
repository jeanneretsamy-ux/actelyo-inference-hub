package actelyohub

// Actelyo symbol from ACTELYO-ERP, shared by desktop icons and web branding.

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

const trayIconSize = 32

var (
	brandBlack = color.RGBA{0x00, 0x00, 0x00, 0xff}
	brandWhite = color.RGBA{0xff, 0xff, 0xff, 0xff}
	brandClear = color.RGBA{0, 0, 0, 0}
)

//go:embed brand/actelyo-symbol.png
var actelyoSymbolPNG []byte

var actelyoSymbol = func() image.Image {
	img, err := png.Decode(bytes.NewReader(actelyoSymbolPNG))
	if err != nil {
		panic("invalid embedded Actelyo symbol: " + err.Error())
	}
	return img
}()

// brandIconImage scales the original asset; transparent fg selects the macOS template.
func brandIconImage(n int, _ color.RGBA, fg color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	bounds := actelyoSymbol.Bounds()
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			c := actelyoSymbol.At(bounds.Min.X+x*bounds.Dx()/n, bounds.Min.Y+y*bounds.Dy()/n)
			if fg.A == 0 {
				_, _, _, a := c.RGBA()
				img.SetRGBA(x, y, color.RGBA{A: uint8(a >> 8)})
			} else {
				img.Set(x, y, c)
			}
		}
	}
	return img
}

func encodePNG(img *image.RGBA) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// BrandIconPNG renders the official Actelyo symbol at n pixels.
// Exporté pour le générateur d'icône du .exe (tools/gen-icon).
func BrandIconPNG(n int) []byte { return encodePNG(brandIconImage(n, brandBlack, brandWhite)) }

// brandTemplatePNG uses the symbol alpha for the macOS menu bar.
//
//lint:ignore U1000 utilisée par sys_tray_darwin.go, invisible sans CGO/macOS
func brandTemplatePNG(n int) []byte {
	return encodePNG(brandIconImage(n, brandBlack, brandClear))
}

// BrandICO emballe une ou plusieurs tailles PNG dans un conteneur .ico. Windows
// accepte le PNG comme image d'une entrée .ico (alpha conservé pour les coins
// arrondis). Plusieurs tailles = un rendu net partout, de la barre des tâches
// (16 px) à la grande tuile de l'explorateur (256 px).
func BrandICO(sizes ...int) []byte {
	if len(sizes) == 0 {
		sizes = []int{trayIconSize}
	}
	imgs := make([][]byte, 0, len(sizes))
	for _, n := range sizes {
		imgs = append(imgs, BrandIconPNG(n))
	}
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, uint16(0))         // réservé
	binary.Write(&ico, binary.LittleEndian, uint16(1))         // type = icône
	binary.Write(&ico, binary.LittleEndian, uint16(len(imgs))) // nombre d'images
	offset := uint32(6 + 16*len(imgs))                         // fin du répertoire
	for i, p := range imgs {
		n := sizes[i]
		// 256 px se code 0 dans un .ico (le champ ne fait qu'un octet).
		ico.WriteByte(byte(n % 256))
		ico.WriteByte(byte(n % 256))
		ico.WriteByte(0)                                        // couleurs de la palette (0 = truecolor)
		ico.WriteByte(0)                                        // réservé
		binary.Write(&ico, binary.LittleEndian, uint16(1))      // plans
		binary.Write(&ico, binary.LittleEndian, uint16(32))     // bits/pixel
		binary.Write(&ico, binary.LittleEndian, uint32(len(p))) // taille données
		binary.Write(&ico, binary.LittleEndian, offset)         // offset données
		offset += uint32(len(p))
	}
	for _, p := range imgs {
		ico.Write(p)
	}
	return ico.Bytes()
}
