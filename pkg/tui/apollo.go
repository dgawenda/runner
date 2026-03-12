// file: pkg/tui/apollo.go
//
// ╔══════════════════════════════════════════════════════════════════════════╗
// ║  Apollo — Panel Wdrożeń                                                ║
// ║                                                                          ║
// ║  Dedykowany tryb dla operacji deployment. Dostępny wyłącznie z         ║
// ║  gałęzi roboczych: master (production) i develop (development).        ║
// ╚══════════════════════════════════════════════════════════════════════════╝

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/gitops"
	"github.com/neution/rnr/pkg/state"
)

// ─── Zakładki Apollo ─────────────────────────────────────────────────────

type ApolloTab int

const (
	ApolloTabOverview ApolloTab = iota // [1] Przegląd
	ApolloTabHistory                   // [2] Historia
)

// ─── Model Apollo ────────────────────────────────────────────────────────

type ApolloModel struct {
	width  int
	height int

	tab ApolloTab

	cfg       *config.Config
	gitStatus *gitops.StatusResult
	stateData *state.State

	envNames    []string
	selectedEnv int

	guards []GuardResult

	forceRedeploy    bool
	statusMsg        string
	statusIsErr      bool
	showBranchSwitch bool
	targetBranch     string
	historyCursor    int
}

func NewApolloModel(width, height int, cfg *config.Config, gitStatus *gitops.StatusResult, stateData *state.State) ApolloModel {
	m := ApolloModel{
		width:     width,
		height:    height,
		cfg:       cfg,
		gitStatus: gitStatus,
		stateData: stateData,
		envNames:  cfg.GetEnvironmentNames(),
	}
	m.refreshGuards()
	return m
}

// ─── Bubble Tea ───────────────────────────────────────────────────────────

func (m ApolloModel) Init() tea.Cmd { return nil }

func (m ApolloModel) Update(msg tea.Msg) (ApolloModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case GitStatusMsg:
		if msg.Err == nil {
			m.gitStatus = msg.Result
			m.refreshGuards()
		}
	case tea.KeyMsg:
		if m.showBranchSwitch {
			return m.handleBranchSwitchKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m ApolloModel) handleKey(msg tea.KeyMsg) (ApolloModel, tea.Cmd) {
	switch msg.String() {
	case "1":
		m.tab = ApolloTabOverview
	case "2":
		m.tab = ApolloTabHistory
		m.historyCursor = 0
	case "up", "k":
		if m.tab == ApolloTabHistory {
			if m.historyCursor > 0 {
				m.historyCursor--
			}
		} else {
			if m.selectedEnv > 0 {
				m.selectedEnv--
				m.refreshGuards()
				m.forceRedeploy = false
				m.statusMsg = ""
			}
		}
	case "down", "j":
		if m.tab == ApolloTabHistory {
			if m.historyCursor < len(m.envDeployRecords())-1 {
				m.historyCursor++
			}
		} else {
			if m.selectedEnv < len(m.envNames)-1 {
				m.selectedEnv++
				m.refreshGuards()
				m.forceRedeploy = false
				m.statusMsg = ""
			}
		}
	case "d", "D":
		return m.tryDeploy()
	case "r", "R":
		return m.tryRollback()
	case "p", "P":
		return m.tryPromote()
	case "s", "S":
		return m.tryBranchSwitch()
	case "f", "F":
		m.forceRedeploy = !m.forceRedeploy
		if m.forceRedeploy {
			m.statusMsg = "⚡ Tryb FORCE włączony — guard 'nowe commity' zignorowany"
		} else {
			m.statusMsg = "Tryb FORCE wyłączony"
		}
		m.statusIsErr = false
		m.refreshGuards()
	}
	return m, nil
}

func (m ApolloModel) handleBranchSwitchKey(msg tea.KeyMsg) (ApolloModel, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		m.showBranchSwitch = false
		return m, func() tea.Msg { return ApolloCheckoutRequestMsg{Branch: m.targetBranch} }
	case "esc", "n", "N", "q":
		m.showBranchSwitch = false
		m.statusMsg = "Przełączenie gałęzi anulowane."
		m.statusIsErr = false
	}
	return m, nil
}

// ─── Logika akcji ─────────────────────────────────────────────────────────

func (m ApolloModel) tryDeploy() (ApolloModel, tea.Cmd) {
	env := m.selectedEnvName()
	if env == "" {
		m.statusMsg = "Brak skonfigurowanych środowisk."
		m.statusIsErr = true
		return m, nil
	}
	for _, g := range m.guards {
		if g.Name == "Gałąź robocza" && !g.Pass {
			envCfg := m.cfg.Environments[env]
			required := envCfg.Branch
			if required == "" {
				if env == "production" {
					required = "master"
				} else {
					required = "develop"
				}
			}
			m.showBranchSwitch = true
			m.targetBranch = required
			m.statusMsg = ""
			return m, nil
		}
	}
	blocking := m.effectiveBlockingGuards()
	if len(blocking) > 0 {
		names := make([]string, 0, len(blocking))
		for _, g := range blocking {
			names = append(names, g.Name)
		}
		m.statusMsg = "❌ Zablokowane: " + strings.Join(names, ", ")
		m.statusIsErr = true
		return m, nil
	}
	m.statusMsg = "🚀 Inicjuję wdrożenie..."
	m.statusIsErr = false
	return m, func() tea.Msg { return ApolloDeployRequestMsg{Env: env, Force: m.forceRedeploy} }
}

func (m ApolloModel) tryRollback() (ApolloModel, tea.Cmd) {
	env := m.selectedEnvName()
	if env == "" {
		m.statusMsg = "Brak środowiska."
		m.statusIsErr = true
		return m, nil
	}
	return m, func() tea.Msg { return ApolloRollbackRequestMsg{Env: env} }
}

func (m ApolloModel) tryPromote() (ApolloModel, tea.Cmd) {
	return m, func() tea.Msg { return ApolloPromoteRequestMsg{} }
}

func (m ApolloModel) tryBranchSwitch() (ApolloModel, tea.Cmd) {
	env := m.selectedEnvName()
	if env == "" {
		return m, nil
	}
	envCfg := m.cfg.Environments[env]
	required := envCfg.Branch
	if required == "" {
		if env == "production" {
			required = "master"
		} else {
			required = "develop"
		}
	}
	if m.gitStatus != nil && m.gitStatus.Branch == required {
		m.statusMsg = fmt.Sprintf("✓ Już jesteś na gałęzi '%s'", required)
		m.statusIsErr = false
		return m, nil
	}
	m.showBranchSwitch = true
	m.targetBranch = required
	return m, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func (m *ApolloModel) refreshGuards() {
	env := m.selectedEnvName()
	if env == "" || m.cfg == nil {
		m.guards = nil
		return
	}
	m.guards = RunDeployGuards(env, m.cfg.Environments[env], m.gitStatus, m.stateData)
}

func (m ApolloModel) effectiveBlockingGuards() []GuardResult {
	var out []GuardResult
	for _, g := range m.guards {
		if g.Pass || g.Level != GuardLevelBlock {
			continue
		}
		if m.forceRedeploy && g.Name == "Nowe commity" {
			continue
		}
		if g.Name == "Gałąź robocza" {
			continue
		}
		out = append(out, g)
	}
	return out
}

func (m ApolloModel) selectedEnvName() string {
	if len(m.envNames) == 0 {
		return ""
	}
	if m.selectedEnv >= len(m.envNames) {
		return m.envNames[0]
	}
	return m.envNames[m.selectedEnv]
}

func (m ApolloModel) envDeployRecords() []state.DeployRecord {
	if m.stateData == nil {
		return nil
	}
	return m.stateData.GetLastN(m.selectedEnvName(), 20)
}

func (m ApolloModel) readyToDeploy() bool {
	return len(m.effectiveBlockingGuards()) == 0 && m.selectedEnvName() != ""
}

// ─── Widok ────────────────────────────────────────────────────────────────

func (m ApolloModel) View() string {
	if m.width < 40 {
		return "Terminal zbyt wąski (min. 40 znaków)"
	}

	w := min(m.width, 96)

	if m.showBranchSwitch {
		return m.renderBranchSwitch(w)
	}

	var sb strings.Builder

	// 1. Top bar Apollo
	sb.WriteString(m.renderTopBar(w))
	sb.WriteString("\n")

	// 2. Tabs — wyłącznie raz
	sb.WriteString(m.renderTabs(w))
	sb.WriteString("\n")

	// 3. Treść zakładki — w ramce o stałej wysokości
	contentLines := m.renderTabContent(w)
	sb.WriteString(contentLines)

	// 4. Status msg
	if m.statusMsg != "" {
		sb.WriteString("\n")
		if m.statusIsErr {
			sb.WriteString(" " + StyleError.Render(m.statusMsg))
		} else {
			sb.WriteString(" " + StyleInfo.Render(m.statusMsg))
		}
	}

	// 5. Key bar
	sb.WriteString("\n")
	sb.WriteString(m.renderKeyBar(w))

	return sb.String()
}

// ─── Top bar ─────────────────────────────────────────────────────────────

func (m ApolloModel) renderTopBar(w int) string {
	apolloLabel := lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorApollo).
		Bold(true).Padding(0, 2).Render("🚀 Apollo")
	subtitle := lipgloss.NewStyle().Foreground(ColorSubtext).
		Render("— Panel Wdrożeń  (Q = Dashboard)")

	// Force badge jeśli aktywny
	var forceBadge string
	if m.forceRedeploy {
		forceBadge = "  " + lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorWarning).
			Bold(true).Padding(0, 1).Render("⚡ FORCE")
	}

	banner := lipgloss.NewStyle().
		Background(lipgloss.Color("#1e1e3f")).
		Foreground(lipgloss.Color("#9090C0")).
		Width(w).Padding(0, 2).
		Render("🛡  Tryb Apollo: wdrożenia wyłącznie z gałęzi roboczych (master/develop) · wszystkie operacje chronione strażnikami")

	topLine := lipgloss.NewStyle().Width(w).Padding(0, 1).
		Render(apolloLabel + "  " + subtitle + forceBadge)

	divLine := lipgloss.NewStyle().Foreground(ColorApollo).Render(repeatChar("━", w))

	return topLine + "\n" + banner + "\n" + divLine
}

// ─── Tabs (TYLKO raz) ────────────────────────────────────────────────────

func (m ApolloModel) renderTabs(w int) string {
	labels := []string{"1 PRZEGLĄD", "2 HISTORIA"}
	bar := RenderTabs(labels, int(m.tab), ColorApollo)
	return bar
}

// ─── Treść aktywnej zakładki ─────────────────────────────────────────────

// renderTabContent renderuje zawartość aktywnej zakładki.
// Zawartość jest zawsze zakończona newline — Bubble Tea nie potrzebuje
// stałej wysokości, ale każda zakładka musi zaczynać się od tej samej pozycji.
func (m ApolloModel) renderTabContent(w int) string {
	switch m.tab {
	case ApolloTabOverview:
		return m.renderOverview(w)
	case ApolloTabHistory:
		return m.renderHistory(w)
	}
	return ""
}

// ─── Zakładka 1: Przegląd ────────────────────────────────────────────────

func (m ApolloModel) renderOverview(w int) string {
	var rows []string

	// Selektor środowisk
	rows = append(rows, "")
	rows = append(rows, m.renderEnvSelector(w))

	// Strażnicy
	rows = append(rows, "")
	rows = append(rows, m.renderGuards(w))

	return strings.Join(rows, "\n")
}

func (m ApolloModel) renderEnvSelector(w int) string {
	header := SectionHeader("🌍", "Środowisko docelowe:", w-2)
	var rows []string
	rows = append(rows, header)

	for i, env := range m.envNames {
		envCfg, ok := m.cfg.Environments[env]
		if !ok {
			continue
		}
		isSelected := i == m.selectedEnv

		var cursor string
		if isSelected {
			cursor = lipgloss.NewStyle().Foreground(ColorApollo).Bold(true).Render("  ▶ ●  ")
		} else {
			cursor = "    ○  "
		}

		badge := EnvBadge(env)
		branch := envCfg.Branch
		if branch == "" {
			branch = "auto"
		}
		branchStr := lipgloss.NewStyle().Foreground(ColorMuted).Render("  ⎇ " + branch)

		var lastDeploy string
		if m.stateData != nil {
			if last := m.stateData.GetLastSuccessful(env); last != nil {
				lastDeploy = lipgloss.NewStyle().Foreground(ColorMuted).
					Render(" · deploy: " + last.StartedAt.Format("02.01 15:04"))
			}
		}

		row := cursor + badge + branchStr + lastDeploy
		if isSelected {
			row = lipgloss.NewStyle().
				Background(lipgloss.Color("#2A2A3E")).
				Width(w - 2).
				Render(row)
		}
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

func (m ApolloModel) renderGuards(w int) string {
	header := SectionHeader("🛡", "Strażnicy wdrożenia:", w-2)
	var rows []string
	rows = append(rows, header)

	if len(m.guards) == 0 {
		rows = append(rows, "  "+StyleMuted.Render("Brak skonfigurowanego środowiska"))
		return strings.Join(rows, "\n")
	}

	allOK := true
	for _, g := range m.guards {
		effective := g.Pass
		if m.forceRedeploy && g.Name == "Nowe commity" && !g.Pass {
			effective = true
		}

		var icon string
		if effective {
			icon = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("✓")
		} else if g.Level == GuardLevelWarn {
			icon = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render("⚠")
			allOK = false
		} else {
			icon = lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render("✗")
			allOK = false
		}

		nameCol := lipgloss.NewStyle().Foreground(ColorSubtext).Width(22).Render(g.Name)
		reasonCol := lipgloss.NewStyle().Foreground(ColorMuted).Render(g.Reason)

		rows = append(rows, "  "+icon+"  "+nameCol+"  "+reasonCol)

		if !effective && g.Hint != "" {
			rows = append(rows,
				"        "+lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4")).
					Render("↳ "+g.Hint),
			)
		}
	}

	// Podsumowanie
	rows = append(rows, "")
	blocking := m.effectiveBlockingGuards()
	branchBlocked := false
	for _, g := range m.guards {
		if g.Name == "Gałąź robocza" && !g.Pass {
			branchBlocked = true
		}
	}

	if allOK || len(blocking) == 0 {
		summary := lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorSuccess).
			Bold(true).Padding(0, 1).
			Render("✅ Wszystkie strażniki OK — wdrożenie gotowe!")
		if m.forceRedeploy {
			summary += "  " + lipgloss.NewStyle().
				Foreground(ColorBg).Background(ColorWarning).
				Padding(0, 1).Render("⚡ FORCE")
		}
		rows = append(rows, "  "+summary)
	} else if branchBlocked && len(blocking) == 0 {
		rows = append(rows, "  "+lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorWarning).
			Padding(0, 1).Render("⚠  Gałąź nieprawidłowa — naciśnij S aby przełączyć"))
	} else {
		rows = append(rows, "  "+lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorError).
			Bold(true).Padding(0, 1).
			Render(fmt.Sprintf("❌  %d strażnik(ów) blokuje wdrożenie", len(BlockingGuards(m.guards)))))
	}

	return strings.Join(rows, "\n")
}

// ─── Zakładka 2: Historia ─────────────────────────────────────────────────

func (m ApolloModel) renderHistory(w int) string {
	records := m.envDeployRecords()
	env := m.selectedEnvName()

	var rows []string
	rows = append(rows, "")

	// Nagłówek sekcji
	rows = append(rows, SectionHeader("📋", "Historia wdrożeń: "+EnvStyle(env).Render(env), w-2))
	rows = append(rows, "")

	if len(records) == 0 {
		rows = append(rows,
			"  "+StyleMuted.Render("Brak historii wdrożeń dla środowiska ")+
				EnvStyle(env).Render(env)+".",
		)
		return strings.Join(rows, "\n")
	}

	// Nagłówek tabeli
	colSt := lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(4).Render("St.")
	colDt := lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(16).Render("Data")
	colH := lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(9).Render("Commit")
	colAu := lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(18).Render("Autor")
	colMsg := lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Render("Wiadomość")
	rows = append(rows, "  "+lipgloss.JoinHorizontal(lipgloss.Top, colSt, colDt, colH, colAu, colMsg))
	rows = append(rows, "  "+lipgloss.NewStyle().Foreground(ColorSurface).Render(repeatChar("─", w-4)))

	maxRows := min(len(records), m.height-18)
	for i, d := range records {
		if i >= maxRows {
			rows = append(rows, "  "+StyleMuted.Render(fmt.Sprintf("... i %d więcej", len(records)-maxRows)))
			break
		}

		var icon string
		switch d.Status {
		case state.StatusSuccess:
			icon = lipgloss.NewStyle().Foreground(ColorSuccess).Render("✓")
		case state.StatusFailed:
			icon = lipgloss.NewStyle().Foreground(ColorError).Render("✗")
		case state.StatusRolledBack:
			icon = lipgloss.NewStyle().Foreground(ColorWarning).Render("↩")
		default:
			icon = lipgloss.NewStyle().Foreground(ColorInfo).Render("⟳")
		}

		shortHash := d.CommitHash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		stC := lipgloss.NewStyle().Width(4).Render(icon)
		dtC := lipgloss.NewStyle().Width(16).Foreground(ColorSubtext).Render(d.StartedAt.Format("02.01 15:04:05"))
		hC := lipgloss.NewStyle().Width(9).Foreground(ColorInfo).Render(shortHash)
		auC := lipgloss.NewStyle().Width(18).Foreground(ColorSubtext).Render(truncateStr(d.CommitAuthor, 16))
		mC := lipgloss.NewStyle().Foreground(ColorText).Render(truncateStr(d.CommitMessage, w-55))

		row := "  " + lipgloss.JoinHorizontal(lipgloss.Top, stC, dtC, hC, auC, mC)
		if i == m.historyCursor {
			row = lipgloss.NewStyle().
				Background(lipgloss.Color("#2A2A3E")).
				Width(w - 2).
				Render(row)
		}
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

// ─── Ekran przełączania gałęzi ───────────────────────────────────────────

func (m ApolloModel) renderBranchSwitch(w int) string {
	current := ""
	if m.gitStatus != nil {
		current = m.gitStatus.Branch
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		"",
		lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).Render("⎇  Przełączenie gałęzi"),
		"",
		lipgloss.NewStyle().Foreground(ColorText).Render(fmt.Sprintf(
			"Apollo wymaga gałęzi '%s' dla tego środowiska.",
			m.targetBranch,
		)),
		"",
		lipgloss.NewStyle().Foreground(ColorSubtext).Render("Aktualna gałąź: ")+
			lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render(current),
		lipgloss.NewStyle().Foreground(ColorSubtext).Render("Docelowa gałąź: ")+
			lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render(m.targetBranch),
		"",
		lipgloss.NewStyle().Foreground(ColorMuted).Render("Apollo wykona: git checkout "+m.targetBranch),
		lipgloss.NewStyle().Foreground(ColorMuted).Render("Upewnij się że masz zatwierdzone wszystkie zmiany."),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top,
			StyleButton.Render(" ENTER / Y = Przełącz "),
			"  ",
			StyleButtonSecondary.Render(" ESC / N = Anuluj "),
		),
		"",
	)

	panel := StylePanelError.Width(min(w-4, 60)).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}

// ─── Key bar ──────────────────────────────────────────────────────────────

func (m ApolloModel) renderKeyBar(w int) string {
	env := m.selectedEnvName()

	branchBlocked := false
	for _, g := range m.guards {
		if g.Name == "Gałąź robocza" && !g.Pass {
			branchBlocked = true
		}
	}

	var bindings []KeyBinding
	if m.readyToDeploy() || m.forceRedeploy {
		bindings = append(bindings, KeyBinding{"D", "Wdróż " + env})
	} else {
		if branchBlocked {
			bindings = append(bindings, KeyBinding{"S", "Przełącz gałąź"})
		}
		bindings = append(bindings, KeyBinding{"D", "Wdróż*"})
	}
	bindings = append(bindings, KeyBinding{"R", "Rollback"}, KeyBinding{"P", "Promote DB"})
	if m.forceRedeploy {
		bindings = append(bindings, KeyBinding{"F", "⚡[FORCE]"})
	} else {
		bindings = append(bindings, KeyBinding{"F", "Wymuś"})
	}
	bindings = append(bindings,
		KeyBinding{"1/2", "Zakładki"},
		KeyBinding{"↑↓", "Środowisko"},
		KeyBinding{"Q", "Wróć"},
	)

	return KeyBar(bindings, w)
}
