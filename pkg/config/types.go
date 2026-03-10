// file: pkg/config/types.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Typy konfiguracyjne systemu rnr                                    ║
// ║                                                                      ║
// ║  Ten plik definiuje wszystkie struktury danych używane przez rnr.   ║
// ║  Mapuje dokładnie YAML z plików rnr.yaml oraz rnr.conf.yaml.        ║
// ╚══════════════════════════════════════════════════════════════════════╝

package config

// ─── Stałe typów etapów potoku ─────────────────────────────────────────────

// StageTypeShell to domyślny typ etapu — wykonuje komendę powłoki.
const StageTypeShell = ""

// StageTypeDatabase oznacza etap migracji bazy danych.
const StageTypeDatabase = "database"

// StageTypeDeploy oznacza etap wdrożeniowy (Netlify, Vercel, SSH, itd.).
const StageTypeDeploy = "deploy"

// StageTypeHealth oznacza automatyczne sprawdzenie zdrowia aplikacji po deploy.
const StageTypeHealth = "health"

// ─── Stałe dostawców wdrożenia ─────────────────────────────────────────────

const (
	ProviderNetlify  = "netlify"
	ProviderVercel   = "vercel"
	ProviderGHPages  = "gh-pages"
	ProviderSSH      = "ssh"
	ProviderFTP      = "ftp"
	ProviderDocker   = "docker"
	ProviderCustom   = "custom"
)

// ─── Stałe dostawców bazy danych ───────────────────────────────────────────

const (
	DBProviderSupabase = "supabase"
	DBProviderPrisma   = "prisma"
	DBProviderPostgres = "postgres"
	DBProviderMySQL    = "mysql"
	DBProviderNone     = "none"
	DBProviderCustom   = "custom"
)

// ─── Struktury rnr.yaml ────────────────────────────────────────────────────

// Stage reprezentuje pojedynczy krok w potoku wdrożeniowym.
// Pole Name jest unikalne w obrębie potoku i służy do pomijania/filtrowania kroków
// przy użyciu flag --skip i --only.
type Stage struct {
	// Name — unikalny identyfikator etapu (np. "install", "migrate", "deploy").
	Name string `yaml:"name"`

	// Run — komenda powłoki do wykonania (dla etapów typu shell).
	Run string `yaml:"run,omitempty"`

	// Type — typ etapu: database | deploy | health | "" (shell domyślnie).
	Type string `yaml:"type,omitempty"`

	// AllowFailure — jeśli true, błąd etapu jest logowany ale potok kontynuuje.
	AllowFailure bool `yaml:"allow_failure,omitempty"`

	// Only — lista środowisk, dla których ten etap jest wykonywany.
	// Puste = wykonaj we wszystkich środowiskach.
	Only []string `yaml:"only,omitempty"`

	// Artifacts — ścieżka do katalogu artefaktów do zachowania po buildzie.
	Artifacts string `yaml:"artifacts,omitempty"`
}

// PipelineConfig to struktura pliku rnr.yaml — bezpieczna do commitowania.
type PipelineConfig struct {
	Stages []Stage `yaml:"stages"`
}

// ─── Struktury rnr.conf.yaml ───────────────────────────────────────────────

// ProjectInfo zawiera metadane projektu.
type ProjectInfo struct {
	// Name — nazwa projektu wyświetlana w TUI.
	Name string `yaml:"name"`

	// Version — wersja projektu (opcjonalne, np. "1.0.0").
	Version string `yaml:"version,omitempty"`

	// Repo — repozytorium GitHub w formacie "owner/repo".
	Repo string `yaml:"repo,omitempty"`

	// Actor — nadpisuje nazwę autora wdrożenia (puste = git config user.name).
	Actor string `yaml:"actor,omitempty"`

	// ActorEmail — nadpisuje email autora (puste = git config user.email).
	ActorEmail string `yaml:"actor_email,omitempty"`
}

// Notifications zawiera konfigurację powiadomień zewnętrznych.
type Notifications struct {
	// SlackWebhook — URL webhooka Slack dla powiadomień wdrożeniowych.
	SlackWebhook string `yaml:"slack_webhook,omitempty"`
}

// DeployConfig zawiera konfigurację dostawcy wdrożenia dla danego środowiska.
// Wypełnij TYLKO sekcję odpowiadającą wybranemu dostawcy.
type DeployConfig struct {
	// Provider — typ dostawcy: netlify | vercel | gh-pages | ssh | ftp | docker | custom.
	Provider string `yaml:"provider"`

	// ── Netlify ──────────────────────────────────────────────────────────
	NetlifyAuthToken string `yaml:"netlify_auth_token,omitempty"`
	NetlifySiteID    string `yaml:"netlify_site_id,omitempty"`
	// NetlifyProd — jeśli true, wdraża na produkcyjny URL (flaga --prod).
	NetlifyProd bool `yaml:"netlify_prod,omitempty"`
	// NetlifyCreateNew — jeśli true i NetlifySiteID jest pusty, rnr automatycznie
	// tworzy nowy projekt Netlify komendą `netlify sites:create` i zapisuje ID.
	NetlifyCreateNew bool `yaml:"netlify_create_new,omitempty"`

	// ── Vercel ───────────────────────────────────────────────────────────
	VercelToken     string `yaml:"vercel_token,omitempty"`
	VercelOrgID     string `yaml:"vercel_org_id,omitempty"`
	VercelProjectID string `yaml:"vercel_project_id,omitempty"`
	VercelProd      bool   `yaml:"vercel_prod,omitempty"`

	// ── SSH / rsync ──────────────────────────────────────────────────────
	SSHHost   string `yaml:"ssh_host,omitempty"`
	SSHUser   string `yaml:"ssh_user,omitempty"`
	SSHPath   string `yaml:"ssh_path,omitempty"`
	SSHKey    string `yaml:"ssh_key,omitempty"`
	SSHSource string `yaml:"ssh_source,omitempty"`

	// ── FTP ──────────────────────────────────────────────────────────────
	FTPHost     string `yaml:"ftp_host,omitempty"`
	FTPUser     string `yaml:"ftp_user,omitempty"`
	FTPPassword string `yaml:"ftp_password,omitempty"`
	FTPPath     string `yaml:"ftp_path,omitempty"`
	FTPSource   string `yaml:"ftp_source,omitempty"`

	// ── Docker ───────────────────────────────────────────────────────────
	DockerImage         string `yaml:"docker_image,omitempty"`
	DockerTag           string `yaml:"docker_tag,omitempty"`
	DockerRegistryUser  string `yaml:"docker_registry_user,omitempty"`
	DockerRegistryToken string `yaml:"docker_registry_token,omitempty"`
	DockerRunCmd        string `yaml:"docker_run_cmd,omitempty"`

	// ── GitHub Pages ─────────────────────────────────────────────────────
	GHPagesBranch string `yaml:"gh_pages_branch,omitempty"`
	GHPagesSource string `yaml:"gh_pages_source,omitempty"`

	// ── Custom ───────────────────────────────────────────────────────────
	DeployCmd   string `yaml:"deploy_cmd,omitempty"`
	RollbackCmd string `yaml:"rollback_cmd,omitempty"`
}

// DatabaseConfig zawiera konfigurację dostawcy bazy danych dla danego środowiska.
type DatabaseConfig struct {
	// Provider — typ dostawcy: supabase | prisma | postgres | mysql | none | custom.
	Provider string `yaml:"provider"`

	// ── Supabase ─────────────────────────────────────────────────────────
	SupabaseProjectRef     string `yaml:"supabase_project_ref,omitempty"`
	SupabaseDBURL          string `yaml:"supabase_db_url,omitempty"`
	SupabaseAnonKey        string `yaml:"supabase_anon_key,omitempty"`
	SupabaseServiceRoleKey string `yaml:"supabase_service_role_key,omitempty"`

	// ── Prisma / Postgres / MySQL ─────────────────────────────────────────
	// DBURL — connection string do bazy danych.
	DBURL           string `yaml:"db_url,omitempty"`
	DBMigrationsDir string `yaml:"db_migrations_dir,omitempty"`

	// ── Custom ───────────────────────────────────────────────────────────
	DBMigrateCmd  string `yaml:"db_migrate_cmd,omitempty"`
	DBRollbackCmd string `yaml:"db_rollback_cmd,omitempty"`
}

// Environment reprezentuje pojedyncze środowisko wdrożeniowe (production, staging, preview).
type Environment struct {
	// Branch — gałąź Git z której wdrażamy to środowisko.
	Branch string `yaml:"branch"`

	// URL — publiczny adres URL środowiska (używany przy health check).
	URL string `yaml:"url,omitempty"`

	// Protected — jeśli true, wymaga potwierdzenia przed wdrożeniem.
	Protected bool `yaml:"protected,omitempty"`

	// Deploy — konfiguracja dostawcy wdrożenia.
	Deploy DeployConfig `yaml:"deploy"`

	// Database — konfiguracja dostawcy bazy danych.
	Database DatabaseConfig `yaml:"database"`

	// Env — zmienne środowiskowe eksportowane przed każdym etapem potoku.
	Env map[string]string `yaml:"env,omitempty"`
}

// ConfConfig to struktura pliku rnr.conf.yaml — NIE COMMITOWAĆ!
type ConfConfig struct {
	// Project — metadane projektu.
	Project ProjectInfo `yaml:"project"`

	// Notifications — konfiguracja powiadomień zewnętrznych.
	Notifications Notifications `yaml:"notifications,omitempty"`

	// Environments — mapa środowisk (klucz = nazwa, np. "production", "staging").
	Environments map[string]Environment `yaml:"environments"`
}

// Config to połączona konfiguracja z obu plików.
// PipelineConfig pochodzi z rnr.yaml, ConfConfig z rnr.conf.yaml.
type Config struct {
	Pipeline *PipelineConfig
	Conf     *ConfConfig
}

// GetEnvironmentNames zwraca posortowaną listę nazw środowisk.
func (c *Config) GetEnvironmentNames() []string {
	if c.Conf == nil {
		return nil
	}
	names := make([]string, 0, len(c.Conf.Environments))
	// Preferowana kolejność
	order := []string{"production", "staging", "preview"}
	used := map[string]bool{}
	for _, name := range order {
		if _, ok := c.Conf.Environments[name]; ok {
			names = append(names, name)
			used[name] = true
		}
	}
	for name := range c.Conf.Environments {
		if !used[name] {
			names = append(names, name)
		}
	}
	return names
}

// GetStagesForEnv zwraca etapy potoku przefiltrowane dla danego środowiska.
func (c *Config) GetStagesForEnv(envName string) []Stage {
	if c.Pipeline == nil {
		return nil
	}
	var result []Stage
	for _, stage := range c.Pipeline.Stages {
		if len(stage.Only) == 0 {
			result = append(result, stage)
			continue
		}
		for _, allowed := range stage.Only {
			if allowed == envName {
				result = append(result, stage)
				break
			}
		}
	}
	return result
}

// AllSecrets zbiera wszystkie wartości wrażliwe z konfiguracji (do maskowania).
func (c *Config) AllSecrets() []string {
	if c.Conf == nil {
		return nil
	}
	var secrets []string
	for _, env := range c.Conf.Environments {
		d := env.Deploy
		secrets = appendNonEmpty(secrets,
			d.NetlifyAuthToken,
			d.VercelToken,
			d.FTPPassword,
			d.DockerRegistryToken,
			d.SSHKey,
		)
		db := env.Database
		secrets = appendNonEmpty(secrets,
			db.SupabaseDBURL,
			db.SupabaseAnonKey,
			db.SupabaseServiceRoleKey,
			db.DBURL,
		)
	}
	secrets = appendNonEmpty(secrets, c.Conf.Notifications.SlackWebhook)
	return secrets
}

func appendNonEmpty(slice []string, values ...string) []string {
	for _, v := range values {
		if len(v) > 4 {
			slice = append(slice, v)
		}
	}
	return slice
}
