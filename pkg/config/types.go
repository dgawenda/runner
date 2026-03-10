// file: pkg/config/types.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Typy konfiguracyjne systemu rnr                                    ║
// ║                                                                      ║
// ║  Architektura plików konfiguracyjnych:                              ║
// ║                                                                      ║
// ║  rnr.yaml         — BEZPIECZNY do commitowania                     ║
// ║    ├── project    — nazwa, wersja, repo                            ║
// ║    ├── environments — gałęzie, URL, dostawcy (BEZ tokenów)         ║
// ║    └── stages     — definicja kroków potoku wdrożeniowego          ║
// ║                                                                      ║
// ║  rnr.conf.yaml    — NIGDY nie commitować! (gitignored)             ║
// ║    ├── project    — opcjonalne nadpisanie aktora/wersji             ║
// ║    ├── notifications — webhooki (Slack, itp.)                      ║
// ║    └── environments — WYŁĄCZNIE tokeny i hasła per środowisko      ║
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

// StageTypeGit oznacza wewnętrzny etap operacji Git (checkout, pull).
// Generowany automatycznie przez rnr — nie pojawia się w rnr.yaml.
const StageTypeGit = "git"

// ─── Stałe dostawców wdrożenia ─────────────────────────────────────────────

const (
	ProviderNetlify = "netlify"
	ProviderVercel  = "vercel"
	ProviderGHPages = "gh-pages"
	ProviderSSH     = "ssh"
	ProviderFTP     = "ftp"
	ProviderDocker  = "docker"
	ProviderCustom  = "custom"
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

// ══════════════════════════════════════════════════════════════════════════════
// STRUKTURY PLIKU rnr.yaml
// Bezpieczny do commitowania — brak tokenów, haseł i credentials.
// ══════════════════════════════════════════════════════════════════════════════

// Stage reprezentuje pojedynczy krok w potoku wdrożeniowym.
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
	Only []string `yaml:"only,omitempty"`

	// Artifacts — ścieżka do katalogu artefaktów do zachowania po buildzie.
	Artifacts string `yaml:"artifacts,omitempty"`
}

// ProjectInfo zawiera publiczne metadane projektu (rnr.yaml).
type ProjectInfo struct {
	// Name — nazwa projektu wyświetlana w TUI i logach.
	Name string `yaml:"name"`

	// Version — wersja projektu (np. "1.0.0").
	Version string `yaml:"version,omitempty"`

	// Repo — repozytorium GitHub w formacie "owner/repo".
	Repo string `yaml:"repo,omitempty"`
}

// DeploySpec definiuje publiczne ustawienia wdrożenia (BEZ tokenów).
// Przechowywane w rnr.yaml — bezpieczne do commitowania.
type DeploySpec struct {
	// Provider — typ dostawcy: netlify | vercel | gh-pages | ssh | ftp | docker | custom.
	Provider string `yaml:"provider"`

	// ── Netlify (ustawienia bez tokenu) ──────────────────────────────────
	// NetlifyProd — jeśli true, wdraża na produkcyjny URL (flaga --prod).
	NetlifyProd bool `yaml:"netlify_prod,omitempty"`

	// ── Vercel (ustawienia bez tokenu) ───────────────────────────────────
	VercelProd bool `yaml:"vercel_prod,omitempty"`

	// ── SSH / rsync (hosty i ścieżki — nie klucz prywatny) ───────────────
	SSHHost   string `yaml:"ssh_host,omitempty"`
	SSHUser   string `yaml:"ssh_user,omitempty"`
	SSHPath   string `yaml:"ssh_path,omitempty"`
	SSHSource string `yaml:"ssh_source,omitempty"`

	// ── GitHub Pages ─────────────────────────────────────────────────────
	GHPagesBranch string `yaml:"gh_pages_branch,omitempty"`
	GHPagesSource string `yaml:"gh_pages_source,omitempty"`

	// ── Docker (obraz — bez danych dostępu do rejestru) ──────────────────
	DockerImage  string `yaml:"docker_image,omitempty"`
	DockerTag    string `yaml:"docker_tag,omitempty"`
	DockerRunCmd string `yaml:"docker_run_cmd,omitempty"`

	// ── Custom (komendy — bez haseł) ─────────────────────────────────────
	DeployCmd   string `yaml:"deploy_cmd,omitempty"`
	RollbackCmd string `yaml:"rollback_cmd,omitempty"`
}

// DatabaseSpec definiuje publiczne ustawienia bazy danych (BEZ credentials).
// Przechowywane w rnr.yaml — bezpieczne do commitowania.
type DatabaseSpec struct {
	// Provider — typ dostawcy: supabase | prisma | postgres | mysql | none | custom.
	Provider string `yaml:"provider"`

	// DBMigrationsDir — katalog z plikami migracji (np. "supabase/migrations").
	DBMigrationsDir string `yaml:"db_migrations_dir,omitempty"`
}

// EnvSpec definiuje konfigurację środowiska w rnr.yaml (BEZ danych wrażliwych).
// Przykład: gałąź Git, URL, ochrona, wybór dostawcy i zmienne środowiskowe.
type EnvSpec struct {
	// Branch — gałąź Git z której wdrażamy to środowisko.
	Branch string `yaml:"branch"`

	// URL — publiczny adres URL środowiska (używany przy health check).
	URL string `yaml:"url,omitempty"`

	// Protected — jeśli true, wymaga potwierdzenia przed wdrożeniem.
	Protected bool `yaml:"protected,omitempty"`

	// Deploy — publiczne ustawienia wdrożenia (dostawca, flagi — BEZ tokenów).
	Deploy DeploySpec `yaml:"deploy"`

	// Database — publiczne ustawienia bazy danych (dostawca — BEZ credentials).
	Database DatabaseSpec `yaml:"database"`

	// Env — zmienne środowiskowe eksportowane przed każdym etapem potoku.
	// Te zmienne są PUBLICZNE — nie wpisuj tu haseł ani tokenów!
	Env map[string]string `yaml:"env,omitempty"`
}

// PipelineConfig to struktura pliku rnr.yaml — BEZPIECZNA do commitowania.
// Definiuje projekt, środowiska (bez sekretów) i kroki potoku wdrożeniowego.
type PipelineConfig struct {
	// Project — metadane projektu (nazwa, wersja, repo).
	Project ProjectInfo `yaml:"project"`

	// Environments — definicje środowisk (branch, URL, dostawcy — BEZ tokenów).
	Environments map[string]EnvSpec `yaml:"environments"`

	// Stages — kolejność i konfiguracja kroków potoku wdrożeniowego.
	Stages []Stage `yaml:"stages"`
}

// ══════════════════════════════════════════════════════════════════════════════
// STRUKTURY PLIKU rnr.conf.yaml
// WYŁĄCZNIE dane wrażliwe — NIE COMMITOWAĆ!
// ══════════════════════════════════════════════════════════════════════════════

// ConfProjectOverride pozwala nadpisać wybrane ustawienia projektu lokalnie.
// Użyteczne gdy actor/email różni się na danym komputerze.
type ConfProjectOverride struct {
	// Actor — nadpisuje nazwę autora wdrożenia (puste = z git config user.name).
	Actor string `yaml:"actor,omitempty"`

	// ActorEmail — nadpisuje email autora (puste = z git config user.email).
	ActorEmail string `yaml:"actor_email,omitempty"`
}

// Notifications zawiera konfigurację powiadomień (tokeny webhooków).
type Notifications struct {
	// SlackWebhook — URL webhooka Slack dla powiadomień wdrożeniowych.
	SlackWebhook string `yaml:"slack_webhook,omitempty"`
}

// DeploySecrets zawiera WYŁĄCZNIE wrażliwe dane wdrożenia.
// Tokeny, klucze API, hasła — wszystko co nie może trafić do repozytorium.
type DeploySecrets struct {
	// ── Netlify ──────────────────────────────────────────────────────────
	NetlifyAuthToken string `yaml:"netlify_auth_token,omitempty"`
	NetlifySiteID    string `yaml:"netlify_site_id,omitempty"`
	// NetlifyCreateNew — jeśli true i brak Site ID, rnr automatycznie tworzy projekt.
	NetlifyCreateNew bool `yaml:"netlify_create_new,omitempty"`

	// ── Vercel ───────────────────────────────────────────────────────────
	VercelToken     string `yaml:"vercel_token,omitempty"`
	VercelOrgID     string `yaml:"vercel_org_id,omitempty"`
	VercelProjectID string `yaml:"vercel_project_id,omitempty"`

	// ── SSH (klucz prywatny) ─────────────────────────────────────────────
	SSHKey string `yaml:"ssh_key,omitempty"`

	// ── FTP (dane dostępu) ───────────────────────────────────────────────
	FTPHost     string `yaml:"ftp_host,omitempty"`
	FTPUser     string `yaml:"ftp_user,omitempty"`
	FTPPassword string `yaml:"ftp_password,omitempty"`
	FTPPath     string `yaml:"ftp_path,omitempty"`
	FTPSource   string `yaml:"ftp_source,omitempty"`

	// ── Docker (dane dostępu do rejestru) ───────────────────────────────
	DockerRegistryUser  string `yaml:"docker_registry_user,omitempty"`
	DockerRegistryToken string `yaml:"docker_registry_token,omitempty"`
}

// DatabaseSecrets zawiera WYŁĄCZNIE wrażliwe dane bazy danych.
type DatabaseSecrets struct {
	// ── Supabase ─────────────────────────────────────────────────────────
	SupabaseProjectRef     string `yaml:"supabase_project_ref,omitempty"`
	SupabaseDBURL          string `yaml:"supabase_db_url,omitempty"`
	SupabaseAnonKey        string `yaml:"supabase_anon_key,omitempty"`
	SupabaseServiceRoleKey string `yaml:"supabase_service_role_key,omitempty"`

	// ── Prisma / Postgres / MySQL ─────────────────────────────────────────
	DBURL string `yaml:"db_url,omitempty"`

	// ── Custom ───────────────────────────────────────────────────────────
	DBMigrateCmd  string `yaml:"db_migrate_cmd,omitempty"`
	DBRollbackCmd string `yaml:"db_rollback_cmd,omitempty"`
}

// EnvSecrets zawiera WYŁĄCZNIE wrażliwe dane dla jednego środowiska.
// Przechowywane w rnr.conf.yaml — NIE COMMITOWAĆ!
type EnvSecrets struct {
	// Deploy — tokeny i hasła dla dostawcy wdrożenia.
	Deploy DeploySecrets `yaml:"deploy,omitempty"`

	// Database — credentials do bazy danych.
	Database DatabaseSecrets `yaml:"database,omitempty"`
}

// ConfConfig to struktura pliku rnr.conf.yaml — NIE COMMITOWAĆ!
// Zawiera WYŁĄCZNIE dane wrażliwe: tokeny, hasła, klucze API.
type ConfConfig struct {
	// Project — opcjonalne nadpisanie danych autora lokalnie.
	Project ConfProjectOverride `yaml:"project,omitempty"`

	// Notifications — webhooki do powiadomień (Slack, itp.).
	Notifications Notifications `yaml:"notifications,omitempty"`

	// Environments — WYŁĄCZNIE sekrety per środowisko.
	// Klucze muszą odpowiadać nazwom środowisk z rnr.yaml.
	Environments map[string]EnvSecrets `yaml:"environments"`
}

// ══════════════════════════════════════════════════════════════════════════════
// TYPY WEWNĘTRZNE (merged — używane przez providers, TUI, logikę wdrożeń)
// Powstają przez scalenie EnvSpec (z rnr.yaml) + EnvSecrets (z rnr.conf.yaml).
// ══════════════════════════════════════════════════════════════════════════════

// DeployConfig to pełna konfiguracja wdrożenia (publiczne + sekrety).
// Używana wewnętrznie przez providers — NIE serializowana bezpośrednio do YAML.
type DeployConfig struct {
	Provider string

	// Netlify
	NetlifyAuthToken string
	NetlifySiteID    string
	NetlifyProd      bool
	NetlifyCreateNew bool

	// Vercel
	VercelToken     string
	VercelOrgID     string
	VercelProjectID string
	VercelProd      bool

	// SSH
	SSHHost   string
	SSHUser   string
	SSHPath   string
	SSHKey    string
	SSHSource string

	// FTP
	FTPHost     string
	FTPUser     string
	FTPPassword string
	FTPPath     string
	FTPSource   string

	// Docker
	DockerImage         string
	DockerTag           string
	DockerRegistryUser  string
	DockerRegistryToken string
	DockerRunCmd        string

	// GitHub Pages
	GHPagesBranch string
	GHPagesSource string

	// Custom
	DeployCmd   string
	RollbackCmd string
}

// DatabaseConfig to pełna konfiguracja bazy danych (publiczne + sekrety).
// Używana wewnętrznie — NIE serializowana bezpośrednio do YAML.
type DatabaseConfig struct {
	Provider        string
	DBMigrationsDir string

	// Supabase
	SupabaseProjectRef     string
	SupabaseDBURL          string
	SupabaseAnonKey        string
	SupabaseServiceRoleKey string

	// Prisma / Postgres / MySQL
	DBURL string

	// Custom
	DBMigrateCmd  string
	DBRollbackCmd string
}

// Environment to scalone środowisko (spec z rnr.yaml + sekrety z rnr.conf.yaml).
// Używane przez cały system po załadowaniu konfiguracji.
type Environment struct {
	Branch    string
	URL       string
	Protected bool
	Deploy    DeployConfig
	Database  DatabaseConfig
	Env       map[string]string
}

// ══════════════════════════════════════════════════════════════════════════════
// CONFIG — połączona konfiguracja, metody pomocnicze
// ══════════════════════════════════════════════════════════════════════════════

// Config to połączona konfiguracja z obu plików.
type Config struct {
	Pipeline     *PipelineConfig
	Conf         *ConfConfig
	Environments map[string]Environment // Scalone: spec + sekrety
}

// GetEnvironmentNames zwraca posortowaną listę nazw środowisk.
func (c *Config) GetEnvironmentNames() []string {
	if c.Environments == nil {
		return nil
	}
	names := make([]string, 0, len(c.Environments))
	// Preferowana kolejność środowisk w UI
	order := []string{"production", "staging", "preview", "local", "dev", "development"}
	used := map[string]bool{}
	for _, name := range order {
		if _, ok := c.Environments[name]; ok {
			names = append(names, name)
			used[name] = true
		}
	}
	for name := range c.Environments {
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
	if c.Environments == nil {
		return nil
	}
	var secrets []string
	for _, env := range c.Environments {
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
	if c.Conf != nil {
		secrets = appendNonEmpty(secrets, c.Conf.Notifications.SlackWebhook)
	}
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
