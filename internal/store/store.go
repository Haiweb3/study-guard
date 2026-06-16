package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Mode string

const (
	ModeQuiet Mode = "quiet"
	ModeKill  Mode = "kill"
)

// BlacklistApp is one entry in the user's custom blacklist.
// ProcessName is what gets passed to `pkill -x` in kill mode.
type BlacklistApp struct {
	Name        string `json:"name"`
	ProcessName string `json:"processName"`
}

type Config struct {
	Blacklist []BlacklistApp `json:"blacklist"`
}

func defaultConfig() Config {
	return Config{
		Blacklist: []BlacklistApp{
			{Name: "微信", ProcessName: "WeChat"},
			{Name: "QQ", ProcessName: "QQ"},
		},
	}
}

type SessionRecord struct {
	StartedAt   time.Time `json:"startedAt"`
	EndedAt     time.Time `json:"endedAt"`
	DurationMin float64   `json:"durationMin"`
	Mode        Mode      `json:"mode"`
	Apps        []string  `json:"apps"`
	AutoEnded   bool      `json:"autoEnded"`
}

// Store persists config and session history as plain JSON files under
// ~/Library/Application Support/StudyGuard. The data is tiny, so a database
// would be overkill.
type Store struct {
	mu          sync.Mutex
	configPath  string
	historyPath string
}

func New() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, "Library", "Application Support", "StudyGuard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{
		configPath:  filepath.Join(dir, "config.json"),
		historyPath: filepath.Join(dir, "sessions.json"),
	}, nil
}

func (s *Store) LoadConfig() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.configPath)
	if errors.Is(err, os.ErrNotExist) {
		cfg := defaultConfig()
		return cfg, s.writeConfigLocked(cfg)
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (s *Store) SaveConfig(cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeConfigLocked(cfg)
}

func (s *Store) writeConfigLocked(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, data, 0o644)
}

func (s *Store) AppendSession(rec SessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.readHistoryLocked()
	if err != nil {
		return err
	}
	records = append(records, rec)
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.historyPath, data, 0o644)
}

// LoadHistory returns the most recent records first. limit <= 0 means no limit.
func (s *Store) LoadHistory(limit int) ([]SessionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.readHistoryLocked()
	if err != nil {
		return nil, err
	}
	result := make([]SessionRecord, 0, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		result = append(result, records[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *Store) readHistoryLocked() ([]SessionRecord, error) {
	data, err := os.ReadFile(s.historyPath)
	if errors.Is(err, os.ErrNotExist) || len(data) == 0 {
		return []SessionRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	var records []SessionRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}
