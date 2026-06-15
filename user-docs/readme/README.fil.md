# pi-web (Remote Control Your Pi)

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/ygncode/pi-web?style=flat&logo=github&label=stars)](https://github.com/ygncode/pi-web/stargazers)
[![npm downloads](https://img.shields.io/npm/dt/@ygncode/pi-web?label=downloads&color=2ea043)](https://www.npmjs.com/package/@ygncode/pi-web)
[![license MIT](https://img.shields.io/npm/l/@ygncode/pi-web?label=license&color=0a7bbb)](../../LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/+NJvFOTTa0wNjNTc9)
![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)

[English](../../README.md) · [Español](README.es.md) · [Français](README.fr.md) · [Deutsch](README.de.md) · [中文](README.zh.md) · [日本語](README.ja.md) · [Bahasa Indonesia](README.id.md) · [Bahasa Melayu](README.ms.md) · [Tiếng Việt](README.vi.md) · [ไทย](README.th.md) · **Filipino** · [မြန်မာ](README.my.md) · [ភាសាខ្មែរ](README.km.md) · [ລາວ](README.lo.md)

</div>

Drive your [pi](https://pi.dev) coding agent mula sa iyong telepono, tablet, o laptop — kahit saan sa iyong network, o malayuan sa pamamagitan ng Tailscale.

Isa itong ganap na PWA, kaya maaari mo itong i-install at gamitin na parang native app sa anumang device. Isipin mo ito bilang iyong sariling personal na AI workspace — tulad ng Cowork ng Claude, ngunit may iba't ibang modelo — makipag-chat sa iba't ibang modelo, mag-code mula sa iyong telepono, o gawin itong [personal assistant](../fil/personal-assistant.md) na naninirahan sa iyong makina.

Gawin mong sarili mo: palitan ang mga tema at font, at gamitin ito sa iyong sariling wika — ang pi-web ay may kasamang maraming wika at maaari kang magdagdag ng sa iyo. Marami pang mga feature ang paparating, ngunit hindi ito magiging bloated: anumang hindi mo kailangan ay maaaring i-off sa settings.

> [!WARNING]
> Ang pi-web ay kasalukuyang nasa **beta**. Magbabago at masisira ang mga bagay!

> [!TIP]
> Bago ka rito? **[Basahin ang user guide →](../fil/README.md)** para sa isang buong tour ng mga feature, mga hakbang sa pag-install, at mga tip.

## Screenshots

<div align="center">
  <img src="../assets/desktop-dark-mode.png" alt="Desktop — dark mode" width="90%" /><br />
  <em>Desktop — dark mode</em>
  <br /><br />
  <img src="../assets/desktop-white-mode.png" alt="Desktop — light mode" width="90%" /><br />
  <em>Desktop — light mode</em>
  <br /><br />
  <img src="../assets/mobile-pwa.png" alt="Mobile PWA" width="90%" /><br />
  <em>Mobile PWA</em>
</div>

## Paano Ito Pinagsasama-sama

```
 pi (terminal)                 Browser (telepono / tablet / laptop)
      │                                │
      │  nagsusulat ng JSONL           │  HTTP + SSE
      ▼                                ▼
 ~/.pi/agent/sessions/  ←───  pi-web (Go HTTP server)
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                 │
              pi --mode rpc      fsnotify         tailscale serve
            (per‑session       (live reload)      (remote HTTPS
             chat worker)                           sa pamamagitan ng
                                                    MagicDNS)
```

- Ang **pi** ay nagsusulat ng conversation JSONL sa `~/.pi/agent/sessions/` habang ito ay gumagana.
- Ang **pi-web** ay isang Go server na nagbabasa ng mga file na iyon, nire-render ang mga ito sa browser, at nag-i-stream ng mga live update sa pamamagitan ng SSE.
- Ang **pi --mode rpc** workers ay humahawak ng chat na sinisimulan sa browser — isa bawat session, tinatanggal pagkatapos ng 10 minuto ng kawalang-galaw.
- Ang **fsnotify** ay nagbabantay sa sessions directory upang ang browser ay mag-reload sa loob ng ilang millisecond ng bagong output.
- Ang **Tailscale Serve** ay nagla-lathala ng localhost server bilang isang HTTPS endpoint sa iyong tailnet.

## Install

```bash
pi install npm:@ygncode/pi-web@beta
```

Iyon na — dina-download nito ang tumutugmang binary, nagse-set up ng auto-start, at nirerehistro ang `/web`, `/pi-web`, `/remote`, at `/refresh` na mga command.

Kapag na-install na, buksan ang `http://127.0.0.1:31415` sa iyong browser. Mula sa pi, gamitin ang `/web` upang agad na buksan ang kasalukuyang session sa iyong browser. Kung tumatakbo ang Tailscale sa iyong makina, awtomatikong nagla-lathala ang pi-web ng isang HTTPS endpoint sa iyong tailnet — gamitin ang `/remote` mula sa pi upang makakuha ng QR code at URL para sa anumang device sa iyong tailnet.

Para sa manual na pag-install, binary downloads, o pagbuo mula sa source, tingnan ang [user-docs/install.md](../fil/install.md).

## Pi Integration

Pagkatapos ng `pi install npm:@ygncode/pi-web@beta`, makukuha mo ang:

| Command | Kung ano ang ginagawa nito |
|---------|---------------------------|
| `/web` | Buksan ang kasalukuyang session sa iyong browser (SSH-aware: nilalaktawan ang browser at ipinapakita lamang ang URL) |
| `/pi-web` | Ipakita ang status, bersyon, simulan/ihinto/i-restart ang server, o i-update |
| `/remote` | Ipakita ang isang QR code at URL para sa remote access sa pamamagitan ng Tailscale |
| `/refresh` | Hilahin ang mga bagong mensahe na isinulat mula sa mga remote browser pabalik sa terminal session |

Ang session **auto-titling** ay built-in sa pi-web mismo at nako-configure sa `/settings` page. Ito ay **naka-on bilang default** at awtomatikong pinapangalanan ang mga session. Maaari mong piliin:

- **Kailan mag-title** — isang beses bawat session, o sa bawat bagong mensahe (ang default).
- **Title model** — isang libre, instant na **built-in word heuristic (walang AI)** bilang default, o pumili ng modelo (hal. isang maliit/mabilis) para sa mas matalino, model-written na mga title.

Ang package ay nag-i-install din ng pi-web binary sa `~/.pi/agent/bin/pi-web` at nagse-set up ng auto-start sa pag-login.

## Auto-Start sa Pag-login

Ang command na `pi install npm:@ygncode/pi-web@beta` ay awtomatikong nagse-set up nito:

| OS | Mekanismo |
|----|-----------|
| macOS | launchd plist sa `~/Library/LaunchAgents/com.pi-web.plist` |
| Linux | systemd user service sa `~/.config/systemd/user/pi-web.service` |

Upang magtakda ng token para sa remote access, lumikha ng `~/.config/pi-web/env`:

```
PI_WEB_TOKEN=your-token-here
```

Para sa higit pang mga detalye (manual setup, custom ports, non-loopback binds), tingnan ang [user-docs/install.md](../fil/install.md).

## Development

```bash
make setup   # i-install ang frontend deps at i-download ang Go modules
make check   # frontend test/build + Go test/vet
make build   # setup kung kinakailangan, i-build ang frontend, pagkatapos ay i-build ang ./pi-web
```
