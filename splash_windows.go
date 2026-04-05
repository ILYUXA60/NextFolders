package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	registerClassExW  = user32.NewProc("RegisterClassExW")
	createWindowExW   = user32.NewProc("CreateWindowExW")
	showWindow        = user32.NewProc("ShowWindow")
	updateWindow      = user32.NewProc("UpdateWindow")
	destroyWindow     = user32.NewProc("DestroyWindow")
	defWindowProcW    = user32.NewProc("DefWindowProcW")
	getMessageW       = user32.NewProc("GetMessageW")
	translateMessage  = user32.NewProc("TranslateMessage")
	dispatchMessageW  = user32.NewProc("DispatchMessageW")
	postMessageW      = user32.NewProc("PostMessageW")
	getModuleHandleW  = kernel32.NewProc("GetModuleHandleW")
	getSystemMetrics  = user32.NewProc("GetSystemMetrics")
	beginPaint        = user32.NewProc("BeginPaint")
	endPaint          = user32.NewProc("EndPaint")
	setBkMode         = gdi32.NewProc("SetBkMode")
	setTextColor      = gdi32.NewProc("SetTextColor")
	createFontW       = gdi32.NewProc("CreateFontW")
	selectObject      = gdi32.NewProc("SelectObject")
	drawTextW         = user32.NewProc("DrawTextW")
	createSolidBrush  = gdi32.NewProc("CreateSolidBrush")
	deleteObject      = gdi32.NewProc("DeleteObject")
	fillRect          = user32.NewProc("FillRect")
	setWindowPos      = user32.NewProc("SetWindowPos")
)

const (
	wsPopup           = 0x80000000
	wsVisible         = 0x10000000
	wsExTopmost       = 0x00000008
	wsExToolWindow    = 0x00000080
	swShow            = 5
	smCxScreen        = 0
	smCyScreen        = 1
	wmPaint           = 0x000F
	wmDestroy         = 0x0002
	wmClose           = 0x0010
	wmUser            = 0x0400
	wmCloseSplash     = wmUser + 1
	transparent       = 1
	dtCenter          = 0x01
	dtVCenter         = 0x04
	dtSingleLine      = 0x20
	swpNoSize         = 0x0001
	swpNoMove         = 0x0002
	hwndTopMost       = ^uintptr(0) // -1
)

type wndClassExW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   syscall.Handle
	Icon       syscall.Handle
	Cursor     syscall.Handle
	Background syscall.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     syscall.Handle
}

type point struct {
	X, Y int32
}

type msg struct {
	Hwnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type paintStruct struct {
	HDC         uintptr
	Erase       int32
	RcPaint     rect
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

var splashHwnd syscall.Handle

func splashWndProc(hwnd syscall.Handle, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := beginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

		// Background color: #0f172a
		bgColor := uintptr(0x002a170f) // BGR format
		brush, _, _ := createSolidBrush.Call(bgColor)
		r := rect{0, 0, 400, 200}
		fillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
		deleteObject.Call(brush)

		// Create font
		fontName, _ := syscall.UTF16PtrFromString("Segoe UI")
		hFont, _, _ := createFontW.Call(
			uintptr(uint32(0xFFFFFFE2)), // -30 height
			0, 0, 0,
			600, // weight (semibold)
			0, 0, 0,
			1, 0, 0, 0, 0,
			uintptr(unsafe.Pointer(fontName)),
		)
		oldFont, _, _ := selectObject.Call(hdc, hFont)

		// Text color: #60a5fa (light blue)
		setTextColor.Call(hdc, uintptr(0x00faa560)) // BGR
		setBkMode.Call(hdc, transparent)

		// Draw title
		title, _ := syscall.UTF16PtrFromString("NextFolders")
		titleRect := rect{0, 50, 400, 120}
		drawTextW.Call(hdc, uintptr(unsafe.Pointer(title)), ^uintptr(0), uintptr(unsafe.Pointer(&titleRect)), dtCenter|dtVCenter|dtSingleLine)

		// Smaller font for subtitle
		selectObject.Call(hdc, oldFont)
		deleteObject.Call(hFont)

		hFont2, _, _ := createFontW.Call(
			uintptr(uint32(0xFFFFFFF0)), // -16 height
			0, 0, 0,
			400, 0, 0, 0,
			1, 0, 0, 0, 0,
			uintptr(unsafe.Pointer(fontName)),
		)
		selectObject.Call(hdc, hFont2)

		// Subtitle color: #94a3b8
		setTextColor.Call(hdc, uintptr(0x00b8a394)) // BGR
		subtitle, _ := syscall.UTF16PtrFromString("Загрузка...")
		subRect := rect{0, 130, 400, 170}
		drawTextW.Call(hdc, uintptr(unsafe.Pointer(subtitle)), ^uintptr(0), uintptr(unsafe.Pointer(&subRect)), dtCenter|dtVCenter|dtSingleLine)

		selectObject.Call(hdc, oldFont)
		deleteObject.Call(hFont2)

		endPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0

	case wmCloseSplash:
		destroyWindow.Call(uintptr(hwnd))
		return 0

	case wmDestroy:
		return 0
	}

	ret, _, _ := defWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return ret
}

func ShowSplash() {
	go func() {
		runtime.LockOSThread()

		hInstance, _, _ := getModuleHandleW.Call(0)
		className, _ := syscall.UTF16PtrFromString("NextFoldersSplash")

		wc := wndClassExW{
			Size:      uint32(unsafe.Sizeof(wndClassExW{})),
			WndProc:   syscall.NewCallback(splashWndProc),
			Instance:  syscall.Handle(hInstance),
			ClassName: className,
		}

		registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))

		screenW, _, _ := getSystemMetrics.Call(smCxScreen)
		screenH, _, _ := getSystemMetrics.Call(smCyScreen)

		splashW := uintptr(400)
		splashH := uintptr(200)
		x := (screenW - splashW) / 2
		y := (screenH - splashH) / 2

		hwnd, _, _ := createWindowExW.Call(
			wsExTopmost|wsExToolWindow,
			uintptr(unsafe.Pointer(className)),
			0,
			wsPopup|wsVisible,
			x, y, splashW, splashH,
			0, 0,
			hInstance,
			0,
		)
		splashHwnd = syscall.Handle(hwnd)

		showWindow.Call(hwnd, swShow)
		updateWindow.Call(hwnd)

		// Message loop
		var m msg
		for {
			ret, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if ret == 0 || int32(ret) == -1 {
				break
			}
			translateMessage.Call(uintptr(unsafe.Pointer(&m)))
			dispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))

			if m.Message == wmDestroy || m.Message == wmCloseSplash {
				break
			}
		}
	}()
}

func HideSplash() {
	if splashHwnd != 0 {
		postMessageW.Call(uintptr(splashHwnd), wmCloseSplash, 0, 0)
		splashHwnd = 0
	}
}
