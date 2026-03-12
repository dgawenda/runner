// file: pkg/tui/apollo.go
//
// ╔══════════════════════════════════════════════════════════════════════════╗
// ║  Apollo — Panel Wdrożeń                                                ║
// ║                                                                          ║
// ║  Dedykowany tryb dla operacji deployment. Dostępny wyłącznie z         ║
// ║  gałęzi roboczych: master (production) i develop (development).        ║
// ║                                                                          ║
// ║  Proces wdrożenia w Apollo:                                             ║
// ║    1. Wybierz środowisko (↑↓)                                          ║
// ║    2. Apollo sprawdza Strażników (guards)                               ║
// ║    3. Jeśli strażnicy zaliczeni → D = wdróż                            ║
// ║    4. Jeśli gałąź nie pasuje → Apollo pyta o przełączenie              ║
// ║                                                                          ║
// ║  Zakładki:                                                               ║
// ║    [1] STARTY   — status strażników + deploy/rollback/promote          ║
// ║    [2] HISTORIA — historia wdrożeń z detalami                          ║
// ║    [3] LOGI     — ostatni log wdrożenia                                ║
// ║                                                                          ║
// ║  Klawiszologia:                                                          ║
// ║    1/2/3         — zakładki                                              ║
// ║    D             — wdróż (gdy wszystkie strażnicy zaliczeni)            ║
// ║    R             — rollback (wybór z historii)                          ║
// ║    P             — promote DB (development → production)                ║
// ║    S             — przełącz gałąź (gdy guard gałęzi nie zaliczony)     ║
// ║    F             — wymusz redeploy (pomija guard "nowe commity")        ║
// ║    Q / ESC       — wróć do Dashboard                                    ║
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

// ApolloTab to aktywna zakładka panelu Apollo.
type ApolloTab int

const (
	ApolloTabOverview ApolloTab = iota // [1] Przegląd: strażnicy + akcje
	ApolloTabHistory                   // [2] Historia wdrożeń
)

// ─── Model Apollo ────────────────────────────────────────────────────────

// ApolloModel to model panelu wdrożeń Apollo.
type ApolloModel struct {
	width  int
	height int

	// Aktywna zakładka
	tab ApolloTab

	// Dane zewnętrzne (aktualizowane przez root model)
	cfg       *config.Config
	gitStatus *gitops.StatusResult
	stateData *state.State

	// Wybrane środowisko (indeks z listy)
	envNames    []string
	selectedEnv int

	// Wyniki strażników dla wybranego środowiska
	guards []GuardResult

	// Stan akcji
	forceRedeploy bool // true = ignoruj guard "nowe commity"
	statusMsg     string
	statusIsErr   bool

	// Ekran potwierdzenia przełączenia gałęzi
	showBranchSwitch bool
	targetBranch     string

	// Historia — kursor
	historyCursor int
}

// NewApolloModel tworzy nowy model Apollo z podanymi danymi.
func NewApolloModel(width, height int, cfg *config.Config, gitStatus *gitops.StatusResult, stateData *state.State) ApolloModel {
	envNames := cfg.GetEnvironmentNames()

	m := ApolloModel{
		width:     width,
		height:    height,
		cfg:       cfg,
		gitStatus: gitStatus,
		stateData: stateData,
		envNames:  envNames,
	}

	m.refreshGuards()
	return m
}

// ─── Interfejs Bubble Tea ─────────────────────────────────────────────────

// Init inicjalizuje model Apollo.
func (m ApolloModel) Init() tea.Cmd {
	return nil
}

// Update obsługuje zdarzenia panelu Apollo.
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

// handleKey obsługuje klawisze w normalnym trybie Apollo.
func (m ApolloModel) handleKey(msg tea.KeyMsg) (ApolloModel, tea.Cmd) {
	switch msg.String() {
	// ── Zakładki ─────────────────────────────────────────────────────────
	case "1":
		m.tab = ApolloTabOverview
	case "2":
		m.tab = ApolloTabHistory
		m.historyCursor = 0

	// ── Nawigacja (środowiska lub historia zależnie od zakładki) ────────
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
			records := m.envDeployRecords()
			if m.historyCursor < len(records)-1 {
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

	// ── Akcje wdrożenia ──────────────────────────────────────────────────
	case "d", "D":
		return m.tryDeploy()
	case "r", "R":
		return m.tryRollback()
	case "p", "P":
		return m.tryPromote()

	// ── Przełącz gałąź (gdy guard gałęzi niespełniony) ───────────────────
	case "s", "S":
		return m.tryBranchSwitch()

	// ── Wymuś redeploy (pomija guard "nowe commity") ─────────────────────
	case "f", "F":
		m.forceRedeploy = !m.forceRedeploy
		if m.forceRedeploy {
			m.statusMsg = "⚡ Tryb wymuszenia włączony — guard 'nowe commity' zignorowany"
			m.statusIsErr = false
		} else {
			m.statusMsg = "Tryb wymuszenia wyłączony"
			m.statusIsErr = false
		}
		m.refreshGuards()
	}

	return m, nil
}

// handleBranchSwitchKey obsługuje klawisze na ekranie potwierdzenia przełączenia gałęzi.
func (m ApolloModel) handleBranchSwitchKey(msg tea.KeyMsg) (ApolloModel, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		// Zatwierdź przełączenie gałęzi
		m.showBranchSwitch = false
		return m, func() tea.Msg {
			return ApolloCheckoutRequestMsg{Branch: m.targetBranch}
		}
	case "esc", "n", "N", "q":
		m.showBranchSwitch = false
		m.statusMsg = "Przełączenie gałęzi anulowane."
		m.statusIsErr = false
	}
	return m, nil
}

// ─── Logika akcji ─────────────────────────────────────────────────────────

// tryDeploy próbuje uruchomić wdrożenie. Sprawdza strażników.
func (m ApolloModel) tryDeploy() (ApolloModel, tea.Cmd) {
	env := m.selectedEnvName()
	if env == "" {
		m.statusMsg = "Brak skonfigurowanych środowisk."
		m.statusIsErr = true
		return m, nil
	}

	// Sprawdź guard gałęzi osobno — zaoferuj przełączenie
	for _, g := range m.guards {
		if g.Name == "Gałąź robocza" && !g.Pass {
			// Zamiast blokady, zaoferuj automatyczne przełączenie
			envCfg := m.cfg.Environments[env]
			required := envCfg.Branch
			if required == "" {
				switch env {
				case "production":
					required = "master"
				default:
					required = "develop"
				}
			}
			m.showBranchSwitch = true
			m.targetBranch = required
			m.statusMsg = ""
			return m, nil
		}
	}

	// Sprawdź pozostałe strażniki blokujące
	// W trybie forceRedeploy ignoruj guard "nowe commity"
	blocking := m.effectiveBlockingGuards()
	if len(blocking) > 0 {
		names := make([]string, 0, len(blocking))
		for _, g := range blocking {
			names = append(names, g.Name)
		}
		m.statusMsg = fmt.Sprintf("❌ Wdrożenie zablokowane: %s", strings.Join(names, ", "))
		m.statusIsErr = true
		return m, nil
	}

	// Wszystko OK — wyślij żądanie wdrożenia
	m.statusMsg = "🚀 Inicjuję wdrożenie..."
	m.statusIsErr = false
	return m, func() tea.Msg {
		return ApolloDeployRequestMsg{Env: env, Force: m.forceRedeploy}
	}
}

// tryRollback próbuje uruchomić rollback.
func (m ApolloModel) tryRollback() (ApolloModel, tea.Cmd) {
	env := m.selectedEnvName()
	if env == "" {
		m.statusMsg = "Brak skonfigurowanych środowisk."
		m.statusIsErr = true
		return m, nil
	}
	return m, func() tea.Msg {
		return ApolloRollbackRequestMsg{Env: env}
	}
}

// tryPromote próbuje uruchomić promote DB.
func (m ApolloModel) tryPromote() (ApolloModel, tea.Cmd) {
	return m, func() tea.Msg {
		return ApolloPromoteRequestMsg{}
	}
}

// tryBranchSwitch inicjuje przełączenie gałęzi dla wybranego środowiska.
func (m ApolloModel) tryBranchSwitch() (ApolloModel, tea.Cmd) {
	env := m.selectedEnvName()
	if env == "" {
		return m, nil
	}

	envCfg := m.cfg.Environments[env]
	required := envCfg.Branch
	if required == "" {
		switch env {
		case "production":
			required = "master"
		default:
			required = "develop"
		}
	}

	if m.gitStatus != nil && m.gitStatus.Branch == required {
		m.statusMsg = fmt.Sprintf("Jesteś już na gałęzi '%s' ✓", required)
		m.statusIsErr = false
		return m, nil
	}

	m.showBranchSwitch = true
	m.targetBranch = required
	return m, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────

// refreshGuards przelicza strażników dla aktualnie wybranego środowiska.
func (m *ApolloModel) refreshGuards() {
	env := m.selectedEnvName()
	if env == "" || m.cfg == nil {
		m.guards = nil
		return
	}
	envCfg := m.cfg.Environments[env]
	m.guards = RunDeployGuards(env, envCfg, m.gitStatus, m.stateData)
}

// effectiveBlockingGuards zwraca strażniki blokujące, z opcjonalnym pominięciem "nowe commity".
func (m ApolloModel) effectiveBlockingGuards() []GuardResult {
	var out []GuardResult
	for _, g := range m.guards {
		if g.Pass || g.Level != GuardLevelBlock {
			continue
		}
		// W trybie forceRedeploy pomijamy guard "Nowe commity"
		if m.forceRedeploy && g.Name == "Nowe commity" {
			continue
		}
		// Guard gałęzi jest obsługiwany osobno (zaoferuj przełączenie)
		if g.Name == "Gałąź robocza" {
			continue
		}
		out = append(out, g)
	}
	return out
}

// selectedEnvName zwraca nazwę aktualnie wybranego środowiska.
func (m ApolloModel) selectedEnvName() string {
	if len(m.envNames) == 0 {
		return ""
	}
	if m.selectedEnv >= len(m.envNames) {
		return m.envNames[0]
	}
	return m.envNames[m.selectedEnv]
}

// envDeployRecords zwraca rekordy wdrożeń dla wybranego środowiska.
func (m ApolloModel) envDeployRecords() []state.DeployRecord {
	if m.stateData == nil {
		return nil
	}
	return m.stateData.GetLastN(m.selectedEnvName(), 20)
}

// readyToDeploy sprawdza czy deploy jest gotowy (wszyscy blokujący strażnicy zaliczeni).
func (m ApolloModel) readyToDeploy() bool {
	return len(m.effectiveBlockingGuards()) == 0 && m.selectedEnvName() != ""
}

// ─── Widok Apollo ─────────────────────────────────────────────────────────

// View renderuje panel Apollo.
func (m ApolloModel) View() string {
	if m.width < 40 {
		return "Terminal zbyt wąski — rozszerz okno do min. 40 znaków."
	}

	contentW := min(m.width-2, 90)

	// Jeśli ekran potwierdzenia przełączenia gałęzi
	if m.showBranchSwitch {
		return m.renderBranchSwitch(contentW)
	}

	sections := []string{
		m.renderHeader(contentW),
		m.renderTabs(contentW),
	}

	switch m.tab {
	case ApolloTabOverview:
		sections = append(sections, m.renderOverview(contentW))
	case ApolloTabHistory:
		sections = append(sections, m.renderHistory(contentW))
	}

	sections = append(sections, m.renderKeyBindings(contentW))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renderuje nagłówek Apollo.
func (m ApolloModel) renderHeader(width int) string {
	apolloStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF79C6"))

	apolloTitle := apolloStyle.Padding(0, 1).Render("🚀 Apollo")
	subtitle := StyleMuted.Render("— Panel Wdrożeń  (Q = wróć do Dashboard)")

	// Baner informacyjny — tryb z guardami
	guardBanner := lipgloss.NewStyle().
		Background(lipgloss.Color("#1e1e3f")).
		Foreground(lipgloss.Color("#FF79C6")).
		Width(width).
		Padding(0, 2).
		Render("⚡ Tryb Apollo: wdrożenia wyłącznie z gałęzi roboczych (master/develop) · wszystkie operacje są chronione strażnikami")

	return apolloTitle + " " + subtitle + "\n" + guardBanner + "\n" + Divider(width)
}

// renderTabs renderuje belkę zakładek.
func (m ApolloModel) renderTabs(width int) string {
	tabs := []struct {
		idx   ApolloTab
		label string
	}{
		{ApolloTabOverview, "1 PRZEGLĄD"},
		{ApolloTabHistory, "2 HISTORIA"},
	}

	var parts []string
	for _, t := range tabs {
		label := " " + t.label + " "
		if m.tab == t.idx {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(lipgloss.Color("#FF79C6")).
				Bold(true).
				Render(label))
		} else {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(ColorSubtext).
				Background(ColorSurface).
				Render(label))
		}
	}

	return lipgloss.NewStyle().Padding(0, 1).Render(strings.Join(parts, " ")) + "\n"
}

// renderOverview renderuje zakładkę Przegląd (strażnicy + środowiska + akcje).
func (m ApolloModel) renderOverview(width int) string {
	var sections []string

	// ── Selektor środowisk ────────────────────────────────────────────────
	sections = append(sections, m.renderEnvSelector(width))

	// ── Strażnicy ─────────────────────────────────────────────────────────
	sections = append(sections, m.renderGuards(width))

	// ── Komunikat statusu ─────────────────────────────────────────────────
	if m.statusMsg != "" {
		msgStyle := lipgloss.NewStyle().Padding(0, 2)
		if m.statusIsErr {
			sections = append(sections, msgStyle.Render(StyleError.Render(m.statusMsg)))
		} else {
			sections = append(sections, msgStyle.Render(StyleInfo.Render(m.statusMsg)))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderEnvSelector renderuje selektor środowisk w stylu Apollo.
func (m ApolloModel) renderEnvSelector(width int) string {
	title := StyleLabel.Padding(0, 2).Render("🌍 Środowisko docelowe:")

	var rows []string
	for i, env := range m.envNames {
		envCfg, ok := m.cfg.Environments[env]
		if !ok {
			continue
		}

		isSelected := i == m.selectedEnv
		prefix := "    ○  "
		if isSelected {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6")).Render("  ▶ ●  ")
		}

		badge := EnvBadge(env)
		branch := envCfg.Branch
		if branch == "" {
			branch = "(auto)"
		}
		info := StyleMuted.Render("  ⎇ " + branch)

		var lastDeploy string
		if m.stateData != nil {
			if last := m.stateData.GetLastSuccessful(env); last != nil {
				lastDeploy = StyleMuted.Render(" · deploy: " + last.StartedAt.Format("02.01 15:04"))
			}
		}

		var row string
		if isSelected {
			row = lipgloss.NewStyle().
				Background(lipgloss.Color("#2A2A3E")).
				Width(width - 2).
				Render(prefix + badge + info + lastDeploy)
		} else {
			row = prefix + badge + info + lastDeploy
		}
		rows = append(rows, row)
	}

	return "\n" + title + "\n" + strings.Join(rows, "\n") + "\n"
}

// renderGuards renderuje listę strażników dla wybranego środowiska.
func (m ApolloModel) renderGuards(width int) string {
	title := StyleLabel.Padding(0, 2).Render("🛡  Strażnicy wdrożenia:")

	if len(m.guards) == 0 {
		return "\n" + title + "\n" + StyleMuted.Padding(0, 4).Render("Brak środowiska — brak strażników") + "\n"
	}

	var rows []string
	allOK := true
	for _, g := range m.guards {
		// Skuteczny status z uwzględnieniem forceRedeploy
		effectivePass := g.Pass
		if m.forceRedeploy && g.Name == "Nowe commity" && !g.Pass {
			effectivePass = true // ignorujemy w trybie force
		}

		var icon, reasonStr string
		if effectivePass {
			icon = StyleSuccess.Render("  ✓")
		} else if g.Level == GuardLevelWarn {
			icon = StyleWarning.Render("  ⚠")
			allOK = false
		} else {
			icon = StyleError.Render("  ✗")
			allOK = false
		}

		nameStr := lipgloss.NewStyle().
			Foreground(ColorSubtext).
			Width(22).
			Render(g.Name)

		reasonStr = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render(g.Reason)

		row := icon + "  " + nameStr + "  " + reasonStr
		rows = append(rows, row)

		// Pokaż wskazówkę dla niespełnionych strażników
		if !effectivePass && g.Hint != "" {
			hintStr := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6272A4")).
				Padding(0, 8).
				Render("↳ " + g.Hint)
			rows = append(rows, hintStr)
		}
	}

	// Podsumowanie
	var summaryStr string
	if allOK || (m.forceRedeploy && len(m.effectiveBlockingGuards()) == 0) {
		summaryStr = "\n  " + StyleSuccess.Render("✅ Wszystkie strażniki zaliczone — wdrożenie gotowe!")
		if m.forceRedeploy {
			summaryStr += "  " + StyleWarning.Render("[FORCE]")
		}
	} else {
		blocked := BlockingGuards(m.guards)
		// Odejmij strażnik gałęzi (obsługiwany przez S)
		realBlocked := 0
		branchBlocked := false
		for _, g := range blocked {
			if g.Name == "Gałąź robocza" {
				branchBlocked = true
			} else {
				realBlocked++
			}
		}

		if branchBlocked && realBlocked == 0 {
			summaryStr = "\n  " + StyleWarning.Render("⚠  Gałąź nieprawidłowa — naciśnij S aby przełączyć")
		} else {
			summaryStr = "\n  " + StyleError.Render(fmt.Sprintf("❌ %d strażnik(ów) blokuje wdrożenie", len(blocked)))
		}
	}

	return "\n" + title + "\n" + strings.Join(rows, "\n") + summaryStr + "\n"
}

// renderHistory renderuje zakładkę Historia.
func (m ApolloModel) renderHistory(width int) string {
	records := m.envDeployRecords()
	env := m.selectedEnvName()

	title := StyleLabel.Padding(0, 2).Render(fmt.Sprintf("📋 Historia wdrożeń: %s", env))

	if len(records) == 0 {
		return "\n" + title + "\n" + StyleMuted.Padding(0, 4).Render("Brak historii wdrożeń dla tego środowiska.") + "\n"
	}

	// Nagłówek tabeli
	colStatus := lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(4).Render("St.")
	colDate := lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(14).Render("Data")
	colHash := lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(8).Render("Commit")
	colAuthor := lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(18).Render("Autor")
	colMsg := lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Render("Wiadomość")
	header := "  " + lipgloss.JoinHorizontal(lipgloss.Top, colStatus, colDate, colHash, colAuthor, colMsg)

	var rows []string
	rows = append(rows, header)
	rows = append(rows, StyleDivider.Render(repeatChar("─", width-4)))

	maxRows := min(len(records), m.height-16)
	for i, d := range records {
		if i >= maxRows {
			rows = append(rows, StyleMuted.Padding(0, 2).
				Render(fmt.Sprintf("... i %d więcej", len(records)-maxRows)))
			break
		}

		selected := i == m.historyCursor

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

		shortHash := d.CommitHash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}
		author := truncateStr(d.CommitAuthor, 16)
		msg := truncateStr(d.CommitMessage, width-52)

		stCol := lipgloss.NewStyle().Width(4).Render(statusIcon)
		dtCol := lipgloss.NewStyle().Width(14).Render(d.StartedAt.Format("02.01 15:04:05"))
		hcol := lipgloss.NewStyle().Width(8).Foreground(ColorInfo).Render(shortHash)
		auCol := lipgloss.NewStyle().Width(18).Render(author)
		msgCol := lipgloss.NewStyle().Render(msg)

		row := "  " + lipgloss.JoinHorizontal(lipgloss.Top, stCol, dtCol, hcol, auCol, msgCol)
		if selected {
			row = lipgloss.NewStyle().
				Background(lipgloss.Color("#2A2A3E")).
				Width(width - 2).
				Render(row)
		}
		rows = append(rows, row)
	}

	return "\n" + title + "\n" + strings.Join(rows, "\n") + "\n"
}

// renderBranchSwitch renderuje ekran potwierdzenia przełączenia gałęzi.
func (m ApolloModel) renderBranchSwitch(width int) string {
	current := ""
	if m.gitStatus != nil {
		current = m.gitStatus.Branch
	}

	warningStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorWarning)
	infoStyle := lipgloss.NewStyle().Foreground(ColorText).Width(width - 6)

	title := warningStyle.Render("⎇  Przełączenie gałęzi")
	msg := infoStyle.Render(fmt.Sprintf(
		"Apollo wymaga gałęzi '%s' dla wybranego środowiska.\n\n"+
			"Aktualna gałąź: %s\n"+
			"Docelowa gałąź: %s\n\n"+
			"Apollo wykona: git checkout %s\n\n"+
			"Upewnij się że masz zatwierdzone wszystkie zmiany.",
		m.targetBranch, current, m.targetBranch, m.targetBranch,
	))

	buttons := lipgloss.JoinHorizontal(lipgloss.Top,
		StyleButton.Render(" ENTER / Y = Przełącz "),
		"  ",
		StyleButtonSecondary.Render(" ESC / N = Anuluj "),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		"", title, "", msg, "", buttons, "",
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		StylePanelError.Width(min(width, 60)).Render(content))
}

// renderKeyBindings renderuje pasek skrótów klawiszowych Apollo.
func (m ApolloModel) renderKeyBindings(width int) string {
	var bindings []string

	env := m.selectedEnvName()

	// Sprawdź czy guard gałęzi jest niespełniony
	branchBlocked := false
	for _, g := range m.guards {
		if g.Name == "Gałąź robocza" && !g.Pass {
			branchBlocked = true
		}
	}

	if m.readyToDeploy() || m.forceRedeploy {
		bindings = append(bindings, keyBind("D", "Wdróż "+env))
	} else {
		if branchBlocked {
			bindings = append(bindings, keyBind("S", "Przełącz gałąź"))
		}
		bindings = append(bindings, keyBind("D", "Wdróż*"))
	}

	bindings = append(bindings,
		keyBind("R", "Rollback"),
		keyBind("P", "Promote DB"),
	)

	if m.forceRedeploy {
		bindings = append(bindings, keyBind("F", "[FORCE]"))
	} else {
		bindings = append(bindings, keyBind("F", "Wymuś"))
	}

	bindings = append(bindings,
		keyBind("1/2", "Zakładki"),
		keyBind("↑↓", "Środowisko"),
		keyBind("Q", "Wróć"),
	)

	divider := Divider(width)
	row := strings.Join(bindings, "  ")

	return "\n" + divider + "\n" + lipgloss.NewStyle().Padding(0, 2).Render(row) + "\n"
}
