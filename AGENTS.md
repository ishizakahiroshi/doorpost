# Agent Entry Point (doorpost)

このリポジトリの運用ガイダンスは `CLAUDE.md` を正本とする。

- プロジェクト概要・ルール: `./CLAUDE.md`
- ユーザー向けドキュメント: `./README.md`
- ローカル/プライベート追記（存在する場合・コミットしない）: `./CLAUDE.local.md` / `./AGENTS.local.md` / `./docs/local/`

個人/グローバル AI ルールは意図的にこのリポジトリの外に置く。各 AI ツールの
グローバル設定を使うこと。本ファイルは fresh public clone でも有効に保つ。

## Non-negotiables (full detail in CLAUDE.md)

- **使う側に固有の項目・語をこのライブラリへ持ち込まない。** 共通部分は `config.Base` と
  `respond` の 2 語だけ。固有のものは、使う側が埋め込んだ外側と自分の `respond.Code` に置く
- 実サーバーのホスト名・IP・顧客名・社内の決定 ID をコード・テスト・ドキュメントへ書かない。
  フィクスチャは RFC 5737 のドキュメント用アドレスと `127.0.0.1`、架空のアプリ名だけを使う
- 鍵の値をエラー文にもログにも出さない。出るのはキー名・アプリ名・添字まで
- 認証が通らなかった理由を応答で区別しない（鍵違いも IP 違いも同じ `invalid_key`）
- `X-Forwarded-For` を許可判定に使わない
- 外部依存は `github.com/BurntSushi/toml` の 1 本だけ。増やす提案は理由を先に出す
- 公開ライブラリなので、型と関数の形を変えると利用側のビルドが壊れる。変える前に数える
- ビルド・コミット禁止、secrets-scan 責務、plan/bugfix/pending md の作成ルール等の AI 作業共通ルールは、各利用者のグローバル AI 設定に従う（作者環境の例: `~/.claude/CLAUDE.md` および `~/.claude/guides/`）
- secrets-scan のこのリポジトリの配線（scanner パス・手動実行コマンド等）は `CLAUDE.md` の「secrets-scan（このリポジトリの配線）」節を参照
- `docs/obsidian/README.md` があれば索引として読み、知識記録は repo 相対の `docs/obsidian` を使う。欠損時に `docs/local` へ黙って fallback しない

ガイダンス間で矛盾が出たら `CLAUDE.md` を優先する。

<!-- many-ai-cli の承認マーカーブロックはここに自動注入される。本ファイルでは持たない。 -->
