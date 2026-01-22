package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "image/jpeg"

	"github.com/chai2010/webp"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/shogo82148/qrcode/rmqr"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	xdraw "golang.org/x/image/draw"
	xfont "golang.org/x/image/font"
	_ "golang.org/x/image/webp"
)

var logMutex sync.Mutex

// グローバルコンフィグ構造体
var appConfig *Config

// Config はアプリケーション全体の設定を保持する構造体
type Config struct {
	PlaceholderAuthorName string         `json:"placeholderAuthorName"`
	OutputDir             string         `json:"outputDir"`
	DateFormat            string         `json:"dateFormat"`
	Fonts                 FontConfig     `json:"fonts"`
	IconPath              string         `json:"iconPath"`
	Layout                LayoutConfig   `json:"layout"`
	Colors                ColorConfig    `json:"colors"`
	Image                 ImageConfig    `json:"image"`
}

type FontConfig struct {
	MonoFont       string   `json:"monoFont"`
	MainFont       string   `json:"mainFont"`
	FallbackFonts  []string `json:"fallbackFonts"`
}

type LayoutConfig struct {
	MarginTop     int     `json:"marginTop"`
	MarginLeft    int     `json:"marginLeft"`
	MarginRight   int     `json:"marginRight"`
	IconSize      int     `json:"iconSize"`
	IconSpacing   int     `json:"iconSpacing"`
	GapSize       int     `json:"gapSize"`
	MainFontSize  float64 `json:"mainFontSize"`
}

type ColorConfig struct {
	TextColorLight      string `json:"textColorLight"`
	TextColorDark       string `json:"textColorDark"`
	BackgroundColorLight string `json:"backgroundColorLight"`
	BackgroundColorDark  string `json:"backgroundColorDark"`
}

type ImageConfig struct {
	DarkThreshold            float64  `json:"darkThreshold"`
	QRScaleFactor            int      `json:"qrScaleFactor"`
	QRRightPadding           int      `json:"qrRightPadding"`
	WebPCompressionQuality   int      `json:"webpCompressionQuality"`
	WebPLossless             bool     `json:"webpLossless"`
	OutputFormat             string   `json:"outputFormat"`
	SupportedInputExtensions []string `json:"supportedInputExtensions"`
}

// デフォルト設定を返す
func getDefaultConfig() *Config {
	return &Config{
		PlaceholderAuthorName: "",
		OutputDir: "",
		DateFormat: "2006-01-02 Mon 15:04:05",
		Fonts: FontConfig{
			MonoFont: "C:\\Windows\\Fonts\\BIZ UDゴシック\\BIZ-UDGothicR.ttc",
			MainFont: "C:\\Windows\\Fonts\\BIZ UDゴシック\\BIZ-UDGothicR.ttc",
			FallbackFonts: []string{
				"C:\\Users\\miwam\\AppData\\Local\\Microsoft\\Windows\\Fonts\\MPLUSRounded1c-Medium.ttf",
				"C:\\Users\\miwam\\AppData\\Local\\Microsoft\\Windows\\Fonts\\OCR-BK.otf",
			},
		},
		IconPath: "./icon",
		Layout: LayoutConfig{
			MarginTop:    69,
			MarginLeft:   20,
			MarginRight:  60,
			IconSize:     28,
			IconSpacing:  12,
			GapSize:      28,
			MainFontSize: 32.0,
		},
		Colors: ColorConfig{
			TextColorLight:       "000000",
			TextColorDark:        "FFFFFF",
			BackgroundColorLight: "FFFFFF",
			BackgroundColorDark:  "000000",
		},
		Image: ImageConfig{
			DarkThreshold:            0.01,
			QRScaleFactor:            3,
			QRRightPadding:           60,
			WebPCompressionQuality:   100,
			WebPLossless:             true,
			OutputFormat:             "auto",
			SupportedInputExtensions: []string{".png", ".webp", ".jpg", ".jpeg"},
		},
	}
}

// loadConfig は複数の候補ファイルから設定を読み込む
func loadConfig() {
	// デフォルト設定で初期化
	appConfig = getDefaultConfig()

	// 優先順序: annotate.config.json → config.json → 環境変数で指定されたファイル
	candidates := []string{"annotate.config.json", "config.json"}
	
	// 環境変数でコンフィグファイルパスが指定されている場合、先頭に追加
	if envConfigPath := os.Getenv("VRCS_ANNOTATE_CONFIG"); envConfigPath != "" {
		candidates = append([]string{envConfigPath}, candidates...)
	}

	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg Config
		if err := json.Unmarshal(b, &cfg); err != nil {
			appendLog(fmt.Sprintf("警告: コンフィグファイル解析エラー (%s): %v", p, err))
			continue
		}
		// デフォルト設定とマージ（空の値はデフォルトを使用）
		appConfig = mergeConfig(appConfig, &cfg)
		appendLog(fmt.Sprintf("コンフィグファイル読み込み成功: %s", p))
		return
	}

	// コンフィグファイルが見つからない場合は、デフォルト設定を使用
	appendLog("コンフィグファイルが見つかりません。デフォルト設定を使用します。")
}

// mergeConfig はデフォルト設定を上書きしない（空でない値のみ上書き）
func mergeConfig(def, override *Config) *Config {
	if override.PlaceholderAuthorName != "" {
		def.PlaceholderAuthorName = override.PlaceholderAuthorName
	}
	if override.OutputDir != "" {
		def.OutputDir = override.OutputDir
	}
	if override.DateFormat != "" {
		def.DateFormat = override.DateFormat
	}
	if override.Fonts.MonoFont != "" {
		def.Fonts.MonoFont = override.Fonts.MonoFont
	}
	if override.Fonts.MainFont != "" {
		def.Fonts.MainFont = override.Fonts.MainFont
	}
	if len(override.Fonts.FallbackFonts) > 0 {
		def.Fonts.FallbackFonts = override.Fonts.FallbackFonts
	}
	if override.IconPath != "" {
		def.IconPath = override.IconPath
	}
	if override.Layout.MarginTop > 0 {
		def.Layout.MarginTop = override.Layout.MarginTop
	}
	if override.Layout.MarginLeft > 0 {
		def.Layout.MarginLeft = override.Layout.MarginLeft
	}
	if override.Layout.MarginRight > 0 {
		def.Layout.MarginRight = override.Layout.MarginRight
	}
	if override.Layout.IconSize > 0 {
		def.Layout.IconSize = override.Layout.IconSize
	}
	if override.Layout.IconSpacing > 0 {
		def.Layout.IconSpacing = override.Layout.IconSpacing
	}
	if override.Layout.GapSize > 0 {
		def.Layout.GapSize = override.Layout.GapSize
	}
	if override.Layout.MainFontSize > 0 {
		def.Layout.MainFontSize = override.Layout.MainFontSize
	}
	if override.Colors.TextColorLight != "" {
		def.Colors.TextColorLight = override.Colors.TextColorLight
	}
	if override.Colors.TextColorDark != "" {
		def.Colors.TextColorDark = override.Colors.TextColorDark
	}
	if override.Colors.BackgroundColorLight != "" {
		def.Colors.BackgroundColorLight = override.Colors.BackgroundColorLight
	}
	if override.Colors.BackgroundColorDark != "" {
		def.Colors.BackgroundColorDark = override.Colors.BackgroundColorDark
	}
	if override.Image.DarkThreshold > 0 {
		def.Image.DarkThreshold = override.Image.DarkThreshold
	}
	if override.Image.QRScaleFactor > 0 {
		def.Image.QRScaleFactor = override.Image.QRScaleFactor
	}
	if override.Image.QRRightPadding > 0 {
		def.Image.QRRightPadding = override.Image.QRRightPadding
	}
	if override.Image.WebPCompressionQuality > 0 {
		def.Image.WebPCompressionQuality = override.Image.WebPCompressionQuality
	}
	// WebPLosslessは明示的に設定を上書き（falseも含む）
	def.Image.WebPLossless = override.Image.WebPLossless
	if override.Image.OutputFormat != "" {
		def.Image.OutputFormat = override.Image.OutputFormat
	}
	if len(override.Image.SupportedInputExtensions) > 0 {
		def.Image.SupportedInputExtensions = override.Image.SupportedInputExtensions
	}
	return def
}

func appendLog(message string) {
	logMutex.Lock()
	defer logMutex.Unlock()
	logPath := "annotate.log"
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		now := time.Now().Format("2006-01-02 15:04:05")
		f.WriteString("[" + now + "] " + message + "\n")
	}
}

// loadFontFromPaths は複数のパスからフォントを読み込み、最初に見つかったものを返す
func loadFontFromPaths(paths []string) []byte {
	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err == nil {
			return data
		}
	}
	return nil
}

// 指定解像度(2048x1440)か判定
func isPrintCameraResolutionOnly(img image.Image) bool {
	bounds := img.Bounds()
	return bounds.Dx() == 2048 && bounds.Dy() == 1440
}

// determineOutputFormat は出力フォーマットを決定する
// outputFormat が "auto" の場合は入力ファイルの拡張子から判定、
// "png" または "webp" の場合はそれを使用する
func determineOutputFormat(inputPath string, configFormat string) string {
	if configFormat == "" || configFormat == "auto" {
		// 入力ファイルの拡張子から判定
		if strings.HasSuffix(strings.ToLower(inputPath), ".webp") {
			return "webp"
		}
		return "png" // デフォルトは PNG
	}
	// コンフィグで指定されたフォーマットを使用
	format := strings.ToLower(configFormat)
	if format == "webp" || format == "png" {
		return format
	}
	return "png" // 無効な値の場合は PNG
}

// isSupportedInputFile は入力ファイルが対応拡張子か判定する
func isSupportedInputFile(filePath string, supportedExts []string) bool {
	if len(supportedExts) == 0 {
		// デフォルト対応拡張子
		supportedExts = []string{".png", ".webp", ".jpg", ".jpeg"}
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, supported := range supportedExts {
		if ext == strings.ToLower(supported) {
			return true
		}
	}
	return false
}

// adjustOutputPath は出力フォーマットに応じてファイルパスの拡張子を調整する
// 例: "image.png" + "webp" → "image.webp"
func adjustOutputPath(outputPath string, outputFormat string) string {
	if outputFormat == "" || outputFormat == "auto" {
		return outputPath
	}

	format := strings.ToLower(outputFormat)
	var newExt string
	
	if format == "webp" {
		newExt = ".webp"
	} else if format == "png" {
		newExt = ".png"
	} else {
		return outputPath // 無効なフォーマットの場合は変更しない
	}

	// 現在の拡張子を取得（元のケースのまま）
	oldExt := filepath.Ext(outputPath)

	// 既に正しい拡張子の場合は変更しない（大文字小文字を区別しない比較）
	if strings.ToLower(oldExt) == newExt {
		return outputPath
	}

	// 拡張子を置換
	if oldExt != "" {
		return outputPath[:len(outputPath)-len(oldExt)] + newExt
	}
	return outputPath + newExt
}

// 出力ディレクトリパスを取得（outputDirBaseが空の場合は画像と同じディレクトリ内の"annotated"を使用）
func getOutputDir(imagePath string) string {
	if appConfig.OutputDir != "" {
		return appConfig.OutputDir
	}
	return filepath.Join(filepath.Dir(imagePath), "annotated")
}

func main() {
	// コンフィグ読み込み
	loadConfig()
	// CLI flags
	jsonOut := flag.Bool("json", false, "出力をJSONにします")       // --json
	rawOut := flag.Bool("raw", false, "デバッグ用に生のメタデータを表示します") // --raw
	pretty := flag.Bool("pretty", false, "JSONを整形して出力します ( --json と併用 )")
	noEscape := flag.Bool("no-escape", false, "JSON出力時にHTMLエスケープを無効化します（危険）")
	ndjson := flag.Bool("ndjson", false, "JSON出力を1行ごとのNDJSONで出力します（--json と併用）")
	noHuman := flag.Bool("no-human", false, "人間向け出力を全て抑制します（--jsonと併用して純粋なJSONのみ出力する）")
	annotate := flag.Bool("annotate", false, "メタデータを画像に追加して出力します")
	autoAnnotate := flag.Bool("auto-annotate", false, "複数ファイルが指定された場合は自動的にアノテーションを有効化します")
	threads := flag.Int("threads", runtime.NumCPU(), "並列処理するワーカー数（デフォルトはCPUコア数）")
	outputDir := flag.String("output-dir", "", "アノテーション付き画像の出力ディレクトリ（指定なしの場合は画像ファイルと同じディレクトリ内のannotatedフォルダを作成）")
	lowPower := flag.Bool("low-power", false, "低負荷モード：スレッド数を1に制限し、処理間に遅延を加えてPCへの負荷を減らします")
	flag.Parse()

	// グローバル変数に出力ディレクトリを設定（CLIオプションがコンフィグを上書き）
	if *outputDir != "" {
		appConfig.OutputDir = *outputDir
	}

	// 低負荷モードの場合、スレッド数を1に制限
	if *lowPower {
		*threads = 1
	}

	if flag.NArg() < 1 {
		fmt.Println("画像ファイルをドラッグ＆ドロップしてください。")
		return
	}

	// 複数ファイルかつ--auto-annotateフラグの場合は--annotateを有効化
	if !*jsonOut && !*rawOut && !*annotate {
		*annotate = true
	}
	// 複数ファイルかつ--auto-annotateフラグの場合は--annotateを有効化（後方互換）
	if *autoAnnotate && flag.NArg() > 1 && !*annotate {
		*annotate = true
	}

	// If JSON output is requested, collect or stream JSON-only output
	if *jsonOut {
		if *ndjson {
			// Stream NDJSON: one JSON object per file, newline-delimited
			for _, path := range flag.Args() {
				meta, err := readVRChatExifPNG(path, true, *noHuman)
				if err != nil {
					fmt.Fprintf(os.Stderr, "エラー (%s): %v\n", path, err)
					continue
				}
				// Use encoder to control escaping
				enc := json.NewEncoder(os.Stdout)
				if *noEscape {
					enc.SetEscapeHTML(false)
				}
				// NDJSON typically shouldn't be pretty-printed
				if err := enc.Encode(meta); err != nil {
					fmt.Fprintf(os.Stderr, "JSON書き出しエラー (%s): %v\n", path, err)
				}
			}
			return
		}

		// Collect all metas into a JSON array
		var all []map[string]interface{}
		for _, path := range flag.Args() {
			meta, err := readVRChatExifPNG(path, true, *noHuman)
			if err != nil {
				fmt.Fprintf(os.Stderr, "エラー (%s): %v\n", path, err)
				continue
			}
			all = append(all, meta)
		}

		// Output array with selected escaping/format
		enc := json.NewEncoder(os.Stdout)
		if *noEscape {
			enc.SetEscapeHTML(false)
		}
		if *pretty {
			enc.SetIndent("", "  ")
		}
		if err := enc.Encode(all); err != nil {
			fmt.Fprintf(os.Stderr, "JSON書き出しエラー: %v\n", err)
		}
		return
	}

	// Non-JSON mode: print human-readable output per file
	if *annotate {
		paths := flag.Args()
		if len(paths) == 0 {
			fmt.Println("画像ファイルをドラッグ＆ドロップしてください。")
			return
		}
		jobs := make(chan string)
		var wg sync.WaitGroup
		worker := func() {
			defer wg.Done()
			for path := range jobs {
				meta, err := readVRChatExifPNG(path, true, true)
				if err != nil {
					msg := fmt.Sprintf("エラー (%s): %v", path, err)
					fmt.Fprintln(os.Stderr, msg)
					appendLog(msg)
					continue
				}
				date, _ := meta["shootDate"].(string)
				worldName, _ := meta["worldName"].(string)
				worldID, _ := meta["worldID"].(string)
				authorName, _ := meta["authorName"].(string)
				authorID, _ := meta["authorID"].(string)
				var worldURL string
				if worldID == "" {
					msg := fmt.Sprintf("警告 (%s): ワールドIDが見つかりません（日時のみ表示）", path)
					fmt.Fprintln(os.Stderr, msg)
					appendLog(msg)
					worldURL = ""
				} else {
					worldURL = fmt.Sprintf("https://vrchat.com/home/world/%s", worldID)
				}
				if err := addMetadataToImage(path, date, worldName, authorName, authorID, worldURL); err != nil {
					msg := fmt.Sprintf("画像処理エラー (%s): %v", path, err)
					fmt.Fprintln(os.Stderr, msg)
					appendLog(msg)
					continue
				}
				msg := fmt.Sprintf("処理完了: %s", path)
				fmt.Println(msg)
				appendLog(msg)
				// 低負荷モード時は処理後に遅延を加える
				if *lowPower {
					// time.Sleep(500 * time.Millisecond)
				}
			}
		}
		// start workers
		n := *threads
		if n < 1 {
			n = 1
		}
		wg.Add(n)
		for i := 0; i < n; i++ {
			go worker()
		}
		// feed jobs
		for _, p := range paths {
			jobs <- p
		}
		close(jobs)
		wg.Wait()
		
		// アノテーション完了後に待機
		// fmt.Println("\n数秒後に自動で終了します...")
		// time.Sleep(3 * time.Second)
		return
	}

	for _, path := range flag.Args() {
		fmt.Printf("\n--- ファイル: %s ---\n", path)
		_, _ = readVRChatExifPNG(path, *jsonOut, *noHuman)
	}

	if !*jsonOut && !*rawOut && !*annotate {
		// fmt.Println("\n数秒後に自動で終了します...")
		// time.Sleep(3 * time.Second)
	}
}

// detectFileType returns a simple file type name
func detectFileType(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return "PNG"
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "WebP"
	}
	if len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8 {
		return "JPEG"
	}
	return "Unknown"
}

func extractPNGDimensions(data []byte) (int, int, error) {
	if len(data) < 24 {
		return 0, 0, errors.New("not a valid PNG for dimension")
	}
	offset := 8
	for offset+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		chunkType := string(data[offset+4 : offset+8])
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + length
		chunkCRCEnd := chunkDataEnd + 4
		if chunkDataEnd > len(data) || chunkCRCEnd > len(data) {
			break
		}
		if chunkType == "IHDR" && length >= 8 {
			width := int(binary.BigEndian.Uint32(data[chunkDataStart : chunkDataStart+4]))
			height := int(binary.BigEndian.Uint32(data[chunkDataStart+4 : chunkDataStart+8]))
			return width, height, nil
		}
		offset = chunkCRCEnd
	}
	return 0, 0, errors.New("IHDR not found")
}

func parseLittle24(b []byte) int {
	return int(b[0]) | int(b[1])<<8 | int(b[2])<<16
}

func extractWebPDimensionsAndFlags(data []byte) (int, int, bool, bool, error) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0, false, false, errors.New("not a valid WebP")
	}
	offset := 12
	var hasAlpha, hasAnim bool
	var width, height int
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + size
		if chunkDataEnd > len(data) {
			break
		}
		switch chunkID {
		case "VP8X":
			if size >= 10 {
				b := data[chunkDataStart:chunkDataEnd]
				flags := b[0]
				hasAlpha = (flags & 0x10) != 0
				hasAnim = (flags & 0x02) != 0
				w := parseLittle24(b[4:7])
				h := parseLittle24(b[7:10])
				width = w + 1
				height = h + 1
			}
		case "ALPH":
			hasAlpha = true
		case "ANIM":
			hasAnim = true
		case "VP8 ":
			if size >= 10 {
				b := data[chunkDataStart:chunkDataEnd]
				if len(b) >= 10 {
					w := int(binary.LittleEndian.Uint16(b[6:8]))
					h := int(binary.LittleEndian.Uint16(b[8:10]))
					if w != 0 && h != 0 {
						width = w
						height = h
					}
				}
			}
		case "VP8L":
			if size >= 5 {
				b := data[chunkDataStart:chunkDataEnd]
				if len(b) >= 5 {
					packed := uint32(b[1]) | uint32(b[2])<<8 | uint32(b[3])<<16 | uint32(b[4])<<24
					w := int((packed & 0x3FFF) + 1)
					h := int(((packed >> 14) & 0x3FFF) + 1)
					if w != 0 && h != 0 {
						width = w
						height = h
					}
				}
			}
		}
		offset = chunkDataEnd
		if size%2 == 1 {
			offset++
		}
	}
	if width == 0 || height == 0 {
		return width, height, hasAlpha, hasAnim, errors.New("dimensions not found")
	}
	return width, height, hasAlpha, hasAnim, nil
}

// プレースホルダー関数（後で実装）
func extractExifFromPNG(data []byte) ([]byte, error) {
	if len(data) < 8 {
		return nil, errors.New("not a valid PNG")
	}

	offset := 8 // skip PNG signature
	for offset+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		chunkType := string(data[offset+4 : offset+8])
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + length
		chunkCRCEnd := chunkDataEnd + 4

		if chunkDataEnd > len(data) || chunkCRCEnd > len(data) {
			break
		}

		if chunkType == "eXIf" {
			return data[chunkDataStart:chunkDataEnd], nil
		}

		offset = chunkCRCEnd
	}

	return nil, errors.New("eXIf chunk not found")
}

func extractExifFromWebP(data []byte) ([]byte, error) {
	if len(data) < 12 {
		return nil, errors.New("not a valid WebP")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, errors.New("not a valid WebP")
	}
	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + size
		if chunkDataEnd > len(data) {
			break
		}
		if chunkID == "EXIF" {
			return data[chunkDataStart:chunkDataEnd], nil
		}
		offset = chunkDataEnd
		if size%2 == 1 {
			offset++
		}
	}
	return nil, errors.New("EXIF chunk not found")
}

func extractTextualMetadataFromPNG(data []byte) (string, error) {
	if len(data) < 8 {
		return "", errors.New("not a valid PNG")
	}

	signature := func(s string) bool {
		return strings.Contains(s, "<x:xmpmeta") || strings.Contains(s, "http://ns.adobe.com/xap/1.0/")
	}

	readITXt := func(d []byte) (string, bool) {
		// iTXt形式: Keyword\0 + CompressionFlag(1) + CompressionMethod(1) + LanguageTag + \0 + TranslatedKeyword + \0 + Text
		i := bytes.IndexByte(d, 0)
		if i == -1 || len(d) < i+2 {
			return "", false
		}
		rest := d[i+1:]
		if len(rest) < 2 {
			return "", false
		}
		compFlag := rest[0]
		// compMethod := rest[1]  // Usually 0 (deflate)
		rest = rest[2:]
		
		// Skip language tag
		langEnd := bytes.IndexByte(rest, 0)
		if langEnd == -1 {
			return "", false
		}
		rest = rest[langEnd+1:]
		
		// Skip translated keyword
		transEnd := bytes.IndexByte(rest, 0)
		if transEnd == -1 {
			return "", false
		}
		textBytes := rest[transEnd+1:]
		
		// Check compression flag
		if compFlag == 1 {
			// Compressed
			zr, err := zlib.NewReader(bytes.NewReader(textBytes))
			if err == nil {
				defer zr.Close()
				if decoded, err := io.ReadAll(zr); err == nil {
					return string(decoded), true
				}
			}
			return "", false
		}
		// Uncompressed
		return string(textBytes), true
	}

	readZTxt := func(d []byte) (string, bool) {
		i := bytes.IndexByte(d, 0)
		if i == -1 || len(d) < i+2 {
			return "", false
		}
		// zTXt形式: キーワード\0 圧縮フラグ(1) 圧縮メソッド(1) 圧縮データ
		rest := d[i+1:]
		if len(rest) < 2 {
			return "", false
		}
		compFlag := rest[0]
		// compMethod := rest[1]  // 通常は0（deflate）
		compData := rest[2:]
		
		if compFlag == 1 {
			// 圧縮されている場合
			zr, err := zlib.NewReader(bytes.NewReader(compData))
			if err != nil {
				return "", false
			}
			defer zr.Close()
			decoded, err := io.ReadAll(zr)
			if err != nil {
				return "", false
			}
			return string(decoded), true
		} else {
			// 圧縮されていない場合
			return string(compData), true
		}
	}

	offset := 8
	var firstText string
	for offset+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		chunkType := string(data[offset+4 : offset+8])
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + length
		chunkCRCEnd := chunkDataEnd + 4

		if chunkDataEnd > len(data) || chunkCRCEnd > len(data) {
			break
		}

		switch chunkType {
		case "tEXt":
			d := data[chunkDataStart:chunkDataEnd]
			var txt string
			if i := bytes.IndexByte(d, 0); i != -1 {
				txt = string(d[i+1:])
			} else {
				txt = string(d)
			}
			if firstText == "" {
				firstText = txt
			}
			if signature(txt) {
				return txt, nil
			}
		case "iTXt":
			txt, ok := readITXt(data[chunkDataStart:chunkDataEnd])
			if ok {
				if firstText == "" {
					firstText = txt
				}
				if signature(txt) {
					return txt, nil
				}
			}
		case "zTXt":
			txt, ok := readZTxt(data[chunkDataStart:chunkDataEnd])
			if ok {
				if firstText == "" {
					firstText = txt
				}
				if signature(txt) {
					return txt, nil
				}
			}
		}

		offset = chunkCRCEnd
	}

	if firstText != "" {
		return firstText, nil
	}

	return "", errors.New("textual metadata not found")
}

func extractTextualMetadataFromWebP(data []byte) (string, error) {
	if len(data) < 12 {
		return "", errors.New("not a valid WebP")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return "", errors.New("not a valid WebP")
	}
	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + size
		if chunkDataEnd > len(data) {
			break
		}
		if chunkID == "XMP " {
			return string(data[chunkDataStart:chunkDataEnd]), nil
		}
		offset = chunkDataEnd
		if size%2 == 1 {
			offset++
		}
	}
	return "", errors.New("XMP chunk not found")
}

func extractVRChatFromXMP(xmp string) (bool, string, string, string) {
	// Returns found, worldID, worldDisplayName, authorID
	dec := xml.NewDecoder(strings.NewReader(xmp))
	const vrcNS = "http://ns.vrchat.com/vrc/1.0/"
	var worldID, worldName, authorID string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Space == vrcNS {
				switch se.Name.Local {
				case "WorldID":
					var v string
					_ = dec.DecodeElement(&v, &se)
					worldID = strings.TrimSpace(v)
				case "WorldDisplayName":
					var v string
					_ = dec.DecodeElement(&v, &se)
					worldName = strings.TrimSpace(v)
				case "AuthorID":
					var v string
					_ = dec.DecodeElement(&v, &se)
					authorID = strings.TrimSpace(v)
				}
			}
		}
	}
	found := worldID != "" || worldName != "" || authorID != ""
	return found, worldID, worldName, authorID
}

// XMPから撮影日を取得する
func extractDateFromXMP(xmp string) string {
	dec := xml.NewDecoder(strings.NewReader(xmp))
	const xmpNS = "http://ns.adobe.com/xap/1.0/"
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Space == xmpNS && se.Name.Local == "CreateDate" {
				var v string
				_ = dec.DecodeElement(&v, &se)
				if v != "" {
					return strings.TrimSpace(v)
				}
			}
		}
	}
	return ""
}

// XMPから作者名を取得する
func extractAuthorFromXMP(xmp string) string {
	dec := xml.NewDecoder(strings.NewReader(xmp))
	const xmpNS = "http://ns.adobe.com/xap/1.0/"
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Space == xmpNS && se.Name.Local == "Author" {
				var v string
				_ = dec.DecodeElement(&v, &se)
				if v != "" {
					return strings.TrimSpace(v)
				}
			}
		}
	}
	return ""
}

// formatXMLString formats XML string with proper indentation
func formatXMLString(xmlStr string) string {
	if xmlStr == "" {
		return ""
	}
	
	var buf bytes.Buffer
	dec := xml.NewDecoder(strings.NewReader(xmlStr))
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// XML parsing failed, return original string
			return xmlStr
		}
		if err := enc.EncodeToken(tok); err != nil {
			return xmlStr
		}
	}
	if err := enc.Flush(); err != nil {
		return xmlStr
	}
	
	result := buf.String()
	if result == "" {
		return xmlStr
	}
	return result
}

func readVRChatExifPNG(filename string, jsonOut, noHuman bool) (map[string]interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ファイル読み込み失敗: %v\n", err)
		return nil, err
	}

	var humanOut io.Writer = os.Stdout
	if jsonOut {
		humanOut = os.Stderr
	}
	if noHuman {
		humanOut = io.Discard
	}

	ft := detectFileType(data)
	fmt.Fprintf(humanOut, "FileType: %s\n", ft)

	meta := map[string]interface{}{"fileName": filename, "fileType": ft}

	switch ft {
	case "PNG":
		if w, h, err := extractPNGDimensions(data); err == nil {
			fmt.Fprintf(humanOut, "ImageWidth: %dpx\n", w)
			fmt.Fprintf(humanOut, "ImageHeight: %dpx\n", h)
			meta["imageWidth"] = w
			meta["imageHeight"] = h
		}
	case "WebP":
		if w, h, hasAlpha, hasAnim, err := extractWebPDimensionsAndFlags(data); err == nil {
			fmt.Fprintf(humanOut, "ImageWidth: %dpx\n", w)
			fmt.Fprintf(humanOut, "ImageHeight: %dpx\n", h)
			fmt.Fprintf(humanOut, "Alpha: %v\n", map[bool]string{true: "Yes", false: "No"}[hasAlpha])
			fmt.Fprintf(humanOut, "Animation: %v\n", map[bool]string{true: "Yes", false: "No"}[hasAnim])
			meta["imageWidth"] = w
			meta["imageHeight"] = h
			meta["alpha"] = hasAlpha
			meta["animation"] = hasAnim
		}
	}

	// Try XMP (PNG)
	if t, err := extractTextualMetadataFromPNG(data); err == nil {
		meta["xmpRawPNG"] = t

		// VRChat用メタデータも抽出
		if ok, wid, wname, aid := extractVRChatFromXMP(t); ok {
			meta["worldID"] = wid
			meta["worldName"] = wname
			meta["authorID"] = aid
		}
		// 撮影日・作者名も抽出
		shootDate := extractDateFromXMP(t)
		if shootDate != "" {
			meta["shootDate"] = shootDate
		}
		authorName := extractAuthorFromXMP(t)
		if authorName != "" {
			meta["authorName"] = authorName
		}
	}
	
	// Try XMP (WebP)
	if t2, err := extractTextualMetadataFromWebP(data); err == nil {
		meta["xmpRawWebP"] = t2

		// Extract VRChat-specific metadata from WebP XMP
		if ok, wid, wname, aid := extractVRChatFromXMP(t2); ok {
			meta["worldID"] = wid
			meta["worldName"] = wname
			meta["authorID"] = aid
		}
		
		// Extract shoot date and author name
		shootDate := extractDateFromXMP(t2)
		if shootDate != "" {
			meta["shootDate"] = shootDate
		}
		
		authorName := extractAuthorFromXMP(t2)
		if authorName != "" {
			meta["authorName"] = authorName
		}
	}

	return meta, nil
}

func isDarkImage(img image.Image) bool {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// サンプリング: 全体の約10%を確認
	sampleStep := 1
	if w > 100 || h > 100 {
		sampleStep = (w + 99) / 10
	}

	var totalBrightness float64
	sampleCount := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleStep {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleStep {
			r, g, b, _ := img.At(x, y).RGBA()
			// RGBA returns 16-bit values
			brightness := float64(r+g+b) / 3.0 / 65535.0
			totalBrightness += brightness
			sampleCount++
		}
	}

	if sampleCount == 0 {
		return false
	}

	averageBrightness := totalBrightness / float64(sampleCount)
	return averageBrightness < appConfig.Image.DarkThreshold
}

// verifyMetadataIntegrity は元のファイルと出力ファイルのメタデータが完全一致しているかを確認
func verifyMetadataIntegrity(origData []byte, outputPath string, isWebP bool) (bool, error) {
	// 出力ファイルを読み込み
	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		return false, fmt.Errorf("出力ファイルの読み込みエラー: %v", err)
	}

	var origXMP, outputXMP string

	if isWebP {
		// WebP メタデータ抽出
		origXMP2, _ := extractTextualMetadataFromWebP(origData)
		origXMP = origXMP2
		outputXMP2, _ := extractTextualMetadataFromWebP(outputData)
		outputXMP = outputXMP2
	} else {
		// PNG メタデータ抽出
		origXMP2, _ := extractTextualMetadataFromPNG(origData)
		origXMP = origXMP2
		outputXMP2, _ := extractTextualMetadataFromPNG(outputData)
		outputXMP = outputXMP2
	}

	// メタデータが完全一致しているか確認
	if origXMP != outputXMP {
		return false, fmt.Errorf("メタデータ不一致: 元のサイズ=%d, 出力のサイズ=%d", len(origXMP), len(outputXMP))
	}

	if origXMP == "" {
		// メタデータがない場合は警告だが続行
		return true, nil
	}

	return true, nil
}

func addMetadataToImage(imagePath string, date string, worldName string, authorName string, authorID string, worldURL string) error {
	// 元のファイルデータを読み込み（メタデータ取得用）
	origData, err := os.ReadFile(imagePath)
	if err != nil {
		return err
	}

	// 画像を読み込む
	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 画像をデコード
	img, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// プリントカメラ解像度判定
	if isPrintCameraResolutionOnly(img) {
		if worldURL == "" {
			// ワールド情報なし → 元画像をそのまま保存
			outputDir := getOutputDir(imagePath)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return err
			}
			outputPath := filepath.Join(outputDir, filepath.Base(imagePath))
			
			// 元画像をコピー
			origData, err := os.ReadFile(imagePath)
			if err != nil {
				return err
			}
			return os.WriteFile(outputPath, origData, 0644)
		}
		
		// ワールド情報あり → rMQRコードのみ白背景で右上に描画
		outImg := image.NewRGBA(bounds)
		draw.Draw(outImg, bounds, img, bounds.Min, draw.Src)

		// QR生成とスケーリング（NearestNeighborで設定値倍）
		// For print camera resolution (2048x1440) always use a white-background QR (no inversion)
		qrImg, err := generateRMQR(worldURL, false)
		if err == nil {
			qrBounds := qrImg.Bounds()
			scaleFactor := appConfig.Image.QRScaleFactor
			scaledWidth := qrBounds.Dx() * scaleFactor
			scaledHeight := qrBounds.Dy() * scaleFactor
			qrX := width - scaledWidth - appConfig.Image.QRRightPadding
			if qrX < 0 {
				qrX = 0
			}
			qrY := 4
			if qrY < 0 {
				qrY = 0
			}

			scaledQR := image.NewRGBA(image.Rect(0, 0, scaledWidth, scaledHeight))
			xdraw.NearestNeighbor.Scale(scaledQR, scaledQR.Bounds(), qrImg, qrBounds, draw.Src, nil)

			// 白背景を敷いてからQRを重ねる
			bgRect := image.Rect(qrX, qrY, qrX+scaledWidth, qrY+scaledHeight)
			draw.Draw(outImg, bgRect, &image.Uniform{color.White}, image.Point{}, draw.Src)
			draw.Draw(outImg, bgRect, scaledQR, image.Point{}, draw.Over)
		}
		
		outputDir := getOutputDir(imagePath)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}
		outputPath := filepath.Join(outputDir, filepath.Base(imagePath))
		
		// 出力フォーマットを決定
		outputFormat := determineOutputFormat(imagePath, appConfig.Image.OutputFormat)
		isWebP := outputFormat == "webp"

		// 出力フォーマットに応じて拡張子を調整
		outputPath = adjustOutputPath(outputPath, outputFormat)
		
		if isWebP {
			var buf bytes.Buffer
			quality := float32(appConfig.Image.WebPCompressionQuality)
			if quality <= 0 || quality > 100 {
				quality = 100
			}
			err = webp.Encode(&buf, outImg, &webp.Options{Lossless: appConfig.Image.WebPLossless, Quality: quality})
			if err != nil {
				return err
			}
			
			outFile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer outFile.Close()
			_, err = outFile.Write(buf.Bytes())
			if err != nil {
				return err
			}
			
			// WebP 保存後に XMP メタデータを追加
			xmpAdded := false
			webpXMP, webpErr := extractTextualMetadataFromWebP(origData)
			pngXMP, pngErr := extractTextualMetadataFromPNG(origData)
			
fmt.Fprintf(os.Stderr, "  [Metadata] WebP XMP: %s (%d bytes)\n", func() string {
			if webpErr != nil { return "ERROR" }
			if webpXMP == "" { return "NOT_FOUND" }
			return "OK"
		}(), len(webpXMP))
		fmt.Fprintf(os.Stderr, "  [Metadata] PNG XMP: %s (%d bytes)\n", func() string {
			if pngErr != nil { return "ERROR" }
			if pngXMP == "" { return "NOT_FOUND" }
			return "OK"
			}(), len(pngXMP))
			
			if webpErr == nil && webpXMP != "" {
				fmt.Fprintf(os.Stderr, "  [Metadata] Writing WebP metadata...\n")
				if err := addXMPToWebP(outputPath, webpXMP); err != nil {
					fmt.Fprintf(os.Stderr, "  [ERROR] WebP metadata write failed: %v\n", err)
					return err
				}
				fmt.Fprintf(os.Stderr, "  [SUCCESS] WebP metadata written\n")
				xmpAdded = true
			}
			// PNG からの変換時は XMP を追加してみる
			if !xmpAdded && pngErr == nil && pngXMP != "" {
				fmt.Fprintf(os.Stderr, "  [Metadata] Writing PNG->WebP metadata...\n")
				if err := addXMPToWebP(outputPath, pngXMP); err != nil {
					fmt.Fprintf(os.Stderr, "  [ERROR] PNG->WebP metadata write failed: %v\n", err)
					return err
				}
				fmt.Fprintf(os.Stderr, "  [SUCCESS] PNG->WebP metadata written\n")
				xmpAdded = true
			}
			
			// メタデータが追加されたかチェック
			if !xmpAdded {
				fmt.Fprintf(os.Stderr, "  [WARNING] Print camera resolution WebP (%s) has no metadata\n", imagePath)
			} else {
				fmt.Fprintf(os.Stderr, "  [SUCCESS] WebP metadata processing completed\n")
			}
			
			// メタデータ検証は暫定的に無効化（保存確認待ち）
			return nil
		} else {
			if strings.HasSuffix(strings.ToLower(outputPath), ".webp") {
				outputPath = outputPath[:len(outputPath)-5] + ".png"
			}
			outFile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer outFile.Close()
			if err := png.Encode(outFile, outImg); err != nil {
				return err
			}
			
			// PNG 保存後に XMP メタデータを追加
// 			xmpAdded := false
// 			pngXMP, pngErr := extractTextualMetadataFromPNG(origData)
// 			webpXMP, webpErr := extractTextualMetadataFromWebP(origData)
// 			
// 			fmt.Fprintf(os.Stderr, "DEBUG: PNG XMP抽出 - エラー: %v, サイズ: %d\n", pngErr, len(pngXMP))
// 			fmt.Fprintf(os.Stderr, "DEBUG: WebP XMP抽出 - エラー: %v, サイズ: %d\n", webpErr, len(webpXMP))
// 			
// 			if pngErr == nil && pngXMP != "" {
// 				fmt.Fprintf(os.Stderr, "DEBUG: PNG XMPを追加します\n")
// 				if err := addXMPToPNG(outputPath, pngXMP); err != nil {
// 					fmt.Fprintf(os.Stderr, "DEBUG: PNG XMP追加エラー: %v\n", err)
// 					return err
// 				}
// 				xmpAdded = true
// 			}
			// WebP からの変換時は XMP を追加してみる
// 			if !xmpAdded && webpErr == nil && webpXMP != "" {
// 				fmt.Fprintf(os.Stderr, "DEBUG: WebP→PNG XMPを追加します\n")
// 				if err := addXMPToPNG(outputPath, webpXMP); err != nil {
// 					fmt.Fprintf(os.Stderr, "DEBUG: WebP→PNG XMP追加エラー: %v\n", err)
// 					return err
// 				}
// 				xmpAdded = true
// 			}
// 			
			// メタデータが追加されたかチェック
// 			if !xmpAdded {
// 				fmt.Fprintf(os.Stderr, "警告: プリントカメラ解像度PNG (%s) にメタデータがありません\n", imagePath)
// 			} else {
// 				fmt.Fprintf(os.Stderr, "DEBUG: PNG XMP追加完了\n")
// 			}
// 			
			// メタデータ検証は暫定的に無効化（保存確認待ち）
// 			return nil
			return nil
		}
	}

	// 通常処理（余白・テキスト・QR）
	marginTop := 69
	newWidth := width
	newHeight := height + marginTop
	var bgColor color.Color
	// 2048x1440解像度の場合は白背景に固定
	if isPrintCameraResolutionOnly(img) {
		bgColor = color.White
	} else if isDarkImage(img) {
		bgColor = color.Black
	} else {
		bgColor = color.White
	}
	newImg := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.Draw(newImg, newImg.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)
	draw.Draw(newImg, image.Rect(0, marginTop, width, marginTop+height), img, bounds.Min, draw.Over)
	
	if worldName == "" {
		if date == "" {
			date = extractDateFromFilename(imagePath)
		}
		worldURL = ""
	}
	
	// テキストとメタデータを描画
	isDark := isDarkImage(img)
	textColor := color.White
	if !isDark {
		textColor = color.Black
	}
	addTextToImage(newImg, date, worldName, authorName, authorID, worldURL, marginTop, newWidth, newHeight, textColor, bgColor, isDark, worldURL != "")
	
	outputDir := getOutputDir(imagePath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	outputPath := filepath.Join(outputDir, filepath.Base(imagePath))

	// 出力フォーマットを決定
	outputFormat := determineOutputFormat(imagePath, appConfig.Image.OutputFormat)
	isWebP := outputFormat == "webp"

	// 出力フォーマットに応じて拡張子を調整
	outputPath = adjustOutputPath(outputPath, outputFormat)

	if isWebP {
		var buf bytes.Buffer
		quality := float32(appConfig.Image.WebPCompressionQuality)
		if quality <= 0 || quality > 100 {
			quality = 100
		}
		err = webp.Encode(&buf, newImg, &webp.Options{Lossless: appConfig.Image.WebPLossless, Quality: quality})
		if err != nil {
			return err
		}
		
		outFile, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer outFile.Close()
		_, err = outFile.Write(buf.Bytes())
		if err != nil {
			return err
		}
		
		// WebP 保存後に XMP メタデータを追加
		if xmp, err := extractTextualMetadataFromWebP(origData); err == nil && xmp != "" {
			if err := addXMPToWebP(outputPath, xmp); err != nil {
				return err
			}
		}
		// PNG からの変換時は XMP を追加してみる
		if xmp2, err := extractTextualMetadataFromPNG(origData); err == nil && xmp2 != "" {
			if err := addXMPToWebP(outputPath, xmp2); err != nil {
				return err
			}
		}
		
		// メタデータ検証は暫定的に無効化（保存確認待ち）
		return nil
	} else {
		if strings.HasSuffix(strings.ToLower(outputPath), ".webp") {
			outputPath = outputPath[:len(outputPath)-5] + ".png"
		}
		outFile, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer outFile.Close()
		if err := png.Encode(outFile, newImg); err != nil {
			return err
		}
		
		// PNG 保存後に XMP メタデータを追加
		if xmp, err := extractTextualMetadataFromPNG(origData); err == nil && xmp != "" {
			fmt.Fprintf(os.Stderr, "  [Metadata] PNG XMP extracted (%d bytes)...\n", len(xmp))
			if err := addXMPToPNG(outputPath, xmp); err != nil {
				fmt.Fprintf(os.Stderr, "  [ERROR] PNG metadata write failed: %v\n", err)
				return err
			}
			fmt.Fprintf(os.Stderr, "  [SUCCESS] PNG metadata written\n")
		} else if xmp == "" {
			fmt.Fprintf(os.Stderr, "  [Metadata] PNG XMP not found\n")
		} else {
			fmt.Fprintf(os.Stderr, "  [Metadata] PNG XMP extraction error: %v\n", err)
		}
		// WebP からの変換時は XMP を追加してみる
		if xmp2, err := extractTextualMetadataFromWebP(origData); err == nil && xmp2 != "" {
			fmt.Fprintf(os.Stderr, "  [Metadata] WebP XMP extracted (%d bytes)...\n", len(xmp2))
			if err := addXMPToPNG(outputPath, xmp2); err != nil {
				fmt.Fprintf(os.Stderr, "  [ERROR] WebP->PNG metadata write failed: %v\n", err)
				return err
			}
			fmt.Fprintf(os.Stderr, "  [SUCCESS] WebP->PNG metadata written\n")
		}
		
		// メタデータ検証は暫定的に無効化（保存確認待ち）
		return nil
	}
}

func extractDateFromFilename(filePath string) string {
	filename := filepath.Base(filePath)
	
	// パターン1: VRChat_2026-01-15_22-52-38.319_3840x2160
	// パターン2: VRChat_2026-01-14_21-49-03.450_2048x1440
	// パターン3: VRChat_1920x1080_2022-06-02_03-11-38.751
	re1 := regexp.MustCompile(`VRChat_(?:\d+x\d+_)?(\d{4}-\d{2}-\d{2})_(\d{2}-\d{2}-\d{2})`)
	if matches := re1.FindStringSubmatch(filename); len(matches) > 2 {
		return matches[1] + "T" + strings.ReplaceAll(matches[2], "-", ":")
	}

	// パターン: com.vrchat.oculus.quest-20220330-003003
	re2 := regexp.MustCompile(`-(\d{8})-(\d{6})`)
	if matches := re2.FindStringSubmatch(filename); len(matches) > 2 {
		dateStr := matches[1]
		timeStr := matches[2]
		return dateStr[0:4] + "-" + dateStr[4:6] + "-" + dateStr[6:8] + "T" + 
			timeStr[0:2] + ":" + timeStr[2:4] + ":" + timeStr[4:6]
	}
	
	return ""
}

func formatDateForDisplay(dateStr string) string {
	// コンフィグで指定されたレイアウト（Go のレイアウト文字列）で日付を整形する。
	// 例: "2006-01-02 Mon 15:04:05"
	layout := strings.TrimSpace(appConfig.DateFormat)
	useUpperWeekday := false
	if layout == "" {
		layout = "2006-01-02 Mon 15:04:05"
		useUpperWeekday = true // 既存デフォルトに近い表記を維持
	}

	// よくある入力フォーマットを順に試す
	candidates := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.000",
		"2006-01-02",
	}
	for _, p := range candidates {
		if t, err := time.Parse(p, dateStr); err == nil {
			formatted := t.Format(layout)
			if useUpperWeekday {
				weekday := t.Format("Mon")
				formatted = strings.ReplaceAll(formatted, weekday, strings.ToUpper(weekday))
			}
			return formatted
		}
	}

	// パースできなければ元の文字列を返す
	return dateStr
}

// rMQRコード（長方形QRコード）を生成
// rMQRコード（横長型）を生成
func generateRMQR(url string, isDark bool) (image.Image, error) {
	// rmqr で Rectangular Micro QR コード生成
	qrImage, err := rmqr.Encode(
		[]byte(url),
		rmqr.WithLevel(rmqr.LevelM),
		rmqr.WithPriority(rmqr.PriorityHeight),
	)
	if err != nil {
		return nil, err
	}
	
	// 黒背景の場合は反転
	if isDark {
		return invertImage(qrImage), nil
	}
	
	return qrImage, nil
}

// 画像を反転する（黒と白を入れ替える）
func invertImage(img image.Image) image.Image {
	bounds := img.Bounds()
	inverted := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// 反転: 各値を 255 - 値 にする (16ビット値を8ビットに変換してから反転)
			inverted.SetRGBA(x, y, color.RGBA{
				R: 255 - uint8(r>>8),
				G: 255 - uint8(g>>8),
				B: 255 - uint8(b>>8),
				A: uint8(a >> 8),
			})
		}
	}
	return inverted
}

// loadSVGIcon はSVGアイコンを読み込んで、指定された色に置き換えて、画像として返す
// targetSize は最終的な出力サイズ（ピクセル）。指定がない場合は 20px。
func loadSVGIcon(iconName, colorHex string, targetSize int) (image.Image, error) {
	if targetSize <= 0 {
		targetSize = appConfig.Layout.IconSize
	}
	// ファイル名マッピング
	fileNameMap := map[string]string{
		"calendar": "calendar_today_24dp_434343.svg",
		"camera":   "photo_camera_24dp_434343.svg",
		"location": "location_pin_24dp_434343.svg",
		"person":   "person_24dp_434343.svg",
		"world":    "public_24dp_434343.svg",
	}

	svgFileName := fileNameMap[iconName]
	if svgFileName == "" {
		svgFileName = iconName + ".svg"
	}

	// 候補パスを順に探す
	candidates := []string{}

	// 1) コンフィグで指定されたパス
	if appConfig.IconPath != "" {
		candidates = append(candidates, filepath.Join(appConfig.IconPath, svgFileName))
	}

	// 2) 実行ファイルのディレクトリ
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(exeDir, "icon", svgFileName))
	}

	// 3) ソースファイルのディレクトリ（開発時に便利）
	if _, file, _, ok := runtime.Caller(0); ok {
		srcDir := filepath.Dir(file)
		candidates = append(candidates, filepath.Join(srcDir, "icon", svgFileName))
	}

	// 4) カレントワーキングディレクトリ（従来の動作）
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "icon", svgFileName))
	}

	// 5) 直接ファイル名（ユーザーが絶対パスを渡した場合など）
	candidates = append(candidates, svgFileName)

	var svgData []byte
	for _, p := range candidates {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		d, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			continue
		}
		svgData = d
		break
	}

	if svgData == nil {
		// 見つからない場合はフォールバックのカラースクエアを返す
		return createColoredSquare(targetSize, targetSize, colorHex), nil
	}

	// 色を置き換える（#434343 -> 指定色）
	svgContent := string(svgData)
	colorHexLower := strings.ToLower(colorHex)

	// fill属性内の色を置き換え（複数パターン対応）
	svgContent = strings.ReplaceAll(svgContent, "fill=\"#434343\"", "fill=\"#"+colorHexLower+"\"")
	svgContent = strings.ReplaceAll(svgContent, "#434343", "#"+colorHexLower)

	// SVGをパースする
	icon, err := oksvg.ReadIconStream(strings.NewReader(svgContent))
	if err != nil {
		return createColoredSquare(targetSize, targetSize, colorHex), nil
	}

	// 高解像度でレンダリングした後に targetSize へスケーリング
	renderSize := targetSize * 2
	iconImg := image.NewRGBA(image.Rect(0, 0, renderSize, renderSize))

	// SVGのターゲットを renderSize に設定
	icon.SetTarget(0, 0, float64(renderSize), float64(renderSize))

	// Scannerの設定
	scanner := rasterx.NewScannerGV(renderSize, renderSize, iconImg, image.Rect(0, 0, renderSize, renderSize))
	dasher := rasterx.NewDasher(renderSize, renderSize, scanner)

	// SVGを描画
	icon.Draw(dasher, 1.0)

	// renderSize から targetSize にリサイズ
	scaled := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))
	xdraw.ApproxBiLinear.Scale(scaled, scaled.Bounds(), iconImg, iconImg.Bounds(), draw.Src, nil)

	return scaled, nil
}

// colorHex に基づいて色付きの正方形を作成（フォールバック）
func createColoredSquare(width, height int, colorHex string) image.Image {
	// 16進数カラーをRGBに変換
	r, g, b := 0, 0, 0
	if len(colorHex) >= 6 {
		fmt.Sscanf(colorHex, "%02x%02x%02x", &r, &g, &b)
	}
	
	c := color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	return img
}

// addTextToImageはマージン部分にテキスト情報を[icon] [date] [icon] [author] [icon] [world] ... [QR]レイアウトで描画
// SVG＋freetype を使用して、余白内に横一行で配置
func addTextToImage(img *image.RGBA, date, worldName, authorName, authorID, worldURL string, marginTop, origWidth, origHeight int, textColor, bgColor color.Color, isDark, needsQR bool) error {
	if marginTop <= 0 {
		return nil
	}
	
	// テキスト色を RGB に変換
	r, g, b, _ := textColor.RGBA()
	colorHex := fmt.Sprintf("%02X%02X%02X", r>>8, g>>8, b>>8)
	
	// フォント読み込み（日時表示用 - モノスペース）
	monoFontData := loadFontFromPaths([]string{appConfig.Fonts.MonoFont})
	var monoFont *truetype.Font
	if monoFontData != nil {
		monoFont, _ = truetype.Parse(monoFontData)
	}
	
	// 標準フォント読み込み
	fontData := loadFontFromPaths([]string{appConfig.Fonts.MainFont})
	if fontData == nil {
		// フォントが見つからない場合はフォールバック
		fontData = loadFontFromPaths(appConfig.Fonts.FallbackFonts)
	}
	if fontData == nil {
		return nil
	}
	font, err := truetype.Parse(fontData)
	if err != nil {
		return nil
	}

	// レイアウト設定をコンフィグから取得
	marginHeight := marginTop
	marginLeft := appConfig.Layout.MarginLeft
	iconSize := appConfig.Layout.IconSize
	iconSpacing := appConfig.Layout.IconSpacing
	gapSize := appConfig.Layout.GapSize
	mainFontSize := appConfig.Layout.MainFontSize
	rightPadding := appConfig.Layout.MarginRight

	// フォントフェイス（測定用）
	mainFace := truetype.NewFace(font, &truetype.Options{Size: mainFontSize, DPI: 72})
	dateFace := mainFace
	if monoFont != nil {
		dateFace = truetype.NewFace(monoFont, &truetype.Options{Size: mainFontSize, DPI: 72})
	}

	// 垂直配置（中央揃え）
	metrics := mainFace.Metrics()
	asc := metrics.Ascent.Round()
	desc := metrics.Descent.Round()
	textHeight := asc + desc
	textBaseline := (marginHeight-textHeight)/2 + asc
	if textBaseline < asc {
		textBaseline = asc
	}
	iconY := (marginHeight - iconSize) / 2
	if iconY < 0 {
		iconY = 0
	}

	// QRコード領域を先に計算（スケーリング設定を使用）
	availableRight := origWidth - rightPadding
	var scaledQR *image.RGBA
	var qrX, qrY, scaledWidth, scaledHeight int
	if needsQR && worldURL != "" {
		qrImg, err := generateRMQR(worldURL, isDark)
		if err == nil {
			qrBounds := qrImg.Bounds()
			scaleFactor := appConfig.Image.QRScaleFactor
			scaledWidth = qrBounds.Dx() * scaleFactor
			scaledHeight = qrBounds.Dy() * scaleFactor
			qrX = origWidth - scaledWidth - appConfig.Image.QRRightPadding
			if qrX < marginLeft {
				qrX = marginLeft
			}
			qrY = (marginHeight - scaledHeight) / 2
			if qrY < 0 {
				qrY = 0
			}
			scaledQR = image.NewRGBA(image.Rect(0, 0, scaledWidth, scaledHeight))
			xdraw.NearestNeighbor.Scale(scaledQR, scaledQR.Bounds(), qrImg, qrBounds, draw.Src, nil)
			availableRight = qrX - 12
		}
	}
	if availableRight < marginLeft {
		availableRight = marginLeft
	}

	// freetype コンテキスト設定
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFontSize(mainFontSize)
	c.SetSrc(image.NewUniform(textColor))
	c.SetDst(img)
	c.SetClip(img.Bounds())

	measureWidth := func(face xfont.Face, s string) int {
		return xfont.MeasureString(face, s).Round()
	}
	fitText := func(face xfont.Face, s string, maxWidth int) string {
		if maxWidth <= 0 {
			return ""
		}
		if measureWidth(face, s) <= maxWidth {
			return s
		}
		ellipsis := "..."
		ellipsisW := measureWidth(face, ellipsis)
		if ellipsisW > maxWidth {
			return ""
		}
		runes := []rune(s)
		for i := len(runes); i > 0; i-- {
			candidate := string(runes[:i]) + ellipsis
			if measureWidth(face, candidate) <= maxWidth {
				return candidate
			}
		}
		return ""
	}

	formattedDate := formatDateForDisplay(date)
	currentX := marginLeft

	// アイコン1: カレンダー
	if calIcon, err := loadSVGIcon("calendar", colorHex, iconSize); err == nil {
		iconRect := image.Rect(currentX, iconY, currentX+iconSize, iconY+iconSize)
		draw.Draw(img, iconRect, calIcon, image.Point{}, draw.Over)
	}
	currentX += iconSize + iconSpacing

	// テキスト: 日時（等幅があれば優先）
	dateText := fitText(dateFace, formattedDate, availableRight-currentX)
	if dateText != "" {
		if monoFont != nil {
			c.SetFont(monoFont)
		} else {
			c.SetFont(font)
		}
		pt := freetype.Pt(currentX, textBaseline)
		c.DrawString(dateText, pt)
		currentX += measureWidth(dateFace, dateText) + gapSize
	}

	// ワールド情報がある場合のみアイコン＆テキスト描画
	if worldName != "" && currentX < availableRight {
		// 撮影者がコンフィグのプレースホルダー名の場合は撮影者セクションを省略
		skipAuthor := false
		if strings.TrimSpace(appConfig.PlaceholderAuthorName) != "" {
			skipAuthor = strings.TrimSpace(authorName) == strings.TrimSpace(appConfig.PlaceholderAuthorName)
		}
		if !skipAuthor {
			// アイコン2: カメラ（作成者）
			if cameraIcon, err := loadSVGIcon("camera", colorHex, iconSize); err == nil {
				iconRect := image.Rect(currentX, iconY, currentX+iconSize, iconY+iconSize)
				draw.Draw(img, iconRect, cameraIcon, image.Point{}, draw.Over)
			}
			currentX += iconSize + iconSpacing
			
			// テキスト: 作成者名（可変幅）
			authorText := fitText(mainFace, authorName, availableRight-currentX)
			if authorText != "" {
				c.SetFont(font)
				pt := freetype.Pt(currentX, textBaseline)
				c.DrawString(authorText, pt)
				currentX += measureWidth(mainFace, authorText) + gapSize
			}
		}
	}

	// ワールド名セクション
	if worldName != "" && currentX < availableRight {
		if locIcon, err := loadSVGIcon("location", colorHex, iconSize); err == nil {
			iconRect := image.Rect(currentX, iconY, currentX+iconSize, iconY+iconSize)
			draw.Draw(img, iconRect, locIcon, image.Point{}, draw.Over)
		}
		currentX += iconSize + iconSpacing

		worldText := fitText(mainFace, worldName, availableRight-currentX)
		if worldText != "" {
			c.SetFont(font)
			pt := freetype.Pt(currentX, textBaseline)
			c.DrawString(worldText, pt)
		}
	}

	// rMQRコード（右側に配置）
	if scaledQR != nil {
		draw.Draw(img, image.Rect(qrX, qrY, qrX+scaledWidth, qrY+scaledHeight), scaledQR, image.Point{}, draw.Over)
	}
	
	return nil
}

// WebP ファイルにメタデータチャンクを追加（堅牢な実装）
// VP8/VP8Lチャンク後に EXIF チャンクと XMP チャンクを挿入
func addMetadataChunksToWebP(webpData []byte, exifData, xmpData []byte) ([]byte, error) {
	if len(webpData) < 12 {
		return nil, errors.New("invalid WebP file: too small")
	}
	
	// RIFFヘッダ検証
	if string(webpData[0:4]) != "RIFF" || string(webpData[8:12]) != "WEBP" {
		return nil, errors.New("invalid WebP file: wrong header")
	}
	
	// ファイルサイズ（12バイト以降）
	fileSize := int(binary.LittleEndian.Uint32(webpData[4:8])) + 8
	if len(webpData) < fileSize {
		return nil, errors.New("invalid WebP file: truncated")
	}
	
	// チャンクを探す
	var result bytes.Buffer
	result.Write(webpData[0:12]) // RIFFヘッダ＋"WEBP"
	
	pos := 12
	metadataInserted := false
	
	for pos < len(webpData) {
		if pos+8 > len(webpData) {
			break
		}
		
		chunkID := string(webpData[pos : pos+4])
		chunkSize := int(binary.LittleEndian.Uint32(webpData[pos+4 : pos+8]))
		chunkDataStart := pos + 8
		chunkDataEnd := chunkDataStart + chunkSize
		
		// チャンク境界検証
		if chunkDataEnd > len(webpData) {
			return nil, errors.New("invalid WebP file: chunk size exceeds file boundary")
		}
		
		// VP8/VP8L/VP8X チャンクの後にメタデータを挿入
		if !metadataInserted && (chunkID == "VP8 " || chunkID == "VP8L" || chunkID == "VP8X") {
			// メインチャンクを追加
			result.Write(webpData[pos : chunkDataEnd])
			
			// パディング（奇数バイト）
			if chunkSize%2 == 1 {
				result.WriteByte(0)
				pos = chunkDataEnd + 1
			} else {
				pos = chunkDataEnd
			}
			
			// メタデータチャンクを追加
			if len(exifData) > 0 {
				if err := writeMetadataChunk(&result, "EXIF", exifData); err != nil {
					return nil, err
				}
			}
			
			if len(xmpData) > 0 {
				if err := writeMetadataChunk(&result, "XMP ", xmpData); err != nil {
					return nil, err
				}
			}
			
			metadataInserted = true
		} else if chunkID != "EXIF" && chunkID != "XMP " && chunkID != "ICCP" {
			// 既存のEXIF/XMP/ICCPチャンクはスキップ（重複を避ける）
			result.Write(webpData[pos : chunkDataEnd])
			
			// パディング
			if chunkSize%2 == 1 {
				result.WriteByte(0)
				pos = chunkDataEnd + 1
			} else {
				pos = chunkDataEnd
			}
		} else {
			// EXIFまたはXMPチャンクをスキップ
			if chunkSize%2 == 1 {
				pos = chunkDataEnd + 1
			} else {
				pos = chunkDataEnd
			}
		}
	}
	
	// ファイルサイズを更新
	resultBytes := result.Bytes()
	newSize := len(resultBytes) - 8
	binary.LittleEndian.PutUint32(resultBytes[4:8], uint32(newSize))
	
	return resultBytes, nil
}

// メタデータチャンクを書き込み（ヘルパー関数）
func writeMetadataChunk(buf *bytes.Buffer, chunkID string, data []byte) error {
	if len(chunkID) != 4 {
		return errors.New("invalid chunk ID length")
	}
	
	// チャンク ID
	buf.WriteString(chunkID)
	
	// チャンクサイズ（リトルエンディアン）
	size := uint32(len(data))
	binary.Write(buf, binary.LittleEndian, size)
	
	// チャンクデータ
	buf.Write(data)
	
	// パディング（奇数バイト）
	if len(data)%2 == 1 {
		buf.WriteByte(0)
	}
	
	return nil
}

// addXMPToPNG はデコード済みの PNG ファイルに XMP メタデータを追加します
// iTXt チャンク（UTF-8対応国際テキスト）を使用して日本語対応を実現します
func addXMPToPNG(pngPath string, xmpData string) error {
	if xmpData == "" {
		return nil
	}

	data, err := os.ReadFile(pngPath)
	if err != nil {
		return err
	}
	if len(data) < 12 {
		return errors.New("invalid PNG file")
	}

	// PNG signature and IHDR check
	if string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		return errors.New("invalid PNG signature")
	}

	// IEND chunk is always "IEND" + 0 length + CRC (12 bytes at the end)
	// We want to insert iTXt just before IEND
	
	// Find IEND chunk
	iendPos := len(data) - 12
	if iendPos < 8 {
		return errors.New("PNG too short for IEND")
	}
	
	// Verify IEND chunk
	if string(data[iendPos+4:iendPos+8]) != "IEND" {
		return errors.New("invalid IEND chunk")
	}

	// Create iTXt chunk
	// iTXt format: Keyword\0 + CompressionFlag(1) + CompressionMethod(1) + LanguageTag + \0 + TranslatedKeyword + \0 + Text
	keyword := "XML:com.adobe.xmp"
	var chunkBuf bytes.Buffer
	chunkBuf.Write([]byte(keyword))
	chunkBuf.WriteByte(0)            // Null separator after keyword
	chunkBuf.WriteByte(0)            // Compression flag: 0 = uncompressed
	chunkBuf.WriteByte(0)            // Compression method (not used if uncompressed)
	chunkBuf.WriteByte(0)            // Null (language tag is empty)
	chunkBuf.WriteByte(0)            // Null (translated keyword is empty)
	chunkBuf.Write([]byte(xmpData))  // XMP text data
	chunkData := chunkBuf.Bytes()

	// Build iTXt chunk: length(4) + "iTXt"(4) + data + CRC(4)
	var newChunk bytes.Buffer
	chunkLen := make([]byte, 4)
	binary.BigEndian.PutUint32(chunkLen, uint32(len(chunkData)))
	newChunk.Write(chunkLen)
	newChunk.Write([]byte("iTXt"))
	newChunk.Write(chunkData)
	
	// Calculate CRC
	crcData := append([]byte("iTXt"), chunkData...)
	crcVal := crc32.ChecksumIEEE(crcData)
	crcBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBytes, crcVal)
	newChunk.Write(crcBytes)

	// Assemble final PNG: original[0:iendPos] + iTXt chunk + IEND chunk
	var result bytes.Buffer
	result.Write(data[:iendPos])       // Everything before IEND
	result.Write(newChunk.Bytes())     // New iTXt chunk
	result.Write(data[iendPos:])       // Original IEND chunk

	return os.WriteFile(pngPath, result.Bytes(), 0644)
}

// addXMPToWebP はデコード済みの WebP ファイルに XMP メタデータを追加
func addXMPToWebP(webpPath string, xmpData string) error {
	if xmpData == "" {
		return nil
	}

	// WebP ファイルを読み込み
	data, err := os.ReadFile(webpPath)
	if err != nil {
		return err
	}

	// WebP シグネチャ確認
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return errors.New("invalid WebP file")
	}

	// WebP チャンク構造
	// RIFF ヘッダ (12 bytes)
	// WEBP チャンク: VP8 / VP8L / VP8X...
	// XMP チャンク: 'XMP ' サイズ データ (パディング)

	// 既存の XMP チャンクを削除（あれば）
	var newData bytes.Buffer
	newData.Write(data[0:12]) // RIFF ヘッダをコピー

	riffSize := int(binary.LittleEndian.Uint32(data[4:8]))
	offset := 12
	xmpAdded := false

	for offset+8 <= len(data) && offset-8 < riffSize {
		if offset+8 > len(data) {
			break
		}

		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + chunkSize

		// パディング対応
		paddedSize := chunkSize
		if chunkSize%2 == 1 {
			paddedSize++
		}
		nextOffset := offset + 8 + paddedSize

		if chunkDataEnd > len(data) {
			break
		}

		// XMP チャンクを削除して新しいものを追加
		if chunkID == "XMP " {
			if !xmpAdded {
				// 新しい XMP チャンクを追加
				xmpBytes := []byte(xmpData)
				newData.Write([]byte("XMP "))
				binary.Write(&newData, binary.LittleEndian, uint32(len(xmpBytes)))
				newData.Write(xmpBytes)
				if len(xmpBytes)%2 == 1 {
					newData.WriteByte(0)
				}
				xmpAdded = true
			}
		} else if chunkID == "EXIF" {
			newData.Write(data[offset : nextOffset])
		} else {
			// その他のチャンクはそのままコピー
			newData.Write(data[offset : nextOffset])
		}

		offset = nextOffset
	}

	// XMP を追加していなければ追加
	if !xmpAdded {
		xmpBytes := []byte(xmpData)
		newData.Write([]byte("XMP "))
		binary.Write(&newData, binary.LittleEndian, uint32(len(xmpBytes)))
		newData.Write(xmpBytes)
		if len(xmpBytes)%2 == 1 {
			newData.WriteByte(0)
		}
	}

	// RIFF サイズを更新
	finalData := newData.Bytes()
	newRiffSize := len(finalData) - 8
	binary.LittleEndian.PutUint32(finalData[4:8], uint32(newRiffSize))

	// ファイルに書き込み
	return os.WriteFile(webpPath, finalData, 0644)
}


