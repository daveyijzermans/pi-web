<h1 align="center">pi-web (Remote Control Your Pi)</h1>

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/ygncode/pi-web?style=flat&logo=github&label=stars&cacheSeconds=21600)](https://github.com/ygncode/pi-web/stargazers)
[![npm downloads](https://img.shields.io/npm/dt/@ygncode/pi-web?label=downloads&color=2ea043)](https://www.npmjs.com/package/@ygncode/pi-web)
[![license MIT](https://img.shields.io/npm/l/@ygncode/pi-web?label=license&color=0a7bbb)](../../LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/+NJvFOTTa0wNjNTc9)
![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)

[English](../../README.md) · [Español](README.es.md) · [Français](README.fr.md) · [Deutsch](README.de.md) · [中文](README.zh.md) · **日本語** · [Bahasa Indonesia](README.id.md) · [Bahasa Melayu](README.ms.md) · [Tiếng Việt](README.vi.md) · [ไทย](README.th.md) · [Filipino](README.fil.md) · [မြန်မာ](README.my.md) · [ភាសាខ្មែរ](README.km.md) · [ລາວ](README.lo.md)

</div>

あなたの [pi](https://pi.dev) コーディングエージェントを、スマートフォン、タブレット、ノートパソコンから操作できます — ネットワーク内のどこからでも、あるいは Tailscale 経由でリモートからも。

完全な PWA なので、インストールすればどんなデバイスでもネイティブアプリのように使えます。自分専用の AI ワークスペースと考えてください — Claude の Cowork のようなものですが、さまざまなモデルを使えます — モデルを切り替えてチャットしたり、スマートフォンからコードを書いたり、あるいはあなたのマシン上で動作する[パーソナルアシスタント](../ja/personal-assistant.md)に仕立てたりできます。

自分好みにカスタマイズ: テーマやフォントを切り替え、自分の言語で使えます — pi-web は複数言語を同梱しており、独自の言語を追加することも可能です。さらに多くの機能が開発中ですが、肥大化することはありません: 不要な機能は設定でオフにできます。

> [!WARNING]
> pi-web は現在 **beta** 段階です。今後変更や破壊的変更が発生します！

> [!TIP]
> 初めての方ですか？ **[ユーザーガイドを読む →](../ja/README.md)** 機能の全体像、インストール手順、ヒントを網羅しています。

## スクリーンショット

<div align="center">
  <img src="../assets/desktop-dark-mode.png" alt="Desktop — dark mode" width="90%" /><br />
  <em>デスクトップ — ダークモード</em>
  <br /><br />
  <img src="../assets/desktop-white-mode.png" alt="Desktop — light mode" width="90%" /><br />
  <em>デスクトップ — ライトモード</em>
  <br /><br />
  <img src="../assets/mobile-pwa.png" alt="Mobile PWA" width="90%" /><br />
  <em>モバイル PWA</em>
</div>

## 全体の仕組み

```
 pi (ターミナル)                 ブラウザ (スマートフォン / タブレット / ノートPC)
      │                                │
      │  JSONL を書き込み              │  HTTP + SSE
      ▼                                ▼
 ~/.pi/agent/sessions/  ←───  pi-web (Go HTTP サーバー)
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                 │
              pi --mode rpc      fsnotify         tailscale serve
            (セッションごと    (ライブリロード)    (MagicDNS 経由の
             のチャットワーカー)                    リモート HTTPS)
```

- **pi** は作業中、会話の JSONL を `~/.pi/agent/sessions/` に書き込みます。
- **pi-web** は Go 製サーバーで、それらのファイルを読み取り、ブラウザで描画し、SSE 経由でライブ更新をストリーミングします。
- **pi --mode rpc** ワーカーがブラウザからのチャットを処理します — セッションごとに 1 つずつ、アイドル状態が 10 分続くと終了します。
- **fsnotify** がセッションディレクトリを監視し、新しい出力があればミリ秒単位でブラウザが再読み込みされます。
- **Tailscale Serve** が localhost サーバーをあなたの tailnet 上の HTTPS エンドポイントとして公開します。

## インストール

```bash
pi install npm:@ygncode/pi-web@beta
```

これだけです — 対応するバイナリをダウンロードし、自動起動を設定し、`/web`、`/pi-web`、`/remote`、`/refresh` コマンドを登録します。

インストール後、ブラウザで `http://127.0.0.1:31415` を開いてください。pi からは `/web` を使うと現在のセッションをブラウザですぐに開けます。お使いのマシンで Tailscale が動作している場合、pi-web は自動的にあなたの tailnet 上に HTTPS エンドポイントを公開します — pi から `/remote` を使うと、tailnet 上の任意のデバイス向けの QR コードと URL を表示します。

手動インストール、バイナリダウンロード、ソースからのビルドについては、[user-docs/install.md](../ja/install.md) を参照してください。

## Pi 連携

`pi install npm:@ygncode/pi-web@beta` を実行すると、以下が利用可能になります:

| コマンド | 機能 |
|---------|--------------|
| `/web` | 現在のセッションをブラウザで開く（SSH 対応: SSH 接続時はブラウザを開かず URL のみ表示） |
| `/pi-web` | ステータス、バージョンの表示、サーバーの起動/停止/再起動、または更新 |
| `/remote` | Tailscale 経由のリモートアクセス用 QR コードと URL を表示 |
| `/refresh` | リモートブラウザから書き込まれた新しいメッセージをターミナルセッションに取り込む |

セッションの**自動タイトル付け**は pi-web 自体に組み込まれており、`/settings` ページで設定します。**デフォルトでオン**になっており、セッションに自動的に名前を付けます。以下の選択が可能です:

- **タイトル付けのタイミング** — セッションごとに 1 回、または新しいメッセージごと（デフォルト）。
- **タイトル付けモデル** — デフォルトは無料で高速な**組み込み単語ヒューリスティック（AI 不使用）**、またはモデル（小型/高速なものなど）を選択して、より賢いモデル生成タイトルを使用できます。

このパッケージは pi-web バイナリを `~/.pi/agent/bin/pi-web` にインストールし、ログイン時の自動起動も設定します。

## ログイン時の自動起動

`pi install npm:@ygncode/pi-web@beta` コマンドが自動的に設定します:

| OS | 仕組み |
|----|-----------|
| macOS | `~/Library/LaunchAgents/com.pi-web.plist` の launchd plist |
| Linux | `~/.config/systemd/user/pi-web.service` の systemd ユーザーサービス |

リモートアクセス用のトークンを設定するには、`~/.config/pi-web/env` を作成します:

```
PI_WEB_TOKEN=your-token-here
```

詳細（手動設定、カスタムポート、非ループバックバインド）については、[user-docs/install.md](../ja/install.md) を参照してください。

## 開発

```bash
make setup   # フロントエンド依存関係のインストールと Go モジュールのダウンロード
make check   # フロントエンドのテスト/ビルド + Go のテスト/vet
make build   # 必要に応じて setup、フロントエンドをビルド、その後 ./pi-web をビルド
```
