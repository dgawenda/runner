# rnr — Runner

> **Potężne narzędzie wdrożeniowe dla Twojego zespołu.**  
> Zapomnij o skomplikowanych platformach CI/CD. `rnr` to przyjazny interfejs terminalowy (TUI), który przeprowadzi Cię przez cały proces wdrożenia — krok po kroku, z animacjami, paskami postępu i pełnym bezpieczeństwem dla Twojego kodu.

---

## Spis treści

1. [Dla kogo jest rnr?](#1-dla-kogo-jest-rnr)
2. [Instalacja jedną komendą](#2-instalacja-jedną-komendą)
3. [Pierwsze uruchomienie — Setup Wizard](#3-pierwsze-uruchomienie--setup-wizard)
4. [Filozofia działania](#4-filozofia-działania)
5. [Architektura trybów — Dashboard, GitPanel, Apollo](#5-architektura-trybów--dashboard-gitpanel-apollo)
6. [GitPanel — Kontrola Repozytorium](#6-gitpanel--kontrola-repozytorium)
7. [Apollo — Panel Wdrożeń z Guardami](#7-apollo--panel-wdrożeń-z-guardami)
8. [System Strażników Wdrożenia (Deploy Guards)](#8-system-strażników-wdrożenia-deploy-guards)
9. [Snapshoty i system Rollback](#9-snapshoty-i-system-rollback)
10. [Potoki wdrożeniowe (Pipeline)](#10-potoki-wdrożeniowe-pipeline)
11. [Pliki konfiguracyjne — rnr.yaml i rnr.conf.yaml](#11-pliki-konfiguracyjne--rnryaml-i-rnrconfyaml)
12. [Integracje z zewnętrznymi dostawcami](#12-integracje-z-zewnętrznymi-dostawcami)
13. [Migracje bazy danych Supabase](#13-migracje-bazy-danych-supabase)
14. [Maskowanie sekretów — bezpieczeństwo tokenów](#14-maskowanie-sekretów--bezpieczeństwo-tokenów)
15. [Dzienniki wdrożeń (Deployment Logs)](#15-dzienniki-wdrożeń-deployment-logs)
16. [Struktura katalogów projektu](#16-struktura-katalogów-projektu)
17. [Komendy CLI](#17-komendy-cli)
18. [Zarządzanie środowiskami](#18-zarządzanie-środowiskami)
19. [Przeglądarka logów](#19-przeglądarka-logów)
20. [FAQ — Często zadawane pytania](#20-faq--często-zadawane-pytania)

---

## 1. Dla kogo jest rnr?

`rnr` powstał z jednej, bardzo konkretnej potrzeby: **wdrożenia kodu nie powinno bać się nikt w zespole**.

Wyobraź sobie sytuację — Twój współpracownik ukończył prace nad nową funkcją, ale jest piątek po południu, a specjalista od DevOps jest niedostępny. Normalnie oznaczałoby to czekanie do poniedziałku lub ryzykowne próby z ręcznym deploymentem przez SSH, `git push` do magicznych gałęzi lub klikanie w dziesiątkach zakładek różnych dashboardów chmurowych.

**Z `rnr` cały ten stres znika.**

`rnr` to narzędzie, które:

- 🛡️ **Chroni Twój kod** — system 6 strażników (guards) blokuje błędne wdrożenia zanim do nich dojdzie
- 🚀 **Apollo** — dedykowany tryb wdrożeń z pełną ochroną, tylko z gałęzi roboczych
- 🔧 **GitPanel** — wizualna kontrola repozytorium w stylu GitKraken, bezpośrednio w terminalu
- 🎯 **Prowadzi za rękę** — interaktywny Setup Wizard i przyjazny Dashboard z klawiaturą strzałkową
- ⚡ **Działa błyskawicznie** — napisany w języku Go, uruchamia się w ułamku sekundy
- 🔐 **Chroni Twoje sekrety** — tokeny API nigdy nie pojawiają się w logach ani na ekranie
- ↩️ **Pozwala cofnąć się** — wybierz konkretne wdrożenie do przywrócenia z interaktywnej historii

---

## 2. Instalacja jedną komendą

> ⚡ **Najprostsza możliwa instalacja z publicznego GitHuba** — jeden wiersz w terminalu.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/dgawenda/runner/master/install.sh)
```

To pobierze skrypt `install.sh` z gałęzi `master` i zainstaluje binarkę `rnr` do katalogu `~/.rnr/`.

> 💡 Z tokenem GitHub (dla prywatnych forków lub ominięcia limitów API):
> ```bash
> GITHUB_TOKEN="twój_token" bash <(curl -fsSL https://raw.githubusercontent.com/dgawenda/runner/master/install.sh)
> ```

### Co robi skrypt instalacyjny?

1. **Wykrywa system operacyjny** (Linux, macOS, Windows) i architekturę (amd64, arm64)
2. **Pobiera najnowszą wersję** binarną `rnr` z GitHub Releases
3. **Tworzy katalog** `~/.rnr` i instaluje plik wykonywalny
4. **Aktualizuje PATH** — dodaje `~/.rnr` do `.bashrc`, `.zshrc` lub `.profile`

Po zakończeniu:

```bash
source ~/.bashrc   # lub ~/.zshrc
rnr version        # → rnr v1.0.1
```

### Szybki start

```bash
cd /ścieżka/do/twojego/projektu
rnr init    # Setup Wizard — generuje rnr.yaml + rnr.conf.yaml
rnr         # otwiera Dashboard
```

---

## 3. Pierwsze uruchomienie — Setup Wizard

`rnr` automatycznie wykrywa stan projektu i dopasowuje działanie:

### Scenariusz A: Świeży projekt (brak plików konfiguracyjnych)

**Pełny Setup Wizard** zapyta o:

1. **Nazwę projektu** i URL repozytorium (pełny link do klonu: `https://github.com/...` lub `git@github.com:...`)
2. **Typ projektu** — frontend (bez bazy) lub fullstack
3. **Dostawcę wdrożenia** — Netlify, Vercel, SSH, Docker, własne skrypty
4. **Konfigurację bazy danych** — Supabase, Prisma, PostgreSQL, lub „Brak bazy" (frontend-only)
5. **Dane autoryzacyjne** — tokeny API w maskowanych polach (`●●●●●`)
6. **GitHub** — opcja podania URL zdalnego repozytorium lub użycia GitHub CLI (`gh`)

Po zakończeniu kreatora `rnr` automatycznie:
- Wygeneruje `rnr.yaml` dopasowany do projektu (bez etapu `migrate` jeśli brak bazy)
- Wygeneruje `rnr.conf.yaml` z credentials
- Doda `rnr.conf.yaml` do `.gitignore`
- Inicjalizuje repozytorium Git jeśli nie istnieje (`git init`)
- Tworzy gałęzie środowiskowe: `master` (production) i `develop` (development)
- Uruchomi Dashboard

#### Automatyczne tworzenie projektu Netlify

W wizardzie, po wyborze Netlify:

| Opcja | Kiedy używać |
|-------|-------------|
| **🔗 Mam już Site ID** | Projekt Netlify już istnieje |
| **✨ Utwórz nowy projekt** | `rnr` automatycznie wywoła `netlify sites:create` |

Jeśli wybierzesz „Utwórz nowy projekt", `rnr` podczas pierwszego deployu stworzy projekt na Netlify, zapisze Site ID w `rnr.conf.yaml` i pokaże go w logach inicjalizacji.

#### Gałęzie środowiskowe

`rnr` automatycznie tworzy gałęzie:

| Środowisko | Gałąź | Opis |
|------------|-------|------|
| `production` | `master` | Wdrożenia produkcyjne |
| `development` | `develop` | Wdrożenia deweloperskie |

---

### Scenariusz B: Sklonowany projekt (rnr.yaml w repo, brak rnr.conf.yaml)

`rnr` wykrywa ten stan i wyświetla kreator credentials — prosi tylko o tokeny bez ponownego definiowania struktury projektu.

```bash
git clone https://github.com/moja-firma/projekt.git
cd projekt
rnr   # ← kreator credentials
```

---

### Scenariusz C: rnr.conf.yaml istnieje, brak rnr.yaml

`rnr` regeneruje `rnr.yaml` na podstawie istniejącej konfiguracji i otwiera Dashboard.

---

## 4. Filozofia działania

`rnr` opiera się na trzech filarach:

### 🎭 Zero-Config — żadnego bólu konfiguracyjnego

Nowe narzędzie powinno działać od razu po instalacji. Setup Wizard prowadzi przez każdy krok z przyjaznym językiem. Nie musisz rozumieć, czym jest `CI/CD pipeline`.

### 🛡️ Safety-First — bezpieczeństwo na pierwszym miejscu

Każde wdrożenie w ReleasePanel (Apollo) jest poprzedzone **weryfikacją 6 strażników**. Jeśli cokolwiek jest nie tak, `rnr` zablokuje deployment i wyjaśni dokładnie co trzeba naprawić.

### 🔮 GitOps — Git jako źródło prawdy

Każda operacja wdrożeniowa jest utrwalana w historii Git. `rnr` tworzy gałęzie zapasowe (`rnr_backup_*`) przed każdym deploymentem.

---

## 5. Architektura trybów — Dashboard, GitPanel, ReleasePanel

`rnr` dzieli się na trzy podnarzędzia dostępne z głównego Dashboard:

```
┌─────────────────────────────────────────────────────────────┐
│  ⚡ rnr / projekt  [G GitPanel]  [A ReleasePanel]  02.01 14:30 │
├─────────────────────────────────────────────────────────────┤
│  Dashboard — centrum dowodzenia                             │
│  Wybierz tryb operacyjny:                                   │
│                                                              │
│  [G]  🔧 GitPanel     — operacje Git (commit, push, checkout) │
│  [A]  🚀 ReleasePanel — wdrożenia z pełną ochroną             │
│  [L]  Logi wdrożeń                                           │
└─────────────────────────────────────────────────────────────┘
```

### 🔧 GitPanel — do codziennego developmentu

Otwierasz klawiszem **`G`** z Dashboard. Pełna kontrola repozytorium bez opuszczania terminala. Zamykasz klawiszem **`Q`** lub **`ESC`**.

### 🚀 ReleasePanel (Apollo) — wyłącznie do wdrożeń

Otwierasz klawiszem **`A`** z Dashboard. Wymaga gałęzi roboczych. Chroniony przez 6 strażników. Zamykasz klawiszem **`Q`** lub **`ESC`**.

---

## 6. GitPanel — Kontrola Repozytorium

> Styl GitKraken, bezpośrednio w Twoim terminalu. Otwórz klawiszem `G`.

GitPanel ma cztery zakładki:

| Zakładka | Opis |
|----------|------|
| **[1] STATUS** | Lista zmienionych plików z checkboxami, podgląd diff, commit, push |
| **[2] GAŁĘZIE** | Lokalne gałęzie + `git checkout` wybranej (ENTER) |
| **[3] HISTORIA** | Tabela ostatnich 30 commitów (hash, autor, data, wiadomość) |
| **[4] GRAF ◈** | Wizualny graf commitów (styl GitKraken, kolorowe linie) |

### Skróty klawiaturowe — GitPanel

| Kontekst | Klawisz | Akcja |
|----------|---------|-------|
| Dowolna zakładka | `TAB` / `1–4` | Przełącz zakładkę |
| Dowolna zakładka | `Q` / `ESC` | Wróć do Dashboard |
| Listy | `↑ / ↓` lub `j / k` | Nawigacja po wierszach |
| **Status** — lista plików | `SPACJA` | Zaznacz/odznacz plik do commita (checkbox) |
| **Status** — lista plików | `a / A` | Zaznacz wszystkie / odznacz wszystkie |
| **Status** — lista plików | `d` / `ENTER` | Podgląd kolorowego `diff` zaznaczonego pliku |
| **Status** — lista plików | `i` | Wejście w edycję wiadomości commita |
| **Status** — lista plików | `m` / `M` | Wygeneruj automatyczny opis commita na podstawie zmian |
| **Status** — input commita | `ENTER` | `git add <wybrane> && git commit` |
| **Status** — input commita | `ESC` | Wyjście z edycji |
| **Status** | `p` / `P` | `git push origin <branch>` |
| **Status** — push conflict | `u` / `U` | `git pull --rebase && push` |
| **Status** — push conflict | `f` / `F` | `git push --force-with-lease` |
| **Gałęzie** | `ENTER` | `git checkout <gałąź>` |
| **Graf** | `↑ / ↓` | Przewijanie grafu |

### Wybór plików do commita

W zakładce **Status** każdy plik ma checkbox `[ ]` lub `[✓]`. Zaznacz konkretne pliki spacją lub użyj `a` dla wszystkich. Przy commicie `rnr` wykona `git add <tylko_zaznaczone>`.

### Auto commit message (GitPanel)

W zakładce **Status**, gdy repozytorium ma zmiany:
- naciśnij **`m`** aby wygenerować automatyczny opis commita na podstawie listy zmienionych plików i aktualnej daty, np.  
  `chore: update app/main.go, pkg/tui/model.go and 3 more files (2026-03-12 14:05)`
- opis pojawia się w polu commita, możesz go **edytować** przed naciśnięciem `ENTER`.

### Obsługa konfliktów push (non-fast-forward)

Jeśli `git push` zostanie odrzucony (remote jest do przodu), GitPanel wyświetli ekran wyboru:

```
⎇  Konflikt Push — remote jest do przodu

Opcje:
  [U]  git pull --rebase + push   ← zalecane
  [F]  git push --force-with-lease
  [ESC] Anuluj
```

### Wizualizacja grafu (styl Dracula)

```
● a1b2c3d  HEAD → master   feat: nowy system płatności  (Jan Kowalski, 2h temu)
│
● b2c3d4e              fix: poprawka formularza        (Anna Nowak, 1d temu)
│╲
│ ● c3d4e5f  origin/develop  chore: aktualizacja deps  (Jan Kowalski, 3d temu)
│╱
● d4e5f6g              init: projekt                   (Jan Kowalski, 7d temu)
```

Kolory: `●` fiolet = commit HEAD, `│╱╲` grafit = linie, `⎇` cyjan = gałąź, `🏷` róż = tag.

---

## 7. ReleasePanel (Apollo) — Panel Wdrożeń z Guardami

> Tryb wdrożeń z maksymalnym bezpieczeństwem. Otwórz klawiszem `A`.

```
🚀 ReleasePanel — Panel Wdrożeń  (Q = wróć do Dashboard)
⚡ Tryb Apollo/ReleasePanel: wdrożenia wyłącznie z gałęzi roboczych (master/develop) · wszystkie operacje są chronione strażnikami
──────────────────────────────────────────────────────────────
 1 PRZEGLĄD    2 HISTORIA
```

### Zakładki ReleasePanel

| Zakładka | Opis |
|----------|------|
| **[1] PRZEGLĄD** | Selektor środowisk + wyniki strażników + przyciski akcji |
| **[2] HISTORIA** | Historia wdrożeń ze statusami, autorami i haszami commitów |

### Skróty klawiaturowe — ReleasePanel

| Klawisz | Akcja |
|---------|-------|
| `1` / `2` | Przełącz zakładkę |
| `↑ / ↓` lub `j / k` | Zmień środowisko (Przegląd) / nawigacja historii |
| `D` | Wdróż na wybrane środowisko (tylko gdy guards OK) |
| `R` | Rollback — wybierz wdrożenie z listy |
| `P` | Promote DB (development → production) |
| `S` | Przełącz gałąź na wymaganą dla środowiska |
| `F` | Włącz/wyłącz tryb wymuszenia (pomija guard "nowe commity") |
| `Q` / `ESC` | Wróć do Dashboard |

### Automatyczne przełączanie gałęzi

Gdy naciskasz `D` lub `S`, Apollo sprawdza czy jesteś na właściwej gałęzi:

```
⎇  Przełączenie gałęzi

Apollo wymaga gałęzi 'master' dla wybranego środowiska.

Aktualna gałąź:  feature/new-payment
Docelowa gałąź:  master

Apollo wykona: git checkout master

Upewnij się że masz zatwierdzone wszystkie zmiany.

[ ENTER / Y = Przełącz ]   [ ESC / N = Anuluj ]
```

### Tryb wymuszenia (Force Redeploy)

Klawisz `F` włącza tryb `[FORCE]`, który ignoruje guard "Nowe commity". Przydatny gdy chcesz powtórzyć deploy bez nowych zmian (np. po zmianie konfiguracji infrastruktury).

---

## 8. System Strażników Wdrożenia (Deploy Guards)

Apollo przed każdym deployem weryfikuje 6 strażników:

| # | Strażnik | Poziom | Co sprawdza |
|---|---------|--------|-------------|
| 1 | **Gałąź robocza** | 🔴 Blokuje | Musi być `master` (prod) lub `develop` (dev) |
| 2 | **Stan Git** | 🔴 Blokuje | Brak detached HEAD, repo ma przynajmniej 1 commit |
| 3 | **Czystość repo** | 🔴 Blokuje | Brak niezatwierdzonych śledzonych plików |
| 4 | **Historia commitów** | 🔴 Blokuje | Repozytorium nie jest puste |
| 5 | **Nowe commity** | 🟡 Ostrzega | Są nowe commity od ostatniego deployu |
| 6 | **Konfiguracja** | 🔴 Blokuje | Dostawca wdrożenia ma kompletne credentials |

### Wizualizacja strażników w Apollo

```
🛡  Strażnicy wdrożenia:

  ✓  Gałąź robocza      Gałąź ⎇ master ✓
  ✓  Stan Git            Repozytorium zdrowe, HEAD: a1b2c3d
  ✓  Czystość repo       Brak niezatwierdzonych zmian ✓
  ✓  Historia commitów   Ostatni commit: a1b2c3d — feat: nowy panel
  ⚠  Nowe commity        Brak zmian od ostatniego deploy (11.03 14:30)
     ↳ Zatwierdź nowe zmiany lub użyj opcji wymuszenia (F — force redeploy)
  ✓  Konfiguracja        Dostawca: netlify ✓

  ⚠  1 strażnik ostrzega — możesz wdrożyć lub użyć F (force)
```

### Stany strażników

| Ikona | Znaczenie | Działanie |
|-------|-----------|-----------|
| `✓` zielony | Zaliczony | Brak |
| `⚠` żółty | Ostrzeżenie (`WARN`) | Deploy możliwy, ale zalecana uwaga |
| `✗` czerwony | Blokada (`BLOCK`) | Deploy niemożliwy — napraw problem |

---

## 9. Snapshoty i system Rollback

### Automatyczne snapshoty

Przed każdym wdrożeniem `rnr` tworzy deterministyczny snapshot w formie gałęzi Git:

```
rnr_backup_<środowisko>_<YYYYMMDD>_<HHMMSS>
```

Przykłady:
- `rnr_backup_production_20260310_143022`
- `rnr_backup_development_20260309_091500`

### Rollback z wyborem wdrożenia

W Apollo naciśnij **`R`** lub na Dashboard **`R`** — otwiera się interaktywna lista wyboru:

```
↩️  Rollback — wybierz wdrożenie
Środowisko: production

  St.  Data           Commit   Autor         Wiadomość
  ──────────────────────────────────────────────────────
▶ ✓    10.03 14:30:22  a1b2c3d  Jan Kowalski  feat: nowy panel
  ✓    09.03 09:15:00  b2c3d4e  Anna Nowak    fix: formularz
  ↩    08.03 17:59:32  ↩ cofn.  Jan Kowalski  Rollback do b2c3d4e

  ↑↓ / j k = nawigacja   ENTER / SPACJA = potwierdź   ESC / Q = anuluj
```

#### Pierwsze wdrożenie — rollback niemożliwy

Jeśli nie ma jeszcze żadnego wdrożenia, `rnr` wyświetli czytelny komunikat:

```
❌ Rollback niemożliwy — pierwsze wdrożenie

Środowisko 'production' nie ma jeszcze żadnych wdrożeń.
Rollback wymaga przynajmniej jednego wdrożenia zakończonego sukcesem.

Uruchom pierwsze wdrożenie klawiszem D.
```

#### ⚠️ Ważne ostrzeżenie dla baz danych

Rollback **cofa kod aplikacji**, ale **NIE cofa zmian w bazie danych**. Jeśli wdrożenie obejmowało migracje bazodanowe, `rnr` wyświetli stosowne ostrzeżenie.

---

## 10. Potoki wdrożeniowe (Pipeline)

Potok wdrożeniowy definiujesz w pliku `rnr.yaml`:

```yaml
# Przykładowy potok dla aplikacji fullstack
stages:

  - name: install
    run: npm ci
    description: "Instalacja zależności npm"

  - name: lint
    run: npm run lint
    allow_failure: true
    description: "Sprawdzanie stylu kodu"

  - name: build
    run: npm run build
    artifacts: dist/
    description: "Budowanie wersji produkcyjnej"

  - name: migrate
    type: database
    only: [production, development]
    description: "Migracje bazy danych Supabase"

  - name: deploy
    type: deploy
    description: "Wdrożenie na Netlify"

  - name: health
    type: health
    allow_failure: true
    description: "Sprawdzenie dostępności po wdrożeniu"
```

### Wizualizacja potoku w TUI

```
Pipeline: Wdrożenie na production
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓  [1/5] install      Instalacja zależności npm           [12s]
✓  [2/5] lint         Sprawdzanie stylu kodu              [3s]
⠸  [3/5] build        Budowanie wersji produkcyjnej       [34s]  ▓▓▓▓░░  62%
○  [4/5] migrate      Migracje bazy danych Supabase       [oczekuje]
○  [5/5] deploy       Wdrożenie na Netlify                [oczekuje]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Logi na żywo:
  [npm] ✓ Build cache warmed up
  [npm] ✓ Uploading 247 files...
```

### Opisy etapów w logach Netlify

Każde wdrożenie na Netlify zawiera szczegółowy opis widoczny w panelu Netlify:

```
[production] Jan Kowalski <jan@firma.pl> — feat: nowy system płatności
deploy: production • branch: master • commit: a1b2c3d • 10.03.2026 14:30:22
project: moja-aplikacja
```

---

## 11. Pliki konfiguracyjne — rnr.yaml i rnr.conf.yaml

### Podział odpowiedzialności

| Cecha | `rnr.yaml` | `rnr.conf.yaml` |
|-------|-----------|-----------------|
| Commitowanie do Git | ✅ **Tak** | ❌ Nigdy |
| Zawartość | Projekt + środowiska + pipeline | Wyłącznie tokeny i hasła |
| Udostępnianie w zespole | ✅ Każdy widzi | ❌ Prywatny per developer |
| Ochrona `.gitignore` | Nie potrzebna | ✅ Automatyczna |

---

### rnr.yaml — Projekt, środowiska i pipeline

```yaml
# ─── Projekt ──────────────────────────────────────────────────────────────
project:
  name: "moja-aplikacja"
  version: "1.0.0"
  repo: "https://github.com/moja-firma/moja-aplikacja.git"

# ─── Środowiska ───────────────────────────────────────────────────────────
environments:

  production:
    branch: "master"        # ← gałąź wymagana przez Apollo
    url: "https://moja-aplikacja.com"
    protected: true         # ⚠️ wymaga potwierdzenia przed deploym

    deploy:
      provider: "netlify"
      netlify_prod: true    # true = --prod (produkcyjny URL Netlify)

    database:
      provider: "supabase"

    env:
      NODE_ENV: "production"

  development:              # ← skrót: dev
    branch: "develop"       # ← gałąź wymagana przez Apollo
    url: ""
    protected: false

    deploy:
      provider: "netlify"
      netlify_prod: false

    database:
      provider: "supabase"

    env:
      NODE_ENV: "development"

# ─── Etapy potoku ──────────────────────────────────────────────────────────
stages:

  - name: install
    run: npm ci

  - name: build
    run: npm run build
    artifacts: dist/

  - name: migrate
    type: database
    only: [production, development]

  - name: deploy
    type: deploy

  - name: health
    type: health
    allow_failure: true
```

---

### rnr.conf.yaml — Sejf Sekretów (NIE COMMITOWAĆ!)

```yaml
project:
  actor: ""          # puste = git config user.name
  actor_email: ""

environments:

  production:
    deploy:
      netlify_auth_token: "nfp_TWOJ_TOKEN"
      netlify_site_id: "uuid-twojej-strony"  # lub zostaw puste + netlify_create_new: true

    database:
      supabase_project_ref: "abcdefghijklmn"
      supabase_db_url: "postgresql://postgres:[HASLO]@db.xxx.supabase.co:5432/postgres"
      supabase_anon_key: "eyJhbGciOiJIUzI1..."

  development:
    deploy:
      netlify_auth_token: "nfp_TWOJ_TOKEN"   # może być ten sam token
      netlify_site_id: "uuid-INNEJ-strony"   # inny Site ID!
      netlify_create_new: false

    database:
      supabase_project_ref: "inny-ref-dev"
      supabase_db_url: "postgresql://..."
      supabase_anon_key: "eyJhbGciOiJIUzI1..."
```

---

## 12. Integracje z zewnętrznymi dostawcami

### Netlify — Wdrożenia frontendowe

`rnr` wywołuje Netlify CLI z odpowiednimi flagami i szczegółowym opisem wdrożenia:

```
netlify deploy --prod \
  --dir dist \
  --message "Jan Kowalski <jan@firma.pl> — feat: nowy panel (production, master, a1b2c3d, 10.03.2026 14:30)"
```

Wymagania: `npm install -g netlify-cli`

#### Automatyczne tworzenie projektu i Site ID

Gdy `netlify_create_new: true` i `netlify_site_id` jest puste:
1. `rnr` wywołuje `netlify sites:create --json`
2. Zapisuje wygenerowany Site ID do `rnr.conf.yaml`
3. Loguje Site ID w pliku `.rnr/logs/init.log`
4. Kontynuuje deploy z nowym Site ID

### Supabase — Migracje bazy danych

```
supabase migration up
SUPABASE_ACCESS_TOKEN=***  (zamaskowany)
SUPABASE_PROJECT_REF=***   (zamaskowany)
```

Wymagania: `npm install -g supabase`

### GitHub CLI (`gh`)

Jeśli `gh` jest zainstalowane, Setup Wizard oferuje użycie go do zarządzania zdalnym repozytorium. `rnr` może sprawdzić dostępność `gh` i zaproponować instalację.

### Własne skrypty

```yaml
deploy:
  deploy_cmd: "bash ./scripts/custom-deploy.sh"
```

---

## 13. Migracje bazy danych Supabase

### Zasada „Roll-forward"

`rnr` stosuje wyłącznie **zmiany addytywne**:

- ✅ Dodawanie kolumn, tabel, indeksów
- ✅ Rozszerzanie typów danych
- ❌ Cofanie usuniętych kolumn z danymi
- ❌ Zmiana typów kolumn z utratą danych

### Komenda `promote`

Przepychanie migracji ze środowiska development do production:

```bash
# W TUI (klawisz P w Dashboard lub Apollo)
# lub z CLI:
rnr promote
```

Proces:
1. Odczytuje skrypty migracji z `supabase/migrations/`
2. Sprawdza, które migracje są już w production
3. Wyświetla listę nowych migracji do zatwierdzenia
4. Aplikuje sekwencyjnie po potwierdzeniu
5. Zapisuje stan w `.rnr/state.json`

---

## 14. Maskowanie sekretów — bezpieczeństwo tokenów

`rnr` parsuje cały output zewnętrznych narzędzi w czasie rzeczywistym:

```
# Bez maskowania (niebezpieczne):
Netlify deploy: token=nfp_abc123xyz456secret

# Z maskowaniem rnr (bezpieczne):
Netlify deploy: token=***
```

Maskowanie obejmuje tokeny API, klucze bazy danych, hasła i credentiale. Logi w `.rnr/logs/` są również maskowane.

---

## 15. Dzienniki wdrożeń (Deployment Logs)

### Szczegółowe logi w czasie rzeczywistym

Każde wdrożenie zapisuje kompletny dziennik do `.rnr/logs/`:

```
=== Wdrożenie: a1b2c3d4-e5f6-... ===
Projekt:     moja-aplikacja
Środowisko:  production
Gałąź:       master
Commit:      a1b2c3d — feat: nowy system płatności
Autor:       Jan Kowalski <jan@firma.pl>
Data:        2026-03-10 14:30:22 CET
--------------------------------------------------

[14:30:22] ▶ ETAP [1/5]: install — Instalacja zależności npm
[14:30:34] ✓ install zakończony [12s]

[14:30:34] ▶ ETAP [2/5]: build — Budowanie wersji produkcyjnej
  > npm run build
  ✓ Build completed in 34s — dist/ (1.8MB)
[14:31:08] ✓ build zakończony [34s]

[14:31:08] ▶ ETAP [3/5]: deploy — Wdrożenie na Netlify
  > netlify deploy --prod --dir dist --message "..."
  ✓ Site deployed: https://moja-aplikacja.netlify.app
[14:31:45] ✓ deploy zakończony [37s]

[14:31:45] ✅ WDROŻENIE ZAKOŃCZONE SUKCESEM [1m 23s]
Środowisko: production | Commit: a1b2c3d | ID: a1b2c3d4-e5f6
```

### Logi po awarii / crash recovery

Jeśli `rnr` napotka krytyczny błąd (panic), log zawiera:

```
[14:31:45] 💥 KRYTYCZNY BŁĄD — crash recovery
  panic: runtime error: ...
  goroutine 1 [running]:
  main.cmdRunPipeline(...)
    /path/to/model.go:1588
  [pełny stack trace]

  Wdrożenie oznaczone jako: FAILED
  Możliwy rollback do poprzedniej wersji: R w Dashboard
```

---

## 16. Struktura katalogów projektu

```
mój-projekt/
├── .rnr/                          # Ukryty katalog rnr
│   ├── snapshots/
│   │   └── state.json             # Historia wdrożeń i snapshoty Git
│   └── logs/                      # Dzienniki wdrożeń
│       ├── production_20260310-143022.log
│       ├── development_20260309-091500.log
│       ├── rollback_20260308-175932.log
│       └── init.log               # Log inicjalizacji projektu
├── src/                           # Kod źródłowy aplikacji
├── rnr.yaml                       # Konfiguracja potoku (commituj!)
├── rnr.conf.yaml                  # Sekrety i tokeny (NIGDY nie commituj!)
└── .gitignore                     # Automatycznie zawiera rnr.conf.yaml
```

### Gałęzie Git tworzone przez rnr

```
master                    ← production
develop                   ← development
rnr_backup_production_*   ← snapshoty przed wdrożeniem
rnr_backup_development_*  ← snapshoty przed wdrożeniem
```

---

## 17. Komendy CLI

```bash
# Główny interfejs TUI (Dashboard)
rnr

# Inicjalizacja projektu (Setup Wizard)
rnr init
rnr init --force     # nadpisz istniejącą konfigurację

# Wdrożenie (otwiera TUI)
rnr deploy production
rnr deploy development

# Rollback (otwiera TUI z ekranem wyboru)
rnr rollback production

# Promote migracji DB (otwiera TUI)
rnr promote

# Logi wdrożeń (bez TUI)
rnr logs
rnr logs production
rnr logs -n 100

# Wersja
rnr version        # → rnr v1.0.1

# Uruchomienie w konkretnym katalogu
rnr --dir /ścieżka/do/projektu
```

### Skróty klawiaturowe — Dashboard

| Klawisz | Akcja |
|---------|-------|
| `G` | 🔧 Otwórz GitPanel (normal dev) |
| `A` | 🚀 Otwórz Apollo (wdrożenia z guardami) |
| `D` | Szybki deploy na wybrane środowisko |
| `R` | Rollback — wybierz wdrożenie z historii |
| `P` | Promote DB (development → production) |
| `L` | Przeglądarka logów |
| `↑ / ↓` | Zmień wybrane środowisko |
| `Q` / `Ctrl+C` | Wyjdź z rnr |

### Skróty klawiaturowe — GitPanel

| Klawisz | Akcja |
|---------|-------|
| `1–4` / `TAB` | Zakładki (Status, Gałęzie, Historia, Graf) |
| `SPACJA` | Zaznacz/odznacz plik do commita |
| `a` / `A` | Zaznacz/odznacz wszystkie pliki |
| `i` | Wpisz wiadomość commita |
| `ENTER` (input) | Commit zaznaczonych plików |
| `p` / `P` | Push |
| `d` / `ENTER` (lista) | Podgląd diff pliku |
| `↑ / ↓` lub `j / k` | Nawigacja |
| `Q` / `ESC` | Wróć do Dashboard |

### Skróty klawiaturowe — Apollo

| Klawisz | Akcja |
|---------|-------|
| `1` / `2` | Zakładki (Przegląd, Historia) |
| `D` | Wdróż (gdy guards OK) |
| `R` | Rollback (wybór z listy) |
| `P` | Promote DB |
| `S` | Przełącz gałąź na wymaganą |
| `F` | Force redeploy (pomija guard nowych commitów) |
| `↑ / ↓` lub `j / k` | Środowisko / nawigacja historii |
| `Q` / `ESC` | Wróć do Dashboard |

---

## 18. Zarządzanie środowiskami

### Nazewnictwo środowisk

| Środowisko | Gałąź | Skrót | Netlify |
|------------|-------|-------|---------|
| `production` | `master` | prod | `projekt-prod.netlify.app` |
| `development` | `develop` | dev | `projekt-dev.netlify.app` |

### Dodawanie środowisk

```bash
rnr env add development     # branch: develop
rnr env add local           # branch: master, bez netlify
rnr env add preview         # branch: feature/*
rnr env list                # wylistuj wszystkie
```

Po dodaniu uzupełnij credentials w `rnr.conf.yaml` i uruchom `rnr`.

---

## 19. Przeglądarka logów

W Dashboard wciśnij **`L`** — otwiera się interaktywna przeglądarka.

```
📄 Logi wdrożeń
──────────────────────────────────────────────────────────────────
▶ production_20260310-143022.log    10.03.2026 14:30:22  12.4KB
  development_20260309-091500.log   09.03.2026 09:15:00   8.7KB
  rollback_production_20260308.log  08.03.2026 17:59:32   3.2KB

──────────────────────────────────────────────────────────────────
  ENTER Otwórz   ↑↓ Nawigacja   R Odśwież   ESC Dashboard
```

| Kolor | Znaczenie |
|-------|-----------|
| 🟢 Zielony | Sukces (`✓`, `✅`, `SUCCESS`) |
| 🔴 Czerwony | Błąd (`✗`, `[ERROR]`, `panic`) |
| 🟡 Żółty | Ostrzeżenie (`⚠`, `[WARN]`) |
| 🔵 Niebieski | Etap potoku (`▶`, `ETAP`) |

---

## 20. FAQ — Często zadawane pytania

**Q: Co zrobić, jeśli wdrożenie się nie powiedzie?**  
A: Apollo lub Dashboard → klawisz `R` → wybierz wdrożenie z listy → `ENTER`. `rnr` przywróci kod do wybranego stanu.

**Q: Dlaczego Apollo mówi "Gałąź nieprawidłowa"?**  
A: Apollo wymaga gałęzi `master` (production) lub `develop` (development). Naciśnij `S` w Apollo — automatycznie przełączy gałąź za Ciebie.

**Q: Czym różni się GitPanel od Apollo?**  
A: **GitPanel** (`G`) — do codziennej pracy: commity, push, checkout, podgląd historii. **Apollo** (`A`) — wyłącznie do wdrożeń, z pełną weryfikacją bezpieczeństwa i strażnikami.

**Q: Czy muszę znać Git, żeby używać rnr?**  
A: Nie! GitPanel i Apollo zarządzają Gitem za Ciebie. Jedyne co musisz wiedzieć: zacommituj zmiany przed wdrożeniem (GitPanel → Status → ENTER).

**Q: Nie mam site ID Netlify — jak stworzyć nowy projekt?**  
A: W Setup Wizard wybierz **„✨ Utwórz nowy projekt"**. `rnr` automatycznie wywoła `netlify sites:create` podczas pierwszego deployu i zapisze Site ID.

**Q: Skąd wiem kto i kiedy wdrożył?**  
A: Przeglądarka logów (`L`) oraz panel Apollo → zakładka Historia. Każdy rekord zawiera: autora Git, hash commita, środowisko, datę i status.

**Q: Co to znaczy "repozytorium brudne"?**  
A: Masz zmiany w plikach, które nie zostały zacommitowane. Otwórz GitPanel (`G`) → zakładka Status → zaznacz pliki → wpisz opis → ENTER.

**Q: Czy logi zawierają moje tokeny?**  
A: Nie. `rnr` automatycznie maskuje wszystkie sekrety z `rnr.conf.yaml`. W logach zamiast tokenów znajdziesz `***`.

**Q: Co to jest `promote`?**  
A: Operacja przesyłająca migracje bazy danych ze środowiska `development` do `production`. Klawisz `P` w Dashboard lub Apollo.

**Q: Push odrzucony przez "non-fast-forward"?**  
A: GitPanel pokaże ekran wyboru: `U` = `git pull --rebase + push` (zalecane) lub `F` = `push --force-with-lease`.

---

## Podziękowania

`rnr` został stworzony z myślą o ludziach — nie tylko o inżynierach. Wierzymy, że narzędzia DevOps powinny być tak przyjazne jak najlepsze aplikacje mobilne.

Zbudowany z ❤️ przy użyciu:
- [Go](https://golang.org/) — szybki, bezpieczny język kompilowany
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — framework TUI z architekturą Elm
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — piękne style dla terminala
- [Bubbles](https://github.com/charmbracelet/bubbles) — gotowe komponenty TUI
- [Cobra](https://github.com/spf13/cobra) — framework CLI

---

*Dokumentacja: rnr v1.0.1*
