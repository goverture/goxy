# Goxy 🕸️

[![Go Reference](https://pkg.go.dev/badge/github.com/goverture/goxy.svg)](https://pkg.go.dev/github.com/goverture/goxy)
[![GitHub release](https://img.shields.io/github/v/release/goverture/goxy)](https://github.com/goverture/goxy/releases)

Lightweight **OpenAI API proxy** with **spending limits**. Drop it in front of your app to control usage & avoid surprise bills.

## 🚀 Install

```bash
go install github.com/goverture/goxy@latest
```

## ⚙️ Run

```bash
goxy --listen :8080 --upstream https://api.openai.com --limit 20
```

Then point your app to `http://localhost:8080`.

## 🧰 Env (optional)

```bash
GOXY_LISTEN=:8080
GOXY_UPSTREAM=https://api.openai.com
GOXY_LIMIT=20
goxy
```

## 🏷️ Release

```bash
git tag -s v1.0.0 -m "v1.0.0"
git push origin v1.0.0
```

## 📜 License

MIT
