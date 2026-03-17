# Limiz — TLS Sertifika Yapılandırması

Limiz, HTTPS üzerinden metrik ve veri sunmak için TLS desteği sağlar.
Sertifikalar üç farklı kaynaktan yüklenebilir:

| Öncelik | Kaynak | Platform |
|---------|--------|----------|
| 1 | Dosya yolu (`cert_file` + `key_file`) | Linux, Windows |
| 2 | Windows Sertifika Deposu (`store_name`) | Yalnızca Windows |
| 3 | Otomatik arama (standart konumlar) | Linux, Windows |

> Limiz başlarken sırasıyla bu kaynakları dener. İlk başarılı olan kullanılır.

---

## 1. Config Dosyası Yapısı

### Linux

```json
{
  "tls": {
    "cert_file": "/etc/limiz/tls/server.crt",
    "key_file": "/etc/limiz/tls/server.key"
  }
}
```

### Windows

```json
{
  "tls": {
    "cert_file": "C:/Program Files/limiz/certs/server.crt",
    "key_file": "C:/Program Files/limiz/certs/server.key",
    "store_name": "Limiz"
  }
}
```

| Alan | Açıklama |
|------|----------|
| `cert_file` | Sertifika dosyasının tam yolu (PEM formatı). |
| `key_file` | Özel anahtar dosyasının tam yolu (PEM formatı). |
| `store_name` | Windows Sertifika Deposu'ndaki sertifika adı (Subject CN) veya SHA-1 parmak izi (thumbprint). Yalnızca Windows'ta geçerlidir. |

---

## 2. Yükleme Öncelik Sırası (Fallback)

```
cert_file + key_file dosyaları mevcut mu?
  ├─ Evet → Dosyalardan yükle ✓
  └─ Hayır
       ├─ store_name belirtilmiş mi? (Windows)
       │    ├─ Evet → Sertifika Deposu'nda ara ✓
       │    └─ Hayır → "Limiz" adıyla depoda ara
       └─ Linux: Standart konumlarda ara
            /etc/limiz/tls/server.crt + server.key
            /etc/limiz/server.crt + server.key
            /etc/ssl/certs/limiz.crt + /etc/ssl/private/limiz.key
            /etc/pki/tls/certs/limiz.crt + /etc/pki/tls/private/limiz.key
```

Bu sayede:
- **Dosya yolları verildiyse** ve dosyalar mevcutsa → dosyadan yüklenir.
- **Dosyalar bulunamazsa** → platforma göre otomatik kaynak aranır.
- **Windows'ta** → Sertifika Deposu (Certificate Store) kontrol edilir.
- **Linux'ta** → Standart dosya konumları taranır.

---

## 3. Windows — Sertifika Deposu (Certificate Store) Kullanımı

Windows'ta sertifikalar, işletim sisteminin yerleşik **Certificate Store** altyapısında saklanabilir.
Limiz, `LocalMachine\My` (Kişisel) deposunu önce, ardından `CurrentUser\My` deposunu arar.

Bu yöntemin avantajları:
- Sertifika ve özel anahtar dosya sisteminde açık metin olarak bulunmaz.
- Özel anahtar **non-exportable** olarak işaretlenebilir (ek güvenlik).
- Sertifika yenileme (renewal) merkezi olarak yönetilebilir.
- Windows güvenlik politikaları (ACL) doğrudan uygulanır.

### 3.1 Self-Signed Sertifika Oluşturma (PowerShell)

Yönetici (Administrator) olarak PowerShell açın:

```powershell
# Self-signed sertifika oluştur (5 yıl geçerli)
$cert = New-SelfSignedCertificate `
    -Subject "CN=Limiz" `
    -DnsName "localhost", $env:COMPUTERNAME `
    -CertStoreLocation "Cert:\LocalMachine\My" `
    -KeyExportPolicy NonExportable `
    -KeySpec KeyExchange `
    -KeyLength 2048 `
    -KeyAlgorithm RSA `
    -HashAlgorithm SHA256 `
    -NotAfter (Get-Date).AddYears(5) `
    -TextExtension @("2.5.29.37={text}1.3.6.1.5.5.7.3.1")

# Parmak izini göster
Write-Host "Thumbprint: $($cert.Thumbprint)"
Write-Host "Subject:    $($cert.Subject)"
```

> **Not:** `-KeyExportPolicy NonExportable` özel anahtarın dışarı aktarılmasını engeller.
> Limiz, NCrypt API üzerinden doğrudan Windows'un anahtar deposunu kullanarak
> imzalama yapar — özel anahtarın dışarı aktarılmasına gerek yoktur.

### 3.2 ECDSA Sertifika Oluşturma (PowerShell)

```powershell
$cert = New-SelfSignedCertificate `
    -Subject "CN=Limiz" `
    -DnsName "localhost", $env:COMPUTERNAME `
    -CertStoreLocation "Cert:\LocalMachine\My" `
    -KeyExportPolicy NonExportable `
    -KeyAlgorithm ECDSA_nistP256 `
    -HashAlgorithm SHA256 `
    -NotAfter (Get-Date).AddYears(5) `
    -TextExtension @("2.5.29.37={text}1.3.6.1.5.5.7.3.1")
```

### 3.3 Mevcut PFX/PEM Sertifikayı İçeri Aktarma (PowerShell)

```powershell
# PFX dosyasından içeri aktar
$password = ConvertTo-SecureString -String "PfxŞifresi" -AsPlainText -Force
Import-PfxCertificate `
    -FilePath "C:\sertifikalar\server.pfx" `
    -CertStoreLocation "Cert:\LocalMachine\My" `
    -Password $password
```

PEM formatındaki sertifikaları önce PFX'e dönüştürmeniz gerekir:

```powershell
# OpenSSL ile PEM → PFX dönüşümü (OpenSSL yüklü olmalı)
openssl pkcs12 -export -out server.pfx -inkey server.key -in server.crt
```

### 3.4 GUI ile Sertifika Ekleme (certlm.msc)

1. **Win + R** → `certlm.msc` yazıp Enter (veya `mmc` → Snap-in Ekle → Sertifikalar → Bilgisayar Hesabı).
2. Sol panelde **Kişisel (Personal)** → **Sertifikalar (Certificates)** öğesini seçin.
3. Sağ tıklayın → **Tüm Görevler (All Tasks)** → **İçeri Aktar (Import)**.
4. Sertifika İçeri Aktarma Sihirbazı açılır:
   - **Dosya:** `.pfx` veya `.p12` dosyasını seçin.
   - **Parola:** PFX dosyasının parolasını girin.
   - **Depo:** "Kişisel (Personal)" seçili olduğundan emin olun.
5. **Son** butonuna tıklayın.

> İçeri aktardıktan sonra sertifikanın Subject (Konu) alanındaki CN değerini
> `config.json` içinde `store_name` olarak kullanın.

### 3.5 Sertifikayı Doğrulama (PowerShell)

```powershell
# Depodaki sertifikaları listele
Get-ChildItem Cert:\LocalMachine\My | Format-Table Subject, Thumbprint, NotAfter

# Belirli bir sertifikayı kontrol et
Get-ChildItem Cert:\LocalMachine\My | Where-Object { $_.Subject -like "*Limiz*" }
```

### 3.6 Config Örnekleri (Windows)

**Yalnızca Sertifika Deposu (dosya yolu gerekmez):**

```json
{
  "tls": {
    "store_name": "Limiz"
  }
}
```

**Hem dosya hem depo (fallback):**

```json
{
  "tls": {
    "cert_file": "C:/Program Files/limiz/certs/server.crt",
    "key_file": "C:/Program Files/limiz/certs/server.key",
    "store_name": "Limiz"
  }
}
```

Bu yapılandırmada: dosyalar mevcutsa dosyadan yüklenir; dosyalar bulunamazsa
Sertifika Deposu'nda "Limiz" konusuyla eşleşen sertifika aranır.

**Thumbprint (parmak izi) ile:**

```json
{
  "tls": {
    "store_name": "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
  }
}
```

> Thumbprint 40 karakterlik hex string olarak verilmelidir (boşluklar ve iki nokta üst üste otomatik temizlenir).

### 3.7 Limiz Servis Hesabı İçin Yetkilendirme

Limiz bir Windows servisi olarak çalışıyorsa, servis hesabının özel anahtara
erişim yetkisi olmalıdır:

```powershell
# Sertifikanın parmak izini alın
$thumb = (Get-ChildItem Cert:\LocalMachine\My | Where-Object { $_.Subject -like "*Limiz*" }).Thumbprint

# Özel anahtar dosyasının yolunu bulun
$cert = Get-ChildItem "Cert:\LocalMachine\My\$thumb"
$keyPath = "$env:ProgramData\Microsoft\Crypto\RSA\MachineKeys"
$keyName = $cert.PrivateKey.CspKeyContainerInfo.UniqueKeyContainerName
$keyFile = Join-Path $keyPath $keyName

# LOCAL SERVICE veya NETWORK SERVICE için izin verin
$acl = Get-Acl $keyFile
$rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    "NT AUTHORITY\LOCAL SERVICE", "Read", "Allow")
$acl.AddAccessRule($rule)
Set-Acl $keyFile $acl
```

> **Not:** Limiz varsayılan olarak `LOCAL_SYSTEM` hesabıyla çalışır ve
> `LocalMachine\My` deposundaki tüm özel anahtarlara erişebilir.
> Farklı bir hesapla çalıştırıyorsanız yukarıdaki adımları uygulayın.

---

## 4. Linux — Dosya Tabanlı Sertifika Yapılandırması

Linux'ta işletim sistemi düzeyinde bir "sunucu sertifika deposu" bulunmamaktadır.
Sertifikalar dosya sistemi üzerinde PEM formatında saklanır.

### 4.1 Self-Signed Sertifika Oluşturma (OpenSSL)

```bash
# Dizin oluştur
sudo mkdir -p /etc/limiz/tls
sudo chmod 750 /etc/limiz/tls
sudo chown root:limiz /etc/limiz/tls

# RSA 2048-bit self-signed sertifika (5 yıl)
sudo openssl req -x509 -nodes \
    -newkey rsa:2048 \
    -keyout /etc/limiz/tls/server.key \
    -out /etc/limiz/tls/server.crt \
    -days 1825 \
    -subj "/CN=Limiz" \
    -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"

# İzinleri ayarla
sudo chmod 640 /etc/limiz/tls/server.key
sudo chmod 644 /etc/limiz/tls/server.crt
sudo chown root:limiz /etc/limiz/tls/server.key
```

### 4.2 ECDSA Sertifika Oluşturma

```bash
sudo openssl req -x509 -nodes \
    -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
    -keyout /etc/limiz/tls/server.key \
    -out /etc/limiz/tls/server.crt \
    -days 1825 \
    -subj "/CN=Limiz" \
    -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"

sudo chmod 640 /etc/limiz/tls/server.key
sudo chown root:limiz /etc/limiz/tls/server.key
```

### 4.3 Let's Encrypt / ACME Sertifika Kullanımı

Certbot veya benzeri bir ACME istemcisi ile alınan sertifikalar doğrudan
dosya yolu olarak verilebilir:

```json
{
  "tls": {
    "cert_file": "/etc/letsencrypt/live/monitor.example.com/fullchain.pem",
    "key_file": "/etc/letsencrypt/live/monitor.example.com/privkey.pem"
  }
}
```

> **Not:** Limiz kullanıcısının `/etc/letsencrypt/live/` ve `/etc/letsencrypt/archive/`
> dizinlerine okuma yetkisi olmalıdır:
> ```bash
> sudo chmod 750 /etc/letsencrypt/{live,archive}
> sudo chgrp limiz /etc/letsencrypt/{live,archive}
> ```

### 4.4 Standart Konumlar (Otomatik Arama)

`cert_file` ve `key_file` belirtilmiş ancak dosyalar mevcut değilse,
Limiz aşağıdaki konumları sırayla tarar:

| Sertifika | Özel Anahtar |
|-----------|-------------|
| `/etc/limiz/tls/server.crt` | `/etc/limiz/tls/server.key` |
| `/etc/limiz/server.crt` | `/etc/limiz/server.key` |
| `/etc/ssl/certs/limiz.crt` | `/etc/ssl/private/limiz.key` |
| `/etc/pki/tls/certs/limiz.crt` | `/etc/pki/tls/private/limiz.key` |

Bu konumlardan birine sertifika yerleştirdiyseniz, config dosyasında
sadece TLS'i etkinleştirmeniz yeterlidir:

```json
{
  "tls": {
    "cert_file": "/etc/limiz/tls/server.crt",
    "key_file": "/etc/limiz/tls/server.key"
  }
}
```

**Debian/Ubuntu** sistemlerde:

```bash
sudo cp server.crt /etc/ssl/certs/limiz.crt
sudo cp server.key /etc/ssl/private/limiz.key
sudo chmod 640 /etc/ssl/private/limiz.key
sudo chown root:limiz /etc/ssl/private/limiz.key
```

**RHEL/CentOS/Fedora** sistemlerde:

```bash
sudo cp server.crt /etc/pki/tls/certs/limiz.crt
sudo cp server.key /etc/pki/tls/private/limiz.key
sudo chmod 640 /etc/pki/tls/private/limiz.key
sudo chown root:limiz /etc/pki/tls/private/limiz.key
```

### 4.5 Config Örnekleri (Linux)

**Açık dosya yolları (önerilen):**

```json
{
  "tls": {
    "cert_file": "/etc/limiz/tls/server.crt",
    "key_file": "/etc/limiz/tls/server.key"
  }
}
```

**CLI bayrakları ile:**

```bash
limiz --tls-cert /etc/limiz/tls/server.crt --tls-key /etc/limiz/tls/server.key
```

---

## 5. Sertifika Doğrulama

### Sertifika bilgilerini görüntüleme (OpenSSL)

```bash
openssl x509 -in /etc/limiz/tls/server.crt -text -noout
```

### Sertifika geçerlilik tarihini kontrol etme

```bash
openssl x509 -in /etc/limiz/tls/server.crt -enddate -noout
```

### Sunucuya bağlanarak sertifikayı test etme

```bash
# Limiz çalışırken
openssl s_client -connect localhost:9110 -servername localhost </dev/null 2>/dev/null | openssl x509 -noout -text
```

### curl ile HTTPS test

```bash
# Self-signed sertifika için -k (insecure) bayrağı gerekebilir
curl -k https://localhost:9110/metrics
```

---

## 6. Sorun Giderme

| Sorun | Çözüm |
|-------|-------|
| `TLS sertifikası yüklenemedi` | `cert_file` ve `key_file` yollarının doğru olduğunu, dosyaların mevcut olduğunu ve Limiz kullanıcısının okuma yetkisine sahip olduğunu kontrol edin. |
| `özel anahtar alınamadı` (Windows) | Servis hesabının özel anahtara erişim yetkisi olduğundan emin olun (bkz. 3.7). |
| `sertifika deposunda bulunamadı` (Windows) | `Get-ChildItem Cert:\LocalMachine\My` ile sertifikanın varlığını doğrulayın. Subject (CN) veya thumbprint değerinin doğru olduğundan emin olun. |
| `x509: certificate signed by unknown authority` | Self-signed sertifika kullanıyorsanız istemci tarafında `-k` bayrağı veya CA sertifikasını sisteme eklemeniz gerekir. |
| Dosya izin hatası (Linux) | `sudo chmod 640 server.key && sudo chown root:limiz server.key` |
| TLS handshake hatası | Sertifika ve özel anahtarın eşleştiğini doğrulayın: `openssl x509 -noout -modulus -in server.crt \| md5sum` ve `openssl rsa -noout -modulus -in server.key \| md5sum` aynı olmalı. |

---

## 7. Güvenlik Önerileri

- Üretim ortamında **self-signed** sertifika yerine güvenilir bir CA'dan alınan sertifika kullanın.
- Özel anahtar dosyasının izinlerini **640** (`rw-r-----`) olarak ayarlayın.
- Windows'ta özel anahtarı **non-exportable** olarak işaretleyin.
- Sertifika süresini düzenli olarak kontrol edin ve yenileyin.
- TLS 1.2 veya üstü kullanıldığından emin olun (Limiz varsayılan olarak TLS 1.2 minimum kullanır).
