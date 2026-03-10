// file: pkg/providers/github.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Integracja GitHub — Releases i Tagi Wdrożeń                       ║
// ║                                                                      ║
// ║  Automatyzuje tworzenie tagów i releasów GitHub po sukcesie         ║
// ║  wdrożenia na produkcję. Generuje czytelne notatki ze zmienionych   ║
// ║  commitów używając konwencji Conventional Commits.                  ║
// ╚══════════════════════════════════════════════════════════════════════╝

package providers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/logger"
)

// ─── GitHub Provider ──────────────────────────────────────────────────────

// GitHubProvider obsługuje integrację z GitHub (tagi, releasey).
type GitHubProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewGitHubProvider tworzy dostawcę GitHub.
func NewGitHubProvider(masker *logger.Masker, log *logger.Logger) *GitHubProvider {
	return &GitHubProvider{masker: masker, log: log}
}

// CreateRelease tworzy nowy release na GitHubie po sukcesie wdrożenia.
// Używa GitHub CLI (gh) jeśli jest dostępne, lub git tag + push.
func (g *GitHubProvider) CreateRelease(ctx context.Context, proj config.ProjectInfo, env, version, commitHash, notes string, outputCh chan<- string) error {
	tagName := fmt.Sprintf("v%s-%s-%s",
		version,
		env,
		time.Now().Format("20060102"))

	send(outputCh, fmt.Sprintf("🏷️  GitHub: tworzę tag %s...", tagName))

	runner := NewRunner(".", g.masker, g.log)

	// Sprawdź czy gh CLI jest dostępne
	checkResult := runner.RunCommand(ctx, "gh", []string{"--version"}, nil, nil)
	if checkResult.Error == nil {
		// Użyj gh CLI
		return g.createReleaseWithGHCLI(ctx, runner, tagName, notes, commitHash, outputCh)
	}

	// Fallback: git tag + push
	return g.createReleaseWithGit(ctx, runner, tagName, notes, commitHash, outputCh)
}

// createReleaseWithGHCLI tworzy release używając oficjalnego GitHub CLI.
func (g *GitHubProvider) createReleaseWithGHCLI(ctx context.Context, runner *Runner, tag, notes, commitHash string, outputCh chan<- string) error {
	args := []string{
		"release", "create", tag,
		"--title", tag,
		"--notes", notes,
		"--target", commitHash,
	}

	result := runner.RunCommand(ctx, "gh", args, nil, outputCh)
	if result.Error != nil {
		return fmt.Errorf("gh release create nieudane: %w", result.Error)
	}

	send(outputCh, fmt.Sprintf("✅ GitHub: release %s utworzony", tag))
	return nil
}

// createReleaseWithGit tworzy tag Git i pushuje go do origin.
func (g *GitHubProvider) createReleaseWithGit(ctx context.Context, runner *Runner, tag, message, commitHash string, outputCh chan<- string) error {
	// Utwórz anotowany tag
	tagResult := runner.RunCommand(ctx, "git",
		[]string{"tag", "-a", tag, commitHash, "-m", message},
		nil, outputCh)
	if tagResult.Error != nil {
		return fmt.Errorf("git tag nieudany: %w", tagResult.Error)
	}

	// Wypchnij tag na origin
	pushResult := runner.RunCommand(ctx, "git",
		[]string{"push", "origin", tag},
		nil, outputCh)
	if pushResult.Error != nil {
		return fmt.Errorf("git push tag nieudany: %w", pushResult.Error)
	}

	send(outputCh, fmt.Sprintf("✅ GitHub: tag %s wypchnięty", tag))
	return nil
}

// ─── Generowanie Notatek do Releasów ─────────────────────────────────────

// GenerateReleaseNotes generuje czytelne notatki do release'u.
// Formatuje listę commitów od ostatniego taga/wdrożenia do HEAD.
func GenerateReleaseNotes(projectName, env, currentVersion string, commits []CommitSummary) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 🚀 Wdrożenie %s na %s\n\n", projectName, env))
	sb.WriteString(fmt.Sprintf("**Wersja:** %s\n", currentVersion))
	sb.WriteString(fmt.Sprintf("**Data:** %s\n\n", time.Now().Format("02.01.2006 15:04")))

	if len(commits) > 0 {
		sb.WriteString("### Zmiany w tej wersji\n\n")

		// Grupuj po typach commitów
		grouped := groupCommitsByType(commits)

		if feats, ok := grouped["feat"]; ok {
			sb.WriteString("#### ✨ Nowe funkcje\n")
			for _, c := range feats {
				sb.WriteString(fmt.Sprintf("- %s (`%s`)\n", c.Description, c.ShortHash))
			}
			sb.WriteString("\n")
		}
		if fixes, ok := grouped["fix"]; ok {
			sb.WriteString("#### 🐛 Naprawione błędy\n")
			for _, c := range fixes {
				sb.WriteString(fmt.Sprintf("- %s (`%s`)\n", c.Description, c.ShortHash))
			}
			sb.WriteString("\n")
		}
		if others, ok := grouped["other"]; ok {
			sb.WriteString("#### 🔧 Pozostałe zmiany\n")
			for _, c := range others {
				sb.WriteString(fmt.Sprintf("- %s (`%s`)\n", c.Description, c.ShortHash))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("---\n")
	sb.WriteString("*Wygenerowano automatycznie przez rnr*\n")

	return sb.String()
}

// CommitSummary przechowuje skrócone informacje o commicie do notatek release.
type CommitSummary struct {
	ShortHash   string
	Type        string // feat, fix, chore, docs, etc.
	Scope       string
	Description string
	Author      string
}

// ParseCommitMessage parsuje wiadomość commita zgodnie z Conventional Commits.
// Format: type(scope): description
func ParseCommitMessage(hash, message string) CommitSummary {
	summary := CommitSummary{
		ShortHash:   hash,
		Description: message,
		Type:        "other",
	}

	// Sprawdź czy to Conventional Commit
	colonIdx := strings.Index(message, ":")
	if colonIdx < 0 {
		return summary
	}

	prefix := message[:colonIdx]
	description := strings.TrimSpace(message[colonIdx+1:])

	// Wyciągnij type i scope
	openParen := strings.Index(prefix, "(")
	closeParen := strings.Index(prefix, ")")

	if openParen >= 0 && closeParen > openParen {
		summary.Type = prefix[:openParen]
		summary.Scope = prefix[openParen+1 : closeParen]
	} else {
		summary.Type = prefix
	}

	summary.Description = description
	return summary
}

// groupCommitsByType grupuje commity według ich typu.
func groupCommitsByType(commits []CommitSummary) map[string][]CommitSummary {
	grouped := map[string][]CommitSummary{}
	for _, c := range commits {
		switch c.Type {
		case "feat", "feature":
			grouped["feat"] = append(grouped["feat"], c)
		case "fix", "bugfix":
			grouped["fix"] = append(grouped["fix"], c)
		default:
			grouped["other"] = append(grouped["other"], c)
		}
	}
	return grouped
}
