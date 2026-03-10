# rnr — Runner

> **Potężne narzędzie wdrożeniowe dla Twojego zespołu.**  
> Zapomnij o skomplikowanych platformach CI/CD. `rnr` to przyjazny interfejs terminalowy (TUI), który przeprowadzi Cię przez cały proces wdrożenia — krok po kroku, z animacjami, paskami postępu i pełnym bezpieczeństwem dla Twojego kodu.

---

## Spis treści

1. [Dla kogo jest rnr?](#1-dla-kogo-jest-rnr)
2. [Instalacja jedną komendą](#2-instalacja-jedną-komendą)
3. [Pierwsze uruchomienie — Setup Wizard](#3-pierwsze-uruchomienie--setup-wizard)
4. [Filozofia działania](#4-filozofia-działania)
5. [Jak działa GitOps i ochrona kodu](#5-jak-działa-gitops-i-ochrona-kodu)
6. [Snapshoty i system Rollback](#6-snapshoty-i-system-rollback)
7. [Potoki wdrożeniowe (Pipeline)](#7-potoki-wdrożeniowe-pipeline)
8. [Pliki konfiguracyjne — rnr.yaml i rnr.conf.yaml](#8-pliki-konfiguracyjne--rnryaml-i-rnrconfyaml)
9. [Integracje z zewnętrznymi dostawcami](#9-integracje-z-zewnętrznymi-dostawcami)
10. [Migracje bazy danych Supabase](#10-migracje-bazy-danych-supabase)
11. [Maskowanie sekretów — bezpieczeństwo tokenów](#11-maskowanie-sekretów--bezpieczeństwo-tokenów)
12. [Struktura katalogów projektu](#12-struktura-katalogów-projektu)
13. [Nazewnictwo gałęzi Git i dzienniki wdrożeń](#13-nazewnictwo-gałęzi-git-i-dzienniki-wdrożeń)
14. [Komendy CLI](#14-komendy-cli)
15. [FAQ — Często zadawane pytania](#15-faq--często-zadawane-pytania)

---

## 1. Dla kogo jest rnr?

`rnr` powstał z jednej, bardzo konkretnej potrzeby: **wdrożenia kodu nie powinno bać się nikt w zespole**.

Wyobraź sobie sytuację — Twój współpracownik ukończył prace nad nową funkcją, ale jest piątek po południu, a specjalista od DevOps jest niedostępny. Normalnie oznaczałoby to czekanie do poniedziałku lub ryzykowne próby z ręcznym deploymentem przez SSH, `git push` do magicznych gałęzi lub klikanie w dziesiątkach zakładek różnych dashboardów chmurowych.

**Z `rnr` cały ten stres znika.**

`rnr` to narzędzie, które:

- 🛡️ **Chroni Twój kod** — przed wdrożeniem sprawdza, czy repozytorium jest czyste, i automatycznie tworzy kopię zapasową
- 🎯 **Prowadzi za rękę** — interaktywny Setup Wizard i przyjazny Dashboard z klawiaturą strzałkową
- ⚡ **Działa błyskawicznie** — napisany w języku Go, uruchamia się w ułamku sekundy
- 🔐 **Chroni Twoje sekrety** — tokeny API nigdy nie pojawiają się w logach ani na ekranie
- ↩️ **Pozwala cofnąć się** — jednym naciśnięciem klawisza możesz przywrócić poprzedni, sprawdzony stan

Jeśli kiedykolwiek bałeś się wpisać `git push` lub nie rozumiałeś, co dzieje się po kliknięciu „Deploy" — `rnr` jest właśnie dla Ciebie.

---

## 2. Instalacja jedną komendą

> ⚡ **One-click install** — wszystko, czego potrzebujesz, to jeden wiersz w terminalu.

Przed uruchomieniem skryptu upewnij się, że posiadasz **token dostępu** do prywatnego repozytorium GitHub organizacji. Skontaktuj się z administratorem, aby go uzyskać.

```bash
GITHUB_TOKEN="twój_token_dostępu" bash <(curl -fsSL https://raw.githubusercontent.com/TWOJA_ORGANIZACJA/rnr/main/install.sh)
```

> 💡 Zamień `TWOJA_ORGANIZACJA` na nazwę organizacji GitHub oraz `twój_token_dostępu` na przydzielony token PAT.

### Co robi skrypt instalacyjny?

Skrypt `install.sh` wykonuje następujące kroki automatycznie:

1. **Wykrywa Twój system operacyjny** (Linux, macOS, Windows) i architekturę procesora (amd64, arm64)
2. **Pobiera najnowszą stabilną wersję** binarną `rnr` z prywatnego repozytorium GitHub przy użyciu autoryzowanego zapytania API (`Authorization: Bearer <TOKEN>`)
3. **Tworzy ukryty katalog** `.rnr` w Twoim katalogu domowym
4. **Instaluje plik wykonywalny** `rnr` w lokalizacji `~/.rnr/rnr`
5. **Aktualizuje PATH** — dodaje `~/.rnr` do zmiennej środowiskowej `PATH` w pliku `.bashrc`, `.zshrc` lub `.profile`
6. **Informuje o następnych krokach** — po instalacji wystarczy wpisać `rnr` w terminalu

Po zakończeniu instalacji wystarczy uruchomić nową sesję terminala lub wykonać:

```bash
source ~/.bashrc  # lub ~/.zshrc, jeśli używasz ZSH
```

A następnie sprawdzić, czy instalacja przebiegła pomyślnie:

```bash
rnr --version
```

---

## 3. Pierwsze uruchomienie — Setup Wizard

Kiedy uruchomisz `rnr` po raz pierwszy w katalogu projektu, który nie posiada jeszcze plików konfiguracyjnych, narzędzie automatycznie uruchomi **interaktywny kreator konfiguracji (Setup Wizard)**.

```
╔══════════════════════════════════════════════════════╗
║          Witaj w rnr — Runner Setup Wizard           ║
║                                                      ║
║  Przeprowadzę Cię przez konfigurację krok po kroku.  ║
║  Użyj klawiszy ↑↓ do nawigacji, Enter do wyboru.    ║
╚══════════════════════════════════════════════════════╝
```

Kreator zapyta Cię o:

- **Nazwę projektu** i URL repozytorium GitHub
- **Wybór dostawców** (Netlify, Supabase, GitHub Releases, własne skrypty)
- **Dane autoryzacyjne** — tokeny API wpisywane w bezpiecznych, maskowanych polach (`*****`)
- **Nazwy środowisk** — np. `staging` i `production`

Po zakończeniu kreatora `rnr` automatycznie:
- Wygeneruje plik `rnr.yaml` (konfiguracja potoku — bezpieczna do commitowania)
- Wygeneruje plik `rnr.conf.yaml` (sekrety — **nigdy nie trafia do Gita**)
- Doda `rnr.conf.yaml` do `.gitignore` automatycznie
- Uruchomi główny Dashboard

---

## 4. Filozofia działania

`rnr` opiera się na trzech filarach:

### 🎭 Zero-Config — żadnego bólu konfiguracyjnego

Nowe narzędzie powinno działać od razu po instalacji. Setup Wizard prowadzi Cię przez każdy krok z przyjaznym językiem. Nie musisz rozumieć, czym jest `CI/CD pipeline` — wystarczy, że odpiszesz na pytania kreatora.

### 🛡️ Safety-First — bezpieczeństwo na pierwszym miejscu

Każde wdrożenie poprzedzone jest **audytem repozytorium Git**. Jeśli cokolwiek jest „brudne" (niezacommitowane zmiany), `rnr` grzecznie odmówi i poinformuje Cię, co wymaga poprawy. Nigdy nie wdroży nieznanego kodu.

### 🔮 GitOps — Git jako źródło prawdy

Każda operacja wdrożeniowa jest utrwalana w historii Git. `rnr` tworzy dedykowane gałęzie zapasowe (`rnr_backup_*`) przed każdym deploymentem, dzięki czemu zawsze możesz wrócić do poprzedniego stanu — nawet jeśli coś pójdzie nie tak.

---

## 5. Jak działa GitOps i ochrona kodu

Zanim `rnr` wykona jakiekolwiek polecenie wdrożeniowe, przeprowadza **pełny audyt drzewa Git**:

```
[1/4] 🔍 Sprawdzanie stanu repozytorium...
      git status --porcelain

[2/4] 📸 Tworzenie snapshotu przedwdrożeniowego...
      Gałąź: rnr_backup_production_20260310_143022

[3/4] 💾 Zapisywanie stanu w .rnr/state.json...

[4/4] 🚀 Uruchamianie potoku wdrożeniowego...
```

### Co się dzieje przy „brudnym" repozytorium?

Jeśli wykryjesz niezacommitowane pliki lub nieśledzone zmiany, `rnr` wyświetli przyjazne ostrzeżenie:

```
╔══════════════════════════════════════════════════════╗
║  ⚠️  Uwaga! Repozytorium jest nieczysté              ║
║                                                      ║
║  Następujące pliki mają niezacommitowane zmiany:     ║
║  → src/components/Header.tsx (zmodyfikowany)         ║
║  → src/utils/api.ts (nowy plik)                      ║
║                                                      ║
║  Aby kontynuować wdrożenie:                          ║
║  1. Wykonaj: git add . && git commit -m "..."        ║
║  2. Wróć do rnr i ponów próbę                        ║
╚══════════════════════════════════════════════════════╝
```

`rnr` **nigdy nie wdroży automatycznie** niezacommitowanego kodu. To celowa decyzja projektowa — każda zmiana musi być świadomie zatwierdzona przez dewelopera.

---

## 6. Snapshoty i system Rollback

### Automatyczne snapshoty

Przed każdym wdrożeniem `rnr` tworzy **deterministyczny snapshot** w formie gałęzi Git:

```
rnr_backup_<środowisko>_<data>_<czas>
```

Przykłady:
- `rnr_backup_production_20260310_143022`
- `rnr_backup_staging_20260309_091500`

Informacja o każdym snapshocie jest zapisywana w pliku `.rnr/state.json`:

```json
{
  "history": [
    {
      "timestamp": "2026-03-10T14:30:22Z",
      "environment": "production",
      "commit_hash": "a1b2c3d4e5f6...",
      "snapshot_ref": "rnr_backup_production_20260310_143022",
      "status": "success",
      "message": "Deploy do Produkcji - [feat: nowy system płatności]"
    }
  ]
}
```

### Funkcja Rollback — „Szybki powrót"

W przypadku wykrycia błędów po wdrożeniu, w Dashboardzie `rnr` dostępna jest opcja **Rollback** (klawisz `R`):

```
╔══════════════════════════════════════════════════════╗
║  ↩️  Rollback — Szybki powrót                        ║
║                                                      ║
║  Ostatnie wdrożenia:                                 ║
║  → [10.03.2026 14:30] production — POWODZENIE ✓     ║
║  → [09.03.2026 09:15] staging — POWODZENIE ✓        ║
║  → [08.03.2026 16:45] production — BŁĄD ✗           ║
║                                                      ║
║  Wybierz punkt przywrócenia: ↑↓ Enter               ║
╚══════════════════════════════════════════════════════╝
```

#### ⚠️ Ważne ostrzeżenie dla baz danych

Rollback **cofa kod aplikacji**, ale **NIE cofa zmian w bazie danych**. Jeśli wdrożenie obejmowało migracje bazodanowe, `rnr` wyświetli wyraźne ostrzeżenie:

```
╔══════════════════════════════════════════════════════╗
║  🚨 OSTRZEŻENIE: Operacja nieodwracalna              ║
║                                                      ║
║  Rollback cofnie KOD APLIKACJI, ale NIE DANE         ║
║  w bazie danych. Jeśli wdrożenie zawierało           ║
║  migracje SQL, cofnięcie kodu może spowodować        ║
║  niespójności danych.                                ║
║                                                      ║
║  Przed kontynuacją skonsultuj się z zespołem.        ║
║  Czy na pewno chcesz kontynuować? [T/N]              ║
╚══════════════════════════════════════════════════════╝
```

---

## 7. Potoki wdrożeniowe (Pipeline)

Potok wdrożeniowy definiujesz w pliku `rnr.yaml`. Każdy etap (`stage`) jest wykonywany sekwencyjnie z pełną informacją wizualną w TUI:

```yaml
# Przykładowy potok dla aplikacji fullstack
stages:
  - name: "Przygotowanie"
    steps:
      - run: "npm run build"
        description: "Budowanie aplikacji frontendowej"

  - name: "Wdrożenie Frontend"
    provider: "netlify"
    steps:
      - deploy:
          environment: "production"

  - name: "Migracje Bazy Danych"
    provider: "supabase"
    steps:
      - database_migrate:
          environment: "production"

  - name: "Powiadomienie"
    provider: "github"
    steps:
      - release:
          environment: "production"
```

### Wizualizacja potoku w TUI

Podczas wykonywania potoku, `rnr` wyświetla w czasie rzeczywistym:

```
Pipeline: Wdrożenie na Produkcję
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓  Etap 1/4: Przygotowanie             [00:12]
⠸  Etap 2/4: Wdrożenie Frontend        [00:34]  ▓▓▓▓▓▓░░░░  62%
○  Etap 3/4: Migracje Bazy Danych      [oczekuje]
○  Etap 4/4: Powiadomienie GitHub      [oczekuje]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Logi na żywo:
  [netlify] ✓ Build cache warmed up
  [netlify] ✓ Uploading 247 files...
  [netlify] ⠸ Processing assets...
```

Spinner `⠸` oznacza aktywne działanie — `rnr` **nigdy nie zamarza wizualnie** bez powodu. Zawsze wiesz, co dzieje się w tle.

---

## 8. Pliki konfiguracyjne — rnr.yaml i rnr.conf.yaml

### rnr.yaml — Mózg operacyjny (bezpieczny do commitowania)

Ten plik zawiera definicję Twojego potoku wdrożeniowego. Jest **bezpieczny** — nie zawiera żadnych sekretów, można go commitować do repozytorium Git.

```yaml
# ╔══════════════════════════════════════════════════════════════════╗
# ║  rnr.yaml — Konfiguracja operacyjna narzędzia rnr (Runner)      ║
# ║                                                                  ║
# ║  Ten plik definiuje POTOKI WDROŻENIOWE dla Twojego projektu.     ║
# ║  Jest bezpieczny do commitowania w repozytorium Git.             ║
# ║                                                                  ║
# ║  Zawiera:                                                        ║
# ║  - Definicję etapów wdrożenia (stages)                           ║
# ║  - Wybór dostawców (netlify, supabase, github, shell)            ║
# ║  - Logikę orchestracji (kolejność kroków)                        ║
# ║                                                                  ║
# ║  NIE umieszczaj tutaj tokenów API ani haseł!                     ║
# ║  Do sekretów służy plik rnr.conf.yaml (ignorowany przez Git).    ║
# ╚══════════════════════════════════════════════════════════════════╝

project:
  name: "moja-aplikacja"
  repo: "https://github.com/organizacja/moja-aplikacja"

stages:
  - name: "deploy-frontend"
    provider: "netlify"
  - name: "deploy-database"
    provider: "supabase"
```

### rnr.conf.yaml — Sejf dewelopera (NIGDY nie commituj!)

Ten plik zawiera **wrażliwe dane** — tokeny API, hasła, klucze dostępu. `rnr` automatycznie dodaje go do `.gitignore`, więc nigdy przypadkowo nie trafi do publicznego repozytorium.

```yaml
# ╔══════════════════════════════════════════════════════════════════╗
# ║  rnr.conf.yaml — Sejf dewelopera (PRYWATNY!)                    ║
# ║                                                                  ║
# ║  UWAGA: Ten plik jest automatycznie dodany do .gitignore         ║
# ║  przez narzędzie rnr. NIGDY nie commituj tego pliku!             ║
# ║                                                                  ║
# ║  Zawiera:                                                        ║
# ║  - Tokeny API (Netlify, Supabase, GitHub)                        ║
# ║  - Klucze serwisowe baz danych                                   ║
# ║  - Zmienne środowiskowe dla każdego środowiska                   ║
# ║                                                                  ║
# ║  Jeśli przypadkowo ujawnisz zawartość tego pliku,                ║
# ║  natychmiast unieważnij wszystkie tokeny w odpowiednich          ║
# ║  serwisach (Netlify, Supabase, GitHub Settings).                 ║
# ╚══════════════════════════════════════════════════════════════════╝

environments:
  - name: "production"
    deploy:
      netlify_auth_token: "nfp_twoj_token_tutaj"
      netlify_site_id: "uuid-twojej-strony"
      netlify_prod: true
    database:
      supabase_project_ref: "twoj-projekt-ref"
      supabase_service_role_key: "eyJhbGciOiJIUzI1NiIs..."
```

### Dlaczego dwa pliki zamiast jednego?

To celowy podział bezpieczeństwa:

| Cecha | `rnr.yaml` | `rnr.conf.yaml` |
|-------|-----------|-----------------|
| Commitowanie do Git | ✅ Bezpieczne | ❌ Zabronione |
| Zawartość | Logika potoku | Sekrety i tokeny |
| Udostępnianie w zespole | ✅ Tak | ❌ Nigdy |
| Generowany przez Wizard | ✅ Tak | ✅ Tak |

---

## 9. Integracje z zewnętrznymi dostawcami

`rnr` nie wykonuje deploymentu samodzielnie — jest **dyrygentem orkiestry**, która zarządza wyspecjalizowanymi narzędziami zewnętrznymi.

### Netlify — Wdrożenia frontendowe

`rnr` wywołuje Netlify CLI (`netlify deploy`) z odpowiednimi flagami zdefiniowanymi w konfiguracji:

```
Provider: Netlify
→ netlify deploy --prod --build-dir dist
→ NETLIFY_AUTH_TOKEN=*** (zamaskowany)
→ NETLIFY_SITE_ID=***    (zamaskowany)
```

Wymagania: Netlify CLI musi być zainstalowane (`npm install -g netlify-cli`)

### Supabase — Migracje bazy danych

`rnr` zarządza migracjami przez Supabase CLI:

```
Provider: Supabase
→ supabase migration up
→ SUPABASE_ACCESS_TOKEN=***  (zamaskowany)
→ SUPABASE_PROJECT_REF=***   (zamaskowany)
```

Wymagania: Supabase CLI musi być zainstalowane (`npm install -g supabase`)

### GitHub — Releases i wiadomości commitów

`rnr` automatycznie tworzy **oznaczone wydania** na GitHubie z czytelną nazwą:

```
Release: "Deploy do Produkcji - [feat: nowy system płatności] (2026-03-10 14:30)"
```

### Własne skrypty (Shell Provider)

Możesz definiować dowolne polecenia shellowe w konfiguracji:

```yaml
deploy:
  deploy_cmd: "bash ./scripts/custom-deploy.sh"
```

---

## 10. Migracje bazy danych Supabase

Zarządzanie bazą danych to najbardziej **krytyczna** część procesu wdrożeniowego. `rnr` traktuje migracje ze szczególną ostrożnością.

### Zasada „Roll-forward"

`rnr` stosuje wyłącznie **zmiany addytywne** (roll-forward) przy migracjach:

- ✅ Dodawanie nowych kolumn, tabel, indeksów
- ✅ Rozszerzanie typów danych
- ❌ Cofanie usuniętych kolumn z danymi (nieodwracalne!)
- ❌ Zmiana typów kolumn z utratą danych

Jeśli Twój skrypt migracji zawiera operacje potencjalnie destrukcyjne (np. `DROP TABLE`, `DELETE FROM`), `rnr` wyświetli ostrzeżenie i wymaga potwierdzenia.

### Komenda `rnr promote`

Dedykowana komenda do przepychania migracji ze środowiska staging do produkcji:

```bash
rnr promote --from staging --to production
```

Proces `promote`:
1. Odczytuje skrypty migracji z katalogu `supabase/migrations/`
2. Sprawdza, które migracje zostały już zastosowane w produkcji
3. Wyświetla listę **nowych, nieaplikowanych migracji** do zatwierdzenia
4. Po zatwierdzeniu przez użytkownika aplikuje je sekwencyjnie
5. Zapisuje stan w `.rnr/state.json`

### Dwie instancje Supabase

`rnr` obsługuje oddzielne projekty Supabase dla każdego środowiska:

```yaml
# rnr.conf.yaml
environments:
  - name: "staging"
    database:
      supabase_project_ref: "projekt-staging-ref"
      supabase_service_role_key: "klucz-staging..."

  - name: "production"
    database:
      supabase_project_ref: "projekt-prod-ref"
      supabase_service_role_key: "klucz-produkcji..."
```

---

## 11. Maskowanie sekretów — bezpieczeństwo tokenów

`rnr` parsuje **cały output** wszystkich zewnętrznych narzędzi (Netlify CLI, Supabase CLI, własne skrypty) w czasie rzeczywistym.

Jeśli w strumieniu wyjściowym pojawi się sekwencja znaków pasująca do któregokolwiek sekretu z `rnr.conf.yaml`, zostanie ona **automatycznie zastąpiona** łańcuchem `***`:

```
# Bez maskowania (niebezpieczne):
Netlify deploy: token=nfp_abc123xyz456secret

# Z maskowaniem rnr (bezpieczne):
Netlify deploy: token=***
```

Maskowanie obejmuje:
- Wszystkie tokeny API z `rnr.conf.yaml`
- Klucze serwisowe baz danych
- Hasła i dane uwierzytelniające

Logi zapisywane do plików w `.rnr/logs/` **również są maskowane** — nikt, kto uzyska dostęp do plików logów, nie zobaczy prawdziwych tokenów.

---

## 12. Struktura katalogów projektu

Po instalacji i konfiguracji Twój projekt będzie wyglądał następująco:

```
mój-projekt/
├── .rnr/                      # Ukryty katalog rnr (automatycznie tworzony)
│   ├── rnr                    # Plik wykonywalny narzędzia
│   ├── state.json             # Historia wdrożeń i snapshoty
│   ├── logs/                  # Dzienniki wdrożeń (z datą w nazwie)
│   │   ├── production_20260310-143022.log
│   │   ├── staging_20260309-091500.log
│   │   └── ...
│   └── snapshots/             # Informacje o snapshotach rollback
├── src/                       # Kod źródłowy Twojej aplikacji
├── rnr.yaml                   # Konfiguracja potoku (bezpieczna, commituj!)
├── rnr.conf.yaml              # Sekrety i tokeny (NIGDY nie commituj!)
├── .gitignore                 # Automatycznie zawiera wpis dla rnr.conf.yaml
└── ...
```

### Pliki logów

Każde wdrożenie tworzy dedykowany plik logu z pełną datą i godziną:

```
.rnr/logs/production_20260310-143022.log
          ^^^^^^^^^^ ^^^^^^^^^^^^^^^
          środowisko data i czas wdrożenia
```

Logi zawierają:
- Pełny output wszystkich wywołanych narzędzi zewnętrznych
- Informacje o każdym kroku potoku
- Błędy i ostrzeżenia
- Wszystko z zamaskowanymi sekretami

---

## 13. Nazewnictwo gałęzi Git i dzienniki wdrożeń

`rnr` nadaje **autorski, unikalny styl** nazewnictwu gałęzi zapasowych w Git. Każda gałąź snapshot nosi starannie skomponowaną nazwę:

```
rnr_backup_<środowisko>_<YYYYMMDD><HHMMSS>
```

Przykłady:
```
rnr_backup_production_20260310143022
rnr_backup_staging_20260309091500
rnr_backup_production_20260308175932
```

### Dlaczego to jest ważne?

Dzięki temu schematowi nazewnictwa:

1. **Porządek w historii Git** — gałęzie `rnr_backup_*` są zawsze łatwo identyfikowalne w grafie commitów
2. **Deterministyczność** — z nazwy gałęzi natychmiast wiesz, kiedy i dla którego środowiska został utworzony snapshot
3. **Automatyczna archiwizacja** — każdy deploy zostawia za sobą wyraźny ślad w Git, który można przeglądać w dowolnym narzędziu (GitKraken, Sourcetree, `git log`)
4. **Szybki rollback** — jeśli coś pójdzie nie tak, możesz bez `rnr` wykonać: `git checkout rnr_backup_production_20260310143022`

### Wiadomości commitów (normalizacja)

Przy tworzeniu GitHub Release, `rnr` generuje znormalizowane wiadomości w formacie:

```
Deploy do Produkcji - [feat: nowy panel administracyjny] (2026-03-10 14:30)
Deploy do Staging - [fix: błąd w formularzu logowania] (2026-03-09 09:15)
```

Format: `Deploy do <Środowisko> - [<treść_commita>] (<data_i_czas>)`

---

## 14. Komendy CLI

```bash
# Uruchomienie głównego interfejsu TUI (Dashboard)
rnr

# Inicjalizacja nowego projektu (Setup Wizard)
rnr init

# Wdrożenie na konkretne środowisko
rnr deploy --env production
rnr deploy --env staging

# Przepchnięcie migracji bazy danych między środowiskami
rnr promote --from staging --to production

# Rollback do poprzedniego wdrożenia
rnr rollback --env production

# Wyświetlenie historii wdrożeń
rnr history

# Wyświetlenie logów ostatniego wdrożenia
rnr logs --env production

# Sprawdzenie wersji narzędzia
rnr --version

# Wyświetlenie pomocy
rnr --help
```

---

## 15. FAQ — Często zadawane pytania

**Q: Co zrobić, jeśli wdrożenie się nie powiedzie w połowie?**  
A: Uruchom `rnr rollback --env <środowisko>` lub wybierz opcję "Rollback" w Dashboard (`R`). `rnr` przywróci kod do stanu sprzed wdrożenia.

**Q: Czy muszę znać Git, żeby używać rnr?**  
A: Nie! `rnr` zarządza Gitem za Ciebie. Jedyne, co musisz wiedzieć, to że przed wdrożeniem musisz zacommitować swoje zmiany (`git add . && git commit -m "opis zmian"`).

**Q: Zgubił mi się token GitHub. Co teraz?**  
A: Skontaktuj się z administratorem systemu. Możesz też wygenerować nowy token w ustawieniach GitHub (Settings → Developer Settings → Personal Access Tokens) i zaktualizować go w `rnr.conf.yaml`.

**Q: Czy mogę użyć rnr bez Netlify lub Supabase?**  
A: Tak! Możesz skonfigurować własne polecenia wdrożeniowe za pomocą `shell` provider. W `rnr.conf.yaml` zdefiniuj `deploy_cmd` z dowolnym poleceniem bash.

**Q: Jak dodać nowego dewelopera do zespołu?**  
A: Niech nowy deweloper uruchomi skrypt instalacyjny z tokenem GitHub, a następnie **ręcznie skopiuj mu zawartość `rnr.conf.yaml`** (przez bezpieczny kanał — nigdy przez Git!). Po skopiowaniu pliku wystarczy uruchomić `rnr`.

**Q: Co to znaczy, że repozytorium jest „brudne"?**  
A: Oznacza to, że masz zmiany w plikach, które nie zostały jeszcze zacommitowane do Gita. `rnr` to wykryje i poprosi Cię o zacommitowanie zmian przed wdrożeniem.

**Q: Gdzie mogę zobaczyć co poszło nie tak przy ostatnim wdrożeniu?**  
A: Pliki logów znajdziesz w `.rnr/logs/`. Możesz też użyć komendy `rnr logs --env production`, która wyświetli ostatni log w Twoim terminalu.

---

## Podziękowania

`rnr` został stworzony z myślą o ludziach — nie tylko o inżynierach. Wierzymy, że narzędzia DevOps powinny być tak przyjazne i czytelne jak najlepsze aplikacje mobilne. Nikt nie powinien bać się wdrożenia.

Zbudowany z ❤️ przy użyciu:
- [Go](https://golang.org/) — szybki, bezpieczny język kompilowany
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — framework TUI z architekturą Elm
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — piękne style dla terminala
- [Bubbles](https://github.com/charmbracelet/bubbles) — gotowe komponenty TUI
- [Cobra](https://github.com/spf13/cobra) — framework CLI
- [Viper](https://github.com/spf13/viper) — zarządzanie konfiguracją

---

*Dokumentacja wygenerowana automatycznie przez rnr. Wersja dokumentacji: 1.0.0*
