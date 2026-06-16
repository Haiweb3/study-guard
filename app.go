package main

import (
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"study-guard/internal/blocker"
	"study-guard/internal/session"
	"study-guard/internal/store"
)

// noiseProcesses are system/UI processes that show up alongside real apps
// when listing foreground processes, but that nobody would want to blacklist.
var noiseProcesses = map[string]bool{
	"Finder":             true,
	"Dock":               true,
	"SystemUIServer":     true,
	"ControlCenter":      true,
	"NotificationCenter": true,
	"loginwindow":        true,
	"WindowManager":      true,
	"System Events":      true,
	"osascript":          true,
	"study-guard":        true,
	"sg_preview":         true,
}

// AppService is bound to the frontend. Every exported method here becomes a
// callable JS/TS function via the generated bindings.
type AppService struct {
	store   *store.Store
	session *session.Manager
	blocker *blocker.Blocker
}

func NewAppService(st *store.Store) *AppService {
	return &AppService{
		store:   st,
		session: session.NewManager(),
		blocker: blocker.New(),
	}
}

func (a *AppService) GetState() session.Info {
	return a.session.Snapshot()
}

func (a *AppService) GetConfig() (store.Config, error) {
	return a.store.LoadConfig()
}

// SaveBlacklist replaces the whole blacklist with the given list.
func (a *AppService) SaveBlacklist(apps []store.BlacklistApp) (store.Config, error) {
	cfg := store.Config{Blacklist: apps}
	if err := a.store.SaveConfig(cfg); err != nil {
		return store.Config{}, err
	}
	return cfg, nil
}

func (a *AppService) GetHistory(limit int) ([]store.SessionRecord, error) {
	return a.store.LoadHistory(limit)
}

// ListRunningApps returns the names of currently running foreground apps, so
// the frontend can offer a pick-list instead of requiring the user to look up
// exact process names in Activity Monitor. The names returned here are the
// same ones macOS uses internally, so they also work directly as the
// `pkill -x` target for kill mode.
func (a *AppService) ListRunningApps() ([]string, error) {
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get name of every process whose background only is false`).Output()
	if err != nil {
		return nil, fmt.Errorf("读取运行中的应用失败: %w", err)
	}

	raw := strings.Split(strings.TrimSpace(string(out)), ", ")
	seen := map[string]bool{}
	apps := make([]string, 0, len(raw))
	for _, name := range raw {
		name = strings.TrimSpace(name)
		if name == "" || noiseProcesses[name] || seen[name] {
			continue
		}
		seen[name] = true
		apps = append(apps, name)
	}
	sort.Strings(apps)
	return apps, nil
}

// StartSession begins blocking the current blacklist using the given mode
// ("quiet" or "kill"). durationMin <= 0 means the session only ends manually.
func (a *AppService) StartSession(mode string, durationMin float64) (session.Info, error) {
	if mode != string(store.ModeQuiet) && mode != string(store.ModeKill) {
		return session.Info{}, fmt.Errorf("未知模式: %s", mode)
	}
	if a.session.Snapshot().State == session.StateActive {
		return session.Info{}, session.ErrAlreadyActive
	}

	cfg, err := a.store.LoadConfig()
	if err != nil {
		return session.Info{}, err
	}
	names := make([]string, 0, len(cfg.Blacklist))
	procs := make([]string, 0, len(cfg.Blacklist))
	for _, app := range cfg.Blacklist {
		names = append(names, app.Name)
		procs = append(procs, app.ProcessName)
	}

	if mode == string(store.ModeQuiet) {
		if err := blocker.StartQuiet(); err != nil {
			return session.Info{}, fmt.Errorf("启动静音模式失败，请先按 README 配置好「学习」专注模式和快捷指令: %w", err)
		}
	}

	if err := a.session.Start(mode, names, durationMin, a.autoEnd); err != nil {
		return session.Info{}, err
	}
	if mode == string(store.ModeKill) {
		a.blocker.StartKill(procs)
	}

	info := a.session.Snapshot()
	a.emit(info)
	return info, nil
}

// EndSession is called once the frontend's own anti-laziness confirmation
// (countdown + explicit confirm) has completed.
func (a *AppService) EndSession() (store.SessionRecord, error) {
	return a.finish(false)
}

func (a *AppService) autoEnd() {
	if _, err := a.finish(true); err != nil {
		log.Println("自动结束学习模式失败:", err)
	}
}

func (a *AppService) finish(autoEnded bool) (store.SessionRecord, error) {
	prev, err := a.session.Stop()
	if err != nil {
		return store.SessionRecord{}, err
	}

	switch prev.Mode {
	case string(store.ModeKill):
		a.blocker.StopKill()
	case string(store.ModeQuiet):
		if err := blocker.StopQuiet(); err != nil {
			log.Println("关闭静音模式失败:", err)
		}
	}

	end := time.Now()
	rec := store.SessionRecord{
		StartedAt:   *prev.StartedAt,
		EndedAt:     end,
		DurationMin: end.Sub(*prev.StartedAt).Minutes(),
		Mode:        store.Mode(prev.Mode),
		Apps:        prev.Apps,
		AutoEnded:   autoEnded,
	}
	if err := a.store.AppendSession(rec); err != nil {
		return store.SessionRecord{}, err
	}

	a.emit(a.session.Snapshot())
	return rec, nil
}

func (a *AppService) emit(info session.Info) {
	if app := application.Get(); app != nil {
		app.Event.Emit("session:changed", info)
	}
}
