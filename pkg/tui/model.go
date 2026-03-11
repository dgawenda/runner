// file: pkg/tui/model.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Root Model Bubble Tea — Maszyna Stanów Aplikacji                  ║
// ║                                                                      ║
// ║  Implementuje architekturę Elm (Model-Update-View) dla Bubble Tea. ║
// ║  Orkiestruje przejścia między ekranami i koordynuje:               ║
// ║    · Setup Wizard (gdy brak konfiguracji)                          ║
// ║    · Dashboard (główny panel sterowania)                           ║
// ║    · Deploy (wykonanie potoku z progress bars)                     ║
// ║    · Rollback (przywracanie z snapshotu)                           ║
// ║    · Promote (migracje DB staging → production)                    ║
// ╚══════════════════════════════════════════════════════════════════════╝

package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/gitops"
	"github.com/neution/rnr/pkg/logger"
	"github.com/neution/rnr/pkg/providers"
	"github.com/neution/rnr/pkg/state"
)

// ─── Typy Ekranów ─────────────────────────────────────────────────────────

// Screen to typ wyliczeniowy ekranów aplikacji.
type Screen int

const (
	ScreenLoading      Screen = iota // Ładowanie konfiguracji
	ScreenWizard                     // Setup Wizard (pierwsze uruchomienie)
	ScreenDashboard                  // Główny Dashboard
	ScreenConfirm                    // Potwierdzenie wdrożenia (środowiska chronione)
	ScreenDeploy                     // Ekran wdrożenia z progress bars
	ScreenGit                        // Git Panel — status, gałęzie, historia, commit
	ScreenRollback                   // Ekran wykonywania rollbacku (progress bars)
	ScreenRollbackPick               // Ekran wyboru wdrożenia do rollbacku (lista)
	ScreenPromote                    // Ekran promote (migracje DB)
	ScreenLogs                       // Przeglądarka logów
	ScreenError                      // Ekran błędu
	ScreenEnvAdd                     // Kreator dodawania nowego środowiska
)

// ─── Wiadomości wewnętrzne pipeline ───────────────────────────────────────

// pipelineEventMsg to wiadomość z kanału pipeline do TUI.
type pipelineEventMsg struct {
	kind       string // "stage_start" | "stage_output" | "stage_done" | "stage_fail" | "done" | "fail"
	index      int
	name       string
	line       string
	durationMS int64
	allowFail  bool
	err        error
	deployID   string
	logFile    string
}

// ─── RollbackPickState ────────────────────────────────────────────────────

// rollbackPickState przechowuje stan ekranu wyboru wdrożenia do rollbacku.
type rollbackPickState struct {
	env     string
	records []state.DeployRecord
	cursor  int
	// onlyOne — true gdy jest tylko 1 wdrożenie (rollback możliwy, ale bez poprzedniej wersji)
	onlyOne bool
}

// ─── Root Model ────────────────────────────────────────────────────────────

// RootModel to główny model aplikacji Bubble Tea.
type RootModel struct {
	// Rozmiary terminala
	width  int
	height int

	// Aktualny ekran
	screen Screen

	// Ścieżka do projektu
	projectRoot string

	// Konfiguracja
	cfg *config.Config

	// Stan wdrożeń
	stateData *state.State

	// Status Git
	gitStatus *gitops.StatusResult

	// Sub-modele ekranów
	wizard    WizardModel
	dashboard DashboardModel
	deploy    DeployModel
	gitPanel  GitPanelModel

	// Spinner ładowania
	spinner spinner.Model
	loading bool
	loadMsg string

	// Potwierdzenie
	confirmEnv    string
	confirmPrompt string
	pendingEnv    string

	// Ekran błędu
	errTitle string
	errMsg   string

	// Aktywny deployment
	deployID  string
	deployEnv string

	// Kanał pipeline
	pipelineCh chan pipelineEventMsg

	// Rollback — ekran wykonania (ScreenRollback)
	rollbackEnv string

	// RollbackPick — ekran wyboru wdrożenia (ScreenRollbackPick)
	rollbackPick rollbackPickState

	// Promote
	promoteSource string
	promoteTarget string

	// Przeglądarka logów
	logs LogsModel
}

// ─── Inicjalizacja ────────────────────────────────────────────────────────

// NewRootModel tworzy nowy root model dla podanego katalogu projektu.
func NewRootModel(projectRoot string) *RootModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	return &RootModel{
		projectRoot: projectRoot,
		screen:      ScreenLoading,
		spinner:     sp,
		loading:     true,
		loadMsg:     "Ładowanie konfiguracji...",
		width:       80,
		height:      24,
	}
}

// ─── Interfejs Bubble Tea ─────────────────────────────────────────────────

// Init uruchamia pierwszą komendę — ładowanie konfiguracji.
// Uruchamia też ticker auto-odświeżania statusu Git (co 3s).
func (m *RootModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.cmdLoadConfig(),
		cmdGitPollTick(), // start live git refresh
	)
}

// cmdGitPollTick planuje kolejne odświeżenie statusu Git za 3 sekundy.
func cmdGitPollTick() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return GitRefreshTickMsg{T: t}
	})
}

// Update to serce pętli Elm — obsługuje wszystkie zdarzenia.
func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.dashboard.width = msg.Width
		m.dashboard.height = msg.Height
		m.deploy.width = msg.Width
		m.deploy.height = msg.Height

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	// ── Ładowanie ─────────────────────────────────────────────────────
	case ConfigLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			// hasPipeline=true oznacza: rnr.yaml jest (sklonowane repo), brak conf.yaml
			// W nowej strukturze to najczęstszy scenariusz "nowy developer w projekcie"
			hasPipeline, _ := config.Exists(m.projectRoot)
			m.wizard = NewWizardModelWithFlags(m.width, m.height, hasPipeline)
			m.screen = ScreenWizard
			cmds = append(cmds, m.wizard.Init())
		} else {
			m.loading = true
			m.loadMsg = "Sprawdzam repozytorium Git..."
			cmds = append(cmds, m.cmdAuditGit(), m.spinner.Tick)
		}

	// ── Auto-odświeżanie Git (live polling co 3s) ─────────────────────
	case GitRefreshTickMsg:
		// Zawsze planuj kolejny tick — niezależnie od ekranu
		cmds = append(cmds, cmdGitPollTick())
		// Odśwież status tylko na Dashboard lub Git Panel (nie podczas deploy)
		if m.screen == ScreenDashboard || m.screen == ScreenGit {
			cmds = append(cmds, m.cmdAuditGitSilent())
		}
		// Odśwież graf jeśli jesteśmy w Git Panelu
		if m.screen == ScreenGit {
			cmds = append(cmds, m.cmdGitLoadGraph())
		}

	case GitStatusMsg:
		// Pierwsza inicjalizacja (loading=true) — przejdź do ładowania stanu
		if m.loading {
			if msg.Err != nil {
				m.showError("Błąd Git", msg.Err.Error(), msg.Err)
				return m, nil
			}
			m.gitStatus = msg.Result
			m.dashboard.gitStatus = msg.Result
			m.gitPanel.gitStatus = msg.Result
			m.loading = true
			m.loadMsg = "Ładowanie historii wdrożeń..."
			cmds = append(cmds, m.cmdLoadState(), m.spinner.Tick)
		} else {
			// Polling — cicha aktualizacja bez zmiany ekranu
			if msg.Err == nil {
				m.gitStatus = msg.Result
				m.dashboard.gitStatus = msg.Result
				m.gitPanel.gitStatus = msg.Result
			}
			// Przekaż też do gitPanel dla natychmiastowego renderowania
			var gpCmd tea.Cmd
			m.gitPanel, gpCmd = m.gitPanel.Update(msg)
			cmds = append(cmds, gpCmd)
		}

	case StateLoadedMsg:
		m.loading = false
		if msg.Err == nil {
			m.stateData = msg.State
		} else {
			m.stateData = &state.State{Version: 1, Deployments: []state.DeployRecord{}}
		}
		m.dashboard = NewDashboardModel(m.width, m.height, m.cfg, m.gitStatus, m.stateData)
		m.screen = ScreenDashboard

	// ── Git Panel — operacje ───────────────────────────────────────────
	case GitCheckoutRequestMsg:
		// Żądanie checkout z GitPanel → wykonaj i odśwież
		cmds = append(cmds, m.cmdGitCheckout(msg.Branch))

	case GitCommitRequestMsg:
		// Żądanie commit z GitPanel → stage wybranych plików + commit
		cmds = append(cmds, m.cmdGitStageAndCommit(msg.Message, msg.Files))

	case GitCheckoutDoneMsg:
		// Wynik checkout → przekaż do gitPanel + odśwież git status + gałęzie
		var gpCmd tea.Cmd
		m.gitPanel, gpCmd = m.gitPanel.Update(msg)
		cmds = append(cmds, gpCmd)
		cmds = append(cmds, m.cmdAuditGitSilent(), m.cmdGitLoadBranches())

	case GitPushRequestMsg:
		// Żądanie push z GitPanel → wypchnij bieżącą gałąź
		cmds = append(cmds, m.cmdGitPush())

	case GitPushDoneMsg:
		// Wynik push → przekaż do gitPanel
		var gpCmd tea.Cmd
		m.gitPanel, gpCmd = m.gitPanel.Update(msg)
		cmds = append(cmds, gpCmd)

	case GitPullRebasePushRequestMsg:
		// Żądanie pull+rebase+push po non-fast-forward
		cmds = append(cmds, m.cmdGitPullRebasePush(msg.Branch))

	case GitPullRebasePushDoneMsg:
		// Wynik pull+rebase+push → przekaż do gitPanel + odśwież status
		var gpCmd tea.Cmd
		m.gitPanel, gpCmd = m.gitPanel.Update(msg)
		cmds = append(cmds, gpCmd)
		cmds = append(cmds, m.cmdAuditGitSilent(), m.cmdGitLoadHistory())

	case GitForcePushRequestMsg:
		// Żądanie force-with-lease push
		cmds = append(cmds, m.cmdGitForcePushWithLease(msg.Branch))

	case GitCommitDoneMsg:
		// Wynik commit → przekaż do gitPanel + odśwież git status + historię
		var gpCmd tea.Cmd
		m.gitPanel, gpCmd = m.gitPanel.Update(msg)
		cmds = append(cmds, gpCmd)
		cmds = append(cmds, m.cmdAuditGitSilent(), m.cmdGitLoadHistory())

	case GitBranchesLoadedMsg:
		var gpCmd tea.Cmd
		m.gitPanel, gpCmd = m.gitPanel.Update(msg)
		cmds = append(cmds, gpCmd)

	case GitHistoryLoadedMsg:
		var gpCmd tea.Cmd
		m.gitPanel, gpCmd = m.gitPanel.Update(msg)
		cmds = append(cmds, gpCmd)

	case GitGraphLoadedMsg:
		// Graf commitów załadowany → przekaż do gitPanel
		var gpCmd tea.Cmd
		m.gitPanel, gpCmd = m.gitPanel.Update(msg)
		cmds = append(cmds, gpCmd)

	case GitDiffRequestMsg:
		// Żądanie diff pliku z GitPanel → załaduj diff
		cmds = append(cmds, m.cmdGitLoadDiff(msg.File))

	case GitDiffLoadedMsg:
		// Diff załadowany → przekaż do gitPanel
		var gpCmd tea.Cmd
		m.gitPanel, gpCmd = m.gitPanel.Update(msg)
		cmds = append(cmds, gpCmd)

	// ── Wizard ────────────────────────────────────────────────────────
	case WizardCompleteMsg:
		// Pokaż ekran ładowania podczas tworzenia konfiguracji,
		// projektów Netlify i gałęzi środowiskowych — może to chwilę potrwać.
		m.loading = true
		m.screen = ScreenLoading
		m.loadMsg = "Inicjalizuję projekt — tworzę konfigurację..."
		cmds = append(cmds, m.cmdGenerateConfigs(msg), m.spinner.Tick)

	// ── Nawigacja ─────────────────────────────────────────────────────
	case NavigateMsg:
		switch msg.Screen {
		case ScreenDashboard:
			m.loading = true
			m.screen = ScreenLoading
			m.loadMsg = "Odświeżam..."
			cmds = append(cmds, m.cmdLoadConfig(), m.spinner.Tick)
		default:
			m.screen = msg.Screen
		}

	case ErrorMsg:
		m.showError(msg.Title, msg.Message, msg.Err)

	// ── Deploy ────────────────────────────────────────────────────────
	case ConfirmDeployMsg:
		env := msg.Env
		m.confirmEnv = env
		m.pendingEnv = env
		m.confirmPrompt = fmt.Sprintf(
			"Środowisko '%s' jest oznaczone jako chronione (protected: true).\n\n"+
				"Upewnij się, że:\n"+
				"  ✓ Kod przeszedł testy i code review\n"+
				"  ✓ Masz zgodę zespołu na wdrożenie\n"+
				"  ✓ Czas wdrożenia jest odpowiedni\n\n"+
				"Kontynuować wdrożenie na PRODUKCJĘ?", env)
		m.screen = ScreenConfirm

	case DeployStartMsg:
		cmds = append(cmds, m.cmdStartDeploy(msg.Env))

	// ── Zdarzenia Pipeline ────────────────────────────────────────────
	case pipelineEventMsg:
		switch msg.kind {
		case "snapshot":
			var cmd tea.Cmd
			m.deploy, cmd = m.deploy.Update(SnapshotCreatedMsg{
				Branch: msg.name,
				Hash:   msg.line,
			})
			cmds = append(cmds, cmd)

		case "stage_start":
			var cmd tea.Cmd
			m.deploy, cmd = m.deploy.Update(StageStartedMsg{Index: msg.index, Name: msg.name})
			cmds = append(cmds, cmd)

		case "stage_output":
			var cmd tea.Cmd
			m.deploy, cmd = m.deploy.Update(StageOutputMsg{Index: msg.index, Line: msg.line})
			cmds = append(cmds, cmd)

		case "stage_done":
			var cmd tea.Cmd
			m.deploy, cmd = m.deploy.Update(StageCompletedMsg{
				Index:      msg.index,
				Name:       msg.name,
				DurationMS: msg.durationMS,
			})
			cmds = append(cmds, cmd)

		case "stage_fail":
			var cmd tea.Cmd
			m.deploy, cmd = m.deploy.Update(StageFailedMsg{
				Index:        msg.index,
				Name:         msg.name,
				DurationMS:   msg.durationMS,
				Err:          msg.err,
				AllowFailure: msg.allowFail,
			})
			cmds = append(cmds, cmd)

		case "stage_skip":
			var cmd tea.Cmd
			m.deploy, cmd = m.deploy.Update(StageSkippedMsg{Index: msg.index, Name: msg.name})
			cmds = append(cmds, cmd)

		case "done":
			var cmd tea.Cmd
			m.deploy, cmd = m.deploy.Update(DeployCompletedMsg{
				Env:        m.deployEnv,
				DeployID:   msg.deployID,
				LogFile:    msg.logFile,
				TotalSteps: len(m.cfg.GetStagesForEnv(m.deployEnv)),
			})
			cmds = append(cmds, cmd)
			cmds = append(cmds, m.cmdMarkDeploySuccess(msg.deployID))

		case "fail":
			var cmd tea.Cmd
			m.deploy, cmd = m.deploy.Update(DeployFailedMsg{
				Env:      m.deployEnv,
				DeployID: msg.deployID,
				StepName: msg.name,
				Err:      msg.err,
			})
			cmds = append(cmds, cmd)
			cmds = append(cmds, m.cmdMarkDeployFailed(msg.deployID))
		}

		// Re-arm kanał nasłuchiwania
		if m.pipelineCh != nil {
			cmds = append(cmds, m.cmdWaitPipeline())
		}

	// ── Rollback — wybór wdrożenia ────────────────────────────────────
	case ShowRollbackPickMsg:
		m.rollbackPick = rollbackPickState{
			env:     msg.Env,
			records: msg.Records,
			cursor:  0,
			onlyOne: len(msg.Records) == 1,
		}
		m.screen = ScreenRollbackPick

	// ── Rollback — wykonanie ──────────────────────────────────────────
	case RollbackStartMsg:
		cmds = append(cmds, m.cmdExecuteRollback(msg))

	case RollbackCompletedMsg:
		m.loading = true
		m.screen = ScreenLoading
		m.loadMsg = "Rollback zakończony — powrót do Dashboard..."
		cmds = append(cmds, m.cmdLoadConfig(), m.spinner.Tick)

	case RollbackFailedMsg:
		m.showError("Błąd Rollbacku",
			"Rollback nieudany: "+msg.Err.Error()+"\n\n"+
				"Wykonaj rollback manualnie:\n  git reset --hard <HASH_COMMITA>",
			msg.Err)

	// ── Promote ───────────────────────────────────────────────────────
	case PromoteStartMsg:
		m.promoteSource = msg.SourceEnv
		m.promoteTarget = msg.TargetEnv
		// Uwaga: m.screen = ScreenPromote jest ustawiane wewnątrz cmdExecutePromote
		// (która tworzy też deploy model + pipelineCh synchronicznie przed Batch).
		cmds = append(cmds, m.cmdExecutePromote(msg))

	case PromoteCompletedMsg:
		// Zachowany dla kompatybilności — nowy flow używa pipelineEventMsg (DeployCompletedMsg).
		// Może być wywołany przez stare ścieżki kodu.
		m.loading = true
		m.screen = ScreenLoading
		m.loadMsg = "Promote zakończony..."
		cmds = append(cmds, m.cmdLoadConfig(), m.spinner.Tick)

	case PromoteFailedMsg:
		// Zachowany dla kompatybilności — nowy flow używa pipelineEventMsg (DeployFailedMsg).
		m.showError("Błąd Promote", msg.Err.Error(), msg.Err)

	// ── Klawiatura ────────────────────────────────────────────────────
	case tea.KeyMsg:
		if cmd := m.handleGlobalKeys(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// ── Propagacja do sub-modeli ──────────────────────────────────────
	switch m.screen {
	case ScreenWizard:
		var cmd tea.Cmd
		m.wizard, cmd = m.wizard.Update(msg)
		cmds = append(cmds, cmd)
	case ScreenDashboard:
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		cmds = append(cmds, cmd)
	case ScreenDeploy, ScreenRollback, ScreenPromote:
		var cmd tea.Cmd
		m.deploy, cmd = m.deploy.Update(msg)
		cmds = append(cmds, cmd)
	case ScreenLogs:
		var cmd tea.Cmd
		m.logs, cmd = m.logs.Update(msg)
		cmds = append(cmds, cmd)
		// ESC na liście logów wraca do Dashboard
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if !m.logs.viewing && (keyMsg.String() == "esc" || keyMsg.String() == "q") {
				m.screen = ScreenDashboard
			}
		}
	case ScreenGit:
		// Przekaż zdarzenia do git panelu — gitPanel przetwarza je PIERWSZY
		var cmd tea.Cmd
		m.gitPanel, cmd = m.gitPanel.Update(msg)
		cmds = append(cmds, cmd)
		// Zamknięcie panelu: TYLKO klawisz Q lub ESC.
		// "G" jest celowo POMINIĘTE tutaj — naciśnięcie G z Dashboard ustawia
		// screen=ScreenGit synchronicznie w cmdOpenGitPanel(), a następna iteracja
		// pętli trafia już do case ScreenGit. Gdyby obsługiwać tu G→zamknij,
		// panel otwierałby się i natychmiast zamykał w JEDNEJ ramce.
		// Użytkownik zamyka panel klawiszem Q lub ESC.
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "q":
				// Nie zamykaj jeśli użytkownik pisze commit (input sfocusowany)
				if !m.gitPanel.commitInput.Focused() {
					m.screen = ScreenDashboard
				}
			case "esc":
				// Nie zamykaj jeśli: diff jest otwarty (ESC = zamknij diff)
				//                  lub input jest sfocusowany (ESC = odblokuj input)
				if !m.gitPanel.showDiff && !m.gitPanel.commitInput.Focused() {
					m.screen = ScreenDashboard
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renderuje aktualny ekran.
func (m *RootModel) View() string {
	switch m.screen {
	case ScreenLoading:
		return m.viewLoading()
	case ScreenWizard:
		return m.viewCentered(m.wizard.View())
	case ScreenDashboard:
		return m.dashboard.View()
	case ScreenDeploy, ScreenRollback:
		return m.deploy.View()
	case ScreenRollbackPick:
		return m.viewRollbackPick()
	case ScreenConfirm:
		return m.viewConfirm()
	case ScreenPromote:
		return m.deploy.View()
	case ScreenLogs:
		return m.logs.View()
	case ScreenGit:
		return m.gitPanel.View()
	case ScreenError:
		return m.viewError()
	default:
		return "Nieznany ekran — odświeżam..."
	}
}

// ─── Widoki ───────────────────────────────────────────────────────────────

func (m *RootModel) viewLoading() string {
	maxW := min(m.width-4, 64)
	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		RnrLogo(),
		"",
		lipgloss.NewStyle().Foreground(ColorSubtext).
			Render(m.spinner.View()+"  "+m.loadMsg),
		"",
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		StylePanel.Width(maxW).Render(content))
}

func (m *RootModel) viewError() string {
	maxW := min(m.width-4, 72)
	titleStr := StyleError.Bold(true).Render("❌ " + m.errTitle)
	msgStr := lipgloss.NewStyle().Foreground(ColorText).Width(maxW - 6).Render(m.errMsg)
	hint := StyleMuted.Render("\n  Q = wyjdź • ENTER = powrót")
	content := lipgloss.JoinVertical(lipgloss.Left,
		"", titleStr, "", msgStr, hint, "",
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		StylePanelError.Width(maxW).Render(content))
}

func (m *RootModel) viewConfirm() string {
	maxW := min(m.width-4, 64)
	warning := StyleWarning.Bold(true).Render("⚠️  UWAGA: Środowisko chronione!")
	prompt := lipgloss.NewStyle().Foreground(ColorText).Width(maxW - 6).Render(m.confirmPrompt)
	env := EnvBadge(m.confirmEnv)
	buttons := lipgloss.JoinHorizontal(lipgloss.Top,
		StyleButton.Render(" ENTER = Potwierdź "),
		"  ",
		StyleButtonSecondary.Render(" ESC = Anuluj "),
	)
	content := lipgloss.JoinVertical(lipgloss.Left,
		"", warning, "", env, "", prompt, "", buttons, "",
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		StylePanelError.Width(maxW).Render(content))
}

// viewRollbackPick renderuje ekran wyboru wdrożenia do rollbacku.
// Wyświetla interaktywną listę udanych wdrożeń z danymi:
// numer, data, hash commita, wiadomość, autor.
func (m *RootModel) viewRollbackPick() string {
	maxW := min(m.width-4, 82)
	rp := m.rollbackPick
	records := rp.records

	// ── Nagłówek ──────────────────────────────────────────────────────────
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	envBadge := EnvBadge(rp.env)
	titleStr := headerStyle.Render("↩️  Rollback — wybierz wdrożenie")

	subStr := lipgloss.NewStyle().Foreground(ColorSubtext).
		Render("Wybrane wdrożenie zostanie przywrócone do środowiska " + rp.env + ".")

	// ── Ostrzeżenie dla jedynego wdrożenia ────────────────────────────────
	warnStr := ""
	if rp.onlyOne {
		warnStr = StyleWarning.Render(
			"⚠️  Tylko jedno wdrożenie w historii — rollback przywróci obecny stan kodu.",
		)
	}

	// ── Lista wdrożeń ─────────────────────────────────────────────────────
	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#44475A")).
		Foreground(lipgloss.Color("#F8F8F2")).
		Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(ColorText)
	mutedStyle := lipgloss.NewStyle().Foreground(ColorSubtext)
	hashStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
	rolledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9"))

	var rows []string
	// Nagłówek tabeli
	headerRow := mutedStyle.Render(fmt.Sprintf(
		"  %-3s  %-16s  %-7s  %-8s  %s",
		"#", "Data", "Commit", "Status", "Opis",
	))
	rows = append(rows, headerRow)
	rows = append(rows, mutedStyle.Render(strings.Repeat("─", min(maxW-6, 74))))

	for i, rec := range records {
		cursor := "  "
		if i == rp.cursor {
			cursor = "▶ "
		}

		// Format daty
		dateStr := rec.StartedAt.Format("02.01 15:04")

		// Krótki hash commita (7 znaków)
		hash := rec.CommitHash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		if hash == "" {
			hash = "-------"
		}

		// Status ikonka
		statusStr := ""
		switch rec.Status {
		case state.StatusSuccess:
			statusStr = successStyle.Render("✓ ok    ")
		case state.StatusRolledBack:
			statusStr = rolledStyle.Render("↩ cofn. ")
		default:
			statusStr = mutedStyle.Render("? unk.  ")
		}

		// Wiadomość commita (skrócona)
		msg := truncateString(rec.CommitMessage, 38)
		if msg == "" {
			msg = mutedStyle.Render("(brak opisu)")
		}

		// Autor (skrócony)
		author := truncateString(rec.CommitAuthor, 14)
		if author != "" {
			msg += mutedStyle.Render("  @" + author)
		}

		rowContent := fmt.Sprintf("%s%-3d  %-16s  %s  %s  %s",
			cursor,
			i+1,
			dateStr,
			hashStyle.Render(hash),
			statusStr,
			msg,
		)

		if i == rp.cursor {
			rows = append(rows, selectedStyle.Render(" "+rowContent+" "))
		} else {
			rows = append(rows, normalStyle.Render(" "+rowContent))
		}
	}

	listStr := strings.Join(rows, "\n")

	// ── Skróty klawiaturowe ───────────────────────────────────────────────
	navHint := StyleMuted.Render(
		"\n  ↑↓ / j k = nawigacja   " +
			"ENTER / SPACJA = potwierdź rollback   " +
			"ESC / Q = anuluj",
	)

	// ── Złóż widok ────────────────────────────────────────────────────────
	parts := []string{"", titleStr, "", envBadge, "", subStr, ""}
	if warnStr != "" {
		parts = append(parts, warnStr, "")
	}
	parts = append(parts, listStr, navHint, "")

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top,
		StylePanelAccent.Width(maxW).Render(content))
}

// truncateString skraca string do maxLen znaków, dodając "…" jeśli jest dłuższy.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

func (m *RootModel) viewCentered(content string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// ─── Obsługa Klawiszy ─────────────────────────────────────────────────────

func (m *RootModel) handleGlobalKeys(msg tea.KeyMsg) tea.Cmd {
	switch m.screen {
	case ScreenDashboard:
		return m.handleDashboardKeys(msg)

	case ScreenError:
		switch msg.String() {
		case "q", "ctrl+c":
			return tea.Quit
		case "enter", "esc":
			m.loading = true
			m.screen = ScreenLoading
			m.loadMsg = "Odświeżam..."
			return tea.Batch(m.cmdLoadConfig(), m.spinner.Tick)
		}

	case ScreenConfirm:
		switch msg.Type {
		case tea.KeyEnter:
			env := m.pendingEnv
			m.pendingEnv = ""
			m.screen = ScreenDashboard
			return m.cmdStartDeploy(env)
		case tea.KeyEsc:
			m.screen = ScreenDashboard
		}

	case ScreenDeploy, ScreenRollback, ScreenPromote:
		if (msg.String() == "enter" || msg.String() == "q") &&
			(m.deploy.completed || m.deploy.failed) {
			m.loading = true
			m.screen = ScreenLoading
			m.loadMsg = "Powrót do Dashboard..."
			return tea.Batch(m.cmdLoadConfig(), m.spinner.Tick)
		}
		if msg.String() == "ctrl+c" {
			return tea.Quit
		}

	case ScreenRollbackPick:
		// Obsługa nawigacji na liście wdrożeń
		n := len(m.rollbackPick.records)
		switch msg.String() {
		case "ctrl+c":
			return tea.Quit
		case "esc", "q", "Q":
			// Anuluj — wróć do Dashboard
			m.screen = ScreenDashboard
		case "up", "k", "K":
			if m.rollbackPick.cursor > 0 {
				m.rollbackPick.cursor--
			}
		case "down", "j", "J":
			if m.rollbackPick.cursor < n-1 {
				m.rollbackPick.cursor++
			}
		case "enter", " ":
			// Potwierdź — uruchom rollback do wybranego wdrożenia
			if n > 0 {
				rec := m.rollbackPick.records[m.rollbackPick.cursor]
				m.screen = ScreenDashboard // zmieniony przez cmdExecuteRollback → ScreenRollback
				return func() tea.Msg {
					return RollbackStartMsg{
						Env:        rec.Env,
						DeployID:   rec.ID,
						CommitHash: rec.CommitHash,
						Branch:     rec.Branch,
						Description: fmt.Sprintf("%s — %s",
							rec.StartedAt.Format("2006-01-02 15:04"),
							truncateString(rec.CommitMessage, 50),
						),
					}
				}
			}
		}
	}
	return nil
}

func (m *RootModel) handleDashboardKeys(msg tea.KeyMsg) tea.Cmd {
	selectedEnv := m.dashboard.SelectedEnv()

	switch msg.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "d", "D":
		if selectedEnv == "" {
			return nil
		}
		return m.cmdInitDeploy(selectedEnv)
	case "r", "R":
		return m.cmdInitRollback(selectedEnv)
	case "p", "P":
		return m.cmdInitPromote()
	case "l", "L":
		return m.cmdOpenLogs()
	case "g", "G":
		// Otwórz Git Panel z aktualnymi danymi
		return m.cmdOpenGitPanel()
	}
	return nil
}

// ─── Komendy ─────────────────────────────────────────────────────────────

func (m *RootModel) cmdLoadConfig() tea.Cmd {
	return func() tea.Msg {
		hasPipeline, hasConf := config.Exists(m.projectRoot)

		// Brak obu plików → pełny wizard
		if !hasPipeline && !hasConf {
			return ConfigLoadedMsg{Err: fmt.Errorf("brak pliku konfiguracyjnego")}
		}

		// conf istnieje, ale brak pipeline → pokaż wizard (conf nowej struktury
		// zawiera TYLKO sekrety, nie ma informacji o środowiskach/dostawcach)
		if hasConf && !hasPipeline {
			// Nie można auto-wygenerować rnr.yaml z samych sekretów.
			// Uruchamiamy wizard z flagą hasExistingConf=false (pełna konfiguracja).
			return ConfigLoadedMsg{Err: fmt.Errorf("brak pliku rnr.yaml")}
		}

		// Brak conf → pokaż wizard (rnr.yaml może istnieć — np. świeży clone repo)
		if !hasConf {
			return ConfigLoadedMsg{Err: fmt.Errorf("brak pliku rnr.conf.yaml")}
		}

		cfg, err := config.Load(m.projectRoot)
		if err != nil {
			return ConfigLoadedMsg{Err: err}
		}
		m.cfg = cfg
		return ConfigLoadedMsg{Err: nil}
	}
}

func (m *RootModel) cmdAuditGit() tea.Cmd {
	return func() tea.Msg {
		result, err := gitops.AuditRepo(m.projectRoot)
		return GitStatusMsg{Result: result, Err: err}
	}
}

func (m *RootModel) cmdLoadState() tea.Cmd {
	return func() tea.Msg {
		stateFile := filepath.Join(m.projectRoot, config.StateFile)
		s, err := state.Load(stateFile)
		return StateLoadedMsg{State: s, Err: err}
	}
}

func (m *RootModel) cmdGenerateConfigs(w WizardCompleteMsg) tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		if err := config.EnsureRnrDir(root); err != nil {
			return ErrorMsg{Title: "Błąd katalogu", Message: err.Error(), Err: err}
		}

		// Otwórz log inicjalizacji — użytkownik może go sprawdzić przez L → lista logów
		initLog, _ := logger.NewForDeployment(
			filepath.Join(root, config.LogsDir),
			"init", logger.NopMasker(),
		)
		logInit := func(format string, args ...any) {
			if initLog != nil {
				initLog.Info(format, args...)
			}
		}
		logInitOK := func(format string, args ...any) {
			if initLog != nil {
				initLog.Success(format, args...)
			}
		}
		logInitErr := func(format string, args ...any) {
			if initLog != nil {
				initLog.Error(format, args...)
			}
		}
		if initLog != nil {
			defer initLog.Close()
		}

		logInit("══════════════════════════════════════════════════════")
		logInit("rnr init: projekt=%s deploy=%s db=%s",
			w.ProjectName, w.DeployProv, w.DBProv)
		logInit("══════════════════════════════════════════════════════")

		// ── Krok 0: Inicjalizacja Git (jeśli brak repo) ───────────────────────
		// Gdy katalog nie jest repo Git — tworzymy lokalne repo automatycznie.
		// Działa out-of-the-box: nie wymaga żadnej akcji od użytkownika.
		if !gitops.HasGitRepo(root) {
			logInit("Git: katalog nie jest repo — wywołuję `git init`...")
			if err := gitops.InitRepo(root); err != nil {
				logInitErr("Git init: błąd: %v", err)
				// Niekrytyczne — kontynuuj inicjalizację
			} else {
				logInitOK("Git init: zainicjalizowano repo w %s", root)
				// Ustaw user.email i user.name jeśli nie są skonfigurowane globalnie
				// (niezbędne dla pierwszego commita)
				gitops.EnsureGitIdentity(root)
			}
		} else {
			logInit("Git: repozytorium już istnieje")
		}

		// ── Krok 1: rnr.yaml (generuj tylko jeśli nie istnieje) ──────────────
		pipelinePath := filepath.Join(root, config.PipelineFile)
		if _, err := os.Stat(pipelinePath); os.IsNotExist(err) {
			projectType := w.ProjectType
			if projectType == "" {
				projectType = "custom"
			}
			content := config.DefaultPipelineYAMLFromWizard(
				w.ProjectName, w.Repo,
				projectType, w.DeployProv, w.DBProv,
			)
			if err := os.WriteFile(pipelinePath, []byte(content), 0o644); err != nil {
				logInitErr("Błąd zapisu rnr.yaml: %v", err)
				return ErrorMsg{Title: "Błąd zapisu rnr.yaml", Message: err.Error(), Err: err}
			}
			logInitOK("Zapisano rnr.yaml")
		} else {
			logInit("rnr.yaml już istnieje — pominięto")
		}

		// ── Krok 2: Automatyczne tworzenie projektów Netlify przez REST API ───
		// Jeśli netlify_create_new: true i token jest podany, zakładamy TERAZ projekty
		// dla produkcji i development — użytkownik nie musi wychodzić z TUI do Netlify UI.
		prodSiteID := w.NetlifySiteID // może być pusty (jeśli user wybrał "utwórz nowy")
		stagingSiteID := ""
		prodCreateNew := w.NetlifyCreateNew && prodSiteID == ""
		stagingCreateNew := w.NetlifyCreateNew

		if w.DeployProv == "netlify" && w.NetlifyCreateNew && w.NetlifyToken != "" {
			logInit("Netlify: tworzenie projektów przez REST API...")

			// Projekt produkcyjny (nazwa projektu bez sufiksu)
			if prodSiteID == "" {
				logInit("Netlify: tworzenie projektu produkcji '%s'...", w.ProjectName)
				id, logs, createErr := providers.NetlifyCreateSiteWithLog(w.NetlifyToken, w.ProjectName)
				for _, l := range logs {
					logInit("  Netlify API: %s", l)
				}
				if createErr == nil && id != "" {
					prodSiteID = id
					prodCreateNew = false // Sukces — nie potrzeba tworzyć przy deployu
					logInitOK("Netlify production: Site ID = %s", id)
				} else if createErr != nil {
					logInitErr("Netlify production: błąd tworzenia: %v", createErr)
					logInit("  → Site zostanie założony automatycznie przy pierwszym deploy")
				}
			}

			// Projekt development (sufiks -dev dla rozróżnienia od produkcji)
			stagingName := w.ProjectName + "-dev" // krótka nazwa, czytelna w Netlify
			logInit("Netlify: tworzenie projektu development '%s'...", stagingName)
			id, logs, createErr := providers.NetlifyCreateSiteWithLog(w.NetlifyToken, stagingName)
			for _, l := range logs {
				logInit("  Netlify API: %s", l)
			}
			if createErr == nil && id != "" {
				stagingSiteID = id
				stagingCreateNew = false // Sukces — nie potrzeba tworzyć przy deployu
				logInitOK("Netlify development: Site ID = %s", id)
			} else if createErr != nil {
				logInitErr("Netlify development: błąd tworzenia: %v", createErr)
				logInit("  → Site zostanie założony automatycznie przy pierwszym deploy")
			}
		} else if w.DeployProv == "netlify" {
			logInit("Netlify: pomijam auto-tworzenie (netlify_create_new=false lub brak tokenu)")
		}

		// ── Krok 3: rnr.conf.yaml — sekrety z finalnymi Site ID ──────────────
		confPath := filepath.Join(root, config.ConfFile)
		confContent := config.DefaultConfYAMLFromWizard(
			w.ProjectName, w.Repo,
			w.DeployProv, w.NetlifyToken,
			prodSiteID, prodCreateNew,
			stagingSiteID, stagingCreateNew,
			w.DBProv, w.SupabaseRef, w.SupabaseURL, w.SupabaseKey,
		)
		if err := os.WriteFile(confPath, []byte(confContent), 0o600); err != nil {
			logInitErr("Błąd zapisu rnr.conf.yaml: %v", err)
			return ErrorMsg{Title: "Błąd zapisu rnr.conf.yaml", Message: err.Error(), Err: err}
		}
		if prodSiteID != "" {
			logInitOK("Zapisano rnr.conf.yaml (production site_id=%s)", prodSiteID)
		} else {
			logInitOK("Zapisano rnr.conf.yaml (production site_id zostanie ustawiony przy deployu)")
		}
		if stagingSiteID != "" {
			logInitOK("  development site_id=%s", stagingSiteID)
		}

		_ = config.EnsureGitignore(root)

		// ── Krok 4: GitHub remote (git remote add/set-url origin <url>) ───────
		// Jeśli użytkownik podał URL repo w wizardzie, ustawiamy remote origin.
		// Operacja niekrytyczna — błąd nie blokuje inicjalizacji.
		if w.GitHubRemoteURL != "" {
			logInit("GitHub remote: ustawianie origin → %s", w.GitHubRemoteURL)
			if err := gitops.SetRemote(root, w.GitHubRemoteURL); err != nil {
				logInitErr("GitHub remote: nie udało się ustawić: %v", err)
			} else {
				logInitOK("GitHub remote: ustawiono origin → %s", w.GitHubRemoteURL)
			}
		} else {
			logInit("GitHub remote: brak URL — repo pozostaje lokalne (bez origin)")
			logInit("  Aby dodać remote: git remote add origin <URL_REPOZYTORIUM>")
		}

		// ── GitHub CLI (gh) — tworzenie repo jeśli dostępny ──────────────────
		// Jeśli `gh` jest dostępny i użytkownik go wybrał, możemy zaoferować
		// `gh repo create`. W tej wersji tylko logujemy informację — użytkownik
		// może uruchomić `gh repo create` manualnie po inicjalizacji.
		if w.UseGhCLI {
			if ghPath, ghErr := exec.LookPath("gh"); ghErr == nil {
				logInitOK("gh CLI: wykryto w %s — możesz teraz: gh repo create", ghPath)
				logInit("  Przykład: gh repo create %s --private --source=.", w.ProjectName)
			} else {
				logInit("gh CLI: nie znaleziono w PATH (pominięty)")
				logInit("  Instalacja: https://cli.github.com/")
			}
		}

		// ── Krok 5: Automatyczne tworzenie gałęzi środowiskowych ─────────────
		// production → branch "master", development → branch "develop"
		// Tworzy gałęzie BEZ przełączania (git branch <name>).
		// Bezpieczne: brak commitów lub brak repo → pominięte cicho.
		if _, err := gitops.EnsureBranch(root, "master"); err == nil {
			logInitOK("Gałąź 'master' → środowisko production")
		}
		if _, err := gitops.EnsureBranch(root, "develop"); err == nil {
			logInitOK("Gałąź 'develop' → środowisko development")
		}

		logInitOK("══════════════════════════════════════════════════════")
		logInitOK("Inicjalizacja zakończona — sprawdź log: %s", func() string {
			if initLog != nil {
				return initLog.FilePath()
			}
			return ".rnr/logs/init_*.log"
		}())

		return NavigateMsg{Screen: ScreenDashboard}
	}
}

func (m *RootModel) cmdInitDeploy(envName string) tea.Cmd {
	return func() tea.Msg {
		// Sprawdź czystość repo tylko dla projektów Git ze ŚLEDZONYMI modyfikacjami.
		// Pliki nieśledzone (??) nie wpływają na git checkout — nie blokujemy deployu.
		// Blokujemy tylko gdy są ŚLEDZONE modyfikacje (M, D, A, R, C, U).
		if m.gitStatus != nil && m.gitStatus.IsGitRepo && !m.gitStatus.IsClean {
			var trackedDirty []gitops.DirtyFile
			for _, f := range m.gitStatus.DirtyFiles {
				if f.Status != "??" {
					trackedDirty = append(trackedDirty, f)
				}
			}
			if len(trackedDirty) > 0 {
				return ErrorMsg{
					Title: "Niezatwierdzone zmiany",
					Message: fmt.Sprintf(
						"Znaleziono %d niezatwierdzonych plików (śledzone).\n\n"+
							"rnr wykona git checkout na gałąź środowiska — niezatwierdzone\n"+
							"zmiany mogą ulec utracie lub powodować konflikty.\n\n"+
							"Zatwierdź zmiany przed wdrożeniem:\n"+
							"  git add . && git commit -m 'opis'\n\n"+
							"lub odłóż na bok:\n"+
							"  git stash",
						len(trackedDirty),
					),
					Err: fmt.Errorf("niezatwierdzone zmiany"),
				}
			}
		}
		if env, ok := m.cfg.Environments[envName]; ok && env.Protected {
			return ConfirmDeployMsg{Env: envName}
		}
		return DeployStartMsg{Env: envName}
	}
}

// cmdInitRollback inicjuje procedurę rollbacku.
// Ładuje historię wdrożeń dla środowiska i:
//   - brak jakichkolwiek wdrożeń → ErrorMsg (pierwsze wdrożenie, rollback niemożliwy)
//   - 1+ udanych wdrożeń → ShowRollbackPickMsg (ekran wyboru)
func (m *RootModel) cmdInitRollback(envName string) tea.Cmd {
	return func() tea.Msg {
		// ── Walidacja stanu ───────────────────────────────────────────────────
		if envName == "" {
			return ErrorMsg{
				Title:   "Brak środowiska",
				Message: "Wybierz środowisko na Dashboardzie (↑↓), a następnie naciśnij R.",
			}
		}

		if m.stateData == nil {
			return ErrorMsg{
				Title: "Brak historii wdrożeń",
				Message: "System nie znalazł pliku historii (.rnr/snapshots/state.json).\n\n" +
					"Rollback jest możliwy dopiero po pierwszym wdrożeniu.\n" +
					"Wykonaj wdrożenie klawiszem D i spróbuj ponownie.",
			}
		}

		// ── Pobierz wszystkie udane wdrożenia dla środowiska ─────────────────
		// Maksymalnie 20 ostatnich — więcej nie ma sensu wyświetlać.
		allRecords := m.stateData.GetLastN(envName, 20)
		var successful []state.DeployRecord
		for _, r := range allRecords {
			if r.Status == state.StatusSuccess || r.Status == state.StatusRolledBack {
				successful = append(successful, r)
			}
		}

		// ── Blokada: brak udanych wdrożeń = rollback niemożliwy ──────────────
		if len(successful) == 0 {
			// Sprawdź czy w ogóle było jakieś wdrożenie (może nieudane)
			allAny := m.stateData.GetLastN(envName, 5)
			if len(allAny) == 0 {
				return ErrorMsg{
					Title: "Rollback niemożliwy — pierwsze wdrożenie",
					Message: fmt.Sprintf(
						"Środowisko '%s' nie ma jeszcze żadnych wdrożeń.\n\n"+
							"Rollback wymaga przynajmniej jednego zakończonego sukcesem\n"+
							"wdrożenia, do którego można przywrócić kod.\n\n"+
							"Uruchom pierwsze wdrożenie klawiszem D.",
						envName,
					),
				}
			}
			return ErrorMsg{
				Title: "Rollback niemożliwy — brak udanych wdrożeń",
				Message: fmt.Sprintf(
					"Środowisko '%s' nie ma żadnego wdrożenia zakończonego sukcesem.\n\n"+
						"Wszystkie poprzednie wdrożenia zakończyły się błędem —\n"+
						"nie ma do czego przywracać kodu.\n\n"+
						"Napraw błędy i wykonaj pomyślne wdrożenie, aby rollback\n"+
						"stał się dostępny.",
					envName,
				),
			}
		}

		// ── Otwórz ekran wyboru wdrożenia ────────────────────────────────────
		return ShowRollbackPickMsg{
			Env:     envName,
			Records: successful,
		}
	}
}

func (m *RootModel) cmdInitPromote() tea.Cmd {
	return func() tea.Msg {
		envs := m.cfg.Environments

		// Pomocnik sprawdzający czy środowisko ma skonfigurowany provider DB.
		hasDB := func(name string) bool {
			env, ok := envs[name]
			if !ok {
				return false
			}
			p := env.Database.Provider
			return p != "" && p != config.DBProviderNone
		}

		// Zbierz środowiska z bazą danych — promote ma sens TYLKO dla środowisk z DB.
		var dbEnvs []string
		for name := range envs {
			if hasDB(name) {
				dbEnvs = append(dbEnvs, name)
			}
		}

		// Blokuj jeśli mniej niż 2 środowiska mają skonfigurowaną bazę danych.
		if len(dbEnvs) < 2 {
			msg := "Promote (podmiana baz danych) wymaga co najmniej dwóch środowisk\n" +
				"z skonfigurowanym dostawcą bazy danych.\n\n"
			if len(dbEnvs) == 0 {
				msg += "Żadne środowisko nie ma skonfigurowanej bazy danych.\n\n"
			} else {
				msg += fmt.Sprintf("Tylko jedno środowisko ma bazę: '%s'.\n\n", dbEnvs[0])
			}
			msg += "Aby skonfigurować bazę danych, ustaw w rnr.yaml:\n" +
				"  environments:\n" +
				"    staging:\n" +
				"      database:\n" +
				"        provider: supabase  # lub: prisma | postgres | mysql\n\n" +
				"Sekrety (URL, klucze) uzupełnij w rnr.conf.yaml."
			return ErrorMsg{
				Title:   "Brak baz danych do promote",
				Message: msg,
			}
		}

		// Znajdź środowisko źródłowe i docelowe spośród TYCH Z BAZĄ DANYCH.
		var sourceEnv, targetEnv string

		// 1. Preferuj dokładne, typowe nazwy (tylko jeśli mają DB)
		// "development" jest priorytetem jako środowisko źródłowe (nowa konwencja)
		for _, name := range []string{"development", "dev", "staging", "stage"} {
			if hasDB(name) && sourceEnv == "" {
				sourceEnv = name
			}
		}
		for _, name := range []string{"production", "prod", "main", "live"} {
			if hasDB(name) && targetEnv == "" {
				targetEnv = name
			}
		}

		// 2. Fallback: protected = target, non-protected = source (oba muszą mieć DB)
		if targetEnv == "" || sourceEnv == "" {
			for name, env := range envs {
				if !hasDB(name) {
					continue
				}
				if env.Protected && targetEnv == "" && name != sourceEnv {
					targetEnv = name
				} else if !env.Protected && sourceEnv == "" && name != targetEnv {
					sourceEnv = name
				}
			}
		}

		// 3. Ostateczny fallback: pierwsze dwa środowiska z DB (bez preferencji nazwy)
		if targetEnv == "" || sourceEnv == "" {
			for _, name := range dbEnvs {
				if sourceEnv == "" {
					sourceEnv = name
				} else if targetEnv == "" && name != sourceEnv {
					targetEnv = name
				}
			}
		}

		if sourceEnv == "" || targetEnv == "" || sourceEnv == targetEnv {
			return ErrorMsg{
				Title:   "Nie można ustalić środowisk promote",
				Message: fmt.Sprintf(
					"Znaleziono środowiska z DB: %v\n\n"+
						"Nie udało się automatycznie ustalić środowiska źródłowego i docelowego.\n"+
						"Upewnij się, że masz co najmniej dwa różne środowiska z bazą danych.",
					dbEnvs),
			}
		}

		// Sprawdź czy oba środowiska mają skonfigurowane sekrety bazy danych.
		sourceCfg := envs[sourceEnv]
		targetCfg := envs[targetEnv]
		sourceHasSecrets := sourceCfg.Database.SupabaseProjectRef != "" ||
			sourceCfg.Database.DBURL != "" ||
			sourceCfg.Database.DBMigrateCmd != ""
		targetHasSecrets := targetCfg.Database.SupabaseProjectRef != "" ||
			targetCfg.Database.DBURL != "" ||
			targetCfg.Database.DBMigrateCmd != ""

		var missingSecrets []string
		if !sourceHasSecrets {
			missingSecrets = append(missingSecrets, fmt.Sprintf("'%s' — brak credentials bazy", sourceEnv))
		}
		if !targetHasSecrets {
			missingSecrets = append(missingSecrets, fmt.Sprintf("'%s' — brak credentials bazy", targetEnv))
		}
		if len(missingSecrets) > 0 {
			return ErrorMsg{
				Title: "Brak credentials bazy danych",
				Message: fmt.Sprintf(
					"Promote: %s → %s\n\n"+
						"Brakujące credentials w rnr.conf.yaml:\n  • %s\n\n"+
						"Uzupełnij sekcje environments.<nazwa>.database w rnr.conf.yaml.",
					sourceEnv, targetEnv,
					strings.Join(missingSecrets, "\n  • ")),
			}
		}

		return PromoteStartMsg{SourceEnv: sourceEnv, TargetEnv: targetEnv}
	}
}

func (m *RootModel) cmdOpenLogs() tea.Cmd {
	return func() tea.Msg {
		logsDir := filepath.Join(m.projectRoot, config.LogsDir)
		m.logs = NewLogsModel(m.width, m.height, logsDir)
		m.screen = ScreenLogs
		return nil
	}
}

// cmdOpenGitPanel otwiera Git Panel i ładuje dane (gałęzie + historia + graf).
func (m *RootModel) cmdOpenGitPanel() tea.Cmd {
	m.gitPanel = NewGitPanelModel(m.width, m.height)
	m.gitPanel.gitStatus = m.gitStatus
	m.screen = ScreenGit
	return tea.Batch(
		m.cmdGitLoadBranches(),
		m.cmdGitLoadHistory(),
		m.cmdGitLoadGraph(), // Wczytaj wizualny graf commitów
	)
}

// cmdAuditGitSilent odświeża status Git bez zmiany ekranu/loadingu.
// Używany przez polling i po operacjach git.
func (m *RootModel) cmdAuditGitSilent() tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		result, err := gitops.AuditRepo(root)
		return GitStatusMsg{Result: result, Err: err}
	}
}

// cmdGitLoadBranches pobiera listę lokalnych gałęzi dla Git Panelu.
func (m *RootModel) cmdGitLoadBranches() tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		branches, err := gitops.GetLocalBranches(root)
		return GitBranchesLoadedMsg{Branches: branches, Err: err}
	}
}

// cmdGitLoadHistory pobiera historię ostatnich 30 commitów.
func (m *RootModel) cmdGitLoadHistory() tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		commits, err := gitops.GetCommitHistory(root, 30)
		return GitHistoryLoadedMsg{Commits: commits, Err: err}
	}
}

// cmdGitCheckout wykonuje git checkout na podaną gałąź.
func (m *RootModel) cmdGitCheckout(branch string) tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		err := gitops.CheckoutBranch(root, branch)
		return GitCheckoutDoneMsg{Branch: branch, Err: err}
	}
}

// cmdGitStageAndCommit wykonuje git add (wybrane pliki lub -A) && git commit -m "message".
// Jeśli files jest puste — stage all (git add -A).
// Jeśli files zawiera ścieżki — stage tylko je (git add -- plik1 plik2...).
func (m *RootModel) cmdGitStageAndCommit(message string, files []string) tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		if err := gitops.StageFiles(root, files); err != nil {
			return GitCommitDoneMsg{Err: fmt.Errorf("git add: %w", err)}
		}
		hash, err := gitops.CommitWithMessage(root, message)
		return GitCommitDoneMsg{Hash: hash, Err: err}
	}
}

// cmdGitPush wypycha bieżącą gałąź do origin (z --set-upstream przy pierwszym razie).
// Wykrywa błąd non-fast-forward i ustawia IsNonFastForward=true w odpowiedzi.
func (m *RootModel) cmdGitPush() tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		// Sprawdź czy jest skonfigurowany remote
		if !gitops.HasRemote(root) {
			return GitPushDoneMsg{
				Err: fmt.Errorf(
					"brak remote 'origin'\n\n" +
						"Dodaj remote:\n  git remote add origin <URL_REPO>\n\n" +
						"Lub w wizardzie setup: rnr init"),
			}
		}
		branch, err := gitops.GetCurrentBranch(root)
		if err != nil {
			return GitPushDoneMsg{Err: fmt.Errorf("nie można odczytać gałęzi: %w", err)}
		}
		_, pushErr := gitops.PushCurrentBranch(root)
		if pushErr != nil {
			return GitPushDoneMsg{
				Branch:           branch,
				Err:              pushErr,
				IsNonFastForward: gitops.IsNonFastForward(pushErr),
			}
		}
		return GitPushDoneMsg{Branch: branch}
	}
}

// cmdGitPullRebasePush wykonuje git pull --rebase, a następnie git push.
// Używane po odrzuceniu push z powodu non-fast-forward.
func (m *RootModel) cmdGitPullRebasePush(branch string) tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		// Krok 1: pull --rebase
		if _, err := gitops.PullRebase(root, "origin", branch); err != nil {
			return GitPullRebasePushDoneMsg{
				Branch: branch,
				Err:    fmt.Errorf("git pull --rebase nieudany: %w\n\nRozwiąż konflikty ręcznie i spróbuj ponownie", err),
			}
		}
		// Krok 2: push po rebase
		if _, err := gitops.PushCurrentBranch(root); err != nil {
			return GitPullRebasePushDoneMsg{
				Branch: branch,
				Err:    fmt.Errorf("push po rebase nieudany: %w", err),
			}
		}
		return GitPullRebasePushDoneMsg{Branch: branch}
	}
}

// cmdGitForcePushWithLease wykonuje git push --force-with-lease.
// Bezpieczniejszy od --force — odrzuca jeśli remote zmienił się od fetch.
func (m *RootModel) cmdGitForcePushWithLease(branch string) tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		if _, err := gitops.PushForceWithLease(root, "origin", branch); err != nil {
			return GitPushDoneMsg{
				Branch:           branch,
				Err:              err,
				IsNonFastForward: gitops.IsNonFastForward(err),
			}
		}
		return GitPushDoneMsg{Branch: branch}
	}
}

// cmdGitLoadGraph pobiera linie wizualnego grafu commitów (git log --graph --all).
func (m *RootModel) cmdGitLoadGraph() tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		lines, err := gitops.GetGraphLog(root, 80)
		return GitGraphLoadedMsg{Lines: lines, Err: err}
	}
}

// cmdGitLoadDiff pobiera diff dla podanego pliku.
func (m *RootModel) cmdGitLoadDiff(file string) tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		lines, err := gitops.GetFileDiff(root, file)
		return GitDiffLoadedMsg{File: file, Lines: lines, Err: err}
	}
}

// cmdStartDeploy przygotowuje model deployu i uruchamia potok.
func (m *RootModel) cmdStartDeploy(envName string) tea.Cmd {
	userStages := m.cfg.GetStagesForEnv(envName)
	m.deployID = uuid.New().String()
	m.deployEnv = envName

	// Przygotuj pełną listę etapów z pre-etapami Git (checkout + pull)
	// dla projektów z repozytorium Git.
	allStages := m.buildAllStages(envName, userStages)

	m.deploy = NewDeployModel(m.width, m.height, envName, allStages, false)
	m.screen = ScreenDeploy

	// Utwórz kanał komunikacji pipeline → TUI
	ch := make(chan pipelineEventMsg, 512)
	m.pipelineCh = ch

	return tea.Batch(
		m.deploy.Init(),
		m.cmdRunPipeline(envName, allStages, ch),
		m.cmdWaitPipeline(),
	)
}

// buildAllStages wstrzykuje pre-etapy Git (checkout, pull) przed etapami
// zdefiniowanymi przez użytkownika — tylko jeśli środowisko ma skonfigurowaną gałąź.
func (m *RootModel) buildAllStages(envName string, userStages []config.Stage) []config.Stage {
	env, ok := m.cfg.Environments[envName]
	if !ok || env.Branch == "" {
		return userStages
	}
	// Pre-etapy Git są pomijane jeśli to nie jest repozytorium Git
	if m.gitStatus != nil && !m.gitStatus.IsGitRepo {
		return userStages
	}

	gitStages := []config.Stage{
		{
			Name: fmt.Sprintf("git checkout %s", env.Branch),
			Type: config.StageTypeGit,
			Run:  "checkout:" + env.Branch,
		},
		{
			Name: fmt.Sprintf("git pull origin %s", env.Branch),
			Type: config.StageTypeGit,
			Run:  "pull:" + env.Branch,
		},
	}
	return append(gitStages, userStages...)
}

// cmdWaitPipeline czeka na kolejną wiadomość z kanału pipeline.
func (m *RootModel) cmdWaitPipeline() tea.Cmd {
	ch := m.pipelineCh
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			m.pipelineCh = nil
			return nil
		}
		return msg
	}
}

// cmdRunPipeline uruchamia cały potok w tle, wysyłając zdarzenia przez kanał.
func (m *RootModel) cmdRunPipeline(envName string, stages []config.Stage, ch chan<- pipelineEventMsg) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		deployID := m.deployID

		// Krok 1: Snapshot
		snap, err := gitops.CreateSnapshot(m.projectRoot, envName)
		if err != nil {
			ch <- pipelineEventMsg{kind: "fail", deployID: deployID,
				name: "snapshot", err: fmt.Errorf("snapshot: %w", err)}
			close(ch)
			return nil
		}
		ch <- pipelineEventMsg{kind: "snapshot", name: snap.Branch, line: snap.CommitHash}

		// Zapisz rekord do state
		stateFile := filepath.Join(m.projectRoot, config.StateFile)
		now := time.Now()
		var commitHash, commitMsg, commitAuthor string
		if m.gitStatus != nil {
			commitHash = m.gitStatus.LastCommit.Hash
			commitMsg = m.gitStatus.LastCommit.Message
			commitAuthor = m.gitStatus.LastCommit.Author
		}
		if m.stateData != nil {
			record := state.DeployRecord{
				ID:            deployID,
				Env:           envName,
				Branch:        snap.Branch,
				CommitHash:    commitHash,
				CommitMessage: commitMsg,
				CommitAuthor:  commitAuthor,
				Snapshot: state.SnapshotInfo{
					Branch:     snap.Branch,
					Tag:        snap.Tag,
					CommitHash: snap.CommitHash,
					CreatedAt:  now,
				},
				StartedAt: now,
				Status:    state.StatusRunning,
			}
			m.stateData.AddDeployment(record)
			_ = state.Save(stateFile, m.stateData)
		}

		// Krok 2: Masker + Logger
		masker := logger.NewMasker(m.cfg.AllSecrets()...)
		log, _ := logger.NewForDeployment(
			filepath.Join(m.projectRoot, config.LogsDir),
			envName, masker,
		)
		logFile := ""
		if log != nil {
			logFile = log.FilePath()
			defer log.Close()
		}

		// Panic recovery — zapisz crash do logu i zakończ pipeline z błędem
		defer func() {
			if r := recover(); r != nil {
				panicErr := fmt.Errorf("PANIC w pipeline: %v", r)
				if log != nil {
					log.Error("══════════════════════════════════════════")
					log.Error("KRYTYCZNY BŁĄD (panic): %v", r)
					log.Error("══════════════════════════════════════════")
				}
				ch <- pipelineEventMsg{kind: "fail", deployID: deployID,
					name: "PANIC", err: panicErr}
				close(ch)
			}
		}()

		envCfg := m.cfg.Environments[envName]

		// Nagłówek logu wdrożenia
		if log != nil {
			log.Info("══════════════════════════════════════════════════════")
			log.Info("rnr deploy: środowisko=%s gałąź=%s etapów=%d",
				envName, snap.Branch, len(stages))
			log.Info("commit: %s %s", snap.CommitHash, commitMsg)
			log.Info("══════════════════════════════════════════════════════")
		}

		// Krok 3: Wykonaj etapy
		for i, stage := range stages {
			ch <- pipelineEventMsg{kind: "stage_start", index: i, name: stage.Name}
			startTime := time.Now()

			// Loguj start etapu
			if log != nil {
				log.Info("┌─ [%d/%d] %s", i+1, len(stages), stage.Name)
			}

			// Kanał wyjścia etapu — każda linia trafia do TUI i do pliku logu.
			// WaitGroup gwarantuje, że goroutyna zakończy wysyłanie do `ch`
			// ZANIM zamkniemy `ch` lub wyślemy kolejne zdarzenia — bez tego
			// grozi "panic: send on closed channel".
			outputCh := make(chan string, 256)
			var outputWg sync.WaitGroup
			outputWg.Add(1)
			go func(idx int) {
				defer outputWg.Done()
				for line := range outputCh {
					// Zapisz każdą linię wyjścia do pliku logu
					if log != nil {
						log.Raw(line)
					}
					// Wyślij do TUI — jeśli ch jest pełny, nie blokuj
					select {
					case ch <- pipelineEventMsg{kind: "stage_output", index: idx, line: line}:
					default:
					}
				}
			}(i)

		var stageErr error

		switch stage.Type {
		case config.StageTypeGit:
			// Pre-etap Git — operacje na repozytorium przed wdrożeniem.
			// stage.Run ma format "checkout:<branch>" lub "pull:<branch>".
			parts := strings.SplitN(stage.Run, ":", 2)
			if len(parts) == 2 {
				op, branch := parts[0], parts[1]
				switch op {
				case "checkout":
					outputCh <- fmt.Sprintf("→ git checkout %s", branch)
					if err := gitops.CheckoutBranch(m.projectRoot, branch); err != nil {
						stageErr = err
					} else {
						outputCh <- fmt.Sprintf("✓ Przełączono na gałąź: %s", branch)
						// Odśwież status Git po checkout
						if status, err2 := gitops.AuditRepo(m.projectRoot); err2 == nil {
							m.gitStatus = status
						}
					}
				case "pull":
					outputCh <- fmt.Sprintf("→ git pull origin %s", branch)
					if err := gitops.PullBranch(m.projectRoot, branch); err != nil {
						// pull jest niekreytyczny — logujemy ostrzeżenie i kontynuujemy
						outputCh <- fmt.Sprintf("⚠ git pull nieudany (kontynuuję): %v", err)
						// Nie ustawiamy stageErr — pull failure nie blokuje deployu
					} else {
						outputCh <- fmt.Sprintf("✓ Pobrano aktualizacje z origin/%s", branch)
					}
				}
			}
		case config.StageTypeDeploy:
			depProv, err := providers.NewDeployProvider(envCfg, masker, log, m.projectRoot)
			if err != nil {
				stageErr = err
			} else {
				stageErr = depProv.Deploy(ctx, envCfg, outputCh)
			}
		case config.StageTypeDatabase:
			dbProv, err := providers.NewDatabaseProvider(envCfg, masker, log)
			if err != nil {
				stageErr = err
			} else {
				stageErr = dbProv.Migrate(ctx, envCfg, outputCh)
			}
		case config.StageTypeHealth:
			if envCfg.URL != "" {
				runner := providers.NewRunner(m.projectRoot, masker, log)
				res := runner.RunShell(ctx,
					fmt.Sprintf(`curl -sf --max-time 30 "%s" > /dev/null`, envCfg.URL),
					envCfg.Env, outputCh)
				stageErr = res.Error
			}
		default:
			if stage.Run != "" {
				runner := providers.NewRunner(m.projectRoot, masker, log)
				res := runner.RunShell(ctx, stage.Run, envCfg.Env, outputCh)
				stageErr = res.Error
			}
		}

		close(outputCh)
		outputWg.Wait() // Poczekaj aż goroutyna wyśle wszystkie linie do `ch`
		durMS := time.Since(startTime).Milliseconds()

			if stageErr != nil {
				if log != nil {
					log.Error("└─ [%d/%d] FAIL: %s (%s): %v",
						i+1, len(stages), stage.Name, formatDuration(durMS), stageErr)
				}
				if stage.AllowFailure {
					ch <- pipelineEventMsg{kind: "stage_fail", index: i, name: stage.Name,
						durationMS: durMS, err: stageErr, allowFail: true}
				} else {
					ch <- pipelineEventMsg{kind: "stage_fail", index: i, name: stage.Name,
						durationMS: durMS, err: stageErr, allowFail: false}
					// Loguj ogólne niepowodzenie deployu
					if log != nil {
						log.Error("══════════════════════════════════════════════════════")
						log.Error("DEPLOY NIEUDANY — etap: %s", stage.Name)
						log.Error("══════════════════════════════════════════════════════")
					}
					ch <- pipelineEventMsg{kind: "fail", deployID: deployID,
						name: stage.Name, err: stageErr}
					close(ch)
					return nil
				}
			} else {
				if log != nil {
					log.Success("└─ [%d/%d] OK: %s (%s)", i+1, len(stages), stage.Name, formatDuration(durMS))
				}
				ch <- pipelineEventMsg{kind: "stage_done", index: i, name: stage.Name, durationMS: durMS}
			}
		}

		// Podsumowanie w logu
		total := time.Since(now)
		if log != nil {
			log.Success("══════════════════════════════════════════════════════")
			log.Success("DEPLOY ZAKOŃCZONY SUKCESEM — środowisko: %s, czas: %s",
				envName, total.Round(time.Millisecond).String())
			log.Success("══════════════════════════════════════════════════════")
		}

		ch <- pipelineEventMsg{kind: "done", deployID: deployID, logFile: logFile}
		close(ch)
		return nil
	}
}

func (m *RootModel) cmdMarkDeploySuccess(deployID string) tea.Cmd {
	return func() tea.Msg {
		if m.stateData == nil {
			return nil
		}
		stateFile := filepath.Join(m.projectRoot, config.StateFile)
		record := m.stateData.GetByID(deployID)
		if record != nil {
			record.Status = state.StatusSuccess
			record.CompletedAt = time.Now()
			m.stateData.UpdateDeployment(*record)
			_ = state.Save(stateFile, m.stateData)
		}
		return nil
	}
}

func (m *RootModel) cmdMarkDeployFailed(deployID string) tea.Cmd {
	return func() tea.Msg {
		if m.stateData == nil {
			return nil
		}
		stateFile := filepath.Join(m.projectRoot, config.StateFile)
		record := m.stateData.GetByID(deployID)
		if record != nil {
			record.Status = state.StatusFailed
			record.CompletedAt = time.Now()
			m.stateData.UpdateDeployment(*record)
			_ = state.Save(stateFile, m.stateData)
		}
		return nil
	}
}

// cmdExecuteRollback wykonuje rollback do wybranego wdrożenia.
// Przywraca kod (git reset/checkout) do commita wybranego przez użytkownika
// na ekranie ScreenRollbackPick, a następnie uruchamia ponowne wdrożenie
// (redeploy) przywróconej wersji.
func (m *RootModel) cmdExecuteRollback(msg RollbackStartMsg) tea.Cmd {
	stages := m.cfg.GetStagesForEnv(msg.Env)
	label := fmt.Sprintf("↩️  rollback → %s", func() string {
		if msg.Description != "" {
			return msg.Description
		}
		if msg.CommitHash != "" && len(msg.CommitHash) >= 7 {
			return msg.CommitHash[:7]
		}
		return msg.Env
	}())
	// Rollback stages: najpierw git reset do wybranego commita, potem redeploy
	rollbackStages := []config.Stage{
		{
			Name:        "git reset — przywracanie kodu",
			Type:        config.StageTypeGit,
			Run:         "checkout:" + msg.Branch,
			Description: "Przywraca kod do stanu z wybranego wdrożenia",
		},
		{
			Name:        "redeploy — wdrożenie przywróconej wersji",
			Type:        config.StageTypeDeploy,
			Description: "Wdraża przywróconą wersję kodu na serwer",
		},
	}
	_ = label
	_ = stages

	m.rollbackEnv = msg.Env
	m.deployEnv = msg.Env
	m.deploy = NewDeployModel(m.width, m.height, msg.Env, rollbackStages, true)
	m.screen = ScreenRollback

	return func() tea.Msg {
		ctx := context.Background()

		envCfg, ok := m.cfg.Environments[msg.Env]
		if !ok {
			return RollbackFailedMsg{Err: fmt.Errorf("środowisko %q nie istnieje", msg.Env)}
		}

		masker := logger.NewMasker(m.cfg.AllSecrets()...)
		log, _ := logger.NewForDeployment(
			filepath.Join(m.projectRoot, config.LogsDir),
			"rollback_"+msg.Env, masker,
		)
		if log != nil {
			defer log.Close()
			log.Info("══════════════════════════════════════════════════════")
			log.Info("↩️  Rollback: %s → commit %s", msg.Env, func() string {
				if len(msg.CommitHash) >= 7 {
					return msg.CommitHash[:7]
				}
				return msg.CommitHash
			}())
			if msg.Description != "" {
				log.Info("  Wdrożenie: %s", msg.Description)
			}
			log.Info("══════════════════════════════════════════════════════")
		}

		// ── Krok 1: Przywróć kod git do wybranego commita ────────────────────
		if msg.CommitHash != "" {
			if log != nil {
				log.Info("git reset --hard %s", msg.CommitHash)
			}
			target := gitops.BuildRollbackTarget(msg.CommitHash, "", "", msg.Env)
			if err := gitops.RestoreSnapshot(m.projectRoot, target); err != nil {
				if log != nil {
					log.Error("git reset nieudany: %v", err)
				}
				return RollbackFailedMsg{Err: fmt.Errorf("przywracanie kodu nieudane: %w", err)}
			}
			if log != nil {
				log.Success("git reset OK — kod przywrócony do %s", msg.CommitHash[:min(7, len(msg.CommitHash))])
			}
		} else {
			if log != nil {
				log.Warn("Brak hash commita — pomijam git reset, redeployuję bieżący stan kodu")
			}
		}

		// ── Krok 2: Redeploy przywróconej wersji ─────────────────────────────
		if log != nil {
			log.Info("Redeploy środowisko %s...", msg.Env)
		}

		// Kanał wyjścia z WaitGroup (nie panic na closed channel)
		outputCh := make(chan string, 256)
		var outputWg sync.WaitGroup
		outputWg.Add(1)
		go func() {
			defer outputWg.Done()
			for line := range outputCh {
				if log != nil {
					log.Raw(line)
				}
			}
		}()

		depProv, err := providers.NewDeployProvider(envCfg, masker, log, m.projectRoot)
		if err != nil {
			close(outputCh)
			outputWg.Wait()
			return RollbackFailedMsg{Err: err}
		}

		deployErr := depProv.Rollback(ctx, envCfg, outputCh)
		close(outputCh)
		outputWg.Wait()

		if deployErr != nil {
			if log != nil {
				log.Error("Redeploy nieudany: %v", deployErr)
			}
			return RollbackFailedMsg{Err: fmt.Errorf("redeploy nieudany: %w", deployErr)}
		}

		if log != nil {
			log.Success("Rollback zakończony sukcesem")
		}
		return RollbackCompletedMsg{Env: msg.Env}
	}
}

// cmdExecutePromote uruchamia promote bazy danych przez pipeline,
// tak jak cmdStartDeploy — z pełnym live output w TUI i paskami postępu.
// Używa pipelineEventMsg zamiast bezpośrednich Promote*Msg, dzięki czemu
// użytkownik widzi ten sam widok co przy zwykłym wdrożeniu.
func (m *RootModel) cmdExecutePromote(msg PromoteStartMsg) tea.Cmd {
	label := fmt.Sprintf("🗄️  promote DB: %s → %s", msg.SourceEnv, msg.TargetEnv)

	// Utwórz jeden etap reprezentujący promote
	stages := []config.Stage{
		{
			Name:        label,
			Type:        config.StageTypeDatabase,
			Description: fmt.Sprintf("Migracje DB ze %s na %s (roll-forward)", msg.SourceEnv, msg.TargetEnv),
		},
	}

	// Inicjalizuj deploy model — taki sam ekran jak przy wdrożeniu
	m.deployID = uuid.New().String()
	m.deployEnv = "promote"
	m.deploy = NewDeployModel(m.width, m.height,
		fmt.Sprintf("Promote: %s → %s", msg.SourceEnv, msg.TargetEnv),
		stages, false)
	m.screen = ScreenPromote

	// Kanał pipeline (taki sam mechanizm jak w cmdRunPipeline)
	ch := make(chan pipelineEventMsg, 256)
	m.pipelineCh = ch

	// Przechwytujemy wartości teraz (przed asynchronicznym wykonaniem)
	deployID := m.deployID
	sourceEnvCfg := m.cfg.Environments[msg.SourceEnv]
	targetEnvCfg := m.cfg.Environments[msg.TargetEnv]
	maskerSecrets := m.cfg.AllSecrets()
	logsDir := filepath.Join(m.projectRoot, config.LogsDir)
	stageLabel := label

	promoteCmd := func() tea.Msg {
		ctx := context.Background()

		masker := logger.NewMasker(maskerSecrets...)
		log, _ := logger.NewForDeployment(logsDir,
			"promote_"+msg.TargetEnv, masker)
		if log != nil {
			defer log.Close()
		}

		// Wyślij start etapu
		ch <- pipelineEventMsg{kind: "stage_start", index: 0, name: stageLabel}
		startTime := time.Now()

		// Kanał output → pipeline channel (live logi w TUI)
		outputCh := make(chan string, 128)
		go func() {
			for line := range outputCh {
				ch <- pipelineEventMsg{kind: "stage_output", index: 0, line: line}
			}
		}()

		// Utwórz dostawcę DB dla środowiska docelowego
		dbProv, err := providers.NewDatabaseProvider(targetEnvCfg, masker, log)
		if err != nil {
			close(outputCh)
			durMS := time.Since(startTime).Milliseconds()
			ch <- pipelineEventMsg{kind: "stage_fail", index: 0, name: stageLabel,
				durationMS: durMS, err: err}
			ch <- pipelineEventMsg{kind: "fail", deployID: deployID, name: stageLabel, err: err}
			close(ch)
			return nil
		}

		// Wykonaj promote (blokuje do zakończenia)
		err = dbProv.Promote(ctx, sourceEnvCfg, targetEnvCfg, outputCh)
		close(outputCh) // Zatrzymaj goroutinę forwarding
		durMS := time.Since(startTime).Milliseconds()

		if err != nil {
			ch <- pipelineEventMsg{kind: "stage_fail", index: 0, name: stageLabel,
				durationMS: durMS, err: err}
			ch <- pipelineEventMsg{kind: "fail", deployID: deployID, name: stageLabel, err: err}
			close(ch)
			return nil
		}

		// Sukces
		ch <- pipelineEventMsg{kind: "stage_done", index: 0, name: stageLabel, durationMS: durMS}
		ch <- pipelineEventMsg{kind: "done", deployID: deployID, logFile: ""}
		close(ch)
		return nil
	}

	return tea.Batch(m.deploy.Init(), m.cmdWaitPipeline(), promoteCmd)
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func (m *RootModel) showError(title, msg string, err error) {
	m.errTitle = title
	m.errMsg = msg
	if err != nil && msg == "" {
		m.errMsg = err.Error()
	}
	m.screen = ScreenError
}
