// file: pkg/tui/deploy.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Ekran Wdrożenia — Progress Bars i Spinnery                         ║
// ║                                                                      ║
// ║  Wyświetla w czasie rzeczywistym:                                   ║
// ║    · Listę etapów z ikonami statusu                                 ║
// ║    · Spinner animowany przy aktywnym etapie                         ║
// ║    · Pasek postępu całego potoku                                    ║
// ║    · Ostatnie linie wyjścia z wykonywanego procesu                  ║
// ╚══════════════════════════════════════════════════════════════════════╝

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/neution/rnr/pkg/config"
)

// ─── Typy ──────────────────────────────────────────────────────────────────

// StageState to stan etapu potoku.
type StageState int

const (
	StagePending StageState = iota
	StageRunning
	StageSuccess
	StageFailed
	StageWarning // allow_failure = true ale etap się nie powiódł
	StageSkipped
)

func (s StageState) String() string {
	switch s {
	case StagePending:
		return "pending"
	case StageRunning:
		return "running"
	case StageSuccess:
		return "success"
	case StageFailed:
		return "failed"
	case StageWarning:
		return "warning"
	case StageSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// StageEntry przechowuje stan i wyniki pojedynczego etapu.
type StageEntry struct {
	Stage      config.Stage
	State      StageState
	StartTime  time.Time
	EndTime    time.Time
	DurationMS int64
	Output     []string
	Err        error
}

// ─── Model Deployu ────────────────────────────────────────────────────────

// DeployModel obsługuje ekran wdrożenia z progress bars i spinnerami.
type DeployModel struct {
	width        int
	height       int
	env          string
	stages       []StageEntry
	currentStep  int
	spinner      spinner.Model
	progress     progress.Model
	outputLines  []string
	maxOutput    int
	isRollback   bool
	completed    bool
	failed       bool
	failedStep   string
	snapshotInfo string
	startTime    time.Time
	logFile      string
}

// NewDeployModel tworzy nowy model ekranu wdrożenia.
func NewDeployModel(width, height int, env string, stages []config.Stage, isRollback bool) DeployModel {
	// Konfiguracja spinnera
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorInfo)

	// Konfiguracja progress bar
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(min(width-20, 60)),
	)

	entries := make([]StageEntry, len(stages))
	for i, s := range stages {
		entries[i] = StageEntry{Stage: s, State: StagePending}
	}

	return DeployModel{
		width:       width,
		height:      height,
		env:         env,
		stages:      entries,
		spinner:     sp,
		progress:    prog,
		maxOutput:   8,
		isRollback:  isRollback,
		startTime:   time.Now(),
	}
}

// ─── Interfejs Bubble Tea ─────────────────────────────────────────────────

// Init inicjalizuje model deployu — uruchamia spinner.
func (m DeployModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update obsługuje zdarzenia ekranu wdrożenia.
func (m DeployModel) Update(msg tea.Msg) (DeployModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = min(m.width-20, 60)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case progress.FrameMsg:
		var cmd tea.Cmd
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		cmds = append(cmds, cmd)

	case SnapshotCreatedMsg:
		if msg.Err == nil {
			m.snapshotInfo = fmt.Sprintf("📸 Snapshot: %s [%s]", msg.Branch, msg.Hash[:7])
		}

	case StageStartedMsg:
		if msg.Index < len(m.stages) {
			m.stages[msg.Index].State = StageRunning
			m.stages[msg.Index].StartTime = time.Now()
			m.currentStep = msg.Index
		}
		cmds = append(cmds, m.spinner.Tick)

	case StageOutputMsg:
		m.appendOutput(msg.Line)

	case StageCompletedMsg:
		if msg.Index < len(m.stages) {
			m.stages[msg.Index].State = StageSuccess
			m.stages[msg.Index].DurationMS = msg.DurationMS
			m.stages[msg.Index].EndTime = time.Now()
		}
		// Aktualizuj pasek postępu
		pct := float64(m.countCompleted()) / float64(len(m.stages))
		cmds = append(cmds, m.progress.SetPercent(pct))

	case StageFailedMsg:
		if msg.Index < len(m.stages) {
			if msg.AllowFailure {
				m.stages[msg.Index].State = StageWarning
			} else {
				m.stages[msg.Index].State = StageFailed
			}
			m.stages[msg.Index].DurationMS = msg.DurationMS
			m.stages[msg.Index].EndTime = time.Now()
			m.stages[msg.Index].Err = msg.Err
			if !msg.AllowFailure {
				m.failed = true
				m.failedStep = msg.Name
			}
		}

	case StageSkippedMsg:
		if msg.Index < len(m.stages) {
			m.stages[msg.Index].State = StageSkipped
		}

	case DeployCompletedMsg:
		m.completed = true
		m.logFile = msg.LogFile
		cmds = append(cmds, m.progress.SetPercent(1.0))

	case DeployFailedMsg:
		m.failed = true
		m.failedStep = msg.StepName

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.completed || m.failed {
				return m, func() tea.Msg {
					return NavigateMsg{Screen: ScreenDashboard}
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renderuje ekran wdrożenia.
func (m DeployModel) View() string {
	contentW := min(m.width-2, 90)

	var sections []string

	// Nagłówek
	sections = append(sections, m.renderHeader(contentW))

	// Snapshot info
	if m.snapshotInfo != "" {
		sections = append(sections, "  "+StyleSuccess.Render(m.snapshotInfo)+"\n")
	}

	// Lista etapów
	sections = append(sections, m.renderStages(contentW))

	// Pasek postępu
	sections = append(sections, m.renderProgress(contentW))

	// Output
	if len(m.outputLines) > 0 {
		sections = append(sections, m.renderOutput(contentW))
	}

	// Status końcowy
	if m.completed || m.failed {
		sections = append(sections, m.renderFinalStatus(contentW))
	}

	// Podpowiedź klawiszy
	if m.completed || m.failed {
		sections = append(sections, StyleMuted.Padding(0, 2).Render("\n  ENTER / Q = powrót do Dashboard"))
	}

	return strings.Join(sections, "")
}

// ─── Sekcje Widoku ────────────────────────────────────────────────────────

func (m DeployModel) renderHeader(width int) string {
	var icon, action string
	if m.isRollback {
		icon = "↩️ "
		action = "Rollback"
	} else {
		icon = "🚀"
		action = "Wdrożenie"
	}

	title := StyleTitle.Render(fmt.Sprintf("%s  %s → %s", icon, action, m.env))
	elapsed := time.Since(m.startTime).Round(time.Second).String()
	timeInfo := StyleMuted.Render("  (" + elapsed + ")")

	return lipgloss.NewStyle().Width(width).Padding(0, 1).
		Render(title+timeInfo) + "\n" + Divider(width) + "\n"
}

func (m DeployModel) renderStages(width int) string {
	var lines []string

	for i, entry := range m.stages {
		var iconStr string
		var nameStyle lipgloss.Style
		var durationStr string

		switch entry.State {
		case StagePending:
			iconStr = StyleStagePending.Render("  ○")
			nameStyle = StyleStagePending
		case StageRunning:
			iconStr = "  " + m.spinner.View()
			nameStyle = StyleStageRunning
		case StageSuccess:
			iconStr = StyleStageSuccess.Render("  ✓")
			nameStyle = StyleStageSuccess
			if entry.DurationMS > 0 {
				durationStr = StyleMuted.Render(fmt.Sprintf("  %s", formatDuration(entry.DurationMS)))
			}
		case StageFailed:
			iconStr = StyleStageFailed.Render("  ✗")
			nameStyle = StyleStageFailed
			if entry.Err != nil {
				durationStr = StyleError.Render("  " + entry.Err.Error())
			}
		case StageWarning:
			iconStr = StyleStageWarning.Render("  ⚠")
			nameStyle = StyleStageWarning
			durationStr = StyleWarning.Render("  (allow_failure)")
		case StageSkipped:
			iconStr = StyleStageSkipped.Render("  ⊘")
			nameStyle = StyleStageSkipped
			durationStr = StyleMuted.Render("  (pominięty)")
		}

		// Ikona i opis etapu
		icon := stageIcon(entry.Stage)
		descText := stageDescription(entry.Stage)
		var desc string
		if descText != "" {
			desc = StyleMuted.Render("  " + truncateStr(descText, width-42))
		}

		isActive := i == m.currentStep && entry.State == StageRunning
		namePart := nameStyle.Render(fmt.Sprintf("%s %-14s", icon, entry.Stage.Name))

		line := fmt.Sprintf("%s  %s%s%s", iconStr, namePart, desc, durationStr)

		if isActive {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("#1E1E2E")).
				Width(width - 4).
				Render(line)
		}

		lines = append(lines, "  "+line)
	}

	return strings.Join(lines, "\n") + "\n"
}

func (m DeployModel) renderProgress(width int) string {
	completed := m.countCompleted()
	total := len(m.stages)

	pctStr := fmt.Sprintf("%d/%d etapów", completed, total)
	bar := m.progress.View()

	return "\n  " + bar + "  " + StyleMuted.Render(pctStr) + "\n"
}

func (m DeployModel) renderOutput(width int) string {
	title := StyleLabel.Padding(0, 2).Render("Wyjście:")

	// Pokaż tylko ostatnie maxOutput linii
	lines := m.outputLines
	if len(lines) > m.maxOutput {
		lines = lines[len(lines)-m.maxOutput:]
	}

	maxLineW := width - 6
	var rendered []string
	for _, line := range lines {
		rendered = append(rendered, "  │ "+StyleMuted.Render(truncateStr(line, maxLineW)))
	}

	return "\n" + title + "\n" + strings.Join(rendered, "\n") + "\n"
}

func (m DeployModel) renderFinalStatus(width int) string {
	_ = width

	if m.completed {
		msg := StyleSuccess.Bold(true).Render("✅ Wdrożenie zakończone sukcesem!")
		elapsed := time.Since(m.startTime).Round(time.Second).String()
		timeMsg := StyleMuted.Render("  Czas: " + elapsed)
		if m.logFile != "" {
			logMsg := StyleMuted.Render("  Log: " + m.logFile)
			return "\n  " + msg + "\n  " + timeMsg + "\n  " + logMsg + "\n"
		}
		return "\n  " + msg + "\n  " + timeMsg + "\n"
	}

	if m.failed {
		msg := StyleError.Bold(true).Render("❌ Wdrożenie nieudane!")
		if m.failedStep != "" {
			msg += StyleError.Render("  Etap: " + m.failedStep)
		}
		hint := StyleWarning.Render("\n  💡 Naciśnij R na Dashboardzie aby przywrócić poprzednią wersję")
		return "\n  " + msg + hint + "\n"
	}

	return ""
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func (m *DeployModel) appendOutput(line string) {
	m.outputLines = append(m.outputLines, line)
	// Ogranicz bufor do 200 linii
	if len(m.outputLines) > 200 {
		m.outputLines = m.outputLines[len(m.outputLines)-200:]
	}
}

func (m DeployModel) countCompleted() int {
	count := 0
	for _, s := range m.stages {
		if s.State == StageSuccess || s.State == StageWarning || s.State == StageSkipped {
			count++
		}
	}
	return count
}

// formatDuration formatuje czas wykonania w czytelnej formie.
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dm%ds", ms/60000, (ms%60000)/1000)
}

// stageIcon zwraca emoji-ikonę odpowiednią dla danego etapu.
// Ikona jest dobierana na podstawie typu lub nazwy etapu.
func stageIcon(s config.Stage) string {
	switch s.Type {
	case config.StageTypeGit:
		return "🔀"
	case config.StageTypeDeploy:
		return "🚀"
	case config.StageTypeDatabase:
		return "🗄️ "
	case config.StageTypeHealth:
		return "💊"
	}

	// Dobierz ikonę na podstawie popularnych nazw etapów
	name := strings.ToLower(s.Name)
	switch {
	case name == "install" || name == "deps" || strings.Contains(name, "install"):
		return "📦"
	case name == "build" || strings.Contains(name, "build"):
		return "🏗️ "
	case strings.HasPrefix(name, "test") || strings.Contains(name, "test"):
		return "🧪"
	case name == "lint" || strings.Contains(name, "lint"):
		return "🔍"
	case name == "typecheck" || name == "types" || strings.Contains(name, "type"):
		return "✅"
	case name == "migrate" || strings.Contains(name, "migrat"):
		return "🗄️ "
	case name == "clean" || strings.Contains(name, "clean"):
		return "🧹"
	case strings.Contains(name, "docker"):
		return "🐳"
	case strings.Contains(name, "git"):
		return "🔀"
	default:
		return "⚙️ "
	}
}

// stageDescription zwraca czytelny opis etapu do wyświetlenia w TUI.
// Używa pola Description z konfiguracji (jeśli jest).
// Jeśli puste, generuje opis automatycznie na podstawie nazwy i komendy.
func stageDescription(s config.Stage) string {
	// Użyj pola Description jeśli zdefiniowane w rnr.yaml
	if s.Description != "" {
		return s.Description
	}

	// Opis na podstawie typu
	switch s.Type {
	case config.StageTypeGit:
		if strings.HasPrefix(s.Run, "checkout:") {
			branch := strings.TrimPrefix(s.Run, "checkout:")
			return fmt.Sprintf("Przełączanie na gałąź %s", branch)
		}
		if strings.HasPrefix(s.Run, "pull:") {
			branch := strings.TrimPrefix(s.Run, "pull:")
			return fmt.Sprintf("Pobieranie aktualizacji z origin/%s", branch)
		}
		return "Operacja Git"
	case config.StageTypeDeploy:
		return "Wdrożenie na serwer / CDN"
	case config.StageTypeDatabase:
		return "Migracje bazy danych"
	case config.StageTypeHealth:
		return "Sprawdzanie dostępności serwisu"
	}

	// Opis na podstawie nazwy
	name := strings.ToLower(s.Name)
	switch {
	case name == "install":
		return "Instalowanie zależności"
	case name == "build":
		return "Budowanie projektu"
	case name == "lint":
		return "Sprawdzanie jakości kodu"
	case name == "typecheck":
		return "Weryfikacja typów TypeScript"
	case name == "test" || name == "test:unit":
		return "Uruchamianie testów jednostkowych"
	case name == "test:e2e":
		return "Uruchamianie testów E2E"
	case name == "migrate":
		return "Migracje bazy danych"
	case name == "clean":
		return "Czyszczenie katalogu dist/"
	default:
		// Pokaż komendę shell jako opis (skróconą)
		if s.Run != "" && len(s.Run) <= 50 {
			return s.Run
		}
		if s.Run != "" {
			return s.Run[:47] + "..."
		}
		return ""
	}
}
