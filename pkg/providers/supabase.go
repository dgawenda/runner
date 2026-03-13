// file: pkg/providers/supabase.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Dostawca Supabase CLI — Migracje Bazodanowe                        ║
// ║                                                                      ║
// ║  Obsługuje zaawansowane migracje bazy danych Supabase z zasadą      ║
// ║  "roll-forward" (TYLKO zmiany addytywne — cofanie jest wyłączone).  ║
// ║                                                                      ║
// ║  Kluczowe operacje:                                                  ║
// ║    · Migrate  — zastosuj migracje na docelowym środowisku           ║
// ║    · Promote  — przepuść migracje ze staging → production           ║
// ║                                                                      ║
// ║  BEZPIECZEŃSTWO: Cofanie migracji bazodanowych jest ZABLOKOWANE.    ║
// ║  Utrata danych produkcyjnych jest nieodwracalna.                    ║
// ╚══════════════════════════════════════════════════════════════════════╝

package providers

import (
	"context"
	"fmt"

	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/logger"
)

// supabaseProvider implementuje DatabaseProvider dla Supabase.
type supabaseProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewSupabaseProvider tworzy dostawcę Supabase.
func NewSupabaseProvider(masker *logger.Masker, log *logger.Logger) DatabaseProvider {
	return &supabaseProvider{masker: masker, log: log}
}

func (p *supabaseProvider) Name() string { return "Supabase" }

// Migrate uruchamia migracje bazy danych Supabase na docelowym środowisku.
// Używa: supabase db push --db-url <URL>
//
// Wymagania:
//   - supabase-cli zainstalowany (npm install -g supabase lub scoop/brew)
//   - supabase_project_ref lub supabase_db_url w rnr.conf.yaml
func (p *supabaseProvider) Migrate(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	// Tymczasowo wyłączamy wykonywanie migracji Supabase z poziomu rnr.
	// rnr będzie się łączył z już istniejącą bazą, ale NIE będzie uruchamiał
	// `supabase db push`. Dzięki temu deploy nie zablokuje się na migracjach.
	send(outputCh, "🛑 Supabase: etap migrate jest tymczasowo WYŁĄCZONY w rnr (pominięty).")
	send(outputCh, "ℹ Upewnij się, że migracje zostały zastosowane ręcznie przed deployem.")
	return nil
}

// Promote przepycha nowe migracje ze środowiska stagingowego do produkcyjnego.
// Jest to implementacja `rnr promote` — dedykowana akcja migracji DB.
//
// Algorytm:
//  1. Pobierz listę zastosowanych migracji na staging (źródło)
//  2. Pobierz listę zastosowanych migracji na production (cel)
//  3. Znajdź różnicę (migracje staging - migracje production)
//  4. Zastosuj brakujące migracje na production
//
// ZASADA ROLL-FORWARD: Cofanie migracji jest świadomie wyłączone.
// Jeśli migracja jest wadliwa, należy napisać nową migrację addytywną,
// a nie cofać istniejące (cofanie może zniszczyć dane produkcyjne).
func (p *supabaseProvider) Promote(ctx context.Context, sourceEnv, targetEnv config.Environment, outputCh chan<- string) error {
	send(outputCh, "🛑 Supabase Promote: operacja promote jest tymczasowo WYŁĄCZONA w rnr.")
	send(outputCh, "ℹ Jeśli potrzebujesz promote, wykonaj migracje ręcznie na produkcji.")
	return nil
}

// checkMigrationStatus sprawdza status migracji Supabase i raportuje do outputCh.
func (p *supabaseProvider) checkMigrationStatus(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	db := env.Database

	var args []string
	if db.SupabaseDBURL != "" {
		args = []string{"migration", "list", "--db-url", db.SupabaseDBURL}
	} else {
		args = []string{"migration", "list", "--project-ref", db.SupabaseProjectRef}
	}

	envVars := mergeEnv(env.Env, map[string]string{
		"SUPABASE_ACCESS_TOKEN": db.SupabaseServiceRoleKey,
	})

	runner := NewRunner(".", p.masker, p.log)
	result := runner.RunCommand(ctx, "supabase", args, envVars, outputCh)
	return result.Error
}

// ─── Prisma Provider ──────────────────────────────────────────────────────

type prismaProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewPrismaProvider tworzy dostawcę migracji Prisma ORM.
func NewPrismaProvider(masker *logger.Masker, log *logger.Logger) DatabaseProvider {
	return &prismaProvider{masker: masker, log: log}
}

func (p *prismaProvider) Name() string { return "Prisma" }

func (p *prismaProvider) Migrate(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	if env.Database.DBURL == "" {
		return fmt.Errorf("brak db_url dla Prisma — uzupełnij w rnr.conf.yaml → environments.X.database.db_url")
	}

	if err := checkCLI("npx", "npm install -g npx  (wymaga Node.js)"); err != nil {
		return err
	}

	send(outputCh, "🔷 Prisma: uruchamiam prisma migrate deploy...")

	envVars := mergeEnv(env.Env, map[string]string{
		"DATABASE_URL": env.Database.DBURL,
	})

	runner := NewRunner(".", p.masker, p.log)
	result := runner.RunCommand(ctx, "npx", []string{"prisma", "migrate", "deploy"}, envVars, outputCh)

	if result.Error != nil {
		return fmt.Errorf("Prisma migracja nieudana: %w", result.Error)
	}

	send(outputCh, "✅ Prisma: migracje zakończone sukcesem")
	return nil
}

func (p *prismaProvider) Promote(ctx context.Context, _, targetEnv config.Environment, outputCh chan<- string) error {
	send(outputCh, "📊 Prisma Promote: stosuję migracje na środowisku docelowym...")
	return p.Migrate(ctx, targetEnv, outputCh)
}

// ─── Postgres Raw Provider ────────────────────────────────────────────────

type postgresProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewPostgresProvider tworzy dostawcę migracji dla czystego PostgreSQL.
func NewPostgresProvider(masker *logger.Masker, log *logger.Logger) DatabaseProvider {
	return &postgresProvider{masker: masker, log: log}
}

func (p *postgresProvider) Name() string { return "PostgreSQL" }

func (p *postgresProvider) Migrate(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	db := env.Database

	if db.DBURL == "" {
		return fmt.Errorf("brak db_url dla PostgreSQL — uzupełnij w rnr.conf.yaml")
	}

	migrationsDir := db.DBMigrationsDir
	if migrationsDir == "" {
		migrationsDir = "./migrations"
	}

	send(outputCh, fmt.Sprintf("🐘 PostgreSQL: stosuję migracje z %s...", migrationsDir))

	// Użyj psql do zastosowania migracji
	cmd := fmt.Sprintf(`
		for f in %s/*.sql; do
			echo "Applying: $f"
			psql "%s" -f "$f" || exit 1
		done
	`, migrationsDir, db.DBURL)

	envVars := mergeEnv(env.Env, map[string]string{
		"DATABASE_URL": db.DBURL,
	})

	runner := NewRunner(".", p.masker, p.log)
	result := runner.RunShell(ctx, cmd, envVars, outputCh)

	if result.Error != nil {
		return fmt.Errorf("PostgreSQL migracja nieudana: %w", result.Error)
	}

	send(outputCh, "✅ PostgreSQL: migracje zakończone sukcesem")
	return nil
}

func (p *postgresProvider) Promote(ctx context.Context, _, targetEnv config.Environment, outputCh chan<- string) error {
	return p.Migrate(ctx, targetEnv, outputCh)
}

// ─── MySQL Provider ───────────────────────────────────────────────────────

type mysqlProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewMySQLProvider tworzy dostawcę migracji MySQL.
func NewMySQLProvider(masker *logger.Masker, log *logger.Logger) DatabaseProvider {
	return &mysqlProvider{masker: masker, log: log}
}

func (p *mysqlProvider) Name() string { return "MySQL" }

func (p *mysqlProvider) Migrate(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	db := env.Database

	if db.DBURL == "" {
		return fmt.Errorf("brak db_url dla MySQL — uzupełnij w rnr.conf.yaml")
	}

	migrationsDir := db.DBMigrationsDir
	if migrationsDir == "" {
		migrationsDir = "./migrations"
	}

	send(outputCh, fmt.Sprintf("🐬 MySQL: stosuję migracje z %s...", migrationsDir))

	cmd := fmt.Sprintf(`
		for f in %s/*.sql; do
			echo "Applying: $f"
			mysql "%s" < "$f" || exit 1
		done
	`, migrationsDir, db.DBURL)

	envVars := mergeEnv(env.Env, map[string]string{
		"DATABASE_URL": db.DBURL,
	})

	runner := NewRunner(".", p.masker, p.log)
	result := runner.RunShell(ctx, cmd, envVars, outputCh)

	if result.Error != nil {
		return fmt.Errorf("MySQL migracja nieudana: %w", result.Error)
	}

	send(outputCh, "✅ MySQL: migracje zakończone sukcesem")
	return nil
}

func (p *mysqlProvider) Promote(ctx context.Context, _, targetEnv config.Environment, outputCh chan<- string) error {
	return p.Migrate(ctx, targetEnv, outputCh)
}
