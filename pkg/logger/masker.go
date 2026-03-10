// file: pkg/logger/masker.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Silnik Maskowania Sekretów                                         ║
// ║                                                                      ║
// ║  Gwarantuje, że żaden token, hasło ani klucz API nie trafi          ║
// ║  w jawnej postaci do logów, stdout ani stderr.                      ║
// ║                                                                      ║
// ║  Każda linia wyjścia z zewnętrznego procesu (Netlify CLI,           ║
// ║  Supabase CLI, etc.) przechodzi przez Masker.Sanitize()             ║
// ║  zanim zostanie zapisana lub wyświetlona.                           ║
// ╚══════════════════════════════════════════════════════════════════════╝

package logger

import (
	"strings"
)

// MaskedPlaceholder to ciąg zastępujący maskowany sekret w logach.
const MaskedPlaceholder = "***"

// Masker przechowuje listę sekretów i zastępuje je w ciągach tekstowych.
// Bezpieczny do użycia współbieżnego (read-only po inicjalizacji).
type Masker struct {
	secrets []string
}

// NewMasker tworzy nowy Masker z podanymi sekretami.
// Puste ciągi i ciągi krótsze niż 4 znaki są ignorowane
// (za krótkie do bezpiecznego rozpoznania bez fałszywych alarmów).
func NewMasker(secrets ...string) *Masker {
	filtered := make([]string, 0, len(secrets))
	for _, s := range secrets {
		s = strings.TrimSpace(s)
		if len(s) >= 4 {
			filtered = append(filtered, s)
		}
	}
	return &Masker{secrets: filtered}
}

// NopMasker zwraca Masker który nic nie maskuje (używany gdy brak konfiguracji).
func NopMasker() *Masker {
	return &Masker{}
}

// Sanitize zastępuje wszystkie znane sekrety placeholderem w podanym ciągu.
// Metoda jest idempotentna: wielokrotne wywołanie daje ten sam wynik.
func (m *Masker) Sanitize(s string) string {
	for _, secret := range m.secrets {
		s = strings.ReplaceAll(s, secret, MaskedPlaceholder)
	}
	return s
}

// SanitizeLines stosuje Sanitize do każdej linii w slice'u.
func (m *Masker) SanitizeLines(lines []string) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = m.Sanitize(line)
	}
	return result
}

// HasSecrets zwraca true jeśli Masker posiada co najmniej jeden sekret.
func (m *Masker) HasSecrets() bool {
	return len(m.secrets) > 0
}

// Count zwraca liczbę zarejestrowanych sekretów.
func (m *Masker) Count() int {
	return len(m.secrets)
}

// MaskingWriter to io.Writer który maskuje sekrety przed zapisem.
// Używany jako wrapper dla os.Stdout/Stderr przy execu zewnętrznych procesów.
type MaskingWriter struct {
	masker    *Masker
	writeFn   func(p []byte) (int, error)
	remainder []byte
}

// NewMaskingWriter tworzy MaskingWriter owijający podaną funkcję zapisu.
func NewMaskingWriter(masker *Masker, writeFn func(p []byte) (int, error)) *MaskingWriter {
	return &MaskingWriter{masker: masker, writeFn: writeFn}
}

// Write implementuje io.Writer. Buforuje dane do pełnej linii, maskuje i zapisuje.
func (w *MaskingWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.remainder = append(w.remainder, p...)

	for {
		idx := strings.IndexByte(string(w.remainder), '\n')
		if idx < 0 {
			break
		}
		line := string(w.remainder[:idx+1])
		w.remainder = w.remainder[idx+1:]

		sanitized := w.masker.Sanitize(line)
		if _, err := w.writeFn([]byte(sanitized)); err != nil {
			return n, err
		}
	}
	return n, nil
}

// Flush zapisuje pozostałe dane w buforze (bez kończącego newline).
func (w *MaskingWriter) Flush() error {
	if len(w.remainder) > 0 {
		sanitized := w.masker.Sanitize(string(w.remainder))
		_, err := w.writeFn([]byte(sanitized))
		w.remainder = nil
		return err
	}
	return nil
}
