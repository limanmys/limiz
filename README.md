# Limiz

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**L**ightweight **I**nterface for **M**onitor and **I**nsight **Z**one — a cross-platform Prometheus metrics exporter written in Go with zero external dependencies. Collects CPU, memory, disk, network and filesystem metrics and exposes them in Prometheus exposition format. Supports TLS, basic auth, a plugin system, JSONL local write, and a browser-based configuration UI.

[Türkçe](#türkçe) · [English](#english)

---

<a name="türkçe"></a>
## Limiz: İzleme ve İçgörü Alanı için Hafif Arayüz

**L**ightweight **I**nterface for **M**onitor and **I**nsight **Z**one — sıfır harici bağımlılıkla Go ile yazılmış, çok platformlu bir Prometheus metrik dışa aktarıcısı. CPU, bellek, disk, ağ ve dosya sistemi metriklerini Prometheus exposition formatında sunar. TLS, temel kimlik doğrulama, plugin sistemi, JSONL yerel yazma ve tarayıcı üzerinden yapılandırma arayüzü içerir.

### Özellikler

- **Metrikler** (`/metrics`) — CPU, bellek, disk G/Ç, dosya sistemi, ağ, yük ortalaması, çalışma süresi (Linux & Windows)
- **Veriler** (`/datas`) — Sistem bilgisi için JSON endpoint: servisler, paketler, işletim sistemi detayları, donanım, açık portlar, disk sağlığı
- **TLS** — Dosya tabanlı PEM sertifikaları veya Windows Sertifika Deposu
- **Temel Auth** — Endpoint bazında yapılandırılabilir (metrikler ve veriler bağımsız)
- **Plugin Sistemi** — Ed25519 imzalı harici binary'lerle her iki endpoint'i genişletme
- **Yerel Yazma** — Metrik anlık görüntülerini rotasyonlu JSONL dosyalarına kaydetme
- **Web Arayüzü** (`/configuration`) — Config dosyasını elle düzenlemeden tarayıcı üzerinden ilk kurulum
- **Windows Servisi** — Tam SCM entegrasyonu (`install`, `uninstall`, `start`, `stop`)

### Hızlı Başlangıç

```bash
# Linux — derle ve çalıştır
go build -o limiz ./cmd/limiz/
./limiz
# → http://localhost:9110/metrics

# Windows — derle ve çalıştır
go build -o limiz.exe ./cmd/limiz/
.\limiz.exe
# → http://localhost:9110/metrics

# İlk kurulum için web arayüzü (config dosyası gerekmez)
# Tarayıcıda http://localhost:9110/configuration adresini açın
```

### Varsayılan Portlar

| Endpoint     | Varsayılan |
|--------------|------------|
| Metrikler    | `http(s)://:9110/metrics` |
| Veriler      | `http(s)://:9110/datas` |
| Yapılandırma | `http(s)://:9110/configuration` |

### Gereksinimler

- Go 1.22+
- Harici bağımlılık yok (yalnızca stdlib)
- Linux veya Windows (amd64)

### Kurulum (Linux)

```bash
# Systemd servisi olarak kur
sudo bash packaging/linux/install.sh

# Kaldırma
sudo bash packaging/linux/install.sh --uninstall
```

Kurulum sonrası dosya yapısı:

```
/usr/local/bin/limiz
/etc/limiz/config.json
/var/lib/limiz/
/etc/systemd/system/limiz.service
```

### Kurulum (Windows)

`setup.exe`'yi Yönetici olarak çalıştırın. Servisi kurar ve ilk çalıştırmada web yapılandırma arayüzünü açar.

Manuel kurulum için:

```powershell
.\limiz.exe --config "C:\Program Files\limiz\config.json" install
.\limiz.exe start
```

Servis komutları: `install`, `uninstall`, `start`, `stop`, `init-config`

### Yapılandırma

Tam yapılandırma referansı için [`docs/config.linux.example.json`](docs/config.linux.example.json) veya [`docs/config.windows.example.json`](docs/config.windows.example.json) dosyalarına bakın.

```json
{
  "listen_address": ":9110",
  "tls": { "cert_file": "/etc/limiz/tls/server.crt", "key_file": "/etc/limiz/tls/server.key" },
  "basic_auth": { "username": "prometheus", "password": "secret" },
  "local_write": { "enabled": true, "interval": "5m", "db_path": "/var/lib/limiz/metrics.jsonl", "rotate": "24h", "max_files": 5 }
}
```

### Plugin Sistemi

Limiz, her iki endpoint için de harici plugin binary'lerini destekler. Plugin'ler çalışma zamanında Ed25519 imzalarıyla doğrulanır. Ayrıntılar için [`docs/plugin-system.md`](docs/plugin-system.md) dosyasına bakın.

### Lisans

MIT — bkz. [LICENSE](LICENSE)

---

<a name="english"></a>
## Limiz: Lightweight Interface for Monitor and Insight Zone

**L**ightweight **I**nterface for **M**onitor and **I**nsight **Z**one — a cross-platform Prometheus metrics exporter written in Go with zero external dependencies. Exposes system metrics in Prometheus exposition format. Supports TLS, basic auth, a plugin system, JSONL local write, and a browser-based configuration UI.

### Features

- **Metrics** (`/metrics`) — CPU, memory, disk I/O, filesystem, network, load average, uptime (Linux & Windows)
- **Datas** (`/datas`) — JSON endpoint for system info: services, packages, OS details, hardware, open ports, disk health
- **TLS** — File-based PEM certificates or Windows Certificate Store
- **Basic Auth** — Per-endpoint configurable (metrics and datas independently)
- **Plugin System** — Extend both endpoints with Ed25519-signed external binaries
- **Local Write** — Persist metrics snapshots to rotated JSONL files
- **Web UI** (`/configuration`) — Browser-based initial setup without editing config files manually
- **Windows Service** — Full SCM integration (`install`, `uninstall`, `start`, `stop`)

### Quick Start

```bash
# Linux — build and run
go build -o limiz ./cmd/limiz/
./limiz
# → http://localhost:9110/metrics

# Windows — build and run
go build -o limiz.exe ./cmd/limiz/
.\limiz.exe
# → http://localhost:9110/metrics

# First-time setup via web UI (no config file needed)
# Open http://localhost:9110/configuration in your browser
```

### Default Ports

| Endpoint   | Default |
|------------|---------|
| Metrics    | `http(s)://:9110/metrics` |
| Datas      | `http(s)://:9110/datas` |
| Config UI  | `http(s)://:9110/configuration` |

### Requirements

- Go 1.22+
- No external dependencies (stdlib only)
- Linux or Windows (amd64)

### Installation (Linux)

```bash
# Install as systemd service
sudo bash packaging/linux/install.sh

# Uninstall
sudo bash packaging/linux/install.sh --uninstall
```

Installed file layout:

```
/usr/local/bin/limiz
/etc/limiz/config.json
/var/lib/limiz/
/etc/systemd/system/limiz.service
```

### Installation (Windows)

Run `setup.exe` as Administrator. It installs the service and opens the web configuration UI on first run.

For manual installation:

```powershell
.\limiz.exe --config "C:\Program Files\limiz\config.json" install
.\limiz.exe start
```

Service commands: `install`, `uninstall`, `start`, `stop`, `init-config`

### Configuration

See [`docs/config.linux.example.json`](docs/config.linux.example.json) or [`docs/config.windows.example.json`](docs/config.windows.example.json) for full reference.

```json
{
  "listen_address": ":9110",
  "tls": { "cert_file": "/etc/limiz/tls/server.crt", "key_file": "/etc/limiz/tls/server.key" },
  "basic_auth": { "username": "prometheus", "password": "secret" },
  "local_write": { "enabled": true, "interval": "5m", "db_path": "/var/lib/limiz/metrics.jsonl", "rotate": "24h", "max_files": 5 }
}
```

### Plugin System

Limiz supports external plugin binaries for both metrics and datas endpoints. Plugins are verified with Ed25519 signatures at runtime. See [`docs/plugin-system.md`](docs/plugin-system.md) for details.

### License

MIT — see [LICENSE](LICENSE)
