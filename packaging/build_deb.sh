#!/usr/bin/env bash
set -euo pipefail

# ============================================================
#  Limiz - DEB Paketi Oluşturma Scripti
# ============================================================
#  Kullanım:
#    ./build_deb.sh
#
#  Gereksinimler:
#    - Kök dizinde derlenmiş "limiz" binary'si
#    - services/limiz.service dosyası
#    - dpkg-deb komutu (genellikle dpkg paketi ile gelir)
# ============================================================

# ---- Sürüm ve Geliştirici Bilgileri ----
VERSION="1.0.0"
MAINTAINER="Ali Orhun Akkirman <aliorhun@example.com>"
DESCRIPTION="Limiz - Prometheus-compatible system metrics exporter"
HOMEPAGE="https://github.com/limanmys/limiz"
ARCH="amd64"

# ---- Sabitler ----
PKG_NAME="limiz"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BUILD_DIR="${ROOT_DIR}/build/deb/${PKG_NAME}_${VERSION}_${ARCH}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# ---- Kontroller ----
[[ -f "${ROOT_DIR}/limiz" ]] || error "Kök dizinde 'limiz' binary'si bulunamadı. Önce derleyin: CGO_ENABLED=0 go build -ldflags='-s -w' -o limiz ./cmd/limiz/"
[[ -f "${SCRIPT_DIR}/linux/limiz.service" ]] || error "packaging/linux/limiz.service dosyası bulunamadı."
command -v dpkg-deb &>/dev/null || error "dpkg-deb bulunamadı. Kurun: sudo apt install dpkg"

# ---- Temizlik ----
rm -rf "${BUILD_DIR}"
info "Build dizini hazırlanıyor: ${BUILD_DIR}"

# ---- Dizin yapısı ----
mkdir -p "${BUILD_DIR}/DEBIAN"
mkdir -p "${BUILD_DIR}/usr/local/bin"
mkdir -p "${BUILD_DIR}/usr/lib/limiz/plugins/metric"
mkdir -p "${BUILD_DIR}/usr/lib/limiz/plugins/data"
mkdir -p "${BUILD_DIR}/usr/lib/limiz/tls"
mkdir -p "${BUILD_DIR}/etc/limiz"
mkdir -p "${BUILD_DIR}/var/lib/limiz"
mkdir -p "${BUILD_DIR}/lib/systemd/system"

# ---- Binary ----
install -m 0755 "${ROOT_DIR}/limiz" "${BUILD_DIR}/usr/local/bin/limiz"
info "Binary eklendi."

# ---- Data Plugins ----
if [[ -f "${ROOT_DIR}/plugins/data/folder-size" ]]; then
    install -m 0755 "${ROOT_DIR}/plugins/data/folder-size" "${BUILD_DIR}/usr/lib/limiz/plugins/data/folder-size"
    if [[ -f "${ROOT_DIR}/plugins/data/folder-size.sig" ]]; then
        install -m 0644 "${ROOT_DIR}/plugins/data/folder-size.sig" "${BUILD_DIR}/usr/lib/limiz/plugins/data/folder-size.sig"
    fi
    info "Data plugin eklendi: folder-size"
else
    warn "plugins/data/folder-size bulunamadı, atlanıyor."
fi

# ---- Servis dosyası ----
install -m 0644 "${SCRIPT_DIR}/linux/limiz.service" "${BUILD_DIR}/lib/systemd/system/limiz.service"
info "Servis dosyası eklendi."

# ---- Boş config ----
cat > "${BUILD_DIR}/etc/limiz/config.json" <<'EOF'
{
}
EOF
chmod 640 "${BUILD_DIR}/etc/limiz/config.json"
info "Varsayılan config oluşturuldu."

# ---- DEBIAN/control ----
INSTALLED_SIZE=$(du -sk "${BUILD_DIR}" | awk '{print $1}')

cat > "${BUILD_DIR}/DEBIAN/control" <<EOF
Package: ${PKG_NAME}
Version: ${VERSION}
Section: monitoring
Priority: optional
Architecture: ${ARCH}
Installed-Size: ${INSTALLED_SIZE}
Maintainer: ${MAINTAINER}
Homepage: ${HOMEPAGE}
Description: ${DESCRIPTION}
 Prometheus node_exporter'ın basitleştirilmiş bir Go implementasyonu.
 Linux sistemlerde temel sistem metriklerini Prometheus exposition
 formatında sunar. Opsiyonel olarak metrikleri SQLite veya JSONL
 formatında yerel diske kaydedebilir.
EOF
info "control dosyası oluşturuldu."

# ---- DEBIAN/conffiles ----
cat > "${BUILD_DIR}/DEBIAN/conffiles" <<'EOF'
/etc/limiz/config.json
EOF

# ---- DEBIAN/preinst ----
cat > "${BUILD_DIR}/DEBIAN/preinst" <<'SCRIPT'
#!/bin/sh
set -e

# Sistem kullanıcısı oluştur
if ! getent passwd limiz >/dev/null 2>&1; then
    useradd --system --no-create-home --shell /usr/sbin/nologin limiz
fi
SCRIPT
chmod 0755 "${BUILD_DIR}/DEBIAN/preinst"

# ---- DEBIAN/postinst ----
cat > "${BUILD_DIR}/DEBIAN/postinst" <<'SCRIPT'
#!/bin/sh
set -e

# Config dosyası yoksa boş JSON olarak oluştur
if [ ! -f /etc/limiz/config.json ]; then
    echo '{}' > /etc/limiz/config.json
fi

# Dizin izinleri
chown root:limiz /etc/limiz
chmod 750 /etc/limiz
chown root:limiz /etc/limiz/config.json
chmod 640 /etc/limiz/config.json
chown limiz:limiz /var/lib/limiz
chmod 750 /var/lib/limiz
chown root:limiz /usr/lib/limiz/plugins/metric
chmod 755 /usr/lib/limiz/plugins/metric
chown root:limiz /usr/lib/limiz/plugins/data
chmod 755 /usr/lib/limiz/plugins/data

# Systemd
systemctl daemon-reload

# İlk kurulumda servisi etkinleştir ve başlat
if [ "$1" = "configure" ]; then
    systemctl enable limiz.service || true
    systemctl start limiz.service || true
fi
SCRIPT
chmod 0755 "${BUILD_DIR}/DEBIAN/postinst"

# ---- DEBIAN/prerm ----
cat > "${BUILD_DIR}/DEBIAN/prerm" <<'SCRIPT'
#!/bin/sh
set -e

# Kaldırma öncesi servisi durdur
if [ "$1" = "remove" ] || [ "$1" = "purge" ]; then
    if systemctl is-active --quiet limiz.service 2>/dev/null; then
        systemctl stop limiz.service || true
    fi
    if systemctl is-enabled --quiet limiz.service 2>/dev/null; then
        systemctl disable limiz.service || true
    fi
fi
SCRIPT
chmod 0755 "${BUILD_DIR}/DEBIAN/prerm"

# ---- DEBIAN/postrm ----
cat > "${BUILD_DIR}/DEBIAN/postrm" <<'SCRIPT'
#!/bin/sh
set -e

systemctl daemon-reload || true

# Purge modunda config ve data dizinlerini sil, kullanıcıyı kaldır
if [ "$1" = "purge" ]; then
    rm -rf /etc/limiz
    rm -rf /var/lib/limiz
    if getent passwd limiz >/dev/null 2>&1; then
        userdel limiz 2>/dev/null || true
    fi
fi
SCRIPT
chmod 0755 "${BUILD_DIR}/DEBIAN/postrm"

# ---- Paketi oluştur ----
DEB_FILE="${ROOT_DIR}/build/deb/${PKG_NAME}_${VERSION}_${ARCH}.deb"
dpkg-deb --build "${BUILD_DIR}" "${DEB_FILE}"

info "DEB paketi oluşturuldu: ${DEB_FILE}"
echo ""
echo "  Kurulum:     sudo dpkg -i ${DEB_FILE}"
echo "  Kaldırma:    sudo dpkg -r ${PKG_NAME}"
echo "  Tamamen sil: sudo dpkg -P ${PKG_NAME}"
echo ""
