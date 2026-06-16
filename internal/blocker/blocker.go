package blocker

import (
	"os/exec"
	"sync"
	"time"
)

const (
	pollInterval = 5 * time.Second

	// These must match the names of two Shortcuts the user creates once in
	// the Shortcuts app (see README): one that turns the "学习" Focus on,
	// one that turns it off. There is no public macOS API to create or edit
	// Focus filters programmatically, so this is the supported way to do it.
	ShortcutOn  = "学习模式-开"
	ShortcutOff = "学习模式-关"
)

// Blocker implements the two blocking modes:
//   - kill mode: repeatedly force-quit blacklisted processes via pkill, so the
//     user can't just relaunch them mid-session.
//   - quiet mode: toggle a pre-configured macOS Focus via `shortcuts run`.
type Blocker struct {
	mu   sync.Mutex
	stop chan struct{}
}

func New() *Blocker {
	return &Blocker{}
}

// StartKill force-quits processNames immediately and keeps re-checking every
// few seconds until StopKill is called.
func (b *Blocker) StartKill(processNames []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stop != nil {
		return
	}
	stop := make(chan struct{})
	b.stop = stop
	killAll(processNames)

	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				killAll(processNames)
			}
		}
	}()
}

func (b *Blocker) StopKill() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stop == nil {
		return
	}
	close(b.stop)
	b.stop = nil
}

func killAll(processNames []string) {
	for _, name := range processNames {
		if name == "" {
			continue
		}
		// Best-effort: it's fine if the app isn't running.
		_ = exec.Command("pkill", "-x", name).Run()
	}
}

func StartQuiet() error {
	return exec.Command("shortcuts", "run", ShortcutOn).Run()
}

func StopQuiet() error {
	return exec.Command("shortcuts", "run", ShortcutOff).Run()
}
