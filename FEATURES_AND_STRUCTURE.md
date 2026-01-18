# VRChat画像アノテーション機能仕様書

## 概要

VRChat、VirtualCastなどのアプリケーションから出力された画像のメタデータを抽出・解析し、画像に注釈（ワールド情報、撮影者、撮影日時など）を追加するツール。

---

## 主要機能一覧

### 1. メタデータ抽出

**対応ファイル形式**：PNG、WebP、JPEG

**抽出情報**：

- EXIF データ（全タグ）
- XMP メタデータ（PNG/WebP両方対応）
- VRChat固有情報：
  - WorldID（ワールドID）
  - WorldDisplayName（ワールド表示名）
  - AuthorID（撮影者ID）
- 撮影日時（ISO 8601形式）
- 作者名

**処理フロー**：

1. ファイル読み込み
2. ファイルタイプ判定（PNG/WebP/JPEG）
3. EXIF抽出（複数の方法で試行）
4. テキストメタデータ（XMP）抽出
5. VRChat専用フィールド抽出

---

### 2. CLI フラグシステム

| フラグ        | 説明                               | 用途                   |
| ------------- | ---------------------------------- | ---------------------- |
| `--json`      | JSON形式で出力                     | 機械可読形式での出力   |
| `--raw`       | デバッグ用に生のメタデータ表示     | トラブルシューティング |
| `--pretty`    | JSON を整形                        | `--json` と併用        |
| `--no-escape` | JSON出力時のHTMLエスケープを無効化 | 特殊文字の保持         |
| `--verbose`   | 詳細な人間向け出力                 | 診断情報表示           |
| `--no-human`  | 人間向け出力を抑制                 | 純粋なJSONのみ出力     |
| `--annotate`  | **メタデータを画像に追加**         | 画像に注釈を付けて保存 |

**出力ターゲット**：

- `--json` 単独：JSON を stdout に出力
- `--json` + `--annotate`：JSON は stderr に出力（画像ファイルは annotated/ に保存）
- `--no-human`：人間向けテキストを完全に抑制

---

### 3. 画像処理・アノテーション機能

#### 通常の画像処理フロー

1. **余白追加**：上部に 69px の余白を追加
   - 背景色：画像の暗さで自動判定（黒/白）
   - テキスト色：背景色の逆色

2. **画像暗度判定**（`isDarkImage()`）
   - サンプリング：全体の約10%を確認
   - 判定基準：平均輝度が 50% 未満 = 暗い画像
   - 暗い → 黒背景＋白テキスト
   - 明るい → 白背景＋黒テキスト

3. **余白にアノテーション追加**
   - タイムスタンプ（左側）
   - 撮影者情報（中央）
   - ワールド情報（右側）
   - rMQRコード（右上）

#### 特殊処理：2048x1440 解像度

**理由**：VRChat のプリントカメラ出力。既に撮影情報が画像に含まれているため、重複を避ける

**ケース1：ワールド情報あり**

- rMQRコード（3倍拡大）のみを白背景で右上に描画（**白背景固定：暗い画像でも反転しない**）
- テキストは追加しない

**ケース2：ワールド情報なし**

- 元の画像をそのままコピー
- 何も追記しない

---

### 4. テキスト・アイコン配置

#### レイアウト

```
[余白 69px]
  ↓
  [icon] [date] [gap] [icon] [author] [gap] [icon] [world] ... [QR Code]
```

#### 具体的な配置

- **Y座標**：48（基準）
  - 元々 38 から 10px 下げた
  - フォント 32px でのベースライン調整
- **X座標開始**：30px
- **アイコン間隔（gap）**：25px

#### コンポーネント詳細

**日付表示**

- アイコン：`calendar_today_24dp_434343.svg`
- フォント：**等幅フォント**（重要）
  "C:\Users\miwam\AppData\Local\Microsoft\Windows\Fonts\OCR-BK.otf"を使用
- 形式：「2026-01-11 SUN 21:02:14」
  - YYYY-MM-DD
  - 曜日（3文字大文字）
  - HH:MM:SS
- 曜日は自動計算（Goの`time.Parse()`使用）

**撮影者表示**

- アイコン：`photo_camera_24dp_434343.svg`
- フォント：標準フォント
- 表示内容：authorName フィールド

**ワールド情報表示**

- アイコン：`location_pin_24dp_434343.svg`
- フォント：標準フォント
- 表示内容：worldName フィールド

**QRコード**

- 形式：rMQRコード（長方形型 Micro QR Code）
- エンコード内容：ワールドURL（`https://vrchat.com/home/world/{worldID}`）
- 拡大率：**3倍**
- 白背景：QRコード周囲に 8px のパディング

---

### 5. フォント処理

#### 標準フォント（優先順）

Windows:

1. `C:\Windows\Fonts\msgothic.ttc`
2. `C:\Windows\Fonts\meiryo.ttc`
3. `C:\Windows\Fonts\YuGothM.ttc`
4. `C:\Windows\Fonts\NotoSansCJK-Regular.ttc`
5. `C:\Windows\Fonts\NotoSansJP-Regular.otf`

Linux:

- `/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc`
- `/usr/share/fonts/truetype/noto/NotoSansJP-Regular.ttf`
- `/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttf`

#### 等幅フォント（優先順）

Windows:

1.  "C:\Users\miwam\AppData\Local\Microsoft\Windows\Fonts\OCR-BK.otf" OCR-Bフォント
2.  `C:\Windows\Fonts\consola.ttf` (Consolas)
3.  `C:\Windows\Fonts\courier.ttf` (Courier New)

Linux:

- `/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf`
- `/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf`

**フォントが見つからない場合**：

- 等幅フォント未検出 → 標準フォントにフォールバック
- 標準フォント未検出 → エラーで処理中止

---

### 6. SVGアイコン処理

#### 処理フロー

1. `icon/` フォルダから SVG ファイル読み込み
2. SVG内の色コード `#434343` を指定色に置換
3. SVGをパース（oksvg ライブラリ）
4. 20x20px のサイズで設定
5. ラスタライズして RGBA 画像に変換

#### アイコン色調整

- 基本：テキスト色と同じ
- コントラスト調整：70% に淡色化
  ```
  R' = R * 0.7
  G' = G * 0.7
  B' = B * 0.7
  ```

#### SVGファイル必須

- `calendar_today_24dp_434343.svg` （日付用）
- `photo_camera_24dp_434343.svg` （撮影者用）
- `location_pin_24dp_434343.svg` （ワールド用）

---

### 7. 日時抽出とフォーマット

#### ファイル名からの日時抽出（`extractDateFromFilename()`）

**VRChat形式**

```
パターン1: VRChat_2026-01-15_22-52-38.319_3840x2160
パターン2: VRChat_2026-01-14_21-49-03.450_2048x1440
パターン3: VRChat_1920x1080_2022-06-02_03-11-38.751
```

→ ISO 8601形式に変換：`2026-01-15T22:52:38`

**VirtualCast/OculusQuest形式**

```
パターン4: com.vrchat.oculus.quest-20220330-003003
パターン5: jp.virtualcast.virtualcast.oculusquest-20211125-172536
```

→ ISO 8601形式に変換：`2022-03-30T00:30:03`

#### タイムスタンプフォーマット（`formatDateAsYMD()`）

```
入力：  "2026-01-15T09:38:22.0000000+09:00"
出力：  "2026-01-15 WED 09:38:22"
```

処理内容：

1. ISO 8601 形式を解析
2. 年月日、時分秒を抽出
3. 日付から曜日を計算
4. 曜日を 3文字英字大文字に変換
5. "YYYY-MM-DD DAY HH:MM:SS" 形式で結合

---

### 8. WebP/PNG メタデータ保持

#### WebP処理

1. 新しい画像を WebP エンコード
2. 元の WebP から EXIF/XMP チャンクを抽出
3. RIFF構造を再構築：
   ```
   [RIFF Header] [既存チャンク] [新EXIF] [新XMP]
   ```
4. ファイルサイズを更新

#### PNG処理（iTXt 実装版）✨ NEW 2026-01-18

**問題**：PNG ファイルに XMP メタデータが保存されていなかった

**解決**：tEXt チャンク → **iTXt チャンク** に変更

**チャンク形式の比較**：

| 形式     | 文字セット | 圧縮 | 国際対応    | 用途                         |
| -------- | ---------- | ---- | ----------- | ---------------------------- |
| tEXt     | ラテン1    | ×    | ×           | ASCII・英数字のみ            |
| **iTXt** | **UTF-8**  | △    | **✅ 対応** | **国際テキスト（日本語OK）** |
| zTXt     | ラテン1    | ✅   | ×           | 圧縮テキスト                 |

**iTXt チャンク構造**（RFC 2891準拠）：

```
Length (4 bytes)
└─ チャンク長
Type: "iTXt" (4 bytes)
└─ チャンクタイプ識別子
Data:
├─ Keyword (variable) + Null Terminator
│  例："XMP" + \0
├─ Compression Flag (1 byte)
│  0: 非圧縮 | 1: 圧縮
├─ Compression Method (1 byte)
│  0: deflate (RFC 1951)
├─ Language Tag (variable) + Null Terminator
│  例："en" + \0 | "" + \0 (空)
├─ Translated Keyword (variable) + Null Terminator
│  例："XMP" + \0 | "" + \0
└─ UTF-8 Text (variable)
   例：XML 形式のメタデータ
CRC32 (4 bytes)
└─ チェックサム
```

**実装処理** (`addXMPToPNG()` 関数)：

1. PNG ファイルシグネチャ検証（`\x89PNG\r\n\x1a\n`）
2. IEND チャンク位置特定（ファイル末尾から 12 bytes）
3. iTXt チャンク構築：
   ```go
   // Keyword
   data := []byte("XMP\x00")
   // Compression flags
   data = append(data, 0x00, 0x00)  // 非圧縮、方式0
   // Language tag
   data = append(data, '\x00')      // 空
   // Translated keyword
   data = append(data, '\x00')      // 空
   // UTF-8 XMP text
   data = append(data, []byte(xmpContent)...)
   ```
4. CRC32 チェックサム計算
5. IEND 前に挿入：
   ```
   PNG Data | NEW iTXt Chunk | IEND Chunk
   ```

**メリット**：

- ✅ 日本語・中国語など多言語対応
- ✅ Eagle・Photoshop等の外部ツールで表示可能
- ✅ WebP と同等のメタデータ互換性実現
- ✅ 最小限のファイルサイズ増加（～1200 bytes）

**テスト結果**（2026-01-18）：

```
✅ VRChat_2026-01-01_00-01-20.301_3840x2160.png
   - iTXt チャンク位置: Chunk #203 (IEND直前)
   - 検証: hexdump 確認、CRC32 正常
   - 抽出: XMP 1178 bytes 完全復元
```

#### PNG テキストチャンク抽出

複数形式に対応した抽出処理（`extractTextualMetadataFromPNG()`）：

```
tEXt チャンク
├─ Keyword + Null
└─ Latin-1 Text

iTXt チャンク (NEW)
├─ Keyword + Null
├─ Compression Flag (1 byte)
├─ Compression Method (1 byte)
├─ Language Tag + Null
├─ Translated Keyword + Null
└─ UTF-8 Text (圧縮の場合は deflate)

zTXt チャンク
├─ Keyword + Null
├─ Compression Method (1 byte)
└─ Deflate Compressed Data
```

自動判別で全形式対応。zTXt 圧縮フラグも正しく処理：

```go
if compressionMethod == 0 {  // deflate
    reader := zlib.NewReader(bytes.NewReader(compressed))
    decompressed, _ := io.ReadAll(reader)
    text = string(decompressed)
}
```

#### JPEG処理

- JPEG は EXIF を基本的に保持しないため、別途処理なし

---

### 9. QRコード処理

#### rMQRコード生成（`generateRMQR()`）

- ライブラリ：`github.com/shogo82148/qrcode/rmqr`
- 生成内容：ワールドURL
  ```
  https://vrchat.com/home/world/{worldID}
  ```

```
- エラー訂正レベル：`rmqr.LevelM` （中程度）

#### QRコード反転処理
- 黒背景の場合：QRコード反転（黒と白を入れ替え）
- 計算式：`255 - 元の値`
- 目的：背景に対するコントラスト確保

---

### 10. ログシステム

#### ファイル
- 場所：`annotated/annotate.log`
- 形式：`[YYYY-MM-DD HH:MM:SS] メッセージ`

#### ログ出力タイミング
- 処理開始
- 警告（ワールドID未検出など）
- エラー発生
- 処理完了

#### メタデータ処理進行状況表示 ✨ NEW 2026-01-18

stderr にリアルタイムで処理状況を表示。アノテーション処理中の詳細なフィードバック：

**PNG 処理時**：
```

[Metadata] PNG XMP extracted (1178 bytes)...
[SUCCESS] PNG metadata written

```

**WebP 処理時**：
```

[Metadata] WebP XMP: OK (1178 bytes)
[Metadata] PNG XMP: NOT_FOUND (0 bytes)
[Metadata] Writing WebP metadata...
[SUCCESS] WebP metadata written

```

**エラー時**：
```

[ERROR] PNG metadata write failed: invalid PNG signature

```

**出力ログレベル**：

| プレフィックス | 意味 | 出力先 |
|---------------|------|--------|
| `[Metadata]` | 情報・処理中 | stderr |
| `[SUCCESS]` | 処理完了 | stderr |
| `[ERROR]` | エラー発生 | stderr |
| `[WARNING]` | 警告 | stderr |

実装箇所：`addMetadataToImage()` 関数内（PNG/WebP 両方対応）

#### スレッドセーフ
- `logMutex` でロック管理
- 複数ファイル同時処理に対応

---

### 11. 出力ファイル構成

```

入力画像/
├── image1.png
├── image2.webp
└── ...

↓ 実行

annotated/
├── image1.png （ワールド情報ありの場合のみQR付き）
├── image2.webp
├── ...
└── annotate.log （実行ログ）

````

#### ファイル形式の判定と保存
- PNG入力 → PNG出力
- WebP入力 → WebP出力
- JPEG入力 → PNG出力（JPEG への書き込みは未サポート）

---

### 12. JSON出力フォーマット

```json
{
  "fileName": "path/to/image.png",
  "fileType": "PNG",
  "imageWidth": 3840,
  "imageHeight": 2160,
  "exifRawBase64": "base64encodeddata...",
  "exifEntries": [
    {"tag": "DateTime", "value": "2026:01:15 09:38:22"},
    {"tag": "ImageDescription", "value": "..."},
    {"tag": "Artist", "value": "..."}
  ],
  "xmpRawPNG": "<?xml version=\"1.0\"...>...",
  "xmpFieldsPNG": [
    {"ns": "http://ns.adobe.com/xap/1.0/", "name": "CreateDate", "value": "2026-01-15T09:38:22.0000000+09:00"},
    ...
  ],
  "worldID": "wrld_xxxxx",
  "worldName": "Example World",
  "authorID": "usr_xxxxx",
  "shootDate": "2026-01-15T09:38:22.0000000+09:00",
  "authorName": "Author Name"
}
````

---

## 技術スタック

### Go パッケージ

- `github.com/chai2010/webp` - WebP エンコード/デコード
- `github.com/dsoprea/go-exif/v3` - EXIF 解析
- `github.com/golang/freetype` - テキスト描画
- `github.com/golang/freetype/truetype` - TTF フォント解析
- `https://github.com/shogo82148/qrcode` - rMQRコード生成
  使い方
  qrcode.Encodeにバイト列を渡すと、 QRコードの画像をimage.Imageとして返します。 あと通常の画像と同じように扱えるので、image/pngなどで書き出してください。

package main

import (
"bytes"
"image/png"
"log"
"os"

    "github.com/shogo82148/qrcode"

)

func main() {
img, err := qrcode.Encode([]byte("Hello QR Code!"))
if err != nil {
log.Fatal(err)
}
var buf bytes.Buffer
if err := png.Encode(&buf, img); err != nil {
log.Fatal(err)
}
if err := os.WriteFile(filename, buf.Bytes(), 0o644); err != nil {
log.Fatal(err)
}
}
QRコードは日本生まれの規格なので、漢字を効率的に格納するモードがあります。 JIS X 0208の範囲内にある文字は自動的に漢字モードになります。

- `github.com/srwiley/oksvg` - SVGパース
- `github.com/srwiley/rasterx` - SVGラスタライズ
- `golang.org/x/image/draw` - 画像スケーリング

### 標準ライブラリ使用

- `image`, `image/color`, `image/draw`, `image/png` - 画像処理
- `encoding/json`, `encoding/xml`, `encoding/base64`, `encoding/binary` - データ処理
- `compress/zlib` - 圧縮（PNG zTXt 対応）
- `flag` - CLI 引数解析
- `sync` - ロック管理
- `time` - 日時処理

---

## 実行例

### メタデータ抽出

```bash
main.exe image.png
```

### JSON出力（複数ファイル）

```bash
main.exe --json image1.png image2.webp
```

### NDJSON ストリーミング

```bash
main.exe --json --ndjson image1.png image2.png | jq .
```

### 画像にアノテーション追加

```bash
main.exe --annotate image.png
```

→ `annotated/image.png` に保存

### デバッグモード

```bash
main.exe --json --raw --verbose image.png 2>&1
```

---

## 重要な定数・設定値

| 項目           | 値       | 用途             |
| -------------- | -------- | ---------------- |
| 余白高さ       | 69px     | 通常処理時       |
| テキストY座標  | 48       | ベースライン     |
| テキストX開始  | 30px     | 左端マージン     |
| テキスト間隔   | 25px     | アイコン間隔     |
| フォントサイズ | 32px     | テキスト描画     |
| アイコンサイズ | 20x20px  | SVG出力          |
| QR拡大率       | 3倍      | 両処理共通       |
| QRパディング   | 8px      | 白背景マージン   |
| 画像暗度閾値   | 50%      | 背景色判定       |
| アイコン淡色率 | 70%      | コントラスト調整 |
| サンプリング率 | ~10%     | 暗度判定         |
| rMQRレベル     | LevelM   | エラー訂正中程度 |
| 2048x1440判定  | 正確一致 | 特殊処理条件     |

---

## エラーハンドリング

### 優雅な失敗

- フォント未検出 → エラーメッセージ表示（処理中止）
- メタデータ未検出 → 警告をログに記録（処理継続）
- アイコン未検出 → スキップ（テキストのみ表示）

### ユーザーフィードバック

- `annotated/annotate.log` に詳細ログ
- stderr に警告・エラー出力
- stdout に人間向けテキスト出力

---

## 今後の改善案

1. JPEG への直接書き込みサポート
2. バッチ処理の進捗表示
3. カスタマイズ可能なテンプレート
4. 複数言語対応（曜日表示など）
5. GUI インターフェース
6. 設定ファイル（.json/.toml）対応
