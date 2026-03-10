// file: pkg/config/config.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Loader konfiguracji systemu rnr                                    ║
// ║                                                                      ║
// ║  Odpowiada za:                                                       ║
// ║    · Ładowanie rnr.yaml (projekt + środowiska + stages)            ║
// ║    · Ładowanie rnr.conf.yaml (TYLKO sekrety per środowisko)        ║
// ║    · Scalanie obu plików w jedną strukturę Config                  ║
// ║    · Generowanie domyślnych plików konfiguracyjnych                 ║
// ║    · Tworzenie struktury katalogów .rnr/                            ║
// ║    · Automatyczne dodawanie wpisów do .gitignore                    ║
// ╚══════════════════════════════════════════════════════════════════════╝

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ─── Stałe ścieżek ─────────────────────────────────────────────────────────

const (
	// PipelineFile — definicja projektu, środowisk i potoku (bezpieczny do commitowania).
	PipelineFile = "rnr.yaml"

	// ConfFile — WYŁĄCZNIE sekrety (NIE COMMITOWAĆ!).
	ConfFile = "rnr.conf.yaml"

	// RnrDir — ukryty katalog główny systemu rnr.
	RnrDir = ".rnr"

	// LogsDir — katalog z logami wdrożeń.
	LogsDir = ".rnr/logs"

	// SnapshotsDir — katalog z informacjami o snapshotach do rollbacku.
	SnapshotsDir = ".rnr/snapshots"

	// StateFile — plik stanu z historią wdrożeń.
	StateFile = ".rnr/snapshots/state.json"
)

// ─── Ładowanie konfiguracji ────────────────────────────────────────────────

// Load ładuje oba pliki konfiguracyjne i scala je w jedną strukturę Config.
// rnr.yaml  → struktury publiczne (projekt, środowiska, stages)
// rnr.conf.yaml → sekrety per środowisko
func Load(projectRoot string) (*Config, error) {
	pipeline, err := loadPipeline(filepath.Join(projectRoot, PipelineFile))
	if err != nil {
		return nil, fmt.Errorf("błąd ładowania %s: %w", PipelineFile, err)
	}

	conf, err := loadConf(filepath.Join(projectRoot, ConfFile))
	if err != nil {
		return nil, fmt.Errorf("błąd ładowania %s: %w", ConfFile, err)
	}

	merged := mergeEnvironments(pipeline, conf)

	return &Config{
		Pipeline:     pipeline,
		Conf:         conf,
		Environments: merged,
	}, nil
}

// loadPipeline ładuje i parsuje plik rnr.yaml.
// Wymaga sekcji stages. Środowiska i projekt są opcjonalne (mogą być puste).
func loadPipeline(path string) (*PipelineConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg PipelineConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("błąd parsowania YAML: %w", err)
	}
	if len(cfg.Stages) == 0 {
		return nil, fmt.Errorf("brak zdefiniowanych etapów (stages) w %s", path)
	}
	if cfg.Environments == nil {
		cfg.Environments = make(map[string]EnvSpec)
	}
	return &cfg, nil
}

// loadConf ładuje i parsuje plik rnr.conf.yaml.
// Zawiera WYŁĄCZNIE sekrety — nie musi mieć wszystkich pól.
func loadConf(path string) (*ConfConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ConfConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("błąd parsowania YAML: %w", err)
	}
	if cfg.Environments == nil {
		cfg.Environments = make(map[string]EnvSecrets)
	}
	return &cfg, nil
}

// mergeEnvironments scala EnvSpec (rnr.yaml) z EnvSecrets (rnr.conf.yaml)
// w gotowe do użycia struktury Environment.
func mergeEnvironments(pipeline *PipelineConfig, conf *ConfConfig) map[string]Environment {
	result := make(map[string]Environment)
	for name, spec := range pipeline.Environments {
		env := Environment{
			ProjectName: pipeline.Project.Name,
			Branch:      spec.Branch,
			URL:         spec.URL,
			Protected:   spec.Protected,
			Env:         spec.Env,
			Deploy: DeployConfig{
				Provider:      spec.Deploy.Provider,
				NetlifyProd:   spec.Deploy.NetlifyProd,
				VercelProd:    spec.Deploy.VercelProd,
				SSHHost:       spec.Deploy.SSHHost,
				SSHUser:       spec.Deploy.SSHUser,
				SSHPath:       spec.Deploy.SSHPath,
				SSHSource:     spec.Deploy.SSHSource,
				GHPagesBranch: spec.Deploy.GHPagesBranch,
				GHPagesSource: spec.Deploy.GHPagesSource,
				DockerImage:   spec.Deploy.DockerImage,
				DockerTag:     spec.Deploy.DockerTag,
				DockerRunCmd:  spec.Deploy.DockerRunCmd,
				DeployCmd:     spec.Deploy.DeployCmd,
				RollbackCmd:   spec.Deploy.RollbackCmd,
			},
			Database: DatabaseConfig{
				Provider:        spec.Database.Provider,
				DBMigrationsDir: spec.Database.DBMigrationsDir,
			},
		}

		// Nakładamy sekrety z rnr.conf.yaml (jeśli istnieją dla tego środowiska)
		if secrets, ok := conf.Environments[name]; ok {
			d := &env.Deploy
			d.NetlifyAuthToken    = secrets.Deploy.NetlifyAuthToken
			d.NetlifySiteID       = secrets.Deploy.NetlifySiteID
			d.NetlifyCreateNew    = secrets.Deploy.NetlifyCreateNew
			d.VercelToken         = secrets.Deploy.VercelToken
			d.VercelOrgID         = secrets.Deploy.VercelOrgID
			d.VercelProjectID     = secrets.Deploy.VercelProjectID
			d.SSHKey              = secrets.Deploy.SSHKey
			d.FTPHost             = secrets.Deploy.FTPHost
			d.FTPUser             = secrets.Deploy.FTPUser
			d.FTPPassword         = secrets.Deploy.FTPPassword
			d.FTPPath             = secrets.Deploy.FTPPath
			d.FTPSource           = secrets.Deploy.FTPSource
			d.DockerRegistryUser  = secrets.Deploy.DockerRegistryUser
			d.DockerRegistryToken = secrets.Deploy.DockerRegistryToken

			db := &env.Database
			db.SupabaseProjectRef     = secrets.Database.SupabaseProjectRef
			db.SupabaseDBURL          = secrets.Database.SupabaseDBURL
			db.SupabaseAnonKey        = secrets.Database.SupabaseAnonKey
			db.SupabaseServiceRoleKey = secrets.Database.SupabaseServiceRoleKey
			db.DBURL                  = secrets.Database.DBURL
			db.DBMigrateCmd           = secrets.Database.DBMigrateCmd
			db.DBRollbackCmd          = secrets.Database.DBRollbackCmd
		}

		result[name] = env
	}
	return result
}

// ─── Sprawdzanie istnienia plików ─────────────────────────────────────────

// Exists sprawdza czy oba pliki konfiguracyjne istnieją.
// Zwraca (hasPipeline, hasConf).
func Exists(projectRoot string) (hasPipeline, hasConf bool) {
	_, errP := os.Stat(filepath.Join(projectRoot, PipelineFile))
	_, errC := os.Stat(filepath.Join(projectRoot, ConfFile))
	return errP == nil, errC == nil
}

// LoadConfOnly ładuje TYLKO plik rnr.conf.yaml bez wymagania rnr.yaml.
func LoadConfOnly(projectRoot string) (*ConfConfig, error) {
	return loadConf(filepath.Join(projectRoot, ConfFile))
}

// LoadPipelineOnly ładuje TYLKO plik rnr.yaml bez wymagania rnr.conf.yaml.
func LoadPipelineOnly(projectRoot string) (*PipelineConfig, error) {
	return loadPipeline(filepath.Join(projectRoot, PipelineFile))
}

// ─── Tworzenie struktury katalogów ────────────────────────────────────────

// EnsureRnrDir tworzy pełną strukturę katalogów .rnr/ jeśli nie istnieje.
func EnsureRnrDir(projectRoot string) error {
	dirs := []string{
		filepath.Join(projectRoot, RnrDir),
		filepath.Join(projectRoot, LogsDir),
		filepath.Join(projectRoot, SnapshotsDir),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("nie można utworzyć katalogu %s: %w", dir, err)
		}
	}
	return nil
}

// ─── Ochrona .gitignore ────────────────────────────────────────────────────

// EnsureGitignore bezwzględnie blokuje commitowanie pliku rnr.conf.yaml.
func EnsureGitignore(projectRoot string) error {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")

	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("nie można odczytać .gitignore: %w", err)
	}

	entries := []string{
		"# rnr — plik sekretów (NIE COMMITOWAĆ!)",
		ConfFile,
		".rnr/logs/",
		".rnr/snapshots/",
	}

	existing := string(content)
	var toAdd []string
	for _, entry := range entries {
		if !strings.Contains(existing, entry) {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("nie można otworzyć .gitignore: %w", err)
	}
	defer f.Close()

	prefix := "\n"
	if len(existing) > 0 && existing[len(existing)-1] == '\n' {
		prefix = ""
	}

	_, err = f.WriteString(prefix + "\n" + strings.Join(toAdd, "\n") + "\n")
	return err
}

// ─── Generowanie pliku rnr.yaml ────────────────────────────────────────────

// DefaultPipelineYAML generuje rnr.yaml z pełną dokumentacją.
// Zachowany dla kompatybilności — deleguje do DefaultPipelineYAMLFromWizard.
func DefaultPipelineYAML(projectName string) string {
	return DefaultPipelineYAMLFromWizard(projectName, "", "custom", "netlify", "none")
}

// DefaultPipelineYAMLForProject generuje rnr.yaml dopasowany do typu projektu.
// Zachowany dla kompatybilności — deleguje do DefaultPipelineYAMLFromWizard.
//
//   - hasDB=true  → dodaje etap 'migrate' (fullstack, Supabase, Prisma, itp.)
//   - hasDB=false → pipeline bez migracji (frontendowy, JAMstack, SPA)
func DefaultPipelineYAMLForProject(projectName string, hasDB bool) string {
	dbProv := DBProviderNone
	if hasDB {
		dbProv = DBProviderSupabase
	}
	return DefaultPipelineYAMLFromWizard(projectName, "", "npm", "netlify", dbProv)
}

// ─── Generowanie pliku rnr.conf.yaml ──────────────────────────────────────

// DefaultConfYAML generuje rnr.conf.yaml z pełną dokumentacją.
func DefaultConfYAML(projectName, repo string) string {
	return fmt.Sprintf(`# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  rnr.conf.yaml — Sejf Sekretów (WYŁĄCZNIE dane wrażliwe)              ║
# ║  Projekt: %-62s ║
# ║                                                                          ║
# ║  ⛔ NIE COMMITOWAĆ — plik jest automatycznie dodany do .gitignore      ║
# ║  ✏️  Każdy developer przechowuje własny plik lokalnie                  ║
# ╚══════════════════════════════════════════════════════════════════════════╝
#
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  CO TUTAJ TRAFIA (TYLKO dane wrażliwe):
#    · Tokeny API (Netlify, Vercel, GitHub)
#    · Klucze Supabase (db_url, anon_key, service_role_key)
#    · Hasła SSH, FTP, Docker Registry
#    · Webhooki Slack
#
#  CZEGO TU NIE MA (jest w rnr.yaml, który możesz commitować):
#    · Gałęzie Git (branch)
#    · Adresy URL środowisk
#    · Wybór dostawców (netlify/supabase/itp.)
#    · Definicja kroków potoku (stages)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# ─── Opcjonalne nadpisanie danych autora ──────────────────────────────────────
# Puste = używa danych z globalnego git config (user.name, user.email)
project:
  actor: ""           # Twoje imię i nazwisko (opcjonalne nadpisanie)
  actor_email: ""     # Twój email (opcjonalne nadpisanie)

# ─── Powiadomienia ───────────────────────────────────────────────────────────
notifications:
  slack_webhook: ""   # Slack Incoming Webhook URL (puste = wyłączone)

# ─── Sekrety per środowisko ───────────────────────────────────────────────────
# Klucze MUSZĄ odpowiadać nazwom środowisk z rnr.yaml → environments.
# Wpisuj TYLKO tokeny i hasła — resztę konfiguruj w rnr.yaml.

environments:

  # ── PRODUKCJA ──────────────────────────────────────────────────────────────
  production:
    deploy:
      # ── Netlify — wpisz swój token i Site ID ──────────────────────────
      netlify_auth_token: ""       # Netlify → User Settings → Personal access tokens
      netlify_site_id: ""          # Netlify → Site settings → General → Site ID

      # ── Vercel (odkomentuj jeśli używasz Vercel) ───────────────────────
      # vercel_token: ""
      # vercel_org_id: ""
      # vercel_project_id: ""

      # ── SSH — klucz prywatny (odkomentuj jeśli SSH) ────────────────────
      # ssh_key: "~/.ssh/id_rsa"

      # ── Docker Registry (odkomentuj jeśli Docker) ──────────────────────
      # docker_registry_user: ""
      # docker_registry_token: ""

    database:
      # ── Supabase — uzupełnij dane projektu produkcyjnego ──────────────
      supabase_project_ref: ""     # Supabase → Project Settings → General → Reference ID
      supabase_db_url: ""          # Supabase → Project Settings → Database → Connection string
      supabase_anon_key: ""        # Supabase → Project Settings → API → anon public
      supabase_service_role_key: "" # Supabase → Project Settings → API → service_role

      # ── Prisma / PostgreSQL bezpośredni (odkomentuj jeśli Prisma) ─────
      # db_url: "postgresql://user:password@host:5432/db"

      # ── Custom migracje (odkomentuj jeśli własny skrypt) ───────────────
      # db_migrate_cmd: "npm run db:migrate"
      # db_rollback_cmd: "npm run db:rollback"

  # ── STAGING ────────────────────────────────────────────────────────────────
  staging:
    deploy:
      netlify_auth_token: ""       # Może być ten sam co production
      netlify_site_id: ""          # INNY Site ID niż production!

    database:
      supabase_project_ref: ""     # INNY projekt Supabase niż production!
      supabase_db_url: ""
      supabase_anon_key: ""
      supabase_service_role_key: ""
`, projectName)
}

// DefaultPipelineYAMLFromWizard generuje rnr.yaml z danymi zebranymi przez Setup Wizard.
// Wypełnia sekcje project i environments na podstawie wyboru użytkownika.
// Sekrety (tokeny, klucze) NIE trafiają tutaj — są zapisywane do rnr.conf.yaml.
//
// projectType: "html" | "npm" | "python" | "custom"
func DefaultPipelineYAMLFromWizard(
	projectName, repo string,
	projectType, deployProv, dbProv string,
) string {
	hasDB := dbProv != "" && dbProv != DBProviderNone

	deployBlock := fmt.Sprintf(`provider: "%s"`, deployProv)
	if deployProv == ProviderNetlify {
		deployBlock = fmt.Sprintf("provider: \"%s\"\n      netlify_prod: true", deployProv)
	} else if deployProv == ProviderVercel {
		deployBlock = fmt.Sprintf("provider: \"%s\"\n      vercel_prod: true", deployProv)
	}

	stagingDeployBlock := fmt.Sprintf(`provider: "%s"`, deployProv)
	if deployProv == ProviderNetlify {
		stagingDeployBlock = fmt.Sprintf("provider: \"%s\"\n      netlify_prod: false", deployProv)
	} else if deployProv == ProviderVercel {
		stagingDeployBlock = fmt.Sprintf("provider: \"%s\"", deployProv)
	}

	dbBlock := fmt.Sprintf(`provider: "%s"`, dbProv)
	if dbProv == "" || dbProv == DBProviderNone {
		dbBlock = `provider: "none"`
	}

	migrateStage := ""
	if hasDB {
		migrateStage = `
  # ── Migracje bazy danych ──────────────────────────────────────────────────
  - name: migrate
    type: database
    only: [production, staging]
`
	}

	// Generuj etapy potoku odpowiednie dla typu projektu
	var stagesBlock string
	switch projectType {
	case "html":
		// Projekt statyczny — bez buildu, bez npm
		stagesBlock = `
  # ── Wdrożenie ─────────────────────────────────────────────────────────────
  # Dla statycznych projektów HTML nie ma etapu budowania.
  # Netlify/Vercel automatycznie wykrywa i serwuje pliki statyczne.
  - name: deploy
    type: deploy

  # ── Sprawdzenie zdrowia aplikacji ─────────────────────────────────────────
  - name: health
    type: health
    allow_failure: true
`
	case "python":
		stagesBlock = fmt.Sprintf(`
  # ── Instalacja zależności Python ──────────────────────────────────────────
  - name: install
    run: pip install -r requirements.txt
    allow_failure: true

  # ── Testy ─────────────────────────────────────────────────────────────────
  - name: test
    run: python -m pytest --tb=short
    allow_failure: true
%s
  # ── Wdrożenie ─────────────────────────────────────────────────────────────
  - name: deploy
    type: deploy

  # ── Sprawdzenie zdrowia aplikacji ─────────────────────────────────────────
  - name: health
    type: health
    allow_failure: true
`, migrateStage)
	case "custom":
		stagesBlock = fmt.Sprintf(`
  # ── WPISZ WŁASNE ETAPY ────────────────────────────────────────────────────
  # Odkomentuj i dostosuj etapy do swojego projektu.

  # - name: install
  #   run: make install

  # - name: build
  #   run: make build
%s
  # ── Wdrożenie ─────────────────────────────────────────────────────────────
  - name: deploy
    type: deploy

  # ── Sprawdzenie zdrowia aplikacji ─────────────────────────────────────────
  - name: health
    type: health
    allow_failure: true
`, migrateStage)
	default: // "npm" i domyślny
		stagesBlock = fmt.Sprintf(`
  # ── Instalacja zależności ──────────────────────────────────────────────────
  - name: install
    run: npm ci

  # ── Jakość kodu ───────────────────────────────────────────────────────────
  - name: lint
    run: npm run lint
    allow_failure: true

  - name: typecheck
    run: npx tsc --noEmit
    allow_failure: true

  # ── Testy ─────────────────────────────────────────────────────────────────
  - name: test:unit
    run: npm run test:unit -- --run --passWithNoTests
    allow_failure: true
%s
  # ── Build ─────────────────────────────────────────────────────────────────
  - name: build
    run: npm run build
    artifacts: dist/

  # ── Wdrożenie ─────────────────────────────────────────────────────────────
  - name: deploy
    type: deploy

  # ── Sprawdzenie zdrowia aplikacji ─────────────────────────────────────────
  - name: health
    type: health
    allow_failure: true
`, migrateStage)
	}

	return fmt.Sprintf(`# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  rnr.yaml — Konfiguracja Projektu i Potoku Wdrożeniowego               ║
# ║  Projekt: %-62s ║
# ║  ✅ BEZPIECZNY DO COMMITOWANIA — brak tokenów i haseł                  ║
# ╚══════════════════════════════════════════════════════════════════════════╝

project:
  name: "%s"
  version: "1.0.0"
  repo: "%s"

environments:

  production:
    branch: "master"
    url: ""
    protected: true

    deploy:
      %s

    database:
      %s

  staging:
    branch: "develop"
    url: ""
    protected: false

    deploy:
      %s

    database:
      %s

stages:
%s`,
		projectName, projectName, repo,
		deployBlock, dbBlock,
		stagingDeployBlock, dbBlock,
		stagesBlock,
	)
}

// DefaultConfYAMLFromWizard generuje rnr.conf.yaml wypełniony danymi z Setup Wizarda.
func DefaultConfYAMLFromWizard(
	projectName, repo string,
	deployProv, netlifyToken, netlifySiteID string,
	netlifyCreateNew bool,
	dbProv, supabaseRef, supabaseURL, supabaseKey string,
) string {
	netlifyCreateNewStr := "false"
	if netlifyCreateNew {
		netlifyCreateNewStr = "true"
	}

	deployBlock := ""
	if deployProv == ProviderNetlify {
		deployBlock = fmt.Sprintf(
			`      netlify_auth_token: "%s"
      netlify_site_id: "%s"
      netlify_create_new: %s`,
			netlifyToken, netlifySiteID, netlifyCreateNewStr)
	} else if deployProv == ProviderVercel {
		deployBlock = fmt.Sprintf(
			`      vercel_token: "%s"`, netlifyToken)
	}

	dbBlock := ""
	if dbProv == DBProviderSupabase {
		dbBlock = fmt.Sprintf(
			`      supabase_project_ref: "%s"
      supabase_db_url: "%s"
      supabase_anon_key: "%s"`,
			supabaseRef, supabaseURL, supabaseKey)
	}

	return fmt.Sprintf(`# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  rnr.conf.yaml — Sejf Sekretów (wygenerowany przez Setup Wizard)      ║
# ║  Projekt: %-62s ║
# ║  ⛔ NIE COMMITOWAĆ — plik jest automatycznie dodany do .gitignore     ║
# ╚══════════════════════════════════════════════════════════════════════════╝

# ─── Opcjonalne nadpisanie autora ─────────────────────────────────────────────
project:
  actor: ""
  actor_email: ""

# ─── Powiadomienia ───────────────────────────────────────────────────────────
notifications:
  slack_webhook: ""

# ─── Sekrety per środowisko ───────────────────────────────────────────────────
environments:

  production:
    deploy:
%s

    database:
%s
`, projectName,
		prefixLines(deployBlock, "    "),
		prefixLines(dbBlock, "    "))
}

// ─── Zarządzanie środowiskami ──────────────────────────────────────────────

// AddEnvironment dodaje nowe środowisko do rnr.yaml (specyfikacja) oraz
// pusty blok sekretów do rnr.conf.yaml (do ręcznego uzupełnienia).
func AddEnvironment(projectRoot, envName, fromEnv string) error {
	pipelinePath := filepath.Join(projectRoot, PipelineFile)
	confPath := filepath.Join(projectRoot, ConfFile)

	// Sprawdź czy środowisko już istnieje
	pipeline, err := loadPipeline(pipelinePath)
	if err != nil {
		return fmt.Errorf("nie można załadować %s: %w", PipelineFile, err)
	}
	if _, exists := pipeline.Environments[envName]; exists {
		return fmt.Errorf("środowisko '%s' już istnieje w %s", envName, PipelineFile)
	}

	// Ustal gałąź i właściwości
	branch := envName
	protected := false
	nodeEnv := envName
	switch envName {
	case "local":
		branch = "master"
		nodeEnv = "development"
	case "dev", "development":
		branch = "develop"
		nodeEnv = "development"
	case "staging":
		branch = "develop"
		nodeEnv = "staging"
	case "production":
		branch = "master"
		protected = true
		nodeEnv = "production"
	}

	// Pobierz ustawienia z szablonu (nie sekrety — tylko dostawców)
	deployProvider := "netlify"
	netlifyProd := false
	dbProvider := "none"

	if tmpl, ok := pipeline.Environments[fromEnv]; ok {
		deployProvider = tmpl.Deploy.Provider
		dbProvider = tmpl.Database.Provider
		if envName == "production" {
			netlifyProd = tmpl.Deploy.NetlifyProd
		}
	}

	protectedStr := "false"
	if protected {
		protectedStr = "true"
	}
	netlifyProdStr := "false"
	if netlifyProd {
		netlifyProdStr = "true"
	}

	// Blok deploy (publiczny, bez tokenów)
	deploySpecBlock := fmt.Sprintf(`provider: "%s"`, deployProvider)
	if deployProvider == ProviderNetlify {
		deploySpecBlock = fmt.Sprintf("provider: \"%s\"\n      netlify_prod: %s", deployProvider, netlifyProdStr)
	}

	// Blok database (publiczny)
	dbSpecBlock := fmt.Sprintf(`provider: "%s"`, dbProvider)

	// Dopisz do rnr.yaml
	envSpecSection := fmt.Sprintf(`
  # ── %s ────────────────────────────────────────────────────────────────────
  %s:
    branch: "%s"
    url: ""
    protected: %s

    deploy:
      %s

    database:
      %s

    env:
      NODE_ENV: "%s"
`, strings.ToUpper(envName), envName, branch, protectedStr,
		deploySpecBlock, dbSpecBlock, nodeEnv)

	pf, err := os.OpenFile(pipelinePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("nie można otworzyć %s: %w", PipelineFile, err)
	}
	defer pf.Close()
	if _, err := pf.WriteString(envSpecSection); err != nil {
		return fmt.Errorf("błąd zapisu do %s: %w", PipelineFile, err)
	}

	// Dopisz pusty blok sekretów do rnr.conf.yaml
	_, confErr := os.Stat(confPath)
	if confErr == nil {
		// Plik istnieje — sprawdź czy środowisko już tam jest
		conf, err := loadConf(confPath)
		if err == nil {
			if _, exists := conf.Environments[envName]; !exists {
				envSecretSection := fmt.Sprintf(`
  # ── %s — uzupełnij tokeny i hasła ────────────────────────────────────────
  %s:
    deploy:
      netlify_auth_token: ""   # ← wpisz token Netlify dla %s
      netlify_site_id: ""      # ← wpisz Site ID dla %s

    database:
      supabase_project_ref: "" # ← wpisz ref projektu Supabase dla %s
      supabase_db_url: ""
      supabase_anon_key: ""
      supabase_service_role_key: ""
`, strings.ToUpper(envName), envName, envName, envName, envName)

				cf, err := os.OpenFile(confPath, os.O_APPEND|os.O_WRONLY, 0o600)
				if err == nil {
					_, _ = cf.WriteString(envSecretSection)
					cf.Close()
				}
			}
		}
	}

	return nil
}

// ListEnvironments zwraca listę nazw środowisk z rnr.yaml.
func ListEnvironments(projectRoot string) ([]string, error) {
	pipeline, err := loadPipeline(filepath.Join(projectRoot, PipelineFile))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pipeline.Environments))
	for name := range pipeline.Environments {
		names = append(names, name)
	}
	return names, nil
}

// SaveConf zapisuje obiekt ConfConfig do pliku rnr.conf.yaml.
func SaveConf(projectRoot string, conf *ConfConfig) error {
	path := filepath.Join(projectRoot, ConfFile)
	data, err := yaml.Marshal(conf)
	if err != nil {
		return fmt.Errorf("nie można serializować konfiguracji: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// prefixLines dodaje prefix do każdej niepustej linii tekstu wieloliniowego.
func prefixLines(text, prefix string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}
