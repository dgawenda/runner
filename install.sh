#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  install.sh — Skrypt Instalacyjny rnr                                  ║
# ║                                                                          ║
# ║  Pobiera najnowszy plik binarny rnr z prywatnego repozytorium GitHub   ║
# ║  i instaluje go w katalogu .rnr/ bieżącego projektu.                  ║
# ║                                                                          ║
# ║  Użycie:                                                                ║
# ║    curl -fsSL https://raw.githubusercontent.com/OWNER/REPO/main/install.sh \
# ║      | bash -s -- --token ghp_TWOJ_TOKEN                               ║
# ║                                                                          ║
# ║  Lub lokalnie:                                                           ║
# ║    ./install.sh --token ghp_TWOJ_TOKEN                                  ║
# ║                                                                          ║
# ║  Opcje:                                                                  ║
# ║    --token TOKEN    GitHub Personal Access Token (wymagany)             ║
# ║    --repo  REPO     Repozytorium GitHub (domyślnie: z zmiennej RNR_REPO)║
# ║    --tag   TAG      Wersja do zainstalowania (domyślnie: najnowsza)     ║
# ║    --dir   DIR      Katalog instalacji (domyślnie: .rnr/)               ║
# ╚══════════════════════════════════════════════════════════════════════════╝

set -euo pipefail

# ─── Kolory ────────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ─── Domyślne wartości ─────────────────────────────────────────────────────

GITHUB_TOKEN="${RNR_GITHUB_TOKEN:-}"
GITHUB_REPO="${RNR_REPO:-neution/rnr}"
RELEASE_TAG="${RNR_VERSION:-latest}"
INSTALL_DIR="${RNR_INSTALL_DIR:-.rnr}"
BINARY_NAME="rnr"

# ─── Parser argumentów ────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    --token)
      GITHUB_TOKEN="$2"
      shift 2
      ;;
    --repo)
      GITHUB_REPO="$2"
      shift 2
      ;;
    --tag)
      RELEASE_TAG="$2"
      shift 2
      ;;
    --dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
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

# ─── Weryfikacja wymagań ───────────────────────────────────────────────────

check_requirements() {
  step "Sprawdzanie wymagań..."

  for cmd in curl tar; do
    if ! command -v "$cmd" &>/dev/null; then
      die "Wymagane narzędzie '$cmd' nie jest zainstalowane."
    fi
  done

  if [[ -z "$GITHUB_TOKEN" ]]; then
    die "Brak tokenu GitHub!\n\n  Podaj token przez:\n  • Argument:   --token ghp_TWOJ_TOKEN\n  • Zmienną:    RNR_GITHUB_TOKEN=ghp_TWOJ_TOKEN ./install.sh\n\n  Token potrzebny do pobrania z prywatnego repozytorium."
  fi

  success "Wymagania spełnione"
}

# ─── Wykrywanie platformy ─────────────────────────────────────────────────

detect_platform() {
  step "Wykrywanie platformy..."

  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"

  case "$OS" in
    linux*)   OS="linux" ;;
    darwin*)  OS="darwin" ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *)        die "Nieobsługiwany system operacyjny: $OS" ;;
  esac

  case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    i386|i686)     ARCH="386" ;;
    *)             die "Nieobsługiwana architektura: $ARCH" ;;
  esac

  PLATFORM="${OS}_${ARCH}"
  success "Platforma: ${CYAN}${PLATFORM}${NC}"
}

# ─── Pobieranie informacji o Release ─────────────────────────────────────

fetch_release_info() {
  step "Pobieranie informacji o wersji..."

  local api_url
  if [[ "$RELEASE_TAG" == "latest" ]]; then
    api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
  else
    api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/tags/${RELEASE_TAG}"
  fi

  local response
  response=$(curl -fsSL \
    -H "Authorization: Bearer ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "$api_url") || die "Nie można pobrać informacji o release. Sprawdź token i nazwę repozytorium."

  RELEASE_TAG_RESOLVED=$(echo "$response" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  if [[ -z "$RELEASE_TAG_RESOLVED" ]]; then
    die "Nie znaleziono release w repozytorium '${GITHUB_REPO}'.\nSprawdź czy token ma dostęp do repozytorium i czy istnieją jakieś release."
  fi

  success "Wersja: ${CYAN}${RELEASE_TAG_RESOLVED}${NC}"

  # Znajdź URL do pobrania dla bieżącej platformy
  ASSET_URL=$(echo "$response" | grep "browser_download_url" | grep "${PLATFORM}" | sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/')

  # Jeśli nie ma .tar.gz sprawdź .zip
  if [[ -z "$ASSET_URL" ]]; then
    ASSET_URL=$(echo "$response" | grep "browser_download_url" | grep "${OS}" | sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/' | head -1)
  fi

  if [[ -z "$ASSET_URL" ]]; then
    die "Brak pliku binarnego dla platformy '${PLATFORM}' w release '${RELEASE_TAG_RESOLVED}'.\n\nDostępne pliki:\n$(echo "$response" | grep '"name"' | grep -v "tag_name" | sed 's/.*"name": *"\([^"]*\)".*/  • \1/')"
  fi

  info "Plik: ${ASSET_URL##*/}"
}

# ─── Pobieranie Binarki ────────────────────────────────────────────────────

download_binary() {
  step "Pobieranie pliku binarnego..."

  # Utwórz tymczasowy katalog
  TMP_DIR=$(mktemp -d)
  trap 'rm -rf "$TMP_DIR"' EXIT

  local archive_file="${TMP_DIR}/rnr_archive"

  # Pobierz z autoryzacją
  curl -fsSL --progress-bar \
    -H "Authorization: Bearer ${GITHUB_TOKEN}" \
    -H "Accept: application/octet-stream" \
    -L "$ASSET_URL" \
    -o "$archive_file" || die "Pobieranie nieudane. Sprawdź połączenie sieciowe i token."

  success "Pobrano plik"

  # Rozpakuj
  step "Rozpakowywanie..."

  if [[ "$ASSET_URL" == *.tar.gz ]]; then
    tar -xzf "$archive_file" -C "$TMP_DIR" || die "Błąd rozpakowywania archiwum tar.gz"
  elif [[ "$ASSET_URL" == *.zip ]]; then
    unzip -q "$archive_file" -d "$TMP_DIR" || die "Błąd rozpakowywania archiwum zip"
  elif file "$archive_file" 2>/dev/null | grep -q "executable"; then
    # Bezpośredni plik binarny
    cp "$archive_file" "${TMP_DIR}/${BINARY_NAME}"
  else
    # Spróbuj traktować jako plik binarny
    cp "$archive_file" "${TMP_DIR}/${BINARY_NAME}"
  fi

  # Znajdź binarny plik
  BINARY_PATH=$(find "$TMP_DIR" -name "${BINARY_NAME}" -o -name "${BINARY_NAME}.exe" 2>/dev/null | head -1)
  if [[ -z "$BINARY_PATH" ]]; then
    # Ostatnia próba: użyj pobranego pliku bezpośrednio
    BINARY_PATH="$archive_file"
  fi

  success "Rozpakowano"
}

# ─── Instalacja ───────────────────────────────────────────────────────────

install_binary() {
  step "Instalowanie do ${INSTALL_DIR}/..."

  # Utwórz katalog docelowy
  mkdir -p "$INSTALL_DIR"

  # Skopiuj binarny plik
  local dest="${INSTALL_DIR}/${BINARY_NAME}"
  cp "$BINARY_PATH" "$dest" || die "Nie można skopiować pliku binarnego"
  chmod +x "$dest" || die "Nie można nadać uprawnień wykonania"

  success "Zainstalowano: ${CYAN}${dest}${NC}"
}

# ─── Weryfikacja ──────────────────────────────────────────────────────────

verify_installation() {
  step "Weryfikacja instalacji..."

  local binary="${INSTALL_DIR}/${BINARY_NAME}"

  if [[ ! -f "$binary" ]]; then
    die "Plik binarny nie istnieje: $binary"
  fi

  if [[ ! -x "$binary" ]]; then
    die "Plik binarny nie ma uprawnień wykonania"
  fi

  local version
  version=$("$binary" version 2>/dev/null || echo "dev")
  success "rnr ${version} zainstalowany pomyślnie"
}

# ─── Konfiguracja PATH ────────────────────────────────────────────────────

setup_path() {
  local binary_dir
  binary_dir=$(realpath "$INSTALL_DIR")

  echo ""
  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${GREEN}  ✅ rnr został zainstalowany pomyślnie!${NC}"
  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
  echo -e "  Uruchom rnr z bieżącego projektu:"
  echo ""
  echo -e "  ${CYAN}${BOLD}./.rnr/rnr${NC}          # Uruchom TUI"
  echo -e "  ${CYAN}${BOLD}./.rnr/rnr init${NC}     # Kreator konfiguracji"
  echo -e "  ${CYAN}${BOLD}./.rnr/rnr --help${NC}   # Pomoc"
  echo ""

  # Zaproponuj alias
  echo -e "  ${YELLOW}💡 Opcjonalnie: dodaj alias do ~/.bashrc lub ~/.zshrc:${NC}"
  echo ""
  echo -e "  ${BOLD}alias rnr='./.rnr/rnr'${NC}"
  echo ""
  echo -e "  ${YELLOW}💡 Lub dodaj do PATH:${NC}"
  echo ""
  echo -e "  ${BOLD}export PATH=\"\$PATH:${binary_dir}\"${NC}"
  echo ""
  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# ─── Główna logika ────────────────────────────────────────────────────────

main() {
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

  check_requirements
  detect_platform
  fetch_release_info
  download_binary
  install_binary
  verify_installation
  setup_path
}

main "$@"
