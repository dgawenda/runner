// file: pkg/tui/guards.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  System Strażników Wdrożenia — Deploy Guards                        ║
// ║                                                                      ║
// ║  Strażnicy to zestaw automatycznych kontroli bezpieczeństwa,        ║
// ║  które muszą zostać zaliczone przed każdym wdrożeniem.              ║
// ║  Zapobiegają przypadkowym deploym na złych gałęziach,               ║
// ║  z nieczystym repozytorium lub bez nowych commitów.                 ║
// ║                                                                      ║
// ║  Strażnicy (w kolejności sprawdzania):                              ║
// ║    1. GAŁĄŹ   — musi być master (prod) lub develop (dev)           ║
// ║    2. GIT OK  — brak błędów w repozytorium, HEAD nie wisi          ║
// ║    3. CZYSTY  — brak niezatwierdzonych śledzo  nych plików          ║
// ║    4. REMOTE  — repozytorium ma skonfigurowany origin               ║
// ║    5. COMMIT  — jest przynajmniej 1 commit do wdrożenia             ║
// ║    6. NOWOŚĆ  — nowe commity od ostatniego deployu tego środowiska  ║
// ║    7. KONFIG  — środowisko ma kompletną konfigurację wdrożenia      ║
// ╚══════════════════════════════════════════════════════════════════════╝

package tui

import (
	"fmt"
	"strings"

	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/gitops"
	"github.com/neution/rnr/pkg/state"
)

// ─── Typy ─────────────────────────────────────────────────────────────────

// GuardLevel określa powagę niespełnionego strażnika.
type GuardLevel int

const (
	// GuardLevelBlock — strażnik blokuje deployment (krytyczny).
	GuardLevelBlock GuardLevel = iota
	// GuardLevelWarn — strażnik ostrzega, ale można kontynuować (opcjonalne).
	GuardLevelWarn
)

// GuardResult to wynik sprawdzenia pojedynczego strażnika.
type GuardResult struct {
	// Name — nazwa strażnika (wyświetlana w TUI).
	Name string
	// Pass — true gdy strażnik zaliczony.
	Pass bool
	// Level — BlOCK lub WARN (WARN nie blokuje deployu).
	Level GuardLevel
	// Reason — krótki opis stanu (1 linia, wyświetlany obok ikony).
	Reason string
	// Hint — wskazówka jak naprawić problem (gdy Pass=false).
	Hint string
}

// deployGuardsInput zawiera wszystkie dane potrzebne do sprawdzenia strażników.
type deployGuardsInput struct {
	env       string
	envCfg    config.Environment
	gitStatus *gitops.StatusResult
	stateData *state.State
}

// ─── Publiczna funkcja ────────────────────────────────────────────────────

// RunDeployGuards uruchamia wszystkie strażniki dla podanego środowiska.
// Zwraca listę wyników — każdy strażnik to jeden element.
// Wywołanie jest tanie (bez I/O) — tylko analiza przekazanych danych.
func RunDeployGuards(env string, envCfg config.Environment, gitStatus *gitops.StatusResult, stateData *state.State) []GuardResult {
	inp := deployGuardsInput{
		env:       env,
		envCfg:    envCfg,
		gitStatus: gitStatus,
		stateData: stateData,
	}

	return []GuardResult{
		guardBranch(inp),
		guardGitHealth(inp),
		guardCleanRepo(inp),
		guardHasCommits(inp),
		guardNewCommits(inp),
		guardConfigComplete(inp),
	}
}

// AllGuardsPass zwraca true gdy WSZYSTKIE strażniki blokujące (GuardLevelBlock) są zaliczone.
// Strażniki z poziomem WARN nie blokują deployu.
func AllGuardsPass(guards []GuardResult) bool {
	for _, g := range guards {
		if !g.Pass && g.Level == GuardLevelBlock {
			return false
		}
	}
	return true
}

// BlockingGuards zwraca tylko strażniki blokujące, które NIE są zaliczone.
func BlockingGuards(guards []GuardResult) []GuardResult {
	var out []GuardResult
	for _, g := range guards {
		if !g.Pass && g.Level == GuardLevelBlock {
			out = append(out, g)
		}
	}
	return out
}

// ─── Poszczególne strażniki ───────────────────────────────────────────────

// guardBranch sprawdza czy aktualna gałąź jest dozwolona do wdrożenia.
// Production wymaga gałęzi 'master', development wymaga 'develop'.
func guardBranch(inp deployGuardsInput) GuardResult {
	g := GuardResult{
		Name:  "Gałąź robocza",
		Level: GuardLevelBlock,
	}

	if inp.gitStatus == nil {
		g.Pass = false
		g.Reason = "Brak danych Git"
		g.Hint = "Poczekaj na załadowanie statusu repozytorium."
		return g
	}

	if !inp.gitStatus.IsGitRepo {
		g.Pass = false
		g.Reason = "Nie jest repozytorium Git"
		g.Hint = "Zainicjuj repozytorium: git init"
		return g
	}

	current := inp.gitStatus.Branch

	// Określ wymaganą gałąź na podstawie środowiska
	requiredBranch := inp.envCfg.Branch
	if requiredBranch == "" {
		// Domyślne mapowanie środowisk na gałęzie
		switch inp.env {
		case "production":
			requiredBranch = "master"
		case "development":
			requiredBranch = "develop"
		default:
			requiredBranch = "develop"
		}
	}

	if current == requiredBranch {
		g.Pass = true
		g.Reason = fmt.Sprintf("Gałąź ⎇ %s ✓", current)
	} else {
		g.Pass = false
		g.Reason = fmt.Sprintf("Jesteś na ⎇ %s (wymagana: ⎇ %s)", current, requiredBranch)
		g.Hint = fmt.Sprintf("Apollo automatycznie przełączy na gałąź '%s'.", requiredBranch)
	}

	return g
}

// guardGitHealth sprawdza czy repozytorium Git jest w zdrowym stanie.
// Blokuje gdy HEAD jest odłączone (detached HEAD) lub brak commitów.
func guardGitHealth(inp deployGuardsInput) GuardResult {
	g := GuardResult{
		Name:  "Stan Git",
		Level: GuardLevelBlock,
	}

	if inp.gitStatus == nil {
		g.Pass = false
		g.Reason = "Brak danych Git"
		g.Hint = "Poczekaj na załadowanie statusu repozytorium."
		return g
	}

	// Odłączona głowa (detached HEAD) = brak gałęzi
	if inp.gitStatus.Branch == "" || inp.gitStatus.Branch == "HEAD" {
		g.Pass = false
		g.Reason = "Detached HEAD — nie jesteś na gałęzi"
		g.Hint = "git checkout <nazwa-gałęzi>"
		return g
	}

	// Brak commitów (pusty hash = nowe repo)
	if inp.gitStatus.LastCommit.Hash == "" ||
		strings.HasPrefix(inp.gitStatus.LastCommit.Hash, "0000000") {
		g.Pass = false
		g.Reason = "Repozytorium nie ma żadnych commitów"
		g.Hint = "Zatwierdź zmiany: git add . && git commit -m 'init'"
		return g
	}

	g.Pass = true
	g.Reason = fmt.Sprintf("Repozytorium zdrowe, HEAD: %s", inp.gitStatus.LastCommit.ShortHash)
	return g
}

// guardCleanRepo sprawdza czy repozytorium nie ma niezatwierdzonych ŚLEDZONYCH plików.
// Pliki nieśledzone (??) są ignorowane — nie wpływają na git checkout.
func guardCleanRepo(inp deployGuardsInput) GuardResult {
	g := GuardResult{
		Name:  "Czystość repo",
		Level: GuardLevelBlock,
	}

	if inp.gitStatus == nil {
		g.Pass = true // Brak danych = nie blokuj
		g.Reason = "Brak danych Git"
		return g
	}

	var trackedDirty []gitops.DirtyFile
	for _, f := range inp.gitStatus.DirtyFiles {
		if f.Status != "??" {
			trackedDirty = append(trackedDirty, f)
		}
	}

	if len(trackedDirty) == 0 {
		g.Pass = true
		g.Reason = "Brak niezatwierdzonych zmian ✓"
	} else {
		g.Pass = false
		g.Reason = fmt.Sprintf("%d niezatwierdzonych plików (śledzone)", len(trackedDirty))
		g.Hint = "Zatwierdź zmiany w GitPanel (G) lub: git add . && git commit"
	}

	return g
}

// guardHasCommits sprawdza czy repozytorium ma przynajmniej jeden commit.
// Jest potrzebny żeby wdrożenie miało co „wdrożyć".
func guardHasCommits(inp deployGuardsInput) GuardResult {
	g := GuardResult{
		Name:  "Historia commitów",
		Level: GuardLevelBlock,
	}

	if inp.gitStatus == nil {
		g.Pass = false
		g.Reason = "Brak danych Git"
		g.Hint = "Poczekaj na załadowanie statusu repozytorium."
		return g
	}

	if inp.gitStatus.LastCommit.Hash == "" ||
		strings.HasPrefix(inp.gitStatus.LastCommit.Hash, "0000000") {
		g.Pass = false
		g.Reason = "Brak commitów w repozytorium"
		g.Hint = "Utwórz pierwszy commit: git add . && git commit -m 'init'"
		return g
	}

	g.Pass = true
	g.Reason = fmt.Sprintf("Ostatni commit: %s — %s",
		inp.gitStatus.LastCommit.ShortHash,
		truncateStr(inp.gitStatus.LastCommit.Message, 40))
	return g
}

// guardNewCommits sprawdza czy od ostatniego wdrożenia danego środowiska
// pojawiły się nowe commity. Blokuje redeploy tego samego commita.
func guardNewCommits(inp deployGuardsInput) GuardResult {
	g := GuardResult{
		Name:  "Nowe commity",
		Level: GuardLevelWarn, // WARN — można wymusić redeploy
	}

	if inp.gitStatus == nil || inp.stateData == nil {
		g.Pass = true
		g.Reason = "Brak historii wdrożeń — pierwszy deploy"
		return g
	}

	last := inp.stateData.GetLastSuccessful(inp.env)
	if last == nil {
		g.Pass = true
		g.Reason = "Brak poprzednich wdrożeń — pierwszy deploy ✓"
		return g
	}

	currentHash := inp.gitStatus.LastCommit.Hash
	lastHash := last.CommitHash

	if currentHash == lastHash {
		g.Pass = false
		g.Reason = fmt.Sprintf("Brak zmian od ostatniego deploy (%s)", last.StartedAt.Format("02.01 15:04"))
		g.Hint = "Zatwierdź nowe zmiany lub użyj opcji wymuszenia (F — force redeploy)"
		return g
	}

	g.Pass = true
	g.Reason = fmt.Sprintf("Nowe commity od %s ✓", last.StartedAt.Format("02.01 15:04"))
	return g
}

// guardConfigComplete sprawdza czy konfiguracja środowiska jest kompletna.
// Weryfikuje czy dostawca wdrożenia ma wymagane pola.
func guardConfigComplete(inp deployGuardsInput) GuardResult {
	g := GuardResult{
		Name:  "Konfiguracja",
		Level: GuardLevelBlock,
	}

	envCfg := inp.envCfg

	// Sprawdź dostawcę deployu
	if envCfg.Deploy.Provider == "" {
		g.Pass = false
		g.Reason = "Brak skonfigurowanego dostawcy wdrożenia"
		g.Hint = "Ustaw deploy.provider w rnr.conf.yaml (np. netlify, ssh)"
		return g
	}

	// Netlify: wymaga site_id LUB flagi create_new
	if envCfg.Deploy.Provider == "netlify" {
		if envCfg.Deploy.NetlifySiteID == "" && !envCfg.Deploy.NetlifyCreateNew {
			g.Pass = false
			g.Reason = "Brak Netlify Site ID (netlify_site_id)"
			g.Hint = "Ustaw netlify_site_id w rnr.conf.yaml lub netlify_create_new: true"
			return g
		}
		if envCfg.Deploy.NetlifyAuthToken == "" {
			g.Pass = false
			g.Reason = "Brak tokenu Netlify (netlify_auth_token)"
			g.Hint = "Ustaw netlify_auth_token w rnr.conf.yaml (sekcja środowiska)"
			return g
		}
	}

	g.Pass = true
	g.Reason = fmt.Sprintf("Dostawca: %s ✓", envCfg.Deploy.Provider)
	return g
}
