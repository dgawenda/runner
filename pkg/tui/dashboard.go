// file: pkg/tui/dashboard.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Dashboard Główny — Centrum Dowodzenia Wdrożeń                     ║
// ║                                                                      ║
// ║  Wyświetla:                                                         ║
// ║    · Status Git (gałąź, ostatni commit, czystość repozytorium)     ║
// ║    · Selektor środowisk (strzałki ↑↓)                              ║
// ║    · Historię ostatnich wdrożeń                                    ║
// ║    · Skróty klawiszowe do wszystkich operacji                      ║
// ╚══════════════════════════════════════════════════════════════════════╝

package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/gitops"
	"github.com/neution/rnr/pkg/state"
)

// ─── Model Dashboardu ─────────────────────────────────────────────────────

// DashboardModel to model głównego panelu sterowania.
type DashboardModel struct {
	width      int
	height     int
	cfg        *config.Config
	gitStatus  *gitops.StatusResult
	stateData  *state.State
	envNames   []string
	selectedEnv int
}

// NewDashboardModel tworzy nowy model dashboardu.
func NewDashboardModel(width, height int, cfg *config.Config, gitStatus *gitops.StatusResult, stateData *state.State) DashboardModel {
	envNames := cfg.GetEnvironmentNames()

	return DashboardModel{
		width:     width,
		height:    height,
		cfg:       cfg,
		gitStatus: gitStatus,
		stateData: stateData,
		envNames:  envNames,
	}
}

// ─── Interfejs Bubble Tea ─────────────────────────────────────────────────

// Init inicjalizuje model dashboardu.
func (m DashboardModel) Init() tea.Cmd {
	return nil
}

// Update obsługuje zdarzenia dashboardu.
func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case GitStatusMsg:
		if msg.Err == nil {
			m.gitStatus = msg.Result
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selectedEnv > 0 {
				m.selectedEnv--
			}
		case "down", "j":
			if m.selectedEnv < len(m.envNames)-1 {
				m.selectedEnv++
			}
		}
	}
	return m, nil
}

// SelectedEnv zwraca nazwę aktualnie wybranego środowiska.
func (m DashboardModel) SelectedEnv() string {
	if len(m.envNames) == 0 {
		return ""
	}
	if m.selectedEnv >= len(m.envNames) {
		return m.envNames[0]
	}
	return m.envNames[m.selectedEnv]
}

// ─── Widok ────────────────────────────────────────────────────────────────

// View renderuje dashboard.
func (m DashboardModel) View() string {
	if m.width < 40 {
		return "Terminal zbyt wąski — rozszerz okno do min. 40 znaków."
	}

	// Maksymalna szerokość zawartości
	contentW := min(m.width-2, 90)

	sections := []string{
		m.renderHeader(contentW),
		m.renderGitStatus(contentW),
		m.renderEnvSelector(contentW),
		m.renderHistory(contentW),
		m.renderKeyBindings(contentW),
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// ─── Sekcje Widoku ────────────────────────────────────────────────────────

func (m DashboardModel) renderHeader(width int) string {
	projectName := ""
	projectVersion := ""
	if m.cfg != nil && m.cfg.Pipeline != nil {
		projectName = m.cfg.Pipeline.Project.Name
		projectVersion = m.cfg.Pipeline.Project.Version
	}

	left := StyleTitle.Render("⚡ rnr")
	if projectName != "" {
		left += StyleMuted.Render(" / ") + StyleBold.Render(projectName)
	}
	if projectVersion != "" {
		left += StyleMuted.Render(" v"+projectVersion)
	}

	right := StyleMuted.Render(time.Now().Format("02.01.2006 15:04") + "  ⟳ live")

	header := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Render(lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(width-len(right)-2).Render(left),
			right,
		))

	return header + "\n" + Divider(width)
}

func (m DashboardModel) renderGitStatus(width int) string {
	if m.gitStatus == nil {
		return lipgloss.NewStyle().Padding(0, 2).Render(
			StyleInfo.Render("⟳  Ładowanie statusu Git..."),
		)
	}

	gs := m.gitStatus
	var lines []string

	// Gałąź i ostatni commit
	branchLine := lipgloss.JoinHorizontal(lipgloss.Top,
		StyleLabel.Render("  Gałąź:     "),
		lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render("⎇  "+gs.Branch),
	)
	lines = append(lines, branchLine)

	if gs.LastCommit.Hash != "" && gs.LastCommit.Hash != "0000000000000000000000000000000000000000" {
		commitLine := lipgloss.JoinHorizontal(lipgloss.Top,
			StyleLabel.Render("  Commit:    "),
			lipgloss.NewStyle().Foreground(ColorMuted).Render(gs.LastCommit.ShortHash+" "),
			StyleValue.Render(truncateStr(gs.LastCommit.Message, width-32)),
		)
		authorLine := lipgloss.JoinHorizontal(lipgloss.Top,
			StyleLabel.Render("  Autor:     "),
			StyleMuted.Render(gs.LastCommit.Author),
			StyleMuted.Render(" · "+gs.LastCommit.RelativeDate),
		)
		lines = append(lines, commitLine, authorLine)
	}

	// Status czystości
	var statusLine string
	if gs.IsClean {
		statusLine = lipgloss.JoinHorizontal(lipgloss.Top,
			StyleLabel.Render("  Status:    "),
			StyleSuccess.Render("✅ Repozytorium czyste — gotowe do wdrożenia"),
		)
	} else {
		statusLine = lipgloss.JoinHorizontal(lipgloss.Top,
			StyleLabel.Render("  Status:    "),
			StyleWarning.Render(fmt.Sprintf("⚠️  Niezatwierdzone zmiany (%d pliki)", len(gs.DirtyFiles))),
		)
		lines = append(lines, statusLine)
		// Pokaż kilka brudnych plików
		for i, f := range gs.DirtyFiles {
			if i >= 3 {
				lines = append(lines, StyleMuted.Render(fmt.Sprintf("    ... i %d więcej", len(gs.DirtyFiles)-3)))
				break
			}
			lines = append(lines, StyleMuted.Render(fmt.Sprintf("    [%s] %s", strings.TrimSpace(f.Status), f.Path)))
		}
		statusLine = "" // Już dodane powyżej
	}

	if statusLine != "" {
		lines = append(lines, statusLine)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return "\n" + content + "\n"
}

func (m DashboardModel) renderEnvSelector(width int) string {
	title := StyleLabel.Padding(0, 2).Render("Środowisko:")

	var rows []string
	for i, env := range m.envNames {
		envCfg, ok := m.cfg.Environments[env]
		if !ok {
			continue
		}

		isSelected := i == m.selectedEnv

		// Ikona zaznaczenia
		prefix := "    ○  "
		if isSelected {
			prefix = lipgloss.NewStyle().Foreground(ColorPrimary).Render("  ▶ ●  ")
		}

		// Badge środowiska
		badge := EnvBadge(env)

		// Informacje o środowisku
		branch := envCfg.Branch
		if branch == "" {
			branch = "(nie ustawiono)"
		}
		info := StyleMuted.Render("  gałąź: " + branch)

		// Ostatnie wdrożenie
		var lastDeploy string
		if m.stateData != nil {
			if last := m.stateData.GetLastSuccessful(env); last != nil {
				lastDeploy = StyleMuted.Render(" · ostatnie: " + last.StartedAt.Format("02.01 15:04"))
			}
		}

		// Ochrona środowiska
		var protectedBadge string
		if envCfg.Protected {
			protectedBadge = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Render(" 🔒 chronione")
		}

		var row string
		if isSelected {
			row = lipgloss.NewStyle().
				Background(lipgloss.Color("#2A2A3E")).
				Width(width - 2).
				Render(prefix + badge + info + lastDeploy + protectedBadge)
		} else {
			row = prefix + badge + info + lastDeploy + protectedBadge
		}
		rows = append(rows, row)
	}

	return "\n" + title + "\n" + strings.Join(rows, "\n") + "\n"
}

func (m DashboardModel) renderHistory(width int) string {
	if m.stateData == nil || len(m.stateData.Deployments) == 0 {
		return "\n" + StyleMuted.Padding(0, 2).Render("Brak historii wdrożeń") + "\n"
	}

	title := StyleLabel.Padding(0, 2).Render("Ostatnie wdrożenia:")
	var rows []string

	deployments := m.stateData.GetLastN("", 4)
	for _, d := range deployments {
		var statusIcon string
		switch d.Status {
		case state.StatusSuccess:
			statusIcon = StyleSuccess.Render("✓")
		case state.StatusFailed:
			statusIcon = StyleError.Render("✗")
		case state.StatusRolledBack:
			statusIcon = StyleWarning.Render("↩")
		default:
			statusIcon = StyleInfo.Render("⟳")
		}

		envBadge := EnvBadge(d.Env)
		shortHash := d.CommitHash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		timeStr := d.StartedAt.Format("02.01 15:04")
		msg := truncateStr(d.CommitMessage, width-50)

		row := fmt.Sprintf("  %s  %s  %s  %s  %s",
			statusIcon, envBadge, StyleMuted.Render(shortHash),
			StyleMuted.Render(timeStr), StyleValue.Render(msg))
		rows = append(rows, row)
	}

	return "\n" + title + "\n" + strings.Join(rows, "\n") + "\n"
}

func (m DashboardModel) renderKeyBindings(width int) string {
	_ = width

	selectedEnv := m.SelectedEnv()
	var bindings []string

	bindings = append(bindings,
		keyBind("D", "Wdróż na "+selectedEnv),
		keyBind("R", "Rollback"),
		keyBind("P", "Promote DB"),
		keyBind("G", "Git Panel"),
		keyBind("L", "Logi"),
		keyBind("↑↓", "Środowisko"),
		keyBind("Q", "Wyjdź"),
	)

	divider := Divider(width)
	row := strings.Join(bindings, "  ")

	return "\n" + divider + "\n" + lipgloss.NewStyle().Padding(0, 2).Render(row) + "\n"
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func keyBind(key, action string) string {
	return lipgloss.NewStyle().
		Foreground(ColorBg).
		Background(ColorSurface).
		Padding(0, 1).
		Render(key) +
		lipgloss.NewStyle().Foreground(ColorSubtext).Render(" "+action)
}

func truncateStr(s string, maxLen int) string {
	if maxLen <= 3 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
