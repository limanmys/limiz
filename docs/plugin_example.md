# Limiz — Data Plugin Geliştirme Rehberi

Bu belge, Limiz `/datas` endpoint'ine veri sağlayan bir data plugin'in nasıl yazılacağını, derleneceğini, imzalanacağını ve yapılandırılacağını adım adım açıklar.

Hazır örnek: [`plugins-src/folder-size/main.go`](../plugins-src/folder-size/main.go)

---

## Metrics Plugin ile Farkı

| | Metrics Plugin | Data Plugin |
|---|---|---|
| Endpoint | `/metrics` | `/datas` |
| Çıktı formatı | Prometheus exposition text | JSON |
| Config alanı | `plugins` | `datas.plugins` |
| ldflags değişkeni | `signing.EmbeddedPublicKey` / `collectors.AllowedPlugins` | `signing.EmbeddedPublicKey` / `datas.AllowedDataPlugins` |

---

## 1. Plugin Yazımı

Data plugin, bağımsız bir Go binary'sidir. İki zorunlu flag desteklemelidir:

| Flag | Davranış |
|---|---|
| `--collect` | stdout'a JSON yaz |
| `--describe` | plugin meta verisini JSON olarak yaz |

Limiz, plugin'i `--collect [args...]` ile çalıştırır ve stdout'tan okuduğu JSON'ı `/datas` yanıtına `"plugin-adı": <veri>` şeklinde ekler.

### Minimal örnek

```go
package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "os"
)

func main() {
    collect  := flag.Bool("collect",  false, "Verileri topla")
    describe := flag.Bool("describe", false, "Meta veri göster")
    flag.Parse()

    switch {
    case *describe:
        fmt.Println(`{
  "name": "my-plugin",
  "type": "data",
  "version": "1.0.0",
  "description": "Örnek data plugin",
  "platform": ["linux", "windows"]
}`)

    case *collect:
        result := map[string]any{
            "status": "ok",
            "value":  42,
        }
        out, _ := json.MarshalIndent(result, "", "  ")
        fmt.Println(string(out))

    default:
        fmt.Fprintln(os.Stderr, "Kullanım: my-plugin --collect")
        os.Exit(2)
    }
}
```

### `folder-size` örneği

`folder-size` plugin'i `--path` argümanlarıyla dizin boyutlarını döndürür:

```
folder-size --collect --path /var/log --path /tmp
```

```json
[
  {"path": "/var/log", "size_bytes": 1073741824, "size_human": "1.00 GB", "file_count": 1247},
  {"path": "/tmp",     "size_bytes": 524288,     "size_human": "512.00 KB", "file_count": 42}
]
```

---

## 2. Derleme

Data plugin'ler `plugins-src/` altında ayrı dizinlerde tutulur.

### Linux

```bash
go build -o plugins/data/folder-size ./plugins-src/folder-size/
```

### Windows (Linux üzerinden cross-compile)

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -o plugins/data/folder-size.exe ./plugins-src/folder-size/
```

---

## 3. İmzalama

Data plugin'ler, metrics plugin'lerle **aynı anahtar çifti** ile imzalanır. `sign-plugin` aracı her iki plugin türü için de kullanılır.

### Anahtar çifti üretme (ilk kez)

```bash
go run ./cmd/sign-plugin --gen-key keys/plugin-signing
```

Üretilen dosyalar:

```
keys/
  plugin-signing.key   ← Ed25519 private key (git'e ekleme!)
  plugin-signing.pub   ← Ed25519 public key  (binary'e gömülür)
```

### Plugin imzalama

Tek binary:
```bash
go run ./cmd/sign-plugin --key keys/plugin-signing.key plugins/data/folder-size
```

Dizindeki tüm binary'ler:
```bash
go run ./cmd/sign-plugin --key keys/plugin-signing.key --all plugins/data/
```

Her binary için yanına `<binary>.sig` dosyası oluşturulur. Bu dosya binary ile birlikte dağıtılmalıdır.

### Sürüm uyumluluğu

Aynı anahtar çifti korunduğu sürece eski plugin binary'leri yeni Limiz sürümleriyle çalışmaya devam eder. Anahtar değişirse tüm plugin'ler yeniden imzalanmalıdır.

---

## 4. Limiz Derlemesi (ldflags)

Data plugin desteği için Limiz binary'sini tek bir public key ile derle:

```bash
PUB_KEY=$(base64 -w0 keys/plugin-signing.pub)

go build \
  -ldflags "-s -w \
    -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=${PUB_KEY}' \
    -X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=dir-size,gpu-info' \
    -X 'github.com/limanmys/limiz/internal/datas.AllowedDataPlugins=folder-size'" \
  -o limiz ./cmd/limiz/
```

Windows için:

```bash
PUB_KEY=$(base64 -w0 keys/plugin-signing.pub)

CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 \
go build \
  -ldflags "-s -w \
    -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=${PUB_KEY}' \
    -X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=dir-size,gpu-info' \
    -X 'github.com/limanmys/limiz/internal/datas.AllowedDataPlugins=folder-size'" \
  -o limiz.exe ./cmd/limiz/
```

> `AllowedDataPlugins` virgülle ayrılmış plugin adlarından oluşur ve derleme zamanında binary'e gömülür. Config'de tanımlı olsa bile bu listedeki olmayan plugin'ler yüklenmez.

---

## 5. Config Yapılandırması

`datas.plugins` bloğu, `/datas` endpoint'ini açan `datas` bloğunun içine yazılır.

**Linux** (`/etc/limiz/config.json`):

```json
"datas": {
  "enabled": true,
  "path": "/datas",
  "cache": {
    "enabled": true,
    "interval": "30s"
  },
  "plugins": {
    "enabled":         true,
    "dir":             "/usr/lib/limiz/plugins/data",
    "default_timeout": "15s",
    "items": [
      {
        "name":    "folder-size",
        "exec":    "folder-size",
        "args":    ["--path", "/var/log", "--path", "/tmp"],
        "timeout": "30s",
        "enabled": true
      }
    ]
  }
}
```

**Windows** (`C:\Program Files\limiz\config.json`):

```json
"datas": {
  "enabled": true,
  "path": "/datas",
  "cache": {
    "enabled": true,
    "interval": "30s"
  },
  "plugins": {
    "enabled":         true,
    "dir":             "C:/Program Files/limiz/plugins/data",
    "default_timeout": "15s",
    "items": [
      {
        "name":    "folder-size",
        "exec":    "folder-size",
        "args":    ["--path", "C:/Logs", "--path", "C:/Temp"],
        "timeout": "30s",
        "enabled": true
      }
    ]
  }
}
```

### Config alanları

| Alan | Açıklama |
|---|---|
| `dir` | Plugin binary'lerinin bulunduğu dizin |
| `default_timeout` | Tüm plugin'ler için varsayılan zaman aşımı |
| `items[].name` | Plugin adı — `AllowedDataPlugins` listesiyle eşleşmeli |
| `items[].exec` | Binary dosya adı (`dir` altında aranır) |
| `items[].args` | `--collect`'e ek geçirilecek argümanlar |
| `items[].timeout` | Bu plugin'e özel zaman aşımı (opsiyonel) |

---

## 6. Doğrulama

Limiz yeniden başlatıldıktan sonra `/datas` endpoint'i plugin adını key olarak içerir:

```json
{
  "timestamp": "2026-03-13T14:30:00+03:00",
  "folder-size": [
    {"path": "/var/log", "size_bytes": 1073741824, "size_human": "1.00 GB", "file_count": 1247},
    {"path": "/tmp",     "size_bytes": 524288,     "size_human": "512.00 KB", "file_count": 42}
  ],
  "os": { ... },
  "hardware": { ... }
}
```

Plugin reddedilirse (allowlist veya imza hatası) log'da görünür:

```
[SECURITY] Data plugin reddedildi [folder-size]: imza doğrulaması BAŞARISIZ
```

---

## Özet: Tüm Adımlar

```bash
# 1. Anahtar çifti üret (ilk kez)
go run ./cmd/sign-plugin --gen-key keys/plugin-signing

# 2. Plugin'i derle
go build -o plugins/data/folder-size ./plugins-src/folder-size/

# 3. Plugin'i imzala
go run ./cmd/sign-plugin --key keys/plugin-signing.key plugins/data/folder-size

# 4. Limiz'i data plugin desteğiyle derle
PUB_KEY=$(base64 -w0 keys/plugin-signing.pub)
go build -ldflags "-s -w -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=${PUB_KEY}' -X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=dir-size' -X 'github.com/limanmys/limiz/internal/datas.AllowedDataPlugins=folder-size'" -o limiz ./cmd/limiz/

# 5. Binary ve imzayı dağıtım dizinine kopyala
cp plugins/data/folder-size      /usr/lib/limiz/plugins/data/
cp plugins/data/folder-size.sig  /usr/lib/limiz/plugins/data/
```
