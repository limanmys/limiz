#!/usr/bin/env bash
set -euo pipefail

# ============================================================
#  Limiz - Kurulum Scripti
# ============================================================
#  Kullanım:
#    sudo ./packaging/linux/install.sh              # Derle ve kur
#    sudo ./packaging/linux/install.sh --uninstall  # Kaldır
# ============================================================

BINARY_NAME="limiz"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/${BINARY_NAME}"
SERVICE_FILE="/etc/systemd/system/${BINARY_NAME}.service"
SERVICE_USER="${BINARY_NAME}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# ---- Root kontrolü ----
[[ $EUID -eq 0 ]] || error "Bu script root olarak çalıştırılmalı: sudo $0"

# ---- Kaldırma modu ----
if [[ "${1:-}" == "--uninstall" ]]; then
    info "Limiz kaldırılıyor..."

    if systemctl is-active --quiet "${BINARY_NAME}" 2>/dev/null; then
        info "Servis durduruluyor..."
        systemctl stop "${BINARY_NAME}"
    fi

    if systemctl is-enabled --quiet "${BINARY_NAME}" 2>/dev/null; then
        info "Servis devre dışı bırakılıyor..."
        systemctl disable "${BINARY_NAME}"
    fi

    [[ -f "${SERVICE_FILE}" ]] && rm -f "${SERVICE_FILE}" && info "Servis dosyası silindi."
    systemctl daemon-reload

    [[ -f "${INSTALL_DIR}/${BINARY_NAME}" ]] && rm -f "${INSTALL_DIR}/${BINARY_NAME}" && info "Binary silindi."

    if id "${SERVICE_USER}" &>/dev/null; then
        userdel "${SERVICE_USER}" 2>/dev/null && info "Kullanıcı '${SERVICE_USER}' silindi."
    fi

    info "Kaldırma tamamlandı."
    warn "Config dosyaları korundu: ${CONFIG_DIR}"
    warn "Silmek isterseniz: sudo rm -rf ${CONFIG_DIR}"
    exit 0
fi

# ---- Go kontrolü ----
if ! command -v go &>/dev/null; then
    error "Go bulunamadı. Lütfen önce Go kurun: https://go.dev/dl/"
fi
info "Go bulundu: $(go version)"

# ---- Derleme ----
info "Derleniyor..."
cd "${ROOT_DIR}"
CGO_ENABLED=0 go build -ldflags="-s -w" -o "${BINARY_NAME}" ./cmd/limiz/
info "Derleme başarılı: ${BINARY_NAME}"

# ---- Sistem kullanıcısı ----
if ! id "${SERVICE_USER}" &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin "${SERVICE_USER}"
    info "Sistem kullanıcısı oluşturuldu: ${SERVICE_USER}"
else
    info "Sistem kullanıcısı zaten mevcut: ${SERVICE_USER}"
fi

# ---- Binary kurulumu ----
install -m 0755 "${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
info "Binary kuruldu: ${INSTALL_DIR}/${BINARY_NAME}"

# ---- Config dizini ----
mkdir -p "${CONFIG_DIR}"

if [[ ! -f "${CONFIG_DIR}/config.json" ]]; then
    cat > "${CONFIG_DIR}/config.json" <<'EOF'
{
}
EOF
    info "Boş config oluşturuldu: ${CONFIG_DIR}/config.json"
    warn "TLS ve basic auth için config.json dosyasını düzenleyin."
    warn "Örnek: ${ROOT_DIR}/docs/config.linux.example.json"
else
    info "Mevcut config korundu: ${CONFIG_DIR}/config.json"
fi

chown -R root:${SERVICE_USER} "${CONFIG_DIR}"
chmod 750 "${CONFIG_DIR}"
chmod 640 "${CONFIG_DIR}/config.json"

# ---- Data dizini (SQLite local write için) ----
DATA_DIR="/var/lib/${BINARY_NAME}"
mkdir -p "${DATA_DIR}"
chown ${SERVICE_USER}:${SERVICE_USER} "${DATA_DIR}"
chmod 750 "${DATA_DIR}"
info "Data dizini hazır: ${DATA_DIR}"

# ---- Systemd servis dosyası ----
install -m 0644 "${SCRIPT_DIR}/${BINARY_NAME}.service" "${SERVICE_FILE}"
info "Servis dosyası kuruldu: ${SERVICE_FILE}"

systemctl daemon-reload
info "Systemd yeniden yüklendi."

# ---- Servisi başlat ----
systemctl enable "${BINARY_NAME}"
systemctl start "${BINARY_NAME}"
info "Servis etkinleştirildi ve başlatıldı."

# ---- Durum kontrolü ----
sleep 1
if systemctl is-active --quiet "${BINARY_NAME}"; then
    info "Servis çalışıyor!"
    echo ""
    echo "  Metrikler:   curl http://localhost:9110/metrics"
    echo "  Durum:       systemctl status ${BINARY_NAME}"
    echo "  Loglar:      journalctl -u ${BINARY_NAME} -f"
    echo "  Kaldırma:    sudo $0 --uninstall"
    echo ""
else
    warn "Servis başlatılamadı. Kontrol edin:"
    echo "  systemctl status ${BINARY_NAME}"
    echo "  journalctl -u ${BINARY_NAME} --no-pager -l"
fi
