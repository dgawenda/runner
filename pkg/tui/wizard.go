// file: pkg/tui/wizard.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Setup Wizard — Interaktywna Konfiguracja Zero-Config               ║
// ║                                                                      ║
// ║  Prowadzi nowego użytkownika przez konfigurację krok po kroku.      ║
// ║  Maskuje pola haseł, generuje pliki konfiguracyjne po zakończeniu.  ║
// ╚══════════════════════════════════════════════════════════════════════╝

package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Typy ──────────────────────────────────────────────────────────────────

// WizardStep to krok kreatora konfiguracji.
type WizardStep int

const (
	wizardStepWelcome WizardStep = iota
	wizardStepProjectName
	wizardStepRepo           // URL klonu GitHub (https:// lub git@) lub puste = repo lokalne
	wizardStepGitHubCLI      // Sprawdzenie i konfiguracja GitHub CLI (gh)
	wizardStepGitHubConnect  // Potwierdzenie remote (HTTPS / SSH / gh / Pomiń)
	wizardStepProjectType    // HTML/npm/custom — wpływa na generowane stage'y
	wizardStepDeployProvider
	wizardStepNetlifyToken
	wizardStepNetlifySiteMode // Czy masz już site? Wybierz: istniejący / utwórz nowy
	wizardStepNetlifySiteID
	wizardStepDBProvider
	wizardStepSupabaseRef
	wizardStepSupabaseURL
	wizardStepSupabaseKey
	wizardStepReview
	wizardStepDone
)

// DeployProviderChoice to wybór dostawcy wdrożenia w wizardzie.
type DeployProviderChoice struct {
	Label string
	Value string
	Emoji string
	Desc  string
}

// projectTypeChoices — typ projektu determinuje generowane etapy pipeline.
var projectTypeChoices = []DeployProviderChoice{
	{"Statyczny HTML/CSS", "html", "🌐", "Tylko pliki HTML — bez budowania, bez npm"},
	{"Node.js / npm", "npm", "📦", "React, Vue, Next.js, Vite — wymaga npm"},
	{"Python / inne", "python", "🐍", "Dowolny projekt z własnym skryptem build"},
	{"Custom (własne komendy)", "custom", "⚙️ ", "Ręcznie zdefiniujesz etapy w rnr.yaml"},
}

var deployProviders = []DeployProviderChoice{
	{"Netlify", "netlify", "🌿", "Frontend hosting, JAMstack (zalecany)"},
	{"Vercel", "vercel", "▲", "Frontend hosting, Next.js"},
	{"SSH/rsync", "ssh", "📡", "Własny serwer VPS"},
	{"GitHub Pages", "gh-pages", "📄", "Statyczne strony GitHub"},
	{"Docker", "docker", "🐳", "Konteneryzacja"},
	{"Custom", "custom", "⚙️ ", "Własna komenda deploy"},
}

// netlifySiteModes — tryb konfiguracji projektu Netlify.
var netlifySiteModes = []DeployProviderChoice{
	{"Mam już Site ID", "existing", "🔗", "Wkleję Site ID z panelu Netlify"},
	{"Utwórz nowy projekt", "create", "✨", "rnr automatycznie założy nowy projekt Netlify"},
}

// gitHubCLIModes — czy używać GitHub CLI.
var gitHubCLIModes = []DeployProviderChoice{
	{"Używam gh CLI (GitHub CLI)", "gh", "🐙", "rnr użyje `gh` do tworzenia repo i autoryzacji"},
	{"Pomiń gh — tylko git push", "git", "📁", "Klasyczny git push (wymaga skonfigurowanego tokenu/SSH)"},
}

// gitHubConnectModes — tryb połączenia z GitHub remote.
var gitHubConnectModes = []DeployProviderChoice{
	{"HTTPS (https://github.com/...)", "https", "🔒", "Standardowe HTTPS (podałeś URL → zostanie ustawiony)"},
	{"SSH (git@github.com:...)", "ssh", "🔑", "SSH (podałeś URL → zostanie ustawiony)"},
	{"Pomiń remote — repo tylko lokalne", "skip", "📁", "Bez remote origin; push ręcznie kiedy chcesz"},
}

var dbProviders = []DeployProviderChoice{
	{"Supabase", "supabase", "⚡", "BaaS z PostgreSQL (zalecany)"},
	{"Prisma ORM", "prisma", "🔷", "TypeScript ORM"},
	{"PostgreSQL", "postgres", "🐘", "Bezpośrednie połączenie"},
	{"MySQL", "mysql", "🐬", "Bezpośrednie połączenie"},
	{"Brak bazy", "none", "○", "Aplikacja bezstanowa"},
	{"Custom", "custom", "⚙️ ", "Własna komenda migracji"},
}

// ─── Model Wizarda ─────────────────────────────────────────────────────────

// WizardModel to model sub-aplikacji Setup Wizard.
type WizardModel struct {
	step          WizardStep
	width         int
	height        int

	// Pola tekstowe
	inputs        []textinput.Model
	activeInput   int

	// Wybory dostawców
	projectTypeChoice      int // indeks w projectTypeChoices
	deployChoice           int
	dbChoice               int
	netlifySiteModeChoice  int // 0 = istniejący, 1 = utwórz nowy
	gitHubCLIChoice        int // 0 = użyj gh, 1 = pomiń gh
	gitHubConnectChoice    int // 0 = HTTPS, 1 = SSH, 2 = pomiń remote

	// Stan GitHub CLI
	ghCLIAvailable bool // true gdy `gh` jest w PATH

	// Zebrane dane
	projectName      string
	repo             string
	projectType      string // "html", "npm", "python", "custom"
	deployProv       string
	netlifyToken     string
	netlifySiteID    string
	netlifyCreateNew bool   // true = rnr sam tworzy projekt na Netlify
	gitHubRemoteURL  string // URL remote GitHub (puste = pomiń)
	dbProv           string
	supabaseRef      string
	supabaseURL      string
	supabaseKey      string

	// Tryb "fresh clone" — rnr.yaml istnieje, brak conf
	hasExistingConf bool

	// Błędy walidacji
	validationErr string
}

// NewWizardModel tworzy nowy model wizarda.
func NewWizardModel(width, height int) WizardModel {
	return NewWizardModelWithFlags(width, height, false)
}

// NewWizardModelWithFlags tworzy model wizarda ze wskaźnikiem czy conf już istnieje.
// hasExistingConf=true gdy rnr.yaml jest obecny ale brak rnr.conf.yaml (np. po git clone).
func NewWizardModelWithFlags(width, height int, hasExistingConf bool) WizardModel {
	m := WizardModel{
		step:            wizardStepWelcome,
		width:           width,
		height:          height,
		hasExistingConf: hasExistingConf,
	}
	m.initInputs()
	return m
}

// initInputs tworzy wszystkie pola tekstowe.
func (m *WizardModel) initInputs() {
	// input[0] = project name
	pName := textinput.New()
	pName.Placeholder = "moj-projekt"
	pName.CharLimit = 64
	pName.Width = 40

	// input[1] = repo (pełny URL klonu, np. https://github.com/owner/repo.git)
	repo := textinput.New()
	repo.Placeholder = "https://github.com/owner/repo.git"
	repo.CharLimit = 256
	repo.Width = 52

	// input[2] = netlify token (masked)
	netlifyToken := textinput.New()
	netlifyToken.Placeholder = "nfp_xxxxxxxxxxxxxxxx"
	netlifyToken.EchoMode = textinput.EchoPassword
	netlifyToken.EchoCharacter = '●'
	netlifyToken.CharLimit = 256
	netlifyToken.Width = 40

	// input[3] = netlify site ID
	netlifySiteID := textinput.New()
	netlifySiteID.Placeholder = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	netlifySiteID.CharLimit = 64
	netlifySiteID.Width = 40

	// input[4] = supabase ref
	supabaseRef := textinput.New()
	supabaseRef.Placeholder = "abcdefghijklmnop"
	supabaseRef.CharLimit = 64
	supabaseRef.Width = 40

	// input[5] = supabase db url (masked)
	supabaseURL := textinput.New()
	supabaseURL.Placeholder = "postgresql://postgres:[password]@db.xxx.supabase.co:5432/postgres"
	supabaseURL.EchoMode = textinput.EchoPassword
	supabaseURL.EchoCharacter = '●'
	supabaseURL.CharLimit = 512
	supabaseURL.Width = 40

	// input[6] = supabase anon key (masked)
	supabaseKey := textinput.New()
	supabaseKey.Placeholder = "eyJhbGciOiJIUzI1NiIsInR5cCI..."
	supabaseKey.EchoMode = textinput.EchoPassword
	supabaseKey.EchoCharacter = '●'
	supabaseKey.CharLimit = 512
	supabaseKey.Width = 40

	m.inputs = []textinput.Model{
		pName, repo, netlifyToken, netlifySiteID,
		supabaseRef, supabaseURL, supabaseKey,
	}
}

// ─── Interfejs Bubble Tea ─────────────────────────────────────────────────

// Init inicjalizuje model wizarda.
func (m WizardModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update obsługuje zdarzenia.
func (m WizardModel) Update(msg tea.Msg) (WizardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	// Aktualizuj aktywny input
	var cmd tea.Cmd
	if m.activeInput < len(m.inputs) {
		m.inputs[m.activeInput], cmd = m.inputs[m.activeInput].Update(msg)
	}
	return m, cmd
}

// View renderuje aktualny krok wizarda.
func (m WizardModel) View() string {
	switch m.step {
	case wizardStepWelcome:
		return m.viewWelcome()
	case wizardStepProjectName:
		return m.viewInput(0, "Nazwa projektu", "Jak nazywa się Twój projekt?",
			"Będzie wyświetlana w Dashboard i logach wdrożeń.")
	case wizardStepRepo:
		return m.viewInput(1, "URL repozytorium GitHub",
			"Podaj pełny URL klonu repozytorium (lub pozostaw puste)",
			"HTTPS: https://github.com/owner/repo.git\n"+
				"SSH:   git@github.com:owner/repo.git\n\n"+
				"Jeśli pominiesz, rnr utworzy lokalne repo Git automatycznie.\n"+
				"Remote możesz dodać później: git remote add origin <url>")
	case wizardStepGitHubCLI:
		return m.viewGitHubCLI()
	case wizardStepGitHubConnect:
		return m.viewGitHubConnect()
	case wizardStepProjectType:
		return m.viewProviderChoice("Typ projektu", projectTypeChoices, m.projectTypeChoice)
	case wizardStepDeployProvider:
		return m.viewProviderChoice("Dostawca wdrożenia", deployProviders, m.deployChoice)
	case wizardStepNetlifyToken:
		return m.viewInput(2, "Token Netlify", "Wklej swój Netlify Auth Token",
			"Znajdziesz go w Netlify → User Settings → Personal access tokens.\nToken jest MASKOWANY i nigdy nie trafi do logów ani git.")
	case wizardStepNetlifySiteMode:
		return m.viewProviderChoice("Projekt Netlify", netlifySiteModes, m.netlifySiteModeChoice)
	case wizardStepNetlifySiteID:
		return m.viewInput(3, "Site ID Netlify", "Wklej Site ID swojego projektu",
			"Znajdziesz go w Netlify → Site settings → General → Site ID.")
	case wizardStepDBProvider:
		return m.viewProviderChoice("Dostawca bazy danych", dbProviders, m.dbChoice)
	case wizardStepSupabaseRef:
		return m.viewInput(4, "Project Ref Supabase", "Podaj Reference ID projektu Supabase",
			"Znajdziesz go w Supabase Dashboard → Project Settings → General.")
	case wizardStepSupabaseURL:
		return m.viewInput(5, "Database URL Supabase", "Wklej Database URL",
			"Znajdziesz go w Supabase → Project Settings → Database → Connection string.\nURL jest MASKOWANY — nie trafi do logów.")
	case wizardStepSupabaseKey:
		return m.viewInput(6, "Anon Key Supabase", "Wklej klucz anon/public",
			"Znajdziesz go w Supabase → Project Settings → API → anon public.\nKlucz jest MASKOWANY.")
	case wizardStepReview:
		return m.viewReview()
	case wizardStepDone:
		return m.viewDone()
	}
	return ""
}

// ─── Widoki ───────────────────────────────────────────────────────────────

func (m WizardModel) viewWelcome() string {
	maxW := min(m.width-4, 72)

	var title, welcomeText, noteText string

	if m.hasExistingConf {
		// Sytuacja: sklonowany repo — jest rnr.yaml, brak rnr.conf.yaml
		title = StyleTitle.Render("🔑 rnr — Uzupełnij Credentials")
		welcomeText = "Wykryłem plik rnr.yaml w tym projekcie.\n\n" +
			"Brakuje jednak pliku rnr.conf.yaml z Twoimi tokenami\n" +
			"(jest gitignored — każdy developer trzyma swój lokalnie).\n\n" +
			"Uzupełnię Twój plik rnr.conf.yaml krok po kroku:\n\n" +
			"  • Podasz dostawcę wdrożenia i token\n" +
			"  • Opcjonalnie skonfigurujesz bazę danych\n" +
			"  • Wygeneruję TYLKO rnr.conf.yaml (lokalnie)"
		noteText = "ℹ️  Pipeline (rnr.yaml) już istnieje — nie zostanie nadpisany"
	} else {
		// Sytuacja: świeży projekt, brak obu plików
		title = StyleTitle.Render("🚀 Witaj w rnr — Kreator Konfiguracji")
		welcomeText = "Nie znalazłem pliku konfiguracyjnego rnr.conf.yaml.\n\n" +
			"Przeprowadzę Cię przez szybką konfigurację:\n\n" +
			"  • Wybierzesz dostawcę wdrożenia (np. Netlify)\n" +
			"  • Opcjonalnie skonfigurujesz bazę danych\n" +
			"  • Wygeneruję pliki konfiguracyjne automatycznie\n\n" +
			"Hasła i tokeny są MASKOWANE — nigdy nie trafią do logów."
		noteText = "✅ Plik rnr.conf.yaml zostanie automatycznie dodany do .gitignore"
	}

	welcome := lipgloss.NewStyle().
		Foreground(ColorText).
		Width(maxW - 4).
		Render(welcomeText)

	noteColor := ColorSuccess
	if m.hasExistingConf {
		noteColor = ColorInfo
	}
	note := lipgloss.NewStyle().
		Foreground(noteColor).
		Italic(true).
		Width(maxW - 4).
		Render(noteText)

	keyHint := StyleMuted.Render("\n  Naciśnij ENTER aby rozpocząć • ESC aby anulować")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		welcome,
		"",
		note,
		keyHint,
	)

	return StylePanelAccent.Width(maxW).Render(content)
}

func (m WizardModel) viewInput(inputIdx int, title, question, hint string) string {
	maxW := min(m.width-4, 72)

	// Aktywuj właściwy input
	for i := range m.inputs {
		if i == inputIdx {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}

	stepInfo := StyleMuted.Render(fmt.Sprintf("  Krok %d z %d", int(m.step), wizardStepReview))
	titleStr := StyleTitle.Render("✏️  " + title)
	questionStr := lipgloss.NewStyle().Foreground(ColorText).Render(question)
	hintStr := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).Render(hint)

	inputStyle := StyleInputFocused.Width(maxW - 8)
	inputStr := inputStyle.Render(m.inputs[inputIdx].View())

	var errStr string
	if m.validationErr != "" {
		errStr = "\n" + StyleError.Render("  ⚠ "+m.validationErr)
	}

	navHint := StyleMuted.Render("\n  ENTER = następny krok • ESC = cofnij • Ctrl+C = anuluj")

	content := lipgloss.JoinVertical(lipgloss.Left,
		stepInfo, "",
		titleStr, "",
		questionStr, "",
		hintStr, "",
		inputStr,
		errStr,
		navHint,
	)

	return StylePanelAccent.Width(maxW).Render(content)
}

func (m WizardModel) viewProviderChoice(title string, choices []DeployProviderChoice, selected int) string {
	maxW := min(m.width-4, 72)

	stepInfo := StyleMuted.Render(fmt.Sprintf("  Krok %d z %d", int(m.step), wizardStepReview))
	titleStr := StyleTitle.Render("🔧 " + title)
	instruction := lipgloss.NewStyle().Foreground(ColorSubtext).
		Render("Wybierz używając strzałek ↑↓, potwierdź ENTER:")

	var items strings.Builder
	for i, choice := range choices {
		var row string
		if i == selected {
			row = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				Render(fmt.Sprintf("  ▶ %s %-12s — %s", choice.Emoji, choice.Label, choice.Desc))
		} else {
			row = lipgloss.NewStyle().
				Foreground(ColorSubtext).
				Render(fmt.Sprintf("    %s %-12s — %s", choice.Emoji, choice.Label, choice.Desc))
		}
		items.WriteString(row + "\n")
	}

	navHint := StyleMuted.Render("\n  ↑↓ = nawigacja • ENTER = wybierz • ESC = cofnij")

	content := lipgloss.JoinVertical(lipgloss.Left,
		stepInfo, "",
		titleStr, "",
		instruction, "",
		items.String(),
		navHint,
	)

	return StylePanelAccent.Width(maxW).Render(content)
}

// viewGitHubCLI renderuje krok sprawdzenia / konfiguracji GitHub CLI (gh).
// Sprawdza czy `gh` jest dostępne i pokazuje instrukcję instalacji jeśli nie.
func (m WizardModel) viewGitHubCLI() string {
	maxW := min(m.width-4, 72)

	stepInfo := StyleMuted.Render(fmt.Sprintf("  Krok %d z %d", int(m.step), wizardStepReview))
	titleStr := StyleTitle.Render("🐙 GitHub CLI (gh)")
	instruction := lipgloss.NewStyle().Foreground(ColorSubtext).
		Render("GitHub CLI pozwala `rnr` tworzyć repozytoria i autoryzować push\nbez ręcznego konfigurowania tokenów SSH/HTTPS.")

	// Status gh CLI
	var ghStatusStr string
	if m.ghCLIAvailable {
		ghStatusStr = StyleSuccess.Render("✅  gh CLI wykryty — możesz używać opcji GitHub CLI")
	} else {
		ghStatusStr = StyleWarning.Render("⚠  gh CLI nie znaleziono w PATH")
	}

	// Instrukcja instalacji (pokazuj zawsze jako hint)
	installStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4")).Italic(true)
	installHint := installStyle.Render(
		"Instalacja gh CLI:\n" +
			"  Linux:  sudo apt install gh\n" +
			"  macOS:  brew install gh\n" +
			"  Windows: winget install --id GitHub.cli\n" +
			"  Więcej: https://cli.github.com/\n\n" +
			"Po instalacji: gh auth login")

	// Lista wyboru
	var items strings.Builder
	for i, choice := range gitHubCLIModes {
		var row string
		if i == m.gitHubCLIChoice {
			row = lipgloss.NewStyle().
				Foreground(ColorPrimary).Bold(true).
				Render(fmt.Sprintf("  ▶ %s %-28s — %s", choice.Emoji, choice.Label, choice.Desc))
		} else {
			row = lipgloss.NewStyle().
				Foreground(ColorSubtext).
				Render(fmt.Sprintf("    %s %-28s — %s", choice.Emoji, choice.Label, choice.Desc))
		}
		items.WriteString(row + "\n")
	}

	navHint := StyleMuted.Render("\n  ↑↓ = nawigacja • ENTER = wybierz • ESC = cofnij")

	content := lipgloss.JoinVertical(lipgloss.Left,
		stepInfo, "",
		titleStr, "",
		instruction, "",
		ghStatusStr, "",
		installHint, "",
		items.String(),
		navHint,
	)
	return StylePanelAccent.Width(maxW).Render(content)
}

// viewGitHubConnect renderuje krok potwierdzenia URL remote i sposobu połączenia.
// URL jest już znany (podany w wizardStepRepo) — ten krok tylko potwierdza.
func (m WizardModel) viewGitHubConnect() string {
	maxW := min(m.width-4, 72)

	repoURL := strings.TrimSpace(m.inputs[1].Value())
	stepInfo := StyleMuted.Render(fmt.Sprintf("  Krok %d z %d", int(m.step), wizardStepReview))
	titleStr := StyleTitle.Render("🔗 Konfiguracja Git Remote")
	instruction := lipgloss.NewStyle().Foreground(ColorSubtext).
		Render("Wybierz jak `rnr` ma skonfigurować remote origin:")

	// Pokaż URL który wpisał użytkownik
	urlLabel := ""
	if repoURL != "" {
		urlLabel = "\n  " + StyleLabel.Render("URL:") + "  " +
			lipgloss.NewStyle().Foreground(ColorInfo).Italic(true).Render(repoURL)
	}

	var items strings.Builder
	for i, choice := range gitHubConnectModes {
		// Dynamicznie dopasuj opcję do podanego URL
		label := choice.Label
		desc := choice.Desc
		if i == 0 && strings.HasPrefix(repoURL, "https://") {
			label = "HTTPS ✓ (wykryto w URL)"
			desc = "Użyj podanego URL jako remote origin"
		} else if i == 1 && strings.HasPrefix(repoURL, "git@") {
			label = "SSH ✓ (wykryto w URL)"
			desc = "Użyj podanego URL jako remote origin"
		}
		var row string
		if i == m.gitHubConnectChoice {
			row = lipgloss.NewStyle().
				Foreground(ColorPrimary).Bold(true).
				Render(fmt.Sprintf("  ▶ %s %-32s — %s", choice.Emoji, label, desc))
		} else {
			row = lipgloss.NewStyle().Foreground(ColorSubtext).
				Render(fmt.Sprintf("    %s %-32s — %s", choice.Emoji, label, desc))
		}
		items.WriteString(row + "\n")
	}

	// Ostrzeżenie gdy brak URL a wybrano HTTPS/SSH
	var warnStr string
	if repoURL == "" && m.gitHubConnectChoice < 2 {
		warnStr = "\n  " + StyleWarning.Render(
			"⚠  Nie podałeś URL repo — cofnij i uzupełnij,\n"+
				"   lub wybierz 'Pomiń remote' i skonfiguruj ręcznie.")
	}

	// Podpowiedź dla gh CLI
	ghHint := ""
	if m.gitHubCLIChoice == 0 && m.ghCLIAvailable {
		ghHint = "\n  " + StyleSuccess.Render("🐙 gh CLI dostępny — repo zostanie zautomatycznie zsynchronizowane")
	} else if m.gitHubCLIChoice == 0 && !m.ghCLIAvailable {
		ghHint = "\n  " + StyleWarning.Render("⚠  gh CLI niedostępny — instalacja pominięta, używam git")
	}

	navHint := StyleMuted.Render("\n  ↑↓ = nawigacja • ENTER = wybierz • ESC = cofnij")

	content := lipgloss.JoinVertical(lipgloss.Left,
		stepInfo, "",
		titleStr, "",
		instruction,
		urlLabel, "",
		items.String(),
		ghHint,
		warnStr,
		navHint,
	)
	return StylePanelAccent.Width(maxW).Render(content)
}

func (m WizardModel) viewReview() string {
	maxW := min(m.width-4, 72)

	titleStr := StyleTitle.Render("📋 Podsumowanie konfiguracji")
	subStr := lipgloss.NewStyle().Foreground(ColorSubtext).
		Render("Sprawdź poniższe dane przed wygenerowaniem plików:")

	row := func(label, value string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top,
			StyleLabel.Width(22).Render(label+":"),
			StyleValue.Render(value),
		)
	}

	deployProv := m.deployProv
	if deployProv == "" {
		deployProv = "netlify"
	}
	dbProv := m.dbProv
	if dbProv == "" {
		dbProv = "none"
	}

	netlifyMode := ""
	if deployProv == "netlify" {
		if m.netlifyCreateNew {
			netlifyMode = "✨ Utwórz nowy projekt automatycznie"
		} else if m.netlifySiteID != "" {
			netlifyMode = "🔗 Site ID: " + m.netlifySiteID
		}
	}

	projectTypeName := m.projectType
	for _, pt := range projectTypeChoices {
		if pt.Value == m.projectType {
			projectTypeName = pt.Emoji + " " + pt.Label
			break
		}
	}
	if projectTypeName == "" {
		projectTypeName = "custom"
	}

	// Określ label dla repo
	repoLabel := m.repo
	if repoLabel == "" {
		repoLabel = "(brak — repo lokalne)"
	}

	rows := []string{
		row("Projekt", m.projectName),
		row("Typ projektu", projectTypeName),
		row("Dostawca deploy", deployProv),
	}
	if netlifyMode != "" {
		rows = append(rows, row("Netlify projekt", netlifyMode))
	}
	if m.projectType != "html" {
		rows = append(rows, row("Dostawca bazy", dbProv))
	}
	if m.gitHubRemoteURL != "" {
		rows = append(rows, row("Clone URL (remote)", m.gitHubRemoteURL))
	} else if m.repo != "" {
		rows = append(rows, row("Clone URL", repoLabel))
		rows = append(rows, row("Remote origin", "(pominięty — repo tylko lokalne)"))
	} else {
		rows = append(rows, row("Remote origin", "(brak — zostanie utworzone repo lokalne)"))
	}
	if m.gitHubCLIChoice == 0 && m.ghCLIAvailable {
		rows = append(rows, row("GitHub CLI (gh)", "✅ dostępny"))
	}

	summary := lipgloss.JoinVertical(lipgloss.Left, rows...)

	note := lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Render("✅ Pliki zostaną wygenerowane:\n  • rnr.yaml (bezpieczny do commitowania)\n  • rnr.conf.yaml (gitignored — tylko lokalny)\n  • .rnr/ (katalog stanu)")

	navHint := StyleMuted.Render("\n  ENTER = generuj pliki • ESC = cofnij")

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStr, "", subStr, "", summary, "", note, navHint,
	)

	return StylePanelAccent.Width(maxW).Render(content)
}

func (m WizardModel) viewDone() string {
	maxW := min(m.width-4, 72)

	titleStr := StyleTitle.Render("✅ Konfiguracja gotowa!")
	body := lipgloss.NewStyle().Foreground(ColorText).
		Render(
			"Pliki zostały wygenerowane. Możesz teraz:\n\n" +
				"  • Edytować rnr.yaml — zdefiniować etapy potoku\n" +
				"  • Uzupełnić rnr.conf.yaml — wpisać credentials\n" +
				"  • Uruchomić rnr ponownie — otworzy się Dashboard\n\n" +
				"Pamiętaj: rnr.conf.yaml jest TYLKO LOKALNY — nie commituj go!")

	navHint := StyleMuted.Render("\n  ENTER = otwórz Dashboard • Q = wyjdź")

	content := lipgloss.JoinVertical(lipgloss.Left,
		"", titleStr, "", body, navHint,
	)

	return StylePanelSuccess.Width(maxW).Render(content)
}

// ─── Obsługa Klawiszy ─────────────────────────────────────────────────────

func (m WizardModel) handleKey(msg tea.KeyMsg) (WizardModel, tea.Cmd) {
	m.validationErr = ""

	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyEsc:
		if m.step > 0 {
			m.step--
		}
		return m, nil

	case tea.KeyEnter:
		return m.advance()

	case tea.KeyUp:
		switch m.step {
		case wizardStepProjectType:
			if m.projectTypeChoice > 0 {
				m.projectTypeChoice--
			}
		case wizardStepDeployProvider:
			if m.deployChoice > 0 {
				m.deployChoice--
			}
		case wizardStepNetlifySiteMode:
			if m.netlifySiteModeChoice > 0 {
				m.netlifySiteModeChoice--
			}
		case wizardStepGitHubCLI:
			if m.gitHubCLIChoice > 0 {
				m.gitHubCLIChoice--
			}
		case wizardStepGitHubConnect:
			if m.gitHubConnectChoice > 0 {
				m.gitHubConnectChoice--
			}
		case wizardStepDBProvider:
			if m.dbChoice > 0 {
				m.dbChoice--
			}
		}
		return m, nil

	case tea.KeyDown:
		switch m.step {
		case wizardStepProjectType:
			if m.projectTypeChoice < len(projectTypeChoices)-1 {
				m.projectTypeChoice++
			}
		case wizardStepDeployProvider:
			if m.deployChoice < len(deployProviders)-1 {
				m.deployChoice++
			}
		case wizardStepNetlifySiteMode:
			if m.netlifySiteModeChoice < len(netlifySiteModes)-1 {
				m.netlifySiteModeChoice++
			}
		case wizardStepGitHubCLI:
			if m.gitHubCLIChoice < len(gitHubCLIModes)-1 {
				m.gitHubCLIChoice++
			}
		case wizardStepGitHubConnect:
			if m.gitHubConnectChoice < len(gitHubConnectModes)-1 {
				m.gitHubConnectChoice++
			}
		case wizardStepDBProvider:
			if m.dbChoice < len(dbProviders)-1 {
				m.dbChoice++
			}
		}
		return m, nil
	}

	// Przekaż do aktywnego inputa
	var cmd tea.Cmd
	inputIdx := m.currentInputIndex()
	if inputIdx >= 0 && inputIdx < len(m.inputs) {
		m.inputs[inputIdx], cmd = m.inputs[inputIdx].Update(msg)
	}
	return m, cmd
}

// advance przechodzi do następnego kroku po walidacji.
func (m WizardModel) advance() (WizardModel, tea.Cmd) {
	switch m.step {
	case wizardStepWelcome:
		m.step = wizardStepProjectName
		m.inputs[0].Focus()

	case wizardStepProjectName:
		name := strings.TrimSpace(m.inputs[0].Value())
		if name == "" {
			m.validationErr = "Nazwa projektu nie może być pusta"
			return m, nil
		}
		m.projectName = name
		m.step = wizardStepRepo
		m.inputs[1].Focus()

	case wizardStepRepo:
		m.repo = strings.TrimSpace(m.inputs[1].Value())
		// Zawsze idź przez krok GitHub CLI (gdy repo puste = repo lokalne)
		// Sprawdź dostępność gh CLI
		m.ghCLIAvailable = isGhCLIAvailable()
		// Jeśli URL jest HTTPS lub SSH, auto-ustaw indeks w gitHubConnectChoice
		if strings.HasPrefix(m.repo, "https://") {
			m.gitHubConnectChoice = 0 // HTTPS
		} else if strings.HasPrefix(m.repo, "git@") {
			m.gitHubConnectChoice = 1 // SSH
		}
		if m.repo != "" {
			// Mamy URL — idź przez gh CLI + connect
			m.step = wizardStepGitHubCLI
		} else {
			// Brak URL → repo lokalne, pomiń GitHub kroki
			m.gitHubRemoteURL = ""
			m.step = wizardStepProjectType
		}

	case wizardStepGitHubCLI:
		// Zapisz wybór gh CLI, idź do connect
		m.step = wizardStepGitHubConnect

	case wizardStepGitHubConnect:
		// Użyj URL podanego przez użytkownika bezpośrednio (jest już pełny)
		repoURL := strings.TrimSpace(m.inputs[1].Value())
		switch m.gitHubConnectChoice {
		case 0, 1: // HTTPS lub SSH — użyj podanego URL
			m.gitHubRemoteURL = repoURL
		case 2: // Pomiń remote
			m.gitHubRemoteURL = ""
		}
		m.step = wizardStepProjectType

	case wizardStepProjectType:
		m.projectType = projectTypeChoices[m.projectTypeChoice].Value
		m.step = wizardStepDeployProvider

	case wizardStepDeployProvider:
		m.deployProv = deployProviders[m.deployChoice].Value
		if m.deployProv == "netlify" {
			m.step = wizardStepNetlifyToken
			m.inputs[2].Focus()
		} else if m.projectType == "html" {
			// HTML bez netlify nie ma DB — idź do review
			m.dbProv = "none"
			m.step = wizardStepReview
		} else {
			m.step = wizardStepDBProvider
		}

	case wizardStepNetlifyToken:
		m.netlifyToken = m.inputs[2].Value()
		m.step = wizardStepNetlifySiteMode

	case wizardStepNetlifySiteMode:
		choice := netlifySiteModes[m.netlifySiteModeChoice].Value
		nextStep := wizardStepDBProvider
		if m.projectType == "html" {
			// HTML nie ma bazy — pomijamy DB
			m.dbProv = "none"
			nextStep = wizardStepReview
		}
		if choice == "create" {
			m.netlifyCreateNew = true
			m.netlifySiteID = ""
			m.step = nextStep
		} else {
			m.netlifyCreateNew = false
			m.step = wizardStepNetlifySiteID
			m.inputs[3].Focus()
		}

	case wizardStepNetlifySiteID:
		m.netlifySiteID = m.inputs[3].Value()
		// Projekty HTML nie mają bazy danych — pomijamy krok DB
		if m.projectType == "html" {
			m.dbProv = "none"
			m.step = wizardStepReview
		} else {
			m.step = wizardStepDBProvider
		}

	case wizardStepDBProvider:
		m.dbProv = dbProviders[m.dbChoice].Value
		if m.dbProv == "supabase" {
			m.step = wizardStepSupabaseRef
			m.inputs[4].Focus()
		} else if m.dbProv == "prisma" {
			// Prisma wymaga tylko db_url, ale to jest w conf — idź do review
			m.step = wizardStepReview
		} else {
			m.step = wizardStepReview
		}

	case wizardStepSupabaseRef:
		m.supabaseRef = strings.TrimSpace(m.inputs[4].Value())
		m.step = wizardStepSupabaseURL
		m.inputs[5].Focus()

	case wizardStepSupabaseURL:
		m.supabaseURL = m.inputs[5].Value()
		m.step = wizardStepSupabaseKey
		m.inputs[6].Focus()

	case wizardStepSupabaseKey:
		m.supabaseKey = m.inputs[6].Value()
		m.step = wizardStepReview

	case wizardStepReview:
		m.step = wizardStepDone
		return m, func() tea.Msg {
			return WizardCompleteMsg{
				ProjectName:      m.projectName,
				Repo:             m.repo,
				ProjectType:      m.projectType,
				DeployProv:       m.deployProv,
				NetlifyToken:     m.netlifyToken,
				NetlifySiteID:    m.netlifySiteID,
				NetlifyCreateNew: m.netlifyCreateNew,
				DBProv:           m.dbProv,
				SupabaseRef:      m.supabaseRef,
				SupabaseURL:      m.supabaseURL,
				SupabaseKey:      m.supabaseKey,
				GitHubRemoteURL:  m.gitHubRemoteURL,
				UseGhCLI:         m.gitHubCLIChoice == 0 && m.ghCLIAvailable,
			}
		}

	case wizardStepDone:
		return m, func() tea.Msg {
			return NavigateMsg{Screen: ScreenDashboard}
		}
	}

	return m, textinput.Blink
}

// currentInputIndex zwraca indeks aktywnego inputa dla bieżącego kroku.
func (m WizardModel) currentInputIndex() int {
	switch m.step {
	case wizardStepProjectName:
		return 0
	case wizardStepRepo:
		return 1
	case wizardStepNetlifyToken:
		return 2
	case wizardStepNetlifySiteID:
		return 3
	case wizardStepSupabaseRef:
		return 4
	case wizardStepSupabaseURL:
		return 5
	case wizardStepSupabaseKey:
		return 6
	default:
		return -1
	}
}

// WizardData zwraca zebrane dane z wizarda.
func (m WizardModel) WizardData() (projectName, repo, deployProv, netlifyToken, netlifySiteID string, netlifyCreateNew bool, dbProv, supabaseRef, supabaseURL, supabaseKey string) {
	return m.projectName, m.repo, m.deployProv,
		m.netlifyToken, m.netlifySiteID, m.netlifyCreateNew,
		m.dbProv, m.supabaseRef, m.supabaseURL, m.supabaseKey
}

// isGhCLIAvailable sprawdza czy GitHub CLI (gh) jest dostępne w PATH.
func isGhCLIAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
