// file: pkg/config/config.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Loader konfiguracji systemu rnr                                    ║
// ║                                                                      ║
// ║  Odpowiada za:                                                       ║
// ║    · Ładowanie rnr.yaml i rnr.conf.yaml                             ║
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
	// PipelineFile — plik potoku (bezpieczny do commitowania).
	PipelineFile = "rnr.yaml"

	// ConfFile — plik sekretów (NIGDY nie commitować!).
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

// Load ładuje oba pliki konfiguracyjne z podanego katalogu projektu.
// Zwraca błąd jeśli którykolwiek z plików nie istnieje lub ma błędy składniowe.
func Load(projectRoot string) (*Config, error) {
	pipeline, err := loadPipeline(filepath.Join(projectRoot, PipelineFile))
	if err != nil {
		return nil, fmt.Errorf("błąd ładowania %s: %w", PipelineFile, err)
	}

	conf, err := loadConf(filepath.Join(projectRoot, ConfFile))
	if err != nil {
		return nil, fmt.Errorf("błąd ładowania %s: %w", ConfFile, err)
	}

	return &Config{
		Pipeline: pipeline,
		Conf:     conf,
	}, nil
}

// loadPipeline ładuje i parsuje plik rnr.yaml.
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
	return &cfg, nil
}

// loadConf ładuje i parsuje plik rnr.conf.yaml.
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
		cfg.Environments = make(map[string]Environment)
	}
	return &cfg, nil
}

// ─── Sprawdzanie istnienia plików ─────────────────────────────────────────

// Exists sprawdza czy oba pliki konfiguracyjne istnieją.
// Zwraca (hasPipeline, hasConf).
func Exists(projectRoot string) (hasPipeline, hasConf bool) {
	_, errP := os.Stat(filepath.Join(projectRoot, PipelineFile))
	_, errC := os.Stat(filepath.Join(projectRoot, ConfFile))
	return errP == nil, errC == nil
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
// Dodaje niezbędne wpisy do .gitignore jeśli jeszcze nie istnieją.
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
		return nil // Wszystko już jest w .gitignore
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

// ─── Generowanie domyślnych plików ────────────────────────────────────────

// DefaultPipelineYAML zwraca zawartość domyślnego pliku rnr.yaml z pełną dokumentacją po polsku.
func DefaultPipelineYAML(projectName string) string {
	return fmt.Sprintf(`# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  rnr.yaml — Definicja Potoku Wdrożeniowego                             ║
# ║  Projekt: %-62s ║
# ║                                                                          ║
# ║  ✅ BEZPIECZNY DO COMMITOWANIA — brak sekretów i tokenów               ║
# ║  🔒 Sekrety i credentials przechowuj w pliku rnr.conf.yaml             ║
# ╚══════════════════════════════════════════════════════════════════════════╝
#
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  TYPY ETAPÓW (type):
#    (brak)     — wykonaj komendę z pola 'run' w powłoce systemu
#    database   — migracja bazy danych (skonfiguruj dostawcę w rnr.conf.yaml)
#    deploy     — wdrożenie aplikacji   (skonfiguruj dostawcę w rnr.conf.yaml)
#    health     — automatyczne sprawdzenie dostępności URL po wdrożeniu
#
#  POLA ETAPU:
#    name          [wymagane]  unikalny identyfikator etapu
#    run           [opcjonalne] komenda powłoki do wykonania
#    type          [opcjonalne] database | deploy | health
#    allow_failure [opcjonalne] true = ostrzeżenie bez przerwania potoku
#    only          [opcjonalne] lista środowisk wykonujących ten etap
#    artifacts     [opcjonalne] ścieżka do katalogu artefaktów po buildzie
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

stages:

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

  - name: test:e2e
    run: npm run test:e2e -- --passWithNoTests
    allow_failure: true
    only: [production, staging]

  # ── Build ─────────────────────────────────────────────────────────────────
  - name: build
    run: npm run build
    artifacts: dist/

  # ── Migracje bazy danych ──────────────────────────────────────────────────
  # Wykonywane tylko na środowiskach produkcyjnych i stagingowych.
  # Dostawca konfigurowany w sekcji 'database' pliku rnr.conf.yaml.
  - name: migrate
    type: database
    only: [production, staging]

  # ── Wdrożenie ─────────────────────────────────────────────────────────────
  # Dostawca konfigurowany w sekcji 'deploy' pliku rnr.conf.yaml.
  - name: deploy
    type: deploy

  # ── Sprawdzenie zdrowia aplikacji ─────────────────────────────────────────
  # Wykonuje żądanie HTTP GET na URL środowiska i sprawdza kod odpowiedzi.
  - name: health
    type: health
    allow_failure: true
`, projectName)
}

// DefaultConfYAML zwraca zawartość domyślnego pliku rnr.conf.yaml z pełną dokumentacją.
func DefaultConfYAML(projectName, repo string) string {
	return fmt.Sprintf(`# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  rnr.conf.yaml — Sejf Dewelopera (Sekrety i Konfiguracja Środowisk)   ║
# ║  Projekt: %-62s ║
# ║                                                                          ║
# ║  ⛔ NIE COMMITOWAĆ — ten plik jest automatycznie dodany do .gitignore  ║
# ║  ✏️  Edytuj ręcznie lub zregeneruj przez:  rnr init --force           ║
# ╚══════════════════════════════════════════════════════════════════════════╝
#
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  WAŻNE ZASADY BEZPIECZEŃSTWA:
#
#  1. Ten plik NIE MOŻE trafić do publicznego repozytorium Git.
#     rnr automatycznie dodaje go do .gitignore przy każdym uruchomieniu.
#
#  2. Tokeny i hasła wpisane tutaj są AUTOMATYCZNIE MASKOWANE we wszystkich
#     logach i wyjściach terminala. Nigdy nie trafią do pliku logu jako
#     jawny tekst — zastąpi je ciąg ***.
#
#  3. Plik przechowuj w menedżerze haseł lub bezpiecznym vaulcie zespołu.
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# ─── Projekt ─────────────────────────────────────────────────────────────────
project:
  name: "%s"
  version: "1.0.0"
  repo: "%s"         # GitHub owner/repo
  actor: ""           # Nadpisz nazwę autora wdrożenia (puste = z git config)
  actor_email: ""     # Nadpisz email autora (puste = z git config)

# ─── Powiadomienia ───────────────────────────────────────────────────────────
notifications:
  slack_webhook: ""   # Slack incoming webhook URL (puste = wyłączone)

# ─── Środowiska ──────────────────────────────────────────────────────────────
# Każde środowisko definiuje:
#   branch      — gałąź Git do wdrożenia
#   url         — publiczny adres URL (używany przy health check)
#   protected   — wymaga potwierdzenia przed wdrożeniem (zalecane dla prod)
#   deploy      — konfiguracja dostawcy wdrożenia + credentials
#   database    — konfiguracja dostawcy bazy danych + credentials
#   env         — zmienne środowiskowe eksportowane przed każdym etapem

environments:

  # ── PRODUKCJA ──────────────────────────────────────────────────────────────
  production:
    branch: "main"
    url: ""
    protected: true   # ⚠️  Wymaga potwierdzenia — zalecane!

    deploy:
      provider: "netlify"   # netlify | vercel | gh-pages | ssh | ftp | docker | custom

      # ── Netlify ────────────────────────────────────────────────────────
      netlify_auth_token: ""
      netlify_site_id: ""
      netlify_prod: true    # true = wdróż na produkcyjny URL (--prod)

      # ── Vercel (odkomentuj jeśli używasz Vercel) ───────────────────────
      # provider: "vercel"
      # vercel_token: ""
      # vercel_org_id: ""
      # vercel_project_id: ""
      # vercel_prod: true

      # ── SSH / rsync (odkomentuj jeśli wdrażasz na serwer VPS) ─────────
      # provider: "ssh"
      # ssh_host: "myserver.com"
      # ssh_user: "deploy"
      # ssh_path: "/var/www/html"
      # ssh_key: "~/.ssh/id_rsa"
      # ssh_source: "dist/"

      # ── Docker (odkomentuj jeśli używasz Docker) ───────────────────────
      # provider: "docker"
      # docker_image: "ghcr.io/owner/myapp"
      # docker_tag: "latest"
      # docker_registry_user: ""
      # docker_registry_token: ""
      # docker_run_cmd: "ssh deploy@server 'docker pull ... && docker-compose up -d'"

      # ── Własna komenda (odkomentuj jeśli masz własny skrypt deploy) ────
      # provider: "custom"
      # deploy_cmd: "./scripts/deploy.sh"
      # rollback_cmd: "./scripts/rollback.sh"

    database:
      provider: "supabase"  # supabase | prisma | postgres | mysql | none | custom

      # ── Supabase (zalecane) ────────────────────────────────────────────
      supabase_project_ref: ""
      supabase_db_url: ""
      supabase_anon_key: ""
      supabase_service_role_key: ""

      # ── Prisma (odkomentuj jeśli używasz Prisma ORM) ───────────────────
      # provider: "prisma"
      # db_url: "postgresql://user:pass@host:5432/db"

      # ── Własna komenda migracji ────────────────────────────────────────
      # provider: "custom"
      # db_migrate_cmd: "npm run db:migrate"
      # db_rollback_cmd: "npm run db:rollback"

    # Zmienne środowiskowe eksportowane przed KAŻDYM etapem potoku
    env:
      NODE_ENV: "production"
      # VITE_APP_URL: "https://myapp.com"
      # VITE_SUPABASE_URL: "https://ref.supabase.co"

  # ── STAGING ────────────────────────────────────────────────────────────────
  staging:
    branch: "develop"
    url: ""
    protected: false

    deploy:
      provider: "netlify"
      netlify_auth_token: ""
      netlify_site_id: ""
      netlify_prod: false

    database:
      provider: "supabase"
      supabase_project_ref: ""
      supabase_db_url: ""
      supabase_anon_key: ""
      supabase_service_role_key: ""

    env:
      NODE_ENV: "staging"

  # ── PREVIEW (gałęzie funkcji) ──────────────────────────────────────────────
  preview:
    branch: ""          # Ustaw przez: rnr deploy preview --branch feat/xyz
    url: ""
    protected: false

    deploy:
      provider: "netlify"
      netlify_auth_token: ""
      netlify_site_id: ""
      netlify_prod: false

    database:
      provider: "none"  # Brak bazy danych dla podglądów (preview)

    env:
      NODE_ENV: "development"
`, projectName, projectName, repo)
}

// SaveConf zapisuje obiekt ConfConfig do pliku rnr.conf.yaml.
func SaveConf(projectRoot string, conf *ConfConfig) error {
	path := filepath.Join(projectRoot, ConfFile)
	data, err := yaml.Marshal(conf)
	if err != nil {
		return fmt.Errorf("nie można serializować konfiguracji: %w", err)
	}
	return os.WriteFile(path, data, 0o600) // 0600 = tylko właściciel może czytać
}
