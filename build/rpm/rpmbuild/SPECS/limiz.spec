Name:           limiz
Version:        1.0.0
Release:        1%{?dist}
Summary:        Limiz - Prometheus-compatible system metrics exporter
License:        MIT
URL:            https://github.com/limanmys/limiz
Packager:       Ali Orhun Akkirman <aliorhun@example.com>
Source0:        %{name}-%{version}.tar.gz

BuildArch:      x86_64
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
install -D -m 0644 limiz.service %{buildroot}/usr/lib/systemd/system/limiz.service

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
getent passwd limiz >/dev/null 2>&1 ||     useradd --system --no-create-home --shell /sbin/nologin limiz

%post
%systemd_post limiz.service
systemctl enable limiz.service >/dev/null 2>&1 || true
systemctl start limiz.service >/dev/null 2>&1 || true

%preun
%systemd_preun limiz.service

%postun
%systemd_postun_with_restart limiz.service

# Kaldırma durumunda (upgrade değil) kullanıcıyı sil
if [ $1 -eq 0 ]; then
    getent passwd limiz >/dev/null 2>&1 && userdel limiz 2>/dev/null || true
fi

%files
%attr(0755, root, root) /usr/local/bin/limiz
/usr/lib/systemd/system/limiz.service
%dir %attr(0750, root, limiz) /etc/limiz
%config(noreplace) %attr(0640, root, limiz) /etc/limiz/config.json
%dir %attr(0750, limiz, limiz) /var/lib/limiz
%dir %attr(0755, root, limiz) /usr/lib/limiz/plugins/metric
%dir %attr(0755, root, limiz) /usr/lib/limiz/plugins/data
%dir %attr(0750, limiz, limiz) /usr/lib/limiz/tls

%changelog
* Tue Mar 17 2026 Ali Orhun Akkirman <aliorhun@example.com> - 1.0.0-1
- Initial package
