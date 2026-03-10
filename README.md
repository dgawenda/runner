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
15. [Zarządzanie środowiskami](#15-zarządzanie-środowiskami)
16. [Przeglądarka logów](#16-przeglądarka-logów)
17. [FAQ — Często zadawane pytania](#17-faq--często-zadawane-pytania)

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

> ⚡ **Najprostsza możliwa instalacja z publicznego GitHuba** — jeden wiersz w terminalu.

Repozytorium `dgawenda/runner` jest **publiczne**, więc w typowym scenariuszu wystarczy:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/dgawenda/runner/master/install.sh)
```

To pobierze skrypt `install.sh` z gałęzi `master` i zainstaluje binarkę `rnr` do katalogu `.rnr/` w Twoim projekcie (lub do katalogu wskazanego przez `--dir`).

> 💡 Jeśli masz bardzo restrykcyjne limity API GitHub albo chcesz instalować z prywatnego forka, możesz przekazać token:
> ```bash
> GITHUB_TOKEN="twój_token" bash <(curl -fsSL https://raw.githubusercontent.com/dgawenda/runner/master/install.sh) --token "$GITHUB_TOKEN"
> ```

### Co robi skrypt instalacyjny?

Skrypt `install.sh` wykonuje następujące kroki automatycznie:

1. **Wykrywa Twój system operacyjny** (Linux, macOS, Windows) i architekturę procesora (amd64, arm64)
2. **Pobiera najnowszą stabilną wersję** binarną `rnr` z GitHub Releases (`dgawenda/runner`) – jeśli podasz token, użyje autoryzowanego zapytania API, inaczej korzysta z publicznego dostępu
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
rnr version
```

### Szybki start z własnym projektem (dowolne „demo”)

Po zainstalowaniu binarki możesz użyć `rnr` w **dowolnym repozytorium**, niezależnie od tego, jak nazywa się Twój projekt/demo:

```bash
cd /ścieżka/do/twojego/projektu   # np. ~/projects/moje-demo
rnr init                          # uruchomi Setup Wizard i wygeneruje rnr.yaml + rnr.conf.yaml
rnr                                # otworzy Dashboard dla tego projektu
```

- `rnr init` zawsze działa „na miejscu” — dostosuje się do aktualnego katalogu (Twojego demo).
- `rnr` można wywoływać jako:
  - globalne polecenie w PATH (jeśli użyłeś `install.sh` jako instalatora systemowego), albo
  - lokalną binarkę z repo `runner`, np. `./.rnr/rnr` uruchomioną z Twojego projektu:

```bash
cd /ścieżka/do/twojego/projektu
/home/alti/neution/runner/.rnr/rnr --dir .
```

README nie zakłada żadnego konkretnego folderu demo — wystarczy, że wejdziesz do swojego projektu i użyjesz powyższych komend.

---

## 3. Pierwsze uruchomienie — Setup Wizard

`rnr` automatycznie wykrywa stan projektu i dopasowuje działanie:

### Scenariusz A: Świeży projekt (brak plików konfiguracyjnych)

Uruchomienie `rnr` w pustym repozytorium uruchamia **pełny Setup Wizard**. Kreator zapyta Cię o:

- **Nazwę projektu** i URL repozytorium GitHub
- **Typ projektu** — frontend (bez bazy) lub fullstack
- **Wybór dostawcy wdrożenia** (Netlify, Vercel, SSH, Docker, własne skrypty)
- **Konfigurację bazy danych** — Supabase, Prisma, PostgreSQL lub „Brak bazy" (frontend-only)
- **Dane autoryzacyjne** — tokeny API wpisywane w bezpiecznych, maskowanych polach (`●●●●●`)

Po zakończeniu kreatora `rnr` automatycznie:
- Wygeneruje `rnr.yaml` **dopasowany do Twojego projektu** (bez etapu `migrate` jeśli wybrano „Brak bazy")
- Wygeneruje `rnr.conf.yaml` z podanymi credentials
- Doda `rnr.conf.yaml` do `.gitignore`
- Uruchomi główny Dashboard

#### Projekt frontendowy (bez bazy danych)

Jeśli w kreatorze wybierzesz **„Brak bazy"** jako dostawcę bazy danych, `rnr` wygeneruje uproszczony pipeline bez etapu migracji:

```yaml
# rnr.yaml — wygenerowany dla projektu frontendowego
stages:
  - name: install
  - name: lint
  - name: typecheck
  - name: build
    artifacts: dist/
  - name: deploy         # ← Netlify / Vercel / SSH
    type: deploy
  - name: health
    type: health
```

#### Automatyczne tworzenie projektu Netlify

W wizardzie, po wyborze Netlify jako dostawcy, możesz wybrać:

| Opcja | Kiedy używać |
|-------|-------------|
| **🔗 Mam już Site ID** | Projekt Netlify już istnieje — wklej ID |
| **✨ Utwórz nowy projekt** | rnr automatycznie wywoła `netlify sites:create` |

Jeśli wybierzesz „Utwórz nowy projekt", `rnr` podczas pierwszego deployu automatycznie stworzy projekt na Netlify i wypisze jego Site ID — zapisz go w `rnr.conf.yaml → netlify_site_id` na przyszłość.

---

### Scenariusz B: Sklonowany projekt (rnr.yaml w repo, brak rnr.conf.yaml)

Gdy `rnr.yaml` jest commitowany w repozytorium (tak jak powinien być), ale brakuje `rnr.conf.yaml` (jest gitignored), `rnr` wykrywa ten stan i:

1. Wyświetla **specjalny komunikat** w wizardzie: *"Wykryłem plik rnr.yaml — uzupełnij tylko swoje credentials"*
2. Prosi tylko o tokeny i credentials (nie pyta o strukturę projektu, która jest w `rnr.yaml`)
3. Generuje TYLKO `rnr.conf.yaml` — nie modyfikuje `rnr.yaml`

```bash
# Typowy workflow dla nowego dewelopera w projekcie:
git clone https://github.com/moja-firma/projekt.git
cd projekt
rnr            # ← wykryje brak rnr.conf.yaml i przeprowadzi przez credentials wizard
```

> 💡 **Dla administratora projektu:** umieść `rnr.yaml` w repozytorium. Każdy deweloper przy pierwszym uruchomieniu `rnr` zostanie przeprowadzony przez kreator credentials — bez ponownego definiowania całej struktury potoku.

---

### Scenariusz C: rnr.conf.yaml istnieje, brak rnr.yaml (np. po utracie pliku)

`rnr` odczytuje `rnr.conf.yaml`, wykrywa typ projektu (sprawdza czy są providery bazy danych) i **automatycznie regeneruje `rnr.yaml`** bez potrzeby uruchamiania wizarda. Następnie otwiera Dashboard.

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

### Podział odpowiedzialności

| Cecha | `rnr.yaml` | `rnr.conf.yaml` |
|-------|-----------|-----------------|
| Commitowanie do Git | ✅ **Tak** | ❌ Nigdy |
| Zawartość | Projekt + środowiska + pipeline | Wyłącznie tokeny i hasła |
| Udostępnianie w zespole | ✅ Każdy widzi | ❌ Prywatny per developer |
| Generowany przez Wizard | ✅ Tak | ✅ Tak |
| Ochrona `.gitignore` | Nie potrzebna | ✅ Automatyczna |

---

### rnr.yaml — Projekt, środowiska i pipeline (bezpieczny do commitowania)

Plik definiuje **całą konfigurację projektu bez żadnych sekretów**. Commituj go do repozytorium — dzięki temu każdy developer w zespole widzi tę samą strukturę wdrożeń.

```yaml
# ─── Projekt ─────────────────────────────────────────────────────────────────
project:
  name: "moja-aplikacja"
  version: "1.0.0"
  repo: "moja-firma/moja-aplikacja"

# ─── Środowiska (BEZ tokenów!) ────────────────────────────────────────────────
# Każde środowisko definiuje: gałąź, URL, dostawców i zmienne środowiskowe.
# Tokeny i hasła → WYŁĄCZNIE rnr.conf.yaml

environments:

  production:
    branch: "master"
    url: "https://moja-aplikacja.com"
    protected: true      # ⚠️ wymaga potwierdzenia przed deploym

    deploy:
      provider: "netlify"
      netlify_prod: true # true = --prod (produkcyjny URL Netlify)

    database:
      provider: "supabase"

    env:
      NODE_ENV: "production"

  staging:
    branch: "develop"
    url: ""
    protected: false

    deploy:
      provider: "netlify"
      netlify_prod: false

    database:
      provider: "supabase"

    env:
      NODE_ENV: "staging"

# ─── Etapy potoku ─────────────────────────────────────────────────────────────
stages:

  - name: install
    run: npm ci

  - name: lint
    run: npm run lint
    allow_failure: true

  - name: build
    run: npm run build
    artifacts: dist/

  - name: migrate          # pomiń jeśli nie używasz bazy danych
    type: database
    only: [production, staging]

  - name: deploy
    type: deploy

  - name: health
    type: health
    allow_failure: true
```

> 💡 **Wskazówka dla projektu frontendowego (bez bazy):** Usuń lub zakomentuj etap `migrate` i ustaw `database.provider: "none"` w każdym środowisku. Wizard zrobi to automatycznie jeśli wybierzesz „Brak bazy".

---

### rnr.conf.yaml — Sejf Sekretów (NIE COMMITOWAĆ!)

Plik zawiera **wyłącznie wrażliwe dane** — tokeny API, hasła, klucze bazy danych. `rnr` automatycznie dodaje go do `.gitignore`. **Każdy developer trzyma własną kopię lokalnie** z własnymi tokenami.

```yaml
# Opcjonalne nadpisanie autora wdrożenia
project:
  actor: ""           # puste = używa git config user.name
  actor_email: ""

# Webhooki (np. Slack)
notifications:
  slack_webhook: ""

# ─── Sekrety per środowisko ────────────────────────────────────────────────
# Klucze MUSZĄ odpowiadać nazwom środowisk z rnr.yaml → environments.

environments:

  production:
    deploy:
      netlify_auth_token: "nfp_TWOJ_TOKEN"   # Netlify → User Settings → PAT
      netlify_site_id: "uuid-twojej-strony"  # Netlify → Site → General → Site ID

    database:
      supabase_project_ref: "abcdefghijklmn" # Supabase → Project Settings → General
      supabase_db_url: "postgresql://postgres:[HASLO]@db.xxx.supabase.co:5432/postgres"
      supabase_anon_key: "eyJhbGciOiJIUzI1..."
      supabase_service_role_key: "eyJhbGciOiJIUzI1..."

  staging:
    deploy:
      netlify_auth_token: "nfp_TWOJ_TOKEN"   # Może być ten sam token
      netlify_site_id: "uuid-INNEJ-strony"   # ALE inny Site ID!

    database:
      supabase_project_ref: "inny-ref-staging"   # INNY projekt Supabase!
      supabase_db_url: "postgresql://..."
      supabase_anon_key: "eyJhbGciOiJIUzI1..."
      supabase_service_role_key: "eyJhbGciOiJIUzI1..."
```

### Przepływ ładowania konfiguracji

Przy starcie `rnr` scala oba pliki w pamięci:

```
rnr.yaml                    rnr.conf.yaml
┌─────────────────┐         ┌─────────────────────┐
│ environments:   │         │ environments:        │
│   production:   │         │   production:        │
│     branch: ...│  merge  │     deploy:          │
│     deploy:     │ ──────► │       netlify_token  │
│       provider  │         │     database:        │
│     database:   │         │       supabase_url   │
│       provider  │         └─────────────────────┘
└─────────────────┘
         ↓
   Merged Environment (używany przez providers, TUI)
```

Tokeny nigdy nie opuszczają pamięci programu w postaci jawnego tekstu — są maskowane w logach i `STDOUT`.

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

#### Automatyczne tworzenie projektu Netlify (gdy jeszcze go nie ma)

Jeśli w Setup Wizard wybierzesz Netlify jako dostawcę deployu, pojawi się dodatkowy krok:

- **„Projekt Netlify”**:
  - **Mam już Site ID** — wklejasz istniejące `netlify_site_id` z panelu Netlify,
  - **Utwórz nowy projekt** — `rnr` sam wywoła `netlify sites:create` i założy nowy projekt.

Wygenerowany fragment konfiguracji po wyborze "Utwórz nowy projekt":

**rnr.yaml** (ustawienia bez sekretów):
```yaml
environments:
  production:
    deploy:
      provider: "netlify"
      netlify_prod: true    # produkcyjny URL
```

**rnr.conf.yaml** (tylko tokeny):
```yaml
environments:
  production:
    deploy:
      netlify_auth_token: "nfp_twój_token"
      netlify_site_id: ""               # zostanie uzupełnione po pierwszym deployu
      netlify_create_new: true          # rnr sam wywoła netlify sites:create
```

Przy pierwszym `rnr deploy`:

1. `rnr` wywoła `netlify sites:create --json`,
2. wyłuska wygenerowany **Site ID**, pokaże go w TUI i logach (zamaskowany w tokenach),
3. użyje go do `netlify deploy`,
4. wyświetli komunikat, abyś **zapisał Site ID** w `rnr.conf.yaml` (`netlify_site_id`), jeśli chcesz mieć go na stałe.

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
rnr init --force     # nadpisz istniejącą konfigurację

# Wdrożenie na konkretne środowisko (otwiera TUI)
rnr deploy production
rnr deploy staging

# Rollback do poprzedniego wdrożenia (otwiera TUI)
rnr rollback production

# Przepchnięcie migracji bazy danych staging → production (otwiera TUI)
rnr promote

# Wyświetlenie logów ostatniego wdrożenia (terminal, bez TUI)
rnr logs
rnr logs production       # tylko logi dla środowiska production
rnr logs -n 100           # ostatnie 100 linii

# Sprawdzenie wersji narzędzia
rnr version

# Uruchomienie z konkretnym katalogiem projektu
rnr --dir /ścieżka/do/projektu

# Wyświetlenie pomocy
rnr --help
rnr env --help
```

### Skróty klawiaturowe w Dashboard TUI

| Klawisz | Akcja |
|---------|-------|
| `D` | Wdróż na wybrane środowisko |
| `R` | Rollback — przywróć poprzednią wersję |
| `P` | Promote — migracje DB staging → production |
| `L` | Otwórz przeglądarkę logów |
| `↑ / ↓` | Zmień wybrane środowisko |
| `Q` / `Ctrl+C` | Wyjdź z rnr |

---

## 15. Zarządzanie środowiskami

`rnr` obsługuje wiele środowisk w jednym projekcie. Środowiska definiujesz w `rnr.conf.yaml` w sekcji `environments`.

### Dodawanie nowego środowiska

```bash
# Dodaj środowisko lokalne
rnr env add local

# Dodaj środowisko dev
rnr env add dev

# Dodaj staging na bazie szablonu z production
rnr env add staging --from production

# Wylistuj wszystkie środowiska
rnr env list
```

Po uruchomieniu `rnr env add local` do `rnr.conf.yaml` zostanie dopisany blok:

```yaml
environments:

  # ── LOCAL ──────────────────────────────────────────────────────────────
  local:
    branch: "master"
    url: ""
    protected: false

    deploy:
      provider: "netlify"
      netlify_auth_token: ""   # ← uzupełnij swój token
      netlify_site_id: ""      # ← uzupełnij Site ID dla local
      netlify_prod: false

    database:
      provider: "none"

    env:
      NODE_ENV: "development"
```

Uzupełnij puste wartości (`netlify_auth_token`, `netlify_site_id`) i uruchom `rnr` — nowe środowisko pojawi się w Dashboard.

### Inicjalizacja środowisk production i dev od razu

Gdy tworzysz nowy projekt i chcesz od razu zainicjować wszystkie podstawowe środowiska:

```bash
rnr init                    # 1. Wizard skonfiguruje production
rnr env add staging         # 2. Dodaj staging
rnr env add local           # 3. Dodaj local (do testowania bez wdrożenia)
```

### Zalecana konfiguracja gałęzi

| Środowisko | Gałąź Git | Czy chronione? |
|------------|-----------|----------------|
| `production` | `master` | ✅ Tak |
| `staging` | `develop` | ❌ Nie |
| `local` / `dev` | `master` lub `feature/*` | ❌ Nie |
| `preview` | dowolna | ❌ Nie |

---

## 16. Przeglądarka logów

`rnr` zawiera wbudowaną interaktywną przeglądarkę logów dostępną z poziomu Dashboard.

### Uruchamianie

W głównym Dashboard wciśnij klawisz **`L`** — otworzy się przeglądarka logów.

```
📄 Logi wdrożeń
──────────────────────────────────────────────────────────────────
▶ production_20260310-143022.log    10.03.2026 14:30:22  12.4KB
  staging_20260309-091500.log       09.03.2026 09:15:00   8.7KB
  rollback_production_20260308.log  08.03.2026 17:59:32   3.2KB

──────────────────────────────────────────────────────────────────
  ENTER Otwórz   ↑↓ Nawigacja   R Odśwież   ESC Dashboard
```

### Podgląd zawartości logu

Po naciśnięciu `ENTER` na wybranym pliku:

```
📄 production_20260310-143022.log
──────────────────────────────────────────────────────────────────
    1 │ [2026-03-10 14:30:22] ▶ ETAP: install
    2 │ npm ci
    3 │ ✓ Zainstalowano 247 pakietów (12.4s)
    4 │ [2026-03-10 14:30:34] ▶ ETAP: build
    5 │ npm run build
    6 │ ✅ Build zakończony — dist/ (1.8MB)
    7 │ [2026-03-10 14:30:51] ▶ ETAP: deploy
    8 │ ✓ Netlify: wdrożono na https://mój-projekt.netlify.app

──────────────────────────────────────────────────────────────────
  ↑↓ Linia   PgUp/PgDn Strona   g/G Pocz./Koniec   ESC Lista
```

### Kolorowanie logów

| Kolor | Znaczenie |
|-------|-----------|
| 🟢 Zielony | Sukces (`✓`, `✅`, `SUCCESS`) |
| 🔴 Czerwony | Błąd (`✗`, `[ERROR]`) |
| 🟡 Żółty | Ostrzeżenie (`⚠`, `[WARN]`) |
| 🔵 Niebieski | Etap potoku (`▶`, `ETAP`) |

> 💡 Możesz też przeglądać logi bez TUI: `rnr logs production`

---

## 17. FAQ — Często zadawane pytania

**Q: Co zrobić, jeśli wdrożenie się nie powiedzie w połowie?**  
A: Uruchom `rnr rollback --env <środowisko>` lub wybierz opcję "Rollback" w Dashboard (`R`). `rnr` przywróci kod do stanu sprzed wdrożenia.

**Q: Czy muszę znać Git, żeby używać rnr?**  
A: Nie! `rnr` zarządza Gitem za Ciebie. Jedyne, co musisz wiedzieć, to że przed wdrożeniem musisz zacommitować swoje zmiany (`git add . && git commit -m "opis zmian"`).

**Q: Zgubił mi się token GitHub. Co teraz?**  
A: Skontaktuj się z administratorem systemu. Możesz też wygenerować nowy token w ustawieniach GitHub (Settings → Developer Settings → Personal Access Tokens) i zaktualizować go w `rnr.conf.yaml`.

**Q: Czy mogę użyć rnr bez Netlify lub Supabase?**  
A: Tak! Możesz skonfigurować własne polecenia wdrożeniowe za pomocą `shell` provider. W `rnr.conf.yaml` zdefiniuj `deploy_cmd` z dowolnym poleceniem bash.

**Q: Jak dodać nowego dewelopera do zespołu?**  
A: Nowy deweloper klonuje repozytorium i uruchamia `rnr`. Narzędzie automatycznie wykryje że `rnr.yaml` istnieje (w repo), ale brak `rnr.conf.yaml` (gitignored) i uruchomi kreator credentials. Każdy deweloper wpisuje swoje własne tokeny.

**Q: Mam tylko frontend — czy muszę konfigurować bazę danych?**  
A: Nie! W Setup Wizard wybierz **„Brak bazy"** jako dostawcę bazy danych. `rnr` wygeneruje uproszczony pipeline bez etapu `migrate`. Nie musisz wpisywać żadnych danych Supabase/PostgreSQL.

**Q: Jak dodać nowe środowisko, np. „local" lub „dev"?**  
A: Użyj komendy `rnr env add local` lub `rnr env add dev`. Polecenie dopisze nowe środowisko do `rnr.conf.yaml` na bazie szablonu. Następnie uzupełnij credentials dla nowego środowiska i uruchom `rnr` — pojawi się w Dashboard.

**Q: Co to znaczy, że repozytorium jest „brudne"?**  
A: Oznacza to, że masz zmiany w plikach, które nie zostały jeszcze zacommitowane do Gita. `rnr` to wykryje i poprosi Cię o zacommitowanie zmian przed wdrożeniem.

**Q: Gdzie mogę zobaczyć co poszło nie tak przy ostatnim wdrożeniu?**  
A: Wciśnij `L` w Dashboard — otworzy się interaktywna przeglądarka logów. Możesz też użyć terminala: `rnr logs production`.

**Q: Czy logi zawierają moje tokeny i hasła?**  
A: Nie. `rnr` automatycznie maskuje wszystkie sekrety z `rnr.conf.yaml` w logach. W pliku logu zamiast rzeczywistego tokenu znajdziesz `***`.

**Q: Nie mam site ID Netlify — jak stworzyć nowy projekt?**  
A: W Setup Wizard przy konfiguracji Netlify wybierz opcję **„✨ Utwórz nowy projekt"**. Podczas pierwszego deployu `rnr` automatycznie wywoła `netlify sites:create` i poda Ci nowe Site ID. Zapisz je w `rnr.conf.yaml → netlify_site_id` na przyszłe użycie.

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
