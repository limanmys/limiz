# Datas Cache Intervals

Each data category and data plugin can have its own cache refresh interval, independent from the global `datas.cache.interval`. This lets slow or expensive collectors (e.g. package lists, SMART data) refresh less frequently while fast ones (e.g. ports, services) stay up-to-date.

## How it works

When `datas.cache.enabled` is `true`, each provider runs in its own background goroutine:

- If a provider has a `cache_interval` configured → uses that interval.
- If not → falls back to the global `datas.cache.interval`.

The `/datas` response is assembled from each provider's latest cached value. Timestamps and timezones reflect the moment of the HTTP request, not the collection time.

## Global default interval

```json
"datas": {
  "cache": {
    "enabled":  true,
    "interval": "30s"
  }
}
```

All providers without a `cache_interval` override will refresh every `30s`.

---

## Per-category interval

Categories accept two forms:

**Shorthand (uses global interval):**
```json
"categories": {
  "services": true,
  "os":       true
}
```

**Full object (per-category interval):**
```json
"categories": {
  "services": true,
  "packages":    { "enabled": true, "cache_interval": "10m" },
  "updates":     { "enabled": true, "cache_interval": "30m" },
  "disk_health": { "enabled": true, "cache_interval": "5m"  },
  "hardware":    { "enabled": true, "cache_interval": "1h"  },
  "os":          true,
  "ports":       true
}
```

Both forms can be mixed freely in the same config.

### Category reference

| Category     | Field name    | Recommended interval | Notes                              |
|--------------|---------------|----------------------|------------------------------------|
| `services`   | `services`    | 30s – 2m             | Fast, safe to refresh often        |
| `packages`   | `packages`    | 5m – 15m             | `dpkg-query` / `rpm` can be slow   |
| `updates`    | `updates`     | 15m – 60m            | `apt`/`dnf` network check          |
| `disk_health`| `disk_health` | 5m – 15m             | `smartctl` per device              |
| `hardware`   | `hardware`    | 1h – 24h             | Rarely changes                     |
| `os`         | `os`          | 1h – 24h             | Static information                 |
| `ports`      | `ports`       | 30s – 2m             | `ss` / `netstat`                   |

---

## Per-plugin interval

Add `cache_interval` to any plugin item:

```json
"plugins": {
  "enabled":         true,
  "dir":             "/usr/lib/limiz/plugins/data",
  "default_timeout": "15s",
  "items": [
    {
      "name":           "folder-size",
      "exec":           "folder-size",
      "args":           ["--path", "/var/log", "--path", "/tmp"],
      "cache_interval": "10m",
      "enabled":        true
    },
    {
      "name":    "custom-inventory",
      "exec":    "custom-inventory",
      "args":    [],
      "enabled": true
    }
  ]
}
```

- `folder-size` refreshes every `10m`.
- `custom-inventory` has no `cache_interval`, so it uses the global `datas.cache.interval`.

---

## Interval resolution order

```
per-provider cache_interval  →  datas.cache.interval (global default)
```

If `cache_interval` is set but invalid (unparseable or < 1s), the global interval is used silently.

---

## Full example

```json
"datas": {
  "enabled":  true,
  "path":     "/datas",
  "cache": {
    "enabled":  true,
    "interval": "30s"
  },
  "categories": {
    "services":    true,
    "packages":    { "enabled": true, "cache_interval": "10m" },
    "updates":     { "enabled": true, "cache_interval": "30m" },
    "disk_health": { "enabled": true, "cache_interval": "5m"  },
    "hardware":    { "enabled": true, "cache_interval": "1h"  },
    "os":          true,
    "ports":       true
  },
  "plugins": {
    "enabled":         true,
    "dir":             "/usr/lib/limiz/plugins/data",
    "default_timeout": "15s",
    "items": [
      {
        "name":           "folder-size",
        "exec":           "folder-size",
        "args":           ["--path", "/var/log"],
        "cache_interval": "10m",
        "enabled":        true
      }
    ]
  }
}
```

In this config:

| Provider      | Refresh interval |
|---------------|-----------------|
| `services`    | 30s (global)    |
| `packages`    | 10m             |
| `updates`     | 30m             |
| `disk_health` | 5m              |
| `hardware`    | 1h              |
| `os`          | 30s (global)    |
| `ports`       | 30s (global)    |
| `folder-size` | 10m             |
