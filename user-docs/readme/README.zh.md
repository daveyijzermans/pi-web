# pi-web (Remote Control Your Pi)

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/ygncode/pi-web?style=flat&logo=github&label=stars)](https://github.com/ygncode/pi-web/stargazers)
[![npm downloads](https://img.shields.io/npm/dt/@ygncode/pi-web?label=downloads&color=2ea043)](https://www.npmjs.com/package/@ygncode/pi-web)
[![license MIT](https://img.shields.io/npm/l/@ygncode/pi-web?label=license&color=0a7bbb)](../../LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/+NJvFOTTa0wNjNTc9)
![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)

[English](../../README.md) · [Español](README.es.md) · [Français](README.fr.md) · [Deutsch](README.de.md) · **中文** · [日本語](README.ja.md) · [Bahasa Indonesia](README.id.md) · [Bahasa Melayu](README.ms.md) · [Tiếng Việt](README.vi.md) · [ไทย](README.th.md) · [Filipino](README.fil.md) · [မြန်မာ](README.my.md) · [ភាសាខ្មែរ](README.km.md) · [ລາວ](README.lo.md)

</div>

驱动你的 [pi](https://pi.dev) 编程助手，随时随地 —— 无论是手机、平板还是笔记本，只要在同一网络内，或通过 Tailscale 远程连接。

它是一个完整的 PWA，你可以像原生应用一样安装到任何设备上使用。把它想象成你自己的个人 AI 工作空间 —— 就像 Claude 的 Cowork，但可以使用不同的模型 —— 跨模型聊天、在手机上写代码，或者把它变成一个常驻你机器上的[个人助手](../zh/personal-assistant.md)。

随你定制：切换主题和字体，使用你自己的语言 —— pi-web 内置多语言支持，你还可以添加自己的语言。更多功能正在开发中，但不会变得臃肿：不需要的功能都可以在设置中关闭。

> [!WARNING]
> pi-web 目前处于 **beta** 阶段。一切仍可能发生变动！

> [!TIP]
> 刚接触？**[阅读用户指南 →](../zh/README.md)** 了解完整的功能介绍、安装步骤和使用技巧。

## 屏幕截图

<div align="center">
  <img src="../assets/desktop-dark-mode.png" alt="Desktop — dark mode" width="90%" /><br />
  <em>桌面端 — 暗色模式</em>
  <br /><br />
  <img src="../assets/desktop-white-mode.png" alt="Desktop — light mode" width="90%" /><br />
  <em>桌面端 — 亮色模式</em>
  <br /><br />
  <img src="../assets/mobile-pwa.png" alt="Mobile PWA" width="90%" /><br />
  <em>移动端 PWA</em>
</div>

## 整体架构

```
 pi (终端)                     浏览器 (手机 / 平板 / 笔记本)
      │                                │
      │  写入 JSONL                    │  HTTP + SSE
      ▼                                ▼
 ~/.pi/agent/sessions/  ←───  pi-web (Go HTTP 服务器)
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                 │
              pi --mode rpc      fsnotify         tailscale serve
            (每个会话一个       (实时重载)      (通过 MagicDNS
             聊天工作进程)                        提供远程 HTTPS)
```

- **pi** 在工作时将对话以 JSONL 格式写入 `~/.pi/agent/sessions/`。
- **pi-web** 是一个 Go 服务器，读取这些文件，在浏览器中渲染，并通过 SSE 流式推送实时更新。
- **pi --mode rpc** 工作进程处理浏览器发起的聊天 —— 每个会话一个，空闲 10 分钟后回收。
- **fsnotify** 监控会话目录，使浏览器在新输出后的毫秒级内即可重载。
- **Tailscale Serve** 将本地服务器以 HTTPS 端点形式发布到你的 tailnet 上。

## 安装

```bash
pi install npm:@ygncode/pi-web@beta
```

就这么简单 —— 它会下载对应的二进制文件，设置开机自启，并注册 `/web`、`/pi-web`、`/remote` 和 `/refresh` 命令。

安装完成后，在浏览器中打开 `http://127.0.0.1:31415`。在 pi 中使用 `/web` 可以立即在浏览器中打开当前会话。如果你的机器上运行着 Tailscale，pi-web 会自动在你的 tailnet 上发布一个 HTTPS 端点 —— 在 pi 中使用 `/remote` 可以获取 QR 码和 URL，供 tailnet 上的任何设备使用。

如需手动安装、二进制下载或从源码构建，请参阅 [user-docs/install.md](../zh/install.md)。

## Pi 集成

执行 `pi install npm:@ygncode/pi-web@beta` 后，你将获得以下命令：

| 命令 | 功能 |
|------|------|
| `/web` | 在浏览器中打开当前会话（支持 SSH 检测：在 SSH 环境下会跳过浏览器并仅显示 URL） |
| `/pi-web` | 显示状态、版本，启动/停止/重启服务器，或更新 |
| `/remote` | 显示 QR 码和 URL，用于通过 Tailscale 远程访问 |
| `/refresh` | 将远程浏览器中写入的新消息拉取回终端会话中 |

会话**自动命名**功能内置于 pi-web 中，可在 `/settings` 页面配置。**默认开启**，会自动为会话命名。你可以选择：

- **命名时机** — 每个会话命名一次，或每条新消息都重新命名（默认）。
- **命名模型** — 默认使用免费且即时的**内置词汇启发式算法（无需 AI）**，也可以选择一个模型（例如小型/快速的模型）来生成更智能的模型驱动标题。

该软件包同时会将 pi-web 二进制文件安装到 `~/.pi/agent/bin/pi-web`，并设置登录时自启。

## 登录自启

`pi install npm:@ygncode/pi-web@beta` 命令会自动完成以下设置：

| 操作系统 | 机制 |
|----------|------|
| macOS | launchd plist，位于 `~/Library/LaunchAgents/com.pi-web.plist` |
| Linux | systemd 用户服务，位于 `~/.config/systemd/user/pi-web.service` |

要设置远程访问令牌，请创建 `~/.config/pi-web/env`：

```
PI_WEB_TOKEN=your-token-here
```

更多详情（手动设置、自定义端口、非 loopback 绑定），请参阅 [user-docs/install.md](../zh/install.md)。

## 开发

```bash
make setup   # 安装前端依赖并下载 Go 模块
make check   # 前端测试/构建 + Go 测试/语法检查
make build   # 按需执行 setup，构建前端，然后构建 ./pi-web
```
