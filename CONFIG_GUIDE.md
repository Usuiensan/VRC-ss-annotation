# 設定ガイド (Configuration Guide)

## 概要

VRCSSAnnotationTool は、設定ファイルを使用して再コンパイルなしに動作をカスタマイズできます。これにより、異なる環境や要件に対応した配布が容易になります。

## 設定ファイルの優先順位

プログラムは以下の順序で設定ファイルを探索します：

1. **環境変数** `VRCS_ANNOTATE_CONFIG` で指定されたパス
2. **annotate.config.json** （カレントディレクトリ）
3. **config.json** （カレントディレクトリ）

設定ファイルが見つからない場合は、デフォルト設定で動作します。

## 設定ファイルの形式

### ファイル名

- `annotate.config.json` - メイン設定ファイル（推奨）
- `config.json` - 代替設定ファイル

### ファイル内容

すべての設定は JSON 形式で記述します。`annotate.config.sample.json` をテンプレートとして使用してください。

## 設定項目詳細

### 基本設定

#### placeholderAuthorName

```json
{
  "placeholderAuthorName": "Guest"
}
```

- **型**: 文字列
- **デフォルト**: 空文字列
- **説明**: 指定した名前と一致する撮影者情報は画像に表示されません。空文字列で無効化します。
- **用途**: ゲストプレイヤーなど特定のプレイヤーをデフォルトで隠す場合に使用

#### outputDir

```json
{
  "outputDir": "D:\\VRChat\\Screenshots\\Annotated"
}
```

- **型**: 文字列
- **デフォルト**: 空文字列（入力画像と同じディレクトリ内の `annotated/` を使用）
- **説明**: アノテーション付き画像の出力ディレクトリパス
- **用途**: 出力先を固定フォルダに指定したい場合

### フォント設定

```json
{
  "fonts": {
    "monoFont": "C:\\Windows\\Fonts\\BIZ UDゴシック\\BIZ-UDGothicR.ttc",
    "mainFont": "C:\\Windows\\Fonts\\BIZ UDゴシック\\BIZ-UDGothicR.ttc",
    "fallbackFonts": [
      "C:\\Users\\username\\AppData\\Local\\Microsoft\\Windows\\Fonts\\Font1.ttf",
      "C:\\Users\\username\\AppData\\Local\\Microsoft\\Windows\\Fonts\\Font2.ttf"
    ]
  }
}
```

- **monoFont**: 日時表示用のモノスペースフォント（TrueType フォント）
- **mainFont**: その他テキスト表示用のフォント
- **fallbackFonts**: フォントが見つからない場合の代替フォント（複数指定可、最初に見つかったものを使用）

**パス指定方法**:

- Windows システムフォント: `C:\\Windows\\Fonts\\...`
- ユーザーフォント: `C:\\Users\\username\\AppData\\Local\\Microsoft\\Windows\\Fonts\\...`
- 相対パス: `./fonts/MyFont.ttf`

### レイアウト設定

```json
{
  "layout": {
    "marginTop": 69,
    "marginLeft": 20,
    "marginRight": 60,
    "iconSize": 28,
    "iconSpacing": 12,
    "gapSize": 28,
    "mainFontSize": 32
  }
}
```

- **marginTop**: 上部余白の高さ（ピクセル）
- **marginLeft**: 左側マージン（ピクセル）
- **marginRight**: 右側マージン（ピクセル）
- **iconSize**: アイコンサイズ（ピクセル）
- **iconSpacing**: アイコンとテキストの間隔（ピクセル）
- **gapSize**: セクション間の間隔（ピクセル）
- **mainFontSize**: テキストのフォントサイズ（ポイント）

### SVG アイコンパス設定

```json
{
  "iconPath": "./icon"
}
```

- **型**: 文字列
- **デフォルト**: `./icon`
- **説明**: SVG アイコンを格納するディレクトリ

**パス指定方法**:

- 相対パス: `./icon` または `../shared/icons`
- 絶対パス: `C:\\App\\Resources\\Icons`

### 色設定

```json
{
  "colors": {
    "textColorLight": "000000",
    "textColorDark": "FFFFFF",
    "backgroundColorLight": "FFFFFF",
    "backgroundColorDark": "000000"
  }
}
```

- **textColorLight**: 明るい背景時のテキスト色（16進数 RGB、先頭の `#` は不要）
- **textColorDark**: 暗い背景時のテキスト色
- **backgroundColorLight**: 明るい背景時のマージン背景色
- **backgroundColorDark**: 暗い背景時のマージン背景色

### 日付表示設定 ⭐ NEW

```json
{
  "dateFormat": "2006-01-02 Mon 15:04:05"
}
```

- **dateFormat**: 撮影日の表示フォーマット（Go のレイアウト文字列）
  - 例: `2006-01-02 Mon 15:04:05` → `2026-01-22 Thu 12:34:56`
  - 例: `2006/01/02 15:04` → `2026/01/22 12:34`
  - 未指定の場合は `2006-01-02 Mon 15:04:05`

### 画像処理設定

```json
{
  "image": {
    "darkThreshold": 0.01,
    "qrScaleFactor": 3,
    "qrRightPadding": 60,
    "webpCompressionQuality": 100,
    "outputFormat": "auto",
    "supportedInputExtensions": [".png", ".webp", ".jpg", ".jpeg"]
  }
}
```

- **darkThreshold**: 画像が「暗い」と判定される平均輝度の閾値（0.0 ～ 1.0）
  - 値が小さいほど、より明るい画像を「暗い」と判定
  - デフォルト `0.01` = 平均輝度 1% 未満なら暗い
- **qrScaleFactor**: rMQR コードの拡大倍率（整数）
  - 値が大きいほど QR コードが大きく表示される
- **qrRightPadding**: QR コードの右端余白（ピクセル）
- **webpCompressionQuality**: WebP出力時の圧縮品質（1～100）
  - 100 = ロスレス圧縮（最高品質）
  - 値が小さいほど圧縮率が上がるが品質が低下
  - **`webpLossless: false` の場合のみ有効**
- **webpLossless**: WebP出力時の圧縮方式 ⭐ NEW
  - `true`: ロスレス圧縮（可逆、品質100%だが容量大）
  - `false`: ロッシー圧縮（非可逆、`webpCompressionQuality` で品質制御）
  - デフォルト: `true`（従来の動作）
- **outputFormat**: 出力画像の形式 ⭐ NEW
  - `"auto"`: 入力ファイルの拡張子に合わせて自動判定（PNG 入力 → PNG 出力）
  - `"png"`: すべての入力を PNG で出力
  - `"webp"`: すべての入力を WebP で出力
- **supportedInputExtensions**: 対応する入力ファイルの拡張子（配列） ⭐ NEW
  - デフォルト: `[".png", ".webp", ".jpg", ".jpeg"]`
  - この拡張子以外のファイルは処理されません

## 使用例

### 例1: 出力ディレクトリを指定

```json
{
  "outputDir": "D:\\VRChat\\Screenshots\\Annotated"
}
```

### 例2: フォントを変更

```json
{
  "fonts": {
    "mainFont": "C:\\Windows\\Fonts\\Arial.ttf",
    "monoFont": "C:\\Windows\\Fonts\\Courier New.ttf",
    "fallbackFonts": []
  }
}
```

### 例3: マージンを広くして大きなテキストを表示

```json
{
  "layout": {
    "marginTop": 120,
    "marginLeft": 30,
    "marginRight": 80,
    "iconSize": 40,
    "mainFontSize": 42
  }
}
```

### 例4: 複数ユーザー向けの設定（最小限の構成）

```json
{
  "placeholderAuthorName": "Guest",
  "outputDir": "C:\\Shared\\AnnotatedScreenshots",
  "layout": {
    "mainFontSize": 28
  }
}
```

### 例5: すべての出力を WebP で統一

```json
{
  "image": {
    "outputFormat": "webp"
  }
}
```

### 例6: 対応拡張子を TIFF と PNG のみに限定

```json
{
  "image": {
    "supportedInputExtensions": [".png", ".tiff", ".tif"]
  }
}
```

### 例7: 複数の出力フォーマット設定

```json
{
  "image": {
    "outputFormat": "auto",
    "qrScaleFactor": 4,
    "darkThreshold": 0.02
  }
}
```

## 環境変数による設定

### コンフィグファイルパスの指定

PowerShell:

```powershell
$env:VRCS_ANNOTATE_CONFIG = "C:\Config\my-vrcs-config.json"
.\VRCSSAnnotationTool.exe image.png
```

CMD:

```batch
set VRCS_ANNOTATE_CONFIG=C:\Config\my-vrcs-config.json
VRCSSAnnotationTool.exe image.png
```

## トラブルシューティング

### 設定ファイルが読み込まれない

1. ファイル名を確認（`annotate.config.json` または `config.json`）
2. JSON フォーマットが正しいか検証（無効な JSON はスキップされます）
3. ログファイルを確認: `annotate.log`
4. コマンドラインで以下を実行：
   ```powershell
   $env:VRCS_ANNOTATE_CONFIG = "C:\Path\To\Config.json"
   .\VRCSSAnnotationTool.exe image.png
   ```

### フォントが見つからない

1. フォントファイルのパスが正しいか確認
2. フォントファイルが実際に存在するか確認
3. 代替フォント（`fallbackFonts`）が設定されているか確認
4. ログファイルで警告を確認

### 色が正しく表示されない

1. 色コードが正しい16進数形式か確認（例：`"000000"` ）
2. RGB の順序が正しいか確認（RR GG BB）

## ベストプラクティス

1. **サンプルファイルから始める**
   - `annotate.config.sample.json` をコピーして編集

2. **必要な設定のみ指定**
   - 指定されていない項目はデフォルト値が使用されます

3. **パスは絶対パスで指定**
   - 相対パスが機能しない場合は絶対パスを使用

4. **ログを確認**
   - `annotate.log` で設定読み込み状況を確認

5. **複数ユーザーで共有する場合**
   - ユーザー固有パスではなく共有フォルダを使用
   - フォントも共有フォルダに配置

## 配布時のチェックリスト

- [ ] `annotate.config.sample.json` を付属
- [ ] 使用フォントがすべてインストール済みか確認
- [ ] SVG アイコンが `icon/` フォルダに含まれているか確認
- [ ] `README.md` に設定手順を記載
- [ ] よくある質問（FAQ）を作成

---

**最終更新**: 2026-01-22
