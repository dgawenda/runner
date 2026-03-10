// file: pkg/tui/gitpanel.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Git Panel — Interaktywna Kontrola Repozytorium                    ║
// ║                                                                      ║
// ║  Pozwala bezpośrednio z TUI:                                        ║
// ║    · [1] STATUS  — podgląd zmian + stage all + commit               ║
// ║    · [2] GAŁĘZIE — lista lokalnych gałęzi + checkout                ║
// ║    · [3] HISTORIA — ostatnie 30 commitów bieżącej gałęzi           ║
// ║                                                                      ║
// ║  Panel odświeżany jest automatycznie co 3s — zmiany w repo        ║
// ║  (nowe commity, zmiany plików) pojawiają się na bieżąco.           ║
// ╚══════════════════════════════════════════════════════════════════════╝

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/neution/rnr/pkg/gitops"
)

// ─── Zakładki Git Panelu ──────────────────────────────────────────────────

// GitPanelTab to zakładka aktywna w Git Panelu.
type GitPanelTab int

const (
	GitTabStatus   GitPanelTab = iota // Zmiany + commit
	GitTabBranches                    // Lista gałęzi + checkout
	GitTabHistory                     // Historia commitów
)

// ─── Model ────────────────────────────────────────────────────────────────

// GitPanelModel obsługuje panel git — podgląd, commit i checkout.
type GitPanelModel struct {
	width  int
	height int

	// Aktywna zakładka
	tab GitPanelTab

	// Dane git (aktualizowane przez root model z pollingu)
	gitStatus *gitops.StatusResult
	branches  []string
	history   []gitops.CommitInfo

	// Status tab — input wiadomości commita
	commitInput textinput.Model

	// Nawigacja w zakładkach
	selectedBranch  int
	selectedHistory int

	// Feedback dla użytkownika
	statusMsg string
	statusErr bool
	loading   bool // trwa operacja (checkout/commit)
}

// NewGitPanelModel tworzy nowy model Git Panelu.
func NewGitPanelModel(width, height int) GitPanelModel {
	ti := textinput.New()
	ti.Placeholder = "Wiadomość commita (np. feat: dodaj funkcję)..."
	ti.CharLimit = 200
	ti.Width = min(width-6, 70)
	ti.Focus()

	return GitPanelModel{
		width:       width,
		height:      height,
		tab:         GitTabStatus,
		commitInput: ti,
	}
}

// ─── Interfejs Bubble Tea ─────────────────────────────────────────────────

// Init inicjalizuje model — brak działań (inicjalizacja w root model).
func (m GitPanelModel) Init() tea.Cmd { return nil }

// Update obsługuje zdarzenia dla Git Panelu.
func (m GitPanelModel) Update(msg tea.Msg) (GitPanelModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ── Aktualizacje danych ────────────────────────────────────────────
	case GitStatusMsg:
		if msg.Err == nil {
			m.gitStatus = msg.Result
			// Jeśli jesteśmy na status tab i commit się zakończył — czyść loading
		}

	case GitBranchesLoadedMsg:
		if msg.Err == nil {
			m.branches = msg.Branches
		}

	case GitHistoryLoadedMsg:
		if msg.Err == nil {
			m.history = msg.Commits
		}

	case GitCheckoutDoneMsg:
		m.loading = false
		if msg.Err != nil {
			m.statusMsg = "✗ Checkout nieudany: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = fmt.Sprintf("✓ Przełączono na gałąź: %s", msg.Branch)
			m.statusErr = false
		}

	case GitCommitDoneMsg:
		m.loading = false
		if msg.Err != nil {
			m.statusMsg = "✗ Commit nieudany: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = fmt.Sprintf("✓ Commit zatwierdzony: %s", msg.Hash)
			m.statusErr = false
			m.commitInput.SetValue("") // czyść po sukcesie
		}

	// ── Klawiatura ────────────────────────────────────────────────────
	case tea.KeyMsg:
		switch msg.String() {

		// Przełączanie zakładek
		case "1":
			m.tab = GitTabStatus
			m.commitInput.Width = min(m.width-6, 70)
			m.commitInput.Focus()

		case "2":
			m.tab = GitTabBranches
			m.commitInput.Blur()

		case "3":
			m.tab = GitTabHistory
			m.commitInput.Blur()

		case "tab":
			m.tab = (m.tab + 1) % 3
			if m.tab == GitTabStatus {
				m.commitInput.Focus()
			} else {
				m.commitInput.Blur()
			}

		// Nawigacja ↑↓
		case "up", "k":
			switch m.tab {
			case GitTabBranches:
				if m.selectedBranch > 0 {
					m.selectedBranch--
				}
			case GitTabHistory:
				if m.selectedHistory > 0 {
					m.selectedHistory--
				}
			}

		case "down", "j":
			switch m.tab {
			case GitTabBranches:
				if m.selectedBranch < len(m.branches)-1 {
					m.selectedBranch++
				}
			case GitTabHistory:
				if m.selectedHistory < len(m.history)-1 {
					m.selectedHistory++
				}
			}

		// Akcja ENTER
		case "enter":
			switch m.tab {

			case GitTabBranches:
				// Checkout wybranej gałęzi
				if m.loading || len(m.branches) == 0 {
					break
				}
				branch := m.branches[m.selectedBranch]
				// Nie checkout na bieżącą gałąź
				if m.gitStatus != nil && branch == m.gitStatus.Branch {
					m.statusMsg = fmt.Sprintf("✓ Już jesteś na gałęzi: %s", branch)
					m.statusErr = false
					break
				}
				m.loading = true
				m.statusMsg = ""
				return m, func() tea.Msg {
					return GitCheckoutRequestMsg{Branch: branch}
				}

			case GitTabStatus:
				// Stage all + commit
				if m.loading {
					break
				}
				if m.gitStatus == nil || m.gitStatus.IsClean {
					m.statusMsg = "ℹ️  Brak zmian do zatwierdzenia"
					m.statusErr = false
					break
				}
				commitMsg := strings.TrimSpace(m.commitInput.Value())
				if commitMsg == "" {
					m.statusMsg = "⚠ Wpisz wiadomość commita"
					m.statusErr = true
					break
				}
				m.loading = true
				m.statusMsg = ""
				return m, func() tea.Msg {
					return GitCommitRequestMsg{Message: commitMsg}
				}
			}
		}

		// Przekaż zdarzenie klawiaturowe do inputa na zakładce Status
		if m.tab == GitTabStatus {
			var cmd tea.Cmd
			m.commitInput, cmd = m.commitInput.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.commitInput.Width = min(m.width-6, 70)
	}

	return m, tea.Batch(cmds...)
}

// ─── Widok ────────────────────────────────────────────────────────────────

// View renderuje Git Panel.
func (m GitPanelModel) View() string {
	contentW := min(m.width-2, 92)

	var parts []string
	parts = append(parts, m.renderHeader(contentW))
	parts = append(parts, m.renderTabs(contentW))
	parts = append(parts, m.renderContent(contentW))
	if m.statusMsg != "" {
		parts = append(parts, m.renderStatusBar())
	}
	parts = append(parts, m.renderKeyBindings(contentW))

	return strings.Join(parts, "")
}

// ─── Sekcje Widoku ────────────────────────────────────────────────────────

func (m GitPanelModel) renderHeader(width int) string {
	branch := "n/a"
	isGit := false
	if m.gitStatus != nil {
		branch = m.gitStatus.Branch
		isGit = m.gitStatus.IsGitRepo
	}

	title := StyleTitle.Render("⎇  Git Panel")

	var branchInfo, statusDot string
	if isGit {
		branchInfo = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render("  " + branch)
		if m.gitStatus != nil && m.gitStatus.IsClean {
			statusDot = StyleSuccess.Render("  ●  czyste")
		} else if m.gitStatus != nil {
			statusDot = StyleWarning.Render(fmt.Sprintf("  ●  %d niezatwierdzonych", len(m.gitStatus.DirtyFiles)))
		}
	} else {
		branchInfo = StyleMuted.Render("  (brak git)")
	}

	// Autorefresh indicator
	refreshInfo := StyleMuted.Render("  ⟳ live")

	header := lipgloss.NewStyle().Width(width).Padding(0, 1).
		Render(title + branchInfo + statusDot + refreshInfo)
	return header + "\n" + Divider(width) + "\n"
}

func (m GitPanelModel) renderTabs(width int) string {
	tabs := []struct {
		label string
		tab   GitPanelTab
	}{
		{"[1] Status", GitTabStatus},
		{"[2] Gałęzie", GitTabBranches},
		{"[3] Historia", GitTabHistory},
	}

	var rendered []string
	for _, t := range tabs {
		if t.tab == m.tab {
			rendered = append(rendered,
				lipgloss.NewStyle().
					Foreground(ColorPrimary).
					Bold(true).
					Underline(true).
					Render(t.label),
			)
		} else {
			rendered = append(rendered, StyleMuted.Render(t.label))
		}
	}
	_ = width
	return "  " + strings.Join(rendered, "  │  ") + "\n\n"
}

func (m GitPanelModel) renderContent(width int) string {
	switch m.tab {
	case GitTabStatus:
		return m.renderStatusTab(width)
	case GitTabBranches:
		return m.renderBranchesTab(width)
	case GitTabHistory:
		return m.renderHistoryTab(width)
	}
	return ""
}

// renderStatusTab wyświetla zmiany w repo i formularz commita.
func (m GitPanelModel) renderStatusTab(width int) string {
	if m.gitStatus == nil {
		return "  " + StyleInfo.Render("⟳ Ładowanie statusu Git...") + "\n"
	}
	if !m.gitStatus.IsGitRepo {
		return "  " + StyleMuted.Render("Ten katalog nie jest repozytorium Git.") + "\n"
	}

	var lines []string

	if m.gitStatus.IsClean {
		// ── Czyste repozytorium ──────────────────────────────────────
		lines = append(lines,
			"  "+StyleSuccess.Render("✅ Repozytorium czyste — brak zmian do zatwierdzenia"),
			"",
		)
		if m.gitStatus.LastCommit.Hash != "" {
			lines = append(lines,
				"  "+StyleLabel.Render("Ostatni commit:"),
				fmt.Sprintf("  %s  %s  %s",
					StyleMuted.Render(m.gitStatus.LastCommit.ShortHash),
					StyleValue.Render(truncateStr(m.gitStatus.LastCommit.Message, width-35)),
					StyleMuted.Render(m.gitStatus.LastCommit.RelativeDate),
				),
				"  "+StyleMuted.Render("  autor: "+m.gitStatus.LastCommit.Author),
			)
		}
	} else {
		// ── Niezatwierdzone zmiany ────────────────────────────────────
		lines = append(lines,
			"  "+StyleWarning.Render(fmt.Sprintf("⚠  %d niezatwierdzonych zmian:", len(m.gitStatus.DirtyFiles))),
			"",
		)

		maxFiles := 12
		for i, f := range m.gitStatus.DirtyFiles {
			if i >= maxFiles {
				lines = append(lines,
					StyleMuted.Render(fmt.Sprintf("    ... i %d więcej plików", len(m.gitStatus.DirtyFiles)-maxFiles)),
				)
				break
			}
			icon, iconStyle := gitFileIcon(f.Status)
			lines = append(lines,
				fmt.Sprintf("    %s %s",
					iconStyle.Render(fmt.Sprintf("[%s]", strings.TrimSpace(f.Status))),
					icon+" "+StyleValue.Render(f.Path),
				),
			)
		}

		lines = append(lines, "", "  "+StyleLabel.Render("Wiadomość commita:"))
		lines = append(lines, "  "+m.commitInput.View())
		lines = append(lines, "")

		if m.loading {
			lines = append(lines, "  "+StyleInfo.Render("⟳ Trwa commit (git add -A && git commit)..."))
		} else {
			lines = append(lines, "  "+StyleMuted.Render("[ENTER] Zatwierdź wszystkie zmiany (git add -A && git commit)"))
		}
	}

	return strings.Join(lines, "\n") + "\n"
}

// renderBranchesTab wyświetla listę lokalnych gałęzi z możliwością checkout.
func (m GitPanelModel) renderBranchesTab(width int) string {
	if len(m.branches) == 0 {
		if m.loading {
			return "  " + StyleInfo.Render("⟳ Ładowanie gałęzi...") + "\n"
		}
		return "  " + StyleMuted.Render("Brak gałęzi lokalnych.") + "\n"
	}

	currentBranch := ""
	if m.gitStatus != nil {
		currentBranch = m.gitStatus.Branch
	}

	var lines []string
	for i, b := range m.branches {
		isCurrent := b == currentBranch
		isSelected := i == m.selectedBranch

		var line string
		if isCurrent {
			// Aktywna gałąź — zawsze zaznaczona gwiazdką
			line = fmt.Sprintf("  ★  %s%s",
				lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render(b),
				StyleMuted.Render("  (aktualna)"),
			)
		} else if isSelected {
			prefix := lipgloss.NewStyle().Foreground(ColorPrimary).Render("  ▶  ")
			name := lipgloss.NewStyle().Foreground(ColorPrimary).Render(b)
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("#2A2A3E")).
				Width(width - 4).
				Render(prefix + name)
		} else {
			line = "     " + StyleValue.Render(b)
		}
		lines = append(lines, line)
	}

	result := strings.Join(lines, "\n") + "\n\n"
	if m.loading {
		result += "  " + StyleInfo.Render("⟳ Trwa checkout...") + "\n"
	} else {
		result += "  " + StyleMuted.Render("[ENTER] Checkout zaznaczonej gałęzi  [↑↓ / j/k] Nawigacja") + "\n"
	}
	return result
}

// renderHistoryTab wyświetla historię ostatnich commitów.
func (m GitPanelModel) renderHistoryTab(width int) string {
	if len(m.history) == 0 {
		return "  " + StyleMuted.Render("Brak historii commitów.") + "\n"
	}

	var lines []string
	// Nagłówek kolumn
	lines = append(lines, fmt.Sprintf("  %s  %s  %s  %s",
		StyleLabel.Render("Hash   "),
		StyleLabel.Render("Kiedy      "),
		StyleLabel.Render("Autor           "),
		StyleLabel.Render("Wiadomość"),
	))
	lines = append(lines, "  "+StyleMuted.Render(strings.Repeat("─", min(width-4, 80))))

	for i, c := range m.history {
		isSelected := i == m.selectedHistory

		line := fmt.Sprintf("  %s  %s  %s  %s",
			lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("%-7s", c.ShortHash)),
			StyleMuted.Render(fmt.Sprintf("%-10s", truncateStr(c.RelativeDate, 10))),
			StyleMuted.Render(fmt.Sprintf("%-16s", truncateStr(c.Author, 16))),
			truncateStr(c.Message, width-50),
		)

		if isSelected {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("#2A2A3E")).
				Width(width - 2).
				Render(line)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n") + "\n"
}

func (m GitPanelModel) renderStatusBar() string {
	if m.statusMsg == "" {
		return ""
	}
	style := StyleSuccess
	if m.statusErr {
		style = StyleError
	}
	return "\n  " + style.Render(m.statusMsg) + "\n"
}

func (m GitPanelModel) renderKeyBindings(width int) string {
	var bindings []string

	switch m.tab {
	case GitTabStatus:
		if m.gitStatus != nil && !m.gitStatus.IsClean {
			bindings = append(bindings, keyBind("ENTER", "Stage All + Commit"))
		}
	case GitTabBranches:
		bindings = append(bindings,
			keyBind("ENTER", "Checkout"),
			keyBind("↑↓ / j/k", "Nawigacja"),
		)
	case GitTabHistory:
		bindings = append(bindings, keyBind("↑↓ / j/k", "Nawigacja"))
	}

	bindings = append(bindings,
		keyBind("TAB / 1/2/3", "Zakładki"),
		keyBind("G / Q", "Powrót do Dashboard"),
	)

	divider := Divider(width)
	row := strings.Join(bindings, "  ")
	return "\n" + divider + "\n" + lipgloss.NewStyle().Padding(0, 2).Render(row) + "\n"
}

// ─── Helpers ─────────────────────────────────────────────────────────────

// gitFileIcon zwraca ikonę i styl dla statusu pliku git (M, A, D, ??, itp.).
func gitFileIcon(status string) (string, lipgloss.Style) {
	s := strings.TrimSpace(status)
	switch {
	case strings.Contains(s, "M"):
		return "~", lipgloss.NewStyle().Foreground(ColorWarning)
	case strings.Contains(s, "A"):
		return "+", lipgloss.NewStyle().Foreground(ColorSuccess)
	case strings.Contains(s, "D"):
		return "−", lipgloss.NewStyle().Foreground(ColorError)
	case strings.Contains(s, "R"):
		return "→", lipgloss.NewStyle().Foreground(ColorInfo)
	case strings.Contains(s, "?"):
		return "?", lipgloss.NewStyle().Foreground(ColorSubtext)
	default:
		return "·", StyleMuted
	}
}
