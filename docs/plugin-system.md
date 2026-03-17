# Limiz Plugin Sistemi — Tasarım Dökümanı

> **Platform:** Linux + Windows

---

## 1. Tasarım Kararı: Neden Exec Plugin?

Go'nun native `plugin` paketi yalnızca Linux'ta çalışır, Windows'u desteklemez.
Bunun yerine **Executable Plugin (Exec) Modeli** seçilmiştir:

- Plugin, bağımsız bir yürütülebilir dosyadır (`.exe` veya düz binary)
- Limiz, her scrape döngüsünde (veya cache aralığında) plugin'i alt-process olarak çalıştırır
- Plugin, Prometheus metin formatında çıktı üretir (stdout)
- Limiz çıktıyı parse eder ve kendi registry'sine ekler

**Avantajlar:**
| Özellik | Açıklama |
|---------|----------|
| Cross-platform | Linux + Windows, herhangi bir dil (Go, Python, PowerShell, Bash) |
| İzolasyon | Plugin crash'i ana süreci etkilemez |
| Dil bağımsız | Mevcut script'ler, araçlar plugin olabilir |
| Güvenli | Timeout, izin kontrolü uygulanabilir |
| Sıfır bağımlılık | Ana binary'ye bağlantı gerekmez |

---

## 2. Genel Mimari

```
┌──────────────────────────────────────────────────────────────────┐
│                        limiz (ana proses)                        │
│                                                                  │
│  ┌──────────────┐    ┌─────────────────────────────────────┐    │
│  │ Registry     │    │ ExecPluginCollector                 │    │
│  │              │    │ (collectors/plugin_exec.go — yeni)  │    │
│  │ cpu          │    │                                     │    │
│  │ memory  ─────┼────│  plugin 1: dir-size    ──► stdout   │    │
│  │ disk         │    │  plugin 2: gpu-info    ──► stdout   │    │
│  │ network      │    │  plugin 3: services    ──► stdout   │    │
│  │ ...          │    │                                     │    │
│  │ [plugins] ◄──┘    │  parse → []Metric                   │    │
│  └──────────────┘    └─────────────────────────────────────┘    │
│                                                                  │
│  config.json → "plugins": { ... }  (yeni alan)                  │
└──────────────────────────────────────────────────────────────────┘

                    ┌─────────────────────────────┐
                    │  plugins/ dizini             │
                    │  ├── dir-size(.exe)           │
                    │  ├── gpu-info(.exe)           │
                    │  └── services(.exe)           │
                    └─────────────────────────────┘
```

---

## 3. Plugin Sözleşmesi (Contract)

Her plugin şu kurallara uymak zorundadır:

### 3.1 Çalışma Modu: `--collect`

Limiz, plugin'i her zaman `--collect` argümanıyla çağırır:

```
./dir-size --collect [ek-argümanlar]
```

Plugin:
- **stdout'a** Prometheus exposition format (v0.0.4) yazar
- **stderr'a** hata ve log mesajları yazar (limiz bunları loglar, metrik olarak almaz)
- **0** exit kodu döner (başarı)
- **non-zero** exit kodu döner (hata — limiz bu plugin'den metrik almaz, uyarı loglar)

### 3.2 Çıktı Formatı

```
# HELP plugin_dir_size_bytes Dizin boyutu (bytes)
# TYPE plugin_dir_size_bytes gauge
plugin_dir_size_bytes{path="/var/log"} 1.073741824e+09
plugin_dir_size_bytes{path="/var/lib/limiz"} 5.24288e+06

# HELP plugin_gpu_temperature_celsius GPU sıcaklığı
# TYPE plugin_gpu_temperature_celsius gauge
plugin_gpu_temperature_celsius{index="0",name="RTX 4090"} 67
```

**Kural:** Tüm metrik isimleri `plugin_` önekiyle başlamalıdır. Bu, core metriklerle çakışmayı önler.

### 3.3 Çalışma Modu: `--describe` (isteğe bağlı)

```
./dir-size --describe
```

Çıktı (JSON):
```json
{
  "name": "dir-size",
  "version": "1.0.0",
  "description": "Belirtilen dizinlerin disk kullanımını ölçer",
  "author": "team",
  "platform": ["linux", "windows"]
}
```

Bu mod; versiyon yönetimi ve plugin keşfi için kullanılır, zorunlu değildir.

---

## 4. Config.json Entegrasyonu

Mevcut config yapısına yeni bir `plugins` alanı eklenir.

### 4.1 Tam Şema

```json
{
  "listen_address": ":9110",
  "metrics_path": "/metrics",
  "local_write": {
    "enabled": true,
    "interval": "5m",
    "db_path": "C:/Program Files/limiz/metrics.db",
    "rotate": "24h",
    "max_files": 100
  },
  "plugins": {
    "enabled": true,
    "dir": "C:/Program Files/limiz/plugins/metric",
    "default_timeout": "10s",
    "items": [
      {
        "name": "dir_size",
        "exec": "dir-size",
        "args": [
          "--path", "C:/inetpub/wwwroot",
          "--path", "C:/logs"
        ],
        "timeout": "30s",
        "enabled": true
      },
      {
        "name": "gpu_info",
        "exec": "gpu-info",
        "args": [],
        "timeout": "15s",
        "enabled": true
      },
      {
        "name": "services",
        "exec": "services",
        "args": ["--state", "running"],
        "timeout": "10s",
        "enabled": true
      }
    ]
  }
}
```

### 4.2 Alan Açıklamaları

| Alan | Tip | Açıklama |
|------|-----|----------|
| `plugins.enabled` | bool | Tüm plugin sistemini açar/kapatır |
| `plugins.dir` | string | Plugin binary'lerinin bulunduğu dizin |
| `plugins.default_timeout` | string | Tüm plugin'ler için varsayılan timeout (ör. `"10s"`) |
| `items[].name` | string | Collector adı (registry'de görünür, benzersiz olmalı) |
| `items[].exec` | string | Binary adı (dir içinde aranır) veya tam yol |
| `items[].args` | []string | Plugin'e geçilecek ek argümanlar |
| `items[].timeout` | string | Bu plugin için timeout (default_timeout'u override eder) |
| `items[].enabled` | bool | Tek plugin'i devre dışı bırakır |

### 4.3 Exec Yol Çözümleme Mantığı

```
exec = "dir-size"
  → plugins.dir/dir-size           (Linux)
  → plugins.dir/dir-size.exe       (Windows)

exec = "./custom/my-plugin"
  → göreceli yol, aynen kullanılır

exec = "/usr/local/bin/my-plugin"
  → mutlak yol, aynen kullanılır
```

---

## 5. Plugin Dosyaları

```
internal/collectors/
├── registry.go
├── plugin_exec.go        ← ExecPlugin collector
├── plugin_config.go      ← PluginsConfig struct
└── plugin_parser.go      ← Prometheus text parser

plugins-src/              ← Örnek plugin kaynak kodları
├── dir-size/
│   └── main.go
├── gpu-info/
│   ├── main.go
│   ├── gpu_linux.go
│   └── gpu_windows.go
└── services/
    ├── main.go
    ├── services_linux.go
    └── services_windows.go
```

### 5.1 `internal/collectors/plugin_config.go`

```go
package collectors

// PluginsConfig yapısı config.json "plugins" alanına karşılık gelir.
type PluginsConfig struct {
    Enabled        bool          `json:"enabled"`
    Dir            string        `json:"dir"`
    DefaultTimeout string        `json:"default_timeout"`
    Items          []PluginItem  `json:"items"`
}

type PluginItem struct {
    Name    string   `json:"name"`
    Exec    string   `json:"exec"`
    Args    []string `json:"args"`
    Timeout string   `json:"timeout"`
    Enabled bool     `json:"enabled"`
}
```

### 5.2 `internal/collectors/plugin_exec.go` — Davranış Tanımı

`ExecPlugin`, `Collector` interface'ini (`Name()`, `Collect()`) aşağıdaki mantıkla uygular:

```
Collect() çağrıldığında:
  1. Plugin binary yolunu çözümle (dir + exec + platform uzantısı)
  2. Timeout hesapla (item.Timeout || config.DefaultTimeout || "10s")
  3. context.WithTimeout ile alt-process başlat
  4. args'a "--collect" ekle, ardından item.Args'ı ekle
  5. stdout'u yakala
  6. Timeout aşılırsa: process öldür, uyarı logla, boş []Metric dön
  7. exit code != 0 ise: stderr'i logla, boş []Metric dön
  8. stdout'u parse et → []Metric
  9. []Metric dön
```

**Önemli:** Her `Collect()` çağrısı yeni bir subprocess başlatır.
Cache modu açıksa (`--cache`) bu sorun değildir — limiz zaten her
cache-interval'de bir kez çağırır.

### 5.3 `internal/collectors/plugin_parser.go` — Prometheus Text Parser

Standart Prometheus exposition format'ı parse eder:

```
# HELP <name> <docstring>    → Metric.Help
# TYPE <name> gauge|counter  → Metric.Type
<name>{<labels>} <value>     → Metric{Name, Labels, Value}
<name> <value>               → Metric{Name, Value} (etiket yok)
```

Basit bir line-by-line parser yeterlidir.
`strconv.ParseFloat` ile değer, `strings.Split` ile label parse edilir.

---

## 6. main.go'da Minimal Değişiklik

Mevcut collector kayıt bloğunun ardına ~10 satır eklenir:

```go
// Mevcut kayıtlar (DOKUNULMAZ):
registry.Register(collectors.NewCPUCollector())
registry.Register(collectors.NewMemoryCollector())
// ...

// YENİ BLOK — plugin yükleyici:
if cfg.Plugins != nil && cfg.Plugins.Enabled {
    for _, item := range cfg.Plugins.Items {
        if !item.Enabled {
            continue
        }
        plugin := collectors.NewExecPlugin(item, cfg.Plugins)
        registry.Register(plugin)
        log.Printf("Plugin yüklendi: %s (%s)", item.Name, item.Exec)
    }
}
```

`Config` struct'ına da yeni alan eklenir:

```go
type Config struct {
    // Mevcut alanlar (DOKUNULMAZ)
    ListenAddress string              `json:"listen_address,omitempty"`
    MetricsPath   string              `json:"metrics_path,omitempty"`
    TLS           *TLSConfig          `json:"tls,omitempty"`
    BasicAuth     *BasicAuthConfig    `json:"basic_auth,omitempty"`
    LocalWrite    *localwriter.Config `json:"local_write,omitempty"`
    Cache         *CacheConfig        `json:"cache,omitempty"`
    // YENİ:
    Plugins       *collectors.PluginsConfig `json:"plugins,omitempty"`
}
```

**Toplam değişiklik:** ~12 satır `main.go` içinde, ~3 yeni dosya.

---

## 7. Örnek Plugin'ler

### 7.1 `dir-size` — Dizin Boyutu (Cross-platform)

**Kullanım:**
```
dir-size --collect --path /var/log --path /home
dir-size --collect --path "C:/inetpub" --path "C:/logs"
```

**Çıktı:**
```
# HELP plugin_dir_size_bytes Dizin toplam boyutu (bytes)
# TYPE plugin_dir_size_bytes gauge
plugin_dir_size_bytes{path="/var/log"} 4.29496729e+09
plugin_dir_size_bytes{path="/home"} 1.073741824e+10

# HELP plugin_dir_file_count Dizindeki dosya sayısı
# TYPE plugin_dir_file_count gauge
plugin_dir_file_count{path="/var/log"} 1247
plugin_dir_file_count{path="/home"} 38291
```

**Uygulama notu:** `filepath.WalkDir` hem Linux hem Windows'ta çalışır.
Büyük dizinlerde timeout riski olduğu için item.timeout değeri yüksek tutulmalıdır (ör. `"60s"`).

---

### 7.2 `gpu-info` — GPU Bilgisi (NVIDIA / cross-platform)

**Kullanım:**
```
gpu-info --collect
```

**Linux çıktısı** (`nvidia-smi` tabanlı):
```
# HELP plugin_gpu_temperature_celsius GPU sıcaklığı
# TYPE plugin_gpu_temperature_celsius gauge
plugin_gpu_temperature_celsius{index="0",name="Tesla T4"} 52

# HELP plugin_gpu_utilization_percent GPU kullanım yüzdesi
# TYPE plugin_gpu_utilization_percent gauge
plugin_gpu_utilization_percent{index="0",name="Tesla T4"} 87

# HELP plugin_gpu_memory_used_bytes Kullanılan GPU belleği
# TYPE plugin_gpu_memory_used_bytes gauge
plugin_gpu_memory_used_bytes{index="0",name="Tesla T4"} 1.2884901888e+10

# HELP plugin_gpu_memory_total_bytes Toplam GPU belleği
# TYPE plugin_gpu_memory_total_bytes gauge
plugin_gpu_memory_total_bytes{index="0",name="Tesla T4"} 1.6106127360e+10

# HELP plugin_gpu_power_watts GPU güç tüketimi
# TYPE plugin_gpu_power_watts gauge
plugin_gpu_power_watts{index="0",name="Tesla T4"} 145.2
```

**Uygulama notu:**
- Linux: `nvidia-smi --query-gpu=... --format=csv,noheader` çalıştırılır
- Windows: aynı komut, PATH'de `nvidia-smi.exe` olması yeterli
- GPU yoksa: plugin 0 çıkar, metrik üretmez (boş stdout)
- AMD GPU için: `rocm-smi` veya WMI tabanlı ayrı branch eklenir

---

### 7.3 `services` — Servis Listesi (Cross-platform)

**Kullanım:**
```
services --collect --state running
services --collect --state all
services --collect --filter nginx --filter postgres
```

**Linux çıktısı** (`systemctl` tabanlı):
```
# HELP plugin_service_up Servis çalışıyor mu (1=running, 0=stopped)
# TYPE plugin_service_up gauge
plugin_service_up{name="nginx",state="running"} 1
plugin_service_up{name="postgresql",state="running"} 1
plugin_service_up{name="redis",state="running"} 1
plugin_service_up{name="limiz",state="running"} 1
```

**Windows çıktısı** (SCM/WMI tabanlı):
```
# HELP plugin_service_up Servis çalışıyor mu (1=running, 0=stopped)
# TYPE plugin_service_up gauge
plugin_service_up{name="LimizSvc",state="Running",start_type="Auto"} 1
plugin_service_up{name="W3SVC",state="Running",start_type="Auto"} 1
plugin_service_up{name="MSSQLSERVER",state="Stopped",start_type="Manual"} 0
```

**Uygulama notu:**
- Linux: `systemctl list-units --type=service --no-pager --plain` parse edilir
- Windows: `wmic service get Name,State,StartMode /format:csv` veya
  `Get-Service | ConvertTo-Json` (PowerShell) tabanlı okuma yapılır
- `--state running` ile yalnızca aktif servisler filtrelenebilir
- `--filter <name>` ile belirli servisler izlenebilir

---

## 8. Plugin Geliştirme Rehberi

### 8.1 Temel Şablon (Go)

```go
//go:build !ignore

package main

import (
    "flag"
    "fmt"
    "os"
)

func main() {
    collect := flag.Bool("collect", false, "Metrikleri topla ve çıktıla")
    describe := flag.Bool("describe", false, "Plugin meta verisini göster")
    flag.Parse()

    switch {
    case *describe:
        printDescribe()
    case *collect:
        if err := collect(); err != nil {
            fmt.Fprintf(os.Stderr, "hata: %v\n", err)
            os.Exit(1)
        }
    default:
        fmt.Fprintln(os.Stderr, "Kullanım: plugin --collect [args]")
        os.Exit(2)
    }
}

func printDescribe() {
    fmt.Println(`{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "Plugin açıklaması",
  "platform": ["linux", "windows"]
}`)
}

func collect() error {
    // Metrik topla
    value := 42.0

    // Prometheus formatında çıktı ver
    fmt.Println(`# HELP plugin_my_metric Metrik açıklaması`)
    fmt.Println(`# TYPE plugin_my_metric gauge`)
    fmt.Printf("plugin_my_metric{label=\"value\"} %g\n", value)
    return nil
}
```

### 8.2 Cross-Platform Derleme

```makefile
# Tüm platformlar için plugin derle
build-plugins:
    GOOS=linux  GOARCH=amd64 go build -o plugins/metric/dir-size     ./plugins-src/dir-size/
    GOOS=windows GOARCH=amd64 go build -o plugins/metric/dir-size.exe ./plugins-src/dir-size/
    GOOS=linux  GOARCH=amd64 go build -o plugins/metric/gpu-info     ./plugins-src/gpu-info/
    GOOS=windows GOARCH=amd64 go build -o plugins/metric/gpu-info.exe ./plugins-src/gpu-info/
    GOOS=linux  GOARCH=amd64 go build -o plugins/metric/services     ./plugins-src/services/
    GOOS=windows GOARCH=amd64 go build -o plugins/metric/services.exe ./plugins-src/services/
```

### 8.3 Python / PowerShell Plugin

Plugin Go olmak zorunda değildir:

**Python (Linux):**
```python
#!/usr/bin/env python3
# usage: python3 my-plugin.py --collect
import sys, os

if "--collect" in sys.argv:
    size = sum(
        os.path.getsize(os.path.join(d, f))
        for d, _, files in os.walk("/var/log")
        for f in files
    )
    print("# HELP plugin_varlog_bytes /var/log boyutu")
    print("# TYPE plugin_varlog_bytes gauge")
    print(f"plugin_varlog_bytes {size}")
```

**PowerShell (Windows):**
```powershell
# usage: powershell.exe -File my-plugin.ps1 --collect
param([switch]$collect)
if ($collect) {
    $services = Get-Service | Where-Object { $_.Status -eq 'Running' }
    Write-Output "# HELP plugin_service_count Çalışan servis sayısı"
    Write-Output "# TYPE plugin_service_count gauge"
    Write-Output "plugin_service_count $($services.Count)"
}
```

Config'de exec olarak şöyle tanımlanır:
```json
{
  "name": "varlog_size",
  "exec": "python3",
  "args": ["/usr/lib/limiz/plugins/metric/my-plugin.py", "--path", "/var/log"]
}
```

---

## 9. Güvenlik: Sadece Onaylı Plugin'lerin Çalışması

### 9.1 Problem

Config dosyasına yeni bir `items[]` satırı eklemek herhangi bir binary'yi çalıştırabilir.
Bu kabul edilemez. Çözüm: **config dosyası tek başına yeterli olmamalı.**

İki bağımsız doğrulama katmanı uygulanır:

| Katman | Yöntem | Bypass Edilemez Çünkü |
|--------|--------|----------------------|
| **1. Derleme-zamanı isim listesi** | Allowlist binary içine gömülür | Config değiştirilebilir, ama binary değiştirilemez |
| **2. Ed25519 dijital imza** | Public key binary içine gömülür | Özel anahtar olmadan geçerli imza üretilemez |

Her iki katman da **aynı anda** doğrulanmalıdır. Biri başarısız olursa plugin çalışmaz.

---

### 9.2 Katman 1 — Derleme-Zamanı İsim Listesi (Allowlist)

Plugin isimleri **limiz binary'si derlenirken** içine gömülür:

```bash
# Sadece bu üç plugin'e izin ver
go build \
  -ldflags "-X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=dir-size,gpu-info,services'" \
  -o limiz.exe ./cmd/limiz/
```

`collectors/plugin_verify.go` içinde:

```go
package collectors

import "strings"

// AllowedPlugins derleme zamanında -ldflags ile set edilir.
// Boş bırakılırsa plugin sistemi tamamen devre dışı kalır.
var AllowedPlugins string // örn: "dir-size,gpu-info,services"

func isAllowedPlugin(name string) bool {
    if AllowedPlugins == "" {
        return false // allowlist boşsa hiçbir plugin çalışmaz
    }
    for _, allowed := range strings.Split(AllowedPlugins, ",") {
        if strings.TrimSpace(allowed) == name {
            return true
        }
    }
    return false
}
```

**Sonuç:** Config'e `"name": "evil-tool"` eklense bile binary içindeki list'te olmadığı
için reddedilir.

---

### 9.3 Katman 2 — Ed25519 Dijital İmza

Her plugin binary'si özel anahtar ile imzalanır. Limiz, binary'yi çalıştırmadan önce
derleme zamanında içine gömülmüş public key ile imzayı doğrular.

#### Neden Ed25519?

| Algoritma | Anahtar Boyu | İmza Boyu | Hız |
|-----------|-------------|-----------|-----|
| RSA-2048 | 256 byte | 256 byte | Yavaş |
| ECDSA-P256 | 32 byte | 64 byte | Orta |
| **Ed25519** | **32 byte** | **64 byte** | **Çok hızlı** |

Ed25519, timing saldırılarına karşı dirençlidir ve Go standard library'de
(`crypto/ed25519`) mevcuttur — ek bağımlılık gerekmez.

#### İmzalama Mantığı

```
İmzalanan şey: SHA256(plugin_binary_dosyası)
İmza: Ed25519Sign(privateKey, sha256_hash)
İmza dosyası: plugins/metric/dir-size.sig   (64 byte, raw binary)
              plugins/metric/dir-size.exe.sig  (Windows)
```

SHA256 hash binary'ye uygulanır, imza da o hash'e uygulanır.
Bu sayede büyük dosyayı bellekte tutmak yerine hash'i imzalamak yeterlidir.

---

### 9.4 Güvenlik Katmanı — Dosyalar

```
internal/collectors/
├── plugin_verify.go      ← allowlist + imza doğrulama
└── plugin_exec.go        ← Collect() içinde verify çağrısı

cmd/sign-plugin/
└── main.go               ← plugin imzalama CLI aracı

keys/                     ← GİT'E EKLENMEMELİ (.gitignore)
├── plugin-signing.key    ← Özel anahtar (sadece CI/CD'de)
└── plugin-signing.pub    ← Public anahtar (binary'ye gömülür)
```

---

### 9.5 `internal/collectors/plugin_verify.go` — Davranış Tanımı

```go
package collectors

import (
    "crypto/ed25519"
    "crypto/sha256"
    "encoding/base64"
    "fmt"
    "os"
)

// EmbeddedPublicKey derleme zamanında -ldflags ile set edilir (base64).
var EmbeddedPublicKey string

// VerifyPlugin: allowlist + imza doğrulaması yapar.
// Hata döndürürse plugin çalıştırılmamalıdır.
func VerifyPlugin(name, binaryPath string) error {
    // Katman 1: allowlist kontrolü
    if !isAllowedPlugin(name) {
        return fmt.Errorf("plugin '%s' allowlist'te yok (derleme zamanında tanımlanmamış)", name)
    }

    // Katman 2: imza doğrulaması
    if EmbeddedPublicKey == "" {
        return fmt.Errorf("public key binary'ye gömülmemiş (ldflags eksik)")
    }

    pubKeyBytes, err := base64.StdEncoding.DecodeString(EmbeddedPublicKey)
    if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
        return fmt.Errorf("geçersiz public key formatı")
    }

    // Binary içeriğini oku ve hash'le
    data, err := os.ReadFile(binaryPath)
    if err != nil {
        return fmt.Errorf("plugin binary okunamadı: %v", err)
    }
    hash := sha256.Sum256(data)

    // .sig dosyasını oku
    sigPath := binaryPath + ".sig"
    sig, err := os.ReadFile(sigPath)
    if err != nil {
        return fmt.Errorf("imza dosyası bulunamadı (%s): %v", sigPath, err)
    }
    if len(sig) != ed25519.SignatureSize {
        return fmt.Errorf("geçersiz imza boyutu: %d", len(sig))
    }

    // İmzayı doğrula
    if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), hash[:], sig) {
        return fmt.Errorf("imza doğrulaması BAŞARISIZ: plugin '%s' yetkisiz veya değiştirilmiş", name)
    }

    return nil
}
```

`plugin_exec.go` içinde `Collect()` başında:

```go
func (p *ExecPlugin) Collect() []Metric {
    binaryPath := p.resolveBinary()

    if err := VerifyPlugin(p.item.Name, binaryPath); err != nil {
        log.Printf("[SECURITY] Plugin reddedildi: %v", err)
        return nil
    }

    // ... subprocess çalıştır
}
```

---

### 9.6 `cmd/sign-plugin/main.go` — İmzalama Aracı

```
Kullanım:
  sign-plugin --key keys/plugin-signing.key plugins/metric/dir-size
  sign-plugin --key keys/plugin-signing.key plugins/metric/dir-size.exe
  sign-plugin --key keys/plugin-signing.key --all plugins/

Çıktı:
  plugins/metric/dir-size.sig      (oluşturuldu)
  plugins/metric/dir-size.exe.sig  (oluşturuldu)
```

**Davranış:**
1. Binary dosyasını oku → SHA256 hesapla
2. Ed25519 private key ile imzala
3. `<binary>.sig` dosyasına yaz (64 byte)
4. Özet çıktısı: `SIGNED dir-size  sha256=abc123...  sig=def456...`

**Anahtar üretimi (bir kez yapılır):**

```bash
# Linux/macOS
go run ./cmd/sign-plugin --gen-key keys/plugin-signing

# Çıktı:
#   keys/plugin-signing.key  (özel anahtar — güvenli tut!)
#   keys/plugin-signing.pub  (public anahtar — binary'ye gömülür)
```

---

### 9.7 Derleme Süreci — Tam Akış

```
┌─────────────────────────────────────────────────────────┐
│                  GELİŞTİRİCİ / CI-CD                    │
│                                                          │
│  1. Plugin kaynak kodunu yaz                             │
│     plugins-src/new-plugin/main.go                       │
│                                                          │
│  2. Plugin'i derle                                       │
│     GOOS=windows go build -o plugins/new-plugin.exe ...  │
│                                                          │
│  3. Plugin'i imzala                                      │
│     sign-plugin --key keys/plugin-signing.key \          │
│                 plugins/new-plugin.exe                   │
│     → plugins/new-plugin.exe.sig oluşur                  │
│                                                          │
│  4. Limiz'i yeni plugin ismiyle derle                    │
│     PUB=$(base64 -w0 keys/plugin-signing.pub)            │
│     go build \                                           │
│       -ldflags "-X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=    │
│                     dir-size,gpu-info,services,          │
│                     new-plugin'                          │
│                  -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey= │
│                     $PUB'" \                             │
│       -o limiz.exe .                                     │
│                                                          │
│  5. Dağıt: limiz.exe + plugins/*.exe + plugins/*.sig     │
└─────────────────────────────────────────────────────────┘
```

---

### 9.8 Saldırı Senaryoları ve Korunma

| Senaryo | Sonuç |
|---------|-------|
| Config'e yeni plugin ismi eklendi | ❌ Allowlist'te yok → reddedilir |
| Plugin binary dosyası değiştirildi | ❌ SHA256 değişti → imza doğrulaması başarısız |
| Başka bir .sig dosyası kopyalandı | ❌ İmza başka binary için geçerli → hash uyuşmuyor |
| .sig dosyası silindi | ❌ İmza dosyası yok → reddedilir |
| Hem binary hem .sig değiştirildi | ❌ Private key olmadan geçerli imza üretilemez |
| Config `plugins.dir` değiştirildi | ❌ Yeni dizindeki binary imzasız → reddedilir |
| Private key çalındı | ⚠️ Kritik — key rotation gerekir, yeni binary dağıtılır |

---

### 9.9 exec.Command Güvenliği

**Doğru (shell injection yok):**
```go
exec.Command(binaryPath, "--collect", "--path", userConfigArg)
```

**Yanlış (shell injection riski):**
```go
exec.Command("sh", "-c", binaryPath + " --path " + userConfigArg)
```

Interpreter plugin'leri (Python, PowerShell) için de aynı kural geçerlidir.
İmza doğrulaması `exec` argümanına (interpreter) değil,
script dosyasının SHA256'sına uygulanmalıdır:

```json
{
  "name": "my-script",
  "exec": "python3",
  "script": "plugins/my-script.py",   ← imzalanan dosya
  "args": ["--path", "/var/log"]
}
```

---

### 9.10 Özel Anahtar Yönetimi

| Kural | Açıklama |
|-------|----------|
| `keys/plugin-signing.key` git'e eklenmemeli | `.gitignore`'a ekle |
| CI/CD ortamında secret olarak saklanmalı | GitHub Secret, Vault, vb. |
| Local geliştirme için ayrı key kullanılabilir | dev vs prod key pair |
| Key rotation: | Yeni key üret → tüm plugin'leri yeniden imzala → limiz'i yeniden derle |

---

## 10. Dizin Yapısı — Yayın Paketi

```
C:\Program Files\limiz\               /opt/limiz/
├── limiz.exe                          ├── limiz
│   (allowlist + pubkey gömülü)        │   (allowlist + pubkey gömülü)
├── config.json                        ├── config.json
├── metrics.db                         ├── metrics.db
└── plugins\                           └── plugins/
    ├── dir-size.exe                       ├── dir-size
    ├── dir-size.exe.sig   ← imza          ├── dir-size.sig
    ├── gpu-info.exe                       ├── gpu-info
    ├── gpu-info.exe.sig   ← imza          ├── gpu-info.sig
    ├── services.exe                       ├── services
    └── services.exe.sig   ← imza          └── services.sig
```

### setup.exe Entegrasyonu

`setup.exe` kurulum sırasında `plugins\metric\` ve `plugins\data\` dizinlerini otomatik oluşturur:

```
C:\Program Files\limiz\
  limiz.exe
  plugins\metric\          ← metrics plugin'leri buraya kopyalanır
    dir-size.exe
    dir-size.exe.sig
  plugins\data\            ← data plugin'leri buraya kopyalanır
    folder-size.exe
    folder-size.exe.sig
  config.json
```

---

## 11. Uygulama Adımları (Sıralı)

| # | Dosya | İşlem | Açıklama |
|---|-------|--------|----------|
| 1 | `internal/collectors/plugin_config.go` | Yeni | `PluginsConfig`, `PluginItem` struct tanımları |
| 2 | `internal/collectors/plugin_parser.go` | Yeni | Prometheus text output parser |
| 3 | `internal/collectors/plugin_verify.go` | Yeni | Allowlist + Ed25519 imza doğrulama |
| 4 | `internal/collectors/plugin_exec.go` | Yeni | `ExecPlugin` collector — verify sonrası exec |
| 5 | `cmd/sign-plugin/main.go` | Yeni | Key üretme + plugin imzalama CLI |
| 6 | `cmd/limiz/main.go` | ~12 satır ekleme | `Config.Plugins` alanı + plugin kayıt bloğu |
| 7 | `plugins-src/dir-size/main.go` | Yeni | Dir-size örnek plugin |
| 8 | `plugins-src/gpu-info/main.go` | Yeni | GPU info örnek plugin |
| 9 | `plugins-src/services/main.go` | Yeni | Services örnek plugin |
| 10 | `Makefile` | Yeni | `build-plugins`, `sign-plugins`, `build-limiz` hedefleri |
| 11 | `.gitignore` | Güncelle | `keys/*.key` ekle |

**Mevcut kod değişikliği:** Yalnızca `main.go`'ya ~12 satır ekleme.
Registry, collectors, localwriter, winsvc — hiçbiri değişmez.

---

## 12. Makefile — Tam Derleme Süreci

```makefile
PRIVKEY := keys/plugin-signing.key
PUBKEY  := keys/plugin-signing.pub

# 1. İlk kurulum: anahtar çifti üret
gen-keys:
	go run ./cmd/sign-plugin --gen-key keys/plugin-signing

# 2. Plugin'leri derle
build-plugins:
	GOOS=linux   GOARCH=amd64 go build -o plugins/metric/dir-size       ./plugins-src/dir-size/
	GOOS=windows GOARCH=amd64 go build -o plugins/metric/dir-size.exe   ./plugins-src/dir-size/
	GOOS=linux   GOARCH=amd64 go build -o plugins/metric/gpu-info       ./plugins-src/gpu-info/
	GOOS=windows GOARCH=amd64 go build -o plugins/metric/gpu-info.exe   ./plugins-src/gpu-info/
	GOOS=linux   GOARCH=amd64 go build -o plugins/metric/services       ./plugins-src/services/
	GOOS=windows GOARCH=amd64 go build -o plugins/metric/services.exe   ./plugins-src/services/

# 3. Plugin'leri imzala
sign-plugins: build-plugins
	go run ./cmd/sign-plugin --key $(PRIVKEY) --all plugins/

# 4. Limiz'i allowlist + public key gömülü olarak derle
ALLOWED_PLUGINS := dir-size,gpu-info,services
PUB_KEY         := $(shell base64 -w0 $(PUBKEY))

build-limiz: sign-plugins
	GOOS=linux GOARCH=amd64 go build \
	  -ldflags "-X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=$(ALLOWED_PLUGINS)' \
	            -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=$(PUB_KEY)'" \
	  -o limiz ./cmd/limiz/
	GOOS=windows GOARCH=amd64 go build \
	  -ldflags "-X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=$(ALLOWED_PLUGINS)' \
	            -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=$(PUB_KEY)'" \
	  -o limiz.exe ./cmd/limiz/

# Tam release: her şeyi sırayla çalıştır
release: sign-plugins build-limiz
```
