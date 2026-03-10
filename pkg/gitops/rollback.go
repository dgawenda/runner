// file: pkg/gitops/rollback.go
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║  Silnik Rollbacku — Bezpieczne Przywracanie Zmian                   ║
// ║                                                                      ║
// ║  Implementuje mechanizm powrotu do stabilnego stanu przed wadliwym  ║
// ║  wdrożeniem. Używa informacji z state.json do identyfikacji         ║
// ║  dokładnego commita do którego należy wrócić.                       ║
// ║                                                                      ║
// ║  OSTRZEŻENIE BEZPIECZEŃSTWA:                                        ║
// ║  Rollback kodu aplikacji NIE COFA migracji bazy danych.             ║
// ║  Bazy danych używają zasady "roll-forward" — cofanie rekordów       ║
// ║  może spowodować nieodwracalne uszkodzenie danych.                  ║
// ╚══════════════════════════════════════════════════════════════════════╝

package gitops

import (
	"fmt"
)

// ─── Typy ──────────────────────────────────────────────────────────────────

// RollbackTarget definiuje cel operacji rollbacku.
type RollbackTarget struct {
	// CommitHash — hash commita do którego wracamy.
	CommitHash string
	// Branch — gałąź backup z której przywracamy (opcjonalna).
	Branch string
	// Tag — tag backup z którego przywracamy (opcjonalna).
	Tag string
	// Description — czytelny opis celu rollbacku.
	Description string
}

// RollbackMode określa tryb przywracania zmian.
type RollbackMode int

const (
	// RollbackModeCheckout — git checkout (bezpieczny, nie niszczy historii).
	RollbackModeCheckout RollbackMode = iota
	// RollbackModeHardReset — git reset --hard (agresywny, zmienia HEAD).
	RollbackModeHardReset
)

// ─── Rollback ─────────────────────────────────────────────────────────────

// RestoreSnapshot przywraca repozytorium do stanu z podanego snapshotu.
// Operacja:
//  1. Weryfikuje że docelowy commit istnieje
//  2. Wykonuje hard reset do docelowego commita
//  3. Zwraca informację o przywróconym stanie
//
// WAŻNE: Ta funkcja celowo NIE cofa migracji bazy danych.
// Wywołujący jest odpowiedzialny za wyświetlenie odpowiedniego ostrzeżenia.
func RestoreSnapshot(workdir string, target RollbackTarget) error {
	if target.CommitHash == "" {
		return fmt.Errorf("brak hasha commita w celu rollbacku")
	}

	// Weryfikacja że commit istnieje
	if err := verifyCommitExists(workdir, target.CommitHash); err != nil {
		return fmt.Errorf("docelowy commit %s nie istnieje: %w",
			truncateHash(target.CommitHash), err)
	}

	// Przywróć do docelowego commita
	if err := hardResetToCommit(workdir, target.CommitHash); err != nil {
		return fmt.Errorf("nie można przywrócić do %s: %w",
			truncateHash(target.CommitHash), err)
	}

	return nil
}

// RestoreFromBranch przywraca stan z gałęzi backup.
// Preferowana metoda gdy snapshot tworzył gałąź backup.
func RestoreFromBranch(workdir, backupBranch string) error {
	// Sprawdź czy gałąź backup istnieje
	exists, err := BranchExists(workdir, backupBranch)
	if err != nil {
		return fmt.Errorf("nie można sprawdzić gałęzi %s: %w", backupBranch, err)
	}
	if !exists {
		return fmt.Errorf("gałąź backup %s nie istnieje — snapshot mógł zostać usunięty", backupBranch)
	}

	// Pobierz hash commita gałęzi backup
	commitHash, err := GetCommitHash(workdir, backupBranch)
	if err != nil {
		return fmt.Errorf("nie można odczytać hasha gałęzi %s: %w", backupBranch, err)
	}

	return hardResetToCommit(workdir, commitHash)
}

// hardResetToCommit wykonuje git reset --hard do podanego commita.
// Czyści nieśledzone pliki aby zapewnić czysty stan roboczy.
func hardResetToCommit(workdir, commitHash string) error {
	// Hard reset — przesuwa HEAD i index do commita
	if _, err := runGit(workdir, "reset", "--hard", commitHash); err != nil {
		return fmt.Errorf("git reset --hard: %w", err)
	}

	// Wyczyść nieśledzone pliki i katalogi
	if _, err := runGit(workdir, "clean", "-fd"); err != nil {
		// Czyszczenie jest pomocnicze — nie przerywaj przy błędzie
		_ = err
	}

	return nil
}

// verifyCommitExists sprawdza czy commit o podanym hashu istnieje w repozytorium.
func verifyCommitExists(workdir, commitHash string) error {
	_, err := runGit(workdir, "cat-file", "-e", commitHash+"^{commit}")
	return err
}

// ─── Helpers ──────────────────────────────────────────────────────────────

// truncateHash skraca hash do 7 znaków dla czytelności.
func truncateHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// BuildRollbackTarget tworzy RollbackTarget z danych snapshotu.
func BuildRollbackTarget(commitHash, branch, tag, env string) RollbackTarget {
	return RollbackTarget{
		CommitHash:  commitHash,
		Branch:      branch,
		Tag:         tag,
		Description: fmt.Sprintf("Przywróć %s do %s", env, truncateHash(commitHash)),
	}
}
