package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.design/x/hotkey"

	"study-guard/internal/session"
	"study-guard/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[session.Info]("session:changed")
}

func main() {
	st, err := store.New()
	if err != nil {
		log.Fatal(err)
	}
	svc := NewAppService(st)

	app := application.New(application.Options{
		Name:        "StudyGuard",
		Description: "学习专注屏蔽工具",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "StudyGuard",
		Width:            340,
		Height:           520,
		Frameless:        true,
		Hidden:           true,
		DisableResize:    true,
		AlwaysOnTop:      true,
		HideOnFocusLost:  true,
		HideOnEscape:     true,
		BackgroundColour: application.NewRGB(247, 247, 250),
		URL:              "/",
	})

	tray := app.SystemTray.New()
	tray.SetLabel(trayLabel(svc.GetState()))
	tray.SetTooltip("StudyGuard - 学习专注")
	tray.AttachWindow(window).WindowOffset(6)
	tray.Run()

	app.Event.On("session:changed", func(event *application.CustomEvent) {
		if info, ok := event.Data.(session.Info); ok {
			tray.SetLabel(trayLabel(info))
		}
	})

	registerToggleHotkey(window)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// registerToggleHotkey binds ⌃⌥S to show/hide the panel, so it's still
// reachable when the tray icon itself is hidden (e.g. behind a notch on a
// crowded menu bar). Registration needs macOS Accessibility (Input
// Monitoring) permission; if it's not granted yet, this just logs and the
// tray icon remains the only way in until the user grants it.
func registerToggleHotkey(window *application.WebviewWindow) {
	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModOption}, hotkey.KeyS)
	go func() {
		if err := hk.Register(); err != nil {
			log.Println("全局快捷键 ⌃⌥S 注册失败，需要在「系统设置 → 隐私与安全性 → 辅助功能」里授权后重启 App:", err)
			return
		}
		for range hk.Keydown() {
			if window.IsVisible() {
				window.Hide()
			} else {
				window.Show()
				window.Focus()
			}
		}
	}()
}

func trayLabel(info session.Info) string {
	if info.State != session.StateActive {
		return "📚"
	}
	if info.Mode == string(store.ModeKill) {
		return "🔒"
	}
	return "🔕"
}
