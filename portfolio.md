---
cover:
  path: portfolio/overview-2026-09-28.jpg
  alt: {ja: "doorpost の紹介動画", en: "doorpost overview video"}
video:
  provider: youtube
  id: "olSq5l35CEg"
  durationSeconds: 20
schemaVersion: 1
color: "#9a6b35"
initials: "dp"
cat: {ja: "Go ライブラリ / HTTP 受付", en: "Go library / HTTP intake"}
tagline: {ja: "小さな HTTP サービスに、共通の入口を。", en: "A shared door frame for small HTTP services."}
short: {ja: "自己管理の Go HTTP サービス向けに、設定検証・鍵と送信元の認証・エラー応答・運用ログをまとめた小さなライブラリ。", en: "A small library for self-hosted Go HTTP services, sharing configuration checks, caller authentication, error responses and operational logging."}
tech: ["Go", "net/http", "TOML"]
store: null
live: null
guide: null
featured: false
---
## ja

管理下のネットワークで、決まった呼び出し元に HTTP の入口を提供するサービスの土台です。TOML 設定の検証、アプリごとの鍵と送信元 IP による認証、共通のエラー表現、受付ごとのログを担当します。業務処理・ルーティング・保存は利用側に残します。単独の画面アプリではなく、Go のサービスから import して使うライブラリです。

## en

A foundation for HTTP services serving known callers inside a network you control. It handles TOML configuration validation, authentication by app key and source address, a shared error vocabulary and request logging. Business logic, routing and storage remain with the consuming service. This is a Go library imported by services, not a standalone graphical application.
