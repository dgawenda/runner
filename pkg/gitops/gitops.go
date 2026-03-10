// file: pkg/gitops/gitops.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Moduł Operacji Git (GitOps Core)                                   ║
// ║                                                                      ║
// ║  Stanowi bramę bezpieczeństwa przed każdym wdrożeniem:              ║
// ║    · Audyt drzewa roboczego (git status --porcelain)                ║
// ║    · Odczyt informacji o gałęzi i ostatnim commicie                 ║
// ║    · Generowanie wiadomości commitów w stylu Konwencji Commitów     ║
// ║                                                                      ║
// ║  ZASADA: Żadne wdrożenie nie może rozpocząć się przy brudnym        ║
// ║  repozytorium. Funkcja IsClean() jest obowiązkową kontrolą.         ║
// ╚══════════════════════════════════════════════════════════════════════╝

package gitops

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ─── Typy ──────────────────────────────────────────────────────────────────

// CommitInfo przechowuje informacje o ostatnim commicie.
type CommitInfo struct {
	// Hash — pełny hash SHA-1 commita.
	Hash string
	// ShortHash — pierwsze 7 znaków hasha.
	ShortHash string
	// Message — wiadomość commita (pierwsza linia).
	Message string
	// Author — imię i nazwisko autora.
	Author string
	// AuthorEmail — email autora.
	AuthorEmail string
	// Date — czas commita.
	Date time.Time
	// RelativeDate — względny czas (np. "2 minuty temu").
	RelativeDate string
}

// DirtyFile reprezentuje plik z brudnym statusem w repozytorium.
type DirtyFile struct {
	// Status — kod statusu git (np. "M " = zmodyfikowany, "??" = nieśledzony).
	Status string
	// Path — ścieżka do pliku.
	Path string
}

// StatusResult wynik audytu repozytorium.
type StatusResult struct {
	// IsClean — true jeśli repozytorium jest czyste i gotowe do wdrożenia.
	IsClean bool
	// IsGitRepo — true jeśli katalog jest repozytorium Git.
	// Projekty bez Git (plain HTML) mają IsGitRepo=false — nie blokują wdrożenia.
	IsGitRepo bool
	// DirtyFiles — lista brudnych plików jeśli repozytorium nie jest czyste.
	DirtyFiles []DirtyFile
	// Branch — aktualna gałąź.
	Branch string
	// LastCommit — informacje o ostatnim commicie.
	LastCommit CommitInfo
}

// ─── Audyt Repozytorium ────────────────────────────────────────────────────

// AuditRepo przeprowadza pełny audyt repozytorium Git.
// Jeśli katalog nie jest repozytorium Git (brak .git), zwraca StatusResult
// z IsClean=true i pustymi polami — nie blokuje wdrożenia dla projektów bez Git.
func AuditRepo(workdir string) (*StatusResult, error) {
	branch, err := GetCurrentBranch(workdir)
	if err != nil {
		// Brak repozytorium Git — nie blokuj, zwróć pusty status
		return &StatusResult{
			IsClean: true,
			Branch:  "(brak git)",
			LastCommit: CommitInfo{
				Hash:      "",
				ShortHash: "",
				Message:   "(brak git — projekt bez repozytorium)",
			},
		}, nil
	}

	dirty, err := getDirtyFiles(workdir)
	if err != nil {
		return nil, fmt.Errorf("nie można sprawdzić statusu git: %w", err)
	}

	commit, err := GetLastCommit(workdir)
	if err != nil {
		commit = &CommitInfo{Message: "(brak commitów)"}
	}

	return &StatusResult{
		IsClean:    len(dirty) == 0,
		IsGitRepo:  true,
		DirtyFiles: dirty,
		Branch:     branch,
		LastCommit: *commit,
	}, nil
}

// getDirtyFiles zwraca listę plików ze "brudnym" statusem git.
// Używa --porcelain dla deterministycznego, parseowalnego wyjścia.
func getDirtyFiles(workdir string) ([]DirtyFile, error) {
	out, err := runGit(workdir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	var files []DirtyFile
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		status := line[:2]
		path := strings.TrimSpace(line[3:])
		files = append(files, DirtyFile{Status: status, Path: path})
	}
	return files, nil
}

// ─── Informacje o Commicie ─────────────────────────────────────────────────

// GetLastCommit pobiera informacje o ostatnim commicie w bieżącej gałęzi.
func GetLastCommit(workdir string) (*CommitInfo, error) {
	// Format: hash|short|message|author|email|iso-date|relative-date
	format := "%H|%h|%s|%an|%ae|%ci|%cr"
	out, err := runGit(workdir, "log", "-1", "--format="+format)
	if err != nil {
		return nil, err
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return &CommitInfo{
			Hash:      "0000000000000000000000000000000000000000",
			ShortHash: "0000000",
			Message:   "(brak commitów)",
		}, nil
	}

	parts := strings.SplitN(out, "|", 7)
	if len(parts) < 7 {
		return nil, fmt.Errorf("nieoczekiwany format wyjścia git log: %q", out)
	}

	var t time.Time
	t, _ = time.Parse("2006-01-02 15:04:05 -0700", parts[5])

	return &CommitInfo{
		Hash:         parts[0],
		ShortHash:    parts[1],
		Message:      parts[2],
		Author:       parts[3],
		AuthorEmail:  parts[4],
		Date:         t,
		RelativeDate: parts[6],
	}, nil
}

// GetCurrentBranch zwraca nazwę bieżącej gałęzi Git.
func GetCurrentBranch(workdir string) (string, error) {
	out, err := runGit(workdir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GetCommitHash zwraca pełny hash HEAD lub podanego ref-a.
func GetCommitHash(workdir, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	out, err := runGit(workdir, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GetRecentCommits zwraca listę ostatnich n commitów.
func GetRecentCommits(workdir string, n int) ([]CommitInfo, error) {
	format := "%H|%h|%s|%an|%ae|%ci|%cr"
	out, err := runGit(workdir, "log", fmt.Sprintf("-%d", n), "--format="+format)
	if err != nil {
		return nil, err
	}

	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 7)
		if len(parts) < 7 {
			continue
		}
		var t time.Time
		t, _ = time.Parse("2006-01-02 15:04:05 -0700", parts[5])
		commits = append(commits, CommitInfo{
			Hash:         parts[0],
			ShortHash:    parts[1],
			Message:      parts[2],
			Author:       parts[3],
			AuthorEmail:  parts[4],
			Date:         t,
			RelativeDate: parts[6],
		})
	}
	return commits, nil
}

// ─── Generowanie wiadomości commitów ──────────────────────────────────────

// FormatDeployCommitMessage tworzy sformatowaną wiadomość commita wdrożenia.
// Wzoruje się na Konwencji Commitów dla spójności historii projektu.
// Przykład: "deploy(production): Wdróż v1.2.3 — feat: add user auth [abc1234]"
func FormatDeployCommitMessage(env, projectName string, commit CommitInfo) string {
	shortMsg := commit.Message
	if len(shortMsg) > 60 {
		shortMsg = shortMsg[:57] + "..."
	}
	return fmt.Sprintf("deploy(%s): %s — %s [%s]",
		env, projectName, shortMsg, commit.ShortHash)
}

// FormatRollbackCommitMessage tworzy wiadomość commita dla operacji rollback.
func FormatRollbackCommitMessage(env string, targetCommit CommitInfo) string {
	return fmt.Sprintf("revert(%s): Przywróć do %s — %s",
		env, targetCommit.ShortHash, targetCommit.Message)
}

// ─── Operacje Git ─────────────────────────────────────────────────────────

// FetchOrigin pobiera zdalne zmiany bez ich mergowania.
func FetchOrigin(workdir string) error {
	_, err := runGit(workdir, "fetch", "--tags", "origin")
	return err
}

// CheckoutBranch przełącza na podaną gałąź.
// Jeśli gałąź nie istnieje lokalnie, próbuje śledzić zdalną (origin/<branch>).
// Zwraca nil jeśli gałąź już jest aktywna.
func CheckoutBranch(workdir, branch string) error {
	// Sprawdź bieżącą gałąź — jeśli już tam jesteśmy, nic nie rób
	current, err := GetCurrentBranch(workdir)
	if err != nil {
		return fmt.Errorf("nie można odczytać bieżącej gałęzi: %w", err)
	}
	if current == branch {
		return nil // już na właściwej gałęzi
	}

	// Sprawdź czy gałąź istnieje lokalnie
	localExists, _ := BranchExists(workdir, branch)
	if localExists {
		_, err = runGit(workdir, "checkout", branch)
		return err
	}

	// Próba: śledź zdalną gałąź origin/<branch>
	_, err = runGit(workdir, "checkout", "-b", branch, "--track", "origin/"+branch)
	if err != nil {
		return fmt.Errorf("nie można przełączyć na gałąź '%s': %w\nUpewnij się że gałąź istnieje lokalnie lub na origin.", branch, err)
	}
	return nil
}

// PullBranch pobiera i merguje zmiany z origin/<branch>.
// Błąd jest niekriytyczny (np. brak połączenia, brak remota) — logujemy ostrzeżenie.
func PullBranch(workdir, branch string) error {
	_, err := runGit(workdir, "pull", "origin", branch, "--ff-only")
	if err != nil {
		return fmt.Errorf("git pull origin %s: %w", branch, err)
	}
	return nil
}

// BranchExists sprawdza czy gałąź istnieje lokalnie lub zdalnie.
func BranchExists(workdir, branch string) (bool, error) {
	_, err := runGit(workdir, "rev-parse", "--verify", branch)
	if err != nil {
		// Gałąź nie istnieje — to nie jest błąd krytyczny
		return false, nil
	}
	return true, nil
}

// TagExists sprawdza czy tag istnieje.
func TagExists(workdir, tag string) (bool, error) {
	out, err := runGit(workdir, "tag", "-l", tag)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == tag, nil
}

// ─── Operacje na Gałęziach i Commitach ────────────────────────────────────

// GetLocalBranches zwraca listę lokalnych gałęzi posortowaną alfabetycznie.
// Aktualnie aktywna gałąź jest zwracana bez gwiazdki (plain name).
func GetLocalBranches(workdir string) ([]string, error) {
	out, err := runGit(workdir, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if b := strings.TrimSpace(line); b != "" {
			branches = append(branches, b)
		}
	}
	return branches, nil
}

// StageAll dodaje wszystkie zmienione, nowe i usunięte pliki do staging area.
// Odpowiednik: git add -A
func StageAll(workdir string) error {
	_, err := runGit(workdir, "add", "-A")
	return err
}

// CommitWithMessage tworzy commit z podaną wiadomością.
// Zwraca hash nowego commita.
func CommitWithMessage(workdir, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("wiadomość commita nie może być pusta")
	}
	_, err := runGit(workdir, "commit", "-m", message)
	if err != nil {
		return "", err
	}
	// Pobierz hash nowego commita
	hash, err := runGit(workdir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "?", nil // commit się powiódł, hash nieznany
	}
	return strings.TrimSpace(hash), nil
}

// GetCommitHistory zwraca ostatnie n commitów bieżącej gałęzi.
// Pola CommitInfo są w pełni wypełnione (Hash, ShortHash, Message, Author, RelativeDate).
func GetCommitHistory(workdir string, n int) ([]CommitInfo, error) {
	// Separator \x00 (null byte) — bezpieczny separator dla wszystkich pól
	const sep = "\x00"
	format := "%H" + sep + "%h" + sep + "%s" + sep + "%an" + sep + "%ar"
	out, err := runGit(workdir, "log",
		fmt.Sprintf("--format=%s", format),
		fmt.Sprintf("-n%d", n),
	)
	if err != nil {
		return nil, err
	}
	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, sep)
		if len(parts) < 5 {
			continue
		}
		commits = append(commits, CommitInfo{
			Hash:         parts[0],
			ShortHash:    parts[1],
			Message:      parts[2],
			Author:       parts[3],
			RelativeDate: parts[4],
		})
	}
	return commits, nil
}

// ─── Wewnętrzne ───────────────────────────────────────────────────────────

// runGit wykonuje komendę git w podanym katalogu i zwraca stdout.
// Zwraca błąd wzbogacony o stderr przy niepowodzeniu.
func runGit(workdir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderrStr)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return stdout.String(), nil
}
