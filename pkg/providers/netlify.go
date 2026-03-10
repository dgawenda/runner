// file: pkg/providers/netlify.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Dostawca Netlify CLI                                               ║
// ║                                                                      ║
// ║  Wrapper dla oficjalnego Netlify CLI (netlify-cli).                 ║
// ║  Automatycznie przekazuje:                                          ║
// ║    · NETLIFY_AUTH_TOKEN jako zmienną środowiskową                   ║
// ║    · --site flaga z netlify_site_id                                 ║
// ║    · --prod jeśli netlify_prod: true                                ║
// ║                                                                      ║
// ║  Token jest ZAWSZE maskowany w logach i wyjściu TUI.                ║
// ╚══════════════════════════════════════════════════════════════════════╝

package providers

import (
	"context"
	"fmt"

	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/logger"
)

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
func (p *netlifyProvider) Deploy(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	d := env.Deploy

	if d.NetlifyAuthToken == "" {
		return fmt.Errorf("brak netlify_auth_token — uzupełnij w rnr.conf.yaml")
	}
	if d.NetlifySiteID == "" {
		return fmt.Errorf("brak netlify_site_id — uzupełnij w rnr.conf.yaml")
	}

	// Buduj argumenty komendy
	args := []string{"deploy", "--site", d.NetlifySiteID, "--json"}
	if d.NetlifyProd {
		args = append(args, "--prod")
		send(outputCh, "🚀 Netlify: wdrożenie na PRODUKCJĘ (--prod)")
	} else {
		send(outputCh, "🔍 Netlify: wdrożenie podglądu (bez --prod)")
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

// Rollback dla Netlify — redeployuje poprzednią wersję.
// Netlify automatycznie obsługuje historię deployów; ten rollback
// realizowany jest przez git reset + ponowny deploy (patrz model rollbacku).
func (p *netlifyProvider) Rollback(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	send(outputCh, "↩️  Netlify: ponowne wdrożenie przywróconej wersji...")
	return p.Deploy(ctx, env, outputCh)
}

// ─── Vercel Provider ──────────────────────────────────────────────────────

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
		return fmt.Errorf("brak vercel_token — uzupełnij w rnr.conf.yaml")
	}

	args := []string{"deploy", "--token", d.VercelToken}

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
		return fmt.Errorf("brak ssh_host — uzupełnij w rnr.conf.yaml")
	}
	if d.SSHUser == "" {
		return fmt.Errorf("brak ssh_user — uzupełnij w rnr.conf.yaml")
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

	// Użyj gh-pages npm package lub git subtree
	cmd := fmt.Sprintf("npx gh-pages -d %s -b %s", source, branch)
	runner := NewRunner(".", p.masker, p.log)
	result := runner.RunShell(ctx, cmd, env.Env, outputCh)

	if result.Error != nil {
		return fmt.Errorf("GitHub Pages deploy nieudany: %w", result.Error)
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
		return fmt.Errorf("brak docker_image — uzupełnij w rnr.conf.yaml")
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
	parts := splitString(image, "/")
	if len(parts) >= 3 {
		return parts[0]
	}
	return "docker.io"
}

// splitString dzieli ciąg po separatorze.
func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}
