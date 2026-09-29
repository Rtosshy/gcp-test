# gcp-test

Cloud Pub/Sub のスキーマ（Protocol Buffers, edition 2023）を Terraform で管理し、Go から publish して挙動を確かめる実験用リポジトリ。

## 構成

| パス | 内容 |
|---|---|
| `proto/event/v1/user_event.proto` | Pub/Sub スキーマとして登録する proto（edition 2023） |
| `gen/event/v1/user_event.pb.go` | proto から生成した Go コード |
| `terraform/` | Pub/Sub スキーマとスキーマ付きトピック |
| `proto-publisher/` | proto メッセージを publish し、スキーマ検証で弾かれたかを表示する CLI |
| `publisher/`, `subscriber/` | スキーマなしのトピック（`my-topic` / `my-sub`）を使う最初のサンプル |

## セットアップ

### 認証

Terraform と Go クライアントはどちらも ADC（Application Default Credentials）を使う。

```sh
gcloud auth application-default login
```

`oauth2: "invalid_grant" "reauth related error (invalid_rapt)"` が出たら、組織ポリシーによる再認証期限切れなので、同じコマンドで取り直す。gcloud 本体のログインとは別物なので、`gcloud` コマンドが通っても ADC は切れていることがある。

### Go コードの生成

```sh
nix shell nixpkgs#protobuf nixpkgs#protoc-gen-go -c \
  protoc -I proto --go_out=gen --go_opt=module=gcp-test/gen event/v1/user_event.proto
```

edition 2023 はデフォルトで explicit presence なので、生成される Go のフィールドはポインタ（`*string` など）になる。値は `proto.String(...)` などで入れる。

### インフラ

```sh
cd terraform
terraform apply
```

作成されるもの:

- `google_pubsub_schema.user_event`（`user-event-schema`, `PROTOCOL_BUFFER`）: proto ファイルを `file()` で読み込む
- `google_pubsub_topic.user_event`（`user-event-topic`）: 上のスキーマを紐付け

サブスクリプションは作っていないので、publish したメッセージはどこにも配信されない。

変数:

| 変数 | デフォルト | 内容 |
|---|---|---|
| `project_id` | `terraform.tfvars` で指定 | GCP プロジェクト |
| `encoding` | `BINARY` | トピックのメッセージエンコーディング（`BINARY` / `JSON`） |
| `first_revision_id` | `null`（最古から） | 検証に使うスキーマリビジョンの下限 |
| `last_revision_id` | `null`（最新まで） | 検証に使うスキーマリビジョンの上限 |

リビジョン範囲は `-var` で渡して試している（例: `terraform apply -var first_revision_id=ffd5d6a7`）。オプションなしで `apply` すると範囲の指定が外れる。

## proto-publisher

```sh
go run ./proto-publisher [flags]
```

| フラグ | デフォルト | 内容 |
|---|---|---|
| `-project` | `gcp-learning-507706` | GCP プロジェクト |
| `-topic` | `user-event-topic` | トピック |
| `-user-id` | `user-123` | `UserEvent.user_id` |
| `-action` | `login` | `UserEvent.action` |
| `-device` | 空（未設定） | `UserEvent.device`。空ならフィールド自体を送らない |
| `-encoding` | `binary` | `binary` / `json`。トピックの設定に合わせる |
| `-raw` | 空 | proto を使わず文字列をそのまま送る |
| `-invalid` | `false` | proto としてパースできないバイト列を送る |

| 結果 | 出力 | 終了コード |
|---|---|---|
| 受け付けられた | `ACCEPTED: message_id=...` | 0 |
| スキーマ検証で弾かれた（`InvalidArgument`） | `REJECTED: <Pub/Sub のエラーメッセージ>` | 1 |
| それ以外（権限不足、トピックが無い等） | `publish failed: ...` | 2 |

`gen/` の生成コードは `device` フィールドを持つリビジョン（`9814ea79`）のまま。削除後のリビジョンに対して `device` 付きメッセージを送る実験のため、あえて再生成していない。

## よく使う確認コマンド

```sh
# スキーマのリビジョン一覧
gcloud pubsub schemas list-revisions user-event-schema --project=gcp-learning-507706 \
  --format='table(revisionId,revisionCreateTime)'

# 特定リビジョンの定義
gcloud pubsub schemas describe user-event-schema@<REVISION_ID> --project=gcp-learning-507706

# トピックのスキーマ設定（エンコーディング・リビジョン範囲）
gcloud pubsub topics describe user-event-topic --project=gcp-learning-507706 \
  --format='yaml(schemaSettings)'

# スキーマ定義をリソースを作らずに検証
gcloud pubsub schemas validate-schema --project=gcp-learning-507706 \
  --type=protocol-buffer --definition-file=proto/event/v1/user_event.proto
```

Terraform の出力 `latest_schema_revision_id` は、スキーマ更新直後に古い値のままになることがあった。リビジョンは gcloud で確認するのが確実。

## 実験ログ（2026-09-29）

### スキーマのリビジョン

| リビジョン | 作成（UTC） | 内容 |
|---|---|---|
| `285ae575` | 00:41 | 最初の定義: `user_id` / `action` / `occurred_at_unix`（int64） |
| `9814ea79` | 00:50 | `string device = 4;` を追加 |
| `ffd5d6a7` | 01:07 | `device` を削除 |

### 1. edition 2023 の proto はスキーマとして使えるか

`gcloud pubsub schemas validate-schema` で `Schema is valid.`。そのまま Terraform でスキーマを作成できた。

### 2. スキーマ検証の基本動作（BINARY、リビジョン範囲なし、`285ae575` のみ）

| 送ったもの | 結果 |
|---|---|
| 正しい `UserEvent` | ACCEPTED |
| `-invalid`（終端しない varint） | REJECTED: `Invalid data in message: Message failed schema validation.` |

### 3. 互換性のない変更: フィールドの型変更

`int64 occurred_at_unix = 3;` → `string occurred_at_unix = 3;`

- `terraform plan` は in-place 更新（Pub/Sub 上は新しいリビジョンの commit）として扱う
- `terraform apply` で Pub/Sub が 400 を返して拒否:
  ```
  Compatibility checking failed to commit a schema revision ...
  Revision is incompatible with previous revision: 285ae575.
  occurred_at_unix of type string is not compatible with ... of type int64
  ```
- リビジョンは増えず、変更前のメッセージはそのまま ACCEPTED

→ 互換性チェックは Terraform ではなく Pub/Sub の API が行い、トピックに届く前に止まる。

### 4. 互換性のある変更: フィールドの追加

`string device = 4;` を追加 → リビジョン `9814ea79` が増えた。

範囲なし（全リビジョンが対象）で、`device` あり・なしの BINARY メッセージはどちらも ACCEPTED。

### 5. 互換性のある変更: フィールドの削除

`device` を削除 → リビジョン `ffd5d6a7` が増えた（削除は互換性のある変更として扱われる）。

### 6. リビジョン範囲と BINARY の検証

| 範囲 | 送ったもの | 結果 |
|---|---|---|
| `first_revision_id=9814ea79`（`9814ea79`, `ffd5d6a7`） | — | 範囲の設定のみ |
| `first_revision_id=ffd5d6a7`（`ffd5d6a7` のみ） | `-device iphone`（BINARY） | REJECTED |

→ **BINARY でも、範囲内のリビジョンに定義されていないフィールド番号を含むメッセージは弾かれる。** protobuf の通常のパースのように未知フィールドを読み飛ばすわけではない。

→ 新リビジョンで削除したフィールドを含むメッセージは、範囲を新リビジョンだけに絞ると通らなくなる。

### 7. 削除したフィールド番号を別の型で再利用

`ffd5d6a7`（番号4なし）に `int64 device_id = 4;` を追加 → Pub/Sub が拒否:

```
Revision is incompatible with previous revision: 9814ea79.
device_id of type int64 is not compatible with device of type string
```

→ 互換性チェックは直前のリビジョンだけでなく、それより前のリビジョン（ここでは `9814ea79`）とも比較している。全リビジョンと比べているのか、トピックの範囲内のリビジョンだけなのかは未確認（このときトピックの範囲は `9814ea79` 以降）。

## 未確認・次に試せること

- 範囲を `9814ea79` まで広げると、`-device iphone` が ACCEPTED に戻るか（範囲内のどれか1つのリビジョンに合えば通るか）
- 番号4を同じ型・別名（`string platform = 4;`）で再利用できるか。できる場合、古い `device` 付きメッセージが `platform` として受け付けられる
- 7 の互換性チェックが、トピックの範囲外の古いリビジョンも対象にするか
- JSON エンコーディングでの挙動（フィールド名で検証されるので、名前の変更が効くはず）
- サブスクリプションを作り、属性 `googclient_schemarevisionid` でどのリビジョンで検証されたかを見る
