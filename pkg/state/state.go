// file: pkg/state/state.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Menedżer Stanu Wdrożeń                                             ║
// ║                                                                      ║
// ║  Zarządza plikiem .rnr/snapshots/state.json przechowującym:         ║
// ║    · Historię wszystkich wdrożeń                                    ║
// ║    · Informacje o snapshotach Git (gałęzie backup, tagi)            ║
// ║    · Dane do rollbacku (hash commita, środowisko, czas)             ║
// ║                                                                      ║
// ║  Plik state.json jest jedynym źródłem prawdy dla mechanizmu         ║
// ║  przywracania zmian. Chroniony przed uszkodzeniem przez atomowy     ║
// ║  zapis (write-rename pattern).                                      ║
// ╚══════════════════════════════════════════════════════════════════════╝

package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ─── Wersja formatu state.json ─────────────────────────────────────────────

const currentVersion = 1

// ─── Typy statusu ──────────────────────────────────────────────────────────

// DeployStatus reprezentuje stan wdrożenia.
type DeployStatus string

const (
	// StatusRunning — wdrożenie jest w toku.
	StatusRunning DeployStatus = "running"
	// StatusSuccess — wdrożenie zakończyło się sukcesem.
	StatusSuccess DeployStatus = "success"
	// StatusFailed — wdrożenie zakończyło się błędem.
	StatusFailed DeployStatus = "failed"
	// StatusRolledBack — wdrożenie zostało cofnięte.
	StatusRolledBack DeployStatus = "rolled_back"
)

// ─── Struktury danych ──────────────────────────────────────────────────────

// StageRecord przechowuje wynik wykonania pojedynczego etapu potoku.
type StageRecord struct {
	// Name — nazwa etapu.
	Name string `json:"name"`
	// Status — wynik: "success" | "failed" | "skipped" | "warning".
	Status string `json:"status"`
	// DurationMS — czas wykonania w milisekundach.
	DurationMS int64 `json:"duration_ms"`
	// Output — ostatnie linie wyjścia (zmasakrowane z sekretów).
	Output string `json:"output,omitempty"`
}

// SnapshotInfo przechowuje dane snapshotu Git wykonanego przed wdrożeniem.
type SnapshotInfo struct {
	// Branch — nazwa gałęzi zapasowej (np. "rnr_backup_production_20240115_143000").
	Branch string `json:"branch,omitempty"`
	// Tag — nazwa taga Git (np. "rnr-snap-production-20240115-143000").
	Tag string `json:"tag,omitempty"`
	// CommitHash — hash commita w momencie tworzenia snapshotu.
	CommitHash string `json:"commit_hash"`
	// CreatedAt — czas utworzenia snapshotu.
	CreatedAt time.Time `json:"created_at"`
}

// DeployRecord przechowuje kompletny rekord jednego wdrożenia.
type DeployRecord struct {
	// ID — unikalny identyfikator UUID wdrożenia.
	ID string `json:"id"`
	// Env — nazwa środowiska (np. "production", "staging").
	Env string `json:"env"`
	// Branch — gałąź Git wdrożona.
	Branch string `json:"branch"`
	// CommitHash — hash HEAD w momencie wdrożenia.
	CommitHash string `json:"commit_hash"`
	// CommitMessage — wiadomość commita.
	CommitMessage string `json:"commit_message"`
	// CommitAuthor — autor commita.
	CommitAuthor string `json:"commit_author"`
	// Snapshot — dane snapshotu przedwdrożeniowego.
	Snapshot SnapshotInfo `json:"snapshot"`
	// StartedAt — czas rozpoczęcia wdrożenia.
	StartedAt time.Time `json:"started_at"`
	// CompletedAt — czas zakończenia wdrożenia.
	CompletedAt time.Time `json:"completed_at,omitempty"`
	// Status — stan wdrożenia.
	Status DeployStatus `json:"status"`
	// Stages — wyniki poszczególnych etapów.
	Stages []StageRecord `json:"stages,omitempty"`
	// RolledBackFrom — ID wdrożenia przed rollbackiem (jeśli to rollback).
	RolledBackFrom string `json:"rolled_back_from,omitempty"`
}

// State jest główną strukturą pliku state.json.
type State struct {
	// Version — wersja formatu pliku (aktualnie 1).
	Version int `json:"version"`
	// Deployments — lista wszystkich wdrożeń (najnowsze na początku).
	Deployments []DeployRecord `json:"deployments"`
}

// ─── Operacje na stanie ────────────────────────────────────────────────────

// Load wczytuje plik state.json. Jeśli nie istnieje, zwraca pusty stan.
func Load(stateFilePath string) (*State, error) {
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{Version: currentVersion, Deployments: []DeployRecord{}}, nil
		}
		return nil, fmt.Errorf("nie można odczytać state.json: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("uszkodzony state.json: %w", err)
	}
	return &s, nil
}

// Save zapisuje stan do pliku używając atomowego wzorca write-rename.
// Chroni przed uszkodzeniem pliku przy nagłym przerwaniu zapisu.
func Save(stateFilePath string, s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("nie można serializować stanu: %w", err)
	}

	// Zapis atomowy: najpierw do pliku tymczasowego, potem rename.
	tmpPath := stateFilePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("nie można zapisać pliku tymczasowego: %w", err)
	}

	if err := os.Rename(tmpPath, stateFilePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("nie można zastąpić state.json: %w", err)
	}

	return nil
}

// AddDeployment dodaje nowy rekord wdrożenia na początek listy.
func (s *State) AddDeployment(record DeployRecord) {
	s.Deployments = append([]DeployRecord{record}, s.Deployments...)
}

// UpdateDeployment aktualizuje istniejący rekord wdrożenia po ID.
func (s *State) UpdateDeployment(record DeployRecord) bool {
	for i, d := range s.Deployments {
		if d.ID == record.ID {
			s.Deployments[i] = record
			return true
		}
	}
	return false
}

// GetLastSuccessful zwraca ostatnie udane wdrożenie dla danego środowiska.
// Zwraca nil jeśli nie znaleziono żadnego.
func (s *State) GetLastSuccessful(env string) *DeployRecord {
	for i := range s.Deployments {
		d := &s.Deployments[i]
		if d.Env == env && d.Status == StatusSuccess {
			return d
		}
	}
	return nil
}

// GetLastN zwraca ostatnie n wdrożeń dla danego środowiska (lub wszystkich jeśli env=="").
func (s *State) GetLastN(env string, n int) []DeployRecord {
	var result []DeployRecord
	for _, d := range s.Deployments {
		if env == "" || d.Env == env {
			result = append(result, d)
		}
		if len(result) >= n {
			break
		}
	}
	return result
}

// GetByID zwraca rekord wdrożenia po ID.
func (s *State) GetByID(id string) *DeployRecord {
	for i := range s.Deployments {
		if s.Deployments[i].ID == id {
			return &s.Deployments[i]
		}
	}
	return nil
}

// GetEnvList zwraca posortowaną listę środowisk z historią.
func (s *State) GetEnvList() []string {
	seen := map[string]bool{}
	for _, d := range s.Deployments {
		seen[d.Env] = true
	}
	result := make([]string, 0, len(seen))
	for env := range seen {
		result = append(result, env)
	}
	sort.Strings(result)
	return result
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// FormatSnapshotBranch tworzy deterministyczną nazwę gałęzi backup.
func FormatSnapshotBranch(env string, t time.Time) string {
	return fmt.Sprintf("rnr_backup_%s_%s", env, t.Format("20060102_150405"))
}

// FormatSnapshotTag tworzy deterministyczną nazwę taga backup.
func FormatSnapshotTag(env string, t time.Time) string {
	return fmt.Sprintf("rnr-snap-%s-%s", env, t.Format("20060102-150405"))
}

// FormatLogFilename tworzy nazwę pliku logu dla danego wdrożenia.
func FormatLogFilename(env string, t time.Time) string {
	return fmt.Sprintf("%s_%s.log", t.Format("2006-01-02_15-04-05"), env)
}

// EnsureDir tworzy katalog jeśli nie istnieje.
func EnsureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
