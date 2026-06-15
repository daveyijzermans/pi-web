<h1 align="center">pi-web (Remote Control Your Pi)</h1>

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/ygncode/pi-web?style=flat&logo=github&label=stars&cacheSeconds=21600)](https://github.com/ygncode/pi-web/stargazers)
[![npm downloads](https://img.shields.io/npm/dt/@ygncode/pi-web?label=downloads&color=2ea043)](https://www.npmjs.com/package/@ygncode/pi-web)
[![license MIT](https://img.shields.io/npm/l/@ygncode/pi-web?label=license&color=0a7bbb)](../../LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/+NJvFOTTa0wNjNTc9)
![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)

[English](../../README.md) · [Español](README.es.md) · [Français](README.fr.md) · [Deutsch](README.de.md) · [中文](README.zh.md) · [日本語](README.ja.md) · [Bahasa Indonesia](README.id.md) · [Bahasa Melayu](README.ms.md) · [Tiếng Việt](README.vi.md) · **ไทย** · [Filipino](README.fil.md) · [မြန်မာ](README.my.md) · [ភាសាខ្មែរ](README.km.md) · [ລາວ](README.lo.md)

</div>

ขับเคลื่อน [pi](https://pi.dev) โค้ดดิ้งเอเจนต์ของคุณจากโทรศัพท์ แท็บเล็ต หรือแล็ปท็อป — ทุกที่บนเครือข่ายของคุณ หรือจากระยะไกลผ่าน Tailscale

มันเป็น PWA เต็มรูปแบบ คุณจึงสามารถติดตั้งและใช้งานเหมือนแอปเนทีฟบนอุปกรณ์ใดก็ได้ คิดซะว่าเป็นพื้นที่ทำงาน AI ส่วนตัวของคุณเอง — เหมือน Cowork ของ Claude แต่ใช้โมเดลที่แตกต่าง — แชทข้ามโมเดล เขียนโค้ดจากโทรศัพท์ หรือเปลี่ยนมันให้เป็น [ผู้ช่วยส่วนตัว](../th/personal-assistant.md) ที่ทำงานอยู่บนเครื่องของคุณ

ปรับแต่งให้เป็นของคุณ: สลับธีมและฟอนต์ และใช้ในภาษาของคุณเอง — pi-web มาพร้อมกับหลายภาษาและคุณสามารถเพิ่มภาษาของคุณเองได้ ฟีเจอร์อื่น ๆ กำลังตามมา แต่มันจะไม่บวม: อะไรที่คุณไม่ต้องการก็ปิดได้ในการตั้งค่า

> [!WARNING]
> pi-web อยู่ในช่วง **beta** สิ่งต่าง ๆ จะเปลี่ยนแปลงและพังได้!

> [!TIP]
> เพิ่งมาใหม่ใช่ไหม? **[อ่านคู่มือผู้ใช้ →](../th/README.md)** เพื่อทัวร์ฟีเจอร์ทั้งหมด ขั้นตอนการติดตั้ง และเคล็ดลับ

## ภาพหน้าจอ

<div align="center">
  <img src="../assets/desktop-dark-mode.png" alt="Desktop — dark mode" width="90%" /><br />
  <em>เดสก์ท็อป — โหมดมืด</em>
  <br /><br />
  <img src="../assets/desktop-white-mode.png" alt="Desktop — light mode" width="90%" /><br />
  <em>เดสก์ท็อป — โหมดสว่าง</em>
  <br /><br />
  <img src="../assets/mobile-pwa.png" alt="Mobile PWA" width="90%" /><br />
  <em>PWA บนมือถือ</em>
</div>

## การทำงานร่วมกัน

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

- **pi** เขียน JSONL การสนทนาลงใน `~/.pi/agent/sessions/` ขณะทำงาน
- **pi-web** คือเซิร์ฟเวอร์ Go ที่อ่านไฟล์เหล่านั้น แสดงผลในเบราว์เซอร์ และสตรีมอัปเดตสดผ่าน SSE
- **pi --mode rpc** เวิร์กเกอร์จัดการแชทที่เริ่มจากเบราว์เซอร์ — หนึ่งตัวต่อเซสชัน ถูกลบหลังว่าง 10 นาที
- **fsnotify** เฝ้าดูไดเรกทอรีเซสชันเพื่อให้เบราว์เซอร์โหลดใหม่ภายในมิลลิวินาทีเมื่อมีผลลัพธ์ใหม่
- **Tailscale Serve** เผยแพร่เซิร์ฟเวอร์ localhost เป็นปลายทาง HTTPS บน tailnet ของคุณ

## ติดตั้ง

```bash
pi install npm:@ygncode/pi-web@beta
```

เท่านั้นเอง — มันจะดาวน์โหลดไบนารีที่ตรงกัน ตั้งค่าเริ่มอัตโนมัติ และลงทะเบียนคำสั่ง `/web`, `/pi-web`, `/remote` และ `/refresh`

เมื่อติดตั้งแล้ว เปิด `http://127.0.0.1:31415` ในเบราว์เซอร์ของคุณ จาก pi ใช้ `/web` เพื่อเปิดเซสชันปัจจุบันในเบราว์เซอร์ทันที ถ้า Tailscale กำลังทำงานบนเครื่องของคุณ pi-web จะเผยแพร่ปลายทาง HTTPS บน tailnet ของคุณโดยอัตโนมัติ — ใช้ `/remote` จาก pi เพื่อรับ QR code และ URL สำหรับอุปกรณ์ใดก็ได้บน tailnet ของคุณ

สำหรับการติดตั้งแบบแมนนวล การดาวน์โหลดไบนารี หรือการ build จากซอร์ส ดูที่ [user-docs/install.md](../th/install.md)

## การเชื่อมต่อกับ Pi

หลังจาก `pi install npm:@ygncode/pi-web@beta` คุณจะได้:

| คำสั่ง | สิ่งที่ทำ |
|---------|--------------|
| `/web` | เปิดเซสชันปัจจุบันในเบราว์เซอร์ของคุณ (รองรับ SSH: ข้ามเบราว์เซอร์และแสดงเฉพาะ URL) |
| `/pi-web` | แสดงสถานะ เวอร์ชัน เริ่ม/หยุด/รีสตาร์ทเซิร์ฟเวอร์ หรืออัปเดต |
| `/remote` | แสดง QR code และ URL สำหรับการเข้าถึงระยะไกลผ่าน Tailscale |
| `/refresh` | ดึงข้อความใหม่ที่เขียนจากเบราว์เซอร์ระยะไกลกลับเข้าสู่เซสชันเทอร์มินัล |

**การตั้งชื่อเซสชันอัตโนมัติ** ถูกสร้างไว้ใน pi-web และกำหนดค่าบนหน้า `/settings` มัน**เปิดอยู่โดยค่าเริ่มต้น**และตั้งชื่อเซสชันโดยอัตโนมัติ คุณสามารถเลือก:

- **เมื่อใดที่จะตั้งชื่อ** — หนึ่งครั้งต่อเซสชัน หรือทุกข้อความใหม่ (ค่าเริ่มต้น)
- **โมเดลสำหรับตั้งชื่อ** — **ฮิวริสติกคำในตัว (ไม่มี AI)** ฟรีและรวดเร็ว โดยค่าเริ่มต้น หรือเลือกโมเดล (เช่น โมเดลเล็ก/เร็ว) สำหรับชื่อที่ฉลาดขึ้นและเขียนโดยโมเดล

แพ็คเกจยังติดตั้งไบนารี pi-web ไปที่ `~/.pi/agent/bin/pi-web` และตั้งค่าเริ่มอัตโนมัติเมื่อล็อกอิน

## เริ่มอัตโนมัติเมื่อล็อกอิน

คำสั่ง `pi install npm:@ygncode/pi-web@beta` ตั้งค่านี้ให้โดยอัตโนมัติ:

| ระบบปฏิบัติการ | กลไก |
|----|-----------|
| macOS | launchd plist ที่ `~/Library/LaunchAgents/com.pi-web.plist` |
| Linux | systemd user service ที่ `~/.config/systemd/user/pi-web.service` |

ในการตั้ง token สำหรับการเข้าถึงระยะไกล สร้าง `~/.config/pi-web/env`:

```
PI_WEB_TOKEN=your-token-here
```

สำหรับรายละเอียดเพิ่มเติม (การตั้งค่าแบบแมนนวล พอร์ตที่กำหนดเอง การ bind แบบ non-loopback) ดูที่ [user-docs/install.md](../th/install.md)

## การพัฒนา

```bash
make setup   # ติดตั้ง frontend deps และดาวน์โหลด Go modules
make check   # frontend test/build + Go test/vet
make build   # setup ถ้าจำเป็น, build frontend, แล้ว build ./pi-web
```
