<h1 align="center">pi-web (Remote Control Your Pi)</h1>

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/ygncode/pi-web?style=flat&logo=github&label=stars&cacheSeconds=86400)](https://github.com/ygncode/pi-web/stargazers)
[![npm downloads](https://img.shields.io/npm/dt/@ygncode/pi-web?label=downloads&color=2ea043)](https://www.npmjs.com/package/@ygncode/pi-web)
[![license MIT](https://img.shields.io/npm/l/@ygncode/pi-web?label=license&color=0a7bbb)](../../LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/+NJvFOTTa0wNjNTc9)
![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)

[English](../../README.md) · [Español](README.es.md) · [Français](README.fr.md) · **Deutsch** · [中文](README.zh.md) · [日本語](README.ja.md) · [Bahasa Indonesia](README.id.md) · [Bahasa Melayu](README.ms.md) · [Tiếng Việt](README.vi.md) · [ไทย](README.th.md) · [Filipino](README.fil.md) · [မြန်မာ](README.my.md) · [ភាសាខ្មែរ](README.km.md) · [ລາວ](README.lo.md)

</div>

Steuere deinen [pi](https://pi.dev) Coding-Agenten von deinem Handy, Tablet oder Laptop — überall in deinem Netzwerk oder remote über Tailscale.

Es ist eine vollständige PWA, sodass du sie installieren und wie eine native App auf jedem Gerät nutzen kannst. Stell es dir als deinen eigenen persönlichen KI-Arbeitsplatz vor — wie Claudes Cowork, aber mit verschiedenen Modellen — chatte modellübergreifend, programmiere von deinem Handy aus oder verwandle es in einen [persönlichen Assistenten](../de/personal-assistant.md), der auf deinem Rechner lebt.

Mach es zu deinem eigenen: Wechsle Themes und Schriftarten und nutze es in deiner eigenen Sprache — pi-web wird mit mehreren Sprachen ausgeliefert und du kannst eigene hinzufügen. Weitere Funktionen sind in Arbeit, aber es wird nicht aufgebläht: Alles, was du nicht brauchst, kannst du in den Einstellungen deaktivieren.

> [!WARNING]
> pi-web befindet sich derzeit in der **beta**-Phase. Dinge werden sich ändern und kaputtgehen!

> [!TIP]
> Neu hier? **[Lies das Benutzerhandbuch →](../de/README.md)** für eine vollständige Tour der Funktionen, Installationsschritte und Tipps.

## Screenshots

<div align="center">
  <img src="../assets/desktop-dark-mode.png" alt="Desktop — dark mode" width="90%" /><br />
  <em>Desktop — Dark Mode</em>
  <br /><br />
  <img src="../assets/desktop-white-mode.png" alt="Desktop — light mode" width="90%" /><br />
  <em>Desktop — Light Mode</em>
  <br /><br />
  <img src="../assets/mobile-pwa.png" alt="Mobile PWA" width="90%" /><br />
  <em>Mobile PWA</em>
</div>

## Wie es zusammenspielt

```
 pi (Terminal)                 Browser (Handy / Tablet / Laptop)
      │                                │
      │  schreibt JSONL               │  HTTP + SSE
      ▼                                ▼
 ~/.pi/agent/sessions/  ←───  pi-web (Go HTTP-Server)
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                 │
              pi --mode rpc      fsnotify         tailscale serve
            (pro Sitzung       (Live-Neuladen)   (Remote-HTTPS
             Chat-Worker)                          via MagicDNS)
```

- **pi** schreibt während der Arbeit Konversationen als JSONL nach `~/.pi/agent/sessions/`.
- **pi-web** ist ein Go-Server, der diese Dateien liest, im Browser darstellt und Live-Updates via SSE streamt.
- **pi --mode rpc**-Worker verarbeiten vom Browser initiierte Chats — einer pro Sitzung, nach 10 Min. Inaktivität beendet.
- **fsnotify** überwacht das Sessions-Verzeichnis, sodass der Browser innerhalb von Millisekunden nach neuer Ausgabe aktualisiert.
- **Tailscale Serve** veröffentlicht den Localhost-Server als HTTPS-Endpunkt in deinem Tailnet.

## Installation

```bash
pi install npm:@ygncode/pi-web@beta
```

Das war's — es lädt die passende Binärdatei herunter, richtet den Autostart ein und registriert die Befehle `/web`, `/pi-web`, `/remote` und `/refresh`.

Öffne nach der Installation `http://127.0.0.1:31415` in deinem Browser. Verwende in pi `/web`, um die aktuelle Sitzung sofort im Browser zu öffnen. Wenn Tailscale auf deinem Rechner läuft, veröffentlicht pi-web automatisch einen HTTPS-Endpunkt in deinem Tailnet — verwende `/remote` in pi, um einen QR-Code und eine URL für jedes Gerät in deinem Tailnet zu erhalten.

Für manuelle Installation, Binär-Downloads oder das Bauen aus dem Quellcode siehe [user-docs/install.md](../de/install.md).

## Pi-Integration

Nach `pi install npm:@ygncode/pi-web@beta` erhältst du:

| Befehl | Was er macht |
|--------|--------------|
| `/web` | Öffnet die aktuelle Sitzung im Browser (SSH-fähig: überspringt den Browser und zeigt nur die URL) |
| `/pi-web` | Zeigt Status, Version, Start/Stopp/Neustart des Servers oder Update |
| `/remote` | Zeigt einen QR-Code und eine URL für den Remote-Zugriff über Tailscale |
| `/refresh` | Holt neue Nachrichten, die von Remote-Browsern geschrieben wurden, zurück in die Terminal-Sitzung |

Die **automatische Titelvergabe** ist direkt in pi-web integriert und wird auf der `/settings`-Seite konfiguriert. Sie ist **standardmäßig aktiviert** und benennt Sitzungen automatisch. Du kannst wählen:

- **Wann ein Titel vergeben wird** — einmal pro Sitzung oder bei jeder neuen Nachricht (Standard).
- **Titelmodell** — standardmäßig eine kostenlose, sofortige **eingebaute Wortheuristik (keine KI)** oder wähle ein Modell (z. B. ein kleines/schnelles) für intelligentere, modellgenerierte Titel.

Das Paket installiert außerdem die pi-web-Binärdatei nach `~/.pi/agent/bin/pi-web` und richtet den Autostart beim Anmelden ein.

## Autostart beim Anmelden

Der Befehl `pi install npm:@ygncode/pi-web@beta` richtet dies automatisch ein:

| OS | Mechanismus |
|----|-------------|
| macOS | launchd-Plist unter `~/Library/LaunchAgents/com.pi-web.plist` |
| Linux | systemd-Benutzerdienst unter `~/.config/systemd/user/pi-web.service` |

Um ein Token für den Remote-Zugriff zu setzen, erstelle `~/.config/pi-web/env`:

```
PI_WEB_TOKEN=your-token-here
```

Für weitere Details (manuelle Einrichtung, benutzerdefinierte Ports, nicht-loopback-Bindungen) siehe [user-docs/install.md](../de/install.md).

## Entwicklung

```bash
make setup   # Frontend-Abhängigkeiten installieren und Go-Module herunterladen
make check   # Frontend-Test/Build + Go-Test/Vet
make build   # Bei Bedarf Setup, Frontend bauen, dann ./pi-web bauen
```
