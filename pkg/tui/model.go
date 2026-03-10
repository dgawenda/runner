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
	"path/filepath"
	"strings"
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
	ScreenLoading   Screen = iota // Ładowanie konfiguracji
	ScreenWizard                  // Setup Wizard (pierwsze uruchomienie)
	ScreenDashboard               // Główny Dashboard
	ScreenConfirm                 // Potwierdzenie wdrożenia (środowiska chronione)
	ScreenDeploy                  // Ekran wdrożenia z progress bars
	ScreenGit                     // Git Panel — status, gałęzie, historia, commit
	ScreenRollback                // Ekran rollbacku
	ScreenPromote                 // Ekran promote (migracje DB)
	ScreenLogs                    // Przeglądarka logów
	ScreenError                   // Ekran błędu
	ScreenEnvAdd                  // Kreator dodawania nowego środowiska
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

	// Rollback
	rollbackEnv string

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
		// Żądanie commit z GitPanel → stage all + commit
		cmds = append(cmds, m.cmdGitStageAndCommit(msg.Message))

	case GitCheckoutDoneMsg:
		// Wynik checkout → przekaż do gitPanel + odśwież git status + gałęzie
		var gpCmd tea.Cmd
		m.gitPanel, gpCmd = m.gitPanel.Update(msg)
		cmds = append(cmds, gpCmd)
		cmds = append(cmds, m.cmdAuditGitSilent(), m.cmdGitLoadBranches())

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
		cmds = append(cmds, m.cmdGenerateConfigs(msg))

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

	// ── Rollback ─────────────────────────────────────────────────────
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
		// Klawiszami G/Q/ESC zamykamy panel — ale TYLKO gdy gitPanel nie obsługuje ich sam.
		// ESC w gitPanel: zamyka diff lub unfocusuje input (nie zamykamy panelu).
		// Q w gitPanel z focusem inputa: wpisuje 'q' do inputa (nie zamykamy panelu).
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "g", "G":
				m.screen = ScreenDashboard
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
	return func() tea.Msg {
		if err := config.EnsureRnrDir(m.projectRoot); err != nil {
			return ErrorMsg{Title: "Błąd katalogu", Message: err.Error(), Err: err}
		}

		// rnr.yaml — generuj tylko jeśli nie istnieje (w nowej strukturze: projekt + środowiska + stages)
		pipelinePath := filepath.Join(m.projectRoot, config.PipelineFile)
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
				return ErrorMsg{Title: "Błąd zapisu rnr.yaml", Message: err.Error(), Err: err}
			}
		}

		// rnr.conf.yaml — TYLKO sekrety (zawsze nadpisz jeśli wizard go generuje)
		confPath := filepath.Join(m.projectRoot, config.ConfFile)
		confContent := config.DefaultConfYAMLFromWizard(
			w.ProjectName, w.Repo,
			w.DeployProv, w.NetlifyToken, w.NetlifySiteID, w.NetlifyCreateNew,
			w.DBProv, w.SupabaseRef, w.SupabaseURL, w.SupabaseKey,
		)
		if err := os.WriteFile(confPath, []byte(confContent), 0o600); err != nil {
			return ErrorMsg{Title: "Błąd zapisu rnr.conf.yaml", Message: err.Error(), Err: err}
		}

		_ = config.EnsureGitignore(m.projectRoot)
		return NavigateMsg{Screen: ScreenDashboard}
	}
}

func (m *RootModel) cmdInitDeploy(envName string) tea.Cmd {
	return func() tea.Msg {
		// Sprawdź czystość repo tylko dla projektów Git z niezatwierdzonymi zmianami.
		// rnr wykona git checkout na odpowiednią gałąź — niezatwierdzone zmiany
		// mogą być nadpisane lub powodować konflikty. Blokujemy przed deploy.
		if m.gitStatus != nil && m.gitStatus.IsGitRepo && !m.gitStatus.IsClean {
			return ErrorMsg{
				Title: "Niezatwierdzone zmiany",
				Message: fmt.Sprintf(
					"Znaleziono %d niezatwierdzonych plików.\n\n"+
						"rnr wykona git checkout na gałąź środowiska — niezatwierdzone\n"+
						"zmiany mogą ulec utracie lub powodować konflikty.\n\n"+
						"Zatwierdź zmiany przed wdrożeniem:\n"+
						"  git add . && git commit -m 'opis'\n\n"+
						"lub odłóż na bok:\n"+
						"  git stash",
					len(m.gitStatus.DirtyFiles),
				),
				Err: fmt.Errorf("niezatwierdzone zmiany"),
			}
		}
		if env, ok := m.cfg.Environments[envName]; ok && env.Protected {
			return ConfirmDeployMsg{Env: envName}
		}
		return DeployStartMsg{Env: envName}
	}
}

func (m *RootModel) cmdInitRollback(envName string) tea.Cmd {
	return func() tea.Msg {
		if m.stateData == nil {
			return ErrorMsg{Title: "Brak historii", Message: "Nie znaleziono historii wdrożeń."}
		}
		last := m.stateData.GetLastSuccessful(envName)
		if last == nil {
			return ErrorMsg{
				Title:   "Brak snapshotu",
				Message: fmt.Sprintf("Brak udanego wdrożenia dla '%s'.", envName),
			}
		}
		return RollbackStartMsg{
			Env:        envName,
			DeployID:   last.ID,
			CommitHash: last.Snapshot.CommitHash,
			Branch:     last.Snapshot.Branch,
		}
	}
}

func (m *RootModel) cmdInitPromote() tea.Cmd {
	return func() tea.Msg {
		envs := m.cfg.Environments

		// Znajdź środowisko źródłowe (staging) i docelowe (production).
		// Priorytety: dokładne nazwy > protected flag > pierwsze środowisko z bazą DB.
		var sourceEnv, targetEnv string

		// 1. Preferuj dokładne, typowe nazwy
		for _, name := range []string{"staging", "stage", "develop", "dev"} {
			if _, ok := envs[name]; ok && sourceEnv == "" {
				sourceEnv = name
			}
		}
		for _, name := range []string{"production", "prod", "main", "live"} {
			if _, ok := envs[name]; ok && targetEnv == "" {
				targetEnv = name
			}
		}

		// 2. Fallback: protected = target, first non-protected with DB = source
		if targetEnv == "" || sourceEnv == "" {
			for name, env := range envs {
				hasDB := env.Database.Provider != "" && env.Database.Provider != "none"
				if env.Protected && hasDB && targetEnv == "" {
					targetEnv = name
				} else if !env.Protected && hasDB && sourceEnv == "" {
					sourceEnv = name
				}
			}
		}

		// 3. Finalny fallback: dowolne dwa środowiska jeśli mamy tylko jedno z DB
		if targetEnv == "" || sourceEnv == "" {
			for name := range envs {
				if name != sourceEnv && targetEnv == "" {
					targetEnv = name
				} else if name != targetEnv && sourceEnv == "" {
					sourceEnv = name
				}
			}
		}

		if sourceEnv == "" || targetEnv == "" {
			return ErrorMsg{
				Title:   "Brak środowisk do promote",
				Message: "Promote wymaga co najmniej dwóch środowisk.\n\n" +
					"Upewnij się, że rnr.yaml ma skonfigurowane sekcje\n" +
					"'environments' z co najmniej dwoma środowiskami.",
			}
		}
		if sourceEnv == targetEnv {
			return ErrorMsg{
				Title:   "Błędna konfiguracja",
				Message: fmt.Sprintf("Źródło i cel promote to to samo środowisko: '%s'.\n", sourceEnv),
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

// cmdGitStageAndCommit wykonuje git add -A && git commit -m "message".
func (m *RootModel) cmdGitStageAndCommit(message string) tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		if err := gitops.StageAll(root); err != nil {
			return GitCommitDoneMsg{Err: fmt.Errorf("git add -A: %w", err)}
		}
		hash, err := gitops.CommitWithMessage(root, message)
		return GitCommitDoneMsg{Hash: hash, Err: err}
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

		envCfg := m.cfg.Environments[envName]

		// Krok 3: Wykonaj etapy
		for i, stage := range stages {
			ch <- pipelineEventMsg{kind: "stage_start", index: i, name: stage.Name}
			startTime := time.Now()

			// Kanał wyjścia etapu
			outputCh := make(chan string, 128)
			go func(idx int) {
				for line := range outputCh {
					ch <- pipelineEventMsg{kind: "stage_output", index: idx, line: line}
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
			depProv, err := providers.NewDeployProvider(envCfg, masker, log)
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
			durMS := time.Since(startTime).Milliseconds()

			if stageErr != nil {
				if stage.AllowFailure {
					ch <- pipelineEventMsg{kind: "stage_fail", index: i, name: stage.Name,
						durationMS: durMS, err: stageErr, allowFail: true}
				} else {
					ch <- pipelineEventMsg{kind: "stage_fail", index: i, name: stage.Name,
						durationMS: durMS, err: stageErr, allowFail: false}
					ch <- pipelineEventMsg{kind: "fail", deployID: deployID,
						name: stage.Name, err: stageErr}
					close(ch)
					return nil
				}
			} else {
				ch <- pipelineEventMsg{kind: "stage_done", index: i, name: stage.Name, durationMS: durMS}
			}
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

func (m *RootModel) cmdExecuteRollback(msg RollbackStartMsg) tea.Cmd {
	stages := m.cfg.GetStagesForEnv(msg.Env)
	m.deploy = NewDeployModel(m.width, m.height, msg.Env, stages, true)
	m.screen = ScreenRollback

	return func() tea.Msg {
		ctx := context.Background()

		// Przywróć do snapshotu wyłącznie przez hash commita.
		// rnr NIE tworzy dodatkowych gałęzi — snapshot to commit hash, nie gałąź.
		if msg.CommitHash != "" {
			target := gitops.BuildRollbackTarget(msg.CommitHash, "", "", msg.Env)
			if err := gitops.RestoreSnapshot(m.projectRoot, target); err != nil {
				return RollbackFailedMsg{Err: err}
			}
		} else {
			// Brak snapshotu (np. projekt bez Git) — tylko redeploy bieżącego stanu
			// send(outputCh, "⚠️  Brak snapshotu git — redeployuję bieżący stan kodu...")
		}

		// Redeploy przywróconej wersji
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
		}

		outputCh := make(chan string, 128)
		go func() {
			for range outputCh {
			}
		}()
		defer close(outputCh)

		depProv, err := providers.NewDeployProvider(envCfg, masker, log)
		if err != nil {
			return RollbackFailedMsg{Err: err}
		}

		if err := depProv.Rollback(ctx, envCfg, outputCh); err != nil {
			return RollbackFailedMsg{Err: err}
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
