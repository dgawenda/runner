// file: pkg/tui/gitpanel.go
//
// ╔══════════════════════════════════════════════════════════════════════════╗
// ║  Git Panel — Wizualna Kontrola Repozytorium (styl GitKraken)           ║
// ║                                                                          ║
// ║  Zakładki:                                                               ║
// ║    · [1] STATUS  — lista zmian z checkboxami, diff, commit, push       ║
// ║    · [2] GAŁĘZIE — lista lokalnych gałęzi + checkout (ENTER)           ║
// ║    · [3] HISTORIA — ostatnie 30 commitów (tabela)                       ║
// ║    · [4] GRAF    — wizualny graf commitów (styl GitKraken)              ║
// ║                                                                          ║
// ║  Wybór plików do commita:                                               ║
// ║    SPACJA   — zaznacz/odznacz plik (checkbox)                           ║
// ║    a / A    — zaznacz wszystkie / odznacz wszystkie                     ║
// ║    [i]      — wpisz wiadomość commita                                   ║
// ║    ENTER    — commit (gdy input sfocusowany)                            ║
// ║    [p]      — push do remote (po commicie)                              ║
// ║                                                                          ║
// ║  Schemat kolorów grafu (Dracula-inspired):                              ║
// ║    ● #BD93F9 commit  │╱╲ #6272A4 graph  HEAD #50FA7B  branch #FFB86C  ║
// ║    hash #8BE9FD      tag #FF79C6         msg  #F8F8F2                   ║
// ║                                                                          ║
// ║  Auto-refresh co 3s — nowe commity widoczne natychmiast.               ║
// ╚══════════════════════════════════════════════════════════════════════════╝

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/neution/rnr/pkg/gitops"
)

// ─── Stałe kolorów grafu (Dracula theme) ─────────────────────────────────

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

// ─── Zakładki Git Panelu ──────────────────────────────────────────────────

// GitPanelTab to zakładka aktywna w Git Panelu.
type GitPanelTab int

const (
	GitTabStatus   GitPanelTab = iota // [1] Zmiany + diff + commit
	GitTabBranches                    // [2] Lista gałęzi + checkout
	GitTabHistory                     // [3] Tabela historii commitów
	GitTabGraph                       // [4] Wizualny graf (styl GitKraken)
)

// ─── Model ────────────────────────────────────────────────────────────────

// GitPanelModel obsługuje panel git — podgląd, commit, checkout i graf.
type GitPanelModel struct {
	width  int
	height int

	// Aktywna zakładka
	tab GitPanelTab

	// Dane git (aktualizowane przez root model z pollingu)
	gitStatus *gitops.StatusResult
	branches  []string
	history   []gitops.CommitInfo
	gitGraph  []string // Linie grafu z git log --graph

	// [1] Status tab — nawigacja plików + checkboxy + commit
	commitInput  textinput.Model
	selectedFile int            // Indeks zaznaczonego pliku w liście zmian
	stagedFiles  map[string]bool // Mapa ścieżka → zaznaczony do commita
	showDiff     bool           // Czy widoczny podgląd diff
	diffLines    []string
	diffFile     string
	diffOffset   int // Scroll offset diff

	// Stan po commicie — oczekiwanie na push
	lastCommitHash string // Hash ostatniego commita (pokazuje opcję push)
	pushAvailable  bool   // True gdy jest remote + można pushować

	// Stan konfliktu push (non-fast-forward)
	pushConflict       bool   // True gdy remote jest ahead — pokazuje opcje
	pushConflictBranch string // Gałąź której dotyczy konflikt

	// [2] Branches tab
	selectedBranch int

	// [3] History tab
	selectedHistory int

	// [4] Graph tab
	selectedGraph int
	graphOffset   int // Scroll offset grafu

	// Feedback dla użytkownika
	statusMsg string
	statusErr bool
	loading   bool // Trwa operacja git (checkout/commit/push)
}

// NewGitPanelModel tworzy nowy model Git Panelu.
func NewGitPanelModel(width, height int) GitPanelModel {
	ti := textinput.New()
	ti.Placeholder = "wpisz wiadomość commita i naciśnij ENTER..."
	ti.CharLimit = 200
	ti.Width = min(width-6, 70)
	// Input domyślnie NIE jest sfocusowany — SPACJA/↑↓ nawiguje po plikach
	// Użytkownik naciska 'i' aby edytować wiadomość commita

	return GitPanelModel{
		width:       width,
		height:      height,
		tab:         GitTabStatus,
		commitInput: ti,
		stagedFiles: make(map[string]bool),
	}
}

// ─── Interfejs Bubble Tea ─────────────────────────────────────────────────

// Init inicjalizuje model.
func (m GitPanelModel) Init() tea.Cmd { return nil }

// Update obsługuje zdarzenia dla Git Panelu.
func (m GitPanelModel) Update(msg tea.Msg) (GitPanelModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ── Aktualizacje danych ────────────────────────────────────────────
	case GitStatusMsg:
		if msg.Err == nil {
			m.gitStatus = msg.Result
			// Wyzeruj selectedFile jeśli wyszła poza zakres
			if m.gitStatus != nil && m.selectedFile >= len(m.gitStatus.DirtyFiles) {
				m.selectedFile = max(0, len(m.gitStatus.DirtyFiles)-1)
			}
			// Usuń ze stagedFiles pliki których już nie ma
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
			// Sprawdź czy push jest możliwy (pushAvailable ustawiany z zewnątrz)
			m.statusMsg = fmt.Sprintf("✓ Commit: %s  [p] Push do remote", msg.Hash)
			m.statusErr = false
			m.commitInput.SetValue("")
			m.commitInput.Blur() // ← KLUCZOWE: odblokuj input po commicie
			m.showDiff = false
			m.stagedFiles = make(map[string]bool) // Wyczyść zaznaczenia
		}

	case GitPushDoneMsg:
		m.loading = false
		if msg.Err != nil {
			if msg.IsNonFastForward {
				// Remote jest do przodu — pokaż opcje zamiast zwykłego błędu
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
			m.lastCommitHash = "" // Wyczyść po push
		}

	case GitPullRebasePushDoneMsg:
		m.loading = false
		m.pushConflict = false
		if msg.Err != nil {
			m.statusMsg = "✗ Pull+rebase+push nieudany: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = fmt.Sprintf("✓ Pull+rebase+push: gałąź %s zsynchronizowana z origin", msg.Branch)
			m.statusErr = false
			m.lastCommitHash = ""
		}

	// ── Klawiatura ────────────────────────────────────────────────────
	case tea.KeyMsg:

		// ── Tryb konfliktu push — obsługa ESC ─────────────────────────
		if m.tab == GitTabStatus && m.pushConflict {
			switch msg.String() {
			case "esc", "q":
				m.pushConflict = false
				m.statusMsg = "Push anulowany"
				m.statusErr = false
			}
			// Klawisze u/f/U/F obsługiwane niżej (ogólna obsługa)
			// Nie blokujemy dalszego przetwarzania — let fall through
		}

		// ── Tryb podglądu diff (Status tab) — osobna obsługa ─────────
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

		// ── Globalny klawisz 'i' — focusuje commit input ───────────
		if m.tab == GitTabStatus && msg.String() == "i" && !m.commitInput.Focused() {
			m.commitInput.Focus()
			m.commitInput.Width = min(m.width-6, 70)
			return m, tea.Batch(cmds...)
		}

		// ── Jeśli commit input jest sfocusowany — przekaż mu zdarzenia ─
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
						// Zbuduj listę zaznaczonych plików (lub wszystkich jeśli brak zaznaczenia)
						filesToStage := m.selectedFilePaths()
						return m, func() tea.Msg {
							return GitCommitRequestMsg{
								Message: commitMsg,
								Files:   filesToStage,
							}
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

		// ── Główny handler klawiszy (input NIE sfocusowany) ───────────
		switch msg.String() {

		// Przełączanie zakładek
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

		// Nawigacja ↑↓ — zależna od aktywnej zakładki
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

		// ── SPACJA — toggle zaznaczenia pliku do commita ──────────────
		case " ":
			if m.tab == GitTabStatus && m.gitStatus != nil && !m.gitStatus.IsClean {
				if len(m.gitStatus.DirtyFiles) > 0 {
					file := m.gitStatus.DirtyFiles[m.selectedFile].Path
					m.stagedFiles[file] = !m.stagedFiles[file]
					// Wyczyść statusMsg aby nie zasłaniał widoku
					m.statusMsg = ""
				}
			}

		// ── 'a' — zaznacz/odznacz wszystkie pliki ─────────────────────
		case "a", "A":
			if m.tab == GitTabStatus && m.gitStatus != nil && !m.gitStatus.IsClean {
				// Sprawdź czy wszystkie są zaznaczone
				allSelected := len(m.stagedFiles) == len(m.gitStatus.DirtyFiles)
				for _, f := range m.gitStatus.DirtyFiles {
					allSelected = allSelected && m.stagedFiles[f.Path]
				}
				if allSelected {
					// Odznacz wszystkie
					m.stagedFiles = make(map[string]bool)
				} else {
					// Zaznacz wszystkie
					for _, f := range m.gitStatus.DirtyFiles {
						m.stagedFiles[f.Path] = true
					}
				}
				m.statusMsg = ""
			}

		// ── Obsługa konfliktu push (non-fast-forward) ─────────────────
		// Aktywne gdy m.pushConflict == true
		case "u", "U":
			// [U] pull --rebase + push (zalecane)
			if m.tab == GitTabStatus && m.pushConflict && !m.loading {
				branch := m.pushConflictBranch
				m.loading = true
				m.pushConflict = false
				m.statusMsg = "↓ git pull --rebase + push..."
				m.statusErr = false
				return m, func() tea.Msg {
					return GitPullRebasePushRequestMsg{Branch: branch}
				}
			}

		case "f", "F":
			// [F] force-with-lease (nadpisuje remote — tylko gdy świadomy wybór)
			if m.tab == GitTabStatus && m.pushConflict && !m.loading {
				branch := m.pushConflictBranch
				m.loading = true
				m.pushConflict = false
				m.statusMsg = "⚡ git push --force-with-lease..."
				m.statusErr = false
				return m, func() tea.Msg {
					return GitForcePushRequestMsg{Branch: branch}
				}
			}

		// ── 'p' — push bieżącej gałęzi ───────────────────────────────
		case "p", "P":
			if m.tab == GitTabStatus && !m.loading {
				m.loading = true
				m.statusMsg = ""
				m.pushConflict = false
				return m, func() tea.Msg {
					return GitPushRequestMsg{}
				}
			}

		// Podgląd diff zaznaczonego pliku lub checkout gałęzi
		case "d":
			if m.tab == GitTabStatus && m.gitStatus != nil && !m.gitStatus.IsClean &&
				len(m.gitStatus.DirtyFiles) > 0 {
				file := m.gitStatus.DirtyFiles[m.selectedFile].Path
				m.statusMsg = ""
				return m, func() tea.Msg {
					return GitDiffRequestMsg{File: file}
				}
			}

		case "enter":
			switch m.tab {
			case GitTabStatus:
				// ENTER na pliku → podgląd diff
				if m.gitStatus != nil && !m.gitStatus.IsClean && len(m.gitStatus.DirtyFiles) > 0 {
					file := m.gitStatus.DirtyFiles[m.selectedFile].Path
					m.statusMsg = ""
					return m, func() tea.Msg {
						return GitDiffRequestMsg{File: file}
					}
				}
			case GitTabBranches:
				// ENTER na gałęzi = checkout
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
				return m, func() tea.Msg {
					return GitCheckoutRequestMsg{Branch: branch}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.commitInput.Width = min(m.width-6, 70)
	}

	return m, tea.Batch(cmds...)
}

// selectedFilePaths zwraca listę ścieżek do zaindeksowania.
// Jeśli użytkownik nie zaznaczył żadnego pliku — zwraca pustą listę (= stage all).
// Jeśli zaznaczył przynajmniej jeden — zwraca tylko zaznaczone.
func (m GitPanelModel) selectedFilePaths() []string {
	if len(m.stagedFiles) == 0 {
		return nil // puste = git add -A
	}
	var paths []string
	for path, checked := range m.stagedFiles {
		if checked {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return nil // żaden nie zaznaczony = stage all
	}
	return paths
}

// ─── Widok ────────────────────────────────────────────────────────────────

// View renderuje Git Panel.
func (m GitPanelModel) View() string {
	contentW := min(m.width-2, 96)

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
			// Pokaż ile plików zaznaczono vs ile jest łącznie
			total := len(m.gitStatus.DirtyFiles)
			checked := m.countChecked()
			if checked > 0 {
				statusDot = StyleWarning.Render(fmt.Sprintf("  ●  %d/%d do commita", checked, total))
			} else {
				statusDot = StyleWarning.Render(fmt.Sprintf("  ●  %d zmian", total))
			}
		}
	} else {
		branchInfo = StyleMuted.Render("  (brak git)")
	}

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
		{"[4] GRAF ◈", GitTabGraph},
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
	case GitTabGraph:
		return m.renderGraphTab(width)
	}
	return ""
}

// ─── [1] Status Tab ───────────────────────────────────────────────────────

// renderStatusTab wyświetla listę zmian z checkboxami, podgląd diff i formularz commita.
func (m GitPanelModel) renderStatusTab(width int) string {
	if m.gitStatus == nil {
		return "  " + StyleInfo.Render("⟳ Ładowanie statusu Git...") + "\n"
	}
	if !m.gitStatus.IsGitRepo {
		return "  " + StyleMuted.Render("Ten katalog nie jest repozytorium Git.") + "\n"
	}

	// ── Tryb diff ─────────────────────────────────────────────────────
	if m.showDiff {
		return m.renderDiffView(width)
	}

	// ── Tryb konfliktu push (non-fast-forward) ─────────────────────
	if m.pushConflict {
		return m.renderPushConflict(width)
	}

	var lines []string

	// Wiersz statusu remote
	if m.gitStatus.HasRemote {
		remoteShort := m.gitStatus.RemoteURL
		if len(remoteShort) > 50 {
			remoteShort = "…" + remoteShort[len(remoteShort)-47:]
		}
		lines = append(lines,
			"  "+StyleMuted.Render("🔗 remote: "+remoteShort),
			"",
		)
	} else {
		lines = append(lines,
			"  "+StyleWarning.Render("⚠  Brak remote origin — push niedostępny"),
			"  "+StyleMuted.Render("   Dodaj: git remote add origin <URL>"),
			"",
		)
	}

	if m.gitStatus.IsClean {
		lines = append(lines,
			"  "+StyleSuccess.Render("✅  Repozytorium czyste — brak zmian do zatwierdzenia"),
			"",
		)
		if m.gitStatus.LastCommit.Hash != "" {
			lines = append(lines,
				"  "+StyleLabel.Render("Ostatni commit:"),
				fmt.Sprintf("  %s  %s  %s",
					StyleMuted.Render(m.gitStatus.LastCommit.ShortHash),
					StyleValue.Render(truncateStr(m.gitStatus.LastCommit.Message, width-40)),
					StyleMuted.Render(m.gitStatus.LastCommit.RelativeDate),
				),
				"  "+StyleMuted.Render("  autor: "+m.gitStatus.LastCommit.Author),
			)
		}
		// Pokaż opcję push jeśli jest remote i ostatni commit
		if !m.loading && m.gitStatus.HasRemote {
			lines = append(lines, "", "  "+StyleMuted.Render("[p] Push do remote"))
		}
	} else {
		checkedCount := m.countChecked()

		// ── Nagłówek listy plików ─────────────────────────────────────
		hintLine := "  " + StyleMuted.Render("↑↓ wybierz  SPACJA zaznacz  [a] wszystkie  [d] diff  [i] commit")
		if checkedCount > 0 {
			hintLine = "  " + lipgloss.NewStyle().Foreground(ColorPrimary).
				Render(fmt.Sprintf("✓ %d plik(ów) zaznaczonych do commita  SPACJA odznacz  [a] reset", checkedCount))
		}
		lines = append(lines,
			"  "+StyleWarning.Render(fmt.Sprintf("⚠  %d niezatwierdzonych zmian:", len(m.gitStatus.DirtyFiles))),
			hintLine,
			"",
		)

		// ── Lista plików z checkboxami ────────────────────────────────
		maxFiles := 12
		for i, f := range m.gitStatus.DirtyFiles {
			if i >= maxFiles {
				lines = append(lines,
					StyleMuted.Render(fmt.Sprintf("     ... i %d więcej", len(m.gitStatus.DirtyFiles)-maxFiles)),
				)
				break
			}
			icon, iconStyle := gitFileIcon(f.Status)
			isSelected := i == m.selectedFile
			isChecked := m.stagedFiles[f.Path]

			// Checkbox
			var checkBox string
			if isChecked {
				checkBox = checkboxOnStyle.Render("[✓]")
			} else {
				checkBox = checkboxOffStyle.Render("[ ]")
			}

			fileStr := fmt.Sprintf("  %s %s %s  %s",
				checkBox,
				iconStyle.Render(fmt.Sprintf("[%s]", strings.TrimSpace(f.Status))),
				icon,
				StyleValue.Render(f.Path),
			)

			if isSelected {
				// Kursor nawigacji
				arrow := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("▶")
				fileStr = " " + arrow + fileStr[2:]
				fileStr = lipgloss.NewStyle().
					Background(lipgloss.Color("#1E1E2E")).
					Width(width - 2).
					Render(strings.TrimLeft(fileStr, " "))
			}
			lines = append(lines, fileStr)
		}

		// ── Podsumowanie zaznaczenia ──────────────────────────────────
		lines = append(lines, "")
		if checkedCount == 0 {
			lines = append(lines,
				"  "+StyleMuted.Render("(brak zaznaczonych → commit obejmie WSZYSTKIE pliki)"),
			)
		} else {
			lines = append(lines,
				"  "+checkboxOnStyle.Render(fmt.Sprintf("Zaznaczono: %d/%d pliku(ów) — commit obejmie tylko zaznaczone", checkedCount, len(m.gitStatus.DirtyFiles))),
			)
		}

		// ── Formularz commita ─────────────────────────────────────────
		lines = append(lines, "")

		if m.commitInput.Focused() {
			lines = append(lines, "  "+lipgloss.NewStyle().Foreground(ColorPrimary).Render("✎  Wiadomość commita [ESC anuluj]:"))
		} else {
			lines = append(lines, "  "+StyleLabel.Render("✎  [i] Napisz commit  [p] Push"))
		}
		lines = append(lines, "  "+m.commitInput.View())
		lines = append(lines, "")

		if m.loading {
			lines = append(lines, "  "+StyleInfo.Render("⟳  Trwa operacja git..."))
		}
	}

	return strings.Join(lines, "\n") + "\n"
}

// renderDiffView wyświetla kolorowy podgląd diff dla wybranego pliku.
func (m GitPanelModel) renderDiffView(width int) string {
	var lines []string

	// ── Nagłówek diff ─────────────────────────────────────────────────
	titleBar := lipgloss.NewStyle().
		Background(lipgloss.Color("#44475A")).
		Width(width - 2).
		Padding(0, 1).
		Render(fmt.Sprintf("📄  Diff: %s   %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true).Render(m.diffFile),
			StyleMuted.Render("[ESC / d] zamknij  [↑↓] przewijaj"),
		))
	lines = append(lines, titleBar, "")

	// ── Zawartość diff ────────────────────────────────────────────────
	visH := m.diffVisibleLines()
	end := min(m.diffOffset+visH, len(m.diffLines))
	for _, l := range m.diffLines[m.diffOffset:end] {
		lines = append(lines, "  "+colorizeDiffLine(l))
	}

	// ── Pasek postępu scroll ──────────────────────────────────────────
	if len(m.diffLines) > visH {
		total := max(1, len(m.diffLines)-visH)
		pct := m.diffOffset * 100 / total
		lines = append(lines, "",
			"  "+StyleMuted.Render(fmt.Sprintf("── %d/%d linii (%d%%) ──", m.diffOffset+1, len(m.diffLines), pct)),
		)
	}

	return strings.Join(lines, "\n") + "\n"
}

// ─── [2] Branches Tab ─────────────────────────────────────────────────────

func (m GitPanelModel) renderBranchesTab(width int) string {
	if len(m.branches) == 0 {
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
		if isCurrent && isSelected {
			line = fmt.Sprintf("  ▶  %s%s",
				lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render(b),
				StyleMuted.Render("  ★ aktywna"),
			)
			line = lipgloss.NewStyle().Background(lipgloss.Color("#1E1E2E")).Width(width-2).Render(line)
		} else if isCurrent {
			line = fmt.Sprintf("  ★  %s%s",
				lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render(b),
				StyleMuted.Render("  (aktywna)"),
			)
		} else if isSelected {
			prefix := lipgloss.NewStyle().Foreground(ColorPrimary).Render("  ▶  ")
			name := lipgloss.NewStyle().Foreground(ColorPrimary).Render(b)
			line = lipgloss.NewStyle().Background(lipgloss.Color("#1E1E2E")).Width(width-2).Render(prefix + name)
		} else {
			line = "     " + StyleValue.Render(b)
		}
		lines = append(lines, line)
	}

	result := strings.Join(lines, "\n") + "\n\n"
	if m.loading {
		result += "  " + StyleInfo.Render("⟳  Trwa checkout...") + "\n"
	} else {
		result += "  " + StyleMuted.Render("[ENTER] Checkout  [↑↓ / j/k] Nawigacja") + "\n"
	}
	return result
}

// ─── [3] Historia Tab ─────────────────────────────────────────────────────

func (m GitPanelModel) renderHistoryTab(width int) string {
	if len(m.history) == 0 {
		return "  " + StyleMuted.Render("Brak historii commitów.") + "\n"
	}

	var lines []string
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
			graphHashStyle.Render(fmt.Sprintf("%-7s", c.ShortHash)),
			StyleMuted.Render(fmt.Sprintf("%-10s", truncateStr(c.RelativeDate, 10))),
			StyleMuted.Render(fmt.Sprintf("%-16s", truncateStr(c.Author, 16))),
			truncateStr(c.Message, width-52),
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

// ─── [4] Graf Tab (styl GitKraken) ───────────────────────────────────────

// renderGraphTab wyświetla wizualny graf commitów z kolorowanymi gałęziami.
func (m GitPanelModel) renderGraphTab(width int) string {
	if len(m.gitGraph) == 0 {
		if m.loading {
			return "  " + StyleInfo.Render("⟳  Ładowanie grafu commitów...") + "\n"
		}
		return "  " + StyleMuted.Render("Brak danych grafu (brak commitów lub nie jest repo Git).") + "\n"
	}

	var lines []string

	// Legenda
	legend := fmt.Sprintf("  %s  %s  %s  %s  %s",
		graphDotStyle.Render("● commit"),
		graphHEADStyle.Render("HEAD"),
		graphBranchStyle.Render("gałąź"),
		graphTagStyle.Render("tag"),
		graphLineStyle.Render("│╱╲ linia"),
	)
	lines = append(lines, legend)
	lines = append(lines, "  "+graphLineStyle.Render(strings.Repeat("─", min(width-4, 80))))
	lines = append(lines, "")

	// Widoczne linie
	visH := m.graphVisibleLines()
	start := m.graphOffset
	end := min(start+visH, len(m.gitGraph))

	for i, rawLine := range m.gitGraph[start:end] {
		globalIdx := start + i
		isSelected := globalIdx == m.selectedGraph

		colorized := colorizeGraphLine(rawLine)

		var rendered string
		if isSelected {
			selector := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("▶")
			rendered = selector + " " + colorized
			rendered = lipgloss.NewStyle().Background(lipgloss.Color("#1E1E2E")).Render(rendered)
		} else {
			rendered = "  " + colorized
		}
		lines = append(lines, rendered)
	}

	// Pasek scroll
	if len(m.gitGraph) > visH {
		total := max(1, len(m.gitGraph)-visH)
		pct := m.graphOffset * 100 / total
		lines = append(lines, "")
		lines = append(lines,
			"  "+StyleMuted.Render(fmt.Sprintf(
				"── linia %d/%d  (%d%%)  [↑↓/j/k] nawigacja ──",
				m.graphOffset+1, len(m.gitGraph), pct,
			)),
		)
	}

	return strings.Join(lines, "\n") + "\n"
}

// renderPushConflict renderuje ekran wyboru akcji po odrzuceniu push (non-fast-forward).
// Pojawia się gdy remote branch jest do przodu względem lokalnej gałęzi.
func (m GitPanelModel) renderPushConflict(width int) string {
	var lines []string

	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true)
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	optStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))

	lines = append(lines,
		"  "+warnStyle.Render("⚠  Push odrzucony — remote jest do przodu (non-fast-forward)"),
		"",
		"  "+infoStyle.Render("Gałąź: "+m.pushConflictBranch),
		"",
		"  "+descStyle.Render("Zdalnie istnieją commity których lokalnie nie masz."),
		"  "+descStyle.Render("Dzieje się tak gdy ktoś inny wypchnął zmiany w między czasie."),
		"",
		"  Wybierz akcję:",
		"",
		"  "+keyStyle.Render("[U]")+" "+optStyle.Render("  pull --rebase + push")+" "+
			descStyle.Render("— (ZALECANE) pobierz zmiany, nałóż swoje na wierzchu i wypchnij"),
		"",
		"  "+keyStyle.Render("[F]")+" "+optStyle.Render("  force-with-lease push")+" "+
			descStyle.Render("— NADPISUJE remote! Używaj tylko gdy wiesz co robisz"),
		"",
		"  "+keyStyle.Render("[ESC]")+" "+descStyle.Render("— anuluj, wróć do Status"),
		"",
	)

	// Ostrzeżenie przy force
	lines = append(lines,
		"  "+descStyle.Render("💡 --force-with-lease jest bezpieczniejszy od --force:"),
		"  "+descStyle.Render("   odrzuci push jeśli ktoś wypchnął nowe commity od Twojego fetch."),
	)

	return strings.Join(lines, "\n") + "\n"
}

// ─── Pasek statusu i klawiszy ─────────────────────────────────────────────

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
		if m.pushConflict {
			// Tryb rozwiązywania konfliktu push
			bindings = append(bindings,
				keyBind("U", "pull+rebase+push"),
				keyBind("F", "force-with-lease"),
				keyBind("ESC", "Anuluj"),
			)
		} else if m.showDiff {
			bindings = append(bindings,
				keyBind("ESC / d", "Zamknij diff"),
				keyBind("↑↓", "Przewijaj"),
			)
		} else if m.gitStatus != nil && !m.gitStatus.IsClean {
			if m.commitInput.Focused() {
				bindings = append(bindings,
					keyBind("ENTER", "Commit"),
					keyBind("ESC", "Anuluj"),
				)
			} else {
				bindings = append(bindings,
					keyBind("↑↓ / j/k", "Wybierz plik"),
					keyBind("SPACJA", "Zaznacz/odznacz"),
					keyBind("a", "Wszystkie"),
					keyBind("d / ENTER", "Diff"),
					keyBind("i", "Wpisz commit"),
					keyBind("p", "Push"),
				)
			}
		} else {
			bindings = append(bindings, keyBind("p", "Push do remote"))
		}
	case GitTabBranches:
		bindings = append(bindings,
			keyBind("ENTER", "Checkout"),
			keyBind("↑↓ / j/k", "Nawigacja"),
		)
	case GitTabHistory:
		bindings = append(bindings, keyBind("↑↓ / j/k", "Nawigacja"))
	case GitTabGraph:
		bindings = append(bindings,
			keyBind("↑↓ / j/k", "Nawigacja"),
		)
	}

	bindings = append(bindings,
		keyBind("TAB / 1-4", "Zakładki"),
		keyBind("Q / ESC", "Powrót"),
	)

	divider := Divider(width)
	row := strings.Join(bindings, "  ")
	return "\n" + divider + "\n" + lipgloss.NewStyle().Padding(0, 2).Render(row) + "\n"
}

// ─── Helpers ─────────────────────────────────────────────────────────────

// countChecked zwraca liczbę zaznaczonych (zaindeksowanych) plików.
func (m GitPanelModel) countChecked() int {
	count := 0
	for _, v := range m.stagedFiles {
		if v {
			count++
		}
	}
	return count
}

// graphVisibleLines zwraca liczbę linii grafu widocznych w panelu.
func (m GitPanelModel) graphVisibleLines() int {
	v := m.height - 14
	if v < 5 {
		v = 5
	}
	return v
}

// diffVisibleLines zwraca liczbę linii diff widocznych w panelu.
func (m GitPanelModel) diffVisibleLines() int {
	v := m.height - 14
	if v < 5 {
		v = 5
	}
	return v
}

// colorizeGraphLine przetwarza jedną linię git log --graph (bez kolorów ANSI)
// i zwraca ją z kolorami Lipgloss oraz znakami Unicode zamiast ASCII.
//
// Schemat transformacji znaków:
//
//	* → ●  (commit dot, fioletowy)
//	| → │  (pionowa linia, niebieskoszary)
//	/ → ╱  (ukos prawy, niebieskoszary)
//	\ → ╲  (ukos lewy, niebieskoszary)
//	- → ─  (pozioma linia)
//	_ → ─  (podkreślenie = pozioma linia)
//	+ → ┼  (skrzyżowanie)
func colorizeGraphLine(line string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}

	runes := []rune(line)
	n := len(runes)

	// Znajdź pozycję * — markera commita
	starPos := -1
	for i, ch := range runes {
		if ch == '*' {
			starPos = i
			break
		}
	}

	var sb strings.Builder

	// Przetworz sekcję grafu (do * włącznie)
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

	// Brak commita w tej linii (czysta linia grafu)
	if starPos < 0 || starPos+1 >= n {
		return sb.String()
	}

	// Przetworz info commita: hash, dekoracje, wiadomość
	rest := strings.TrimLeft(string(runes[starPos+1:]), " ")
	if rest == "" {
		return sb.String()
	}

	// Wyodrębnij hash (pierwsze słowo)
	spaceIdx := strings.IndexByte(rest, ' ')
	if spaceIdx < 0 {
		sb.WriteString(" " + graphHashStyle.Render(rest))
		return sb.String()
	}
	hash := rest[:spaceIdx]
	remainder := rest[spaceIdx+1:]

	sb.WriteString(" " + graphHashStyle.Render(hash) + " ")

	// Sprawdź czy są dekoracje ref: (HEAD -> master, origin/master)
	if strings.HasPrefix(remainder, "(") {
		closeIdx := strings.Index(remainder, ")")
		if closeIdx > 0 {
			decoStr := remainder[1:closeIdx]
			msg := strings.TrimSpace(remainder[closeIdx+1:])

			// Renderuj dekoracje
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
					branchName := strings.TrimSpace(strings.TrimPrefix(d, "HEAD ->"))
					if branchName != "" {
						sb.WriteString(graphHEADStyle.Render(" " + branchName))
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
//
// Schemat kolorów:
//
//	+ linie dodane     → zielony
//	- linie usunięte   → czerwony
//	@@ nagłówki hunka  → niebieski
//	diff/index linie   → szary
//	+++ / ---          → szary (meta-nagłówki)
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

// max zwraca większą z dwóch liczb całkowitych.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
