// file: pkg/logger/logger.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Logger Wdrożeń z Maskowaniem Sekretów                              ║
// ║                                                                      ║
// ║  Zapisuje logi wdrożeń do plików w .rnr/logs/ z timestampami.       ║
// ║  Każda linia jest sanityzowana przez Masker przed zapisem.          ║
// ║  Pliki logów mają czytelne nazwy: 2024-01-15_14-30-00_prod.log      ║
// ╚══════════════════════════════════════════════════════════════════════╝

package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level reprezentuje poziom ważności wpisu logu.
type Level string

const (
	LevelInfo    Level = "INFO"
	LevelSuccess Level = "OK  "
	LevelWarning Level = "WARN"
	LevelError   Level = "ERR "
	LevelDebug   Level = "DBG "
)

// Logger zapisuje zdarzenia wdrożenia do pliku i opcjonalnie do dodatkowego writera.
// Wszystkie wpisy są maskowane z sekretów przed zapisem.
type Logger struct {
	mu       sync.Mutex
	file     *os.File
	masker   *Masker
	tee      io.Writer // opcjonalny dodatkowy writer (np. TUI channel)
	filePath string
}

// New tworzy nowy Logger zapisujący do podanego pliku.
// Jeśli plik nie istnieje, zostanie automatycznie utworzony.
// masker może być nil — w takim przypadku używany jest NopMasker.
func New(logDir, filename string, masker *Masker) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("nie można utworzyć katalogu logów %s: %w", logDir, err)
	}

	if masker == nil {
		masker = NopMasker()
	}

	filePath := filepath.Join(logDir, filename)
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("nie można otworzyć pliku logu %s: %w", filePath, err)
	}

	return &Logger{
		file:     f,
		masker:   masker,
		filePath: filePath,
	}, nil
}

// NewForDeployment tworzy Logger dla konkretnego wdrożenia.
// Nazwa pliku jest generowana automatycznie na podstawie środowiska i czasu.
func NewForDeployment(logsDir, env string, masker *Masker) (*Logger, error) {
	now := time.Now()
	filename := fmt.Sprintf("%s_%s.log", now.Format("2006-01-02_15-04-05"), env)
	return New(logsDir, filename, masker)
}

// SetTee ustawia dodatkowy writer (np. kanał TUI) do którego trafiają wpisy logu.
// Wpisy trafiają do pliku I do tee równocześnie.
func (l *Logger) SetTee(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tee = w
}

// Info zapisuje wpis informacyjny.
func (l *Logger) Info(format string, args ...any) {
	l.write(LevelInfo, format, args...)
}

// Success zapisuje wpis o sukcesie.
func (l *Logger) Success(format string, args ...any) {
	l.write(LevelSuccess, format, args...)
}

// Warn zapisuje ostrzeżenie.
func (l *Logger) Warn(format string, args ...any) {
	l.write(LevelWarning, format, args...)
}

// Error zapisuje błąd.
func (l *Logger) Error(format string, args ...any) {
	l.write(LevelError, format, args...)
}

// Debug zapisuje wpis debugowania.
func (l *Logger) Debug(format string, args ...any) {
	l.write(LevelDebug, format, args...)
}

// Raw zapisuje surowy ciąg (np. wyjście zewnętrznego procesu) bez formatowania.
// Nadal przechodzi przez masker.
func (l *Logger) Raw(text string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		sanitized := l.masker.Sanitize(line)
		l.writeLine(sanitized)
	}
}

// FilePath zwraca ścieżkę do aktualnego pliku logu.
func (l *Logger) FilePath() string {
	return l.filePath
}

// Close zamyka plik logu. Powinno być wywołane defer po New.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// ─── Metody wewnętrzne ─────────────────────────────────────────────────────

func (l *Logger) write(level Level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	msg = l.masker.Sanitize(msg)

	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] [%s] %s", timestamp, level, msg)
	l.writeLine(line)
}

func (l *Logger) writeLine(line string) {
	entry := line + "\n"
	if l.file != nil {
		_, _ = l.file.WriteString(entry)
	}
	if l.tee != nil {
		_, _ = l.tee.Write([]byte(entry))
	}
}

// ─── TeeWriter ──────────────────────────────────────────────────────────────

// TeeWriter implementuje io.Writer przekazując dane zarówno do loggera jak i kanału TUI.
// Używany do przechwytywania wyjścia zewnętrznych procesów.
type TeeWriter struct {
	logger *Logger
	ch     chan<- string
}

// NewTeeWriter tworzy writer który pisze do loggera i kanału TUI.
func NewTeeWriter(logger *Logger, ch chan<- string) *TeeWriter {
	return &TeeWriter{logger: logger, ch: ch}
}

// Write implementuje io.Writer.
func (t *TeeWriter) Write(p []byte) (int, error) {
	text := string(p)
	t.logger.Raw(text)
	if t.ch != nil {
		lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
		for _, line := range lines {
			if line != "" {
				select {
				case t.ch <- line:
				default: // Nie blokuj jeśli kanał pełny
				}
			}
		}
	}
	return len(p), nil
}
