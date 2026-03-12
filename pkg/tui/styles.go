// file: pkg/tui/styles.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Definicje Stylów Wizualnych (Lipgloss)                             ║
// ║                                                                      ║
// ║  Centralna paleta kolorów i stylów dla całego interfejsu TUI.       ║
// ║  Catppuccin Mocha — ciemne tło, pastelowe akcenty, wysoki kontrast. ║
// ╚══════════════════════════════════════════════════════════════════════╝

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Paleta Kolorów ────────────────────────────────────────────────────────

const (
	// Tło
	ColorBg      = lipgloss.Color("#1E1E2E")
	ColorBgAlt   = lipgloss.Color("#181825")
	ColorBgCard  = lipgloss.Color("#24273A")
	ColorSurface = lipgloss.Color("#313244")
	ColorOverlay = lipgloss.Color("#45475A")

	// Tekst
	ColorText    = lipgloss.Color("#CDD6F4")
	ColorSubtext = lipgloss.Color("#A6ADC8")
	ColorMuted   = lipgloss.Color("#585B70")

	// Akcenty
	ColorPrimary   = lipgloss.Color("#CBA6F7") // Mauve (fioletowy)
	ColorSecondary = lipgloss.Color("#89DCEB") // Sky (cyjan)
	ColorAccent    = lipgloss.Color("#F5C2E7") // Pink (różowy)
	ColorGold      = lipgloss.Color("#F9E2AF") // Yellow

	// Statusy
	ColorSuccess = lipgloss.Color("#A6E3A1") // Green
	ColorWarning = lipgloss.Color("#F9E2AF") // Yellow
	ColorError   = lipgloss.Color("#F38BA8") // Red
	ColorInfo    = lipgloss.Color("#89B4FA") // Blue

	// Środowiska
	ColorProduction  = lipgloss.Color("#F38BA8") // Czerwony = uwaga
	ColorDevelopment = lipgloss.Color("#A6E3A1") // Zielony = bezpieczny

	// Apollo
	ColorApollo = lipgloss.Color("#FF79C6") // Pink
)

// ─── Podstawowe Style ─────────────────────────────────────────────────────

var (
	StyleTitle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	StyleBold  = lipgloss.NewStyle().Bold(true).Foreground(ColorText)
	StyleLabel = lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true)
	StyleValue = lipgloss.NewStyle().Foreground(ColorText)
	StyleMuted = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleCode  = lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBgAlt).Padding(0, 1)

	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	StyleInfo    = lipgloss.NewStyle().Foreground(ColorInfo)

	StyleDivider = lipgloss.NewStyle().Foreground(ColorSurface)
	StyleDividerAccent = lipgloss.NewStyle().Foreground(ColorOverlay)

	// Panele z obramowaniem
	StylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSurface).
			Padding(0, 1)

	StylePanelAccent = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	StylePanelError = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorError).
			Padding(0, 1)

	StylePanelSuccess = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorSuccess).
				Padding(0, 1)

	// Elementy interaktywne
	StyleSelected = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	StyleInput = lipgloss.NewStyle().
			Foreground(ColorText).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorOverlay)

	StyleInputFocused = lipgloss.NewStyle().
				Foreground(ColorText).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(ColorPrimary)

	StyleButton = lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorPrimary).
			Padding(0, 2).Bold(true)
	StyleButtonSecondary = lipgloss.NewStyle().
				Foreground(ColorText).Background(ColorSurface).
				Padding(0, 2)

	// Etapy potoku
	StyleStagePending = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleStageRunning = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	StyleStageSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleStageFailed  = lipgloss.NewStyle().Foreground(ColorError)
	StyleStageWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleStageSkipped = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
)

// ─── Zakładki (Tabs) ─────────────────────────────────────────────────────

// TabStyle renderuje pasek zakładek.
// active: indeks aktywnej zakładki (0-based), labels: etykiety zakładek.
// activeColor: kolor tła aktywnej zakładki.
func RenderTabs(labels []string, active int, activeColor lipgloss.Color) string {
	var parts []string
	for i, label := range labels {
		if i == active {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(activeColor).
				Bold(true).
				Padding(0, 2).
				Render(label))
		} else {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(ColorSubtext).
				Background(ColorSurface).
				Padding(0, 2).
				Render(label))
		}
	}
	return " " + strings.Join(parts, " ") + " "
}

// ─── Karty sekcji ─────────────────────────────────────────────────────────

// SectionHeader zwraca nagłówek sekcji z poziomą linią.
func SectionHeader(icon, title string, width int) string {
	label := lipgloss.NewStyle().
		Foreground(ColorSubtext).
		Bold(true).
		Render(icon + "  " + title)
	labelLen := lipgloss.Width(label)
	lineLen := width - labelLen - 3
	if lineLen < 1 {
		lineLen = 1
	}
	line := lipgloss.NewStyle().Foreground(ColorSurface).Render(strings.Repeat("─", lineLen))
	return " " + label + "  " + line
}

// Badge renderuje kolorowy badge.
func Badge(text string, fg, bg lipgloss.Color) string {
	return lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Padding(0, 1).
		Bold(true).
		Render(text)
}

// ─── Środowiska ──────────────────────────────────────────────────────────

// EnvColor zwraca kolor dla nazwy środowiska.
func EnvColor(envName string) lipgloss.Color {
	switch envName {
	case "production":
		return ColorProduction
	case "development":
		return ColorDevelopment
	default:
		return ColorInfo
	}
}

// EnvStyle zwraca styl dla nazwy środowiska.
func EnvStyle(envName string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(EnvColor(envName)).Bold(true)
}

// EnvBadge zwraca badge środowiska.
func EnvBadge(envName string) string {
	c := EnvColor(envName)
	bg := lipgloss.Color("#1E1E2E")
	switch envName {
	case "production":
		bg = lipgloss.Color("#3B1219")
	case "development":
		bg = lipgloss.Color("#1A2E1A")
	}
	return lipgloss.NewStyle().
		Foreground(c).
		Background(bg).
		Bold(true).
		Padding(0, 1).
		Render(envName)
}

// ─── Etapy potoku ─────────────────────────────────────────────────────────

func StageIcon(status string) string {
	switch status {
	case "pending":
		return StyleStagePending.Render("  ○")
	case "running":
		return StyleStageRunning.Render("  ◉")
	case "success":
		return StyleStageSuccess.Render("  ✓")
	case "failed":
		return StyleStageFailed.Render("  ✗")
	case "warning":
		return StyleStageWarning.Render("  ⚠")
	case "skipped":
		return StyleStageSkipped.Render("  ⊘")
	default:
		return "   "
	}
}

// ─── Pasek klawiszy ───────────────────────────────────────────────────────

// KeyBar renderuje dolny pasek klawiszy.
func KeyBar(bindings []KeyBinding, width int) string {
	var parts []string
	for _, b := range bindings {
		parts = append(parts,
			lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(ColorSurface).
				Padding(0, 1).
				Render(b.Key)+
				lipgloss.NewStyle().Foreground(ColorSubtext).Render(" "+b.Action),
		)
	}

	bar := strings.Join(parts, "  ")
	line := lipgloss.NewStyle().Foreground(ColorSurface).Render(strings.Repeat("─", width))
	return line + "\n" + lipgloss.NewStyle().Padding(0, 1).Render(bar)
}

// KeyBinding to definicja skrótu klawiszowego.
type KeyBinding struct {
	Key    string
	Action string
}

// ─── Layout Helpers ──────────────────────────────────────────────────────

// CenterText centruje tekst.
func CenterText(text string, width int) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, text)
}

// Divider tworzy linię podziału.
func Divider(width int) string {
	return StyleDivider.Render(repeatChar("─", width))
}

// DividerAccent tworzy widoczniejszą linię.
func DividerAccent(width int) string {
	return StyleDividerAccent.Render(repeatChar("─", width))
}

func repeatChar(char string, n int) string {
	if n <= 0 {
		return ""
	}
	r := make([]byte, 0, n*len(char))
	for i := 0; i < n; i++ {
		r = append(r, []byte(char)...)
	}
	return string(r)
}

// ─── Logo / Banner ────────────────────────────────────────────────────────

// RnrLogo zwraca ASCII art logo narzędzia rnr.
func RnrLogo() string {
	logo := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(
		`  ██████╗ ███╗   ██╗██████╗
  ██╔══██╗████╗  ██║██╔══██╗
  ██████╔╝██╔██╗ ██║██████╔╝
  ██╔══██╗██║╚██╗██║██╔══██╗
  ██║  ██║██║ ╚████║██║  ██║
  ╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝`)
	subtitle := lipgloss.NewStyle().Foreground(ColorSubtext).Italic(true).
		Render("  runner — deployment bez stresu")
	return logo + "\n" + subtitle
}
