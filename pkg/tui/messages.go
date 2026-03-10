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

// RollbackStartMsg inicjuje procedurę rollbacku.
type RollbackStartMsg struct {
	Env         string
	DeployID    string
	CommitHash  string
	Branch      string
	Description string
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

// GitCommitRequestMsg — żądanie stage all + commit (z git panelu → root model).
type GitCommitRequestMsg struct {
	Message string
}

// GitCommitDoneMsg — wynik operacji git commit.
type GitCommitDoneMsg struct {
	Hash string // skrócony hash nowego commita
	Err  error
}
