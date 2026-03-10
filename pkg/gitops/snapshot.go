// file: pkg/gitops/snapshot.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  System Snapshotów Przedwdrożeniowych                               ║
// ║                                                                      ║
// ║  Przed KAŻDYM wdrożeniem rejestruje punkt przywracania jako        ║
// ║  hash commita HEAD (bez tworzenia dodatkowych gałęzi Git).         ║
// ║                                                                      ║
// ║  ZASADA: rnr NIE tworzy dodatkowych gałęzi Git ani nie przełącza  ║
// ║  ich automatycznie. Zarządzanie gałęziami należy do dewelopera.   ║
// ║                                                                      ║
// ║  Rollback przywraca kod do zapisanego hash commita bez zmiany      ║
// ║  bieżącej gałęzi.                                                   ║
// ╚══════════════════════════════════════════════════════════════════════╝

package gitops

import (
	"fmt"
	"time"
)

// ─── Typy ──────────────────────────────────────────────────────────────────

// SnapshotResult zawiera informacje o zarejestrowanym punkcie przywracania.
type SnapshotResult struct {
	// Branch — bieżąca gałąź w momencie snapshotu (tylko informacyjnie).
	Branch string
	// Tag — pusty (zachowany dla kompatybilności).
	Tag string
	// CommitHash — hash HEAD commita — właściwy punkt przywracania.
	CommitHash string
	// CreatedAt — czas zarejestrowania snapshotu.
	CreatedAt time.Time
}

// ─── Snapshot ────────────────────────────────────────────────────────────

// CreateSnapshot rejestruje hash aktualnego HEAD jako punkt przywracania.
//
// Celowo NIE tworzy dodatkowych gałęzi Git ani tagów.
// rnr zarządza snapshotami wyłącznie przez plik .rnr/snapshots/state.json.
//
// Jeśli katalog nie jest repozytorium Git, snapshot zwraca pusty hash
// i nie blokuje wdrożenia (np. dla projektów statycznych bez Git).
func CreateSnapshot(workdir, env string) (*SnapshotResult, error) {
	now := time.Now()

	// Pobierz bieżącą gałąź (informacyjnie)
	branch, _ := GetCurrentBranch(workdir)

	// Pobierz hash HEAD — to jest właściwy snapshot
	commitHash, err := GetCommitHash(workdir, "HEAD")
	if err != nil {
		// Brak git repo lub brak commitów — nie blokuj wdrożenia
		commitHash = ""
	}

	return &SnapshotResult{
		Branch:     branch,
		Tag:        "",
		CommitHash: commitHash,
		CreatedAt:  now,
	}, nil
}

// RecordSnapshot tworzy snapshot bez wymagania repozytorium Git.
// Używany gdy projekt nie korzysta z Git (np. HTML bez historii).
func RecordSnapshot(workdir, env string) *SnapshotResult {
	snap, _ := CreateSnapshot(workdir, env)
	if snap == nil {
		return &SnapshotResult{
			Branch:    env,
			CreatedAt: time.Now(),
		}
	}
	return snap
}

// ─── Cleanup Snapshotów ───────────────────────────────────────────────────

// PruneOldSnapshots — zachowane dla kompatybilności, nie robi nic
// (snapshoty są teraz zarządzane przez state.json, nie gałęzie Git).
func PruneOldSnapshots(_ string, _ string, _ int) error {
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func formatSnapshotBranch(env string, t time.Time) string {
	return fmt.Sprintf("rnr_snap_%s_%s", env, t.Format("20060102_150405"))
}

func formatSnapshotTag(env string, t time.Time) string {
	return fmt.Sprintf("rnr-snap-%s-%s", env, t.Format("20060102-150405"))
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range splitByNewline(s) {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitByNewline(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func trimBranchName(s string) string {
	s = trimLeft(s, " *\t")
	s = trimRight(s, " \t\r\n")
	return s
}

func trimLeft(s, cutset string) string {
	for len(s) > 0 && containsRune(cutset, rune(s[0])) {
		s = s[1:]
	}
	return s
}

func trimRight(s, cutset string) string {
	for len(s) > 0 && containsRune(cutset, rune(s[len(s)-1])) {
		s = s[:len(s)-1]
	}
	return s
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
