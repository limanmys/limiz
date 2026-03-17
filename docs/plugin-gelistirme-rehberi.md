# Limiz — Yeni Plugin Geliştirme Rehberi

Bu rehber, limiz'e sıfırdan yeni bir plugin eklemenin **adım adım** sürecini anlatır.
Örnek olarak `dir-size` plugin'i kullanılmaktadır.

---

## Ön Koşullar

- Go 1.22+
- Proje kök dizininde `keys/plugin-signing.key` mevcut olmalı
  (yoksa ilk kurulum adımını izleyin)
- `sign-plugin` aracı derlenmiş olmalı

---

## 0. İlk Kurulum: Anahtar Çifti (Bir Kez Yapılır)

```bash
# Geliştirme ortamında bir kez çalıştırılır
go run ./cmd/sign-plugin --gen-key keys/plugin-signing

# Çıktı:
#   keys/plugin-signing.key  ← GİT'E EKLEMEYİN
#   keys/plugin-signing.pub  ← binary'ye gömülür
```

`.gitignore`'a ekleyin:
```
keys/*.key
```

---

## 1. Plugin Kaynak Kodunu Yaz

`plugins-src/<isim>/main.go` dosyası oluşturun.

### Zorunlu Kurallar

| Kural | Açıklama |
|-------|----------|
| `--collect` flag'i | Limiz tarafından her zaman bu argümanla çağrılır |
| `plugin_` öneki | Tüm metrik isimleri `plugin_` ile başlamalıdır |
| stdout → metrikler | Prometheus exposition format; yalnızca metrikler |
| stderr → loglar | Hatalar ve uyarılar stderr'e yazılır |
| Exit 0 = başarı | Non-zero exit → limiz metrikleri almaz, uyarı loglar |

### Temel Şablon

```go
package main

import (
    "flag"
    "fmt"
    "os"
)

type pathList []string
func (p *pathList) String() string        { return "" }
func (p *pathList) Set(v string) error    { *p = append(*p, v); return nil }

func main() {
    var paths pathList
    collect  := flag.Bool("collect",  false, "Metrikleri topla")
    describe := flag.Bool("describe", false, "Meta veri göster")
    flag.Var(&paths, "path", "Hedef (tekrarlanabilir)")
    flag.Parse()

    switch {
    case *describe:
        fmt.Println(`{"name":"myplugin","version":"1.0.0"}`)
    case *collect:
        if err := collectMetrics(paths); err != nil {
            fmt.Fprintln(os.Stderr, "hata:", err)
            os.Exit(1)
        }
    default:
        fmt.Fprintln(os.Stderr, "Kullanım: myplugin --collect [args]")
        os.Exit(2)
    }
}

func collectMetrics(paths []string) error {
    fmt.Println("# HELP plugin_my_metric Açıklama")
    fmt.Println("# TYPE plugin_my_metric gauge")
    for _, p := range paths {
        value := 42.0 // gerçek hesaplama buraya
        fmt.Printf("plugin_my_metric{path=\"%s\"} %g\n", p, value)
    }
    return nil
}
```

---

## 2. Plugin'i Manuel Olarak Test Et

Limiz'e dahil etmeden önce plugin'i doğrudan çalıştırın:

```bash
# Linux'ta derle ve test et
go run ./plugins-src/dir-size/ --collect --path /var/log --path /tmp

# Beklenen çıktı:
# HELP plugin_dir_size_bytes Dizin toplam boyutu (bytes)
# TYPE plugin_dir_size_bytes gauge
plugin_dir_size_bytes{path="/var/log"} 4.29496729e+09
plugin_dir_size_bytes{path="/tmp"} 1.048576e+06

# HELP plugin_dir_file_count Dizindeki toplam dosya sayısı
# TYPE plugin_dir_file_count gauge
plugin_dir_file_count{path="/var/log"} 1247
plugin_dir_file_count{path="/tmp"} 38
```

---

## 3. Plugin'i Derle

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o plugins/metric/dir-size ./plugins-src/dir-size/

# Windows
GOOS=windows GOARCH=amd64 go build -o plugins/metric/dir-size.exe ./plugins-src/dir-size/
```

---

## 4. Plugin'i İmzala

```bash
# Linux binary
go run ./cmd/sign-plugin \
  --key keys/plugin-signing.key \
  plugins/metric/dir-size

# Windows binary
go run ./cmd/sign-plugin \
  --key keys/plugin-signing.key \
  plugins/metric/dir-size.exe

# Veya tüm plugins/ klasörünü tek seferde imzala
go run ./cmd/sign-plugin \
  --key keys/plugin-signing.key \
  --all plugins/metric/

# Çıktı:
# SIGNED  dir-size                             sha256=121cf026...
# SIGNED  dir-size.exe                         sha256=5a3b7f91...
```

İmza başarılıysa `plugins/metric/dir-size.sig` ve `plugins/metric/dir-size.exe.sig` dosyaları oluşur.

---

## 5. Limiz'i Yeni Plugin Adıyla Yeniden Derle

Plugin ismi derleme zamanında binary'ye gömülür. Yeni bir plugin eklendiğinde
limiz **mutlaka yeniden derlenmeli**dir.

```bash
# Mevcut public key'i base64'e çevir
PUB_KEY=$(base64 -w0 keys/plugin-signing.pub)

# Onaylı plugin listesini güncelle (yeni plugin eklendi)
ALLOWED="dir-size,gpu-info,services"   # ← yeni plugin ismini buraya ekle

# Linux
go build \
  -ldflags "-X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=$ALLOWED' \
            -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=$PUB_KEY'" \
  -o limiz ./cmd/limiz/

# Windows
GOOS=windows GOARCH=amd64 go build \
  -ldflags "-X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=$ALLOWED' \
            -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=$PUB_KEY'" \
  -o limiz.exe ./cmd/limiz/
```

---

## 6. config.json Güncelle

```json
{
  "listen_address": ":9110",
  "plugins": {
    "enabled": true,
    "dir": "/usr/lib/limiz/plugins/metric",
    "default_timeout": "10s",
    "items": [
      {
        "name": "dir_size",
        "exec": "dir-size",
        "args": ["--path", "/var/log", "--path", "/var/lib/limiz"],
        "timeout": "30s",
        "enabled": true
      }
    ]
  }
}
```

**Windows için:**
```json
{
  "plugins": {
    "enabled": true,
    "dir": "C:/Program Files/limiz/plugins/metric",
    "default_timeout": "10s",
    "items": [
      {
        "name": "dir_size",
        "exec": "dir-size",
        "args": ["--path", "C:/inetpub/wwwroot", "--path", "C:/logs"],
        "timeout": "30s",
        "enabled": true
      }
    ]
  }
}
```

> **Önemli:** `"name"` alanı, derleme zamanında `-ldflags`'e verdiğiniz isimle
> **birebir eşleşmelidir**. Farklı olursa plugin güvenlik kontrolünden geçemez.

---

## 7. Çalışmayı Doğrula

```bash
# Limiz'i başlat
./limiz --config /opt/limiz/config.json

# Log çıktısında şunu görmelisiniz:
# Plugin yüklendi: dir_size (dir-size)

# Metrics endpoint'i kontrol et
curl -s http://localhost:9110/metrics | grep plugin_dir
```

Beklenen çıktı:
```
# HELP plugin_dir_size_bytes Dizin toplam boyutu (bytes)
# TYPE plugin_dir_size_bytes gauge
plugin_dir_size_bytes{path="/var/log"} 4.29496729e+09
```

---

## 8. Olası Hatalar ve Çözümleri

| Hata | Neden | Çözüm |
|------|-------|-------|
| `plugin 'X' allowlist'te tanımlı değil` | Plugin ismi ldflags'e eklenmemiş | Limiz'i yeni isimle yeniden derle |
| `imza dosyası bulunamadı` | `.sig` dosyası yok | `sign-plugin` ile imzala |
| `imza doğrulaması BAŞARISIZ` | Binary değişmiş veya yanlış key kullanılmış | Binary'yi yeniden derle ve imzala |
| `geçersiz EmbeddedPublicKey` | `-ldflags` eksik | Derleme komutuna `-X EmbeddedPublicKey=...` ekle |
| `timeout aşıldı` | Plugin çok yavaş | `timeout` değerini artır veya hedef dizini küçült |
| `metrik üretilmedi` | Plugin çalıştı ama stdout boş | Plugin'i `--collect` ile manuel test et |

---

## 9. Makefile ile Tam Otomasyon

Projeye bir `Makefile` ekleyerek tüm süreci tek komuta indirgebilirsiniz:

```makefile
PRIVKEY         := keys/plugin-signing.key
PUBKEY          := keys/plugin-signing.pub
ALLOWED_PLUGINS := dir-size,gpu-info,services
PUB_KEY         := $(shell base64 -w0 $(PUBKEY) 2>/dev/null)

.PHONY: gen-keys build-plugins sign-plugins build-limiz release

gen-keys:
	go run ./cmd/sign-plugin --gen-key keys/plugin-signing

build-plugins:
	GOOS=linux   GOARCH=amd64 go build -o plugins/metric/dir-size     ./plugins-src/dir-size/
	GOOS=windows GOARCH=amd64 go build -o plugins/metric/dir-size.exe ./plugins-src/dir-size/

sign-plugins: build-plugins
	go run ./cmd/sign-plugin --key $(PRIVKEY) --all plugins/metric/

build-limiz: sign-plugins
	GOOS=linux GOARCH=amd64 go build \
	  -ldflags "-X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=$(ALLOWED_PLUGINS)' \
	            -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=$(PUB_KEY)'" \
	  -o limiz ./cmd/limiz/
	GOOS=windows GOARCH=amd64 go build \
	  -ldflags "-X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=$(ALLOWED_PLUGINS)' \
	            -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=$(PUB_KEY)'" \
	  -o limiz.exe ./cmd/limiz/

# Yeni plugin eklenince: make release
release: sign-plugins build-limiz
	@echo "Hazır: limiz + plugins/ + .sig dosyaları"
```

Kullanım:
```bash
# İlk kurulum
make gen-keys

# Yeni plugin ekleyince
make release

# Sadece plugin derle + imzala
make sign-plugins
```

---

## 10. Özet: Yeni Plugin Kontrol Listesi

- [ ] `plugins-src/<isim>/main.go` oluşturuldu
- [ ] `--collect` argümanı destekleniyor
- [ ] Tüm metrik isimleri `plugin_` önekiyle başlıyor
- [ ] `go run ./plugins-src/<isim>/ --collect` çıktısı doğru
- [ ] `plugins/<isim>` ve `plugins/<isim>.exe` derlendi
- [ ] `plugins/<isim>.sig` ve `plugins/<isim>.exe.sig` imzalandı
- [ ] `ALLOWED_PLUGINS`'e isim eklendi ve limiz yeniden derlendi
- [ ] `config.json`'da `plugins.items[]` güncellendi
- [ ] `curl .../metrics | grep plugin_` çıktısı doğrulandı
