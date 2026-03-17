# Limiz

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> **English summary below** — [Jump to English](#english)

Prometheus node_exporter'ın basitleştirilmiş bir Go implementasyonu. Linux ve Windows sistemlerde temel sistem metriklerini Prometheus exposition formatında sunar. Ek olarak JSON tabanlı `/datas` endpoint'i, TLS desteği, temel auth, plugin sistemi ve web üzerinden konfigürasyon arayüzü sunar.

---

<a name="english"></a>
## English

Limiz is a lightweight, cross-platform Prometheus metrics exporter written in Go with zero external dependencies. It exposes system metrics in Prometheus exposition format and provides an additional JSON data endpoint.

### Features

- **Metrics** (`/metrics`) — CPU, memory, disk I/O, filesystem, network, load average, uptime (Linux & Windows)
- **Datas** (`/datas`) — JSON endpoint for system info: services, packages, OS details, hardware, open ports, disk health
- **TLS** — File-based PEM certificates or Windows Certificate Store
- **Basic Auth** — Per-endpoint configurable (metrics and datas independently)
- **Plugin System** — Extend both metrics and datas endpoints with signed external binaries (Ed25519)
- **Local Write** — Persist metrics snapshots to JSONL files with rotation
- **Web UI** (`/configuration`) — Browser-based initial setup without editing config files manually
- **Windows Service** — Full SCM integration (`install`, `uninstall`, `start`, `stop`)

### Quick Start

```bash
# Linux — build and run
go build -o limiz .
./limiz
# → http://localhost:9110/metrics

# Windows — build and run
go build -o limiz.exe .
.\limiz.exe
# → http://localhost:9110/metrics

# First-time setup via web UI (no config file needed)
# Open http://localhost:9110/configuration in your browser
```

### Default Ports

| Endpoint | Default |
|----------|---------|
| Metrics  | `http(s)://:9110/metrics` |
| Datas    | `http(s)://:9110/datas` |
| Config UI | `http(s)://:9110/configuration` |

### Requirements

- Go 1.22+
- No external dependencies (stdlib only)
- Linux or Windows (amd64)

### Installation (Linux)

```bash
# Install as systemd service
sudo bash services/install.sh
```

### Installation (Windows)

Run `setup.exe` as Administrator. It installs the service and opens the web configuration UI on first run.

### Configuration

See [`docs/config.linux.example.json`](docs/config.linux.example.json) or [`docs/config.windows.example.json`](docs/config.windows.example.json) for full configuration reference.

### Plugin System

Limiz supports external plugin binaries for both metrics and datas endpoints. Plugins are verified with Ed25519 signatures at runtime. See [`docs/plugin-system.md`](docs/plugin-system.md) for details.

### License

MIT — see [LICENSE](LICENSE)

---

## Toplanan Metrikler

| Collector    | Kaynak            | Metrikler                                                   |
|-------------|-------------------|-------------------------------------------------------------|
| CPU         | `/proc/stat`      | CPU süreleri (user/system/idle/iowait/...), context switch  |
| Memory      | `/proc/meminfo`   | MemTotal, MemFree, MemAvailable, Buffers, Cached, Swap...  |
| Disk I/O    | `/proc/diskstats` | Read/write bytes, IOPS, I/O time                           |
| Network     | `/proc/net/dev`   | RX/TX bytes, packets, errors, drops                        |
| Load Avg    | `/proc/loadavg`   | 1/5/15 dakika load average                                 |
| Filesystem  | `/proc/mounts`    | Disk boyutu, boş alan, inode sayıları                       |
| Uptime      | `/proc/uptime`    | Boot zamanı, sistem saati                                   |

## Örnek Metrik Çıktısı

`curl http://localhost:9110/metrics` komutuyla alınacak örnek çıktılara aşağıdaki bağlantılardan ulaşabilirsiniz:

- [Linux Örnek Çıktısı](docs/example_metric_linux.md)
- [Windows Örnek Çıktısı](docs/example_metric_win.md)

## Derleme

### Linux

```bash
go build -o limiz ./cmd/limiz/
```

### Windows (cross-compile)

```bash
GOOS=windows GOARCH=amd64 go build -o limiz.exe ./cmd/limiz/
```

### Windows üzerinde (PowerShell)

```powershell
go build -o limiz.exe ./cmd/limiz/
```

### Windows Hızlı Başlangıç (Tek Satır)

Derleme sonrası, config dosyası olmadan doğrudan flag'lerle çalıştırabilirsiniz:

```powershell
.\limiz.exe --local-write --local-interval 5m --local-db "C:\Program Files\limiz\metrics.db"
```

Bu komut 9110 portunda metrik yayını yapar ve her 5 dakikada bir tüm metrikleri belirtilen SQLite dosyasına kaydeder. Dizin yoksa otomatik oluşturulur. `Ctrl+C` ile durdurduğunuzda son bir snapshot alınıp veritabanı temiz kapatılır.

## Kullanım

### Basit (TLS ve auth olmadan)

```bash
./limiz
# http://localhost:9110/metrics
```

### Basic Auth ile

```bash
./limiz \
  --auth-user prometheus \
  --auth-pass secret
```

### TLS ile

```bash
# Self-signed sertifika oluşturma (test amaçlı)
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt \
  -days 365 -nodes -subj '/CN=localhost'

./limiz \
  --tls-cert server.crt \
  --tls-key server.key
```

### TLS + Basic Auth (config dosyası ile)

```bash
./limiz --config config.json
```

`config.json` örneği (Linux):

```json
{
  "listen_address": ":9110",
  "metrics_path": "/metrics",
  "tls": {
    "cert_file": "/etc/certs/server.crt",
    "key_file": "/etc/certs/server.key"
  },
  "basic_auth": {
    "username": "prometheus",
    "password": "secret"
  },
  "local_write": {
    "enabled": true,
    "interval": "5m",
    "db_path": "/var/lib/limiz/metrics.db",
    "rotate": "24h",
    "max_files": 5
  }
}
```

`config.json` örneği (Windows):

```json
{
  "listen_address": ":9110",
  "metrics_path": "/metrics",
  "tls": {
    "cert_file": "C:/Program Files/limiz/certs/server.crt",
    "key_file": "C:/Program Files/limiz/certs/server.key"
  },
  "basic_auth": {
    "username": "prometheus",
    "password": "secret"
  },
  "local_write": {
    "enabled": true,
    "interval": "5m",
    "db_path": "C:/Program Files/limiz/metrics.db",
    "rotate": "24h",
    "max_files": 5
  }
}
```

> **Dosya yolları hakkında:** JSON'da backslash (`\`) escape karakteridir.
> Windows yollarını yazarken üç seçeneğiniz var:
>
> | Format | Örnek | Not |
> |--------|-------|-----|
> | Forward slash | `"C:/ProgramData/metrics.db"` | **Önerilen** — Go ve SQLite sorunsuz kabul eder |
> | Çift backslash | `"C:\\ProgramData\\metrics.db"` | JSON escape kuralına uygun |
> | Relative path | `"data/metrics.db"` | Binary'nin bulunduğu dizine göre |
>
> Tek backslash (`"C:\ProgramData"`) **kullanmayın** — JSON parse hatası verir.

> **Öncelik sırası:** CLI flag > config dosyası > varsayılan değer.
> Örneğin config'de `"listen_address": ":8080"` yazsa bile `--listen-address :9090` ile çalıştırırsanız 9090 kullanılır.

### Tüm Flagler

| Flag               | Varsayılan   | Açıklama                                |
|--------------------|-------------|----------------------------------------|
| `--listen-address` | `:9110`     | Dinlenecek adres ve port                |
| `--metrics-path`   | `/metrics`  | Metriklerin sunulacağı path             |
| `--config`         | (yok)       | Config dosyası yolu (JSON)              |
| `--tls-cert`       | (yok)       | TLS sertifika dosyası                   |
| `--tls-key`        | (yok)       | TLS private key dosyası                 |
| `--auth-user`      | (yok)       | Basic auth kullanıcı adı                |
| `--auth-pass`      | (yok)       | Basic auth parola                       |

### Config Dosyası Alanları

| Alan              | Tip     | Varsayılan   | Açıklama                     |
|-------------------|---------|-------------|------------------------------|
| `listen_address`  | string  | `:9110`     | Dinlenecek adres ve port      |
| `metrics_path`    | string  | `/metrics`  | Metriklerin sunulacağı path   |
| `tls.cert_file`   | string  | —           | TLS sertifika dosya yolu      |
| `tls.key_file`    | string  | —           | TLS private key dosya yolu    |
| `basic_auth.username` | string | —        | Basic auth kullanıcı adı     |
| `basic_auth.password` | string | —        | Basic auth parola             |
| `local_write.enabled`  | bool   | `false`  | Local write'ı etkinleştir    |
| `local_write.interval` | string | `5m`     | Yazma aralığı (ör. 30s, 5m, 1h) |
| `local_write.db_path`  | string | `metrics.db` | SQLite dosya yolu        |
| `local_write.rotate`   | string | `24h`    | DB rotasyon süresi (0=kapalı) |
| `local_write.max_files` | int   | `5`      | Saklanacak max rotated dosya |

## `/configuration` Endpoint'i

`/configuration` endpoint'i, tarayıcı üzerinden ilk kurulum veya mevcut config durumunu kontrol etmek için kullanılır.
Auth veya TLS gerektirmez; her zaman erişilebilir durumdadır.

### GET `/configuration`

| Durum | Yanıt |
|-------|-------|
| Config dosyası dolu (servis kurulu) | `configuration: ok` (text/plain) |
| Config dosyası boş veya yok | Config oluşturma formu (HTML) |

```bash
curl http://localhost:9110/configuration
# → configuration: ok
```

Config yoksa tarayıcıda `http://localhost:9110/configuration` adresini açtığınızda
JSON textarea'sı içeren bir form görürsünüz. Form, tam config yapısını örnek değerlerle
doldurulmuş olarak sunar.

### POST `/configuration`

Config dosyası **boşken** JSON body ile POST atarak config oluşturulabilir.

```bash
curl -X POST http://localhost:9110/configuration \
  -H "Content-Type: application/json" \
  -d '{
    "listen_address": ":9110",
    "local_write": {
      "enabled": true,
      "interval": "5m",
      "db_path": "C:/Program Files/limiz/metrics.db"
    }
  }'
```

| Durum | HTTP Kodu | Yanıt |
|-------|-----------|-------|
| Config yoktu, başarıyla yazıldı | `200 OK` | `configuration saved` |
| Config zaten mevcuttu | `409 Conflict` | `configuration already exists` |
| Geçersiz JSON | `400 Bad Request` | Hata mesajı |

> **Not:** `POST /configuration` yalnızca **ilk kurulum** içindir. Config dosyası bir kez
> oluşturulduktan sonra bu endpoint üzerinden değiştirilemez. Değişiklik yapmak için
> config dosyasını doğrudan düzenleyin ve servisi yeniden başlatın.

### Kullanım Senaryosu

Servisi uzak bir Windows makineye kurduğunuzda, config dosyası yoksa:

1. Tarayıcıdan `http://<sunucu-ip>:9110/configuration` adresini açın
2. Formdaki JSON'u ihtiyacınıza göre düzenleyin
3. **Kaydet** butonuna tıklayın — config disk'e yazılır
4. Servisi yeniden başlatın: `.\limiz.exe stop` + `.\limiz.exe start`

## Local Write (SQLite'a Kayıt)

Metrikler opsiyonel olarak her X aralıkta SQLite veritabanına kaydedilebilir.
Bu sayede Prometheus olmadan da geçmiş veriler sorgulanabilir ve başka
uygulamalarla (Python, Go, DB Browser, PowerShell vb.) okunabilir.

### CLI ile etkinleştirme

**Linux:**
```bash
# Her 5 dakikada metrics.db'ye yaz
./limiz --local-write --local-interval 5m

# Özel konum, 1 saatlik rotasyon, max 10 dosya
./limiz \
  --local-write \
  --local-interval 1m \
  --local-db /var/lib/limiz/metrics.db \
  --local-rotate 1h \
  --local-max-files 10
```

**Windows (PowerShell):**
```powershell
# Her 5 dakikada metrics.db'ye yaz
.\limiz.exe --local-write --local-interval 5m

# Özel konum
.\limiz.exe `
  --local-write `
  --local-interval 1m `
  --local-db "C:\Program Files\limiz\metrics.db" `
  --local-rotate 1h `
  --local-max-files 10
```

> **Not:** CLI'da Windows yollarını tırnak içinde ve normal backslash ile yazabilirsiniz.
> Backslash escape sorunu sadece JSON dosyalarında geçerlidir.

### Config dosyası ile

**Linux:**
```json
{
  "local_write": {
    "enabled": true,
    "interval": "5m",
    "db_path": "/var/lib/limiz/metrics.db",
    "rotate": "24h",
    "max_files": 5
  }
}
```

**Windows:**
```json
{
  "local_write": {
    "enabled": true,
    "interval": "5m",
    "db_path": "C:/Program Files/limiz/metrics.db",
    "rotate": "24h",
    "max_files": 5
  }
}
```

### Local Write CLI Flagleri

| Flag               | Varsayılan    | Açıklama                                 |
|--------------------|--------------|------------------------------------------|
| `--local-write`    | `false`      | Local write'ı etkinleştir                 |
| `--local-interval` | `5m`         | Yazma aralığı (30s, 5m, 1h, vb.)         |
| `--local-db`       | `metrics.db` | SQLite dosya yolu                         |
| `--local-rotate`   | `24h`        | Bu süre sonunda DB rotate edilir (0=kapalı)|
| `--local-max-files`| `5`          | Saklanacak max rotate dosya sayısı        |

### Rotasyon

`rotate` süresi dolduğunda mevcut DB dosyası zaman damgası ile yeniden adlandırılır
ve yeni bir boş DB oluşturulur:

**Linux:**
```
/var/lib/limiz/metrics.db                    # Aktif (güncel)
/var/lib/limiz/metrics-20260313-080000.db    # 08:00'da rotate edilmiş
/var/lib/limiz/metrics-20260312-080000.db    # Önceki gün
```

**Windows:**
```
C:\Program Files\limiz\metrics.db                    # Aktif (güncel)
C:\Program Files\limiz\metrics-20260313-080000.db    # 08:00'da rotate edilmiş
C:\Program Files\limiz\metrics-20260312-080000.db    # Önceki gün
```

`max_files` limitine ulaşılınca en eski dosyalar otomatik silinir.

### SQLite Şeması

```sql
CREATE TABLE metrics (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp  TEXT    NOT NULL,    -- ISO 8601 / RFC 3339 (UTC)
    ts_unix    REAL    NOT NULL,    -- Unix timestamp (saniye, ms hassasiyet)
    name       TEXT    NOT NULL,    -- Metrik adı (ör. node_cpu_seconds_total)
    labels     TEXT    NOT NULL,    -- Label'lar: "cpu=cpu0,mode=user"
    value      REAL    NOT NULL,    -- Metrik değeri
    type       TEXT    NOT NULL     -- gauge veya counter
);

-- İndeksler
CREATE INDEX idx_metrics_ts ON metrics(ts_unix);
CREATE INDEX idx_metrics_name ON metrics(name);
CREATE INDEX idx_metrics_name_ts ON metrics(name, ts_unix);
```

### Örnek Sorgular

```sql
-- Son 1 saatteki ortalama memory kullanımı
SELECT
  datetime(ts_unix, 'unixepoch', 'localtime') AS zaman,
  ROUND(value / 1024 / 1024 / 1024, 2) AS gb
FROM metrics
WHERE name = 'node_memory_MemAvailable_bytes'
  AND ts_unix > unixepoch() - 3600
ORDER BY ts_unix;

-- CPU kullanım trendi (her snapshot için toplam idle oranı)
SELECT
  timestamp,
  ROUND(SUM(CASE WHEN labels LIKE '%mode=idle%' THEN value ELSE 0 END) /
        SUM(CASE WHEN labels LIKE '%mode=%' THEN value ELSE 0 END) * 100, 1) AS idle_pct
FROM metrics
WHERE name = 'node_cpu_seconds_total'
  AND labels NOT LIKE '%cpu=total%'
GROUP BY timestamp
ORDER BY timestamp;

-- Disk I/O: son 24 saatte yazılan toplam byte
SELECT
  labels AS device,
  ROUND((MAX(value) - MIN(value)) / 1024 / 1024 / 1024, 2) AS written_gb
FROM metrics
WHERE name = 'node_disk_written_bytes_total'
  AND ts_unix > unixepoch() - 86400
GROUP BY labels;

-- Belirli bir zaman aralığındaki tüm metrikler
SELECT * FROM metrics
WHERE ts_unix BETWEEN 1710300000 AND 1710400000
ORDER BY ts_unix, name;
```

### Başka Uygulamalardan Okuma

**Python:**
```python
import sqlite3
conn = sqlite3.connect("metrics.db")
rows = conn.execute("""
    SELECT timestamp, name, labels, value
    FROM metrics WHERE name = 'node_load1'
    ORDER BY ts_unix DESC LIMIT 10
""").fetchall()
for r in rows:
    print(r)
```

**PowerShell (Windows):**
```powershell
# SQLite modülü ile
Import-Module PSSQLite
Invoke-SqliteQuery -DataSource "metrics.db" -Query @"
    SELECT timestamp, name, value FROM metrics
    WHERE name LIKE 'node_memory%' ORDER BY ts_unix DESC LIMIT 20
"@
```

**DB Browser for SQLite:**
Dosyayı doğrudan [DB Browser for SQLite](https://sqlitebrowser.org/) ile açarak
görsel olarak sorgulayabilir, grafikleyebilir ve CSV'ye export edebilirsiniz.

## Prometheus Yapılandırması

```yaml
scrape_configs:
  - job_name: 'limiz'
    # TLS kullanılıyorsa:
    scheme: https
    tls_config:
      insecure_skip_verify: true  # self-signed için
    # Basic auth kullanılıyorsa:
    basic_auth:
      username: prometheus
      password: secret
    static_configs:
      - targets: ['localhost:9110']
```

## Systemd ile Kurulum

Otomatik kurulum scripti ile binary derleme, sistem kullanıcısı oluşturma, config dizini
hazırlama ve systemd servisi kaydetme işlemleri tek komutla yapılır:

```bash
sudo ./services/install.sh
```

Kurulum sonrası:

```bash
# Servis durumu
systemctl status limiz

# Logları takip et
journalctl -u limiz -f

# Servisi yeniden başlat (config değişikliği sonrası)
sudo systemctl restart limiz

# Kaldırma
sudo ./services/install.sh --uninstall
```

### Config düzenleme

Kurulum sonrası TLS veya basic auth eklemek için:

```bash
sudo nano /etc/limiz/config.json
sudo systemctl restart limiz
```

### Dosya Yapısı (kurulum sonrası)

```
/usr/local/bin/limiz          # Binary
/etc/limiz/config.json        # Config (root:limiz, 640)
/etc/systemd/system/limiz.service  # Systemd unit
```

## Windows Desteği

Limiz, Windows üzerinde de çalışır. Go'nun build tag mekanizması sayesinde
platform algılaması otomatiktir — aynı kaynak koddan `GOOS=windows` ile derlediğinizde
Windows collector'ları devreye girer.

### Windows'ta Metrik Kaynakları

Tüm collector'lar **wmic** ve **typeperf** komutları üzerinden WMI sınıflarını sorgular.
Windows 10, Windows 11 ve Windows Server 2016+ ile uyumludur.

| Collector   | WMI Sınıfı / Komut                     | Not                                               |
|-------------|------------------------------------------|----------------------------------------------------|
| CPU         | `wmic cpu` + `typeperf`                  | LoadPercentage, context switches/sec                |
| Memory      | `wmic os`                                | TotalVisibleMemorySize, FreePhysicalMemory, Swap    |
| Disk I/O    | `wmic Win32_PerfFormattedData_PerfDisk`  | Read/write bytes/sec, IOPS, queue length            |
| Network     | `wmic Win32_PerfFormattedData_Tcpip`     | RX/TX bytes/packets/errors per sec                  |
| Load Avg    | `wmic Win32_PerfOS_System`               | ProcessorQueueLength (load1/5/15 yaklaşımı)         |
| Filesystem  | `wmic logicaldisk`                       | Sadece sabit diskler (DriveType=3)                  |
| Uptime      | `wmic os LastBootUpTime`                 | Boot zamanı ve sistem saati                         |

### Windows Config Örneği

Projede `docs` klasörü içerisinde hazır bir `config.windows.example.json` dosyası bulunur. Önerilen dizin yapısı:

```
C:\Program Files\limiz\
├── limiz.exe
├── config.json
├── certs\
│   ├── server.crt
│   └── server.key
└── data\
    ├── metrics.db                        # Aktif veritabanı
    ├── metrics-20260313-080000.db        # Rotate edilmiş
    └── metrics-20260312-080000.db
```

`config.json` (Windows, tüm özellikler):

```json
{
  "listen_address": ":9110",
  "metrics_path": "/metrics",
  "tls": {
    "cert_file": "C:/Program Files/limiz/certs/server.crt",
    "key_file": "C:/Program Files/limiz/certs/server.key"
  },
  "basic_auth": {
    "username": "prometheus",
    "password": "secret"
  },
  "local_write": {
    "enabled": true,
    "interval": "5m",
    "db_path": "C:/Program Files/limiz/data/metrics.db",
    "rotate": "24h",
    "max_files": 5
  }
}
```

### Windows Servisi Olarak Çalıştırma

Tek exe ile servis kurulumu, başlatma, durdurma ve kaldırma yapılabilir.
Harici araç (NSSM vb.) gerekmez. Tüm komutlar **Yönetici olarak çalıştırılmalıdır.**

#### Config Oluşturma (Tek Komut)

`init-config` komutu ile flag'lerden config.json dosyası üretilir:

```powershell
# Sadece local write ile (en yaygın kullanım)
.\limiz.exe `
  --config "C:\Program Files\limiz\config.json" `
  --local-write `
  --local-interval 5m `
  --local-db "C:/Program Files/limiz/metrics.db" `
  --local-rotate 24h `
  --local-max-files 100 `
  init-config
```

Çıktı:
```
Config written to: C:\Program Files\limiz\config.json
{
  "listen_address": ":9110",
  "metrics_path": "/metrics",
  "local_write": {
    "enabled": true,
    "interval": "5m",
    "db_path": "C:/Program Files/limiz/metrics.db",
    "rotate": "24h",
    "max_files": 100
  }
}
```

Tüm seçenekler ile:

```powershell
.\limiz.exe `
  --config "C:\Program Files\limiz\config.json" `
  --listen-address :8080 `
  --local-write `
  --local-interval 2m `
  --local-db "C:/Program Files/limiz/data/metrics.db" `
  --local-rotate 12h `
  --local-max-files 50 `
  --auth-user prometheus `
  --auth-pass supersecret `
  --tls-cert "C:/certs/server.crt" `
  --tls-key "C:/certs/server.key" `
  init-config
```

> **Not:** `--config` flag'i çıktı dosya yolunu belirler. Verilmezse mevcut dizinde `config.json` oluşturulur.
> `db_path` içinde forward slash (`C:/...`) kullanın — JSON uyumlu ve Go/SQLite tarafından sorunsuz kabul edilir.

Linux'ta da aynı şekilde çalışır:

```bash
./limiz \
  --config /etc/limiz/config.json \
  --local-write \
  --local-interval 5m \
  --local-db /var/lib/limiz/metrics.db \
  --local-rotate 24h \
  --local-max-files 100 \
  init-config
```

#### Kurulum (Servis)

```powershell
# 1. Dizin oluştur ve exe'yi kopyala
mkdir "C:\Program Files\limiz"
copy limiz.exe "C:\Program Files\limiz\"
cd "C:\Program Files\limiz"

# 2. Config oluştur (tek komut)
.\limiz.exe `
  --config "C:\Program Files\limiz\config.json" `
  --local-write `
  --local-interval 5m `
  --local-db "C:/Program Files/limiz/metrics.db" `
  --local-rotate 24h `
  --local-max-files 100 `
  init-config

# 3. Servisi kur ve başlat
.\limiz.exe --config "C:\Program Files\limiz\config.json" install
.\limiz.exe start
```

#### Servis Yönetimi

```powershell
# Başlat
.\limiz.exe start

# Durdur
.\limiz.exe stop

# Durum kontrolü
sc.exe query Limiz

# Kaldır (önce durdurur, sonra siler)
.\limiz.exe uninstall
```

#### Servis Komutları Özeti

| Komut       | Açıklama                                         |
|-------------|--------------------------------------------------|
| `install`   | Servisi Windows SCM'e kaydeder (AutoStart)        |
| `uninstall` | Servisi durdurur ve SCM'den siler                 |
| `start`     | Kayıtlı servisi başlatır                          |
| `stop`      | Çalışan servisi durdurur                          |

> **Not:** `install` komutu çalıştırıldığında `--config` ile verdiğiniz yol ve
> `--run-service` flag'i servis kaydına gömülür. Servis başlatıldığında SCM bu
> parametrelerle exe'yi otomatik çalıştırır. Config dosyasını değiştirirseniz
> servisi yeniden başlatmanız (`stop` + `start`) yeterlidir; yeniden `install`
> gerekmez. Config dosyasının **yolunu** değiştirmek isterseniz `uninstall` + `install`
> yapmalısınız.

#### Console Modu (servis olmadan)

Exe'yi doğrudan çalıştırırsanız normal konsol modunda çalışır:

```powershell
# Config dosyası ile
.\limiz.exe --config config.json

# Flag'lerle — en yaygın kullanım (derleme sonrası tek satırda çalıştır)
.\limiz.exe --local-write --local-interval 5m --local-db "C:\Program Files\limiz\metrics.db"

# Tüm seçenekler ile
.\limiz.exe `
  --listen-address :9110 `
  --local-write `
  --local-interval 2m `
  --local-db "C:\Program Files\limiz\metrics.db" `
  --local-rotate 12h `
  --local-max-files 100 `
  --auth-user prometheus `
  --auth-pass secret
```

### Windows'ta Bilinen Kısıtlamalar

- Metrikler `wmic` komutlarına dayandığından her scrape'de küçük bir gecikme olabilir
- Load average Windows'ta doğrudan mevcut değildir; yerine processor queue length kullanılır
- Disk I/O metrikleri kümülatif counter yerine anlık per-second değer olarak raporlanır
- `wmic` Windows 11 ve sonrasında deprecate edilmiştir; gelecek sürümde PowerShell CIM'e geçiş planlanabilir

## Orijinal node_exporter ile Farklar

- Linux ve Windows destekler (platform-specific collector'lar)
- Daha az collector (textfile, systemd, hwmon vb. yok)
- Tek binary, harici bağımlılık yok
- Basit JSON config formatı
- Port varsayılan olarak 9110 (orijinal 9100)
