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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/logger"
)

const netlifyInstallHint = "npm install -g netlify-cli\n  lub: yarn global add netlify-cli"

// netlifyProvider implementuje DeployProvider dla Netlify.
type netlifyProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewNetlifyProvider tworzy dostawcę Netlify.
func NewNetlifyProvider(masker *logger.Masker, log *logger.Logger) DeployProvider {
	return &netlifyProvider{masker: masker, log: log}
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
		siteID, err := p.createSiteViaAPI(d.NetlifyAuthToken, outputCh)
		if err != nil {
			return err
		}
		d.NetlifySiteID = siteID
		send(outputCh, fmt.Sprintf("📌 ZAPAMIĘTAJ → Site ID: %s — wpisz go w rnr.conf.yaml → environments.X.deploy.netlify_site_id", siteID))
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

	// Buduj argumenty komendy
	args := []string{"deploy", "--site", d.NetlifySiteID}
	if d.NetlifyProd {
		args = append(args, "--prod")
		send(outputCh, "🚀 Netlify: wdrożenie na PRODUKCJĘ (--prod)")
	} else {
		send(outputCh, "🔍 Netlify: wdrożenie podglądu (preview deploy)")
	}

	// Zmienne środowiskowe dla Netlify CLI
	envVars := mergeEnv(env.Env, map[string]string{
		"NETLIFY_AUTH_TOKEN": d.NetlifyAuthToken,
		"NETLIFY_SITE_ID":    d.NetlifySiteID,
	})

	runner := NewRunner(".", p.masker, p.log)
	result := runner.RunCommand(ctx, "netlify", args, envVars, outputCh)

	if result.Error != nil {
		return fmt.Errorf("Netlify deploy nieudany: %w", result.Error)
	}

	send(outputCh, "✅ Netlify: wdrożenie zakończone sukcesem")
	return nil
}

// createSiteViaAPI tworzy nowy projekt Netlify przez REST API (bez CLI!).
// Wystarczy token — nie wymaga zainstalowanego netlify-cli.
func (p *netlifyProvider) createSiteViaAPI(token string, outputCh chan<- string) (string, error) {
	send(outputCh, "✨ Netlify: tworzenie nowego projektu przez API...")

	req, err := http.NewRequest("POST", "https://api.netlify.com/api/v1/sites", strings.NewReader("{}"))
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

	send(outputCh, fmt.Sprintf("✅ Nowy projekt Netlify: %s (URL: %s)", result.Name, result.URL))
	return result.ID, nil
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
