# pi-web (Remote Control Your Pi)

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/ygncode/pi-web?style=flat&logo=github&label=stars)](https://github.com/ygncode/pi-web/stargazers)
[![npm downloads](https://img.shields.io/npm/dt/@ygncode/pi-web?label=downloads&color=2ea043)](https://www.npmjs.com/package/@ygncode/pi-web)
[![license MIT](https://img.shields.io/npm/l/@ygncode/pi-web?label=license&color=0a7bbb)](../../LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/+NJvFOTTa0wNjNTc9)
![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)

[English](../../README.md) · [Español](README.es.md) · [Français](README.fr.md) · [Deutsch](README.de.md) · [中文](README.zh.md) · [日本語](README.ja.md) · [Bahasa Indonesia](README.id.md) · [Bahasa Melayu](README.ms.md) · **Tiếng Việt** · [ไทย](README.th.md) · [Filipino](README.fil.md) · [မြန်မာ](README.my.md) · [ភាសាខ្មែរ](README.km.md) · [ລາວ](README.lo.md)

</div>

Điều khiển [pi](https://pi.dev) coding agent của bạn từ điện thoại, máy tính bảng, hoặc laptop — ở bất kỳ đâu trên mạng của bạn, hoặc từ xa qua Tailscale.

Đây là một PWA đầy đủ, bạn có thể cài đặt và sử dụng như ứng dụng native trên mọi thiết bị. Hãy xem nó như không gian làm việc AI cá nhân của riêng bạn — giống như Cowork của Claude, nhưng với nhiều model khác nhau — trò chuyện qua các model, code từ điện thoại, hoặc biến nó thành [trợ lý cá nhân](../vi/personal-assistant.md) chạy trên máy của bạn.

Tùy biến theo ý bạn: đổi theme và font chữ, sử dụng bằng ngôn ngữ của bạn — pi-web có sẵn nhiều ngôn ngữ và bạn có thể thêm ngôn ngữ riêng. Nhiều tính năng đang được phát triển, nhưng sẽ không bị phình to: mọi thứ bạn không cần đều có thể tắt trong cài đặt.

> [!WARNING]
> pi-web hiện đang trong giai đoạn **beta**. Mọi thứ sẽ thay đổi và có thể bị lỗi!

> [!TIP]
> Mới dùng? **[Đọc hướng dẫn sử dụng →](../vi/README.md)** để xem đầy đủ tính năng, các bước cài đặt, và mẹo.

## Ảnh Chụp Màn Hình

<div align="center">
  <img src="../assets/desktop-dark-mode.png" alt="Desktop — chế độ tối" width="90%" /><br />
  <em>Desktop — chế độ tối</em>
  <br /><br />
  <img src="../assets/desktop-white-mode.png" alt="Desktop — chế độ sáng" width="90%" /><br />
  <em>Desktop — chế độ sáng</em>
  <br /><br />
  <img src="../assets/mobile-pwa.png" alt="PWA trên di động" width="90%" /><br />
  <em>PWA trên di động</em>
</div>

## Cách Mọi Thứ Kết Hợp

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

- **pi** ghi hội thoại dạng JSONL vào `~/.pi/agent/sessions/` trong khi hoạt động.
- **pi-web** là một máy chủ Go đọc các file đó, hiển thị chúng trong trình duyệt, và truyền cập nhật trực tiếp qua SSE.
- Các worker **pi --mode rpc** xử lý trò chuyện khởi tạo từ trình duyệt — mỗi phiên một worker, bị dọn dẹp sau 10 phút không hoạt động.
- **fsnotify** theo dõi thư mục sessions để trình duyệt tải lại trong vài mili giây khi có output mới.
- **Tailscale Serve** công bố máy chủ localhost thành một endpoint HTTPS trên tailnet của bạn.

## Cài Đặt

```bash
pi install npm:@ygncode/pi-web@beta
```

Vậy là xong — nó tải binary phù hợp, thiết lập tự động khởi động, và đăng ký các lệnh `/web`, `/pi-web`, `/remote`, và `/refresh`.

Sau khi cài đặt, mở `http://127.0.0.1:31415` trong trình duyệt. Từ pi, dùng `/web` để mở phiên hiện tại trong trình duyệt ngay lập tức. Nếu Tailscale đang chạy trên máy của bạn, pi-web tự động công bố một endpoint HTTPS trên tailnet — dùng `/remote` từ pi để lấy mã QR và URL cho mọi thiết bị trên tailnet của bạn.

Để cài đặt thủ công, tải binary, hoặc build từ source, xem [user-docs/install.md](../vi/install.md).

## Tích Hợp Với Pi

Sau khi `pi install npm:@ygncode/pi-web@beta`, bạn có:

| Lệnh | Chức năng |
|------|-----------|
| `/web` | Mở phiên hiện tại trong trình duyệt (nhận biết SSH: bỏ qua trình duyệt và chỉ hiển thị URL) |
| `/pi-web` | Hiển thị trạng thái, phiên bản, khởi động/dừng/khởi động lại máy chủ, hoặc cập nhật |
| `/remote` | Hiển thị mã QR và URL để truy cập từ xa qua Tailscale |
| `/refresh` | Kéo tin nhắn mới được viết từ trình duyệt từ xa về lại phiên terminal |

Tự động đặt tiêu đề phiên được tích hợp sẵn trong pi-web và cấu hình tại trang `/settings`. Tính năng này được **bật mặc định** và tự động đặt tên cho các phiên. Bạn có thể chọn:

- **Khi nào đặt tiêu đề** — một lần mỗi phiên, hoặc mỗi khi có tin nhắn mới (mặc định).
- **Model đặt tiêu đề** — miễn phí, tức thì bằng **heuristic từ tích hợp sẵn (không dùng AI)** theo mặc định, hoặc chọn một model (vd: model nhỏ/nhanh) để có tiêu đề thông minh hơn do model viết.

Gói cũng cài đặt binary pi-web vào `~/.pi/agent/bin/pi-web` và thiết lập tự động khởi động khi đăng nhập.

## Tự Động Khởi Động Khi Đăng Nhập

Lệnh `pi install npm:@ygncode/pi-web@beta` tự động thiết lập việc này:

| Hệ điều hành | Cơ chế |
|--------------|--------|
| macOS | launchd plist tại `~/Library/LaunchAgents/com.pi-web.plist` |
| Linux | systemd user service tại `~/.config/systemd/user/pi-web.service` |

Để đặt token cho truy cập từ xa, tạo file `~/.config/pi-web/env`:

```
PI_WEB_TOKEN=your-token-here
```

Để biết thêm chi tiết (thiết lập thủ công, cổng tùy chỉnh, bind không qua loopback), xem [user-docs/install.md](../vi/install.md).

## Phát Triển

```bash
make setup   # cài đặt frontend deps và tải Go modules
make check   # frontend test/build + Go test/vet
make build   # setup nếu cần, build frontend, sau đó build ./pi-web
```
