// file: pkg/tui/messages.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Typy Wiadomości Bubble Tea (Tea Messages)                          ║
// ║                                                                      ║
// ║  Wszystkie niestandardowe zdarzenia przepływające przez pętlę Elm.  ║
// ║  Wiadomości są jedynym mechanizmem komunikacji między goroutinami   ║
// ║  (pipeline execution) a głównym modelem TUI.                        ║
// ╚══════════════════════════════════════════════════════════════════════╝

package tui

import (
	"time"

	"github.com/neution/rnr/pkg/gitops"
	"github.com/neution/rnr/pkg/state"
)

// ─── Wiadomości Inicjalizacji ─────────────────────────────────────────────

// GitStatusMsg dostarcza wynik audytu repozytorium Git.
type GitStatusMsg struct {
	Result *gitops.StatusResult
	Err    error
}

// ConfigLoadedMsg informuje o pomyślnym załadowaniu konfiguracji.
type ConfigLoadedMsg struct {
	Err error
}

// StateLoadedMsg informuje o pomyślnym załadowaniu state.json.
type StateLoadedMsg struct {
	State *state.State
	Err   error
}

// ─── Wiadomości Wdrożenia ─────────────────────────────────────────────────

// DeployStartMsg inicjuje procedurę wdrożenia dla danego środowiska.
type DeployStartMsg struct {
	Env string
}

// SnapshotCreatedMsg informuje o pomyślnym utworzeniu snapshotu.
type SnapshotCreatedMsg struct {
	Branch string
	Tag    string
	Hash   string
	Err    error
}

// StageStartedMsg informuje o rozpoczęciu etapu potoku.
type StageStartedMsg struct {
	// Index — indeks etapu w potoku (0-based).
	Index int
	// Name — nazwa etapu.
	Name string
}

// StageOutputMsg niesie linię wyjścia z wykonywanego etapu.
type StageOutputMsg struct {
	// Index — indeks etapu.
	Index int
	// Line — zamaskowana linia wyjścia.
	Line string
}

// StageCompletedMsg informuje o zakończeniu etapu sukcesem.
type StageCompletedMsg struct {
	Index      int
	Name       string
	DurationMS int64
}

// StageFailedMsg informuje o błędzie etapu.
type StageFailedMsg struct {
	Index        int
	Name         string
	DurationMS   int64
	Err          error
	AllowFailure bool
}

// StageSkippedMsg informuje o pominięciu etapu (only: [env]).
type StageSkippedMsg struct {
	Index int
	Name  string
}

// DeployCompletedMsg informuje o zakończeniu całego potoku sukcesem.
type DeployCompletedMsg struct {
	Env        string
	DeployID   string
	LogFile    string
	TotalSteps int
}

// DeployFailedMsg informuje o błędzie całego potoku.
type DeployFailedMsg struct {
	Env      string
	DeployID string
	StepName string
	Err      error
}

// ─── Wiadomości Rollbacku ─────────────────────────────────────────────────

// ShowRollbackPickMsg otwiera ekran wyboru wdrożenia do rollbacku.
// Zawiera listę udanych wdrożeń dla danego środowiska posortowanych od najnowszych.
type ShowRollbackPickMsg struct {
	Env     string
	Records []state.DeployRecord
}

// RollbackStartMsg inicjuje procedurę rollbacku do wybranego wdrożenia.
type RollbackStartMsg struct {
	Env         string
	DeployID    string
	CommitHash  string // hash commita który był wdrożony (przywracamy DO tego stanu)
	Branch      string
	Description string // opis wdrożenia (widoczny w ekranie rollbacku)
}

// RollbackProgressMsg niesie aktualizację postępu rollbacku.
type RollbackProgressMsg struct {
	Step    string
	Message string
}

// RollbackCompletedMsg informuje o pomyślnym rollbacku i redeploymencie.
type RollbackCompletedMsg struct {
	Env string
}

// RollbackFailedMsg informuje o błędzie rollbacku.
type RollbackFailedMsg struct {
	Err error
}

// ─── Wiadomości Promote ───────────────────────────────────────────────────

// PromoteStartMsg inicjuje procedurę promote (migracje DB).
type PromoteStartMsg struct {
	SourceEnv string
	TargetEnv string
}

// PromoteCompletedMsg informuje o pomyślnym promote.
type PromoteCompletedMsg struct{}

// PromoteFailedMsg informuje o błędzie promote.
type PromoteFailedMsg struct {
	Err error
}

// ─── Wiadomości Nawigacji ─────────────────────────────────────────────────

// NavigateMsg zmienia aktywny ekran.
type NavigateMsg struct {
	Screen Screen
}

// ErrorMsg wyświetla ekran błędu.
type ErrorMsg struct {
	Title   string
	Message string
	Err     error
}

// ConfirmDeployMsg wysyłane gdy użytkownik potwierdza wdrożenie.
type ConfirmDeployMsg struct {
	Env string
}

// ConfirmRollbackMsg wysyłane gdy użytkownik potwierdza rollback.
type ConfirmRollbackMsg struct {
	Env        string
	DeployID   string
	CommitHash string
}

// WizardCompleteMsg informuje o zakończeniu Setup Wizarda.
type WizardCompleteMsg struct {
	ProjectName      string
	Repo             string
	// ProjectType określa typ projektu: "html", "npm", "custom"
	// Wpływa na generowane etapy pipeline w rnr.yaml.
	ProjectType      string
	DeployProv       string
	NetlifyToken     string
	NetlifySiteID    string
	NetlifyCreateNew bool // true = rnr sam tworzy projekt Netlify przy pierwszym deploy
	DBProv           string
	SupabaseRef      string
	SupabaseURL      string
	SupabaseKey      string
	// GitHubRemoteURL — pełny URL klonu repozytorium (opcjonalne).
	// Jeśli niepuste, rnr wywoła git remote add/set-url origin <url> podczas init.
	// Format: "https://github.com/owner/repo.git" lub "git@github.com:owner/repo.git"
	GitHubRemoteURL string
	// UseGhCLI — true gdy użytkownik wybrał GitHub CLI (gh) w wizardzie.
	// Loguje informację o użyciu gh i wskazówki do `gh repo create`.
	UseGhCLI bool
}

// OutputLineMsg — linia wyjścia do podglądu logów w deploy/rollback.
type OutputLineMsg struct {
	Line string
}

// ─── Wiadomości Git Panel ─────────────────────────────────────────────────

// GitRefreshTickMsg — tik timera auto-odświeżania statusu Git.
// Wysyłany co N sekund, wyzwala ponowny audit repozytorium.
type GitRefreshTickMsg struct {
	T time.Time
}

// GitBranchesLoadedMsg — lista lokalnych gałęzi po załadowaniu.
type GitBranchesLoadedMsg struct {
	Branches []string
	Err      error
}

// GitHistoryLoadedMsg — historia commitów po załadowaniu.
type GitHistoryLoadedMsg struct {
	Commits []gitops.CommitInfo
	Err     error
}

// GitCheckoutRequestMsg — żądanie checkout do podanej gałęzi (z git panelu → root model).
type GitCheckoutRequestMsg struct {
	Branch string
}

// GitCheckoutDoneMsg — wynik operacji git checkout.
type GitCheckoutDoneMsg struct {
	Branch string
	Err    error
}

// GitCommitRequestMsg — żądanie stage wybranych plików + commit (z git panelu → root model).
// Files: lista ścieżek do zaindeksowania; puste = git add -A (wszystkie zmiany).
type GitCommitRequestMsg struct {
	Message string
	Files   []string // puste = stage all
}

// GitCommitDoneMsg — wynik operacji git commit.
type GitCommitDoneMsg struct {
	Hash string // skrócony hash nowego commita
	Err  error
}

// GitGraphLoadedMsg — linie wizualnego grafu commitów (git log --graph).
type GitGraphLoadedMsg struct {
	Lines []string
	Err   error
}

// GitDiffRequestMsg — żądanie załadowania diffa pliku (z git panelu → root model).
type GitDiffRequestMsg struct {
	File string
}

// GitDiffLoadedMsg — zawartość diffa dla wybranego pliku.
type GitDiffLoadedMsg struct {
	File  string
	Lines []string
	Err   error
}

// GitPushRequestMsg — żądanie git push bieżącej gałęzi (z git panelu → root model).
type GitPushRequestMsg struct{}

// GitPushDoneMsg — wynik operacji git push.
// IsNonFastForward=true gdy push odrzucony bo remote jest do przodu (ktoś inny pushował).
type GitPushDoneMsg struct {
	Branch           string
	Err              error
	IsNonFastForward bool // true = remote jest ahead → zaproponuj pull+rebase lub force
}

// GitPullRebasePushRequestMsg — żądanie git pull --rebase + push (po non-fast-forward).
type GitPullRebasePushRequestMsg struct {
	Branch string
}

// GitPullRebasePushDoneMsg — wynik operacji pull --rebase + push.
type GitPullRebasePushDoneMsg struct {
	Branch string
	Err    error
}

// GitForcePushRequestMsg — żądanie git push --force-with-lease (gdy użytkownik świadomie chce).
type GitForcePushRequestMsg struct {
	Branch string
}

// GitForcePushDoneMsg — wynik operacji git push --force-with-lease.
type GitForcePushDoneMsg struct {
	Branch string
	Err    error
}

// ─── Wiadomości Apollo Panel ──────────────────────────────────────────────

// ApolloDeployRequestMsg — żądanie wdrożenia z panelu Apollo.
// Env: nazwa środowiska. Force: true = pomiń guard "nowe commity".
type ApolloDeployRequestMsg struct {
	Env   string
	Force bool
}

// ApolloRollbackRequestMsg — żądanie rollbacku z panelu Apollo.
type ApolloRollbackRequestMsg struct {
	Env string
}

// ApolloPromoteRequestMsg — żądanie promote DB z panelu Apollo.
type ApolloPromoteRequestMsg struct{}

// ApolloCheckoutRequestMsg — żądanie przełączenia gałęzi z panelu Apollo.
// Wysyłane gdy Apollo musi automatycznie przełączyć na właściwą gałąź.
type ApolloCheckoutRequestMsg struct {
	Branch string
}

// ApolloCheckoutDoneMsg — wynik automatycznego przełączenia gałęzi przez Apollo.
type ApolloCheckoutDoneMsg struct {
	Branch string
	Err    error
}

// ApolloModeEnteredMsg — informacja o wejściu w tryb Apollo.
// Używane do wyświetlenia komunikatu o zmianie trybu.
type ApolloModeEnteredMsg struct{}

// GitPanelModeEnteredMsg — informacja o wejściu w tryb GitPanel.
// Używane do wyświetlenia komunikatu o zmianie trybu.
type GitPanelModeEnteredMsg struct{}
