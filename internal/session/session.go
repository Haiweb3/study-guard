package session

import (
	"errors"
	"sync"
	"time"
)

type State string

const (
	StateIdle   State = "idle"
	StateActive State = "active"
)

var (
	ErrAlreadyActive = errors.New("学习模式已经在进行中")
	ErrNotActive     = errors.New("当前没有进行中的学习模式")
)

// Info is a snapshot of the session state, safe to serialise and send to the frontend.
type Info struct {
	State     State      `json:"state"`
	Mode      string     `json:"mode,omitempty"`
	Apps      []string   `json:"apps,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndsAt    *time.Time `json:"endsAt,omitempty"`
}

// Manager is a small state machine: idle <-> active, with an optional timer
// for sessions that should end automatically. It knows nothing about what a
// session actually blocks - that's wired up by the caller via onAutoEnd.
type Manager struct {
	mu    sync.Mutex
	info  Info
	timer *time.Timer
}

func NewManager() *Manager {
	return &Manager{info: Info{State: StateIdle}}
}

// Start activates a session. durationMin <= 0 means manual (no auto end).
// onAutoEnd runs on its own goroutine when the timer fires.
func (m *Manager) Start(mode string, apps []string, durationMin float64, onAutoEnd func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.info.State == StateActive {
		return ErrAlreadyActive
	}

	now := time.Now()
	info := Info{State: StateActive, Mode: mode, Apps: apps, StartedAt: &now}
	if durationMin > 0 {
		end := now.Add(time.Duration(durationMin * float64(time.Minute)))
		info.EndsAt = &end
		m.timer = time.AfterFunc(time.Until(end), onAutoEnd)
	}
	m.info = info
	return nil
}

// Stop deactivates the session and returns the info as it was right before stopping.
func (m *Manager) Stop() (Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.info.State != StateActive {
		return Info{}, ErrNotActive
	}
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	prev := m.info
	m.info = Info{State: StateIdle}
	return prev, nil
}

func (m *Manager) Snapshot() Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.info
}
