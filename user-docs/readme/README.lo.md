<h1 align="center">pi-web (Remote Control Your Pi)</h1>

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/ygncode/pi-web?style=flat&logo=github&label=stars)](https://github.com/ygncode/pi-web/stargazers)
[![npm downloads](https://img.shields.io/npm/dt/@ygncode/pi-web?label=downloads&color=2ea043)](https://www.npmjs.com/package/@ygncode/pi-web)
[![license MIT](https://img.shields.io/npm/l/@ygncode/pi-web?label=license&color=0a7bbb)](../../LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/+NJvFOTTa0wNjNTc9)
![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)

[English](../../README.md) · [Español](README.es.md) · [Français](README.fr.md) · [Deutsch](README.de.md) · [中文](README.zh.md) · [日本語](README.ja.md) · [Bahasa Indonesia](README.id.md) · [Bahasa Melayu](README.ms.md) · [Tiếng Việt](README.vi.md) · [ไทย](README.th.md) · [Filipino](README.fil.md) · [မြန်မာ](README.my.md) · [ភាសាខ្មែរ](README.km.md) · **ລາວ**

</div>

ໃຊ້ [pi](https://pi.dev) ໂຕແທນຂຽນລະຫັດຂອງທ່ານຈາກໂທລະສັບ, ແທັບເລັດ, ຫຼື ແລັບທັອບ — ທຸກບ່ອນໃນເຄືອຂ່າຍຂອງທ່ານ, ຫຼື ຈາກໄລຍະໄກຜ່ານ Tailscale.

ມັນເປັນ PWA ເຕັມຮູບແບບ, ດັ່ງນັ້ນທ່ານສາມາດຕິດຕັ້ງ ແລະ ໃຊ້ມັນເໝືອນແອັບດັ້ງເດີມໃນທຸກອຸປະກອນ. ຄິດວ່າມັນເປັນພື້ນທີ່ເຮັດວຽກ AI ສ່ວນຕົວຂອງທ່ານ — ຄ້າຍກັບ Cowork ຂອງ Claude, ແຕ່ມີຫຼາຍໂມເດວ — ສົນທະນາຂ້າມໂມເດວ, ຂຽນລະຫັດຈາກໂທລະສັບ, ຫຼື ປ່ຽນມັນເປັນ [ຜູ້ຊ່ວຍສ່ວນຕົວ](../lo/personal-assistant.md) ທີ່ອາໄສຢູ່ໃນເຄື່ອງຂອງທ່ານ.

ປັບແຕ່ງຕາມໃຈ: ປ່ຽນຮູບແບບ ແລະ ຟອນ, ແລະ ໃຊ້ໃນພາສາຂອງທ່ານເອງ — pi-web ມາພ້ອມກັບຫຼາຍພາສາ ແລະ ທ່ານສາມາດເພີ່ມພາສາຂອງທ່ານເອງໄດ້. ຟີເຈີເພີ່ມເຕີມກຳລັງຈະມາ, ແຕ່ມັນຈະບໍ່ບວມເພີ້ມ: ທຸກສິ່ງທີ່ທ່ານບໍ່ຕ້ອງການສາມາດປິດໄດ້ໃນການຕັ້ງຄ່າ.

> [!WARNING]
> pi-web ປັດຈຸບັນຢູ່ໃນສະຖານະ **beta**. ສິ່ງຕ່າງໆຈະປ່ຽນແປງ ແລະ ພັງ!

> [!TIP]
> ໃໝ່ທີ່ນີ້ບໍ? **[ອ່ານຄູ່ມືຜູ້ໃຊ້ →](../lo/README.md)** ສຳລັບການທົວຟີເຈີເຕັມຮູບແບບ, ຂັ້ນຕອນການຕິດຕັ້ງ, ແລະ ເຄັດລັບ.

## ພາບໜ້າຈໍ

<div align="center">
  <img src="../assets/desktop-dark-mode.png" alt="ເດສທັອບ — ໂໝດມືດ" width="90%" /><br />
  <em>ເດສທັອບ — ໂໝດມືດ</em>
  <br /><br />
  <img src="../assets/desktop-white-mode.png" alt="ເດສທັອບ — ໂໝດສະຫວ່າງ" width="90%" /><br />
  <em>ເດສທັອບ — ໂໝດສະຫວ່າງ</em>
  <br /><br />
  <img src="../assets/mobile-pwa.png" alt="PWA ມືຖື" width="90%" /><br />
  <em>PWA ມືຖື</em>
</div>

## ມັນເຮັດວຽກຮ່ວມກັນແນວໃດ

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

- **pi** ຂຽນ JSONL ການສົນທະນາລົງໃນ `~/.pi/agent/sessions/` ໃນຂະນະທີ່ມັນເຮັດວຽກ.
- **pi-web** ແມ່ນເຊີບເວີ Go ທີ່ອ່ານໄຟລ໌ເຫຼົ່ານັ້ນ, ສະແດງຜົນໃນເບຣົາວ໌ເຊີ, ແລະ ສົ່ງຂໍ້ມູນສົດຜ່ານ SSE.
- **pi --mode rpc** workers ຈັດການການສົນທະນາທີ່ເລີ່ມຈາກເບຣົາວ໌ເຊີ — ໜຶ່ງໂຕຕໍ່ session, ຖືກປິດຫຼັງຈາກບໍ່ມີການໃຊ້ງານ 10 ນາທີ.
- **fsnotify** ເຝົ້າເບິ່ງໂຟນເດີ sessions ເພື່ອໃຫ້ເບຣົາວ໌ເຊີໂຫຼດໃໝ່ພາຍໃນມິນລິວິນາທີຫຼັງຈາກມີຂໍ້ມູນໃໝ່.
- **Tailscale Serve** ເຜີຍແພ່ເຊີບເວີ localhost ເປັນຈຸດໝາຍ HTTPS ໃນ tailnet ຂອງທ່ານ.

## ຕິດຕັ້ງ

```bash
pi install npm:@ygncode/pi-web@beta
```

ເທົ່ານັ້ນ — ມັນດາວໂຫຼດໄບນາຣີທີ່ກົງກັນ, ຕັ້ງຄ່າເລີ່ມຕົ້ນອັດຕະໂນມັດ, ແລະ ລົງທະບຽນຄຳສັ່ງ `/web`, `/pi-web`, `/remote`, ແລະ `/refresh`.

ເມື່ອຕິດຕັ້ງແລ້ວ, ເປີດ `http://127.0.0.1:31415` ໃນເບຣົາວ໌ເຊີຂອງທ່ານ. ຈາກ pi, ໃຊ້ `/web` ເພື່ອເປີດ session ປັດຈຸບັນໃນເບຣົາວ໌ເຊີທັນທີ. ຖ້າ Tailscale ກຳລັງເຮັດວຽກຢູ່ໃນເຄື່ອງຂອງທ່ານ, pi-web ຈະເຜີຍແພ່ຈຸດໝາຍ HTTPS ອັດຕະໂນມັດໃນ tailnet ຂອງທ່ານ — ໃຊ້ `/remote` ຈາກ pi ເພື່ອຮັບລະຫັດ QR ແລະ URL ສຳລັບທຸກອຸປະກອນໃນ tailnet ຂອງທ່ານ.

ສຳລັບການຕິດຕັ້ງແບບກຳນົດເອງ, ດາວໂຫຼດໄບນາຣີ, ຫຼື ສ້າງຈາກຊອຣ໌ສ໌ໂຄ້ດ, ເບິ່ງ [user-docs/install.md](../lo/install.md).

## ການເຊື່ອມໂຍງກັບ Pi

ຫຼັງຈາກ `pi install npm:@ygncode/pi-web@beta`, ທ່ານຈະໄດ້ຮັບ:

| ຄຳສັ່ງ | ສິ່ງທີ່ມັນເຮັດ |
|---------|--------------|
| `/web` | ເປີດ session ປັດຈຸບັນໃນເບຣົາວ໌ເຊີ (ຮູ້ຈັກ SSH: ຂ້າມເບຣົາວ໌ເຊີ ແລະ ສະແດງສະເພາະ URL) |
| `/pi-web` | ສະແດງສະຖານະ, ເວີຊັນ, ເລີ່ມ/ຢຸດ/ເລີ່ມໃໝ່ເຊີບເວີ, ຫຼື ອັບເດດ |
| `/remote` | ສະແດງລະຫັດ QR ແລະ URL ສຳລັບການເຂົ້າເຖິງຈາກໄລຍະໄກຜ່ານ Tailscale |
| `/refresh` | ດຶງຂໍ້ຄວາມໃໝ່ທີ່ຂຽນຈາກເບຣົາວ໌ເຊີໄລຍະໄກກັບມາໃສ່ session ເທີມິນອລ |

ການ **ຕັ້ງຊື່ອັດຕະໂນມັດ** ຂອງ session ຖືກສ້າງມາໃນຕົວ pi-web ເອງ ແລະ ຕັ້ງຄ່າໄດ້ທີ່ໜ້າ `/settings`. ມັນ **ເປີດໃຊ້ຕາມຄ່າເລີ່ມຕົ້ນ** ແລະ ຕັ້ງຊື່ໃຫ້ sessions ອັດຕະໂນມັດ. ທ່ານສາມາດເລືອກ:

- **ເວລາໃດທີ່ຈະຕັ້ງຊື່** — ຄັ້ງດຽວຕໍ່ session, ຫຼື ທຸກຂໍ້ຄວາມໃໝ່ (ຄ່າເລີ່ມຕົ້ນ).
- **ໂມເດວຕັ້ງຊື່** — ໃຊ້ **heuristic ຄຳສັບໃນຕົວທີ່ຟຣີ ແລະ ທັນທີ (ບໍ່ໃຊ້ AI)** ຕາມຄ່າເລີ່ມຕົ້ນ, ຫຼື ເລືອກໂມເດວ (ເຊັ່ນ ໂຕນ້ອຍ/ໄວ) ສຳລັບຊື່ທີ່ຂຽນໂດຍໂມເດວທີ່ສະຫຼາດກວ່າ.

ແພັກເກດຍັງຕິດຕັ້ງໄບນາຣີ pi-web ໃສ່ `~/.pi/agent/bin/pi-web` ແລະ ຕັ້ງຄ່າເລີ່ມຕົ້ນອັດຕະໂນມັດເມື່ອເຂົ້າສູ່ລະບົບ.

## ເລີ່ມຕົ້ນອັດຕະໂນມັດເມື່ອເຂົ້າສູ່ລະບົບ

ຄຳສັ່ງ `pi install npm:@ygncode/pi-web@beta` ຕັ້ງຄ່ານີ້ອັດຕະໂນມັດ:

| ລະບົບປະຕິບັດການ | ກົນໄກ |
|----|-----------|
| macOS | launchd plist ທີ່ `~/Library/LaunchAgents/com.pi-web.plist` |
| Linux | systemd user service ທີ່ `~/.config/systemd/user/pi-web.service` |

ເພື່ອຕັ້ງໂທເຄັນສຳລັບການເຂົ້າເຖິງຈາກໄລຍະໄກ, ສ້າງ `~/.config/pi-web/env`:

```
PI_WEB_TOKEN=your-token-here
```

ສຳລັບລາຍລະອຽດເພີ່ມເຕີມ (ຕັ້ງຄ່າແບບກຳນົດເອງ, ພອດທີ່ກຳນົດເອງ, ການຜູກທີ່ບໍ່ແມ່ນ loopback), ເບິ່ງ [user-docs/install.md](../lo/install.md).

## ການພັດທະນາ

```bash
make setup   # ຕິດຕັ້ງ dependencies ສ່ວນໜ້າ ແລະ ດາວໂຫຼດ Go modules
make check   # ທົດສອບ/ສ້າງສ່ວນໜ້າ + Go test/vet
make build   # ຕັ້ງຄ່າຖ້າຈຳເປັນ, ສ້າງສ່ວນໜ້າ, ຈາກນັ້ນສ້າງ ./pi-web
```
