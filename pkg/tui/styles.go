// file: pkg/tui/styles.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Definicje Stylów Wizualnych (Lipgloss)                             ║
// ║                                                                      ║
// ║  Centralna paleta kolorów i stylów dla całego interfejsu TUI.       ║
// ║  Projekt wizualny inspirowany Catppuccin Mocha — ciemne tło,       ║
// ║  pastelowe akcenty, wysoki kontrast dla czytelności w terminalu.    ║
// ╚══════════════════════════════════════════════════════════════════════╝

package tui

import "github.com/charmbracelet/lipgloss"

// ─── Paleta Kolorów ────────────────────────────────────────────────────────

const (
	// Kolory tła
	ColorBg      = lipgloss.Color("#1E1E2E") // Ciemny granat
	ColorBgAlt   = lipgloss.Color("#181825") // Jeszcze ciemniejszy
	ColorSurface = lipgloss.Color("#313244") // Panel, ramki
	ColorOverlay = lipgloss.Color("#45475A") // Subtelna ramka

	// Kolory tekstu
	ColorText    = lipgloss.Color("#CDD6F4") // Jasny niebieski-biały
	ColorSubtext = lipgloss.Color("#A6ADC8") // Przygaszony tekst
	ColorMuted   = lipgloss.Color("#6C7086") // Bardzo przygaszony

	// Kolory akcentów
	ColorPrimary   = lipgloss.Color("#CBA6F7") // Fioletowy (Mauve)
	ColorSecondary = lipgloss.Color("#89DCEB") // Cyjan (Sky)
	ColorAccent    = lipgloss.Color("#F5C2E7") // Różowy (Pink)

	// Kolory statusów
	ColorSuccess = lipgloss.Color("#A6E3A1") // Zielony (Green)
	ColorWarning = lipgloss.Color("#F9E2AF") // Żółty (Yellow)
	ColorError   = lipgloss.Color("#F38BA8") // Czerwony (Red)
	ColorInfo    = lipgloss.Color("#89B4FA") // Niebieski (Blue)

	// Kolory środowisk
	ColorProduction = lipgloss.Color("#F38BA8") // Czerwony = uwaga!
	ColorStaging    = lipgloss.Color("#F9E2AF") // Żółty = ostrożność
	ColorPreview    = lipgloss.Color("#A6E3A1") // Zielony = bezpieczny
)

// ─── Style Typograficzne ──────────────────────────────────────────────────

var (
	// StyleTitle — duży tytuł z fioletowym akcentem
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(0, 1)

	// StyleSubtitle — podtytuł
	StyleSubtitle = lipgloss.NewStyle().
			Foreground(ColorSubtext).
			Italic(true)

	// StyleLabel — etykieta pola
	StyleLabel = lipgloss.NewStyle().
			Foreground(ColorSubtext).
			Bold(true)

	// StyleValue — wartość pola
	StyleValue = lipgloss.NewStyle().
			Foreground(ColorText)

	// StyleMuted — przygaszony tekst
	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// StyleBold — pogrubiony tekst
	StyleBold = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	// StyleCode — kod/komenda
	StyleCode = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Background(ColorBgAlt).
			Padding(0, 1)
)

// ─── Style Statusów ───────────────────────────────────────────────────────

var (
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	StyleInfo    = lipgloss.NewStyle().Foreground(ColorInfo)
)

// ─── Style Ramek i Paneli ─────────────────────────────────────────────────

var (
	// StylePanel — główny panel z ramką
	StylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSurface).
			Padding(1, 2)

	// StylePanelAccent — panel z kolorową ramką
	StylePanelAccent = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	// StylePanelError — panel z czerwoną ramką
	StylePanelError = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorError).
			Padding(1, 2)

	// StylePanelSuccess — panel z zieloną ramką
	StylePanelSuccess = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorSuccess).
				Padding(1, 2)

	// StyleDivider — linia podziału
	StyleDivider = lipgloss.NewStyle().
			Foreground(ColorSurface)
)

// ─── Style Elementów Interaktywnych ──────────────────────────────────────

var (
	// StyleSelected — zaznaczony element listy
	StyleSelected = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	// StyleUnselected — niezaznaczony element listy
	StyleUnselected = lipgloss.NewStyle().
			Foreground(ColorSubtext)

	// StyleCursor — kursor wyboru
	StyleCursor = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	// StyleInput — pole tekstowe
	StyleInput = lipgloss.NewStyle().
			Foreground(ColorText).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorOverlay)

	// StyleInputFocused — aktywne pole tekstowe
	StyleInputFocused = lipgloss.NewStyle().
				Foreground(ColorText).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(ColorPrimary)

	// StyleButton — przycisk
	StyleButton = lipgloss.NewStyle().
			Foreground(ColorBg).
			Background(ColorPrimary).
			Padding(0, 3).
			Bold(true)

	// StyleButtonSecondary — przycisk drugorzędny
	StyleButtonSecondary = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(ColorSurface).
				Padding(0, 3)
)

// ─── Style Środowisk ─────────────────────────────────────────────────────

// EnvStyle zwraca styl dla nazwy środowiska.
func EnvStyle(envName string) lipgloss.Style {
	switch envName {
	case "production":
		return lipgloss.NewStyle().
			Foreground(ColorProduction).
			Bold(true).
			Padding(0, 1)
	case "staging":
		return lipgloss.NewStyle().
			Foreground(ColorStaging).
			Bold(true).
			Padding(0, 1)
	default:
		return lipgloss.NewStyle().
			Foreground(ColorPreview).
			Bold(true).
			Padding(0, 1)
	}
}

// EnvBadge zwraca badge środowiska (stylizowany tag).
func EnvBadge(envName string) string {
	return EnvStyle(envName).Render(" " + envName + " ")
}

// ─── Style Etapów ────────────────────────────────────────────────────────

var (
	// StyleStagePending — etap oczekujący
	StyleStagePending = lipgloss.NewStyle().Foreground(ColorMuted)

	// StyleStageRunning — etap w toku
	StyleStageRunning = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)

	// StyleStageSuccess — etap zakończony sukcesem
	StyleStageSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)

	// StyleStageFailed — etap zakończony błędem
	StyleStageFailed = lipgloss.NewStyle().Foreground(ColorError)

	// StyleStageWarning — etap z ostrzeżeniem (allow_failure)
	StyleStageWarning = lipgloss.NewStyle().Foreground(ColorWarning)

	// StyleStageSkipped — etap pominięty
	StyleStageSkipped = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
)

// StageIcon zwraca ikonę etapu.
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

// ─── Layout Helpers ───────────────────────────────────────────────────────

// CenterText centruje tekst w podanej szerokości.
func CenterText(text string, width int) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, text)
}

// Divider tworzy poziomą linię podziału.
func Divider(width int) string {
	return StyleDivider.Render(repeatChar("─", width))
}

// repeatChar powtarza znak n razy.
func repeatChar(char string, n int) string {
	if n <= 0 {
		return ""
	}
	result := make([]byte, 0, n*len(char))
	for i := 0; i < n; i++ {
		result = append(result, []byte(char)...)
	}
	return string(result)
}

// ─── Logo / Banner ────────────────────────────────────────────────────────

// RnrLogo zwraca ASCII art logo narzędzia rnr.
func RnrLogo() string {
	logo := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Render(`
  ██████╗ ███╗   ██╗██████╗
  ██╔══██╗████╗  ██║██╔══██╗
  ██████╔╝██╔██╗ ██║██████╔╝
  ██╔══██╗██║╚██╗██║██╔══██╗
  ██║  ██║██║ ╚████║██║  ██║
  ╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝`)

	subtitle := lipgloss.NewStyle().
		Foreground(ColorSubtext).
		Italic(true).
		Render("  runner — deployment bez stresu")

	return logo + "\n" + subtitle
}
