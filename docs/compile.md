# Limiz — Derleme Kılavuzu

Bu belge, Limiz'in plugin imzalama altyapısını ve Linux / Windows için derleme adımlarını açıklar.

---

## Plugin İmzalama Sistemi

Limiz, güvenlik için iki katmanlı bir plugin doğrulama mekanizması kullanır.

### Nasıl Çalışır?

1. **Derleme-zamanı allowlist** — hangi plugin adlarının yüklenebileceği, binary'e `-ldflags` ile gömülür. Config'de tanımlı olsa bile allowlist'te olmayan bir plugin yüklenmez.

2. **Ed25519 dijital imza** — her plugin binary'si için bir `.sig` dosyası üretilir. Limiz, plugin'i yüklemeden önce:
   - Plugin binary'sinin SHA256 hash'ini hesaplar.
   - `<plugin>.sig` dosyasını okur.
   - Derleme zamanında gömülmüş public key ile imzayı doğrular.
   - Doğrulama başarısız olursa plugin kesinlikle çalıştırılmaz.

### Anahtar Çifti Yönetimi

Tek bir anahtar çifti hem metric plugin'leri hem de data plugin'leri için kullanılır. Anahtar çifti **bir kez** üretilir ve tüm sürümler boyunca korunur. Anahtar çifti değişirse eski plugin'lerin `.sig` dosyaları geçersiz kalır ve yeniden imzalanmaları gerekir.

```
keys/
  plugin-signing.key   ← Ed25519 private key (64 byte, git'e ekleme!)
  plugin-signing.pub   ← Ed25519 public key  (32 byte, binary'e gömülür)
```

> **Önemli:** `plugin-signing.key` dosyasını asla git'e ekleme. `.gitignore`'a ekle.

### Anahtar Çifti Üretme (ilk kurulum)

```bash
go run ./cmd/sign-plugin --gen-key keys/plugin-signing
```

### Plugin İmzalama

Tek bir binary imzalamak için:

```bash
go run ./cmd/sign-plugin --key keys/plugin-signing.key plugins/metric/dir-size
go run ./cmd/sign-plugin --key keys/plugin-signing.key --all plugins/data/
```

Her binary için yanına `<binary>.sig` dosyası oluşturulur. Bu dosyalar plugin binary'siyle birlikte dağıtılmalıdır.

### Sürüm Uyumluluğu

| Durum | Sonuç |
|---|---|
| Eski plugin + yeni Limiz (aynı anahtar çifti) | Çalışır — binary değişmedi, `.sig` hâlâ geçerli |
| Eski plugin + yeni Limiz (yeni anahtar çifti) | Reddedilir — eski `.sig` yeni public key ile doğrulanamaz |
| Plugin binary değişti, `.sig` güncellenmedi | Reddedilir — hash eşleşmiyor |

---

## Linux için Derleme

### Gereksinimler

- Go 1.22+
- CGO gerekmez (`CGO_ENABLED=0` ile derlenir)

### Limiz Binary

```bash
PUB_KEY=$(base64 -w0 keys/plugin-signing.pub)

CGO_ENABLED=0 go build \
  -ldflags "-s -w \
    -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=${PUB_KEY}' \
    -X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=dir-size,gpu-info,services' \
    -X 'github.com/limanmys/limiz/internal/datas.AllowedDataPlugins=folder-size'" \
  -o limiz \
  ./cmd/limiz/
```

`AllowedPlugins` ve `AllowedDataPlugins` değerlerini dağıtımda izin vermek istediğin plugin adlarıyla güncelle (virgülle ayrılmış).

### Plugin'leri Derleme

Her plugin ayrı bir Go modülüdür. Örnek:

```bash
go build -o plugins/metric/dir-size ./plugins-src/dir-size/
go build -o plugins/data/folder-size ./plugins-src/folder-size/
```

### Dağıtım Dosyaları

```
limiz
plugins/metric/
  dir-size
  dir-size.sig
plugins/data/
  folder-size
  folder-size.sig
/etc/limiz/config.json
```

---

## Windows için Derleme

Windows derlemesi Linux üzerinden çapraz derleme (`cross-compile`) ile yapılır.

### Gereksinimler

- Go 1.22+
- `mingw-w64` (`gcc` Windows için cross-compile desteği)

```bash
# Ubuntu/Debian
sudo apt install gcc-mingw-w64-x86-64
```

### Limiz Binary (limiz.exe)

```bash
PUB_KEY=$(base64 -w0 keys/plugin-signing.pub)

CGO_ENABLED=1 \
CC=x86_64-w64-mingw32-gcc \
GOOS=windows \
GOARCH=amd64 \
go build \
  -ldflags "-s -w \
    -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=${PUB_KEY}' \
    -X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=dir-size,gpu-info,services' \
    -X 'github.com/limanmys/limiz/internal/datas.AllowedDataPlugins=folder-size'" \
  -o limiz.exe \
  ./cmd/limiz/
```

### Setup Binary (setup.exe)

`setup.exe`, `limiz.exe`'yi kendi içinde barındırır. Bu nedenle önce `limiz.exe` derlenmeli, ardından `cmd/setup/` dizinine kopyalanmalıdır.

```bash
# 1. limiz.exe'yi cmd/setup/ dizinine kopyala
cp limiz.exe cmd/setup/limiz.exe

# 2. setup.exe'yi derle (limiz.exe gömülü gelir)
CGO_ENABLED=0 \
GOOS=windows \
GOARCH=amd64 \
go build \
  -ldflags "-s -w -H=windowsgui" \
  -o setup.exe \
  ./cmd/setup/
```

> `-H=windowsgui` bayrağı, kurulum sırasında arka planda bir terminal penceresi açılmasını engeller.

`setup.exe` çalıştırıldığında:
1. Admin yetkisi kontrol eder.
2. `C:\Program Files\limiz\` ve `C:\Program Files\limiz\plugins\` dizinlerini oluşturur.
3. Mevcut servisi durdurur (güncelleme senaryosu — servis kaydı korunur).
4. Gömülü `limiz.exe`'yi `C:\Program Files\limiz\` altına yazar.
5. `C:\Program Files\limiz\base.config.json` dosyasını boş (`{}`) olarak oluşturur (yoksa); varsa korur.
6. Servisi `--config C:\Program Files\limiz\config.json` ile kurar ve başlatır.
7. İlk çalışmada `config.json` yoksa servis `http://localhost:9110/configuration` adresinden web UI ile yapılandırılmayı bekler.

### Plugin'leri Windows İçin Derleme

```bash
CGO_ENABLED=0 \
GOOS=windows \
GOARCH=amd64 \
go build -o plugins/metric/dir-size.exe ./plugins-src/dir-size/
```

İmzalama Linux'taki gibi yapılır — `sign-plugin` aynı `keys/plugin-signing.key` dosyasını kullanır:

```bash
go run ./cmd/sign-plugin --key keys/plugin-signing.key plugins/metric/dir-size.exe
go run ./cmd/sign-plugin --key keys/plugin-signing.key plugins/data/folder-size.exe
```

### Dağıtım Dosyaları

**Tek dosya kurulum:**
```
setup.exe          ← limiz.exe gömülü; bunu kullanıcıya gönder
```

**Manuel kurulum:**
```
limiz.exe
plugins\metric\
  dir-size.exe
  dir-size.exe.sig
plugins\data\
  folder-size.exe
  folder-size.exe.sig
config.json
```

---

## Özet: Tüm Adımlar

```bash
# 1. Anahtar çifti üret (sadece ilk kez)
go run ./cmd/sign-plugin --gen-key keys/plugin-signing

# 2. Plugin'leri derle ve imzala
go build -o plugins/metric/dir-size ./plugins-src/dir-size/
go build -o plugins/data/folder-size ./plugins-src/folder-size/
go run ./cmd/sign-plugin --key keys/plugin-signing.key --all plugins/metric/
go run ./cmd/sign-plugin --key keys/plugin-signing.key --all plugins/data/

# 3a. Linux binary'sini derle
PUB_KEY=$(base64 -w0 keys/plugin-signing.pub)
CGO_ENABLED=0 go build \
  -ldflags "-s -w \
    -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=${PUB_KEY}' \
    -X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=dir-size,gpu-info' \
    -X 'github.com/limanmys/limiz/internal/datas.AllowedDataPlugins=folder-size'" \
  -o limiz ./cmd/limiz/

# 3b. Windows binary'lerini derle
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 \
  go build \
    -ldflags "-s -w \
      -X 'github.com/limanmys/limiz/internal/signing.EmbeddedPublicKey=${PUB_KEY}' \
      -X 'github.com/limanmys/limiz/internal/collectors.AllowedPlugins=dir-size,gpu-info' \
      -X 'github.com/limanmys/limiz/internal/datas.AllowedDataPlugins=folder-size'" \
    -o limiz.exe ./cmd/limiz/
cp limiz.exe cmd/setup/limiz.exe
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H=windowsgui" -o setup.exe ./cmd/setup/
```
