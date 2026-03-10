// file: pkg/gitops/snapshot.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  System Snapshotów Przedwdrożeniowych                               ║
// ║                                                                      ║
// ║  Przed KAŻDYM wdrożeniem tworzy deterministyczny punkt przywracania ║
// ║  w postaci gałęzi backup i taga Git.                                ║
// ║                                                                      ║
// ║  Nazwy są deterministyczne i zawierają środowisko + timestamp:      ║
// ║    Gałąź: rnr_backup_production_20240115_143000                     ║
// ║    Tag:   rnr-snap-production-20240115-143000                       ║
// ║                                                                      ║
// ║  Snapshot chroni przed utratą kodu przy wadliwym wdrożeniu.         ║
// ╚══════════════════════════════════════════════════════════════════════╝

package gitops

import (
	"fmt"
	"time"
)

// ─── Typy ──────────────────────────────────────────────────────────────────

// SnapshotResult zawiera informacje o utworzonym snapshocie.
type SnapshotResult struct {
	// Branch — nazwa gałęzi backup.
	Branch string
	// Tag — nazwa taga Git.
	Tag string
	// CommitHash — hash commita wskazywanego przez snapshot.
	CommitHash string
	// CreatedAt — czas utworzenia snapshotu.
	CreatedAt time.Time
}

// ─── Tworzenie Snapshotu ───────────────────────────────────────────────────

// CreateSnapshot tworzy deterministyczny snapshot przedwdrożeniowy.
// Tworzy zarówno gałąź backup jak i tag Git wskazujący na aktualny HEAD.
// Bezpieczne do wielokrotnego wywołania — sprawdza czy snapshot już istnieje.
func CreateSnapshot(workdir, env string) (*SnapshotResult, error) {
	now := time.Now()

	branch := formatSnapshotBranch(env, now)
	tag := formatSnapshotTag(env, now)

	// Pobierz aktualny hash HEAD
	commitHash, err := GetCommitHash(workdir, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("nie można odczytać HEAD: %w", err)
	}

	// Utwórz gałąź backup
	if err := createBackupBranch(workdir, branch, commitHash); err != nil {
		return nil, fmt.Errorf("nie można utworzyć gałęzi backup %s: %w", branch, err)
	}

	// Utwórz tag
	if err := createBackupTag(workdir, tag, commitHash, env); err != nil {
		// Tag jest opcjonalny — nie przerywamy przy błędzie
		_ = err
	}

	return &SnapshotResult{
		Branch:     branch,
		Tag:        tag,
		CommitHash: commitHash,
		CreatedAt:  now,
	}, nil
}

// createBackupBranch tworzy nową gałąź wskazującą na podany commit.
func createBackupBranch(workdir, branch, commitHash string) error {
	// Sprawdź czy gałąź już istnieje
	exists, err := BranchExists(workdir, branch)
	if err != nil {
		return err
	}
	if exists {
		return nil // Snapshot już istnieje — idempotentne
	}

	_, err = runGit(workdir, "branch", branch, commitHash)
	return err
}

// createBackupTag tworzy anotowany tag wskazujący na podany commit.
func createBackupTag(workdir, tag, commitHash, env string) error {
	exists, err := TagExists(workdir, tag)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	message := fmt.Sprintf("Snapshot rnr przed wdrożeniem na %s", env)
	_, err = runGit(workdir, "tag", "-a", tag, commitHash, "-m", message)
	return err
}

// ─── Cleanup Snapshotów ───────────────────────────────────────────────────

// PruneOldSnapshots usuwa stare gałęzie backup (starsze niż maxKeep ostatnich).
// Chroni repozytorium przed zaśmiecaniem przez stare snapshoty.
func PruneOldSnapshots(workdir, env string, maxKeep int) error {
	// Pobierz listę gałęzi backup dla środowiska
	prefix := fmt.Sprintf("rnr_backup_%s_", env)
	out, err := runGit(workdir, "branch", "--list", prefix+"*", "--sort=-creatordate")
	if err != nil {
		return fmt.Errorf("nie można listować gałęzi: %w", err)
	}

	var branches []string
	for _, line := range splitLines(out) {
		b := trimBranchName(line)
		if b != "" {
			branches = append(branches, b)
		}
	}

	// Usuń stare gałęzie (zachowaj maxKeep najnowszych)
	if len(branches) <= maxKeep {
		return nil
	}

	for _, branch := range branches[maxKeep:] {
		if _, err := runGit(workdir, "branch", "-D", branch); err != nil {
			// Nie przerywaj przy błędzie czyszczenia — to operacja niekriytyczna
			continue
		}
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func formatSnapshotBranch(env string, t time.Time) string {
	return fmt.Sprintf("rnr_backup_%s_%s", env, t.Format("20060102_150405"))
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
