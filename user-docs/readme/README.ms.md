<h1 align="center">pi-web (Remote Control Your Pi)</h1>

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/ygncode/pi-web?style=flat&logo=github&label=stars)](https://github.com/ygncode/pi-web/stargazers)
[![npm downloads](https://img.shields.io/npm/dt/@ygncode/pi-web?label=downloads&color=2ea043)](https://www.npmjs.com/package/@ygncode/pi-web)
[![license MIT](https://img.shields.io/npm/l/@ygncode/pi-web?label=license&color=0a7bbb)](../../LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/+NJvFOTTa0wNjNTc9)
![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)

[English](../../README.md) · [Español](README.es.md) · [Français](README.fr.md) · [Deutsch](README.de.md) · [中文](README.zh.md) · [日本語](README.ja.md) · [Bahasa Indonesia](README.id.md) · **Bahasa Melayu** · [Tiếng Việt](README.vi.md) · [ไทย](README.th.md) · [Filipino](README.fil.md) · [မြန်မာ](README.my.md) · [ភាសាខ្មែរ](README.km.md) · [ລາວ](README.lo.md)

</div>

Kawal [pi](https://pi.dev) coding agent anda dari telefon, tablet, atau komputer riba — di mana-mana sahaja dalam rangkaian anda, atau dari jauh melalui Tailscale.

Ia adalah PWA penuh, jadi anda boleh memasangnya dan menggunakannya seperti aplikasi asli pada mana-mana peranti. Anggaplah ia sebagai ruang kerja AI peribadi anda sendiri — seperti Cowork milik Claude, tetapi dengan pelbagai model — berbual merentasi model, menulis kod dari telefon anda, atau jadikannya [pembantu peribadi](../ms/personal-assistant.md) yang tinggal di mesin anda.

Jadikannya milik anda: tukar tema dan fon, dan gunakannya dalam bahasa anda sendiri — pi-web disertakan dengan pelbagai bahasa dan anda boleh menambah bahasa anda sendiri. Lebih banyak ciri sedang dalam perjalanan, tetapi ia tidak akan menjadi sarat: apa-apa yang anda tidak perlukan boleh dimatikan dalam tetapan.

> [!WARNING]
> pi-web kini dalam versi **beta**. Banyak perkara akan berubah dan rosak!

> [!TIP]
> Baru di sini? **[Baca panduan pengguna →](../ms/README.md)** untuk lawatan penuh ciri, langkah pemasangan, dan petua.

## Tangkapan Skrin

<div align="center">
  <img src="../assets/desktop-dark-mode.png" alt="Desktop — mod gelap" width="90%" /><br />
  <em>Desktop — mod gelap</em>
  <br /><br />
  <img src="../assets/desktop-white-mode.png" alt="Desktop — mod terang" width="90%" /><br />
  <em>Desktop — mod terang</em>
  <br /><br />
  <img src="../assets/mobile-pwa.png" alt="Mobile PWA" width="90%" /><br />
  <em>Mobile PWA</em>
</div>

## Bagaimana Ia Bersatu

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

- **pi** menulis JSONL perbualan ke `~/.pi/agent/sessions/` semasa ia berfungsi.
- **pi-web** ialah pelayan Go yang membaca fail tersebut, memaparkannya dalam pelayar, dan menstrim kemas kini langsung melalui SSE.
- **pi --mode rpc** pekerja mengendalikan perbualan yang dimulakan pelayar — satu setiap sesi, dihapuskan selepas 10 minit melahu.
- **fsnotify** memantau direktori sesi supaya pelayar memuat semula dalam masa milisaat selepas output baru muncul.
- **Tailscale Serve** menerbitkan pelayan localhost sebagai titik akhir HTTPS pada tailnet anda.

## Pemasangan

```bash
pi install npm:@ygncode/pi-web@beta
```

Itu sahaja — ia memuat turun binari yang sepadan, menyediakan auto-mula, dan mendaftarkan arahan `/web`, `/pi-web`, `/remote`, dan `/refresh`.

Setelah dipasang, buka `http://127.0.0.1:31415` dalam pelayar anda. Dari pi, gunakan `/web` untuk membuka sesi semasa dalam pelayar anda serta-merta. Jika Tailscale sedang berjalan pada mesin anda, pi-web menerbitkan titik akhir HTTPS secara automatik pada tailnet anda — gunakan `/remote` dari pi untuk mendapatkan kod QR dan URL untuk mana-mana peranti pada tailnet anda.

Untuk pemasangan manual, muat turun binari, atau binaan dari sumber, lihat [user-docs/install.md](../ms/install.md).

## Integrasi Pi

Selepas `pi install npm:@ygncode/pi-web@beta`, anda mendapat:

| Arahan | Fungsinya |
|---------|--------------|
| `/web` | Buka sesi semasa dalam pelayar anda (sedar SSH: langkau pelayar dan tunjukkan URL sahaja) |
| `/pi-web` | Tunjukkan status, versi, mula/henti/mula semula pelayan, atau kemas kini |
| `/remote` | Tunjukkan kod QR dan URL untuk akses jauh melalui Tailscale |
| `/refresh` | Tarik mesej baru yang ditulis dari pelayar jauh kembali ke sesi terminal |

**Auto-tajuk** sesi dibina terus ke dalam pi-web dan dikonfigurasi pada halaman `/settings`. Ia **dihidupkan secara lalai** dan menamakan sesi secara automatik. Anda boleh memilih:

- **Bila untuk menajuk** — sekali setiap sesi, atau pada setiap mesej baru (lalai).
- **Model tajuk** — **heuristik kata terbina (tanpa AI)** yang percuma dan serta-merta secara lalai, atau pilih model (contohnya yang kecil/pantas) untuk tajuk yang lebih pintar dan ditulis oleh model.

Pakej ini juga memasang binari pi-web ke `~/.pi/agent/bin/pi-web` dan menyediakan auto-mula semasa log masuk.

## Auto-Mula Semasa Log Masuk

Arahan `pi install npm:@ygncode/pi-web@beta` menyediakan ini secara automatik:

| OS | Mekanisme |
|----|-----------|
| macOS | launchd plist di `~/Library/LaunchAgents/com.pi-web.plist` |
| Linux | systemd user service di `~/.config/systemd/user/pi-web.service` |

Untuk menetapkan token untuk akses jauh, cipta `~/.config/pi-web/env`:

```
PI_WEB_TOKEN=your-token-here
```

Untuk butiran lanjut (penyediaan manual, port tersuai, ikatan bukan-loopback), lihat [user-docs/install.md](../ms/install.md).

## Pembangunan

```bash
make setup   # pasang dependensi frontend dan muat turun modul Go
make check   # ujian/bina frontend + ujian/vet Go
make build   # setup jika perlu, bina frontend, kemudian bina ./pi-web
```
