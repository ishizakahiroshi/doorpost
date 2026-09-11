<!-- このファイルはプロジェクト固有ルールのみを書く。個人/グローバル AI ルール
（言語・確認スタイル・出力フォーマット等）は各 AI ツールのグローバル設定へ。
fresh public clone でも有効な内容に保つこと。 -->

# doorpost 開発ガイド

> **このファイルは索引であって本文ではない。** 全 AI セッションで全文がロードされるので、
> ルールの本文はここへ書かず、破る人が必ず開く場所（コード・テスト・検査スクリプト）へ置き、
> ここには索引の 1 行だけを残す。新しいルールを足す前に既存の CLAUDE.md・skill・guide を検索し、
> 正本が既にあれば参照だけにする。詳細は下記「設計原則の索引」。

## プロジェクト概要

doorpost は、**自分が管理するネットワークの中で、決まった数の呼び出し元へ HTTP の口を
開けるサービス**の土台だけを引き受けるライブラリ。受け口を持つサービスが、業務に入る前に
必ず答える 4 つを担当する。設定の読み込みと検証・鍵と呼び出し元 IP による認証・
エラー応答の語彙・受付 1 件につき 1 行の運用ログ。

**同じ 4 つを複数のサービスへ書き写すと、直し漏れが起きる。**1 本だけ古い実装が残り、
そこだけ検証が緩い、という壊れ方をする。だから 1 か所に置いて import する。

想定利用者は、`net/http` で小さな受け口を書く人。フレームワークが要る人は
フレームワークを使えばよい。

## やらないこと（スコープ外）

- **HTTP の送信**。どこへも何も送らない
- **ルーティング・ハンドラ・ミドルウェア**。使う側が書く
- **保存**。データベースもキャッシュもファイルも持たず、呼び出しをまたぐ状態を持たない
- **業務ロジック・宛先の解決・再送・並列実行**
- **利用者・セッション・権限**。扱うのは「名乗ったアプリ本人か」だけで、
  「そのアプリが何をしてよいか」は使う側の話

## 技術スタック

| 項目 | 内容 |
|---|---|
| 言語 | Go 1.22（利用側の最低要件になるので、必要が無い限り上げない） |
| module | `github.com/ishizakahiroshi/doorpost` |
| 外部依存 | `github.com/BurntSushi/toml` の 1 本だけ。増やさない |
| 配布 | タグを打つだけ（Go のモジュールはレジストリへの公開が無い） |

## ディレクトリ構成

| パス | 内容 |
|---|---|
| `config/` | 設定の読み込みと検証。共通部分は `Base`、使う側は埋め込んで自分の項目を足す |
| `config/common.example.toml` | 共通部分の雛形。テストが読むので、形を変えたら一緒に直る |
| `gate/` | 鍵と呼び出し元 IP による認証 |
| `respond/` | エラーの語彙と JSON 応答の形 |
| `oplog/` | 受付 1 件につき 1 行の運用ログ |
| `scripts/` | 検査スクリプト |

## 主要コマンド

- ビルド: `go build ./...` ／ 静的検査: `go vet ./...` ／ テスト: `go test ./... -count=1`
- 整形: `gofmt -l .`（出力が空であること）
- secrets-scan（手動）: `node scripts/secrets-scan.mjs --staged --block`
- CLAUDE.md 構造検査: `node scripts/check-claude-md.mjs`

## 設計原則の索引（本文は正本にある）

| ルール | 正本（本文はここ） | 機械検査 |
|---|---|---|
| 使う側に固有の項目をこのライブラリへ持ち込まない。共通部分は `Base`、固有は埋め込んだ外側 | `config/config.go` のパッケージコメント | `config/config_test.go` の `TestDecodeReadsCommonAndServiceFields` |
| エラーの語も同じ。汎用の 2 語だけを持ち、使う側は `respond.Code` で自分の語を足す | `respond/respond.go` | `respond/respond_test.go` の「使う側が足した語」 |
| 鍵は 2 本まで。1 本だと入れ替えの日に止まり、3 本以上は消し忘れが生き続ける | `config/config.go` の `MaxKeysPerApp` | `TestKeysAtMostTwo` |
| 検証で落ちた理由を応答で区別しない。鍵違いも IP 違いも同じ `invalid_key` | `gate/gate.go` のパッケージコメント | `TestAuthorizeRejects` |
| エラー文にもログにも鍵の値を出さない | `config/config.go` の `validate` | `TestErrorsNeverContainKeyValues` |
| `X-Forwarded-For` を許可判定に使わない | `gate/gate.go` の `ClientIP` | `TestAuthorizeIgnoresForwardedFor` |
| 知らない設定キーはエラー。綴り間違いを既定値で通さない | `config/config.go` の `Decode` | `TestDecodeRejects` の「知らないキーがある」 |
| 空の `allow_ips` を「全許可」と読まない | `config/config.go` の `validate` | `TestDecodeRejects` の「allow_ips が空」 |

**新しいルールを足す前に、まずこの表に 1 行足せる形にできないかを考える。**

## AI 作業共通ルール

ビルド・コミット禁止、secrets-scan 責務、plan/bugfix/pending md の作成ルール等の AI 作業共通ルールは、各利用者のグローバル AI 設定に従う（作者環境の例: `~/.claude/CLAUDE.md` および `~/.claude/guides/`）。

このリポジトリ固有:

- **上流と下流の向きを守る。** 4 つの責務はこのリポジトリが正本で、利用側（作者の非公開リポを含む）は module として import する。利用側で直して後からこちらへ写す運用はしない
- **実サーバーのホスト名・IP・顧客名・社内の決定 ID をコード・テスト・ドキュメントへ書かない。** フィクスチャは RFC 5737 のドキュメント用アドレス（`192.0.2.0/24` / `198.51.100.0/24` / `203.0.113.0/24`）と `127.0.0.1`、架空のアプリ名だけを使う
- **公開ライブラリなので、破壊的な変更は利用側のビルドを壊す。** 型と関数の形を変えるときは、先に利用側で何が壊れるかを数える

## Obsidian artifacts

If `docs/obsidian/README.md` exists, use it as an index for related knowledge artifacts.
Use the repository-relative `docs/obsidian` entry. Do not write to a central absolute
path and do not silently fall back to `docs/local` when the entry is missing.

## secrets-scan（このリポジトリの配線）

書く瞬間の責務（固有名詞の一般化・fixture は合成データ等）は上記「AI 作業共通ルール」の参照先に従う。このリポジトリ固有の配線は以下:

- scanner: `scripts/secrets-scan.mjs`（手動実行: `node scripts/secrets-scan.mjs --staged --block`）
- layer 3: `.github/workflows/secrets-scan.yml`
- env (full coverage に必要・未設定なら構造 regex のみで継続): `KB_ROOT` / `FAMILY_ROOT`。設定詳細は `scripts/secrets-scan.mjs` の冒頭コメント

## 関連ドキュメント

| 項目 | パス |
|---|---|
| ユーザー向け README（英語） | `README.md` |
| Codex/他 AI 用入口 | `AGENTS.md` |
| ローカル作業ノート（非公開） | `docs/local/`（存在する場合） |
| Obsidian knowledge artifacts | `docs/obsidian/`（存在する場合。作業キューではない） |
