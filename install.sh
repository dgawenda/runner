#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  install.sh — Skrypt Instalacyjny rnr                                  ║
# ║                                                                          ║
# ║  TRYB ZDALNY (domyślny) — pobiera z GitHub Releases:                   ║
# ║    curl -fsSL https://raw.githubusercontent.com/dgawenda/runner/master/install.sh \║
# ║      | bash -s -- --token ghp_TWOJ_TOKEN                               ║
# ║                                                                          ║
# ║  TRYB LOKALNY — instaluje skompilowaną binarki z bieżącego katalogu:   ║
# ║    ./install.sh --local                                                 ║
# ║    ./install.sh --local --tag v1.0.0                                   ║
# ║                                                                          ║
# ║  Opcje:                                                                  ║
# ║    --token TOKEN    GitHub Personal Access Token                        ║
# ║    --repo  REPO     Repozytorium GitHub (domyślnie: dgawenda/runner)    ║
# ║    --tag   TAG      Wersja (domyślnie: najnowsza lub v1.0.0 lokalnie)  ║
# ║    --dir   DIR      Katalog instalacji (domyślnie: .rnr/)               ║
# ║    --local          Instaluj z lokalnej binarki (bez GitHub)            ║
# ╚══════════════════════════════════════════════════════════════════════════╝

set -eo pipefail

# ─── Kolory ────────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ─── Domyślne wartości ─────────────────────────────────────────────────────

GITHUB_TOKEN="${RNR_GITHUB_TOKEN:-}"
GITHUB_REPO="${RNR_REPO:-dgawenda/runner}"
RELEASE_TAG="${RNR_VERSION:-latest}"
INSTALL_DIR="${RNR_INSTALL_DIR:-.rnr}"
BINARY_NAME="rnr"
LOCAL_MODE=false

# ─── Parser argumentów ────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    --token)  GITHUB_TOKEN="$2"; shift 2 ;;
    --repo)   GITHUB_REPO="$2";  shift 2 ;;
    --tag)    RELEASE_TAG="$2";  shift 2 ;;
    --dir)    INSTALL_DIR="$2";  shift 2 ;;
    --local)  LOCAL_MODE=true;   shift   ;;
    -h|--help)
      grep "^# ║" "$0" | sed 's/^# ║  //' | sed 's/ *║$//'
      exit 0
      ;;
    *)
      echo -e "${RED}Nieznany argument: $1${NC}" >&2
      exit 1
      ;;
  esac
done

# ─── Funkcje pomocnicze ────────────────────────────────────────────────────

info()    { echo -e "${BLUE}ℹ  ${NC}$*"; }
success() { echo -e "${GREEN}✓  ${NC}$*"; }
warn()    { echo -e "${YELLOW}⚠  ${NC}$*"; }
error()   { echo -e "${RED}✗  ${NC}$*" >&2; }
step()    { echo -e "${PURPLE}▶  ${BOLD}$*${NC}"; }

die() {
  error "$*"
  exit 1
}

# ─── Logo ─────────────────────────────────────────────────────────────────

print_logo() {
  echo ""
  echo -e "${PURPLE}${BOLD}"
  echo "  ██████╗ ███╗   ██╗██████╗ "
  echo "  ██╔══██╗████╗  ██║██╔══██╗"
  echo "  ██████╔╝██╔██╗ ██║██████╔╝"
  echo "  ██╔══██╗██║╚██╗██║██╔══██╗"
  echo "  ██║  ██║██║ ╚████║██║  ██║"
  echo "  ╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝"
  echo -e "${NC}"
  echo -e "  ${CYAN}runner — Instalator${NC}"
  echo ""
}

# ─── Tryb lokalny ─────────────────────────────────────────────────────────
# Instaluje binarki skompilowaną lokalnie poleceniem 'go build'.
# Szuka pliku o nazwie pasującej do wzorca rnr_vX.Y.Z-PLATFORM lub po prostu 'rnr'.

install_local() {
  step "Tryb lokalny — szukam lokalnej binarki..."

  # Wykryj platformę
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"
  case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    i386|i686)     ARCH="386"   ;;
  esac
  case "$OS" in
    linux*)  OS="linux"   ;;
    darwin*) OS="darwin"  ;;
  esac

  # Ustal szukaną wersję
  local tag="${RELEASE_TAG}"
  if [[ "$tag" == "latest" ]]; then
    # Znajdź najnowszą binarki pasującą do wzorca
    tag=$(ls rnr_v*-"${OS}_${ARCH}" 2>/dev/null | sort -V | tail -1 | grep -oP 'v[\d.]+' || echo "v1.0.0")
  fi

  local candidates=(
    "rnr_${tag}-${OS}_${ARCH}"
    "rnr_${tag}-linux_amd64"
    "rnr_${tag}"
    "rnr"
  )

  local local_bin=""
  for candidate in "${candidates[@]}"; do
    if [[ -f "$candidate" && -x "$candidate" ]]; then
      local_bin="$candidate"
      break
    elif [[ -f "$candidate" ]]; then
      local_bin="$candidate"
      break
    fi
  done

  if [[ -z "$local_bin" ]]; then
    die "Nie znaleziono lokalnej binarki.\n\n" \
        "Skompiluj ją najpierw:\n" \
        "  go build -o rnr_${tag}-${OS}_${ARCH} ./cmd/rnr/\n\n" \
        "Lub uruchom bez --local, aby pobrać z GitHub Releases."
  fi

  info "Znaleziono: ${CYAN}${local_bin}${NC}"
  mkdir -p "$INSTALL_DIR"

  local dest="${INSTALL_DIR}/${BINARY_NAME}"
  cp "$local_bin" "$dest"
  chmod +x "$dest"

  success "Zainstalowano lokalnie: ${CYAN}${dest}${NC}  (źródło: ${local_bin})"
  INSTALLED_BINARY="$dest"
}

# ─── Weryfikacja wymagań (tryb zdalny) ────────────────────────────────────

check_requirements() {
  step "Sprawdzanie wymagań..."

  for cmd in curl tar; do
    if ! command -v "$cmd" &>/dev/null; then
      die "Wymagane narzędzie '$cmd' nie jest zainstalowane."
    fi
  done

  if [[ -z "$GITHUB_TOKEN" ]]; then
    warn "Brak tokenu GitHub (--token). Dla prywatnego repo wymagany.\n  Zmienne środowiskowe: RNR_GITHUB_TOKEN"
  fi

  success "Wymagania spełnione"
}

# ─── Wykrywanie platformy ─────────────────────────────────────────────────

detect_platform() {
  step "Wykrywanie platformy..."

  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"

  case "$OS" in
    linux*)              OS="linux"   ;;
    darwin*)             OS="darwin"  ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) die "Nieobsługiwany system operacyjny: $OS" ;;
  esac

  case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    i386|i686)     ARCH="386"   ;;
    *) die "Nieobsługiwana architektura: $ARCH" ;;
  esac

  PLATFORM="${OS}_${ARCH}"
  success "Platforma: ${CYAN}${PLATFORM}${NC}"
}

# ─── Pobieranie informacji o Release ─────────────────────────────────────

fetch_release_info() {
  step "Pobieranie informacji o release..."

  local api_url
  if [[ "$RELEASE_TAG" == "latest" ]]; then
    api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
  else
    api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/tags/${RELEASE_TAG}"
  fi

  local curl_opts=(-fsSL -H "Accept: application/vnd.github+json" -H "X-GitHub-Api-Version: 2022-11-28")
  if [[ -n "$GITHUB_TOKEN" ]]; then
    curl_opts+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
  fi

  local response
  response=$(curl "${curl_opts[@]}" "$api_url" 2>&1) || \
    die "Nie można pobrać informacji o release.\n\n" \
        "Sprawdź:\n" \
        "  • Token GitHub (--token ghp_...) dla prywatnych repo\n" \
        "  • Czy release '$RELEASE_TAG' istnieje w '$GITHUB_REPO'\n" \
        "  • Czy masz połączenie z internetem\n\n" \
        "Tryb offline (lokalny build):\n  ./install.sh --local"

  RELEASE_TAG_RESOLVED=$(echo "$response" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/' | head -1)

  if [[ -z "$RELEASE_TAG_RESOLVED" ]]; then
    # Sprawdź czy to błąd API (np. 404, 403)
    local message
    message=$(echo "$response" | grep '"message"' | sed 's/.*"message": *"\([^"]*\)".*/\1/' | head -1)
    if [[ -n "$message" ]]; then
      die "GitHub API: $message\n\nDla prywatnego repo użyj: ./install.sh --token ghp_TWOJ_TOKEN\nLub tryb lokalny: ./install.sh --local"
    fi
    die "Nie znaleziono release '$RELEASE_TAG' w '$GITHUB_REPO'.\n\nUtwórz release na GitHub lub uruchom: ./install.sh --local"
  fi

  success "Wersja: ${CYAN}${RELEASE_TAG_RESOLVED}${NC}"

  # Szukaj asset o nazwie rnr_<tag>-<platform> (wzorzec dynamiczny po wersji)
  # Kolejność priorytetów:
  #   1. rnr_v1.0.0-linux_amd64   (dokładna platforma)
  #   2. rnr_v1.0.0-linux         (tylko OS)
  #   3. cokolwiek z PLATFORM w nazwie
  #   4. cokolwiek z OS w nazwie

  local tag_clean="${RELEASE_TAG_RESOLVED#v}"  # bez 'v' prefix

  ASSET_URL=$(echo "$response" | grep "browser_download_url" | \
    grep -i "${RELEASE_TAG_RESOLVED}-${PLATFORM}\|rnr_${tag_clean}-${PLATFORM}\|rnr_v${tag_clean}-${PLATFORM}" | \
    sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/' | head -1)

  if [[ -z "$ASSET_URL" ]]; then
    ASSET_URL=$(echo "$response" | grep "browser_download_url" | grep -i "${PLATFORM}" | \
      sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/' | head -1)
  fi

  if [[ -z "$ASSET_URL" ]]; then
    ASSET_URL=$(echo "$response" | grep "browser_download_url" | grep -i "${OS}" | \
      sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/' | head -1)
  fi

  if [[ -z "$ASSET_URL" ]]; then
    local available
    available=$(echo "$response" | grep '"name"' | grep -v "tag_name" | \
      sed 's/.*"name": *"\([^"]*\)".*/  • \1/' | head -10)
    die "Brak pliku binarnego dla platformy '${PLATFORM}' w release '${RELEASE_TAG_RESOLVED}'.\n\nDostępne pliki:\n${available}\n\nLub uruchom tryb lokalny: ./install.sh --local"
  fi

  info "Asset: ${CYAN}${ASSET_URL##*/}${NC}"
}

# ─── Pobieranie i rozpakowywanie binarki ──────────────────────────────────

download_binary() {
  step "Pobieranie binarki..."

  TMP_DIR=$(mktemp -d)
  # Sprzątaj po sobie nawet przy błędzie
  trap 'rm -rf "$TMP_DIR"' EXIT

  local archive_file="${TMP_DIR}/rnr_download"

  local curl_opts=(-fsSL --progress-bar -H "Accept: application/octet-stream" -L -o "$archive_file")
  if [[ -n "$GITHUB_TOKEN" ]]; then
    curl_opts+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
  fi

  curl "${curl_opts[@]}" "$ASSET_URL" || die "Pobieranie nieudane. Sprawdź połączenie sieciowe."

  success "Pobrano ($(du -sh "$archive_file" 2>/dev/null | cut -f1 || echo '?'))"

  step "Rozpakowywanie..."

  local binary_in_tmp="${TMP_DIR}/${BINARY_NAME}"

  if [[ "$ASSET_URL" == *.tar.gz || "$ASSET_URL" == *.tgz ]]; then
    tar -xzf "$archive_file" -C "$TMP_DIR" || die "Błąd rozpakowywania tar.gz"
    # Znajdź binarki w wypakowanych plikach
    local found
    found=$(find "$TMP_DIR" -name "${BINARY_NAME}" -not -name "*.tar.gz" -type f 2>/dev/null | head -1)
    if [[ -n "$found" ]]; then
      binary_in_tmp="$found"
    fi
  elif [[ "$ASSET_URL" == *.zip ]]; then
    command -v unzip &>/dev/null || die "Wymagane narzędzie 'unzip' nie jest zainstalowane."
    unzip -q "$archive_file" -d "$TMP_DIR" || die "Błąd rozpakowywania zip"
    local found
    found=$(find "$TMP_DIR" -name "${BINARY_NAME}" -not -name "*.zip" -type f 2>/dev/null | head -1)
    if [[ -n "$found" ]]; then
      binary_in_tmp="$found"
    fi
  else
    # Bezpośredni plik binarny — skopiuj i nadaj prawa
    cp "$archive_file" "$binary_in_tmp"
  fi

  if [[ ! -f "$binary_in_tmp" ]]; then
    die "Nie znaleziono binarki '${BINARY_NAME}' po rozpakowaniu.\nSprawdź zawartość release na GitHub."
  fi

  chmod +x "$binary_in_tmp"
  BINARY_PATH="$binary_in_tmp"
  success "Rozpakowywanie zakończone"
}

# ─── Instalacja ───────────────────────────────────────────────────────────

install_binary() {
  step "Instalowanie do '${INSTALL_DIR}/'..."

  mkdir -p "$INSTALL_DIR" || die "Nie można utworzyć katalogu '$INSTALL_DIR'"

  local dest="${INSTALL_DIR}/${BINARY_NAME}"
  cp "$BINARY_PATH" "$dest" || die "Nie można skopiować binarki"
  chmod +x "$dest"

  INSTALLED_BINARY="$dest"
  success "Zainstalowano: ${CYAN}${dest}${NC}"
}

# ─── Weryfikacja ──────────────────────────────────────────────────────────

verify_installation() {
  step "Weryfikacja..."

  local binary="${INSTALLED_BINARY:-${INSTALL_DIR}/${BINARY_NAME}}"

  if [[ ! -f "$binary" ]]; then
    die "Plik binarny nie istnieje: $binary"
  fi
  if [[ ! -x "$binary" ]]; then
    chmod +x "$binary" || die "Nie można nadać uprawnień wykonania: $binary"
  fi

  # Uruchom z timeoutem — TUI blokuje, więc używamy 'version' subcommand
  local ver
  if command -v timeout &>/dev/null; then
    ver=$(timeout 3 "$binary" version 2>/dev/null || echo "?")
  else
    ver=$("$binary" version 2>/dev/null || echo "?")
  fi

  success "Działa: ${CYAN}${ver}${NC}"
}

# ─── Sprawdzenie narzędzi zewnętrznych (providers) ────────────────────────

setup_providers() {
  echo ""
  step "Sprawdzanie narzędzi zewnętrznych..."

  # Node.js
  if command -v node &>/dev/null; then
    success "Node.js: ${CYAN}$(node --version 2>/dev/null)${NC}"
  else
    warn "Node.js nie znaleziono — wymagany dla Netlify/Vercel/npm projektów"
  fi

  # Netlify CLI
  if command -v netlify &>/dev/null; then
    success "netlify-cli: ${CYAN}$(netlify --version 2>/dev/null | head -1)${NC}"
  else
    if command -v npm &>/dev/null; then
      info "Instaluję netlify-cli..."
      if npm install -g netlify-cli --quiet --no-audit 2>/dev/null; then
        success "netlify-cli zainstalowany"
      else
        warn "Nie udało się zainstalować netlify-cli automatycznie"
        warn "Zainstaluj ręcznie: ${BOLD}npm install -g netlify-cli${NC}"
      fi
    else
      warn "netlify-cli nie zainstalowany — jeśli używasz Netlify:\n  ${BOLD}npm install -g netlify-cli${NC}"
    fi
  fi

  # Supabase CLI
  if command -v supabase &>/dev/null; then
    success "supabase-cli: ${CYAN}$(supabase --version 2>/dev/null)${NC}"
  else
    warn "supabase-cli nie zainstalowany — jeśli używasz Supabase:\n  ${BOLD}npm install -g supabase${NC}  lub  ${BOLD}brew install supabase/tap/supabase${NC}"
  fi

  # Git
  if command -v git &>/dev/null; then
    success "git: ${CYAN}$(git --version 2>/dev/null)${NC}"
  else
    warn "git nie zainstalowany: ${BOLD}sudo apt install git${NC}"
  fi

  # curl
  if command -v curl &>/dev/null; then
    success "curl: ${CYAN}$(curl --version 2>/dev/null | head -1 | cut -d' ' -f1-2)${NC}"
  fi
}

# ─── Podsumowanie ─────────────────────────────────────────────────────────

print_summary() {
  local binary_dir
  binary_dir=$(realpath "$INSTALL_DIR" 2>/dev/null || echo "$INSTALL_DIR")

  echo ""
  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${GREEN}  ✅  rnr zainstalowany pomyślnie w ${CYAN}${binary_dir}/rnr${NC}"
  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
  echo -e "  Uruchom z katalogu projektu:"
  echo ""
  echo -e "  ${CYAN}${BOLD}./.rnr/rnr${NC}          → uruchamia TUI"
  echo -e "  ${CYAN}${BOLD}./.rnr/rnr init${NC}     → kreator konfiguracji (pierwszy raz)"
  echo -e "  ${CYAN}${BOLD}./.rnr/rnr --help${NC}   → lista komend"
  echo ""
  echo -e "  ${YELLOW}${BOLD}💡 Dodaj alias (wklej do ~/.bashrc lub ~/.zshrc):${NC}"
  echo ""
  echo -e "     ${BOLD}alias rnr='./.rnr/rnr'${NC}"
  echo ""
  echo -e "  ${YELLOW}${BOLD}💡 Lub dodaj katalog do PATH:${NC}"
  echo ""
  echo -e "     ${BOLD}export PATH=\"\$PATH:${binary_dir}\"${NC}"
  echo ""
  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
}

# ─── Główna logika ────────────────────────────────────────────────────────

main() {
  print_logo

  if [[ "$LOCAL_MODE" == "true" ]]; then
    # ── Tryb lokalny: skopiuj skompilowaną binarki ─────────────────────
    install_local
    verify_installation
  else
    # ── Tryb zdalny: pobierz z GitHub Releases ─────────────────────────
    check_requirements
    detect_platform
    fetch_release_info
    download_binary
    install_binary
    verify_installation
  fi

  setup_providers
  print_summary
}

main "$@"
