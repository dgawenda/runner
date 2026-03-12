// file: pkg/tui/dashboard.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Dashboard Główny — Centrum Dowodzenia                             ║
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

type DashboardModel struct {
	width       int
	height      int
	cfg         *config.Config
	gitStatus   *gitops.StatusResult
	stateData   *state.State
	envNames    []string
	selectedEnv int
}

func NewDashboardModel(width, height int, cfg *config.Config, gitStatus *gitops.StatusResult, stateData *state.State) DashboardModel {
	return DashboardModel{
		width:     width,
		height:    height,
		cfg:       cfg,
		gitStatus: gitStatus,
		stateData: stateData,
		envNames:  cfg.GetEnvironmentNames(),
	}
}

func (m DashboardModel) Init() tea.Cmd { return nil }

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

func (m DashboardModel) View() string {
	if m.width < 40 {
		return "Terminal zbyt wąski (min. 40 znaków)"
	}
	w := min(m.width, 96)

	var rows []string
	rows = append(rows, m.renderTopBar(w))
	rows = append(rows, m.renderGitCard(w))
	rows = append(rows, m.renderEnvAndHistory(w))
	rows = append(rows, m.renderKeyBar(w))

	return strings.Join(rows, "\n")
}

// ─── Top bar ─────────────────────────────────────────────────────────────

func (m DashboardModel) renderTopBar(w int) string {
	projectName := ""
	projectVersion := ""
	if m.cfg != nil && m.cfg.Pipeline != nil {
		projectName = m.cfg.Pipeline.Project.Name
		projectVersion = m.cfg.Pipeline.Project.Version
	}

	// Lewa strona — logo + projekt
	left := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("⚡ rnr")
	if projectName != "" {
		left += lipgloss.NewStyle().Foreground(ColorMuted).Render(" / ")
		left += lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(projectName)
	}
	if projectVersion != "" {
		left += lipgloss.NewStyle().Foreground(ColorMuted).Render("  " + projectVersion)
	}

	// Przyciski trybów
	gitBtn := lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorSecondary).
		Padding(0, 1).Bold(true).Render("G GitPanel")
	releaseBtn := lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorApollo).
		Padding(0, 1).Bold(true).Render("A ReleasePanel")
	modes := gitBtn + " " + releaseBtn

	// Prawa strona — czas
	timeStr := lipgloss.NewStyle().Foreground(ColorMuted).
		Render(time.Now().Format("02.01.2006 15:04") + "  ⟳ live")

	leftFull := left + "   " + modes
	leftW := w - lipgloss.Width(timeStr) - 2
	if leftW < 0 {
		leftW = 0
	}

	topLine := lipgloss.NewStyle().Width(w).
		Background(ColorBgAlt).Padding(0, 1).
		Render(lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(leftW).Render(leftFull),
			timeStr,
		))

	divLine := lipgloss.NewStyle().Foreground(ColorPrimary).Render(repeatChar("━", w))
	return topLine + "\n" + divLine
}

// ─── Karta Git Status ─────────────────────────────────────────────────────

func (m DashboardModel) renderGitCard(w int) string {
	if m.gitStatus == nil {
		return "\n " + StyleInfo.Render("⟳  Ładowanie statusu Git...") + "\n"
	}

	gs := m.gitStatus
	inner := w - 4

	var rows []string

	// Wiersz 1: gałąź + status
	branchBadge := lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorSecondary).
		Bold(true).Padding(0, 1).Render("⎇  " + gs.Branch)

	var statusBadge string
	if gs.IsClean {
		statusBadge = lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorSuccess).
			Padding(0, 1).Render("✓ czyste")
	} else {
		statusBadge = lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorWarning).
			Padding(0, 1).Render(fmt.Sprintf("⚠ %d zmian", len(gs.DirtyFiles)))
	}

	rows = append(rows, "  "+branchBadge+"  "+statusBadge)

	// Wiersz 2: ostatni commit
	if gs.LastCommit.Hash != "" && gs.LastCommit.Hash != "0000000000000000000000000000000000000000" {
		hashPart := lipgloss.NewStyle().Foreground(ColorMuted).Render(gs.LastCommit.ShortHash)
		msgPart := lipgloss.NewStyle().Foreground(ColorText).Render(truncateStr(gs.LastCommit.Message, inner-40))
		authorPart := lipgloss.NewStyle().Foreground(ColorMuted).Render("  " + gs.LastCommit.Author + " · " + gs.LastCommit.RelativeDate)
		rows = append(rows, "  "+hashPart+"  "+msgPart+authorPart)
	}

	// Brudne pliki (kilka)
	if !gs.IsClean {
		shown := gs.DirtyFiles
		extra := 0
		if len(shown) > 4 {
			extra = len(shown) - 4
			shown = shown[:4]
		}
		for _, f := range shown {
			icon, sty := gitFileIcon(f.Status)
			rows = append(rows,
				"    "+sty.Render(fmt.Sprintf("[%s]", strings.TrimSpace(f.Status)))+" "+icon+"  "+
					lipgloss.NewStyle().Foreground(ColorSubtext).Render(f.Path),
			)
		}
		if extra > 0 {
			rows = append(rows, "    "+StyleMuted.Render(fmt.Sprintf("... i %d więcej", extra)))
		}
	}

	content := strings.Join(rows, "\n")

	borderColor := ColorSurface
	if !gs.IsClean {
		borderColor = ColorWarning
	} else if gs.IsClean {
		borderColor = ColorSuccess
	}

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(w - 2).
		Padding(0, 1).
		Render(content)

	return "\n" + card
}

// ─── Środowiska + Historia ────────────────────────────────────────────────

func (m DashboardModel) renderEnvAndHistory(w int) string {
	repoBlock := m.renderRepoSummary(w)
	histBlock := m.renderHistoryBlock(w)
	return "\n" + repoBlock + "\n" + histBlock
}

// renderRepoSummary wyświetla podsumowanie repozytorium (remote + ahead/behind).
func (m DashboardModel) renderRepoSummary(w int) string {
	header := SectionHeader("⮃", "Synchronizacja repozytorium:", w-2)

	if m.gitStatus == nil {
		return header + "\n " + StyleInfo.Render("⟳  Ładowanie statusu Git...") + "\n"
	}

	gs := m.gitStatus
	var rows []string
	rows = append(rows, header)

	// Remote
	if gs.HasRemote {
		remoteShort := gs.RemoteURL
		if len(remoteShort) > 60 {
			remoteShort = "…" + remoteShort[len(remoteShort)-57:]
		}
		rows = append(rows, "  "+StyleMuted.Render("🔗 "+remoteShort))
	} else {
		rows = append(rows,
			"  "+StyleWarning.Render("⚠  Brak remote origin — push niedostępny"),
			"  "+StyleMuted.Render("   Dodaj: git remote add origin <URL>"),
		)
	}

	// Gałąź
	rows = append(rows,
		"  "+StyleLabel.Render("Gałąź: ")+
			lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render("⎇ "+gs.Branch),
	)

	// Ahead / Behind
	if gs.HasRemote {
		if gs.HasUpstream {
			switch {
			case gs.Ahead == 0 && gs.Behind == 0:
				rows = append(rows,
					"  "+StyleSuccess.Render("⮃ Gałąź zsynchronizowana z origin"),
				)
			default:
				var parts []string
				if gs.Ahead > 0 {
					parts = append(parts,
						lipgloss.NewStyle().Foreground(ColorSuccess).
							Render(fmt.Sprintf("↑ %d lokalnych", gs.Ahead)),
					)
				}
				if gs.Behind > 0 {
					parts = append(parts,
						lipgloss.NewStyle().Foreground(ColorWarning).
							Render(fmt.Sprintf("↓ %d do pobrania", gs.Behind)),
					)
				}
				rows = append(rows,
					"  "+StyleMuted.Render("⮃ "+strings.Join(parts, "  ")),
				)
			}
		} else {
			rows = append(rows,
				"  "+StyleMuted.Render("⮃ Brak upstream — pierwszy push ustawi śledzenie (origin/"+gs.Branch+")"),
			)
		}
	}

	return strings.Join(rows, "\n")
}

func (m DashboardModel) renderEnvBlock(w int) string {
	header := SectionHeader("🌍", "Środowisko:", w-2)

	var rows []string
	for i, env := range m.envNames {
		envCfg, ok := m.cfg.Environments[env]
		if !ok {
			continue
		}

		isSelected := i == m.selectedEnv

		// Ikona kursora
		var cursor string
		if isSelected {
			cursor = lipgloss.NewStyle().Foreground(ColorApollo).Bold(true).Render("▶ ")
		} else {
			cursor = "  "
		}

		badge := EnvBadge(env)
		branch := envCfg.Branch
		if branch == "" {
			branch = "auto"
		}
		branchStr := lipgloss.NewStyle().Foreground(ColorMuted).Render("  ⎇ " + branch)

		var extras []string
		if envCfg.Protected {
			extras = append(extras, lipgloss.NewStyle().Foreground(ColorWarning).Render("🔒 chronione"))
		}
		if m.stateData != nil {
			if last := m.stateData.GetLastSuccessful(env); last != nil {
				extras = append(extras, lipgloss.NewStyle().Foreground(ColorMuted).
					Render("ostatnie: "+last.StartedAt.Format("02.01 15:04")))
			}
		}

		extrasStr := ""
		if len(extras) > 0 {
			extrasStr = "  " + strings.Join(extras, "  ")
		}

		row := cursor + badge + branchStr + extrasStr
		if isSelected {
			row = lipgloss.NewStyle().
				Background(lipgloss.Color("#2A2A3E")).
				Width(w - 4).
				Padding(0, 1).
				Render(row)
		}
		rows = append(rows, " "+row)
	}

	return header + "\n" + strings.Join(rows, "\n")
}

func (m DashboardModel) renderHistoryBlock(w int) string {
	if m.stateData == nil || len(m.stateData.Deployments) == 0 {
		return " " + StyleMuted.Render("Brak historii wdrożeń")
	}

	header := SectionHeader("📋", "Ostatnie wdrożenia:", w-2)
	var rows []string

	for _, d := range m.stateData.GetLastN("", 4) {
		var icon string
		switch d.Status {
		case state.StatusSuccess:
			icon = StyleSuccess.Render("✓")
		case state.StatusFailed:
			icon = StyleError.Render("✗")
		case state.StatusRolledBack:
			icon = StyleWarning.Render("↩")
		default:
			icon = StyleInfo.Render("⟳")
		}

		shortHash := d.CommitHash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		envStr := EnvStyle(d.Env).Render(fmt.Sprintf("%-12s", d.Env))
		hashStr := lipgloss.NewStyle().Foreground(ColorInfo).Render(fmt.Sprintf("%-8s", shortHash))
		timeStr := lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("%-12s", d.StartedAt.Format("02.01 15:04")))
		msgStr := lipgloss.NewStyle().Foreground(ColorSubtext).Render(truncateStr(d.CommitMessage, w-50))

		rows = append(rows, "  "+icon+"  "+envStr+"  "+hashStr+"  "+timeStr+"  "+msgStr)
	}

	return header + "\n" + strings.Join(rows, "\n")
}

// ─── Key Bar ──────────────────────────────────────────────────────────────

func (m DashboardModel) renderKeyBar(w int) string {
	bindings := []KeyBinding{
		{"G", "GitPanel 🔧"},
		{"A", "ReleasePanel 🚀"},
		{"L", "Logi"},
		{"Q", "Wyjdź"},
	}
	return "\n" + KeyBar(bindings, w)
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func keyBind(key, action string) string {
	return lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorSurface).
		Padding(0, 1).Render(key) +
		lipgloss.NewStyle().Foreground(ColorSubtext).Render(" " + action)
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
