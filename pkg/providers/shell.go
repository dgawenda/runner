// file: pkg/providers/shell.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Executor Komend Powłoki z Maskowaniem Sekretów                     ║
// ║                                                                      ║
// ║  Centralny moduł wykonywania zewnętrznych procesów.                 ║
// ║  KAŻDA linia wyjścia (stdout + stderr) przechodzi przez masker      ║
// ║  zanim trafi do logów lub TUI.                                      ║
// ║                                                                      ║
// ║  Udostępnia zarówno niskopoziomowy RunCommand jak i wygodny         ║
// ║  RunShell (dla komend powłoki z sh -c "...").                       ║
// ╚══════════════════════════════════════════════════════════════════════╝

package providers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/neution/rnr/pkg/config"
	"github.com/neution/rnr/pkg/logger"
)

// ─── Runner ────────────────────────────────────────────────────────────────

// Runner wykonuje zewnętrzne procesy ze strumieniowaniem wyjścia.
type Runner struct {
	masker  *logger.Masker
	log     *logger.Logger
	workdir string
}

// NewRunner tworzy nowy Runner dla podanego katalogu roboczego.
func NewRunner(workdir string, masker *logger.Masker, log *logger.Logger) *Runner {
	return &Runner{
		masker:  masker,
		log:     log,
		workdir: workdir,
	}
}

// RunResult zawiera wynik wykonania procesu.
type RunResult struct {
	// ExitCode — kod wyjścia procesu.
	ExitCode int
	// Output — wszystkie linie wyjścia (zamaskowane).
	Output []string
	// Error — błąd jeśli proces zakończył się niepowodzeniem.
	Error error
}

// ─── Wykonanie Komend ─────────────────────────────────────────────────────

// RunShell wykonuje komendę powłoki używając sh -c "command".
// Wszystkie zmienne środowiskowe z envVars są eksportowane przed wykonaniem.
// Wyjście jest strumieniowane do outputCh linia po linii, zamaskowane.
func (r *Runner) RunShell(ctx context.Context, command string, envVars map[string]string, outputCh chan<- string) *RunResult {
	return r.RunCommand(ctx, "sh", []string{"-c", command}, envVars, outputCh)
}

// RunCommand wykonuje podany program z argumentami.
// envVars są mergowane z aktualnym środowiskiem procesu.
func (r *Runner) RunCommand(ctx context.Context, program string, args []string, envVars map[string]string, outputCh chan<- string) *RunResult {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = r.workdir
	cmd.Env = BuildEnv(envVars)

	result := &RunResult{}

	// Podłącz strumienie wyjścia
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		result.Error = fmt.Errorf("nie można podłączyć stdout: %w", err)
		return result
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		result.Error = fmt.Errorf("nie można podłączyć stderr: %w", err)
		return result
	}

	// Uruchom proces
	if err := cmd.Start(); err != nil {
		result.Error = fmt.Errorf("nie można uruchomić %s: %w", program, err)
		return result
	}

	// Strumieniuj stdout i stderr równolegle
	doneCh := make(chan struct{}, 2)

	streamFn := func(pipe io.Reader) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := r.masker.Sanitize(scanner.Text())
			result.Output = append(result.Output, line)
			if r.log != nil {
				r.log.Raw(line)
			}
			send(outputCh, line)
		}
		doneCh <- struct{}{}
	}

	go streamFn(stdoutPipe)
	go streamFn(stderrPipe)

	// Poczekaj na zakończenie strumieniowania
	<-doneCh
	<-doneCh

	// Poczekaj na zakończenie procesu
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Error = fmt.Errorf("proces zakończony błędem (kod %d)", result.ExitCode)
	}

	return result
}

// ─── Helpers ─────────────────────────────────────────────────────────────

// BuildEnv buduje slice zmiennych środowiskowych mergując aktualne środowisko z dodatkowymi.
func BuildEnv(extra map[string]string) []string {
	// Zacznij od aktualnego środowiska
	env := os.Environ()

	// Dodaj/nadpisz zmienymi z mapy
	for k, v := range extra {
		// Usuń istniejącą wartość jeśli jest
		prefix := k + "="
		filtered := make([]string, 0, len(env))
		for _, e := range env {
			if !strings.HasPrefix(e, prefix) {
				filtered = append(filtered, e)
			}
		}
		env = append(filtered, k+"="+v)
	}

	return env
}

// executeCustomCmd wykonuje własną komendę z konfiguracji środowiska.
func executeCustomCmd(ctx context.Context, cmd string, env config.Environment, masker *logger.Masker, log *logger.Logger, outputCh chan<- string) error {
	if cmd == "" {
		return fmt.Errorf("pusta komenda — sprawdź konfigurację w rnr.conf.yaml")
	}
	runner := NewRunner(".", masker, log)
	result := runner.RunShell(ctx, cmd, env.Env, outputCh)
	return result.Error
}

// ─── Custom Deploy Provider ────────────────────────────────────────────────

type customDeployProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewCustomDeployProvider tworzy dostawcę wykonującego własną komendę deployu.
func NewCustomDeployProvider(masker *logger.Masker, log *logger.Logger) DeployProvider {
	return &customDeployProvider{masker: masker, log: log}
}

func (p *customDeployProvider) Name() string { return "custom" }

func (p *customDeployProvider) Deploy(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	return executeCustomCmd(ctx, env.Deploy.DeployCmd, env, p.masker, p.log, outputCh)
}

func (p *customDeployProvider) Rollback(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	if env.Deploy.RollbackCmd == "" {
		send(outputCh, "Brak komendy rollback dla dostawcy custom — rollback odbywa się przez git.")
		return nil
	}
	return executeCustomCmd(ctx, env.Deploy.RollbackCmd, env, p.masker, p.log, outputCh)
}

// ─── Custom Database Provider ─────────────────────────────────────────────

type customDatabaseProvider struct {
	masker *logger.Masker
	log    *logger.Logger
}

// NewCustomDatabaseProvider tworzy dostawcę bazy z własną komendą migracji.
func NewCustomDatabaseProvider(masker *logger.Masker, log *logger.Logger) DatabaseProvider {
	return &customDatabaseProvider{masker: masker, log: log}
}

func (p *customDatabaseProvider) Name() string { return "custom" }

func (p *customDatabaseProvider) Migrate(ctx context.Context, env config.Environment, outputCh chan<- string) error {
	if env.Database.DBMigrateCmd == "" {
		return fmt.Errorf("brak komendy migracji (db_migrate_cmd) — sprawdź rnr.conf.yaml")
	}
	return executeCustomCmd(ctx, env.Database.DBMigrateCmd, env, p.masker, p.log, outputCh)
}

func (p *customDatabaseProvider) Promote(_ context.Context, _, _ config.Environment, outputCh chan<- string) error {
	send(outputCh, "Promote nie jest obsługiwany przez dostawcę custom — wykonaj ręcznie.")
	return nil
}
