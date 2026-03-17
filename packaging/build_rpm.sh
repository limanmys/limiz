#!/usr/bin/env bash
set -euo pipefail

# ============================================================
#  Limiz - RPM Paketi Oluşturma Scripti
# ============================================================
#  Kullanım:
#    ./build_rpm.sh
#
#  Gereksinimler:
#    - Kök dizinde derlenmiş "limiz" binary'si
#    - services/limiz.service dosyası
#    - rpmbuild komutu (rpm-build paketi ile gelir)
# ============================================================

# ---- Sürüm ve Geliştirici Bilgileri ----
VERSION="1.0.0"
RELEASE="1"
PACKAGER="Ali Orhun Akkirman <aliorhun@example.com>"
DESCRIPTION="Limiz - Prometheus-compatible system metrics exporter"
LICENSE="MIT"
URL="https://github.com/limanmys/limiz"
ARCH="x86_64"

# ---- Sabitler ----
PKG_NAME="limiz"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BUILD_ROOT="${ROOT_DIR}/build/rpm"
RPMBUILD_DIR="${BUILD_ROOT}/rpmbuild"

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
command -v rpmbuild &>/dev/null || error "rpmbuild bulunamadı. Kurun: sudo dnf install rpm-build  (veya sudo yum install rpm-build)"

# ---- Temizlik ----
rm -rf "${RPMBUILD_DIR}"
info "Build dizini hazırlanıyor: ${RPMBUILD_DIR}"

# ---- rpmbuild dizin yapısı ----
mkdir -p "${RPMBUILD_DIR}"/{BUILD,RPMS,SOURCES,SPECS,SRPMS,BUILDROOT}

# ---- Kaynak arşivi oluştur ----
SOURCE_DIR="${RPMBUILD_DIR}/SOURCES/${PKG_NAME}-${VERSION}"
mkdir -p "${SOURCE_DIR}"
cp "${ROOT_DIR}/limiz" "${SOURCE_DIR}/"
cp "${SCRIPT_DIR}/linux/limiz.service" "${SOURCE_DIR}/"

tar czf "${RPMBUILD_DIR}/SOURCES/${PKG_NAME}-${VERSION}.tar.gz" \
    -C "${RPMBUILD_DIR}/SOURCES" "${PKG_NAME}-${VERSION}"
rm -rf "${SOURCE_DIR}"
info "Kaynak arşivi oluşturuldu."

# ---- SPEC dosyası ----
cat > "${RPMBUILD_DIR}/SPECS/${PKG_NAME}.spec" <<SPEC
Name:           ${PKG_NAME}
Version:        ${VERSION}
Release:        ${RELEASE}%{?dist}
Summary:        ${DESCRIPTION}
License:        ${LICENSE}
URL:            ${URL}
Packager:       ${PACKAGER}
Source0:        %{name}-%{version}.tar.gz

BuildArch:      ${ARCH}
Requires(pre):  shadow-utils
Requires:       systemd

%description
Prometheus node_exporter'ın basitleştirilmiş bir Go implementasyonu.
Linux sistemlerde temel sistem metriklerini Prometheus exposition
formatında sunar. Opsiyonel olarak metrikleri SQLite veya JSONL
formatında yerel diske kaydedebilir.

%prep
%setup -q

%install
rm -rf %{buildroot}

# Binary
install -D -m 0755 limiz %{buildroot}/usr/local/bin/limiz

# Servis dosyası
install -D -m 0644 limiz.service %{buildroot}%{_unitdir}/limiz.service

# Config dizini ve varsayılan config
install -d -m 0750 %{buildroot}/etc/limiz
cat > %{buildroot}/etc/limiz/config.json <<'EOF'
{
}
EOF
chmod 0640 %{buildroot}/etc/limiz/config.json

# Data dizini
install -d -m 0750 %{buildroot}/var/lib/limiz

# Plugin dizinleri
install -d -m 0755 %{buildroot}/usr/lib/limiz/plugins/metric
install -d -m 0755 %{buildroot}/usr/lib/limiz/plugins/data
install -d -m 0750 %{buildroot}/usr/lib/limiz/tls

%pre
# Sistem kullanıcısı oluştur
getent passwd limiz >/dev/null 2>&1 || \
    useradd --system --no-create-home --shell /sbin/nologin limiz

%post
%systemd_post limiz.service
systemctl enable limiz.service >/dev/null 2>&1 || true
systemctl start limiz.service >/dev/null 2>&1 || true

%preun
%systemd_preun limiz.service

%postun
%systemd_postun_with_restart limiz.service

# Kaldırma durumunda (upgrade değil) kullanıcıyı sil
if [ \$1 -eq 0 ]; then
    getent passwd limiz >/dev/null 2>&1 && userdel limiz 2>/dev/null || true
fi

%files
%attr(0755, root, root) /usr/local/bin/limiz
%{_unitdir}/limiz.service
%dir %attr(0750, root, limiz) /etc/limiz
%config(noreplace) %attr(0640, root, limiz) /etc/limiz/config.json
%dir %attr(0750, limiz, limiz) /var/lib/limiz
%dir %attr(0755, root, limiz) /usr/lib/limiz/plugins/metric
%dir %attr(0755, root, limiz) /usr/lib/limiz/plugins/data
%dir %attr(0750, limiz, limiz) /usr/lib/limiz/tls

%changelog
* $(LC_ALL=C date "+%a %b %d %Y") ${PACKAGER} - ${VERSION}-${RELEASE}
- Initial package
SPEC
info "SPEC dosyası oluşturuldu."

# ---- RPM oluştur ----
rpmbuild --define "_topdir ${RPMBUILD_DIR}" -bb "${RPMBUILD_DIR}/SPECS/${PKG_NAME}.spec"

RPM_FILE=$(find "${RPMBUILD_DIR}/RPMS" -name "*.rpm" -type f | head -1)

if [[ -n "${RPM_FILE}" ]]; then
    # RPM'i build kök dizinine kopyala
    cp "${RPM_FILE}" "${BUILD_ROOT}/"
    FINAL_RPM="${BUILD_ROOT}/$(basename "${RPM_FILE}")"
    info "RPM paketi oluşturuldu: ${FINAL_RPM}"
    echo ""
    echo "  Kurulum:     sudo rpm -i ${FINAL_RPM}"
    echo "               sudo dnf install ${FINAL_RPM}"
    echo "  Kaldırma:    sudo rpm -e ${PKG_NAME}"
    echo ""
else
    error "RPM dosyası bulunamadı!"
fi
