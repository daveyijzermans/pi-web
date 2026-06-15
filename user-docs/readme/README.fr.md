# pi-web (Remote Control Your Pi)

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/ygncode/pi-web?style=flat&logo=github&label=stars)](https://github.com/ygncode/pi-web/stargazers)
[![npm downloads](https://img.shields.io/npm/dt/@ygncode/pi-web?label=downloads&color=2ea043)](https://www.npmjs.com/package/@ygncode/pi-web)
[![license MIT](https://img.shields.io/npm/l/@ygncode/pi-web?label=license&color=0a7bbb)](../../LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/+NJvFOTTa0wNjNTc9)
![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)

[English](../../README.md) · [Español](README.es.md) · **Français** · [Deutsch](README.de.md) · [中文](README.zh.md) · [日本語](README.ja.md) · [Bahasa Indonesia](README.id.md) · [Bahasa Melayu](README.ms.md) · [Tiếng Việt](README.vi.md) · [ไทย](README.th.md) · [Filipino](README.fil.md) · [မြန်မာ](README.my.md) · [ភាសាខ្មែរ](README.km.md) · [ລາວ](README.lo.md)

</div>

Commencez votre agent de codage [pi](https://pi.dev) depuis votre téléphone, tablette ou ordinateur portable — n'importe où sur votre réseau, ou à distance via Tailscale.

C'est une PWA complète, vous pouvez donc l'installer et l'utiliser comme une application native sur n'importe quel appareil. Considérez-la comme votre propre espace de travail IA personnel — comme Cowork de Claude, mais avec différents modèles — discutez à travers les modèles, codez depuis votre téléphone, ou transformez-la en [assistant personnel](../fr/personal-assistant.md) qui réside sur votre machine.

Faites-la vôtre : changez les thèmes et les polices, et utilisez-la dans votre propre langue — pi-web est livré avec plusieurs langues et vous pouvez ajouter la vôtre. D'autres fonctionnalités sont en préparation, mais sans devenir gonflé : tout ce dont vous n'avez pas besoin peut être désactivé dans les paramètres.

> [!WARNING]
> pi-web est actuellement en **beta**. Les choses vont changer et casser !

> [!TIP]
> Nouveau ici ? **[Lisez le guide utilisateur →](../fr/README.md)** pour une visite complète des fonctionnalités, les étapes d'installation et des astuces.

## Captures d'écran

<div align="center">
  <img src="../assets/desktop-dark-mode.png" alt="Desktop — dark mode" width="90%" /><br />
  <em>Ordinateur de bureau — mode sombre</em>
  <br /><br />
  <img src="../assets/desktop-white-mode.png" alt="Desktop — light mode" width="90%" /><br />
  <em>Ordinateur de bureau — mode clair</em>
  <br /><br />
  <img src="../assets/mobile-pwa.png" alt="Mobile PWA" width="90%" /><br />
  <em>PWA mobile</em>
</div>

## Comment tout s'articule

```
 pi (terminal)                 Navigateur (téléphone / tablette / ordinateur portable)
      │                                │
      │  écrit JSONL                   │  HTTP + SSE
      ▼                                ▼
 ~/.pi/agent/sessions/  ←───  pi-web (serveur HTTP Go)
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                 │
              pi --mode rpc      fsnotify         tailscale serve
            (worker de chat    (rechargement     (HTTPS distant
             par session)        en direct)       via MagicDNS)
```

- **pi** écrit les conversations au format JSONL dans `~/.pi/agent/sessions/` au fur et à mesure.
- **pi-web** est un serveur Go qui lit ces fichiers, les affiche dans le navigateur et diffuse les mises à jour en direct via SSE.
- Les workers **pi --mode rpc** gèrent les discussions initiées depuis le navigateur — un par session, supprimés après 10 min d'inactivité.
- **fsnotify** surveille le répertoire des sessions pour que le navigateur se recharge en quelques millisecondes après une nouvelle sortie.
- **Tailscale Serve** publie le serveur localhost comme point de terminaison HTTPS sur votre tailnet.

## Installation

```bash
pi install npm:@ygncode/pi-web@beta
```

C'est tout — il télécharge le binaire correspondant, configure le démarrage automatique et enregistre les commandes `/web`, `/pi-web`, `/remote` et `/refresh`.

Une fois installé, ouvrez `http://127.0.0.1:31415` dans votre navigateur. Depuis pi, utilisez `/web` pour ouvrir instantanément la session en cours dans votre navigateur. Si Tailscale est en cours d'exécution sur votre machine, pi-web publie automatiquement un point de terminaison HTTPS sur votre tailnet — utilisez `/remote` depuis pi pour obtenir un QR code et une URL pour tout appareil sur votre tailnet.

Pour les installations manuelles, les téléchargements de binaires ou la compilation depuis les sources, consultez [user-docs/install.md](../fr/install.md).

## Intégration Pi

Après `pi install npm:@ygncode/pi-web@beta`, vous obtenez :

| Commande | Ce qu'elle fait |
|---------|--------------|
| `/web` | Ouvre la session en cours dans votre navigateur (compatible SSH : ignore le navigateur et affiche seulement l'URL) |
| `/pi-web` | Affiche le statut, la version, démarre/arrête/redémarre le serveur, ou met à jour |
| `/remote` | Affiche un QR code et une URL pour l'accès à distance via Tailscale |
| `/refresh` | Récupère les nouveaux messages écrits depuis des navigateurs distants dans la session du terminal |

Le **titrage automatique** des sessions est intégré à pi-web même et se configure sur la page `/settings`. Il est **activé par défaut** et nomme automatiquement les sessions. Vous pouvez choisir :

- **Quand titrer** — une fois par session, ou à chaque nouveau message (par défaut).
- **Modèle de titre** — une **heuristique de mots intégrée gratuite et instantanée (sans IA)** par défaut, ou choisissez un modèle (par ex. un petit/rapide) pour des titres plus intelligents écrits par le modèle.

Le paquet installe également le binaire pi-web dans `~/.pi/agent/bin/pi-web` et configure le démarrage automatique à la connexion.

## Démarrage automatique à la connexion

La commande `pi install npm:@ygncode/pi-web@beta` configure cela automatiquement :

| OS | Mécanisme |
|----|-----------|
| macOS | launchd plist dans `~/Library/LaunchAgents/com.pi-web.plist` |
| Linux | service systemd utilisateur dans `~/.config/systemd/user/pi-web.service` |

Pour définir un jeton pour l'accès à distance, créez `~/.config/pi-web/env` :

```
PI_WEB_TOKEN=your-token-here
```

Pour plus de détails (configuration manuelle, ports personnalisés, liaisons non-loopback), consultez [user-docs/install.md](../fr/install.md).

## Développement

```bash
make setup   # installer les dépendances frontend et télécharger les modules Go
make check   # test/compilation frontend + test/vet Go
make build   # configure si nécessaire, compile le frontend, puis compile ./pi-web
```
