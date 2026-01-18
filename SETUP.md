# セットアップガイド - VRChat Image Annotation Tool

このドキュメントは、**開発環境を構築したい方**向けのセットアップ手順です。

通常の利用ユーザーは [README.md](README.md) をご覧ください。

---

## 前提条件

### 動作確認環境

| 項目             | 仕様                   |
| ---------------- | ---------------------- |
| OS               | Windows 10/11 (64-bit) |
| Go               | 1.25.5 以上            |
| RAM              | 2GB 以上               |
| ディスク空き容量 | 500MB 以上             |

### 必須ソフトウェア

- **Go 1.25.5 以上**
- **Git**（リポジトリからクローンする場合）
- **PowerShell 5.1 以上**（Windows 10/11 標準装備）

---

## 📥 インストール手順

### Step 1: Go のインストール

#### Windows での インストール

1. [Go Download ページ](https://golang.org/dl/) にアクセス
2. `go1.25.5.windows-amd64.msi` をダウンロード
3. インストーラーを実行
4. インストール完了後、ターミナルを再起動

#### インストール確認

```powershell
go version
# 結果例: go version go1.25.5 windows/amd64
```

### Step 2: Git のインストール（オプション）

GitHub からクローンする場合のみ必要。

1. [Git Download ページ](https://git-scm.com/download/win) からダウンロード
2. インストーラーを実行
3. デフォルト設定で進める

#### インストール確認

```powershell
git --version
# 結果例: git version 2.43.0.windows.1
```

### Step 3: リポジトリのセットアップ

#### パターン A: Git でクローン（推奨）

```powershell
# リポジトリをクローン
git clone https://github.com/yourusername/VRCSSAnnotationTool.git
cd VRCSSAnnotationTool

# ブランチ確認
git branch
# 結果: * main
```

#### パターン B: ZIP ファイルをダウンロード

1. GitHub のリポジトリページにアクセス
2. `Code` → `Download ZIP` をクリック
3. `VRCSSAnnotationTool-main.zip` をダウンロード
4. 任意のフォルダに解凍
5. PowerShell でそのフォルダに移動

```powershell
cd VRCSSAnnotationTool-main
```

### Step 4: 依存パッケージのインストール

```powershell
# go.mod と go.sum をもとにパッケージをダウンロード
go mod download

# 依存関係を確認
go mod graph | head -20
```

**インストール対象パッケージ**:

- github.com/chai2010/webp
- github.com/dsoprea/go-exif/v3
- github.com/golang/freetype
- github.com/shogo82148/qrcode/rmqr
- github.com/srwiley/oksvg

---

## 🔨 ビルド手順

### 開発版ビルド（デバッグ情報付き）

```powershell
# 現在のディレクトリでビルド
go build -o VRCSSAnnotationTool.exe

# 実行確認
.\VRCSSAnnotationTool.exe --json pic/VRChat_2026-01-01_00-01-20.301_3840x2160.png
```

### リリース版ビルド（最適化）

```powershell
# 最適化ビルド
go build -ldflags "-s -w" -o VRCSSAnnotationTool.exe

# ストリップ版ビルド（より小さいサイズ）
go build -ldflags "-s -w -X main.version=1.0.0" -o VRCSSAnnotationTool.exe
```

**ビルド結果**:

```
VRCSSAnnotationTool.exe  (13MB 程度)
```

---

## 🧪 テスト実行

### 基本テスト

```powershell
# PNG ファイルのメタデータを表示
.\VRCSSAnnotationTool.exe --json pic/VRChat_2026-01-01_00-01-20.301_3840x2160.png

# 出力例:
# {
#   "fileName": "pic/VRChat_2026-01-01_00-01-20.301_3840x2160.png",
#   "worldName": "Example World",
#   "authorName": "Your Name",
#   "shootDate": "2026-01-18T12:34:56.0000000+09:00"
# }
```

### 注釈追加テスト

```powershell
# 画像に注釈を追加
.\VRCSSAnnotationTool.exe --annotate pic/VRChat_2026-01-01_00-01-20.301_3840x2160.png

# 結果確認
ls -la annotated/
# VRChat_2026-01-01_00-01-20.301_3840x2160.png
# annotate.log
```

### バッチ処理テスト

```powershell
# 複数ファイルを処理
.\VRCSSAnnotationTool.exe --auto-annotate pic/*.png

# ログ確認
cat annotated/annotate.log
```

---

## 📝 フォント設定

このツールは以下のフォントを使用します。

### 必須フォント（日本語テキスト描画用）

Windows でのフォント検索順序：

1. Meiryo (`C:\Windows\Fonts\meiryo.ttc`)
2. Yu Gothic (`C:\Windows\Fonts\YuGothM.ttc`)
3. Noto Sans CJK (`C:\Windows\Fonts\NotoSansCJK-Regular.ttc`)

#### フォント のインストール

Windows 10/11 では大抵のフォントが既にインストールされていますが、必要な場合：

```powershell
# "Noto Sans JP" を Microsoft Store からインストール
# または、Microsoft Typography サイトからダウンロード

# インストール確認
Get-ChildItem C:\Windows\Fonts\*meiryo* -ErrorAction SilentlyContinue
```

### 等幅フォント（タイムスタンプ表示用）

1. OCR-B (`C:\Users\<YourName>\AppData\Local\Microsoft\Windows\Fonts\OCR-BK.otf`)
2. Consolas (`C:\Windows\Fonts\consola.ttf`)
3. Courier New (`C:\Windows\Fonts\courier.ttf`)

#### OCR-B フォントのインストール

```powershell
# ダウンロード（例）
# https://github.com/googlei18n/fonts/releases/download/noto-sans-v20180226/NotoSans-*.zip

# インストール後、以下のパスに配置
C:\Users\$env:USERNAME\AppData\Local\Microsoft\Windows\Fonts\OCR-BK.otf
```

---

## 🗂️ ディレクトリ構成

セットアップ完了後のディレクトリ構成：

```
VRCSSAnnotationTool/
├── main.go                        # メインソースコード
├── go.mod                         # Go モジュール定義
├── go.sum                         # パッケージチェックサム
├── VRCSSAnnotationTool.exe        # コンパイル済みバイナリ
├── README.md                      # ユーザードキュメント
├── SETUP.md                       # このファイル
├── RELEASE_NOTES.md               # リリースノート
├── IMPLEMENTATION_COMPLETE.md     # 実装完了レポート
├── FEATURES_AND_STRUCTURE.md      # 機能詳細仕様
├── WORK_PLAN.md                   # 作業計画
├── annotate.bat                   # ドラッグ&ドロップ用
├── check-xmp.bat                  # メタデータ確認用
├── check-xmp.ps1                  # PowerShell スクリプト
├── check.bat                      # テスト実行用
├── inspect-png.ps1                # PNG 検査用
├── icon/                          # SVG アイコン
│   ├── calendar_today_24dp_434343.svg
│   ├── location_pin_24dp_434343.svg
│   ├── photo_camera_24dp_434343.svg
│   └── ... (その他)
├── pic/                           # テストサンプル画像
│   ├── VRChat_2026-01-01_00-01-20.301_3840x2160.png
│   ├── VRChat_2025-12-08_12-11-20.489_2560x1440.png
│   └── ... (その他)
├── xmp/                           # XMP メタデータサンプル
│   └── (複数の .xml ファイル)
└── annotated/                     # 出力ディレクトリ（自動作成）
    ├── image.png
    └── annotate.log
```

---

## 🔧 開発環境のセットアップ（IDE）

### Visual Studio Code（推奨）

#### 1. VS Code をインストール

[Visual Studio Code ダウンロード](https://code.visualstudio.com/Download) からダウンロード。

#### 2. Go 拡張機能をインストール

```powershell
# VS Code の拡張機能マーケットプレイスで "Go" を検索
# または、以下をコマンドで実行
code --install-extension golang.go
```

#### 3. 拡張パッケージをインストール

VS Code で以下が自動インストールされます：

- gopls（Go Language Server）
- dlv（デバッガー）

#### 4. ワークスペースを開く

```powershell
code .
```

#### 推奨設定（.vscode/settings.json）

```json
{
  "go.lintOnSave": "package",
  "go.lintTool": "golangci-lint",
  "go.lintArgs": ["--fast"],
  "editor.formatOnSave": true,
  "[go]": {
    "editor.defaultFormatter": "golang.go",
    "editor.codeActionsOnSave": {
      "source.organizeImports": true
    }
  }
}
```

### GoLand IDE（有料・高機能）

公式 IDE：https://www.jetbrains.com/go/

---

## 🐛 デバッグ方法

### コマンドラインでのデバッグ

```powershell
# デバッグ情報付きビルド
go build -gcflags="all=-N -l" -o VRCSSAnnotationTool_debug.exe

# 実行（詳細ログ出力）
.\VRCSSAnnotationTool_debug.exe --verbose --raw pic/test.png
```

### dlv デバッガの使用

```powershell
# dlv インストール（初回のみ）
go install github.com/go-delve/delve/cmd/dlv@latest

# デバッグ実行
dlv debug

# ブレークポイント設定
(dlv) break main.main
(dlv) continue
(dlv) print imagePath
```

---

## 📊 ビルド情報の追加

バイナリにバージョン情報を埋め込む：

```powershell
$version = "1.0.0"
$buildDate = Get-Date -Format "2006-01-02"
$gitCommit = git rev-parse --short HEAD

go build `
    -ldflags "-X main.Version=$version -X main.BuildDate=$buildDate -X main.GitCommit=$gitCommit" `
    -o VRCSSAnnotationTool.exe
```

バージョン確認：

```powershell
.\VRCSSAnnotationTool.exe --version
# Version: 1.0.0 (built: 2026-01-18, commit: a1b2c3d)
```

---

## 🚀 クロスコンパイル（Linux/Mac）

### Linux 版バイナリの作成

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o VRCSSAnnotationTool_linux

# 元に戻す
$env:GOOS = "windows"
$env:GOARCH = "amd64"
```

### macOS 版バイナリの作成

```powershell
$env:GOOS = "darwin"
$env:GOARCH = "amd64"
go build -o VRCSSAnnotationTool_mac

# Apple Silicon 対応
$env:GOARCH = "arm64"
go build -o VRCSSAnnotationTool_mac_arm64
```

---

## ⚠️ トラブルシューティング

### エラー 1: "Go コマンドが見つかりません"

```
'go' is not recognized as an internal or external command
```

**解決**:

```powershell
# Go のインストールを確認
go version

# インストールされていない場合
# 再度インストール実行
```

### エラー 2: "モジュール go.mod が見つかりません"

```
go: go.mod not found in current directory or any parent directory
```

**解決**:

```powershell
# リポジトリのルートディレクトリにいることを確認
pwd
# D:\新しいフォルダー\新しいフォルダー

# go.mod が存在するか確認
ls go.mod

# なければ初期化（ただし既存プロジェクトでは不要）
go mod init VRCSSAnnotationTool
```

### エラー 3: "パッケージがダウンロード できません"

```
go: github.com/chai2010/webp: invalid version: unknown revision ...
```

**解決**:

```powershell
# go.sum をリセット
rm go.sum

# 再度ダウンロード
go mod download

# キャッシュをクリア
go clean -modcache
```

### エラー 4: "フォントが見つかりません"

```
ERROR: Font not found
```

**解決**: [📝 フォント設定](#-フォント設定) セクションを参照。

---

## 📚 参考資料

### 公式ドキュメント

- [Go 公式サイト](https://golang.org/)
- [Go モジュール ガイド](https://golang.org/wiki/Modules)
- [Go ビルド ドキュメント](https://golang.org/doc/install)

### 関連パッケージ

- [chai2010/webp](https://github.com/chai2010/webp)
- [golang/freetype](https://github.com/golang/freetype)
- [shogo82148/qrcode](https://github.com/shogo82148/qrcode)

---

**最終更新**: 2026-01-18  
**推奨環境**: Windows 10/11, Go 1.25.5+  
**サポート**: GitHub Issues
