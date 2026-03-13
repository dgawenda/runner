// file: pkg/tui/gitpanel.go
//
// ╔══════════════════════════════════════════════════════════════════════════╗
// ║  Git Panel — Wizualna Kontrola Repozytorium (styl GitKraken)           ║
// ║                                                                          ║
// ║  Zakładki:                                                               ║
// ║    [1] STATUS  — zmiany z checkboxami, diff, commit, push              ║
// ║    [2] GAŁĘZIE — lista lokalnych gałęzi + checkout                     ║
// ║    [3] HISTORIA — ostatnie 30 commitów                                  ║
// ║    [4] GRAF    — wizualny graf (styl GitKraken)                         ║
// ╚══════════════════════════════════════════════════════════════════════════╝

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/neution/rnr/pkg/gitops"
)

// ─── Kolory grafu i diff (Dracula) ───────────────────────────────────────

var (
	graphLineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	graphDotStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9")).Bold(true)
	graphHashStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	graphMsgStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))
	graphHEADStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Bold(true)
	graphBranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C"))
	graphTagStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))

	diffAddStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
	diffRemStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
	diffHunkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	diffHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))

	checkboxOnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Bold(true)
	checkboxOffStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
)

// ─── Zakładki ─────────────────────────────────────────────────────────────

type GitPanelTab int

const (
	GitTabStatus   GitPanelTab = iota
	GitTabBranches
	GitTabHistory
	GitTabGraph
)

// ─── Model ────────────────────────────────────────────────────────────────

type GitPanelModel struct {
	width  int
	height int

	tab GitPanelTab

	gitStatus *gitops.StatusResult
	branches  []string
	history   []gitops.CommitInfo
	gitGraph  []string

	// Status tab
	commitInput  textinput.Model
	selectedFile int
	stagedFiles  map[string]bool
	showDiff     bool
	diffLines    []string
	diffFile     string
	diffOffset   int

	lastCommitHash string
	pushAvailable  bool

	// Push conflict
	pushConflict       bool
	pushConflictBranch string

	// Branches tab
	selectedBranch int

	// History tab
	selectedHistory int

	// Graph tab
	selectedGraph int
	graphOffset   int

	// Feedback
	statusMsg string
	statusErr bool
	loading   bool

	// Auto commit message (ostatnio wygenerowany)
	autoCommitGenerated bool

	// Typ commita dla auto-opisu (feat/fix/refactor/chore)
	commitTypeIndex int
}

func NewGitPanelModel(width, height int) GitPanelModel {
	ti := textinput.New()
	ti.Placeholder = "wpisz wiadomość commita i naciśnij ENTER..."
	ti.CharLimit = 200
	ti.Width = min(width-6, 70)

	return GitPanelModel{
		width:       width,
		height:      height,
		tab:             GitTabStatus,
		commitInput:     ti,
		stagedFiles:     make(map[string]bool),
		commitTypeIndex: 0, // domyślnie "feat"
	}
}

// ─── Bubble Tea ───────────────────────────────────────────────────────────

func (m GitPanelModel) Init() tea.Cmd { return nil }

func (m GitPanelModel) Update(msg tea.Msg) (GitPanelModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case GitStatusMsg:
		if msg.Err == nil {
			m.gitStatus = msg.Result
			if m.gitStatus != nil && m.selectedFile >= len(m.gitStatus.DirtyFiles) {
				m.selectedFile = max(0, len(m.gitStatus.DirtyFiles)-1)
			}
			newPaths := make(map[string]bool)
			if m.gitStatus != nil {
				for _, f := range m.gitStatus.DirtyFiles {
					newPaths[f.Path] = true
				}
			}
			for path := range m.stagedFiles {
				if !newPaths[path] {
					delete(m.stagedFiles, path)
				}
			}
		}

	case GitBranchesLoadedMsg:
		if msg.Err == nil {
			m.branches = msg.Branches
		}

	case GitHistoryLoadedMsg:
		if msg.Err == nil {
			m.history = msg.Commits
		}

	case GitGraphLoadedMsg:
		if msg.Err == nil {
			m.gitGraph = msg.Lines
		}

	case GitDiffLoadedMsg:
		if msg.Err == nil {
			m.diffLines = msg.Lines
			m.diffFile = msg.File
			m.showDiff = true
			m.diffOffset = 0
		} else {
			m.statusMsg = "✗ Diff nieudany: " + msg.Err.Error()
			m.statusErr = true
		}

	case GitCheckoutDoneMsg:
		m.loading = false
		if msg.Err != nil {
			m.statusMsg = "✗ Checkout nieudany: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = fmt.Sprintf("✓ Przełączono na: %s", msg.Branch)
			m.statusErr = false
		}

	case GitCommitDoneMsg:
		m.loading = false
		if msg.Err != nil {
			m.statusMsg = "✗ Commit nieudany: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.lastCommitHash = msg.Hash
			m.statusMsg = fmt.Sprintf("✓ Commit: %s  —  [p] Push do remote", msg.Hash)
			m.statusErr = false
			m.commitInput.SetValue("")
			m.commitInput.Blur()
			m.showDiff = false
			m.stagedFiles = make(map[string]bool)
		}

	case GitPushDoneMsg:
		m.loading = false
		if msg.Err != nil {
			if msg.IsNonFastForward {
				m.pushConflict = true
				m.pushConflictBranch = msg.Branch
				m.statusMsg = "⚠  Push odrzucony — remote ma nowe commity"
				m.statusErr = true
			} else {
				m.pushConflict = false
				m.statusMsg = "✗ Push nieudany: " + msg.Err.Error()
				m.statusErr = true
			}
		} else {
			m.pushConflict = false
			m.statusMsg = fmt.Sprintf("✓ Wypchnięto gałąź %s → origin", msg.Branch)
			m.statusErr = false
			m.lastCommitHash = ""
		}

	case GitPullRebasePushDoneMsg:
		m.loading = false
		m.pushConflict = false
		if msg.Err != nil {
			m.statusMsg = "✗ Pull+rebase+push nieudany: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = fmt.Sprintf("✓ Pull+rebase+push OK: gałąź %s zsynchronizowana", msg.Branch)
			m.statusErr = false
			m.lastCommitHash = ""
		}

	case tea.KeyMsg:
		// Tryb konfliktu push — ESC
		if m.tab == GitTabStatus && m.pushConflict {
			switch msg.String() {
			case "esc", "q":
				m.pushConflict = false
				m.statusMsg = "Push anulowany"
				m.statusErr = false
			}
		}

		// Tryb diff
		if m.tab == GitTabStatus && m.showDiff {
			switch msg.String() {
			case "esc", "d", "q":
				m.showDiff = false
				m.diffLines = nil
			case "up", "k":
				if m.diffOffset > 0 {
					m.diffOffset--
				}
			case "down", "j":
				visH := m.diffVisibleLines()
				if m.diffOffset < len(m.diffLines)-visH {
					m.diffOffset++
				}
			case "pgup":
				m.diffOffset = max(0, m.diffOffset-10)
			case "pgdown":
				visH := m.diffVisibleLines()
				m.diffOffset = min(max(0, len(m.diffLines)-visH), m.diffOffset+10)
			}
			return m, tea.Batch(cmds...)
		}

		// Klawisz 'i' — focusuj commit input
		if m.tab == GitTabStatus && msg.String() == "i" && !m.commitInput.Focused() {
			m.commitInput.Focus()
			m.commitInput.Width = min(m.width-6, 70)
			return m, tea.Batch(cmds...)
		}

		// Commit input sfocusowany
		if m.tab == GitTabStatus && m.commitInput.Focused() {
			switch msg.String() {
			case "esc":
				m.commitInput.Blur()
				return m, tea.Batch(cmds...)
			case "enter":
				if !m.loading {
					commitMsg := strings.TrimSpace(m.commitInput.Value())
					if commitMsg == "" {
						m.statusMsg = "⚠  Wpisz wiadomość commita"
						m.statusErr = true
					} else if m.gitStatus == nil || m.gitStatus.IsClean {
						m.statusMsg = "ℹ  Brak zmian do zatwierdzenia"
						m.statusErr = false
					} else {
						m.loading = true
						m.statusMsg = ""
						filesToStage := m.selectedFilePaths()
						return m, func() tea.Msg {
							return GitCommitRequestMsg{Message: commitMsg, Files: filesToStage}
						}
					}
				}
				return m, tea.Batch(cmds...)
			default:
				var cmd tea.Cmd
				m.commitInput, cmd = m.commitInput.Update(msg)
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		}

		// ENTER z głównego widoku Status:
		// - jeśli commit input NIE jest sfocusowany, ale ma treść → spróbuj commit
		// - jeśli commit input pusty → sfocusuj pole, żeby user mógł wpisać opis
		if m.tab == GitTabStatus && !m.commitInput.Focused() && msg.String() == "enter" {
			if !m.loading {
				commitMsg := strings.TrimSpace(m.commitInput.Value())
				if commitMsg == "" {
					// Brak opisu — przełącz fokus na input
					m.commitInput.Focus()
					m.commitInput.Width = min(m.width-6, 70)
					m.statusMsg = "✎ Wpisz opis commita i naciśnij ENTER"
					m.statusErr = false
				} else if m.gitStatus == nil || m.gitStatus.IsClean {
					m.statusMsg = "ℹ  Brak zmian do zatwierdzenia"
					m.statusErr = false
				} else {
					m.loading = true
					m.statusMsg = ""
					filesToStage := m.selectedFilePaths()
					return m, func() tea.Msg {
						return GitCommitRequestMsg{Message: commitMsg, Files: filesToStage}
					}
				}
			}
			return m, tea.Batch(cmds...)
		}

		// Główny handler klawiszy
		switch msg.String() {
		case "1":
			m.tab = GitTabStatus
			m.showDiff = false
		case "2":
			m.tab = GitTabBranches
			m.commitInput.Blur()
		case "3":
			m.tab = GitTabHistory
			m.commitInput.Blur()
		case "4":
			m.tab = GitTabGraph
			m.commitInput.Blur()
		case "tab":
			m.tab = (m.tab + 1) % 4
			if m.tab != GitTabStatus {
				m.commitInput.Blur()
			}
			m.showDiff = false

		case "up", "k":
			switch m.tab {
			case GitTabStatus:
				if m.selectedFile > 0 {
					m.selectedFile--
				}
			case GitTabBranches:
				if m.selectedBranch > 0 {
					m.selectedBranch--
				}
			case GitTabHistory:
				if m.selectedHistory > 0 {
					m.selectedHistory--
				}
			case GitTabGraph:
				if m.selectedGraph > 0 {
					m.selectedGraph--
					if m.selectedGraph < m.graphOffset {
						m.graphOffset = m.selectedGraph
					}
				}
			}

		case "down", "j":
			switch m.tab {
			case GitTabStatus:
				if m.gitStatus != nil && m.selectedFile < len(m.gitStatus.DirtyFiles)-1 {
					m.selectedFile++
				}
			case GitTabBranches:
				if m.selectedBranch < len(m.branches)-1 {
					m.selectedBranch++
				}
			case GitTabHistory:
				if m.selectedHistory < len(m.history)-1 {
					m.selectedHistory++
				}
			case GitTabGraph:
				if m.selectedGraph < len(m.gitGraph)-1 {
					m.selectedGraph++
					visH := m.graphVisibleLines()
					if m.selectedGraph >= m.graphOffset+visH {
						m.graphOffset = m.selectedGraph - visH + 1
					}
				}
			}

		case " ":
			if m.tab == GitTabStatus && m.gitStatus != nil && !m.gitStatus.IsClean {
				if len(m.gitStatus.DirtyFiles) > 0 {
					file := m.gitStatus.DirtyFiles[m.selectedFile].Path
					m.stagedFiles[file] = !m.stagedFiles[file]
					m.statusMsg = ""
				}
			}

		case "a", "A":
			if m.tab == GitTabStatus && m.gitStatus != nil && !m.gitStatus.IsClean {
				allSelected := true
				for _, f := range m.gitStatus.DirtyFiles {
					if !m.stagedFiles[f.Path] {
						allSelected = false
						break
					}
				}
				if allSelected {
					m.stagedFiles = make(map[string]bool)
				} else {
					for _, f := range m.gitStatus.DirtyFiles {
						m.stagedFiles[f.Path] = true
					}
				}
				m.statusMsg = ""
			}

		case "u", "U":
			if m.tab == GitTabStatus && m.pushConflict && !m.loading {
				branch := m.pushConflictBranch
				m.loading = true
				m.pushConflict = false
				m.statusMsg = "↓ git pull --rebase + push..."
				m.statusErr = false
				return m, func() tea.Msg { return GitPullRebasePushRequestMsg{Branch: branch} }
			}

		case "f", "F":
			if m.tab == GitTabStatus && m.pushConflict && !m.loading {
				branch := m.pushConflictBranch
				m.loading = true
				m.pushConflict = false
				m.statusMsg = "⚡ git push --force-with-lease..."
				m.statusErr = false
				return m, func() tea.Msg { return GitForcePushRequestMsg{Branch: branch} }
			}

		case "p", "P":
			if m.tab == GitTabStatus && !m.loading {
				m.loading = true
				m.statusMsg = ""
				m.pushConflict = false
				return m, func() tea.Msg { return GitPushRequestMsg{} }
			}

		case "d":
			if m.tab == GitTabStatus && m.gitStatus != nil && !m.gitStatus.IsClean &&
				len(m.gitStatus.DirtyFiles) > 0 {
				file := m.gitStatus.DirtyFiles[m.selectedFile].Path
				m.statusMsg = ""
				return m, func() tea.Msg { return GitDiffRequestMsg{File: file} }
			}

		case "t", "T":
			// [t] — zmień typ commita dla auto-opisu
			if m.tab == GitTabStatus {
				m.commitTypeIndex = (m.commitTypeIndex + 1) % 4
				types := []string{"feat", "fix", "refactor", "chore"}
				ct := types[m.commitTypeIndex]
				m.statusMsg = fmt.Sprintf("Typ commita ustawiony na: %s", ct)
				m.statusErr = false
			}
			return m, tea.Batch(cmds...)

		case "m", "M":
			// [m] — wygeneruj automatyczną wiadomość commita na podstawie zmian
			if m.tab == GitTabStatus && m.gitStatus != nil && !m.gitStatus.IsClean {
				ctypes := []string{"feat", "fix", "refactor", "chore"}
				ct := ctypes[m.commitTypeIndex%len(ctypes)]

				gs := m.gitStatus
				branch := gs.Branch
				if branch == "" {
					branch = "HEAD"
				}

				author := gs.LastCommit.Author
				if author == "" {
					author = "unknown"
				}
				email := gs.LastCommit.AuthorEmail
				if email == "" {
					email = "unknown@example.com"
				}

				fileCount := len(gs.DirtyFiles)
				now := time.Now()

				msg := fmt.Sprintf(
					"%s(%s): %s <%s> — %d plik(ów) (%s)",
					ct,
					branch,
					author,
					email,
					fileCount,
					now.Format("2006-01-02 15:04"),
				)

				m.commitInput.SetValue(msg)
				m.autoCommitGenerated = true
				m.statusMsg = "✎ Wygenerowano automatyczny opis commita — możesz go edytować przed ENTER"
				m.statusErr = false
			}
			// nie wykonuj dalej żadnych akcji (np. diff)
			return m, tea.Batch(cmds...)

		case "enter":
			switch m.tab {
			case GitTabStatus:
				if m.gitStatus != nil && !m.gitStatus.IsClean && len(m.gitStatus.DirtyFiles) > 0 {
					file := m.gitStatus.DirtyFiles[m.selectedFile].Path
					m.statusMsg = ""
					return m, func() tea.Msg { return GitDiffRequestMsg{File: file} }
				}
			case GitTabBranches:
				if m.loading || len(m.branches) == 0 {
					break
				}
				branch := m.branches[m.selectedBranch]
				if m.gitStatus != nil && branch == m.gitStatus.Branch {
					m.statusMsg = fmt.Sprintf("✓ Już jesteś na: %s", branch)
					m.statusErr = false
					break
				}
				m.loading = true
				m.statusMsg = ""
				return m, func() tea.Msg { return GitCheckoutRequestMsg{Branch: branch} }
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.commitInput.Width = min(m.width-6, 70)
	}

	return m, tea.Batch(cmds...)
}

func (m GitPanelModel) selectedFilePaths() []string {
	if len(m.stagedFiles) == 0 {
		return nil
	}
	var paths []string
	for path, checked := range m.stagedFiles {
		if checked {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	return paths
}

// ─── Widok ────────────────────────────────────────────────────────────────

func (m GitPanelModel) View() string {
	w := min(m.width, 96)

	var sb strings.Builder

	sb.WriteString(m.renderTopBar(w))
	sb.WriteString("\n")
	sb.WriteString(m.renderTabs(w))
	sb.WriteString("\n")
	sb.WriteString(m.renderContent(w))
	if m.statusMsg != "" {
		sb.WriteString(m.renderStatusBar())
	}
	sb.WriteString("\n")
	sb.WriteString(m.renderKeyBar(w))

	return sb.String()
}

// ─── Top bar ─────────────────────────────────────────────────────────────

func (m GitPanelModel) renderTopBar(w int) string {
	gitLabel := lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorSecondary).
		Bold(true).Padding(0, 2).Render("🔧 GitPanel")

	branch := "n/a"
	if m.gitStatus != nil {
		branch = m.gitStatus.Branch
	}
	branchBadge := lipgloss.NewStyle().
		Foreground(ColorBg).Background(lipgloss.Color("#44475A")).
		Padding(0, 1).Bold(true).Render("⎇  " + branch)

	var statusBadge string
	if m.gitStatus == nil {
		statusBadge = lipgloss.NewStyle().Foreground(ColorMuted).Render("⟳ ładowanie...")
	} else if m.gitStatus.IsClean {
		statusBadge = lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorSuccess).
			Padding(0, 1).Render("✓ czyste")
	} else {
		total := len(m.gitStatus.DirtyFiles)
		checked := m.countChecked()
		var txt string
		if checked > 0 {
			txt = fmt.Sprintf("✓ %d/%d zaznaczono", checked, total)
		} else {
			txt = fmt.Sprintf("⚠ %d zmian", total)
		}
		statusBadge = lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorWarning).
			Padding(0, 1).Render(txt)
	}

	refreshStr := lipgloss.NewStyle().Foreground(ColorMuted).Render("  ⟳ live")

	topLine := lipgloss.NewStyle().Width(w).Background(ColorBgAlt).Padding(0, 1).
		Render(gitLabel + "  " + branchBadge + "  " + statusBadge + refreshStr)
	divLine := lipgloss.NewStyle().Foreground(ColorSecondary).Render(repeatChar("━", w))

	return topLine + "\n" + divLine
}

// ─── Tabs (TYLKO raz) ────────────────────────────────────────────────────

func (m GitPanelModel) renderTabs(w int) string {
	labels := []string{"1 STATUS", "2 GAŁĘZIE", "3 HISTORIA", "4 GRAF ◈"}
	bar := RenderTabs(labels, int(m.tab), ColorSecondary)
	return bar
}

func (m GitPanelModel) renderContent(w int) string {
	switch m.tab {
	case GitTabStatus:
		return m.renderStatusTab(w)
	case GitTabBranches:
		return m.renderBranchesTab(w)
	case GitTabHistory:
		return m.renderHistoryTab(w)
	case GitTabGraph:
		return m.renderGraphTab(w)
	}
	return ""
}

// ─── [1] Status Tab ───────────────────────────────────────────────────────

func (m GitPanelModel) renderStatusTab(w int) string {
	if m.gitStatus == nil {
		return "\n  " + StyleInfo.Render("⟳ Ładowanie statusu Git...") + "\n"
	}
	if !m.gitStatus.IsGitRepo {
		return "\n  " + StyleMuted.Render("Ten katalog nie jest repozytorium Git.") + "\n"
	}

	if m.showDiff {
		return m.renderDiffView(w)
	}
	if m.pushConflict {
		return m.renderPushConflict(w)
	}

	var rows []string
	rows = append(rows, "")

	// Remote
	if m.gitStatus.HasRemote {
		remoteShort := m.gitStatus.RemoteURL
		if len(remoteShort) > 55 {
			remoteShort = "…" + remoteShort[len(remoteShort)-52:]
		}
		rows = append(rows, "  "+lipgloss.NewStyle().Foreground(ColorMuted).Render("🔗 "+remoteShort))

		// Informacje ahead/behind względem upstream
		if m.gitStatus.HasUpstream {
			a := m.gitStatus.Ahead
			b := m.gitStatus.Behind
			var syncLine string
			switch {
			case a == 0 && b == 0:
				syncLine = lipgloss.NewStyle().Foreground(ColorSuccess).Render("⮃ Gałąź zsynchronizowana z origin")
			default:
				var parts []string
				if a > 0 {
					parts = append(parts, lipgloss.NewStyle().Foreground(ColorSuccess).Render(fmt.Sprintf("↑ %d lokalnych", a)))
				}
				if b > 0 {
					parts = append(parts, lipgloss.NewStyle().Foreground(ColorWarning).Render(fmt.Sprintf("↓ %d do pobrania", b)))
				}
				syncLine = lipgloss.NewStyle().Foreground(ColorMuted).Render("⮃ " + strings.Join(parts, "  "))
			}
			rows = append(rows, "  "+syncLine)
		} else {
			rows = append(rows,
				"  "+lipgloss.NewStyle().Foreground(ColorMuted).
					Render("⮃ Brak upstream — pierwszy push ustawi śledzenie (origin/"+m.gitStatus.Branch+")"),
			)
		}
	} else {
		rows = append(rows,
			"  "+lipgloss.NewStyle().Foreground(ColorBg).Background(ColorWarning).Padding(0, 1).
				Render("⚠ Brak remote origin — push niedostępny"),
			"  "+StyleMuted.Render("  Dodaj: git remote add origin <URL>"),
		)
	}
	rows = append(rows, "")

	if m.gitStatus.IsClean {
		rows = append(rows,
			"  "+lipgloss.NewStyle().Foreground(ColorBg).Background(ColorSuccess).
				Padding(0, 1).Bold(true).Render("✅  Repozytorium czyste — brak zmian"),
		)
		if m.gitStatus.LastCommit.Hash != "" {
			rows = append(rows, "")
			rows = append(rows, "  "+SectionHeader("📌", "Ostatni commit:", w-4))
			rows = append(rows, fmt.Sprintf("  %s  %s  %s",
				lipgloss.NewStyle().Foreground(ColorMuted).Render(m.gitStatus.LastCommit.ShortHash),
				lipgloss.NewStyle().Foreground(ColorText).Render(truncateStr(m.gitStatus.LastCommit.Message, w-40)),
				lipgloss.NewStyle().Foreground(ColorMuted).Render(m.gitStatus.LastCommit.RelativeDate),
			))
			rows = append(rows, "  "+lipgloss.NewStyle().Foreground(ColorMuted).
				Render("autor: "+m.gitStatus.LastCommit.Author))
		}
		if !m.loading && m.gitStatus.HasRemote {
			rows = append(rows, "", "  "+StyleMuted.Render("[p] Push do remote"))
		}
	} else {
		checkedCount := m.countChecked()
		total := len(m.gitStatus.DirtyFiles)

		// Nagłówek listy
		rows = append(rows, "  "+SectionHeader("📝", fmt.Sprintf("%d niezatwierdzonych zmian:", total), w-4))
		rows = append(rows, "")

		// Legenda (dostosowana do stanu)
		if checkedCount > 0 {
			legendBadge := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorSuccess).
				Padding(0, 1).Render(fmt.Sprintf("✓ %d zaznaczono", checkedCount))
			rows = append(rows, "  "+legendBadge+
				lipgloss.NewStyle().Foreground(ColorMuted).Render("  SPACJA odznacz  [a] reset  [i] commit"))
		} else {
			rows = append(rows, "  "+lipgloss.NewStyle().Foreground(ColorMuted).
				Render("↑↓ wybierz  SPACJA zaznacz  [a] wszystkie  [d] diff  [i] commit"))
		}
		rows = append(rows, "")

		// Lista plików
		maxFiles := 14
		for i, f := range m.gitStatus.DirtyFiles {
			if i >= maxFiles {
				rows = append(rows, "    "+StyleMuted.Render(fmt.Sprintf("... i %d więcej", total-maxFiles)))
				break
			}

			icon, iconStyle := gitFileIcon(f.Status)
			isSelected := i == m.selectedFile
			isChecked := m.stagedFiles[f.Path]

			var cb string
			if isChecked {
				cb = checkboxOnStyle.Render("[✓]")
			} else {
				cb = checkboxOffStyle.Render("[ ]")
			}

			statusTag := iconStyle.Render(fmt.Sprintf("[%s]", strings.TrimSpace(f.Status)))
			pathStr := lipgloss.NewStyle().Foreground(ColorText).Render(f.Path)

			line := fmt.Sprintf("  %s %s %s  %s", cb, statusTag, icon, pathStr)

			if isSelected {
				arrow := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render("▶")
				line = " " + arrow + " " + cb + " " + statusTag + " " + icon + "  " + pathStr
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#1A2433")).
					Width(w - 2).
					Render(line)
			}
			rows = append(rows, line)
		}

		// Podsumowanie zaznaczenia
		rows = append(rows, "")
		if checkedCount == 0 {
			rows = append(rows, "  "+StyleMuted.Render("(brak zaznaczonych → commit obejmie WSZYSTKIE pliki)"))
		} else {
			rows = append(rows, "  "+checkboxOnStyle.Render(fmt.Sprintf(
				"Zaznaczono: %d/%d — commit obejmie tylko zaznaczone", checkedCount, total,
			)))
		}

		// Formularz commita
		rows = append(rows, "")
		if m.commitInput.Focused() {
			rows = append(rows, "  "+lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render("✎  Wiadomość commita [ESC anuluj]:"))
		} else {
			commitTypes := []string{"feat", "fix", "refactor", "chore"}
			ct := commitTypes[m.commitTypeIndex%len(commitTypes)]
			hints := fmt.Sprintf("[i] Napisz commit    [m] Auto opis    [t] Typ (%s)    [p] Push", ct)
			rows = append(rows, "  "+lipgloss.NewStyle().Foreground(ColorSubtext).Render("  "+hints))
		}
		rows = append(rows, "  "+m.commitInput.View())
		rows = append(rows, "")

		if m.loading {
			rows = append(rows, "  "+lipgloss.NewStyle().Foreground(ColorInfo).Render("⟳  Trwa operacja git..."))
		}
	}

	return strings.Join(rows, "\n") + "\n"
}

// renderDiffView wyświetla kolorowy podgląd diff.
func (m GitPanelModel) renderDiffView(w int) string {
	var rows []string

	titleBar := lipgloss.NewStyle().
		Background(lipgloss.Color("#313244")).
		Width(w - 2).Padding(0, 1).
		Render(fmt.Sprintf("📄  Diff: %s   %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true).Render(m.diffFile),
			lipgloss.NewStyle().Foreground(ColorMuted).Render("[ESC / d] zamknij  [↑↓] przewijaj"),
		))
	rows = append(rows, "", titleBar, "")

	visH := m.diffVisibleLines()
	end := min(m.diffOffset+visH, len(m.diffLines))
	for _, l := range m.diffLines[m.diffOffset:end] {
		rows = append(rows, "  "+colorizeDiffLine(l))
	}

	if len(m.diffLines) > visH {
		total := max(1, len(m.diffLines)-visH)
		pct := m.diffOffset * 100 / total
		rows = append(rows, "", "  "+StyleMuted.Render(
			fmt.Sprintf("── %d/%d linii (%d%%) ──", m.diffOffset+1, len(m.diffLines), pct),
		))
	}

	return strings.Join(rows, "\n") + "\n"
}

// ─── [2] Branches Tab ─────────────────────────────────────────────────────

func (m GitPanelModel) renderBranchesTab(w int) string {
	var rows []string
	rows = append(rows, "")

	if len(m.branches) == 0 {
		rows = append(rows, "  "+StyleMuted.Render("Brak gałęzi lokalnych."))
		return strings.Join(rows, "\n") + "\n"
	}

	currentBranch := ""
	if m.gitStatus != nil {
		currentBranch = m.gitStatus.Branch
	}

	rows = append(rows, "  "+SectionHeader("⎇", "Gałęzie lokalne:", w-4))
	rows = append(rows, "")

	for i, b := range m.branches {
		isCurrent := b == currentBranch
		isSelected := i == m.selectedBranch

		var prefix, nameStr string

		if isCurrent {
			prefix = lipgloss.NewStyle().Foreground(ColorSuccess).Render("  ★ ")
			nameStr = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render(b)
			nameStr += lipgloss.NewStyle().Foreground(ColorMuted).Render("  (aktywna)")
		} else if isSelected {
			prefix = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render("  ▶ ")
			nameStr = lipgloss.NewStyle().Foreground(ColorSecondary).Render(b)
		} else {
			prefix = "    "
			nameStr = lipgloss.NewStyle().Foreground(ColorText).Render(b)
		}

		line := prefix + nameStr
		if isSelected && !isCurrent {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("#1A2433")).
				Width(w - 2).Render(line)
		}
		rows = append(rows, line)
	}

	rows = append(rows, "")
	if m.loading {
		rows = append(rows, "  "+StyleInfo.Render("⟳  Trwa checkout..."))
	} else {
		rows = append(rows, "  "+StyleMuted.Render("[ENTER] Checkout  [↑↓ / j/k] Nawigacja"))
	}

	return strings.Join(rows, "\n") + "\n"
}

// ─── [3] History Tab ──────────────────────────────────────────────────────

func (m GitPanelModel) renderHistoryTab(w int) string {
	var rows []string
	rows = append(rows, "")

	if len(m.history) == 0 {
		rows = append(rows, "  "+StyleMuted.Render("Brak historii commitów."))
		return strings.Join(rows, "\n") + "\n"
	}

	rows = append(rows, "  "+SectionHeader("📜", "Historia commitów:", w-4))
	rows = append(rows, "")

	// Nagłówek tabeli
	rows = append(rows, fmt.Sprintf("  %s  %s  %s  %s",
		lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(8).Render("Hash"),
		lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(11).Render("Kiedy"),
		lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Width(17).Render("Autor"),
		lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true).Render("Wiadomość"),
	))
	rows = append(rows, "  "+lipgloss.NewStyle().Foreground(ColorSurface).Render(repeatChar("─", min(w-4, 80))))

	for i, c := range m.history {
		isSelected := i == m.selectedHistory

		line := fmt.Sprintf("  %s  %s  %s  %s",
			graphHashStyle.Render(fmt.Sprintf("%-7s", c.ShortHash)),
			lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("%-10s", truncateStr(c.RelativeDate, 10))),
			lipgloss.NewStyle().Foreground(ColorSubtext).Render(fmt.Sprintf("%-16s", truncateStr(c.Author, 16))),
			lipgloss.NewStyle().Foreground(ColorText).Render(truncateStr(c.Message, w-52)),
		)

		if isSelected {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("#1A2433")).
				Width(w - 2).
				Render(line)
		}
		rows = append(rows, line)
	}

	return strings.Join(rows, "\n") + "\n"
}

// ─── [4] Graf Tab ─────────────────────────────────────────────────────────

func (m GitPanelModel) renderGraphTab(w int) string {
	var rows []string
	rows = append(rows, "")

	if len(m.gitGraph) == 0 {
		msg := "Brak danych grafu (brak commitów lub nie jest repo Git)."
		if m.loading {
			msg = "⟳  Ładowanie grafu commitów..."
		}
		rows = append(rows, "  "+StyleMuted.Render(msg))
		return strings.Join(rows, "\n") + "\n"
	}

	// Legenda
	legend := fmt.Sprintf("  %s  %s  %s  %s  %s",
		graphDotStyle.Render("● commit"),
		graphHEADStyle.Render("HEAD"),
		graphBranchStyle.Render("gałąź"),
		graphTagStyle.Render("tag"),
		graphLineStyle.Render("│╱╲ linia"),
	)
	rows = append(rows, legend)
	rows = append(rows, "  "+graphLineStyle.Render(repeatChar("─", min(w-4, 80))))
	rows = append(rows, "")

	visH := m.graphVisibleLines()
	start := m.graphOffset
	end := min(start+visH, len(m.gitGraph))

	for i, rawLine := range m.gitGraph[start:end] {
		globalIdx := start + i
		isSelected := globalIdx == m.selectedGraph
		colorized := colorizeGraphLine(rawLine)
		var rendered string
		if isSelected {
			rendered = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render("▶") + " " + colorized
			rendered = lipgloss.NewStyle().Background(lipgloss.Color("#1A2433")).Render(rendered)
		} else {
			rendered = "  " + colorized
		}
		rows = append(rows, rendered)
	}

	if len(m.gitGraph) > visH {
		total := max(1, len(m.gitGraph)-visH)
		pct := m.graphOffset * 100 / total
		rows = append(rows, "", "  "+StyleMuted.Render(fmt.Sprintf(
			"── linia %d/%d  (%d%%)  [↑↓/j/k] nawigacja ──",
			m.graphOffset+1, len(m.gitGraph), pct,
		)))
	}

	return strings.Join(rows, "\n") + "\n"
}

// renderPushConflict renderuje ekran konfliktu push.
func (m GitPanelModel) renderPushConflict(w int) string {
	var rows []string
	rows = append(rows, "")

	warnBadge := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorError).Bold(true).Padding(0, 1).
		Render("⚠  Push odrzucony — non-fast-forward")

	rows = append(rows,
		"  "+warnBadge,
		"",
		"  "+lipgloss.NewStyle().Foreground(ColorInfo).Render("Gałąź: "+m.pushConflictBranch),
		"",
		"  "+lipgloss.NewStyle().Foreground(ColorSubtext).Render("Zdalnie istnieją commity których lokalnie nie masz."),
		"  "+lipgloss.NewStyle().Foreground(ColorSubtext).Render("Dzieje się gdy ktoś inny wypchnął zmiany w między czasie."),
		"",
		"  Wybierz akcję:",
		"",
	)

	uKey := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorSuccess).Padding(0, 1).Bold(true).Render("[U]")
	fKey := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorWarning).Padding(0, 1).Bold(true).Render("[F]")
	escKey := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorSurface).Padding(0, 1).Render("[ESC]")

	rows = append(rows,
		"  "+uKey+"  "+lipgloss.NewStyle().Foreground(ColorText).Render("git pull --rebase + push")+
			"  "+lipgloss.NewStyle().Foreground(ColorMuted).Render("(ZALECANE) pobierz zmiany i wypchnij"),
		"",
		"  "+fKey+"  "+lipgloss.NewStyle().Foreground(ColorText).Render("force-with-lease push")+
			"  "+lipgloss.NewStyle().Foreground(ColorMuted).Render("NADPISUJE remote — tylko gdy świadomy wybór"),
		"",
		"  "+escKey+"  "+lipgloss.NewStyle().Foreground(ColorMuted).Render("Anuluj, wróć do Status"),
		"",
		"  "+lipgloss.NewStyle().Foreground(ColorMuted).Render("💡 --force-with-lease odrzuci push jeśli ktoś wypchnął nowe commity od Twojego fetch."),
	)

	return strings.Join(rows, "\n") + "\n"
}

// ─── Status bar ───────────────────────────────────────────────────────────

func (m GitPanelModel) renderStatusBar() string {
	if m.statusMsg == "" {
		return ""
	}
	var badge string
	if m.statusErr {
		badge = lipgloss.NewStyle().Foreground(ColorBg).Background(ColorError).Padding(0, 1).Render(m.statusMsg)
	} else {
		badge = lipgloss.NewStyle().Foreground(ColorBg).Background(ColorSuccess).Padding(0, 1).Render(m.statusMsg)
	}
	return "\n  " + badge + "\n"
}

// ─── Key bar ──────────────────────────────────────────────────────────────

func (m GitPanelModel) renderKeyBar(w int) string {
	var bindings []KeyBinding

	switch m.tab {
	case GitTabStatus:
		if m.pushConflict {
			bindings = []KeyBinding{{"U", "pull+rebase+push"}, {"F", "force-with-lease"}, {"ESC", "Anuluj"}}
		} else if m.showDiff {
			bindings = []KeyBinding{{"ESC / d", "Zamknij diff"}, {"↑↓", "Przewijaj"}}
		} else if m.gitStatus != nil && !m.gitStatus.IsClean {
			if m.commitInput.Focused() {
				bindings = []KeyBinding{{"ENTER", "Commit"}, {"ESC", "Anuluj"}}
			} else {
				bindings = []KeyBinding{
					{"↑↓ / j k", "Wybierz plik"},
					{"SPACJA", "Zaznacz/odznacz"},
					{"a", "Wszystkie"},
					{"d / ENTER", "Diff"},
					{"i", "Wpisz commit"},
					{"p", "Push"},
				}
			}
		} else {
			bindings = []KeyBinding{{"p", "Push do remote"}}
		}
	case GitTabBranches:
		bindings = []KeyBinding{{"ENTER", "Checkout"}, {"↑↓ / j k", "Nawigacja"}}
	case GitTabHistory:
		bindings = []KeyBinding{{"↑↓ / j k", "Nawigacja"}}
	case GitTabGraph:
		bindings = []KeyBinding{{"↑↓ / j k", "Nawigacja"}}
	}

	bindings = append(bindings,
		KeyBinding{"TAB / 1-4", "Zakładki"},
		KeyBinding{"Q / ESC", "Powrót"},
	)

	return KeyBar(bindings, w)
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func (m GitPanelModel) countChecked() int {
	count := 0
	for _, v := range m.stagedFiles {
		if v {
			count++
		}
	}
	return count
}

func (m GitPanelModel) graphVisibleLines() int {
	v := m.height - 14
	if v < 5 {
		v = 5
	}
	return v
}

func (m GitPanelModel) diffVisibleLines() int {
	v := m.height - 14
	if v < 5 {
		v = 5
	}
	return v
}

// colorizeGraphLine koloruje jedną linię git log --graph.
func colorizeGraphLine(line string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}

	runes := []rune(line)
	n := len(runes)

	starPos := -1
	for i, ch := range runes {
		if ch == '*' {
			starPos = i
			break
		}
	}

	var sb strings.Builder

	graphEnd := n
	if starPos >= 0 {
		graphEnd = starPos + 1
	}
	for i := 0; i < graphEnd; i++ {
		ch := runes[i]
		switch ch {
		case '*':
			sb.WriteString(graphDotStyle.Render("●"))
		case '|':
			sb.WriteString(graphLineStyle.Render("│"))
		case '/':
			sb.WriteString(graphLineStyle.Render("╱"))
		case '\\':
			sb.WriteString(graphLineStyle.Render("╲"))
		case '-':
			sb.WriteString(graphLineStyle.Render("─"))
		case '_':
			sb.WriteString(graphLineStyle.Render("─"))
		case '+':
			sb.WriteString(graphLineStyle.Render("┼"))
		default:
			sb.WriteRune(ch)
		}
	}

	if starPos < 0 || starPos+1 >= n {
		return sb.String()
	}

	rest := strings.TrimLeft(string(runes[starPos+1:]), " ")
	if rest == "" {
		return sb.String()
	}

	spaceIdx := strings.IndexByte(rest, ' ')
	if spaceIdx < 0 {
		sb.WriteString(" " + graphHashStyle.Render(rest))
		return sb.String()
	}
	hash := rest[:spaceIdx]
	remainder := rest[spaceIdx+1:]
	sb.WriteString(" " + graphHashStyle.Render(hash) + " ")

	if strings.HasPrefix(remainder, "(") {
		closeIdx := strings.Index(remainder, ")")
		if closeIdx > 0 {
			decoStr := remainder[1:closeIdx]
			msg := strings.TrimSpace(remainder[closeIdx+1:])
			sb.WriteString(graphLineStyle.Render("("))
			decos := strings.Split(decoStr, ", ")
			for i, d := range decos {
				d = strings.TrimSpace(d)
				if i > 0 {
					sb.WriteString(graphLineStyle.Render(", "))
				}
				switch {
				case strings.HasPrefix(d, "HEAD ->"):
					sb.WriteString(graphHEADStyle.Render("HEAD →"))
					if bn := strings.TrimSpace(strings.TrimPrefix(d, "HEAD ->")); bn != "" {
						sb.WriteString(graphHEADStyle.Render(" " + bn))
					}
				case strings.HasPrefix(d, "tag:"):
					sb.WriteString(graphTagStyle.Render(d))
				default:
					sb.WriteString(graphBranchStyle.Render(d))
				}
			}
			sb.WriteString(graphLineStyle.Render(") "))
			sb.WriteString(graphMsgStyle.Render(msg))
		} else {
			sb.WriteString(graphMsgStyle.Render(remainder))
		}
	} else {
		sb.WriteString(graphMsgStyle.Render(remainder))
	}

	return sb.String()
}

// colorizeDiffLine koloruje jedną linię unified diff.
func colorizeDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		return diffHeaderStyle.Render(line)
	case strings.HasPrefix(line, "+"):
		return diffAddStyle.Render(line)
	case strings.HasPrefix(line, "-"):
		return diffRemStyle.Render(line)
	case strings.HasPrefix(line, "@@"):
		return diffHunkStyle.Render(line)
	case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file"):
		return diffHeaderStyle.Render(line)
	default:
		return line
	}
}

// gitFileIcon zwraca ikonę i styl dla statusu pliku git.
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
		return "?", lipgloss.NewStyle().Foreground(ColorMuted)
	default:
		return "·", StyleMuted
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
