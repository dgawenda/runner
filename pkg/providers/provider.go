// file: pkg/providers/provider.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Interfejs Dostawców Zewnętrznych (Provider Architecture)           ║
// ║                                                                      ║
// ║  rnr operuje na architekturze wrapperowej — jest dyrygentem         ║
// ║  zewnętrznych narzędzi CLI (Netlify, Supabase, itd.).               ║
// ║                                                                      ║
// ║  Każdy dostawca implementuje interfejs Provider, co umożliwia:      ║
// ║    · Zamianę dostawcy bez zmian w logice wdrożenia                  ║
// ║    · Testowanie z zaślepkami (mock providers)                       ║
// ║    · Jednolite strumieniowanie wyjścia przez masker                 ║
// ╚══════════════════════════════════════════════════════════════════════╝

package providers

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/logger"
)

// ─── Interfejsy ────────────────────────────────────────────────────────────

// DeployProvider definiuje kontrakt dla dostawców wdrożenia aplikacji.
type DeployProvider interface {
	// Name zwraca czytelną nazwę dostawcy.
	Name() string

	// Deploy wykonuje wdrożenie na docelowe środowisko.
	// outputCh otrzymuje linie wyjścia do wyświetlenia w TUI.
	Deploy(ctx context.Context, env config.Environment, outputCh chan<- string) error

	// Rollback wykonuje cofnięcie wdrożenia (jeśli dostawca to wspiera).
	// Dla większości dostawców rollback odbywa się przez git + ponowny deploy.
	Rollback(ctx context.Context, env config.Environment, outputCh chan<- string) error
}

// DatabaseProvider definiuje kontrakt dla dostawców bazy danych.
type DatabaseProvider interface {
	// Name zwraca czytelną nazwę dostawcy.
	Name() string

	// Migrate wykonuje migracje bazy danych.
	Migrate(ctx context.Context, env config.Environment, outputCh chan<- string) error

	// Promote przepycha migracje ze środowiska źródłowego do docelowego.
	// Używane przez komendę `rnr promote` dla Supabase.
	// WAŻNE: Promote używa zasady "roll-forward" — nie cofa danych!
	Promote(ctx context.Context, sourceEnv, targetEnv config.Environment, outputCh chan<- string) error
}

// ─── Fabryki ──────────────────────────────────────────────────────────────

// NewDeployProvider tworzy odpowiedni dostawca wdrożenia na podstawie konfiguracji.
// projectRoot jest potrzebny przez Netlify provider do zapisu site ID po automatycznym
// utworzeniu projektu (netlify_create_new: true).
// Zwraca błąd jeśli dostawca jest nieznany lub brak wymaganej konfiguracji.
func NewDeployProvider(env config.Environment, masker *logger.Masker, log *logger.Logger, projectRoot string) (DeployProvider, error) {
	switch env.Deploy.Provider {
	case config.ProviderNetlify:
		return NewNetlifyProvider(masker, log, projectRoot), nil
	case config.ProviderVercel:
		return NewVercelProvider(masker, log), nil
	case config.ProviderSSH:
		return NewSSHProvider(masker, log), nil
	case config.ProviderGHPages:
		return NewGHPagesProvider(masker, log), nil
	case config.ProviderDocker:
		return NewDockerProvider(masker, log), nil
	case config.ProviderCustom:
		return NewCustomDeployProvider(masker, log), nil
	case "":
		return nil, fmt.Errorf("nie skonfigurowano dostawcy wdrożenia (pole 'provider' w sekcji 'deploy')")
	default:
		return nil, fmt.Errorf("nieznany dostawca wdrożenia: %q (obsługiwane: netlify, vercel, gh-pages, ssh, docker, custom)", env.Deploy.Provider)
	}
}

// NewDatabaseProvider tworzy odpowiedni dostawca bazy danych na podstawie konfiguracji.
func NewDatabaseProvider(env config.Environment, masker *logger.Masker, log *logger.Logger) (DatabaseProvider, error) {
	switch env.Database.Provider {
	case config.DBProviderSupabase:
		return NewSupabaseProvider(masker, log), nil
	case config.DBProviderPrisma:
		return NewPrismaProvider(masker, log), nil
	case config.DBProviderPostgres:
		return NewPostgresProvider(masker, log), nil
	case config.DBProviderMySQL:
		return NewMySQLProvider(masker, log), nil
	case config.DBProviderNone, "":
		return NewNopDatabaseProvider(), nil
	case config.DBProviderCustom:
		return NewCustomDatabaseProvider(masker, log), nil
	default:
		return nil, fmt.Errorf("nieznany dostawca bazy danych: %q (obsługiwane: supabase, prisma, postgres, mysql, none, custom)", env.Database.Provider)
	}
}

// ─── NOP Providers (zaślepki gdy brak konfiguracji) ───────────────────────

// nopDatabaseProvider to dostawca który nic nie robi (dla provider: none).
type nopDatabaseProvider struct{}

// NewNopDatabaseProvider tworzy dostawcę który nic nie robi.
func NewNopDatabaseProvider() DatabaseProvider {
	return &nopDatabaseProvider{}
}

func (n *nopDatabaseProvider) Name() string { return "none" }

func (n *nopDatabaseProvider) Migrate(_ context.Context, _ config.Environment, outputCh chan<- string) error {
	send(outputCh, "Brak dostawcy bazy danych — etap migracji pominięty.")
	return nil
}

func (n *nopDatabaseProvider) Promote(_ context.Context, _, _ config.Environment, outputCh chan<- string) error {
	send(outputCh, "Brak dostawcy bazy danych — etap promote pominięty.")
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────

// checkCLI sprawdza czy wymagane narzędzie CLI jest dostępne w PATH.
// Jeśli brak, zwraca czytelny błąd z komendą instalacji.
// Nie blokuje — wywoływać PRZED pierwszą operacją CLI w każdym providerze.
func checkCLI(name, installHint string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf(
			"narzędzie '%s' nie jest zainstalowane lub nie jest w PATH\n\n"+
				"Instalacja:\n  %s\n\n"+
				"Po instalacji uruchom wdrożenie ponownie.",
			name, installHint,
		)
	}
	return nil
}

// send wysyła linię do kanału bez blokowania.
func send(ch chan<- string, msg string) {
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
	default:
	}
}
