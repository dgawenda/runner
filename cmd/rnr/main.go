// file: cmd/rnr/main.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Punkt Wejścia — rnr CLI                                           ║
// ║                                                                      ║
// ║  Inicjalizuje narzędzie rnr i uruchamia interfejs TUI Bubble Tea.  ║
// ║  Zarządza Cobra CLI dla komend CLI (deploy, rollback, init, logs). ║
// ╚══════════════════════════════════════════════════════════════════════╝

package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/tui"
)

// Version jest wstrzykiwana podczas budowania przez ldflags.
var Version = "dev"

func main() {
	root := buildCLI()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "rnr: błąd:", err)
		os.Exit(1)
	}
}

// buildCLI buduje strukturę komend Cobra.
func buildCLI() *cobra.Command {
	var projectRoot string

	root := &cobra.Command{
		Use:   "rnr",
		Short: "rnr — deployment bez stresu",
		Long: `
  ██████╗ ███╗   ██╗██████╗
  ██╔══██╗████╗  ██║██╔══██╗
  ██████╔╝██╔██╗ ██║██████╔╝
  ██╔══██╗██║╚██╗██║██╔══██╗
  ██║  ██║██║ ╚████║██║  ██║
  ╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝

  runner ` + Version + ` — deployment bez stresu
  Dokumentacja: README.md
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(projectRoot)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&projectRoot, "dir", "C", "",
		"Katalog projektu (domyślnie: bieżący katalog)")

	// Komenda: init
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Uruchom Setup Wizard i wygeneruj pliki konfiguracyjne",
		Long: `Uruchamia interaktywny kreator konfiguracji.
Generuje pliki rnr.yaml i rnr.conf.yaml oraz aktualizuje .gitignore.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			root := resolveProjectRoot(projectRoot)

			if !force {
				_, hasConf := config.Exists(root)
				if hasConf {
					fmt.Fprintf(os.Stderr,
						"⚠️  Plik rnr.conf.yaml już istnieje.\n"+
							"Użyj flagi --force aby go nadpisać:\n"+
							"  rnr init --force\n")
					os.Exit(1)
				}
			}
			return runTUI(root)
		},
	}
	initCmd.Flags().Bool("force", false, "Nadpisz istniejącą konfigurację")
	root.AddCommand(initCmd)

	// Komenda: deploy
	deployCmd := &cobra.Command{
		Use:   "deploy [środowisko]",
		Short: "Wdróż na podane środowisko",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := "production"
			if len(args) > 0 {
				env = args[0]
			}
			fmt.Printf("🚀 Uruchamiam wdrożenie na '%s' przez TUI...\n", env)
			return runTUI(resolveProjectRoot(projectRoot))
		},
	}
	root.AddCommand(deployCmd)

	// Komenda: rollback
	rollbackCmd := &cobra.Command{
		Use:   "rollback [środowisko]",
		Short: "Przywróć poprzednią wersję na podanym środowisku",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := "production"
			if len(args) > 0 {
				env = args[0]
			}
			fmt.Printf("↩️  Uruchamiam rollback dla '%s' przez TUI...\n", env)
			return runTUI(resolveProjectRoot(projectRoot))
		},
	}
	root.AddCommand(rollbackCmd)

	// Komenda: promote
	promoteCmd := &cobra.Command{
		Use:   "promote",
		Short: "Przepuść migracje DB ze staging na production",
		Long: `Stosuje migracje bazy danych ze środowiska staging na production.
UWAGA: Operacja nieodwracalna! Używaj zasady 'roll-forward'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("📊 Uruchamiam promote przez TUI...")
			return runTUI(resolveProjectRoot(projectRoot))
		},
	}
	root.AddCommand(promoteCmd)

	// Komenda: logs
	logsCmd := &cobra.Command{
		Use:   "logs [środowisko]",
		Short: "Pokaż logi ostatnich wdrożeń",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := resolveProjectRoot(projectRoot)
			return showLogs(root, args)
		},
	}
	logsCmd.Flags().IntP("lines", "n", 50, "Liczba ostatnich linii logu")
	root.AddCommand(logsCmd)

	// Komenda: version
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Pokaż wersję rnr",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("rnr %s\n", Version)
		},
	}
	root.AddCommand(versionCmd)

	return root
}

// runTUI uruchamia główny interfejs TUI Bubble Tea.
func runTUI(projectRoot string) error {
	// Upewnij się że katalog .rnr istnieje
	if err := config.EnsureRnrDir(projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Nie można utworzyć .rnr/: %v\n", err)
		// Nie przerywaj — kontynuuj bez .rnr/
	}

	// Utwórz root model
	model := tui.NewRootModel(projectRoot)

	// Uruchom Bubble Tea z opcjami
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Pełnoekranowy tryb alternatywny
		tea.WithMouseCellMotion(), // Obsługa myszy
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("błąd TUI: %w", err)
	}

	return nil
}

// showLogs wyświetla logi wdrożeń w terminalu.
func showLogs(projectRoot string, args []string) error {
	logsDir := filepath.Join(projectRoot, config.LogsDir)

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Brak logów wdrożeń. Uruchom pierwsze wdrożenie.")
			return nil
		}
		return fmt.Errorf("nie można odczytać katalogu logów: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("Brak plików logów w", logsDir)
		return nil
	}

	// Pokaż ostatni log (lub przefiltrowany po środowisku)
	envFilter := ""
	if len(args) > 0 {
		envFilter = args[0]
	}

	var logFiles []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			if envFilter == "" || containsStr(e.Name(), envFilter) {
				logFiles = append(logFiles, e)
			}
		}
	}

	if len(logFiles) == 0 {
		fmt.Printf("Brak logów dla środowiska '%s'\n", envFilter)
		return nil
	}

	// Pokaż ostatni plik
	lastLog := logFiles[len(logFiles)-1]
	logPath := filepath.Join(logsDir, lastLog.Name())

	fmt.Printf("📄 Log: %s\n\n", logPath)

	data, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("nie można odczytać logu: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// resolveProjectRoot zwraca katalog projektu (bieżący jeśli nie podano).
func resolveProjectRoot(override string) string {
	if override != "" {
		abs, err := filepath.Abs(override)
		if err == nil {
			return abs
		}
		return override
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
