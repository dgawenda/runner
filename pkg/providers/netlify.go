// file: pkg/providers/netlify.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Dostawca Netlify CLI                                               ║
// ║                                                                      ║
// ║  Wrapper dla oficjalnego Netlify CLI (netlify-cli).                 ║
// ║  Przed pierwszym użyciem sprawdza czy netlify-cli jest w PATH —    ║
// ║  jeśli brak, wyświetla czytelną instrukcję instalacji.             ║
// ║                                                                      ║
// ║  Tworzenie nowych projektów Netlify ODBYWA SIĘ PRZEZ REST API      ║
// ║  (bez CLI) — wystarczy token autoryzacji.                           ║
// ║                                                                      ║
// ║  Token jest ZAWSZE maskowany w logach i wyjściu TUI.                ║
// ╚══════════════════════════════════════════════════════════════════════╝

package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/logger"
	"gopkg.in/yaml.v3"
)

// timeNowUnix zwraca aktualny czas Unix — helper dla netlifySlug retry.
func timeNowUnix() int64 { return time.Now().UnixMilli() }

const netlifyInstallHint = "npm install -g netlify-cli\n  lub: yarn global add netlify-cli"

// netlifyProvider implementuje DeployProvider dla Netlify.
type netlifyProvider struct {
	masker      *logger.Masker
	log         *logger.Logger
	projectRoot string // Ścieżka do projektu — do zapisu site ID po auto-tworzeniu
}

// NewNetlifyProvider tworzy dostawcę Netlify.
// projectRoot jest potrzebny do zapisu nowo utworzonego Site ID do rnr.conf.yaml.
func NewNetlifyProvider(masker *logger.Masker, log *logger.Logger, projectRoot string) DeployProvider {
	return &netlifyProvider{masker: masker, log: log, projectRoot: projectRoot}
}

func (p *netlifyProvider) Name() string { return "Netlify" }

// Deploy wdraża aplikację używając Netlify CLI.
// Komenda: netlify deploy [--prod] --site <SITE_ID> --dir <artifacts>
//
// Wymagania:
//   - netlify-cli zainstalowany globalnie (npm install -g netlify-cli)
//   - netlify_auth_token w rnr.conf.yaml
//   - netlify_site_id w rnr.conf.yaml (lub netlify_create_new: true)
func (p *netlifyProvider) Deploy(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	d := env.Deploy

	if d.NetlifyAuthToken == "" {
		return fmt.Errorf("brak netlify_auth_token — uzupełnij w rnr.conf.yaml → environments.X.deploy.netlify_auth_token")
	}

	// ── Tryb: utwórz nowy projekt przez Netlify REST API ────────────────
	if d.NetlifyCreateNew && d.NetlifySiteID == "" {
		siteID, err := p.createSiteViaAPI(d.NetlifyAuthToken, env.ProjectName, outputCh)
		if err != nil {
			return err
		}
		d.NetlifySiteID = siteID

		// Zapisz Site ID do rnr.conf.yaml — trwałe zapamiętanie na kolejne uruchomienia
		if p.projectRoot != "" {
			if saveErr := p.saveNetlifySiteID(env.ProjectName, siteID, outputCh); saveErr != nil {
				// Niepowodzenie zapisu jest niekrytyczne — logujemy, ale nie blokujemy deployu
				send(outputCh, fmt.Sprintf("⚠️  Nie udało się zapisać Site ID do rnr.conf.yaml: %v", saveErr))
				send(outputCh, fmt.Sprintf("📌 ZAPAMIĘTAJ RĘCZNIE → Site ID: %s", siteID))
			} else {
				send(outputCh, fmt.Sprintf("💾 Site ID '%s' zapisany automatycznie do rnr.conf.yaml", siteID))
			}
		} else {
			send(outputCh, fmt.Sprintf("📌 ZAPAMIĘTAJ → Site ID: %s — wpisz go w rnr.conf.yaml → environments.X.deploy.netlify_site_id", siteID))
		}
	}

	if d.NetlifySiteID == "" {
		return fmt.Errorf(
			"brak netlify_site_id\n\n" +
				"Opcje:\n" +
				"  1. Wpisz Site ID w rnr.conf.yaml → environments.X.deploy.netlify_site_id\n" +
				"     (znajdziesz go w Netlify → Site settings → General → Site ID)\n" +
				"  2. Ustaw netlify_create_new: true aby rnr automatycznie założył projekt",
		)
	}

	// ── Sprawdź czy netlify-cli jest zainstalowany ───────────────────────
	if err := checkCLI("netlify", netlifyInstallHint); err != nil {
		return err
	}

	// ── Wczytaj zmienne z pliku .env ─────────────────────────────────────
	// Zmienne z .env są potrzebne podczas builda i wdrożenia.
	// Sekrety są automatycznie maskowane w logach.
	dotEnvVars := readDotEnv(".")
	if len(dotEnvVars) > 0 {
		send(outputCh, fmt.Sprintf("📄 Netlify: załadowano %d zmiennych z .env", len(dotEnvVars)))
	}

	// Buduj argumenty komendy
	args := []string{"deploy", "--site", d.NetlifySiteID, "--dir", "."}
	if d.NetlifyProd {
		args = append(args, "--prod")
		send(outputCh, "🚀 Netlify: wdrożenie na PRODUKCJĘ (--prod)")
	} else {
		send(outputCh, "🔍 Netlify: wdrożenie podglądu (preview deploy)")
	}

	// Przekaż zmienne środowiskowe do Netlify CLI jako flagi --env KEY=VALUE.
	// Kolejność priorytetu: env.Env (rnr.yaml) > .env plik > token Netlify
	for k, v := range dotEnvVars {
		// Nie nadpisuj zmiennych zdefiniowanych bezpośrednio w rnr.yaml
		if _, ok := env.Env[k]; !ok {
			args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Zmienne środowiskowe dla procesu CLI (systemowe — dla autoryzacji)
	envVars := mergeEnv(dotEnvVars, env.Env)
	envVars = mergeEnv(envVars, map[string]string{
		"NETLIFY_AUTH_TOKEN": d.NetlifyAuthToken,
		"NETLIFY_SITE_ID":    d.NetlifySiteID,
	})

	send(outputCh, fmt.Sprintf("⚙️  netlify deploy --site %s %s",
		d.NetlifySiteID,
		func() string {
			if d.NetlifyProd {
				return "--prod"
			}
			return "(preview)"
		}(),
	))

	runner := NewRunner(".", p.masker, p.log)
	result := runner.RunCommand(ctx, "netlify", args, envVars, outputCh)

	if result.Error != nil {
		return fmt.Errorf("Netlify deploy nieudany: %w", result.Error)
	}

	send(outputCh, "✅ Netlify: wdrożenie zakończone sukcesem")
	return nil
}

// readDotEnv wczytuje plik .env z podanego katalogu i zwraca mapę KEY → VALUE.
// Ignoruje komentarze (#), puste linie i nieprawidłowe wpisy.
// Format pliku .env:
//
//	KEY=value
//	KEY="wartość z spacjami"
//	# To jest komentarz
func readDotEnv(dir string) map[string]string {
	result := make(map[string]string)
	f, err := os.Open(dir + "/.env")
	if err != nil {
		return result // Brak pliku .env — normalne, nie błąd
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Pomiń komentarze i puste linie
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Pomiń linie bez znaku =
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Usuń opcjonalne cudzysłowy
		if (strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) ||
			(strings.HasPrefix(val, `'`) && strings.HasSuffix(val, `'`)) {
			val = val[1 : len(val)-1]
		}

		if key != "" {
			result[key] = val
		}
	}
	return result
}

// createSiteViaAPI tworzy nowy projekt Netlify przez REST API (bez CLI!).
// Wystarczy token — nie wymaga zainstalowanego netlify-cli.
// Projekt dostaje nazwę opartą o projectName (slugified), nie losowy hash.
func (p *netlifyProvider) createSiteViaAPI(token, projectName string, outputCh chan<- string) (string, error) {
	// Przygotuj nazwę projektu: tylko małe litery, cyfry, myślniki
	siteName := netlifySlug(projectName)
	if siteName == "" {
		siteName = "rnr-project"
	}

	send(outputCh, fmt.Sprintf("✨ Netlify: tworzenie projektu '%s' przez REST API...", siteName))

	body := fmt.Sprintf(`{"name":%q,"custom_domain":null}`, siteName)
	req, err := http.NewRequest("POST", "https://api.netlify.com/api/v1/sites", strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("błąd tworzenia zapytania do Netlify API: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("błąd połączenia z Netlify API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return "", fmt.Errorf("nieautoryzowany dostęp do Netlify API — sprawdź netlify_auth_token")
	}
	if resp.StatusCode == 422 {
		// Nazwa zajęta — spróbuj z losowym suffixem
		send(outputCh, fmt.Sprintf("⚠ Nazwa '%s' jest zajęta — dodaję losowy sufiks...", siteName))
		siteName = fmt.Sprintf("%s-%d", siteName, timeNowUnix()%10000)
		body = fmt.Sprintf(`{"name":%q}`, siteName)
		req2, _ := http.NewRequest("POST", "https://api.netlify.com/api/v1/sites", strings.NewReader(body))
		req2.Header.Set("Authorization", "Bearer "+token)
		req2.Header.Set("Content-Type", "application/json")
		resp2, err2 := http.DefaultClient.Do(req2)
		if err2 != nil {
			return "", fmt.Errorf("błąd połączenia z Netlify API (retry): %w", err2)
		}
		defer resp2.Body.Close()
		resp = resp2
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Netlify API zwróciło błąd HTTP %d — sprawdź token i spróbuj ponownie", resp.StatusCode)
	}

	var result struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("błąd parsowania odpowiedzi Netlify API: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("Netlify API nie zwróciło Site ID — sprawdź token i spróbuj ponownie")
	}

	send(outputCh, fmt.Sprintf("✅ Nowy projekt Netlify: '%s'", result.Name))
	send(outputCh, fmt.Sprintf("   🌐 URL: %s", result.URL))
	return result.ID, nil
}

// saveNetlifySiteID zapisuje nowo utworzony Site ID do rnr.conf.yaml.
// Wyszukuje środowisko po nazwie projektu i ustawia netlify_site_id.
// Ustawia też netlify_create_new: false, żeby kolejne uruchomienia nie tworzyły nowych projektów.
func (p *netlifyProvider) saveNetlifySiteID(projectName, siteID string, outputCh chan<- string) error {
	confPath := filepath.Join(p.projectRoot, config.ConfFile)

	// Wczytaj istniejący plik conf
	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("odczyt %s: %w", config.ConfFile, err)
	}

	var conf config.ConfConfig
	if err := yaml.Unmarshal(data, &conf); err != nil {
		return fmt.Errorf("parsowanie %s: %w", config.ConfFile, err)
	}
	if conf.Environments == nil {
		conf.Environments = make(map[string]config.EnvSecrets)
	}

	// Zaktualizuj site ID we wszystkich środowiskach które mają ten token
	// ale nie mają site ID (mogły być właśnie tworzone)
	updated := false
	for envName, envSecrets := range conf.Environments {
		if envSecrets.Deploy.NetlifyCreateNew && envSecrets.Deploy.NetlifySiteID == "" {
			envSecrets.Deploy.NetlifySiteID = siteID
			envSecrets.Deploy.NetlifyCreateNew = false // wyłącz auto-tworzenie po pierwszym sukcesie
			conf.Environments[envName] = envSecrets
			updated = true
			send(outputCh, fmt.Sprintf("✏️  Zaktualizowano środowisko '%s' w rnr.conf.yaml", envName))
		}
	}

	if !updated {
		return fmt.Errorf("nie znaleziono środowiska z netlify_create_new: true i pustym netlify_site_id")
	}

	// Zapisz z powrotem (atomowy zapis przez temp file)
	newData, err := yaml.Marshal(&conf)
	if err != nil {
		return fmt.Errorf("serializacja %s: %w", config.ConfFile, err)
	}

	tmpPath := confPath + ".tmp"
	if err := os.WriteFile(tmpPath, newData, 0o600); err != nil {
		return fmt.Errorf("zapis tymczasowy: %w", err)
	}
	return os.Rename(tmpPath, confPath)
}

// netlifySlug zamienia nazwę projektu na prawidłowy slug Netlify:
// tylko małe litery, cyfry i myślniki, max 63 znaki.
func netlifySlug(name string) string {
	slug := strings.ToLower(name)
	var b strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' || r == '-' || r == '.' {
			b.WriteRune('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > 63 {
		result = result[:63]
	}
	return result
}

// Rollback dla Netlify — redeployuje przywróconą wersję kodu.
func (p *netlifyProvider) Rollback(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	send(outputCh, "↩️  Netlify: ponowne wdrożenie przywróconej wersji...")
	return p.Deploy(ctx, env, outputCh)
}

// extractNetlifySiteID wyłuskuje Site ID z wyjścia `netlify sites:create`.
// Zachowane dla kompatybilności wstecznej.
func extractNetlifySiteID(output string) string {
	reJSON := regexp.MustCompile(`"id"\s*:\s*"([a-f0-9\-]{20,})"`)
	if m := reJSON.FindStringSubmatch(output); len(m) == 2 {
		return m[1]
	}
	reText := regexp.MustCompile(`(?i)site\s+id[:\s]+([a-f0-9\-]{20,})`)
	if m := reText.FindStringSubmatch(output); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// ─── Vercel Provider ──────────────────────────────────────────────────────

const vercelInstallHint = "npm install -g vercel\n  lub: yarn global add vercel"

type vercelProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewVercelProvider tworzy dostawcę Vercel.
func NewVercelProvider(masker *logger.Masker, log *logger.Logger) DeployProvider {
	return &vercelProvider{masker: masker, log: log}
}

func (p *vercelProvider) Name() string { return "Vercel" }

func (p *vercelProvider) Deploy(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	d := env.Deploy

	if d.VercelToken == "" {
		return fmt.Errorf("brak vercel_token — uzupełnij w rnr.conf.yaml → environments.X.deploy.vercel_token")
	}

	if err := checkCLI("vercel", vercelInstallHint); err != nil {
		return err
	}

	args := []string{"deploy", "--token", d.VercelToken, "--yes"}

	if d.VercelOrgID != "" {
		args = append(args, "--scope", d.VercelOrgID)
	}
	if d.VercelProd {
		args = append(args, "--prod")
		send(outputCh, "🚀 Vercel: wdrożenie na PRODUKCJĘ (--prod)")
	} else {
		send(outputCh, "🔍 Vercel: wdrożenie podglądu")
	}

	envVars := mergeEnv(env.Env, map[string]string{
		"VERCEL_TOKEN":      d.VercelToken,
		"VERCEL_ORG_ID":     d.VercelOrgID,
		"VERCEL_PROJECT_ID": d.VercelProjectID,
	})

	runner := NewRunner(".", p.masker, p.log)
	result := runner.RunCommand(ctx, "vercel", args, envVars, outputCh)

	if result.Error != nil {
		return fmt.Errorf("Vercel deploy nieudany: %w", result.Error)
	}

	send(outputCh, "✅ Vercel: wdrożenie zakończone sukcesem")
	return nil
}

func (p *vercelProvider) Rollback(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	send(outputCh, "↩️  Vercel: ponowne wdrożenie przywróconej wersji...")
	return p.Deploy(ctx, env, outputCh)
}

// ─── SSH Provider ─────────────────────────────────────────────────────────

type sshProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewSSHProvider tworzy dostawcę wdrożenia przez SSH/rsync.
func NewSSHProvider(masker *logger.Masker, log *logger.Logger) DeployProvider {
	return &sshProvider{masker: masker, log: log}
}

func (p *sshProvider) Name() string { return "SSH/rsync" }

func (p *sshProvider) Deploy(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	d := env.Deploy

	if d.SSHHost == "" {
		return fmt.Errorf("brak ssh_host — uzupełnij w rnr.yaml → environments.X.deploy.ssh_host")
	}
	if d.SSHUser == "" {
		return fmt.Errorf("brak ssh_user — uzupełnij w rnr.yaml → environments.X.deploy.ssh_user")
	}

	if err := checkCLI("rsync", "sudo apt install rsync\n  lub: brew install rsync"); err != nil {
		return err
	}

	source := d.SSHSource
	if source == "" {
		source = "dist/"
	}

	dest := fmt.Sprintf("%s@%s:%s", d.SSHUser, d.SSHHost, d.SSHPath)
	send(outputCh, fmt.Sprintf("📡 SSH: rsync %s → %s", source, dest))

	args := []string{"-avz", "--delete"}
	if d.SSHKey != "" {
		args = append(args, "-e", fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", d.SSHKey))
	}
	args = append(args, source, dest)

	runner := NewRunner(".", p.masker, p.log)
	result := runner.RunCommand(ctx, "rsync", args, env.Env, outputCh)

	if result.Error != nil {
		return fmt.Errorf("SSH deploy nieudany: %w", result.Error)
	}

	send(outputCh, "✅ SSH: synchronizacja zakończona sukcesem")
	return nil
}

func (p *sshProvider) Rollback(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	send(outputCh, "↩️  SSH: ponowna synchronizacja przywróconej wersji...")
	return p.Deploy(ctx, env, outputCh)
}

// ─── GitHub Pages Provider ────────────────────────────────────────────────

type ghPagesProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewGHPagesProvider tworzy dostawcę GitHub Pages.
func NewGHPagesProvider(masker *logger.Masker, log *logger.Logger) DeployProvider {
	return &ghPagesProvider{masker: masker, log: log}
}

func (p *ghPagesProvider) Name() string { return "GitHub Pages" }

func (p *ghPagesProvider) Deploy(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	d := env.Deploy

	branch := d.GHPagesBranch
	if branch == "" {
		branch = "gh-pages"
	}
	source := d.GHPagesSource
	if source == "" {
		source = "dist/"
	}

	send(outputCh, fmt.Sprintf("📄 GitHub Pages: publikacja %s na gałąź %s", source, branch))

	if err := checkCLI("git", "Zainstaluj Git: https://git-scm.com"); err != nil {
		return err
	}

	// Użyj git subtree push
	runner := NewRunner(".", p.masker, p.log)
	cmd := fmt.Sprintf("git subtree push --prefix=%s origin %s", source, branch)
	result := runner.RunShell(ctx, cmd, env.Env, outputCh)

	if result.Error != nil {
		// Fallback: npx gh-pages jeśli dostępny
		send(outputCh, "⚠️  git subtree nieudany, próbuję npx gh-pages...")
		if checkErr := checkCLI("npx", "npm install -g npx"); checkErr == nil {
			cmd2 := fmt.Sprintf("npx gh-pages -d %s -b %s", source, branch)
			result = runner.RunShell(ctx, cmd2, env.Env, outputCh)
		}
		if result.Error != nil {
			return fmt.Errorf("GitHub Pages deploy nieudany: %w", result.Error)
		}
	}

	send(outputCh, "✅ GitHub Pages: publikacja zakończona sukcesem")
	return nil
}

func (p *ghPagesProvider) Rollback(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	send(outputCh, "↩️  GitHub Pages: ponowna publikacja przywróconej wersji...")
	return p.Deploy(ctx, env, outputCh)
}

// ─── Docker Provider ──────────────────────────────────────────────────────

type dockerProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewDockerProvider tworzy dostawcę Docker.
func NewDockerProvider(masker *logger.Masker, log *logger.Logger) DeployProvider {
	return &dockerProvider{masker: masker, log: log}
}

func (p *dockerProvider) Name() string { return "Docker" }

func (p *dockerProvider) Deploy(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	d := env.Deploy

	if d.DockerImage == "" {
		return fmt.Errorf("brak docker_image — uzupełnij w rnr.yaml → environments.X.deploy.docker_image")
	}

	if err := checkCLI("docker", "Zainstaluj Docker: https://docs.docker.com/get-docker/"); err != nil {
		return err
	}

	tag := d.DockerTag
	if tag == "" {
		tag = "latest"
	}
	fullImage := d.DockerImage + ":" + tag

	runner := NewRunner(".", p.masker, p.log)

	// Krok 1: build obrazu
	send(outputCh, fmt.Sprintf("🐳 Docker: build %s", fullImage))
	buildResult := runner.RunCommand(ctx, "docker", []string{"build", "-t", fullImage, "."}, env.Env, outputCh)
	if buildResult.Error != nil {
		return fmt.Errorf("docker build nieudany: %w", buildResult.Error)
	}

	// Krok 2: logowanie do rejestru (jeśli skonfigurowane)
	if d.DockerRegistryUser != "" && d.DockerRegistryToken != "" {
		registry := extractRegistry(d.DockerImage)
		send(outputCh, fmt.Sprintf("🔐 Docker: logowanie do rejestru %s", registry))
		loginResult := runner.RunCommand(ctx, "docker",
			[]string{"login", registry, "-u", d.DockerRegistryUser, "--password-stdin"},
			env.Env, outputCh)
		if loginResult.Error != nil {
			return fmt.Errorf("docker login nieudany: %w", loginResult.Error)
		}
	}

	// Krok 3: push do rejestru
	send(outputCh, fmt.Sprintf("📤 Docker: push %s", fullImage))
	pushResult := runner.RunCommand(ctx, "docker", []string{"push", fullImage}, env.Env, outputCh)
	if pushResult.Error != nil {
		return fmt.Errorf("docker push nieudany: %w", pushResult.Error)
	}

	// Krok 4: uruchomienie na serwerze (jeśli skonfigurowano komendę)
	if d.DockerRunCmd != "" {
		send(outputCh, "🚀 Docker: uruchamianie nowego kontenera...")
		runResult := runner.RunShell(ctx, d.DockerRunCmd, env.Env, outputCh)
		if runResult.Error != nil {
			return fmt.Errorf("docker run nieudany: %w", runResult.Error)
		}
	}

	send(outputCh, "✅ Docker: wdrożenie zakończone sukcesem")
	return nil
}

func (p *dockerProvider) Rollback(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	send(outputCh, "↩️  Docker: ponowne wdrożenie przywróconej wersji...")
	return p.Deploy(ctx, env, outputCh)
}

// ─── Helpers ─────────────────────────────────────────────────────────────

// mergeEnv łączy dwie mapy zmiennych środowiskowych, druga nadpisuje pierwszą.
func mergeEnv(base, extra map[string]string) map[string]string {
	result := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}

// extractRegistry wyciąga nazwę rejestru z nazwy obrazu Docker.
func extractRegistry(image string) string {
	parts := strings.SplitN(image, "/", 3)
	if len(parts) >= 3 && strings.Contains(parts[0], ".") {
		return parts[0]
	}
	return "docker.io"
}
