# VRChat Image Annotation Tool

[![Go](https://img.shields.io/badge/Go-1.25.5-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Status](https://img.shields.io/badge/Status-Production%20Ready-brightgreen.svg)](IMPLEMENTATION_COMPLETE.md)

VRChat や VirtualCast のスクリーンショットから自動的にメタデータを抽出し、画像にワールド情報・撮影者・撮影日時を注釈として追加するコマンドラインツール。

**主な特徴**: PNG/WebP メタデータ永続化、rMQR コード生成、リアルタイムログ表示

---

## 📦 クイックスタート

### インストール

#### 方法 1: バイナリをダウンロード

[最新リリース](https://github.com/yourusername/VRCSSAnnotationTool/releases) から `VRCSSAnnotationTool.exe` をダウンロード。

#### 方法 2: ソースからビルド

```bash
# リポジトリをクローン
git clone https://github.com/yourusername/VRCSSAnnotationTool.git
cd VRCSSAnnotationTool

# ビルド
go build -o VRCSSAnnotationTool.exe .

# 依存パッケージをインストール
go mod download
```

詳細は [SETUP.md](SETUP.md) を参照。

---

## 🚀 使用方法

### 基本的な使い方

#### 1. メタデータを確認（JSON 出力）

```bash
VRCSSAnnotationTool.exe --json image.png
```

**出力例**:

```json
{
  "fileName": "VRChat_2026-01-18_12-34-56.789_3840x2160.png",
  "worldName": "Example World",
  "authorName": "Your Name",
  "shootDate": "2026-01-18T12:34:56"
}
```

#### 2. 画像に注釈を追加

```bash
VRCSSAnnotationTool.exe --annotate image.png
```

**結果**: `annotated/image.png` に注釈付き画像を保存

```
[注釈内容]
- 撮影日時（左側）: 2026-01-18 WED 12:34:56
- 撮影者（中央）: Your Name
- ワールド名（右側）: Example World
- rMQR コード（右上）: ワールドURL

> Note: For VRChat 'print camera' images at resolution **2048×1440**, only the QR code is added at the top-right on a **fixed white background** (no inversion on dark images); text annotations are omitted.
```

#### 3. 複数ファイルを一括処理

```bash
VRCSSAnnotationTool.exe --auto-annotate image1.png image2.webp image3.jpg
```

#### 4. ドラッグ&ドロップ（Windows）

`annotate.bat` に画像ファイルをドラッグ&ドロップするだけで自動処理。

---

## 🛠️ CLI オプション

| オプション        | 説明                   | 例                               |
| ----------------- | ---------------------- | -------------------------------- |
| `--json`          | JSON 形式で出力        | `--json image.png`               |
| `--pretty`        | JSON を整形            | `--json --pretty image.png`      |
| `--ndjson`        | NDJSON ストリーミング  | `--ndjson image1.png image2.png` |
| `--annotate`      | 注釈を追加して保存     | `--annotate image.png`           |
| `--auto-annotate` | 複数ファイル自動処理   | `--auto-annotate *.png`          |
| `--raw`           | デバッグ用生データ表示 | `--raw image.png`                |
| `--verbose`       | 詳細ログ表示           | `--verbose --annotate image.png` |

---

## 👀 VRChat写真フォルダ監視

`watch` サブコマンドで VRChat 写真フォルダ以下を監視し、起動後に追加された画像だけを処理できます。Eagle には元画像を登録し、Amazon Photos 用には `Annotated` 直下へ注釈付き画像または無加工コピーを保存します。

```bash
VRCSSAnnotationTool.exe watch --root "C:\FURUKAWA\VRChat_pic"
```

設定例:

```json
{
  "watcher": {
    "vrchatPhotoRoot": "C:\\FURUKAWA\\VRChat_pic",
    "amazonPhotosOutputDir": "",
    "vrchatLogDir": "",
    "visitLogDir": "visit-logs",
    "fileStabilityWaitSeconds": 5,
    "stableCheckIntervalSeconds": 1,
    "stableCheckCount": 3,
    "scanIntervalSeconds": 3,
    "logPollIntervalSeconds": 2
  },
  "eagle": {
    "enabled": true,
    "baseUrl": "http://localhost:41595",
    "folderId": "",
    "folders": []
  },
  "state": {
    "path": "watch-state.jsonl"
  }
}
```

追加サブコマンド:

| コマンド | 説明 |
| --- | --- |
| `watch` | VRChat写真フォルダを監視し、新規画像を安定待ち後に処理 |
| `process-file <path>` | 単一ファイルを手動処理 |
| `test-eagle` | Eagle Web API V2 (`/api/v2/app/info`) の接続確認 |
| `print-config` | 読み込み済み設定をJSONで表示 |
| `retry-failed` | `watch-state.jsonl` の失敗エントリを再試行 |
| `reprocess-state` | `watch-state.jsonl` の既存 `source_path` を重複排除して全件再処理 |

種別分類と出力:

| 種別 | 判定 | Eagle | Amazon Photos |
| --- | --- | --- | --- |
| `photo` | `VRChat_YYYY-MM-DD_HH-MM-SS.xxx_...` | 元画像を `type:photo`、月、ワールド情報付きで登録 | 上部注釈と rMQR 付きコピー |
| `print` | パスに `\Print\` を含む | 元画像を `type:print` で登録 | ワールドIDがあれば rMQR のみ追加、なければ無加工コピー |
| `sticker` | パスに `\Stickers\` を含む | 元画像を `type:sticker` で登録 | 無加工コピー |
| `stamp` | パスに `\Stamp\` を含む | 元画像を `type:stamp` で登録 | 無加工コピー |
| `emoji` | パスまたはファイル名に `emoji` / `emote` 系文字列 | 元画像を `type:emoji` で登録 | 無加工コピー |
| `unknown` | その他 | 元画像を `type:unknown` で登録 | 無加工コピー |

Print Cameraは、VRCX固有の `撮影者_YYYY-MM-DD_HH-MM-SS.mmm_prnt_UUID` 形式を `print` として処理します。同じ撮影でVRChat側に保存される `VRChat_..._2048x1440.png` は重複防止のため対象外です。

Eagle 連携は `POST http://localhost:41595/api/v2/item/add` を使います。Eagle が起動していない場合でも Amazon Photos 出力は独立して実行され、失敗内容は `watch-state.jsonl` に追記されます。

`watch` / `process-file` / `reprocess-state` は VRChat の `output_log_*.txt` を読み、撮影時刻にいたワールド状態を復元します。`watcher.vrchatLogDir` が空の場合は `%USERPROFILE%\AppData\LocalLow\VRChat\VRChat` を使います。`watch` 実行中はワールド移動、同席ユーザーの join/leave を `visitLogDir` 配下に `vrchat-visits-YYYY-MM-DD.jsonl` として日別に記録します。

通常写真では、撮影時刻に対応する `output_log` のワールドIDまたはワールド名が取れた場合、画像内メタデータよりログ復元結果を優先します。Print Cameraでは画像メタデータの `worldID` を優先してrMQRを作成します。Print Cameraの `worldID` が欠落している場合は、その撮影時刻の同席者一覧に撮影者が記録されている場合だけログから補完します。同席ユーザーとインスタンス情報はログから取得できた場合に Eagle の `user:*` タグと `annotation` に反映します。

`reprocess-state` は既存の `watch-state.jsonl` を破壊的に書き換えません。既存行の `source_path` を読み、存在するファイルを現在のログ復元ルールで再処理し、同じ JSONL 形式の新しい結果を末尾へ追記します。Eagle 側は通常の `item/add` を呼ぶため、既に登録済みの画像を再インポートする運用では Eagle 側の重複扱いに注意してください。

---

## 📋 出力ファイル

### JSON 出力の例

```json
{
  "fileName": "path/to/image.png",
  "fileType": "PNG",
  "imageWidth": 3840,
  "imageHeight": 2160,
  "worldID": "wrld_xxxxx",
  "worldName": "Example World",
  "authorID": "usr_xxxxx",
  "authorName": "Your Name",
  "shootDate": "2026-01-18T12:34:56.0000000+09:00"
}
```

### アノテーション出力

入力ファイルは `annotated/` ディレクトリに以下のように保存：

```
annotated/
├── image1.png           (注釈追加版)
├── image2.webp          (注釈追加版)
└── annotate.log         (処理ログ)
```

**log ファイル例**:

```
[2026-01-18 12:34:56] Processing: image1.png
[2026-01-18 12:34:57] [Metadata] PNG XMP extracted (1178 bytes)...
[2026-01-18 12:34:57] [SUCCESS] PNG metadata written
[2026-01-18 12:34:57] Completed: annotated/image1.png
```

---

## 🔧 トラブルシューティング

### 問題 1: "フォントが見つかりません" エラー

```
ERROR: Font not found
```

**原因**: 日本語フォント（Meiryo, YuGothic など）がインストールされていない。

**解決方法**:

```powershell
# Windows 10/11 ならフォントをインストール
# Control Panel → Fonts → "Noto Sans JP" をインストール
```

### 問題 2: "メタデータが抽出されません"

**原因**: VRChat 標準出力ではない画像の可能性。

**確認方法**:

```bash
VRCSSAnnotationTool.exe --raw image.png
```

詳細なメタデータ情報が表示されます。

### 問題 3: 出力画像の品質が低い

**原因**: 背景判定の問題。

**確認**:

- 背景が真っ黒または真っ白か確認
- 非常に大きい解像度（8K 以上）の場合は処理に時間がかかる

### 問題 4: Eagle で メタデータが表示されない

**原因**: PNG が iTXt チャンク対応ではない（古いバージョンの可能性）。

**確認**:

```powershell
# PowerShell で XMP メタデータを確認
.\check-xmp.ps1 annotated/image.png
```

---

## 📊 機能一覧

### ✅ 完全実装済み（v1.0.0）

#### メタデータ処理

- [x] PNG メタデータ抽出（XMP, tEXt, iTXt, zTXt チャンク）
- [x] WebP メタデータ抽出（XMP, EXIF）
- [x] VRChat 固有フィールド抽出（WorldID, AuthorID など）
- [x] **PNG メタデータ永続化（iTXt UTF-8 形式）** ⭐ NEW
- [x] **メタデータ処理進行状況表示** ⭐ NEW

#### 画像処理

- [x] 背景色自動判定
- [x] テキスト描画（日本語対応）
- [x] SVG アイコン処理
- [x] rMQR コード生成

#### 出力形式

- [x] JSON 出力
- [x] NDJSON ストリーミング
- [x] PNG 出力
- [x] WebP 出力

---

## 🧪 テスト結果

| 解像度    | ファイル形式 | メタデータ永続化 | 実行時間 |
| --------- | ------------ | ---------------- | -------- |
| 3840x2160 | PNG          | ✅               | 1.2s     |
| 2560x1440 | PNG          | ✅               | 0.8s     |
| 1440x2560 | PNG          | ✅               | 0.7s     |
| 1920x1080 | WebP         | ✅               | 0.6s     |

**日本語メタデータ**: ✅ 完全対応  
**Eagle 互換性**: ✅ 確認済み  
**Adobe Bridge**: ✅ 確認済み

---

## 📄 ドキュメント

- [IMPLEMENTATION_COMPLETE.md](IMPLEMENTATION_COMPLETE.md) - 実装完了レポート
- [FEATURES_AND_STRUCTURE.md](FEATURES_AND_STRUCTURE.md) - 詳細機能説明書
- [SETUP.md](SETUP.md) - セットアップガイド
- [RELEASE_NOTES.md](RELEASE_NOTES.md) - リリースノート
- [WORK_PLAN.md](WORK_PLAN.md) - 今後の作業計画

---

## 🤝 貢献

バグ報告・機能リクエストは [Issues](https://github.com/yourusername/VRCSSAnnotationTool/issues) へ。

## 📝 ライセンス

MIT License - 詳細は [LICENSE](LICENSE) を参照。

---

## 🙋 FAQ

**Q: 対応ファイル形式は？**  
A: PNG, WebP, JPEG（JPEG は注釈追加後 PNG で出力）

**Q: 出力画像のサイズは？**  
A: 入力と同じ。余白は上部 69px のみ追加。

**Q: バッチ処理の上限は？**  
A: メモリ許す限り無制限。RAM 使用率に注意。

**Q: 外部サーバーに送信される？**  
A: No - すべてローカルで処理。

**Q: コマンドライン以外に GUI はある？**  
A: 現在のところ未実装。ドラッグ&ドロップ対応の `annotate.bat` をご利用ください。

---

**最終更新**: 2026-01-18  
**バージョン**: 1.0.0 (Production Ready)  
**ステータス**: ✅ 本番利用可能
