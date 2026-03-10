// file: pkg/tui/logs.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Przeglądarka Logów Wdrożeń                                         ║
// ║                                                                      ║
// ║  Interaktywny TUI do przeglądania plików logów w .rnr/logs/.        ║
// ║  Obsługuje:                                                          ║
// ║    · Listę plików posortowaną według daty (najnowsze na górze)      ║
// ║    · Przeglądanie zawartości z kolorowaniem poziomów logów          ║
// ║    · Przewijanie klawiszami ↑↓, PgUp/PgDn, g/G                     ║
// ║    · Filtrowanie logów po środowisku (kolor badge)                  ║
// ╚══════════════════════════════════════════════════════════════════════╝

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Typy ─────────────────────────────────────────────────────────────────

// logFileInfo przechowuje metadane jednego pliku logu.
type logFileInfo struct {
	Name    string
	ModTime time.Time
	Path    string
	Size    int64
}

// LogsModel to model ekranu przeglądarki logów.
type LogsModel struct {
	width        int
	height       int
	logDir       string
	files        []logFileInfo
	selectedFile int

	// Stan podglądu zawartości pliku
	viewing      bool
	content      []string
	scrollOffset int
}

// ─── Konstruktor ──────────────────────────────────────────────────────────

// NewLogsModel tworzy nowy model przeglądarki logów.
func NewLogsModel(width, height int, logDir string) LogsModel {
	m := LogsModel{
		width:  width,
		height: height,
		logDir: logDir,
	}
	m.loadFiles()
	return m
}

// loadFiles odczytuje listę plików logów posortowaną od najnowszego.
func (m *LogsModel) loadFiles() {
	entries, err := os.ReadDir(m.logDir)
	if err != nil {
		m.files = nil
		return
	}

	var files []logFileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".log") && !strings.HasSuffix(e.Name(), ".txt") {
			// Akceptuj też pliki bez rozszerzenia
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, logFileInfo{
				Name:    e.Name(),
				ModTime: info.ModTime(),
				Path:    filepath.Join(m.logDir, e.Name()),
				Size:    info.Size(),
			})
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, logFileInfo{
			Name:    e.Name(),
			ModTime: info.ModTime(),
			Path:    filepath.Join(m.logDir, e.Name()),
			Size:    info.Size(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})
	m.files = files
	if m.selectedFile >= len(m.files) {
		m.selectedFile = 0
	}
}

// ─── Interfejs Bubble Tea ─────────────────────────────────────────────────

// Init inicjalizuje model logów.
func (m LogsModel) Init() tea.Cmd {
	return nil
}

// Update obsługuje zdarzenia modelu logów.
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m LogsModel) handleKey(msg tea.KeyMsg) (LogsModel, tea.Cmd) {
	if m.viewing {
		return m.handleContentKey(msg)
	}
	return m.handleListKey(msg)
}

// handleListKey obsługuje klawisze na ekranie listy plików.
func (m LogsModel) handleListKey(msg tea.KeyMsg) (LogsModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedFile > 0 {
			m.selectedFile--
		}
	case "down", "j":
		if m.selectedFile < len(m.files)-1 {
			m.selectedFile++
		}
	case "enter", " ":
		if len(m.files) > 0 {
			m.openFile(m.selectedFile)
		}
	case "r", "R":
		// Odśwież listę
		m.loadFiles()
	}
	return m, nil
}

// handleContentKey obsługuje klawisze podczas przeglądania zawartości pliku.
func (m LogsModel) handleContentKey(msg tea.KeyMsg) (LogsModel, tea.Cmd) {
	viewHeight := m.height - 8
	if viewHeight < 4 {
		viewHeight = 4
	}

	switch msg.String() {
	case "esc", "q", "backspace":
		m.viewing = false
		m.content = nil
		m.scrollOffset = 0

	case "up", "k":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}

	case "down", "j":
		maxScroll := len(m.content) - viewHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.scrollOffset < maxScroll {
			m.scrollOffset++
		}

	case "pgup":
		m.scrollOffset -= viewHeight / 2
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}

	case "pgdown":
		maxScroll := len(m.content) - viewHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		m.scrollOffset += viewHeight / 2
		if m.scrollOffset > maxScroll {
			m.scrollOffset = maxScroll
		}

	case "g", "home":
		m.scrollOffset = 0

	case "G", "end":
		maxScroll := len(m.content) - viewHeight
		if maxScroll > 0 {
			m.scrollOffset = maxScroll
		}
	}

	return m, nil
}

// openFile otwiera plik logu do przeglądania.
func (m *LogsModel) openFile(idx int) {
	if idx < 0 || idx >= len(m.files) {
		return
	}
	data, err := os.ReadFile(m.files[idx].Path)
	if err != nil {
		m.content = []string{StyleError.Render("Błąd odczytu pliku: " + err.Error())}
	} else {
		m.content = strings.Split(string(data), "\n")
	}
	m.viewing = true
	// Zacznij od końca (najnowsze wpisy)
	viewHeight := m.height - 8
	if viewHeight < 4 {
		viewHeight = 4
	}
	m.scrollOffset = len(m.content) - viewHeight
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// ─── Widoki ───────────────────────────────────────────────────────────────

// View renderuje ekran logów.
func (m LogsModel) View() string {
	if m.viewing {
		return m.viewContent()
	}
	return m.viewList()
}

// viewList renderuje listę plików logów.
func (m LogsModel) viewList() string {
	contentW := min(m.width-2, 100)

	title := StyleTitle.Render("📄 Logi wdrożeń")
	header := "\n" + lipgloss.NewStyle().Padding(0, 1).Render(title) + "\n" + Divider(contentW)

	if len(m.files) == 0 {
		empty := lipgloss.NewStyle().Padding(1, 3).Foreground(ColorSubtext).Render(
			"Brak plików logów.\n\n" +
				"Logi pojawią się po pierwszym wdrożeniu.\n" +
				"Uruchom: " + StyleCode.Render("rnr deploy production"),
		)
		hint := "\n" + Divider(contentW) + "\n" +
			lipgloss.NewStyle().Padding(0, 2).Render(
				keyBind("ESC", "Powrót do Dashboard"),
			)
		return header + "\n" + empty + hint
	}

	// Oblicz widoczny zakres przy dużej liczbie plików
	visibleHeight := m.height - 10
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	start := 0
	if m.selectedFile >= visibleHeight {
		start = m.selectedFile - visibleHeight + 1
	}

	var rows []string
	for i := start; i < len(m.files) && i < start+visibleHeight; i++ {
		f := m.files[i]
		isSelected := i == m.selectedFile

		sizeStr := formatLogSize(f.Size)
		timeStr := f.ModTime.Format("02.01.2006 15:04:05")

		// Koloruj nazwę na podstawie środowiska
		nameColor := ColorSubtext
		if strings.Contains(f.Name, "production") {
			nameColor = ColorError
		} else if strings.Contains(f.Name, "staging") {
			nameColor = ColorWarning
		} else if strings.Contains(f.Name, "rollback") {
			nameColor = ColorSecondary
		} else if strings.Contains(f.Name, "promote") {
			nameColor = ColorAccent
		}

		nameStr := lipgloss.NewStyle().Foreground(nameColor).Bold(isSelected).Render(f.Name)
		metaStr := StyleMuted.Render("  " + timeStr + "  " + sizeStr)

		var row string
		if isSelected {
			row = lipgloss.NewStyle().
				Background(lipgloss.Color("#2A2A3E")).
				Width(contentW - 2).
				Padding(0, 1).
				Render("▶ " + nameStr + metaStr)
		} else {
			row = lipgloss.NewStyle().Padding(0, 3).Render(nameStr + metaStr)
		}
		rows = append(rows, row)
	}

	// Wskaźnik scroll gdy więcej plików niż widać
	scrollIndicator := ""
	if len(m.files) > visibleHeight {
		scrollIndicator = StyleMuted.Render(fmt.Sprintf(
			"\n  %d/%d plików  ↑↓ przewijaj", m.selectedFile+1, len(m.files),
		))
	}

	hint := "\n" + Divider(contentW) + "\n" +
		lipgloss.NewStyle().Padding(0, 2).Render(strings.Join([]string{
			keyBind("ENTER", "Otwórz"),
			keyBind("↑↓", "Nawigacja"),
			keyBind("R", "Odśwież"),
			keyBind("ESC", "Dashboard"),
		}, "  "))

	return header + "\n" +
		strings.Join(rows, "\n") +
		scrollIndicator +
		hint
}

// viewContent renderuje zawartość wybranego pliku logu.
func (m LogsModel) viewContent() string {
	contentW := min(m.width-2, 100)

	fileName := "(brak)"
	if m.selectedFile < len(m.files) {
		fileName = m.files[m.selectedFile].Name
	}

	title := StyleTitle.Render("📄 " + fileName)
	header := "\n" + lipgloss.NewStyle().Padding(0, 1).Render(title) + "\n" + Divider(contentW)

	// Oblicz viewport
	viewHeight := m.height - 8
	if viewHeight < 4 {
		viewHeight = 4
	}

	start := m.scrollOffset
	end := start + viewHeight
	if end > len(m.content) {
		end = len(m.content)
	}

	var lines []string
	for i := start; i < end; i++ {
		lineNum := i + 1
		numStr := StyleMuted.Render(fmt.Sprintf("%5d │ ", lineNum))
		rawLine := m.content[i]

		// Kolorowanie na podstawie zawartości
		var lineStr string
		switch {
		case strings.Contains(rawLine, "[ERROR]") ||
			strings.Contains(rawLine, "BŁĄD") ||
			strings.Contains(rawLine, "✗"):
			lineStr = lipgloss.NewStyle().Foreground(ColorError).Render(rawLine)
		case strings.Contains(rawLine, "[WARN]") ||
			strings.Contains(rawLine, "⚠") ||
			strings.Contains(rawLine, "WARNING"):
			lineStr = lipgloss.NewStyle().Foreground(ColorWarning).Render(rawLine)
		case strings.Contains(rawLine, "[OK]") ||
			strings.Contains(rawLine, "✓") ||
			strings.Contains(rawLine, "✅") ||
			strings.Contains(rawLine, "SUCCESS"):
			lineStr = lipgloss.NewStyle().Foreground(ColorSuccess).Render(rawLine)
		case strings.Contains(rawLine, "▶") ||
			strings.Contains(rawLine, "ETAP") ||
			strings.Contains(rawLine, "STAGE"):
			lineStr = lipgloss.NewStyle().Foreground(ColorInfo).Render(rawLine)
		default:
			lineStr = lipgloss.NewStyle().Foreground(ColorText).Render(rawLine)
		}

		lines = append(lines, numStr+lineStr)
	}

	// Pasek scrollowania
	totalLines := len(m.content)
	pct := 0
	if totalLines > viewHeight && totalLines > 0 {
		pct = (m.scrollOffset * 100) / (totalLines - viewHeight)
		if pct > 100 {
			pct = 100
		}
	} else {
		pct = 100
	}
	scrollBar := StyleMuted.Render(fmt.Sprintf(
		"  Linia %d/%d  (%d%%)", m.scrollOffset+1, totalLines, pct,
	))

	hint := "\n" + Divider(contentW) + "\n" +
		lipgloss.NewStyle().Padding(0, 2).Render(strings.Join([]string{
			keyBind("↑↓", "Linia"),
			keyBind("PgUp/PgDn", "Strona"),
			keyBind("g/G", "Pocz./Koniec"),
			keyBind("ESC", "Lista"),
		}, "  ")+"   "+scrollBar)

	return header + "\n" +
		strings.Join(lines, "\n") +
		hint
}

// ─── Helpers ──────────────────────────────────────────────────────────────

// formatLogSize formatuje rozmiar pliku w czytelny sposób.
func formatLogSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%dB", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	}
}
