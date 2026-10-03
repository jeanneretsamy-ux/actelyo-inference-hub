package ajean

// Actelyo system icons are derived from the official ERP PNG.

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

//go:embed actelyo-logo.png
var actelyoLogoPNG []byte

const trayIconSize = 32

var (
	brandBlack = color.RGBA{0x00, 0x00, 0x00, 0xff}
	brandWhite = color.RGBA{0xff, 0xff, 0xff, 0xff}
	brandClear = color.RGBA{0, 0, 0, 0}
)

// Keep the logo aspect ratio and alpha at each system-icon size.
func brandIconImage(n int, bg, fg color.RGBA) *image.RGBA {
	source, err := png.Decode(bytes.NewReader(actelyoLogoPNG))
	if err != nil {
		panic(err)
	}
	result := image.NewRGBA(image.Rect(0, 0, n, n))
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	scaledWidth, scaledHeight := n, n
	if width > height {
		scaledHeight = n * height / width
	} else {
		scaledWidth = n * width / height
	}
	if scaledWidth < 1 {
		scaledWidth = 1
	}
	if scaledHeight < 1 {
		scaledHeight = 1
	}
	left, top := (n-scaledWidth)/2, (n-scaledHeight)/2
	for y := 0; y < scaledHeight; y++ {
		for x := 0; x < scaledWidth; x++ {
			pixel := source.At(bounds.Min.X+x*width/scaledWidth, bounds.Min.Y+y*height/scaledHeight)
			if fg.A == 0 {
				_, _, _, alpha := pixel.RGBA()
				pixel = color.RGBA{A: uint8(alpha >> 8)}
			}
			result.Set(left+x, top+y, pixel)
		}
	}
	return result
}

func encodePNG(img *image.RGBA) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// BrandIconPNG rend l'icône de marque (noir + « j » blanc) en PNG de n pixels.
// Exporté pour le générateur d'icône du .exe (tools/gen-icon).
func BrandIconPNG(n int) []byte { return encodePNG(brandIconImage(n, brandBlack, brandWhite)) }

// brandTemplatePNG rend la variante « template » attendue par macOS : seule la
// couche alpha compte, le système colore la forme selon le thème de la barre de
// menus. Le « j » est donc DÉCOUPÉ (transparent) dans un carré opaque, sans quoi
// une icône entièrement noire disparaît sur une barre de menus sombre.
//
// Utilisée par sys_tray_darwin.go — un fichier que seule une compilation avec
// CGO voit. Les analyseurs lancés sans CGO la croient morte : elle avait été
// supprimée à ce titre, ce qui a cassé la compilation macOS en CI.
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
