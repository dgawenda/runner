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
	ScreenRollback                // Ekran rollbacku
	ScreenPromote                 // Ekran promote (migracje DB)
	ScreenLogs                    // Przeglądarka logów
	ScreenError                   // Ekran błędu
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
func (m *RootModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.cmdLoadConfig(),
	)
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
			m.wizard = NewWizardModel(m.width, m.height)
			m.screen = ScreenWizard
			cmds = append(cmds, m.wizard.Init())
		} else {
			m.loading = true
			m.loadMsg = "Sprawdzam repozytorium Git..."
			cmds = append(cmds, m.cmdAuditGit(), m.spinner.Tick)
		}

	case GitStatusMsg:
		m.loading = false
		if msg.Err != nil {
			m.showError("Błąd Git", msg.Err.Error(), msg.Err)
			return m, nil
		}
		m.gitStatus = msg.Result
		m.loading = true
		m.loadMsg = "Ładowanie historii wdrożeń..."
		cmds = append(cmds, m.cmdLoadState(), m.spinner.Tick)

	case StateLoadedMsg:
		m.loading = false
		if msg.Err == nil {
			m.stateData = msg.State
		} else {
			m.stateData = &state.State{Version: 1, Deployments: []state.DeployRecord{}}
		}
		m.dashboard = NewDashboardModel(m.width, m.height, m.cfg, m.gitStatus, m.stateData)
		m.screen = ScreenDashboard

	// ── Wizard ────────────────────────────────────────────────────────
	case WizardCompleteMsg:
		cmds = append(cmds, m.cmdGenerateConfigs(msg.ProjectName, msg.Repo))

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
		m.screen = ScreenPromote
		cmds = append(cmds, m.cmdExecutePromote(msg))

	case PromoteCompletedMsg:
		m.loading = true
		m.screen = ScreenLoading
		m.loadMsg = "Promote zakończony..."
		cmds = append(cmds, m.cmdLoadConfig(), m.spinner.Tick)

	case PromoteFailedMsg:
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
	case ScreenDeploy, ScreenRollback:
		var cmd tea.Cmd
		m.deploy, cmd = m.deploy.Update(msg)
		cmds = append(cmds, cmd)
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

	case ScreenDeploy, ScreenRollback:
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
	}
	return nil
}

// ─── Komendy ─────────────────────────────────────────────────────────────

func (m *RootModel) cmdLoadConfig() tea.Cmd {
	return func() tea.Msg {
		hasPipeline, hasConf := config.Exists(m.projectRoot)
		if !hasPipeline || !hasConf {
			return ConfigLoadedMsg{Err: fmt.Errorf("brak pliku konfiguracyjnego")}
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

func (m *RootModel) cmdGenerateConfigs(projectName, repo string) tea.Cmd {
	return func() tea.Msg {
		if err := config.EnsureRnrDir(m.projectRoot); err != nil {
			return ErrorMsg{Title: "Błąd katalogu", Message: err.Error(), Err: err}
		}
		pipelinePath := filepath.Join(m.projectRoot, config.PipelineFile)
		if _, err := os.Stat(pipelinePath); os.IsNotExist(err) {
			content := config.DefaultPipelineYAML(projectName)
			if err := os.WriteFile(pipelinePath, []byte(content), 0o644); err != nil {
				return ErrorMsg{Title: "Błąd zapisu", Message: err.Error(), Err: err}
			}
		}
		confPath := filepath.Join(m.projectRoot, config.ConfFile)
		content := config.DefaultConfYAML(projectName, repo)
		if err := os.WriteFile(confPath, []byte(content), 0o600); err != nil {
			return ErrorMsg{Title: "Błąd zapisu", Message: err.Error(), Err: err}
		}
		_ = config.EnsureGitignore(m.projectRoot)
		return NavigateMsg{Screen: ScreenDashboard}
	}
}

func (m *RootModel) cmdInitDeploy(envName string) tea.Cmd {
	return func() tea.Msg {
		if m.gitStatus != nil && !m.gitStatus.IsClean {
			return ErrorMsg{
				Title: "Repozytorium nie jest czyste",
				Message: fmt.Sprintf(
					"rnr wymaga zatwierdzonego kodu przed wdrożeniem.\n\n"+
						"Znaleziono %d niezatwierdzonych zmian.\n\n"+
						"Zatwierdź zmiany:\n"+
						"  git add . && git commit -m 'opis'\n\n"+
						"lub odłóż na bok:\n"+
						"  git stash",
					len(m.gitStatus.DirtyFiles),
				),
				Err: fmt.Errorf("brudne repozytorium"),
			}
		}
		if env, ok := m.cfg.Conf.Environments[envName]; ok && env.Protected {
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
		envs := m.cfg.GetEnvironmentNames()
		hasStaging, hasProd := false, false
		for _, e := range envs {
			switch e {
			case "staging":
				hasStaging = true
			case "production":
				hasProd = true
			}
		}
		if !hasStaging || !hasProd {
			return ErrorMsg{
				Title:   "Brak środowisk",
				Message: "Promote wymaga środowisk 'staging' i 'production'.",
			}
		}
		return PromoteStartMsg{SourceEnv: "staging", TargetEnv: "production"}
	}
}

// cmdStartDeploy przygotowuje model deployu i uruchamia potok.
func (m *RootModel) cmdStartDeploy(envName string) tea.Cmd {
	stages := m.cfg.GetStagesForEnv(envName)
	m.deployID = uuid.New().String()
	m.deployEnv = envName
	m.deploy = NewDeployModel(m.width, m.height, envName, stages, false)
	m.screen = ScreenDeploy

	// Utwórz kanał komunikacji pipeline → TUI
	ch := make(chan pipelineEventMsg, 512)
	m.pipelineCh = ch

	return tea.Batch(
		m.deploy.Init(),
		m.cmdRunPipeline(envName, stages, ch),
		m.cmdWaitPipeline(),
	)
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

		envCfg := m.cfg.Conf.Environments[envName]

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

		if msg.Branch != "" {
			if err := gitops.RestoreFromBranch(m.projectRoot, msg.Branch); err != nil {
				return RollbackFailedMsg{Err: err}
			}
		} else if msg.CommitHash != "" {
			target := gitops.BuildRollbackTarget(msg.CommitHash, "", "", msg.Env)
			if err := gitops.RestoreSnapshot(m.projectRoot, target); err != nil {
				return RollbackFailedMsg{Err: err}
			}
		} else {
			return RollbackFailedMsg{Err: fmt.Errorf("brak danych snapshotu")}
		}

		// Redeploy przywróconej wersji
		envCfg, ok := m.cfg.Conf.Environments[msg.Env]
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

func (m *RootModel) cmdExecutePromote(msg PromoteStartMsg) tea.Cmd {
	m.deploy = NewDeployModel(m.width, m.height, "promote", nil, false)

	return func() tea.Msg {
		ctx := context.Background()

		sourceEnv := m.cfg.Conf.Environments[msg.SourceEnv]
		targetEnv := m.cfg.Conf.Environments[msg.TargetEnv]

		masker := logger.NewMasker(m.cfg.AllSecrets()...)
		log, _ := logger.NewForDeployment(
			filepath.Join(m.projectRoot, config.LogsDir),
			"promote", masker,
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

		dbProv, err := providers.NewDatabaseProvider(targetEnv, masker, log)
		if err != nil {
			return PromoteFailedMsg{Err: err}
		}

		if err := dbProv.Promote(ctx, sourceEnv, targetEnv, outputCh); err != nil {
			return PromoteFailedMsg{Err: err}
		}

		return PromoteCompletedMsg{}
	}
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
