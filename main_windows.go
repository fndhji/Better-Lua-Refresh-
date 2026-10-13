//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	WS_OVERLAPPED        = 0x00000000
	WS_CAPTION           = 0x00C00000
	WS_SYSMENU           = 0x00080000
	WS_MINIMIZEBOX       = 0x00020000
	WS_CLIPCHILDREN      = 0x02000000
	WS_VISIBLE           = 0x10000000
	WS_CHILD             = 0x40000000
	WS_BORDER            = 0x00800000
	WS_VSCROLL           = 0x00200000
	WS_TABSTOP           = 0x00010000
	ES_LEFT              = 0x0000
	ES_MULTILINE         = 0x0004
	ES_AUTOVSCROLL       = 0x0040
	ES_AUTOHSCROLL       = 0x0080
	ES_READONLY          = 0x0800
	BS_OWNERDRAW         = 0x0000000B
	WM_CREATE            = 0x0001
	WM_DESTROY           = 0x0002
	WM_PAINT             = 0x000F
	WM_ERASEBKGND        = 0x0014
	WM_COMMAND           = 0x0111
	WM_DRAWITEM          = 0x002B
	WM_CTLCOLOREDIT      = 0x0133
	WM_CTLCOLORSTATIC    = 0x0138
	WM_TIMER             = 0x0113
	WM_SETFONT           = 0x0030
	WM_GETTEXT           = 0x000D
	WM_GETTEXTLENGTH     = 0x000E
	WM_SETICON           = 0x0080
	WM_APP_NOTIFY        = 0x8001
	WM_APP_SNAPSHOT      = 0x8002
	WM_APP_PATH          = 0x8003
	DT_LEFT              = 0
	DT_CENTER            = 1
	DT_VCENTER           = 4
	DT_WORDBREAK         = 0x10
	DT_SINGLELINE        = 0x20
	DT_NOPREFIX          = 0x800
	SW_SHOW              = 5
	COLOR_WINDOW         = 5
	TRANSPARENT          = 1
	EM_SETSEL            = 0x00B1
	EM_SCROLLCARET       = 0x00B7
	MB_OK                = 0
	MB_ICONERROR         = 0x10
	MB_ICONINFORMATION   = 0x40
	BIF_NEWDIALOGSTYLE   = 0x0040
	BIF_RETURNONLYFSDIRS = 0x0001
	CS_HREDRAW           = 2
	CS_VREDRAW           = 1
	IDC_INSTALL          = 101
	IDC_BROWSE           = 102
	IDC_START            = 103
	IDC_STOP             = 104
	IDC_AUTOSTART        = 105
	IDC_LANGUAGE         = 106
	IDC_TUTORIAL         = 107
	IDC_PATH             = 108
	IDC_CONSOLE          = 109
	IDC_FOLDER           = 110
	IDC_DETAILS          = 111
	ERROR_ALREADY_EXISTS = 183
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	shell32                = syscall.NewLazyDLL("shell32.dll")
	ole32                  = syscall.NewLazyDLL("ole32.dll")
	createWindowEx         = user32.NewProc("CreateWindowExW")
	defWindowProc          = user32.NewProc("DefWindowProcW")
	registerClass          = user32.NewProc("RegisterClassExW")
	showWindow             = user32.NewProc("ShowWindow")
	updateWindow           = user32.NewProc("UpdateWindow")
	sendMessage            = user32.NewProc("SendMessageW")
	postMessage            = user32.NewProc("PostMessageW")
	setWindowText          = user32.NewProc("SetWindowTextW")
	getWindowText          = user32.NewProc("GetWindowTextW")
	messageBox             = user32.NewProc("MessageBoxW")
	invalidateRect         = user32.NewProc("InvalidateRect")
	beginPaint             = user32.NewProc("BeginPaint")
	endPaint               = user32.NewProc("EndPaint")
	isDialogMessage        = user32.NewProc("IsDialogMessageW")
	drawFocusRect          = user32.NewProc("DrawFocusRect")
	adjustWindowRect       = user32.NewProc("AdjustWindowRect")
	getSystemMetrics       = user32.NewProc("GetSystemMetrics")
	dwmSetWindowAttribute  = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	getCursorPos           = user32.NewProc("GetCursorPos")
	windowFromPoint        = user32.NewProc("WindowFromPoint")
	getClientRect          = user32.NewProc("GetClientRect")
	fillRect               = user32.NewProc("FillRect")
	drawText               = user32.NewProc("DrawTextW")
	setTextColor           = gdi32.NewProc("SetTextColor")
	setBkColor             = gdi32.NewProc("SetBkColor")
	setBkMode              = gdi32.NewProc("SetBkMode")
	selectObject           = gdi32.NewProc("SelectObject")
	createSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	createPen              = gdi32.NewProc("CreatePen")
	roundRect              = gdi32.NewProc("RoundRect")
	createFont             = gdi32.NewProc("CreateFontW")
	deleteObject           = gdi32.NewProc("DeleteObject")
	setTimer               = user32.NewProc("SetTimer")
	killTimer              = user32.NewProc("KillTimer")
	loadCursor             = user32.NewProc("LoadCursorW")
	getModuleHandle        = kernel32.NewProc("GetModuleHandleW")
	postQuitMessage        = user32.NewProc("PostQuitMessage")
	translateMessage       = user32.NewProc("TranslateMessage")
	dispatchMessage        = user32.NewProc("DispatchMessageW")
	getMessage             = user32.NewProc("GetMessageW")
	browseForFolder        = shell32.NewProc("SHBrowseForFolderW")
	getFolderPath          = shell32.NewProc("SHGetPathFromIDListW")
	cotaskmemfree          = ole32.NewProc("CoTaskMemFree")
	createMutex            = kernel32.NewProc("CreateMutexW")
	openMutex              = kernel32.NewProc("OpenMutexW")
	closeHandle            = kernel32.NewProc("CloseHandle")
	createSnapshot         = kernel32.NewProc("CreateToolhelp32Snapshot")
	process32First         = kernel32.NewProc("Process32FirstW")
	process32Next          = kernel32.NewProc("Process32NextW")
	openProcess            = kernel32.NewProc("OpenProcess")
	queryProcessName       = kernel32.NewProc("QueryFullProcessImageNameW")
	setProcessDpiAwareness = user32.NewProc("SetProcessDPIAware")
	appHome                = filepath.Join(os.Getenv("LOCALAPPDATA"), "ExtremeLuaRefresh")
	cfgPath                = filepath.Join(appHome, "settings.json")
	statusPath             = filepath.Join(appHome, "monitor_status.json")
	appLogPath             = filepath.Join(appHome, "app.log")
	watcherLogPath         = filepath.Join(appHome, "watcher.log")
	stopFlag               = filepath.Join(appHome, "monitor.stop")
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type wndclass struct {
	Size, Style                        uint32
	WindowProc                         uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	IconSmall                          uintptr
}
type msg struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
	Private        uint32
}
type paintstruct struct {
	DC                 uintptr
	Erase              int32
	Paint              rect
	Restore, IncUpdate int32
	Reserved           [32]byte
}
type drawItem struct {
	CtlType, CtlID, ItemID, ItemAction, ItemState uint32
	HwndItem, HDC                                 uintptr
	ItemRect                                      rect
	ItemData                                      uintptr
}
type browseInfo struct {
	Owner, PidlRoot uintptr
	DisplayName     *uint16
	Title           *uint16
	Flags           uint32
	Callback        uintptr
	LParam          uintptr
	Image           int32
}
type processEntry struct {
	Size, Usage, ProcessID      uint32
	DefaultHeap                 uintptr
	ModuleID, Threads, ParentID uint32
	BasePriority                int32
	Flags                       uint32
	Exe                         [260]uint16
}

type guiState struct {
	hwnd, pathEdit, console                   uintptr
	controls                                  map[int]uintptr
	lang                                      string
	auto                                      bool
	installed                                 string
	guide                                     bool
	status                                    MonitorStatus
	renderedLog                               string
	lastLogTime                               time.Time
	darkBrush, editBrush, consoleBrush        uintptr
	normalFont, boldFont, monoFont, titleFont uintptr
	busy                                      bool
}

var ui guiState
var hoveredButton uintptr
var detailedLogs bool
var latestLogs string

// Disk I/O and Steam discovery must never run inside the Win32 message pump.
// Only the window thread mutates guiState or HWND controls.
type guiSnapshot struct {
	status MonitorStatus
	logs   string
}

var guiSnapshotCh = make(chan guiSnapshot, 2)
var guiPathCh = make(chan string, 1)

func readLogsBackground() string {
	return tail(appLogPath, 9000) + "\r\nWatcher\r\n" + tail(watcherLogPath, 14000)
}
func queueSnapshot(v guiSnapshot) {
	select {
	case guiSnapshotCh <- v:
	default:
		// Replace stale data with the newest snapshot, never block the watcher.
		select {
		case <-guiSnapshotCh:
		default:
		}
		select {
		case guiSnapshotCh <- v:
		default:
		}
	}
}
func backgroundPoll(hwnd uintptr, done <-chan struct{}) {
	ticker := time.NewTicker(1100 * time.Millisecond)
	defer ticker.Stop()
	for {
		queueSnapshot(guiSnapshot{status: readStatus(statusPath), logs: readLogsBackground()})
		postMessage.Call(hwnd, WM_APP_SNAPSHOT, 0, 0)
		select {
		case <-done:
			return
		case <-ticker.C:
		}
	}
}
func renderSnapshot() {
	var snap guiSnapshot
	updated := false
	for {
		select {
		case snap = <-guiSnapshotCh:
			updated = true
		default:
			if !updated {
				return
			}
			statusChanged := ui.status.Mode != snap.status.Mode
			ui.status = snap.status
			latestLogs = snap.logs
			text := displayLogs(latestLogs)
			if text != ui.renderedLog {
				ui.renderedLog = text
				callText(ui.console, text)
				sendMessage.Call(ui.console, EM_SETSEL, ^uintptr(0), ^uintptr(0))
				sendMessage.Call(ui.console, EM_SCROLLCARET, 0, 0)
			}
			if statusChanged {
				repaint()
			}
			return
		}
	}
}

func w(s string) *uint16              { return syscall.StringToUTF16Ptr(s) }
func color(r, g, b byte) uintptr      { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }
func callText(hwnd uintptr, s string) { setWindowText.Call(hwnd, uintptr(unsafe.Pointer(w(s)))) }
func clientText(hwnd uintptr) string {
	b := make([]uint16, 32768)
	getWindowText.Call(hwnd, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return syscall.UTF16ToString(b)
}
func createControl(class, text string, style uint32, x, y, width, height int, id int, parent uintptr) uintptr {
	hwnd, _, _ := createWindowEx.Call(0, uintptr(unsafe.Pointer(w(class))), uintptr(unsafe.Pointer(w(text))), uintptr(style|WS_CHILD|WS_VISIBLE), uintptr(x), uintptr(y), uintptr(width), uintptr(height), parent, uintptr(id), 0, 0)
	return hwnd
}
func font(name string, size int, bold bool) uintptr {
	weight := uintptr(400)
	if bold {
		weight = 600
	}
	h, _, _ := createFont.Call(uintptr(int32(-size)), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(w(name))))
	return h
}
func setFont(hwnd, f uintptr) { sendMessage.Call(hwnd, WM_SETFONT, f, 1) }
func txt(hdc uintptr, s string, r rect, fontHandle, colorValue uintptr, flags uintptr) {
	old, _, _ := selectObject.Call(hdc, fontHandle)
	setBkMode.Call(hdc, TRANSPARENT)
	setTextColor.Call(hdc, colorValue)
	drawText.Call(hdc, uintptr(unsafe.Pointer(w(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags|DT_NOPREFIX)
	selectObject.Call(hdc, old)
}
func rectangle(hdc uintptr, r rect, brush uintptr) {
	fillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
}

// Rounded surfaces share the same geometry for controls and painted containers.
func roundedSurface(hdc uintptr, r rect, fill, border uintptr, radius int32) {
	b, _, _ := createSolidBrush.Call(fill)
	pen, _, _ := createPen.Call(0, 1, border)
	ob, _, _ := selectObject.Call(hdc, b)
	op, _, _ := selectObject.Call(hdc, pen)
	roundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius*2), uintptr(radius*2))
	selectObject.Call(hdc, ob)
	selectObject.Call(hdc, op)
	deleteObject.Call(b)
	deleteObject.Call(pen)
}
func drawButton(di *drawItem) uintptr {
	hdc, id, r := di.HDC, int(di.CtlID), di.ItemRect
	surface := color(22, 22, 25)
	if id == IDC_START || id == IDC_STOP || id == IDC_AUTOSTART {
		surface = color(28, 28, 31)
	}
	bg, _, _ := createSolidBrush.Call(surface)
	rectangle(hdc, r, bg)
	deleteObject.Call(bg)
	base, fg, border := color(36, 36, 40), color(230, 230, 233), color(49, 49, 54)
	if id == IDC_START {
		base, fg, border = color(239, 91, 25), color(255, 255, 255), color(239, 91, 25)
	}
	if id == IDC_LANGUAGE || id == IDC_TUTORIAL || id == IDC_DETAILS {
		base, border = surface, surface
		fg = color(156, 156, 165)
	}
	if hoveredButton == di.HwndItem {
		base = color(49, 49, 54)
		border = color(67, 67, 73)
		fg = color(255, 255, 255)
		if id == IDC_START {
			base = color(255, 112, 45)
			border = base
		}
	}
	if di.ItemState&1 != 0 {
		base = color(58, 49, 43)
		border = color(116, 75, 50)
	}
	if id == IDC_INSTALL && ui.busy {
		base = color(45, 39, 34)
		fg = color(189, 156, 133)
	}
	if di.ItemState&0x10 != 0 && di.ItemState&0x200 == 0 {
		border = color(226, 126, 74)
	}
	roundedSurface(hdc, rect{r.Left, r.Top, r.Right - 1, r.Bottom - 1}, base, border, 9)
	if id == IDC_AUTOSTART {
		txt(hdc, tr("Avec Windows", "With Windows"), rect{r.Left + 12, r.Top, r.Right - 60, r.Bottom}, ui.normalFont, fg, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		track, knob := color(68, 68, 76), color(209, 209, 215)
		if ui.auto {
			track = color(227, 83, 18)
			knob = color(255, 255, 255)
		}
		toggle := rect{r.Right - 52, r.Top + 12, r.Right - 12, r.Top + 32}
		roundedSurface(hdc, toggle, track, track, 10)
		x := toggle.Left + 3
		if ui.auto {
			x += 20
		}
		roundedSurface(hdc, rect{x, toggle.Top + 3, x + 14, toggle.Top + 17}, knob, knob, 7)
	} else {
		f := ui.normalFont
		if id == IDC_START {
			f = ui.boldFont
		}
		txt(hdc, buttonLabel(id), rect{r.Left + 8, r.Top, r.Right - 8, r.Bottom}, f, fg, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	return 1
}
func displayLogs(raw string) string {
	if detailedLogs {
		return raw
	}
	var lines []string
	for _, line := range strings.Split(raw, "\r\n") {
		if strings.TrimSpace(line) == "" || line == "Watcher" || strings.Contains(line, "STARTUP:") || strings.Contains(line, "ELR Desktop started.") || strings.Contains(line, "Win32 message pump") {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return tr("Aucune activité pour le moment.", "No activity yet.")
	}
	return strings.Join(lines, "\r\n\r\n")
}
func tr(fr, en string) string {
	if ui.lang == "en" {
		return en
	}
	return fr
}
func buttonLabel(id int) string {
	switch id {
	case IDC_DETAILS:
		if detailedLogs {
			return tr("Vue simple", "Simple view")
		}
		return tr("Détails", "Details")
	case IDC_INSTALL:
		if ui.busy {
			return tr("Installation…", "Installing…")
		}
		return tr("Installer / MAJ", "Install / update")
	case IDC_BROWSE:
		return tr("Parcourir", "Browse")
	case IDC_START:
		return tr("Démarrer", "Start")
	case IDC_STOP:
		return tr("Arrêter", "Stop")
	case IDC_AUTOSTART:
		if ui.auto {
			return tr("Démarrage auto  ·  Activé", "Auto-start  ·  On")
		}
		return tr("Démarrage auto  ·  Désactivé", "Auto-start  ·  Off")
	case IDC_LANGUAGE:
		if ui.lang == "fr" {
			return "EN"
		}
		return "FR"
	case IDC_TUTORIAL:
		if ui.guide {
			return tr("Retour au journal", "Back to activity")
		}
		return tr("Guide", "Guide")
	case IDC_FOLDER:
		return tr("Ouvrir le dossier", "Open folder")
	}
	return ""
}
func repaint() {
	invalidateRect.Call(ui.hwnd, 0, 0)
	for id, h := range ui.controls {
		callText(h, buttonLabel(id))
		invalidateRect.Call(h, 0, 0)
	}
}
func drawGUI(hdc uintptr) {
	var rc rect
	getClientRect.Call(ui.hwnd, uintptr(unsafe.Pointer(&rc)))
	rectangle(hdc, rc, ui.darkBrush)
	white, muted, orange := color(240, 240, 243), color(148, 148, 158), color(243, 105, 39)
	label := func(s string, x, y, right, bottom int32, f, c uintptr) {
		txt(hdc, s, rect{x, y, right, bottom}, f, c, DT_LEFT|DT_WORDBREAK)
	}
	label("Extreme Lua Refresh", 32, 25, 720, 59, ui.titleFont, white)
	label("Garry’s Mod", 33, 65, 720, 89, ui.normalFont, muted)
	label(tr("Dossier du jeu", "Game folder"), 32, 115, 400, 137, ui.boldFont, white)
	roundedSurface(hdc, rect{32, 148, 680, 194}, color(30, 30, 34), color(47, 47, 52), 9)
	roundedSurface(hdc, rect{32, 230, 344, 584}, color(28, 28, 31), color(43, 43, 48), 12)
	label(tr("Surveillance", "Monitoring"), 56, 254, 320, 279, ui.boldFont, white)
	status, desc, c := tr("À l’arrêt", "Stopped"), tr("Démarre le moniteur pour surveiller GMod.", "Start the monitor to watch for GMod."), muted
	switch ui.status.Mode {
	case "waiting":
		status, desc, c = tr("En attente", "Waiting"), tr("Le moniteur attend le lancement de GMod.", "The monitor is waiting for GMod to launch."), orange
	case "running":
		status, desc, c = tr("En cours", "Running"), tr("GMod est ouvert. Le watcher est actif.", "GMod is open. The watcher is active."), color(119, 196, 150)
	case "error":
		status, desc, c = tr("À vérifier", "Needs attention"), tr("Consulte le journal pour voir l’erreur.", "Check the activity log for the error."), color(230, 129, 113)
	}
	label(status, 56, 298, 320, 333, ui.titleFont, c)
	label(desc, 56, 346, 314, 390, ui.normalFont, muted)
	label(tr("Démarrage automatique", "Automatic start"), 56, 481, 320, 505, ui.normalFont, muted)
	label(tr("Journal", "Activity"), 368, 232, 650, 257, ui.boldFont, white)
	roundedSurface(hdc, rect{368, 276, 968, 584}, color(18, 18, 21), color(41, 41, 46), 12)
	if ui.guide {
		label(tr("Premiers pas", "Getting started"), 388, 292, 940, 319, ui.boldFont, white)
		label(tr("1. Sélectionne le dossier garrysmod et installe ELR.\n\n2. Démarre le moniteur, puis lance Garry’s Mod.\n\n3. Au premier lancement, redémarre GMod si une partie était déjà ouverte.", "1. Select the garrysmod folder and install ELR.\n\n2. Start the monitor, then launch Garry’s Mod.\n\n3. On first installation, restart GMod if a game was already open."), 388, 335, 940, 473, ui.normalFont, muted)
		label("elr_status  /  elr_force nom_addon", 388, 494, 940, 516, ui.monoFont, orange)
		label(tr("Lua uniquement. Le moniteur reste actif après fermeture.", "Lua only. The monitor stays active after closing this window."), 388, 538, 940, 569, ui.normalFont, muted)
	}
}
func wndProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	switch message {
	case WM_TIMER:
		if wparam == 1 {
			var p point
			getCursorPos.Call(uintptr(unsafe.Pointer(&p)))
			hit, _, _ := windowFromPoint.Call(uintptr(uint64(uint32(p.X)) | uint64(uint32(p.Y))<<32))
			next := uintptr(0)
			for _, h := range ui.controls {
				if hit == h {
					next = h
					break
				}
			}
			if next != hoveredButton {
				old := hoveredButton
				hoveredButton = next
				if old != 0 {
					invalidateRect.Call(old, 0, 0)
				}
				if next != 0 {
					invalidateRect.Call(next, 0, 0)
				}
			}
		}
		return 0
	case WM_CREATE:
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps paintstruct
		hdc, _, _ := beginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		drawGUI(hdc)
		endPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_CTLCOLOREDIT, WM_CTLCOLORSTATIC:
		hdc := wparam
		setTextColor.Call(hdc, color(207, 207, 214))
		if lparam == ui.console {
			setBkColor.Call(hdc, color(18, 18, 21))
			return ui.consoleBrush
		}
		setBkColor.Call(hdc, color(30, 30, 34))
		return ui.editBrush
	case WM_DRAWITEM:
		di := (*drawItem)(unsafe.Pointer(lparam))
		return drawButton(di)
	case WM_COMMAND:
		if uintptr(uint16(wparam)) != 0 {
			handleButton(int(uint16(wparam)))
		}
		return 0
	case WM_APP_SNAPSHOT:
		renderSnapshot()
		return 0
	case WM_APP_PATH:
		select {
		case found := <-guiPathCh:
			// Never overwrite a folder typed by the user while discovery runs.
			if ui.pathEdit != 0 && strings.TrimSpace(clientText(ui.pathEdit)) == "" && found != "" {
				callText(ui.pathEdit, found)
				logEvent("Steam/GMod folder detected: " + found)
			}
		default:
		}
		return 0
	case WM_APP_NOTIFY:
		ui.busy = false
		repaint()
		return 0
	case WM_DESTROY:
		killTimer.Call(hwnd, 1)
		for _, h := range []uintptr{ui.darkBrush, ui.editBrush, ui.consoleBrush, ui.normalFont, ui.boldFont, ui.titleFont, ui.monoFont} {
			if h != 0 {
				deleteObject.Call(h)
			}
		}
		postQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := defWindowProc.Call(hwnd, uintptr(message), wparam, lparam)
	return ret
}
func message(title, msg string, errorFlag bool) {
	flag := uintptr(MB_OK | MB_ICONINFORMATION)
	if errorFlag {
		flag = MB_OK | MB_ICONERROR
	}
	messageBox.Call(ui.hwnd, uintptr(unsafe.Pointer(w(msg))), uintptr(unsafe.Pointer(w(title))), flag)
}
func logEvent(text string) {
	_ = os.MkdirAll(appHome, 0755)
	f, err := os.OpenFile(appLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] %s\r\n", time.Now().Format("15:04:05"), text)
}

var monitorStartMu sync.Mutex
var lastMonitorStart time.Time

func monitorAlreadyRunning() bool {
	mutexName := w("Local\\ELR_WatcherMonitor_v1")
	handle, _, _ := openMutex.Call(0x00100000, 0, uintptr(unsafe.Pointer(mutexName))) // SYNCHRONIZE
	if handle == 0 {
		return false
	}
	closeHandle.Call(handle)
	return true
}

func startMonitor() error {
	monitorStartMu.Lock()
	defer monitorStartMu.Unlock()
	if monitorAlreadyRunning() {
		return nil
	}
	// Avoid a second launch while the first agent is acquiring its mutex.
	if time.Since(lastMonitorStart) < 3*time.Second {
		return nil
	}
	resident, err := installedResident(loadSettings(cfgPath), appHome)
	if err != nil {
		return err
	}
	_ = os.Remove(stopFlag)
	cmd := exec.Command(resident, "--monitor")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		return err
	}
	lastMonitorStart = time.Now()
	return nil
}
func stopMonitor() {
	_ = os.WriteFile(stopFlag, []byte("stop"), 0600)
	logEvent("Stop requested")
}
func regStartup(yes bool) error {
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	if !yes {
		out, err := exec.Command("reg.exe", "delete", key, "/v", "ExtremeLuaRefresh", "/f").CombinedOutput()
		if err != nil && !strings.Contains(strings.ToLower(string(out)), "unable") && !strings.Contains(strings.ToLower(string(out)), "introuvable") && !strings.Contains(strings.ToLower(string(out)), "cannot find") {
			return fmt.Errorf("reg delete: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	resident, err := installedResident(loadSettings(cfgPath), appHome)
	if err != nil {
		return err
	}
	value := `"` + resident + `" --monitor`
	out, err := exec.Command("reg.exe", "add", key, "/v", "ExtremeLuaRefresh", "/t", "REG_SZ", "/d", value, "/f").CombinedOutput()
	if err != nil {
		return fmt.Errorf("reg add: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
func handleButton(id int) {
	switch id {
	case IDC_DETAILS:
		detailedLogs = !detailedLogs
		ui.renderedLog = displayLogs(latestLogs)
		callText(ui.console, ui.renderedLog)
		sendMessage.Call(ui.console, EM_SETSEL, ^uintptr(0), ^uintptr(0))
		sendMessage.Call(ui.console, EM_SCROLLCARET, 0, 0)
		repaint()

	case IDC_LANGUAGE:
		if ui.lang == "fr" {
			ui.lang = "en"
		} else {
			ui.lang = "fr"
		}
		s := loadSettings(cfgPath)
		s.Language = ui.lang
		_ = saveJSON(cfgPath, s)
		for _, ctrl := range []int{IDC_INSTALL, IDC_BROWSE, IDC_START, IDC_STOP, IDC_AUTOSTART, IDC_LANGUAGE, IDC_TUTORIAL, IDC_FOLDER} {
			if h := ui.controls[ctrl]; h != 0 {
				callText(h, buttonLabel(ctrl))
			}
		}
		ui.renderedLog = "" // update translated header on next background snapshot
		repaint()
	case IDC_TUTORIAL:
		ui.guide = !ui.guide
		if ui.guide {
			showWindow.Call(ui.console, 0)
			showWindow.Call(ui.controls[IDC_DETAILS], 0)
		} else {
			showWindow.Call(ui.console, SW_SHOW)
			showWindow.Call(ui.controls[IDC_DETAILS], SW_SHOW)
		}
		repaint()
	case IDC_BROWSE:
		var name [260]uint16
		bi := browseInfo{Owner: ui.hwnd, DisplayName: &name[0], Title: w(tr("Sélectionne le dossier GarrysMod ou garrysmod", "Select GarrysMod or garrysmod folder")), Flags: BIF_NEWDIALOGSTYLE | BIF_RETURNONLYFSDIRS}
		pidl, _, _ := browseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
		if pidl != 0 {
			var buf [32768]uint16
			ok, _, _ := getFolderPath.Call(pidl, uintptr(unsafe.Pointer(&buf[0])))
			cotaskmemfree.Call(pidl)
			if ok != 0 {
				callText(ui.pathEdit, syscall.UTF16ToString(buf[:]))
			}
		}
	case IDC_INSTALL:
		if ui.busy {
			return
		}
		ui.busy = true
		repaint()
		destination := clientText(ui.pathEdit)
		go func() {
			root, err := canonicalGmodFolder(destination)
			if err != nil {
				logEvent("INSTALL ERROR: " + err.Error())
				postMessage.Call(ui.hwnd, WM_APP_NOTIFY, 0, 0)
				return
			}
			files, err := installPayload(root)
			if err != nil {
				logEvent("INSTALL ERROR: " + err.Error())
				postMessage.Call(ui.hwnd, WM_APP_NOTIFY, 0, 0)
				return
			}
			_ = os.MkdirAll(appHome, 0755)
			self, err := os.Executable()
			if err != nil {
				logEvent("INSTALL ERROR executable: " + err.Error())
				postMessage.Call(ui.hwnd, WM_APP_NOTIFY, 0, 0)
				return
			}
			resident, err := installResidentExecutable(self, appHome)
			if err != nil {
				logEvent("INSTALL ERROR resident: " + err.Error())
				postMessage.Call(ui.hwnd, WM_APP_NOTIFY, 0, 0)
				return
			}
			s := loadSettings(cfgPath)
			s.GameDir = root
			s.Language = ui.lang
			s.Resident = resident
			if err := saveJSON(cfgPath, s); err != nil {
				logEvent("Settings error: " + err.Error())
			}
			if s.Auto {
				if err := regStartup(true); err != nil {
					logEvent("Auto-start error: " + err.Error())
				}
				if err := startMonitor(); err != nil {
					logEvent("Monitor start error: " + err.Error())
				}
			}
			logEvent("Resident version: " + resident)
			logEvent(fmt.Sprintf("Installed %d ELR files in %s", files, filepath.Join(root, "addons", addonFolder)))
			logEvent("If the GMod Lua bootstrap was not loaded before, restart GMod ONCE. After that, Lua updates are hot-reloaded.")
			postMessage.Call(ui.hwnd, WM_APP_NOTIFY, 0, 0)
		}()
	case IDC_AUTOSTART:
		if _, err := installedResident(loadSettings(cfgPath), appHome); err != nil {
			message("ELR", tr("Clique d'abord sur INSTALLER.", "Click INSTALL first."), true)
			return
		}
		s := loadSettings(cfgPath)
		s.Auto = !s.Auto
		s.Language = ui.lang
		if err := regStartup(s.Auto); err != nil {
			message("ELR", err.Error(), true)
			return
		}
		if err := saveJSON(cfgPath, s); err != nil {
			message("ELR", err.Error(), true)
			return
		}
		ui.auto = s.Auto
		if s.Auto {
			if err := startMonitor(); err != nil {
				logEvent("Monitor start: " + err.Error())
			}
			logEvent("Automatic monitor enabled at Windows logon")
		} else {
			stopMonitor()
			logEvent("Automatic monitor disabled")
		}
		repaint()
	case IDC_START:
		if _, err := installedResident(loadSettings(cfgPath), appHome); err != nil {
			message("ELR", tr("Installe d'abord le programme.", "Install the application first."), true)
			return
		}
		if monitorAlreadyRunning() {
			logEvent("Monitor already running; duplicate start ignored")
		} else if err := startMonitor(); err != nil {
			logEvent("MONITOR START FAILED: " + err.Error())
		} else {
			logEvent("Monitor start requested")
		}
	case IDC_STOP:
		stopMonitor()
	case IDC_FOLDER:
		s := loadSettings(cfgPath)
		if s.GameDir == "" {
			message("ELR", tr("Installe d'abord ELR.", "Install ELR first."), true)
			return
		}
		_ = exec.Command("explorer.exe", filepath.Join(s.GameDir, "addons", addonFolder)).Start()
	}
}
func tail(path string, max int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return ""
	}
	start := stat.Size() - int64(max)
	if start < 0 {
		start = 0
	}
	_, _ = f.Seek(start, io.SeekStart)
	b, _ := io.ReadAll(f)
	s := string(b)
	if start > 0 {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
}
func runGUI() {
	// Window creation and the Win32 message pump must live on the same OS thread.
	// Go can otherwise migrate the goroutine between Win32 calls.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	_ = os.MkdirAll(appHome, 0755)
	logEvent("STARTUP: process entered GUI")
	saved := loadSettings(cfgPath)
	ui.lang = saved.Language
	ui.auto = saved.Auto
	ui.darkBrush, _, _ = createSolidBrush.Call(color(22, 22, 25))
	ui.editBrush, _, _ = createSolidBrush.Call(color(30, 30, 34))
	ui.consoleBrush, _, _ = createSolidBrush.Call(color(18, 18, 21))
	ui.normalFont = font("Segoe UI", 15, false)
	ui.boldFont = font("Segoe UI", 15, true)
	ui.titleFont = font("Segoe UI", 26, true)
	ui.monoFont = font("Consolas", 13, false)
	className := w("ExtremeLuaRefreshDesktopWindow")
	inst, _, _ := getModuleHandle.Call(0)
	cursor, _, _ := loadCursor.Call(0, 32512)
	cls := wndclass{Size: uint32(unsafe.Sizeof(wndclass{})), Style: CS_HREDRAW | CS_VREDRAW, WindowProc: syscall.NewCallback(wndProc), Instance: inst, Cursor: cursor, Background: ui.darkBrush, ClassName: className}
	if ok, _, regErr := registerClass.Call(uintptr(unsafe.Pointer(&cls))); ok == 0 {
		logEvent("STARTUP ERROR: RegisterClassExW: " + regErr.Error())
		return
	}
	logEvent("STARTUP: Win32 class registered")
	style := uintptr(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX | WS_CLIPCHILDREN)
	bounds := rect{0, 0, 1000, 656}
	adjustWindowRect.Call(uintptr(unsafe.Pointer(&bounds)), style, 0)
	width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
	sw, _, _ := getSystemMetrics.Call(0)
	sh, _, _ := getSystemMetrics.Call(1)
	x, y := (int32(sw)-width)/2, (int32(sh)-height)/2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	hwnd, _, err := createWindowEx.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(w("Extreme Lua Refresh"))), style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0, inst, 0)
	if hwnd == 0 {
		logEvent("STARTUP ERROR: CreateWindowEx: " + err.Error())
		fmt.Fprintln(os.Stderr, "CreateWindowEx failed:", err)
		return
	}
	logEvent("STARTUP: window created")
	ui.hwnd = hwnd
	// Keep the native title bar and system scaling; use dark chrome where supported.
	if dwmSetWindowAttribute.Find() == nil {
		dark := int32(1)
		dwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4)
		caption := uint32(color(22, 22, 25))
		dwmSetWindowAttribute.Call(hwnd, 35, uintptr(unsafe.Pointer(&caption)), 4)
	}
	ui.controls = make(map[int]uintptr)
	gameDir := saved.GameDir
	// Never invoke reg.exe, read Steam VDFs, or stat arbitrary drives here.
	// Discovery runs asynchronously after the window has been painted.
	ui.pathEdit = createControl("EDIT", gameDir, ES_AUTOHSCROLL|WS_TABSTOP, 46, 162, 620, 23, IDC_PATH, hwnd)
	setFont(ui.pathEdit, ui.normalFont)
	newButton := func(id int, x, y, width, height int) {
		h := createControl("BUTTON", buttonLabel(id), BS_OWNERDRAW|WS_TABSTOP, x, y, width, height, id, hwnd)
		ui.controls[id] = h
		setFont(h, ui.boldFont)
	}
	newButton(IDC_LANGUAGE, 916, 28, 52, 32)
	newButton(IDC_BROWSE, 692, 148, 112, 46)
	newButton(IDC_INSTALL, 816, 148, 152, 46)
	newButton(IDC_START, 56, 409, 164, 46)
	newButton(IDC_STOP, 230, 409, 90, 46)
	newButton(IDC_AUTOSTART, 56, 516, 264, 44)
	newButton(IDC_FOLDER, 32, 608, 168, 32)
	newButton(IDC_TUTORIAL, 800, 608, 168, 32)
	newButton(IDC_DETAILS, 864, 225, 104, 36)
	ui.console = createControl("EDIT", "", WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY, 384, 292, 568, 276, IDC_CONSOLE, hwnd)
	setFont(ui.console, ui.monoFont)
	showWindow.Call(hwnd, SW_SHOW)
	updateWindow.Call(hwnd)
	setTimer.Call(hwnd, 1, 65, 0)
	logEvent("STARTUP: controls created; Win32 message pump ready")
	done := make(chan struct{})
	go backgroundPoll(hwnd, done)
	if gameDir == "" {
		go func() {
			found := discoverGmod()
			select {
			case guiPathCh <- found:
			default:
			}
			postMessage.Call(hwnd, WM_APP_PATH, 0, 0)
		}()
	}
	var m msg
	for {
		v, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(v) <= 0 {
			break
		}
		if handled, _, _ := isDialogMessage.Call(hwnd, uintptr(unsafe.Pointer(&m))); handled != 0 {
			continue
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	close(done)
	logEvent("GUI closed")
}

// Steam installs: default folder + additional Steam library folders.
func discoverGmod() string {
	roots := []string{}
	if p := os.Getenv("ProgramFiles(x86)"); p != "" {
		roots = append(roots, filepath.Join(p, "Steam"))
	}
	if p := os.Getenv("ProgramFiles"); p != "" {
		roots = append(roots, filepath.Join(p, "Steam"))
	}
	if p := os.Getenv("LOCALAPPDATA"); p != "" {
		roots = append(roots, filepath.Join(p, "Steam"))
	}
	if p := os.Getenv("STEAM_PATH"); p != "" {
		roots = append(roots, p)
	}
	if data, err := exec.Command("reg.exe", "query", `HKCU\Software\Valve\Steam`, "/v", "SteamPath").Output(); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if idx := strings.Index(line, "REG_SZ"); idx >= 0 {
				roots = append(roots, strings.TrimSpace(line[idx+len("REG_SZ"):]))
			}
		}
	}
	seen := map[string]bool{}
	for _, root := range roots {
		root = filepath.Clean(strings.ReplaceAll(root, "/", string(os.PathSeparator)))
		libs := []string{root}
		vdf, _ := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
		rx := regexp.MustCompile(`(?i)"path"\s+"([^"]+)"`)
		for _, m := range rx.FindAllStringSubmatch(string(vdf), -1) {
			libs = append(libs, strings.ReplaceAll(m[1], `\\`, `\`))
		}
		for _, lib := range libs {
			k := strings.ToLower(lib)
			if seen[k] {
				continue
			}
			seen[k] = true
			loc := filepath.Join(lib, "steamapps", "common", "GarrysMod", "garrysmod")
			if _, err := canonicalGmodFolder(loc); err == nil {
				return loc
			}
		}
	}
	return ""
}

func monitorGmodRunning(root string) bool {
	// TH32CS_SNAPPROCESS = 2
	snap, _, _ := createSnapshot.Call(2, 0)
	if snap == 0 || snap == ^uintptr(0) {
		return false
	}
	defer closeHandle.Call(snap)
	p := processEntry{Size: uint32(unsafe.Sizeof(processEntry{}))}
	ok, _, _ := process32First.Call(snap, uintptr(unsafe.Pointer(&p)))
	for ok != 0 {
		name := strings.ToLower(syscall.UTF16ToString(p.Exe[:]))
		if name == "gmod.exe" || name == "gmod_x64.exe" || name == "hl2.exe" {
			h, _, _ := openProcess.Call(0x1000, 0, uintptr(p.ProcessID)) // PROCESS_QUERY_LIMITED_INFORMATION
			if h != 0 {
				var buf [32768]uint16
				sz := uint32(len(buf))
				good, _, _ := queryProcessName.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&sz)))
				closeHandle.Call(h)
				if good != 0 {
					gameExe := strings.ToLower(filepath.Clean(syscall.UTF16ToString(buf[:sz])))
					base := strings.ToLower(filepath.Clean(filepath.Dir(root)))
					if strings.HasPrefix(gameExe, base+string(os.PathSeparator)) {
						return true
					}
				} else if name == "gmod.exe" {
					return true
				}
			}
		}
		ok, _, _ = process32Next.Call(snap, uintptr(unsafe.Pointer(&p)))
	}
	return false
}

func runMonitor() {
	_ = os.MkdirAll(appHome, 0755)
	name := w("Local\\ELR_WatcherMonitor_v1")
	h, _, createErr := createMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return
	}
	defer closeHandle.Call(h)
	if createErr == syscall.Errno(ERROR_ALREADY_EXISTS) {
		return
	}
	_ = os.Remove(stopFlag)
	logEvent("Monitor agent started")
	var child *exec.Cmd
	var lastRoot string
	var nextStart time.Time
	var recentError string
	var pid int
	killChild := func() {
		if child != nil {
			_ = child.Process.Kill()
			_, _ = child.Process.Wait()
			child = nil
			pid = 0
			logEvent("Watcher stopped")
		}
	}
	defer func() {
		killChild()
		_ = saveJSON(statusPath, MonitorStatus{Mode: "offline", Timestamp: time.Now().Unix()})
		logEvent("Monitor agent stopped")
	}()
	for {
		if _, err := os.Stat(stopFlag); err == nil {
			_ = os.Remove(stopFlag)
			return
		}
		s := loadSettings(cfgPath)
		if s.GameDir == "" {
			_ = saveJSON(statusPath, MonitorStatus{Mode: "error", Timestamp: time.Now().Unix(), Error: "Not installed"})
			time.Sleep(time.Second)
			continue
		}
		root, err := canonicalGmodFolder(s.GameDir)
		if err != nil {
			killChild()
			recentError = err.Error()
			_ = saveJSON(statusPath, MonitorStatus{Mode: "error", Timestamp: time.Now().Unix(), Error: recentError})
			time.Sleep(time.Second)
			continue
		}
		if lastRoot != "" && !strings.EqualFold(lastRoot, root) {
			killChild()
		}
		lastRoot = root
		game := monitorGmodRunning(root)
		if !game {
			killChild()
			nextStart = time.Time{}
		}
		if game && child != nil {
			if child.ProcessState != nil && child.ProcessState.Exited() {
				child = nil
				pid = 0
				nextStart = time.Now().Add(4 * time.Second)
			}
			// PowerShell may exit unexpectedly; detect using a non-blocking query.
			if child != nil {
				active, _, _ := getExitCodeProcess(child.Process.Pid)
				if !active {
					_, _ = child.Process.Wait()
					child = nil
					pid = 0
					nextStart = time.Now().Add(4 * time.Second)
					logEvent("Watcher unexpectedly exited; retry in 4s")
				}
			}
		}
		if game && child == nil && time.Now().After(nextStart) {
			script := filepath.Join(root, "addons", addonFolder, "tools", "Watcher.ps1")
			if _, err := os.Stat(script); err != nil {
				recentError = "Watcher.ps1 missing"
				logEvent("Missing Watcher.ps1")
			}
			if _, err := os.Stat(script); err == nil {
				f, e := os.OpenFile(watcherLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
				if e == nil {
					cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", script)
					cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
					cmd.Stdout = f
					cmd.Stderr = f
					if e = cmd.Start(); e == nil {
						child = cmd
						pid = cmd.Process.Pid
						recentError = ""
						logEvent(fmt.Sprintf("GMod running, watcher launched (PID %d)", pid))
					}
					f.Close()
				}
				if e != nil {
					recentError = e.Error()
					logEvent("Watcher start error: " + recentError)
					nextStart = time.Now().Add(5 * time.Second)
				}
			}
		}
		status := MonitorStatus{Mode: "waiting", Game: game, Watcher: child != nil, PID: pid, Timestamp: time.Now().Unix(), Error: recentError}
		if game && child != nil {
			status.Mode = "running"
		}
		if game && child == nil {
			status.Mode = "error"
		}
		_ = saveJSON(statusPath, status)
		time.Sleep(1200 * time.Millisecond)
	}
}
func getExitCodeProcess(pid int) (bool, uint32, error) {
	h, _, e := openProcess.Call(0x1000, 0, uintptr(pid))
	if h == 0 {
		return false, 0, e
	}
	defer closeHandle.Call(h)
	proc := kernel32.NewProc("GetExitCodeProcess")
	var code uint32
	ok, _, err := proc.Call(h, uintptr(unsafe.Pointer(&code)))
	if ok == 0 {
		return false, code, err
	}
	return code == 259, code, nil // STILL_ACTIVE
}
func main() {
	// The monitor uses no interactive window; GUI uses the Win32 message loop.
	for _, arg := range os.Args[1:] {
		if arg == "--monitor" {
			runMonitor()
			return
		}
	}
	runGUI()
}
