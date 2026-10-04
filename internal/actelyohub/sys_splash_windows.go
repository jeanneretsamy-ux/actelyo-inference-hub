//go:build windows

package actelyohub

// sys_splash_windows.go — écran de démarrage « Lancement de ACTELYO INFERENCE HUB… ».
//
// Petite fenêtre sans bordure, à coins arrondis, aux couleurs de la marque
// (fond noir, symbole Actelyo, texte blanc), affichée le temps que le
// serveur monte et que le navigateur s'ouvre. Rendu GDI (pur syscall, aucun
// CGO). Elle tourne sur son propre thread avec sa boucle de messages ; close()
// lui envoie WM_CLOSE.

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const (
	splashW = 440
	splashH = 150
)

var (
	u32s = syscall.NewLazyDLL("user32.dll")
	g32s = syscall.NewLazyDLL("gdi32.dll")
	k32s = syscall.NewLazyDLL("kernel32.dll")

	pRegisterClassExW = u32s.NewProc("RegisterClassExW")
	pCreateWindowExW  = u32s.NewProc("CreateWindowExW")
	pDefWindowProcW   = u32s.NewProc("DefWindowProcW")
	pShowWindow       = u32s.NewProc("ShowWindow")
	pUpdateWindow     = u32s.NewProc("UpdateWindow")
	pGetMessageW      = u32s.NewProc("GetMessageW")
	pTranslateMessage = u32s.NewProc("TranslateMessage")
	pDispatchMessageW = u32s.NewProc("DispatchMessageW")
	pPostQuitMessage  = u32s.NewProc("PostQuitMessage")
	pDestroyWindow    = u32s.NewProc("DestroyWindow")
	pPostMessageW     = u32s.NewProc("PostMessageW")
	pBeginPaint       = u32s.NewProc("BeginPaint")
	pEndPaint         = u32s.NewProc("EndPaint")
	pFillRect         = u32s.NewProc("FillRect")
	pDrawTextW        = u32s.NewProc("DrawTextW")
	pGetSystemMetrics = u32s.NewProc("GetSystemMetrics")
	pLoadCursorW      = u32s.NewProc("LoadCursorW")
	pSetWindowRgn     = u32s.NewProc("SetWindowRgn")

	pCreateSolidBrush   = g32s.NewProc("CreateSolidBrush")
	pDeleteObject       = g32s.NewProc("DeleteObject")
	pCreateFontW        = g32s.NewProc("CreateFontW")
	pSelectObject       = g32s.NewProc("SelectObject")
	pSetTextColor       = g32s.NewProc("SetTextColor")
	pSetBkMode          = g32s.NewProc("SetBkMode")
	pCreateRoundRectRgn = g32s.NewProc("CreateRoundRectRgn")

	pGetModuleHandleW = k32s.NewProc("GetModuleHandleW")

	splashClassOnce sync.Once
	splashProc      = syscall.NewCallback(splashWndProc)
)

type rect struct{ left, top, right, bottom int32 }
type point struct{ x, y int32 }
type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}
type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}
type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

const (
	// Fond noir : même symbole Actelyo que le favicon et les icônes
	// système (voir sys_brand_icon.go). L'écran de démarrage restait bleu.
	colBrand = 0x00000000 // COLORREF = 0x00BBGGRR
	colWhite = 0x00FFFFFF
)

type splash struct{ hwnd uintptr }

func showSplash(text string) *splash {
	s := &splash{}
	ready := make(chan uintptr, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		hwnd := createSplashWindow()
		ready <- hwnd
		if hwnd != 0 {
			pumpMessages()
		}
	}()
	s.hwnd = <-ready
	return s
}

func (s *splash) close() {
	if s == nil || s.hwnd == 0 {
		return
	}
	const wmClose = 0x0010
	pPostMessageW.Call(s.hwnd, wmClose, 0, 0)
}

func createSplashWindow() uintptr {
	className, _ := syscall.UTF16PtrFromString("ACTELYO_HUBSplash")
	hInst, _, _ := pGetModuleHandleW.Call(0)

	splashClassOnce.Do(func() {
		idcArrow := uintptr(32512)
		cursor, _, _ := pLoadCursorW.Call(0, idcArrow)
		wc := wndClassExW{
			lpfnWndProc:   splashProc,
			hInstance:     hInst,
			hCursor:       cursor,
			lpszClassName: className,
		}
		wc.cbSize = uint32(unsafe.Sizeof(wc))
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	})

	const (
		wsPopup      = 0x80000000
		exTopmost    = 0x00000008
		exToolwindow = 0x00000080
		smCX         = 0
		smCY         = 1
	)
	cx, _, _ := pGetSystemMetrics.Call(smCX)
	cy, _, _ := pGetSystemMetrics.Call(smCY)
	x := (int(cx) - splashW) / 2
	y := (int(cy) - splashH) / 2
	title, _ := syscall.UTF16PtrFromString("ACTELYO INFERENCE HUB")

	hwnd, _, _ := pCreateWindowExW.Call(
		uintptr(exTopmost|exToolwindow),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		uintptr(uint32(wsPopup)),
		uintptr(x), uintptr(y), uintptr(splashW), uintptr(splashH),
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return 0
	}
	// Coins arrondis.
	rgn, _, _ := pCreateRoundRectRgn.Call(0, 0, splashW+1, splashH+1, 22, 22)
	pSetWindowRgn.Call(hwnd, rgn, 1)

	const swShowNoActivate = 4
	pShowWindow.Call(hwnd, swShowNoActivate)
	pUpdateWindow.Call(hwnd)
	return hwnd
}

func pumpMessages() {
	var m msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = erreur
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func splashWndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	const (
		wmDestroy = 0x0002
		wmClose   = 0x0010
		wmPaint   = 0x000F
	)
	switch message {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		paintSplash(hdc)
		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmClose:
		pDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

func paintSplash(hdc uintptr) {
	// Fond aux couleurs de la marque.
	bg, _, _ := pCreateSolidBrush.Call(colBrand)
	full := rect{0, 0, splashW, splashH}
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&full)), bg)
	pDeleteObject.Call(bg)

	// Render the official symbol on the black splash background.
	const logo = 58
	ox, oy := int32(40), int32((splashH-logo)/2)
	symbol := brandIconImage(logo, brandBlack, brandWhite)
	for y := 0; y < logo; y++ {
		for x := 0; x < logo; {
			c := symbol.RGBAAt(x, y)
			end := x + 1
			for end < logo && symbol.RGBAAt(end, y) == c {
				end++
			}
			if c.A != 0 {
				brush, _, _ := pCreateSolidBrush.Call(uintptr(c.R) | uintptr(c.G)<<8 | uintptr(c.B)<<16)
				r := rect{ox + int32(x), oy + int32(y), ox + int32(end), oy + int32(y+1)}
				pFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
				pDeleteObject.Call(brush)
			}
			x = end
		}
	}

	// Textes (blanc, fond transparent).
	const transparent = 1
	pSetBkMode.Call(hdc, transparent)
	pSetTextColor.Call(hdc, colWhite)

	textX := ox + logo + 26
	drawText(hdc, "ACTELYO INFERENCE HUB", -30, 700, rect{textX, 40, splashW - 20, 82})
	drawText(hdc, "Lancement en cours…", -17, 400, rect{textX, 84, splashW - 20, 118})
}

func drawText(hdc uintptr, s string, height, weight int32, r rect) {
	face, _ := syscall.UTF16PtrFromString("Segoe UI")
	const (
		defaultCharset   = 1
		cleartypeQuality = 5
	)
	font, _, _ := pCreateFontW.Call(
		uintptr(height), 0, 0, 0, uintptr(weight),
		0, 0, 0, defaultCharset, 0, 0, cleartypeQuality, 0,
		uintptr(unsafe.Pointer(face)),
	)
	old, _, _ := pSelectObject.Call(hdc, font)
	txt, _ := syscall.UTF16PtrFromString(s)
	const (
		dtLeft       = 0x0000
		dtSingleline = 0x0020
		dtVcenter    = 0x0004
		dtNoclip     = 0x0100
	)
	rc := r
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(txt)), ^uintptr(0),
		uintptr(unsafe.Pointer(&rc)), dtLeft|dtSingleline|dtVcenter|dtNoclip)
	pSelectObject.Call(hdc, old)
	pDeleteObject.Call(font)
}
