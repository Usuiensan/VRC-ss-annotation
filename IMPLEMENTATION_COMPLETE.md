# VRChat Image Annotation Tool - 実装完了レポート

## 📋 プロジェクト概要

VRChat/VirtualCast のスクリーンショットに対し、メタデータ（撮影日時、ワールド名、撮影者）を自動抽出・注釈化する Go 製ツール。PNG/WebP 形式で XMP メタデータを永続化し、外部ツール（Eagle等）での閲覧も対応。

---

## ✅ 実装状況

### 完了した機能（全18機能）

#### メタデータ抽出・解析

1. **ファイル形式検出** (`detectFileType`) - PNG/WebP/JPEG 判別 ✅
2. **PNG メタデータ抽出** (`extractPNGDimensions`, `extractTextualMetadataFromPNG`) ✅
3. **WebP メタデータ抽出** (`extractWebPDimensionsAndFlags`, `extractTextualMetadataFromWebP`) ✅
4. **テキストメタデータ解析** 
   - tEXt チャンク（ラテン1文字セット）
   - iTXt チャンク（UTF-8 国際テキスト）
   - zTXt チャンク（圧縮テキスト）
   ✅
5. **XMP パース** (`extractVRChatFromXMP`, `extractDateFromXMP`, `extractAuthorFromXMP`) ✅
6. **VRChat メタデータ統合** (`readVRChatExifPNG`) ✅

#### 画像処理・アノテーション

7. **明度判定** (`isDarkImage`) - 背景が黒い画像を判定 ✅
8. **SVG アイコン処理** (`loadSVGIcon`) - 色置き換え + 20×20 にリサイズ ✅
9. **rMQRコード生成** (`generateRMQR`) - 背景に応じて反転対応 ✅
10. **画像反転** (`invertImage`) - 黒白ピクセル反転 ✅
11. **テキスト描画** (`addTextToImage`) - メタデータ + アイコン + QRコード配置 ✅
12. **メタデータ永続化**
    - PNG iTXt チャンク追加 (`addXMPToPNG`)
    - WebP XMP チャンク追加 (`addXMPToWebP`)
    ✅

#### 日時処理

13. **ファイル名日時抽出** (`extractDateFromFilename`) - VRChat 標準形式対応 ✅
14. **日時フォーマット** (`formatDateAsYMD`) - "YYYY-MM-DD DAY HH:MM:SS" 形式 ✅

#### CLI・オートメーション

15. **JSON 出力** (`--json`, `--pretty`, `--ndjson`) ✅
16. **アノテーション** (`--annotate`, `--auto-annotate`) ✅
17. **ドラッグ&ドロップ対応** (`annotate.bat`) ✅
18. **ログ出力** (`annotated/annotate.log`) ✅

### 最新機能（2026-01-18追加）

#### PNG メタデータ永続化 ✅NEW

**問題**: PNG メタデータが保存されていなかった

**解決策**:
- **tEXt チャンク** → **iTXt チャンク** に変更
  - tEXt：ラテン1文字セット（日本語非対応）
  - iTXt：UTF-8 国際テキスト（日本語対応）
  
**実装詳細** (`addXMPToPNG` 関数):
```
PNG ファイル構造:
┌─────────────────┐
│ PNG Signature   │ (8 bytes: \x89PNG\r\n\x1a\n)
├─────────────────┤
│ IHDR チャンク   │
├─────────────────┤
│ IDAT チャンク   │ (画像データ)
│ (複数)          │
├─────────────────┤
│ iTXt チャンク   │ ← 新規挿入
│ (メタデータ)    │
├─────────────────┤
│ IEND チャンク   │ (最後の8バイト)
└─────────────────┘

iTXt フォーマット:
Keyword\0 + CompFlag(1) + CompMethod(1) + LangTag\0 + TransKeyword\0 + UTF-8 Text
```

**効果**:
- Eagle 等の外部ツールでメタデータ表示可能
- WebP と同等の互換性確保
- ファイルサイズ効率化（1200バイト程度のオーバーヘッド）

#### メタデータ処理進行状況表示 ✅NEW

PNG/WebP 処理中にリアルタイムで以下を表示：

```
[Metadata] PNG XMP extracted (1178 bytes)...
[SUCCESS] PNG metadata written

[Metadata] WebP XMP: OK (1178 bytes)
[Metadata] PNG XMP: NOT_FOUND (0 bytes)
[Metadata] Writing WebP metadata...
[SUCCESS] WebP metadata written
```

---

## 📊 テスト結果（最新）

### 2026-01-18 テスト

```
✅ VRChat_2026-01-01_00-01-20.301_3840x2160.png
   - 解像度: 3840×2160
   - 入力メタデータ: XMP（1178 bytes）
   - 処理: PNG アノテーション + iTXt 追加
   - 出力: 6,606,881 bytes
   - メタデータチャンク: Chunk 203 (iTXt), Chunk 204 (IEND)
   - 確認: ✅ メタデータ抽出成功

✅ VRChat_2025-12-08_12-11-20.489_2560x1440.png
   - 処理: PNG アノテーション + iTXt 追加
   - 出力メタデータ: worldName, authorName, shootDate 保持
   - 確認: ✅ メタデータ永続化確認

✅ VRChat_2025-12-07_22-53-31.235_1440x2560.png
   - 処理: PNG アノテーション + iTXt 追加
   - 確認: ✅ 正常完了
```

---

## 🛠️ 技術スタック

### 言語・フレームワーク

- **Go 1.25.5**
- **標準ライブラリ** (image, png, json, xml, flag, binary, crc32)

### 依存パッケージ

| パッケージ                        | 用途                     |
| --------------------------------- | ------------------------ |
| github.com/chai2010/webp          | WebP エンコード/デコード |
| github.com/golang/freetype        | テキスト描画（TrueType） |
| github.com/shogo82148/qrcode/rmqr | rMQR コード生成          |
| github.com/srwiley/oksvg          | SVG パース               |
| golang.org/x/image                | 高度な画像処理           |

---

## 📈 コード統計

| メトリクス     | 値          |
| -------------- | ----------- |
| 総行数         | 1,820+ 行   |
| 関数数         | 32 個       |
| PNG 処理関数   | 3 個 (新規) |

---

## 🔐 堅牢性対応

### PNG メタデータ処理

```
✅ PNG シグネチャ検証（\x89PNG...）
✅ IEND チャンク位置特定
✅ iTXt チャンク構造検証
✅ CRC32 チェックサム計算
✅ UTF-8 テキスト安全処理
```

### PNG テキストチャンク抽出

```
✅ tEXt チャンク対応（ラテン1）
✅ iTXt チャンク対応（UTF-8）
✅ zTXt チャンク対応（圧縮）
✅ 複数形式混在処理
✅ 圧縮フラグ判定
```

---

## 🚀 使用方法

### 基本的な使用

```bash
# JSON 出力
.\VRCSSAnnotationTool.exe --json image.png

# アノテーション追加
.\VRCSSAnnotationTool.exe --annotate image.png

# ドラッグ&ドロップ
annotate.bat image.png
```

---

## ✨ 品質指標

| 項目               | 状態                      |
| ------------------ | ------------------------- |
| コンパイル         | ✅ 成功 (Go 1.25.5)       |
| テスト             | ✅ 3/3 完全成功           |
| メタデータ永続化   | ✅ PNG/WebP 両対応        |
| ログ表示           | ✅ 進行状況表示実装       |
| ドキュメント       | ✅ 完備                   |

---

**最終更新**: 2026-01-18  
**ステータス**: 本番利用可能 ✅  
**メタデータ永続化**: 完全実装 ✅
