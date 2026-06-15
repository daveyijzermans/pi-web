<h1 align="center">pi-web (Remote Control Your Pi)</h1>

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/ygncode/pi-web?style=flat&logo=github&label=stars&cacheSeconds=21600)](https://github.com/ygncode/pi-web/stargazers)
[![npm downloads](https://img.shields.io/npm/dt/@ygncode/pi-web?label=downloads&color=2ea043)](https://www.npmjs.com/package/@ygncode/pi-web)
[![license MIT](https://img.shields.io/npm/l/@ygncode/pi-web?label=license&color=0a7bbb)](../../LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/+NJvFOTTa0wNjNTc9)
![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)

[English](../../README.md) · [Español](README.es.md) · [Français](README.fr.md) · [Deutsch](README.de.md) · [中文](README.zh.md) · [日本語](README.ja.md) · [Bahasa Indonesia](README.id.md) · [Bahasa Melayu](README.ms.md) · [Tiếng Việt](README.vi.md) · [ไทย](README.th.md) · [Filipino](README.fil.md) · **မြန်မာ** · [ភាសាខ្មែរ](README.km.md) · [ລາວ](README.lo.md)

</div>

---

သင့် [pi](https://pi.dev) coding agent ကို သင့်ဖုန်း၊ တက်ဘလက် သို့မဟုတ် လက်ပ်တော့မှ မောင်းနှင်ပါ — သင့်ကွန်ရက်အတွင်း မည်သည့်နေရာမှမဆို၊ သို့မဟုတ် Tailscale မှတစ်ဆင့် အဝေးမှ။

၎င်းသည် full PWA ဖြစ်သောကြောင့် မည်သည့်စက်ပစ္စည်းတွင်မဆို install လုပ်၍ native app တစ်ခုကဲ့သို့ အသုံးပြုနိုင်သည်။ ၎င်းကို သင့်ကိုယ်ပိုင် AI workspace အဖြစ် တွေးကြည့်ပါ — Claude ၏ Cowork ကဲ့သို့သော်လည်း မတူညီသော model များဖြင့် — model များကြားတွင် chat လုပ်ခြင်း၊ သင့်ဖုန်းမှ code ရေးခြင်း၊ သို့မဟုတ် သင့်စက်ပေါ်တွင် ရှင်သန်နေသည့် [personal assistant](../my/personal-assistant.md) အဖြစ် ပြောင်းလဲအသုံးပြုပါ။

သင့်စိတ်ကြိုက်ပြုလုပ်ပါ- theme များနှင့် font များ ပြောင်းပါ၊ သင့်ကိုယ်ပိုင်ဘာသာစကားဖြင့် အသုံးပြုပါ — pi-web တွင် ဘာသာစကားများစွာ ပါဝင်ပြီး သင့်ကိုယ်ပိုင်ဘာသာစကားကိုလည်း ထည့်သွင်းနိုင်သည်။ နောက်ထပ် features များ လာနေဆဲဖြစ်သော်လည်း bloated ဖြစ်လာမည်မဟုတ်ပါ- သင်မလိုအပ်သည့်အရာများကို settings တွင် ပိတ်ထားနိုင်သည်။

> [!WARNING]
> pi-web သည် လက်ရှိတွင် **beta** အဆင့်တွင်ရှိသည်။ အရာများ ပြောင်းလဲမည်၊ ပျက်စီးနိုင်သည်။

> [!TIP]
> အသစ်ရောက်ရှိလာပါသလား။ **[အသုံးပြုသူလမ်းညွှန်ကို ဖတ်ရှုပါ →](../my/README.md)** features များ၊ install ပြုလုပ်နည်းအဆင့်များနှင့် အကြံပြုချက်များ အပြည့်အစုံအတွက်။

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

## ၎င်းတို့ မည်သို့ ချိတ်ဆက်အလုပ်လုပ်ပုံ

```
 pi (terminal)                 Browser (phone / tablet / laptop)
      │                                │
      │  writes JSONL                  │  HTTP + SSE
      ▼                                ▼
 ~/.pi/agent/sessions/  ←───  pi-web (Go HTTP server)
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                 │
              pi --mode rpc      fsnotify         tailscale serve
            (per‑session       (live reload)      (remote HTTPS
             chat worker)                           via MagicDNS)
```

- **pi** သည် အလုပ်လုပ်နေစဉ် conversation JSONL ကို `~/.pi/agent/sessions/` သို့ ရေးသားသည်။
- **pi-web** သည် ထိုဖိုင်များကို ဖတ်ရှုကာ browser တွင် render လုပ်ပြီး SSE မှတစ်ဆင့် live update များ stream လုပ်သည့် Go server တစ်ခုဖြစ်သည်။
- **pi --mode rpc** worker များသည် browser မှစတင်သော chat ကို ကိုင်တွယ်သည် — session တစ်ခုလျှင် တစ်ခု၊ ၁၀ မိနစ် idle ပြီးနောက် ရပ်ဆိုင်းခံရသည်။
- **fsnotify** သည် sessions directory ကို စောင့်ကြည့်နေသောကြောင့် output အသစ်ထွက်ပြီး မီလီစက္ကန့်အတွင်း browser က reload လုပ်သည်။
- **Tailscale Serve** သည် localhost server ကို သင့် tailnet ပေါ်တွင် HTTPS endpoint အဖြစ် ထုတ်ဝေပေးသည်။

## Install

```bash
pi install npm:@ygncode/pi-web@beta
```

ဒါပါပဲ — ၎င်းသည် သင့်လျော်သော binary ကို download လုပ်ကာ auto‑start ကို ပြင်ဆင်ပေးပြီး `/web`, `/pi-web`, `/remote`, နှင့် `/refresh` commands များကို register လုပ်ပေးသည်။

Install လုပ်ပြီးပါက သင့် browser တွင် `http://127.0.0.1:31415` ကိုဖွင့်ပါ။ pi မှ `/web` ကိုသုံး၍ လက်ရှိ session ကို သင့် browser တွင် ချက်ချင်းဖွင့်ပါ။ သင့်စက်ပေါ်တွင် Tailscale လည်ပတ်နေပါက pi-web သည် သင့် tailnet ပေါ်တွင် HTTPS endpoint တစ်ခုကို အလိုအလျောက် ထုတ်ဝေပေးသည် — pi မှ `/remote` ကိုသုံး၍ သင့် tailnet ပေါ်ရှိ မည်သည့်စက်အတွက်မဆို QR code နှင့် URL ကို ရယူပါ။

Manual install များ၊ binary download များ၊ သို့မဟုတ် source မှ build လုပ်ခြင်းအတွက် [user-docs/install.md](../my/install.md) တွင် ကြည့်ရှုပါ။

## Pi Integration

`pi install npm:@ygncode/pi-web@beta` ပြီးနောက် သင်ရရှိမည့်အရာများ-

| Command | ၎င်းလုပ်ဆောင်ချက် |
|---------|--------------|
| `/web` | လက်ရှိ session ကို သင့် browser တွင်ဖွင့်ပါ (SSH-aware- browser ကိုကျော်၍ URL ကိုသာပြသသည်) |
| `/pi-web` | Status၊ version၊ server ကို start/stop/restart၊ သို့မဟုတ် update ပြသပါ |
| `/remote` | Tailscale မှတစ်ဆင့် အဝေးမှ အသုံးပြုရန်အတွက် QR code နှင့် URL ကိုပြသပါ |
| `/refresh` | အဝေး browser များမှ ရေးသားထားသော message အသစ်များကို terminal session သို့ ပြန်လည်ဆွဲယူပါ |

Session **auto-titling** ကို pi-web တွင် တည်ဆောက်ထားပြီး `/settings` စာမျက်နှာတွင် configure ပြုလုပ်နိုင်သည်။ ၎င်းသည် **default အားဖြင့် on** ဖြစ်ပြီး session များကို အလိုအလျောက် အမည်ပေးသည်။ သင်ရွေးချယ်နိုင်သည်များ-

- **အမည်ပေးချိန်** — session တစ်ခုလျှင် တစ်ကြိမ်၊ သို့မဟုတ် message အသစ်တိုင်းတွင် (default)။
- **ခေါင်းစဉ်ပေး model** — default အားဖြင့် အခမဲ့၊ ချက်ချင်းရလဒ်ထွက်သည့် **built-in word heuristic (AI မဟုတ်)**၊ သို့မဟုတ် model တစ်ခုကို ရွေးချယ်ပါ (ဥပမာ သေးငယ်/မြန်ဆန်သော model) ပိုမိုထက်မြက်သော model ရေးသားသည့် ခေါင်းစဉ်များအတွက်။

ထို package သည် pi-web binary ကို `~/.pi/agent/bin/pi-web` တွင် install လုပ်ပြီး login တွင် auto-start ကိုလည်း ပြင်ဆင်ပေးသည်။

## Login တွင် Auto-Start

`pi install npm:@ygncode/pi-web@beta` command က ၎င်းကို အလိုအလျောက် ပြင်ဆင်ပေးသည်-

| OS | ယန္တရား |
|----|-----------|
| macOS | `~/Library/LaunchAgents/com.pi-web.plist` တွင် launchd plist |
| Linux | `~/.config/systemd/user/pi-web.service` တွင် systemd user service |

အဝေးမှ အသုံးပြုရန်အတွက် token သတ်မှတ်ရန် `~/.config/pi-web/env` တွင် ဖန်တီးပါ-

```
PI_WEB_TOKEN=your-token-here
```

အသေးစိတ်အချက်အလက်များအတွက် (manual setup၊ custom ports၊ non-loopback binds) [user-docs/install.md](../my/install.md) တွင် ကြည့်ရှုပါ။

## Development

```bash
make setup   # frontend deps များ install လုပ်ပြီး Go modules download လုပ်ပါ
make check   # frontend test/build + Go test/vet
make build   # လိုအပ်ပါက setup လုပ်၊ frontend build လုပ်၊ ထို့နောက် ./pi-web build လုပ်ပါ
```
