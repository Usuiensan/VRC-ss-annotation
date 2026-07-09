package main

import (
	"bufio"
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
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

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
var stateMutex sync.Mutex
var configMutex sync.Mutex
var vrchatContext *VRChatLogTracker

// グローバルコンフィグ構造体
var appConfig *Config

// Config はアプリケーション全体の設定を保持する構造体
type Config struct {
	PlaceholderAuthorName string             `json:"placeholderAuthorName"`
	OutputDir             string             `json:"outputDir"`
	DateFormat            string             `json:"dateFormat"`
	Fonts                 FontConfig         `json:"fonts"`
	IconPath              string             `json:"iconPath"`
	Layout                LayoutConfig       `json:"layout"`
	Colors                ColorConfig        `json:"colors"`
	Image                 ImageConfig        `json:"image"`
	Watcher               WatcherConfig      `json:"watcher"`
	Eagle                 EagleConfig        `json:"eagle"`
	State                 StateConfig        `json:"state"`
	Notifications         NotificationConfig `json:"notifications"`
}

type FontConfig struct {
	MonoFont      string   `json:"monoFont"`
	MainFont      string   `json:"mainFont"`
	FallbackFonts []string `json:"fallbackFonts"`
}

type LayoutConfig struct {
	MarginTop    int     `json:"marginTop"`
	MarginLeft   int     `json:"marginLeft"`
	MarginRight  int     `json:"marginRight"`
	IconSize     int     `json:"iconSize"`
	IconSpacing  int     `json:"iconSpacing"`
	GapSize      int     `json:"gapSize"`
	MainFontSize float64 `json:"mainFontSize"`
}

type ColorConfig struct {
	TextColorLight       string `json:"textColorLight"`
	TextColorDark        string `json:"textColorDark"`
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

type WatcherConfig struct {
	VRChatPhotoRoot            string `json:"vrchatPhotoRoot"`
	AmazonPhotosOutputDir      string `json:"amazonPhotosOutputDir"`
	VRChatLogDir               string `json:"vrchatLogDir"`
	VisitLogDir                string `json:"visitLogDir"`
	FileStabilityWaitSeconds   int    `json:"fileStabilityWaitSeconds"`
	StableCheckIntervalSeconds int    `json:"stableCheckIntervalSeconds"`
	StableCheckCount           int    `json:"stableCheckCount"`
	ScanIntervalSeconds        int    `json:"scanIntervalSeconds"`
	LogPollIntervalSeconds     int    `json:"logPollIntervalSeconds"`
}

type EagleConfig struct {
	Enabled  *bool    `json:"enabled,omitempty"`
	BaseURL  string   `json:"baseUrl"`
	FolderID string   `json:"folderId"`
	Folders  []string `json:"folders"`
}

type StateConfig struct {
	Path string `json:"path"`
}

type NotificationConfig struct {
	ToastEnabled *bool `json:"toastEnabled,omitempty"`
}

// デフォルト設定を返す
func getDefaultConfig() *Config {
	defaultEagleEnabled := true
	defaultToastEnabled := true
	return &Config{
		PlaceholderAuthorName: "",
		OutputDir:             "",
		DateFormat:            "2006-01-02 Mon 15:04:05",
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
		Watcher: WatcherConfig{
			VRChatPhotoRoot:            "",
			AmazonPhotosOutputDir:      "",
			VRChatLogDir:               "",
			VisitLogDir:                "visit-logs",
			FileStabilityWaitSeconds:   5,
			StableCheckIntervalSeconds: 1,
			StableCheckCount:           3,
			ScanIntervalSeconds:        3,
			LogPollIntervalSeconds:     2,
		},
		Eagle: EagleConfig{
			Enabled: &defaultEagleEnabled,
			BaseURL: "http://localhost:41595",
		},
		State: StateConfig{
			Path: "watch-state.jsonl",
		},
		Notifications: NotificationConfig{
			ToastEnabled: &defaultToastEnabled,
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
	if override.Watcher.VRChatPhotoRoot != "" {
		def.Watcher.VRChatPhotoRoot = override.Watcher.VRChatPhotoRoot
	}
	if override.Watcher.AmazonPhotosOutputDir != "" {
		def.Watcher.AmazonPhotosOutputDir = override.Watcher.AmazonPhotosOutputDir
	}
	if override.Watcher.VRChatLogDir != "" {
		def.Watcher.VRChatLogDir = override.Watcher.VRChatLogDir
	}
	if override.Watcher.VisitLogDir != "" {
		def.Watcher.VisitLogDir = override.Watcher.VisitLogDir
	}
	if override.Watcher.FileStabilityWaitSeconds > 0 {
		def.Watcher.FileStabilityWaitSeconds = override.Watcher.FileStabilityWaitSeconds
	}
	if override.Watcher.StableCheckIntervalSeconds > 0 {
		def.Watcher.StableCheckIntervalSeconds = override.Watcher.StableCheckIntervalSeconds
	}
	if override.Watcher.StableCheckCount > 0 {
		def.Watcher.StableCheckCount = override.Watcher.StableCheckCount
	}
	if override.Watcher.ScanIntervalSeconds > 0 {
		def.Watcher.ScanIntervalSeconds = override.Watcher.ScanIntervalSeconds
	}
	if override.Watcher.LogPollIntervalSeconds > 0 {
		def.Watcher.LogPollIntervalSeconds = override.Watcher.LogPollIntervalSeconds
	}
	if override.Eagle.Enabled != nil {
		def.Eagle.Enabled = override.Eagle.Enabled
	}
	if override.Eagle.BaseURL != "" {
		def.Eagle.BaseURL = override.Eagle.BaseURL
	}
	if override.Eagle.FolderID != "" {
		def.Eagle.FolderID = override.Eagle.FolderID
	}
	if len(override.Eagle.Folders) > 0 {
		def.Eagle.Folders = override.Eagle.Folders
	}
	if override.State.Path != "" {
		def.State.Path = override.State.Path
	}
	if override.Notifications.ToastEnabled != nil {
		def.Notifications.ToastEnabled = override.Notifications.ToastEnabled
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

type SourceType string

const (
	SourceTypePhoto   SourceType = "photo"
	SourceTypePrint   SourceType = "print"
	SourceTypeSticker SourceType = "sticker"
	SourceTypeStamp   SourceType = "stamp"
	SourceTypeEmoji   SourceType = "emoji"
	SourceTypeUnknown SourceType = "unknown"
)

type PhotoRecord struct {
	SourcePath         string     `json:"source_path"`
	SourceType         SourceType `json:"source_type"`
	WorldID            string     `json:"world_id,omitempty"`
	WorldName          string     `json:"world_name,omitempty"`
	InstanceID         string     `json:"instance_id,omitempty"`
	InstanceType       string     `json:"instance_type,omitempty"`
	ShootDate          string     `json:"shoot_date,omitempty"`
	AuthorName         string     `json:"author_name,omitempty"`
	PresentUsers       []string   `json:"present_users,omitempty"`
	OutputPath         string     `json:"output_path,omitempty"`
	WorldFilledFromLog bool       `json:"world_filled_from_log,omitempty"`
}

type ProcessStateEntry struct {
	Timestamp          string     `json:"timestamp"`
	SourcePath         string     `json:"source_path"`
	SourceType         SourceType `json:"source_type"`
	OutputPath         string     `json:"output_path,omitempty"`
	EagleStatus        string     `json:"eagle_status"`
	AmazonStatus       string     `json:"amazon_status"`
	WorldID            string     `json:"world_id,omitempty"`
	WorldName          string     `json:"world_name,omitempty"`
	InstanceID         string     `json:"instance_id,omitempty"`
	InstanceType       string     `json:"instance_type,omitempty"`
	PresentUsers       []string   `json:"present_users,omitempty"`
	WorldFilledFromLog bool       `json:"world_filled_from_log,omitempty"`
	Error              string     `json:"error,omitempty"`
	Size               int64      `json:"size"`
	ModTimeUnix        int64      `json:"mod_time_unix"`
}

type eagleAddRequest struct {
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	Website    string   `json:"website,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Folders    []string `json:"folders,omitempty"`
	Annotation string   `json:"annotation,omitempty"`
}

func runSubcommand(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "watch":
		fs := flag.NewFlagSet("watch", flag.ExitOnError)
		root := fs.String("root", appConfig.Watcher.VRChatPhotoRoot, "VRChat写真フォルダ")
		amazonDir := fs.String("amazon-output-dir", appConfig.Watcher.AmazonPhotosOutputDir, "Amazon Photos用出力ディレクトリ")
		_ = fs.Parse(args[1:])
		if *root != "" {
			appConfig.Watcher.VRChatPhotoRoot = *root
		}
		if *amazonDir != "" {
			appConfig.Watcher.AmazonPhotosOutputDir = *amazonDir
		}
		return true, watchPhotoRoot()
	case "process-file":
		fs := flag.NewFlagSet("process-file", flag.ExitOnError)
		amazonDir := fs.String("amazon-output-dir", appConfig.Watcher.AmazonPhotosOutputDir, "Amazon Photos用出力ディレクトリ")
		_ = fs.Parse(args[1:])
		if *amazonDir != "" {
			appConfig.Watcher.AmazonPhotosOutputDir = *amazonDir
		}
		if fs.NArg() != 1 {
			return true, errors.New("process-file には画像パスを1つだけ指定してください")
		}
		entry := processWatchedFile(fs.Arg(0), true)
		if entry.Error != "" {
			return true, errors.New(entry.Error)
		}
		return true, nil
	case "test-eagle":
		return true, testEagleConnection()
	case "print-config":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return true, enc.Encode(appConfig)
	case "retry-failed":
		return true, retryFailed()
	default:
		return false, nil
	}
}

func eagleEnabled() bool {
	return appConfig.Eagle.Enabled == nil || *appConfig.Eagle.Enabled
}

func classifySourceType(path string) SourceType {
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	lowerBase := strings.ToLower(filepath.Base(path))
	if strings.Contains(lowerPath, "/stickers/") {
		return SourceTypeSticker
	}
	if strings.Contains(lowerPath, "/print/") {
		return SourceTypePrint
	}
	if strings.Contains(lowerPath, "/stamp/") {
		return SourceTypeStamp
	}
	if strings.Contains(lowerPath, "emoji") || strings.Contains(lowerPath, "emojis") ||
		strings.Contains(lowerPath, "emote") || strings.Contains(lowerPath, "emoticon") {
		return SourceTypeEmoji
	}
	re := regexp.MustCompile(`(?i)^VRChat_\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}\.\d+_.*`)
	if re.MatchString(lowerBase) {
		return SourceTypePhoto
	}
	return SourceTypeUnknown
}

func buildPhotoRecord(path string, sourceType SourceType) PhotoRecord {
	meta, err := readVRChatExifPNG(path, true, true)
	record := PhotoRecord{SourcePath: path, SourceType: sourceType}
	if err == nil {
		record.WorldID, _ = meta["worldID"].(string)
		record.WorldName, _ = meta["worldName"].(string)
		record.ShootDate, _ = meta["shootDate"].(string)
		record.AuthorName, _ = meta["authorName"].(string)
	}
	if record.ShootDate == "" {
		record.ShootDate = extractDateFromFilename(path)
	}
	if vrchatContext != nil {
		snap := vrchatContext.SnapshotAt(photoTimeForRecord(record, path))
		if record.WorldID == "" && snap.WorldID != "" {
			record.WorldID = snap.WorldID
			record.WorldFilledFromLog = true
		}
		if record.WorldName == "" && snap.WorldName != "" {
			record.WorldName = snap.WorldName
			record.WorldFilledFromLog = true
		}
		if record.InstanceID == "" {
			record.InstanceID = snap.InstanceID
		}
		if record.InstanceType == "" {
			record.InstanceType = snap.InstanceType
		}
		if len(record.PresentUsers) == 0 {
			record.PresentUsers = snap.PresentUsers
		}
	}
	return record
}

func photoTimeForRecord(record PhotoRecord, path string) time.Time {
	if t, ok := parsePhotoTime(record.ShootDate); ok {
		return t
	}
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Now()
}

func parsePhotoTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.0000000-07:00",
		"2006-01-02T15:04:05.0000000",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

type VRChatContextSnapshot struct {
	At           time.Time `json:"at"`
	WorldID      string    `json:"world_id,omitempty"`
	WorldName    string    `json:"world_name,omitempty"`
	InstanceID   string    `json:"instance_id,omitempty"`
	InstanceType string    `json:"instance_type,omitempty"`
	PresentUsers []string  `json:"present_users,omitempty"`
}

type VRChatVisitEvent struct {
	Timestamp       string   `json:"timestamp"`
	Event           string   `json:"event"`
	LogPath         string   `json:"log_path,omitempty"`
	WorldID         string   `json:"world_id,omitempty"`
	WorldName       string   `json:"world_name,omitempty"`
	InstanceID      string   `json:"instance_id,omitempty"`
	InstanceType    string   `json:"instance_type,omitempty"`
	UserName        string   `json:"user_name,omitempty"`
	PresentUsers    []string `json:"present_users,omitempty"`
	VisitStartedAt  string   `json:"visit_started_at,omitempty"`
	VisitEndedAt    string   `json:"visit_ended_at,omitempty"`
	DurationSeconds int64    `json:"duration_seconds,omitempty"`
	Continues       bool     `json:"continues,omitempty"`
	Note            string   `json:"note,omitempty"`
}

type VRChatLogTracker struct {
	mu           sync.RWMutex
	logDir       string
	visitLogDir  string
	pollInterval time.Duration
	currentLog   string
	offset       int64
	day          string

	worldID        string
	worldName      string
	instanceID     string
	instanceType   string
	presentUsers   map[string]bool
	history        []VRChatContextSnapshot
	visitStartedAt time.Time
}

func startVRChatLogTracker() *VRChatLogTracker {
	logDir := strings.TrimSpace(appConfig.Watcher.VRChatLogDir)
	if logDir == "" {
		logDir = defaultVRChatLogDir()
	}
	if logDir == "" {
		appendLog("VRChatログディレクトリを特定できませんでした")
		return nil
	}
	if info, err := os.Stat(logDir); err != nil || !info.IsDir() {
		appendLog(fmt.Sprintf("VRChatログディレクトリが見つかりません: %s", logDir))
		return nil
	}
	visitLogDir := strings.TrimSpace(appConfig.Watcher.VisitLogDir)
	if visitLogDir == "" {
		visitLogDir = "visit-logs"
	}
	interval := appConfig.Watcher.LogPollIntervalSeconds
	if interval <= 0 {
		interval = 2
	}
	tracker := &VRChatLogTracker{
		logDir:       logDir,
		visitLogDir:  visitLogDir,
		pollInterval: time.Duration(interval) * time.Second,
		day:          time.Now().Format("2006-01-02"),
		presentUsers: make(map[string]bool),
	}
	if err := os.MkdirAll(visitLogDir, 0755); err != nil {
		appendLog(fmt.Sprintf("訪問ログディレクトリを作成できません: %v", err))
		return nil
	}
	go tracker.Run()
	return tracker
}

func defaultVRChatLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "AppData", "LocalLow", "VRChat", "VRChat")
}

func (t *VRChatLogTracker) Run() {
	t.poll()
	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()
	for range ticker.C {
		t.poll()
	}
}

func (t *VRChatLogTracker) poll() {
	t.writeDayRolloverIfNeeded(time.Now())
	latest, err := latestVRChatLogFile(t.logDir)
	if err != nil || latest == "" {
		return
	}
	t.mu.RLock()
	currentLog := t.currentLog
	t.mu.RUnlock()
	if latest != currentLog {
		now := time.Now()
		t.mu.Lock()
		prevSnap := t.snapshotLocked(now)
		prevStartedAt := t.visitStartedAt
		prevLog := t.currentLog
		t.currentLog = latest
		t.offset = 0
		t.worldID = ""
		t.worldName = ""
		t.instanceID = ""
		t.instanceType = ""
		t.presentUsers = make(map[string]bool)
		t.visitStartedAt = time.Time{}
		t.history = append(t.history, t.snapshotLocked(now))
		t.mu.Unlock()
		if prevSnap.WorldID != "" || prevSnap.InstanceID != "" {
			t.appendVisitEnd(now, prevStartedAt, prevSnap, prevLog, "log_file_changed")
		}
		t.appendEvent(now, "log_file_changed", "", "新しいVRChatログを追跡します")
	}
	t.readNewLines(latest)
}

func latestVRChatLogFile(logDir string) (string, error) {
	var latestPath string
	var latestTime time.Time
	err := filepath.WalkDir(logDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if !strings.HasPrefix(name, "output_log_") || !strings.HasSuffix(name, ".txt") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if latestPath == "" || info.ModTime().After(latestTime) {
			latestPath = path
			latestTime = info.ModTime()
		}
		return nil
	})
	return latestPath, err
}

func (t *VRChatLogTracker) readNewLines(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	t.mu.RLock()
	offset := t.offset
	t.mu.RUnlock()
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return
		}
	}
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		t.handleLogLine(path, scanner.Text())
	}
	if pos, err := f.Seek(0, io.SeekCurrent); err == nil {
		t.mu.Lock()
		t.offset = pos
		t.mu.Unlock()
	}
}

func (t *VRChatLogTracker) handleLogLine(logPath, line string) {
	ts, ok := parseVRChatLogTime(line)
	if !ok {
		ts = time.Now()
	}
	t.writeDayRolloverIfNeeded(ts)
	if worldID, instanceID, matched := parseJoiningWorld(line); matched {
		t.mu.Lock()
		prevSnap := t.snapshotLocked(ts)
		prevStartedAt := t.visitStartedAt
		t.worldID = worldID
		t.instanceID = instanceID
		t.instanceType = classifyInstanceType(instanceID)
		t.worldName = ""
		t.presentUsers = make(map[string]bool)
		t.visitStartedAt = ts
		snap := t.snapshotLocked(ts)
		t.history = append(t.history, snap)
		t.mu.Unlock()
		if prevSnap.WorldID != "" || prevSnap.InstanceID != "" {
			t.appendVisitEnd(ts, prevStartedAt, prevSnap, logPath, "world_leave")
		}
		t.appendEventWithSnapshot(ts, "world_join", "", logPath, snap)
		return
	}
	if worldName, matched := parseEnteringRoom(line); matched {
		t.mu.Lock()
		t.worldName = worldName
		snap := t.snapshotLocked(ts)
		t.history = append(t.history, snap)
		t.mu.Unlock()
		t.appendEventWithSnapshot(ts, "world_name", "", logPath, snap)
		return
	}
	if userName, matched := parsePlayerJoined(line); matched {
		t.mu.Lock()
		t.presentUsers[userName] = true
		snap := t.snapshotLocked(ts)
		t.history = append(t.history, snap)
		t.mu.Unlock()
		t.appendEventWithSnapshot(ts, "player_join", userName, logPath, snap)
		return
	}
	if userName, matched := parsePlayerLeft(line); matched {
		t.mu.Lock()
		delete(t.presentUsers, userName)
		snap := t.snapshotLocked(ts)
		t.history = append(t.history, snap)
		t.mu.Unlock()
		t.appendEventWithSnapshot(ts, "player_left", userName, logPath, snap)
		return
	}
}

func parseVRChatLogTime(line string) (time.Time, bool) {
	re := regexp.MustCompile(`^(\d{4}\.\d{2}\.\d{2} \d{2}:\d{2}:\d{2})`)
	m := re.FindStringSubmatch(line)
	if len(m) != 2 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006.01.02 15:04:05", m[1], time.Local)
	return t, err == nil
}

func parseJoiningWorld(line string) (string, string, bool) {
	re := regexp.MustCompile(`(?i)\bJoining\s+(wrld_[A-Za-z0-9-]+):([^\s]+)`)
	m := re.FindStringSubmatch(line)
	if len(m) != 3 {
		return "", "", false
	}
	return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), true
}

func parseEnteringRoom(line string) (string, bool) {
	patterns := []string{
		`(?i)Entering Room:\s*(.+)$`,
		`(?i)Joining or Creating Room:\s*(.+)$`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		m := re.FindStringSubmatch(line)
		if len(m) == 2 {
			name := strings.TrimSpace(m[1])
			if name != "" {
				return name, true
			}
		}
	}
	return "", false
}

func parsePlayerJoined(line string) (string, bool) {
	re := regexp.MustCompile(`(?i)OnPlayerJoined[:\s]+(.+)$`)
	m := re.FindStringSubmatch(line)
	if len(m) != 2 {
		return "", false
	}
	return cleanVRChatUserName(m[1]), cleanVRChatUserName(m[1]) != ""
}

func parsePlayerLeft(line string) (string, bool) {
	re := regexp.MustCompile(`(?i)OnPlayerLeft[:\s]+(.+)$`)
	m := re.FindStringSubmatch(line)
	if len(m) != 2 {
		return "", false
	}
	return cleanVRChatUserName(m[1]), cleanVRChatUserName(m[1]) != ""
}

func cleanVRChatUserName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	if idx := strings.Index(value, " ("); idx > 0 {
		value = value[:idx]
	}
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}

func classifyInstanceType(instanceID string) string {
	lower := strings.ToLower(instanceID)
	switch {
	case strings.Contains(lower, "~private"):
		return "private"
	case strings.Contains(lower, "~friends"):
		return "friends"
	case strings.Contains(lower, "~hidden"):
		return "hidden"
	case strings.Contains(lower, "~group"):
		return "group"
	case instanceID != "":
		return "public"
	default:
		return ""
	}
}

func (t *VRChatLogTracker) SnapshotAt(at time.Time) VRChatContextSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.history) == 0 {
		return t.snapshotLocked(at)
	}
	if at.IsZero() {
		return t.history[len(t.history)-1]
	}
	if at.Before(t.history[0].At) {
		return VRChatContextSnapshot{At: at}
	}
	selected := t.history[0]
	for _, snap := range t.history {
		if snap.At.After(at) {
			break
		}
		selected = snap
	}
	return selected
}

func (t *VRChatLogTracker) snapshotLocked(at time.Time) VRChatContextSnapshot {
	users := make([]string, 0, len(t.presentUsers))
	for user := range t.presentUsers {
		users = append(users, user)
	}
	sort.Strings(users)
	return VRChatContextSnapshot{
		At:           at,
		WorldID:      t.worldID,
		WorldName:    t.worldName,
		InstanceID:   t.instanceID,
		InstanceType: t.instanceType,
		PresentUsers: users,
	}
}

func (t *VRChatLogTracker) appendEventWithSnapshot(at time.Time, event, userName, logPath string, snap VRChatContextSnapshot) {
	t.mu.RLock()
	visitStartedAt := t.visitStartedAt
	t.mu.RUnlock()
	entry := VRChatVisitEvent{
		Timestamp:      at.Format(time.RFC3339),
		Event:          event,
		LogPath:        logPath,
		WorldID:        snap.WorldID,
		WorldName:      snap.WorldName,
		InstanceID:     snap.InstanceID,
		InstanceType:   snap.InstanceType,
		UserName:       userName,
		PresentUsers:   snap.PresentUsers,
		VisitStartedAt: formatOptionalTime(visitStartedAt),
	}
	t.writeVisitEvent(at, entry)
}

func (t *VRChatLogTracker) appendEvent(at time.Time, event, userName, note string) {
	snap := t.SnapshotAt(at)
	t.mu.RLock()
	logPath := t.currentLog
	visitStartedAt := t.visitStartedAt
	t.mu.RUnlock()
	entry := VRChatVisitEvent{
		Timestamp:      at.Format(time.RFC3339),
		Event:          event,
		LogPath:        logPath,
		WorldID:        snap.WorldID,
		WorldName:      snap.WorldName,
		InstanceID:     snap.InstanceID,
		InstanceType:   snap.InstanceType,
		UserName:       userName,
		PresentUsers:   snap.PresentUsers,
		VisitStartedAt: formatOptionalTime(visitStartedAt),
		Note:           note,
	}
	t.writeVisitEvent(at, entry)
}

func (t *VRChatLogTracker) appendVisitEnd(at, startedAt time.Time, snap VRChatContextSnapshot, logPath, event string) {
	entry := VRChatVisitEvent{
		Timestamp:       at.Format(time.RFC3339),
		Event:           event,
		LogPath:         logPath,
		WorldID:         snap.WorldID,
		WorldName:       snap.WorldName,
		InstanceID:      snap.InstanceID,
		InstanceType:    snap.InstanceType,
		PresentUsers:    snap.PresentUsers,
		VisitStartedAt:  formatOptionalTime(startedAt),
		VisitEndedAt:    at.Format(time.RFC3339),
		DurationSeconds: durationSeconds(startedAt, at),
	}
	t.writeVisitEvent(at, entry)
}

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func durationSeconds(start, end time.Time) int64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0
	}
	return int64(end.Sub(start).Seconds())
}

func (t *VRChatLogTracker) writeDayRolloverIfNeeded(at time.Time) {
	day := at.Format("2006-01-02")
	t.mu.Lock()
	oldDay := t.day
	if oldDay == "" {
		t.day = day
		t.mu.Unlock()
		return
	}
	if oldDay == day {
		t.mu.Unlock()
		return
	}
	snap := t.snapshotLocked(at)
	visitStartedAt := t.visitStartedAt
	t.day = day
	t.history = append(t.history, snap)
	t.mu.Unlock()
	if snap.WorldID == "" && snap.InstanceID == "" {
		return
	}
	oldEvent := VRChatVisitEvent{
		Timestamp:       at.Format(time.RFC3339),
		Event:           "day_rollover_end",
		WorldID:         snap.WorldID,
		WorldName:       snap.WorldName,
		InstanceID:      snap.InstanceID,
		InstanceType:    snap.InstanceType,
		PresentUsers:    snap.PresentUsers,
		VisitStartedAt:  formatOptionalTime(visitStartedAt),
		VisitEndedAt:    at.Format(time.RFC3339),
		DurationSeconds: durationSeconds(visitStartedAt, at),
		Continues:       true,
		Note:            "日付変更時点でVRChat滞在中の可能性があります",
	}
	newEvent := oldEvent
	newEvent.Event = "day_rollover_start"
	newEvent.VisitEndedAt = ""
	t.writeVisitEventForDay(oldDay, oldEvent)
	t.writeVisitEventForDay(day, newEvent)
}

func (t *VRChatLogTracker) writeVisitEvent(at time.Time, entry VRChatVisitEvent) {
	t.writeVisitEventForDay(at.Format("2006-01-02"), entry)
}

func (t *VRChatLogTracker) writeVisitEventForDay(day string, entry VRChatVisitEvent) {
	if day == "" {
		day = time.Now().Format("2006-01-02")
	}
	if err := os.MkdirAll(t.visitLogDir, 0755); err != nil {
		appendLog(fmt.Sprintf("訪問ログディレクトリを作成できません: %v", err))
		return
	}
	path := filepath.Join(t.visitLogDir, "vrchat-visits-"+day+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		appendLog(fmt.Sprintf("訪問ログを書き込めません: %v", err))
		return
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(entry); err != nil {
		appendLog(fmt.Sprintf("訪問ログをエンコードできません: %v", err))
	}
}

func waitForStableFile(path string) error {
	if appConfig.Watcher.FileStabilityWaitSeconds > 0 {
		time.Sleep(time.Duration(appConfig.Watcher.FileStabilityWaitSeconds) * time.Second)
	}
	interval := appConfig.Watcher.StableCheckIntervalSeconds
	if interval <= 0 {
		interval = 1
	}
	needed := appConfig.Watcher.StableCheckCount
	if needed <= 0 {
		needed = 3
	}
	var lastSize int64 = -1
	var lastMod time.Time
	stableCount := 0
	for stableCount < needed {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		_ = f.Close()
		if info.Size() == lastSize && info.ModTime().Equal(lastMod) {
			stableCount++
		} else {
			stableCount = 1
			lastSize = info.Size()
			lastMod = info.ModTime()
		}
		if stableCount < needed {
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
	return nil
}

func watchPhotoRoot() error {
	if vrchatContext == nil {
		vrchatContext = startVRChatLogTracker()
	}
	root := appConfig.Watcher.VRChatPhotoRoot
	if root == "" {
		return errors.New("watcher.vrchatPhotoRoot が必要です。watch -root <path> を渡すか annotate.config.json に設定してください")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	initial, err := scanImageFiles(absRoot)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(initial))
	for _, path := range initial {
		seen[path] = struct{}{}
	}
	fmt.Printf("%s を監視しています（既存ファイル %d 件は対象外）\n", absRoot, len(seen))
	interval := appConfig.Watcher.ScanIntervalSeconds
	if interval <= 0 {
		interval = 3
	}
	for {
		paths, err := scanImageFiles(absRoot)
		if err != nil {
			appendLog(fmt.Sprintf("watch scan error: %v", err))
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		for _, path := range paths {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			go func(p string) {
				if err := waitForStableFile(p); err != nil {
					entry := stateEntryForPath(p)
					entry.EagleStatus = "skipped"
					entry.AmazonStatus = "failed"
					entry.Error = fmt.Sprintf("ファイル安定待ちに失敗しました: %v", err)
					appendState(entry)
					appendLog(entry.Error)
					notifyFailure(entry)
					return
				}
				entry := processWatchedFile(p, false)
				if entry.Error != "" {
					fmt.Fprintf(os.Stderr, "watch processing error (%s): %s\n", p, entry.Error)
				}
			}(path)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

func scanImageFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.EqualFold(d.Name(), "annotated") {
				return filepath.SkipDir
			}
			return nil
		}
		if isSupportedInputFile(path, appConfig.Image.SupportedInputExtensions) {
			abs, err := filepath.Abs(path)
			if err == nil {
				paths = append(paths, abs)
			}
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

func processWatchedFile(path string, force bool) ProcessStateEntry {
	absPath, _ := filepath.Abs(path)
	if !force && alreadyProcessed(absPath) {
		entry := stateEntryForPath(absPath)
		entry.EagleStatus = "skipped"
		entry.AmazonStatus = "skipped"
		return entry
	}
	sourceType := classifySourceType(absPath)
	record := buildPhotoRecord(absPath, sourceType)
	entry := stateEntryForPath(absPath)
	entry.SourceType = sourceType
	entry.WorldID = record.WorldID
	entry.WorldName = record.WorldName
	entry.InstanceID = record.InstanceID
	entry.InstanceType = record.InstanceType
	entry.PresentUsers = record.PresentUsers
	entry.WorldFilledFromLog = record.WorldFilledFromLog

	if !eagleEnabled() {
		entry.EagleStatus = "skipped"
	} else if err := exportToEagle(record); err != nil {
		entry.EagleStatus = "failed"
		entry.Error = joinErrors(entry.Error, fmt.Sprintf("eagle: %v", err))
	} else {
		entry.EagleStatus = "success"
	}
	outputPath, err := exportToAmazon(record)
	entry.OutputPath = outputPath
	if err != nil {
		entry.AmazonStatus = "failed"
		entry.Error = joinErrors(entry.Error, fmt.Sprintf("amazon: %v", err))
	} else {
		entry.AmazonStatus = "success"
	}
	appendState(entry)
	if entry.Error != "" {
		appendLog(fmt.Sprintf("処理失敗: %s: %s", absPath, entry.Error))
		notifyFailure(entry)
	} else {
		appendLog(fmt.Sprintf("処理完了: %s", absPath))
	}
	return entry
}

func exportToEagle(record PhotoRecord) error {
	req := buildEagleRequest(record)
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	url := strings.TrimRight(appConfig.Eagle.BaseURL, "/") + "/api/v2/item/add"
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("HTTPステータス %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var result struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("Eagle API応答の解析に失敗しました: %v", err)
	}
	if !strings.EqualFold(result.Status, "success") {
		if result.Message == "" {
			result.Message = "詳細なし"
		}
		return fmt.Errorf("Eagle APIエラー: %s", result.Message)
	}
	return nil
}

func buildEagleRequest(record PhotoRecord) eagleAddRequest {
	tags := []string{"VRChat", "type:" + string(record.SourceType)}
	if record.WorldName != "" && (record.SourceType == SourceTypePhoto || record.SourceType == SourceTypePrint) {
		tags = append(tags, "wrld:"+record.WorldName)
	}
	if ym := shootMonth(record.ShootDate); ym != "" && (record.SourceType == SourceTypePhoto || record.SourceType == SourceTypePrint) {
		tags = append(tags, ym)
	}
	if record.SourceType == SourceTypePhoto {
		for _, user := range record.PresentUsers {
			if strings.TrimSpace(user) != "" {
				tags = append(tags, "user:"+strings.TrimSpace(user))
			}
		}
	}
	folders := append([]string{}, appConfig.Eagle.Folders...)
	if appConfig.Eagle.FolderID != "" {
		folders = append(folders, appConfig.Eagle.FolderID)
	}
	req := eagleAddRequest{
		Path:    record.SourcePath,
		Name:    strings.TrimSuffix(filepath.Base(record.SourcePath), filepath.Ext(record.SourcePath)),
		Tags:    tags,
		Folders: folders,
	}
	if (record.SourceType == SourceTypePhoto || record.SourceType == SourceTypePrint) && record.WorldID != "" {
		req.Website = "https://vrchat.com/home/world/" + record.WorldID
		var lines []string
		if record.WorldName != "" {
			lines = append(lines, "World: "+record.WorldName)
		}
		if record.InstanceID != "" {
			instance := record.InstanceID
			if record.InstanceType != "" {
				instance += "(" + record.InstanceType + ")"
			}
			lines = append(lines, "Instance: "+instance)
		}
		req.Annotation = strings.Join(lines, "\n")
	}
	return req
}

func exportToAmazon(record PhotoRecord) (string, error) {
	outputDir := amazonOutputDir()
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	outputPath := filepath.Join(outputDir, filepath.Base(record.SourcePath))
	if samePath(record.SourcePath, outputPath) {
		return "", errors.New("元画像の上書きを防止しました")
	}
	switch record.SourceType {
	case SourceTypePhoto:
		configMutex.Lock()
		oldOutput := appConfig.OutputDir
		appConfig.OutputDir = outputDir
		defer func() {
			appConfig.OutputDir = oldOutput
			configMutex.Unlock()
		}()
		worldURL := ""
		if record.WorldID != "" {
			worldURL = "https://vrchat.com/home/world/" + record.WorldID
		}
		worldIconName := ""
		if record.WorldFilledFromLog {
			worldIconName = "lock"
		}
		if err := addMetadataToImageWithWorldIcon(record.SourcePath, record.ShootDate, record.WorldName, record.AuthorName, "", worldURL, worldIconName); err != nil {
			return outputPath, err
		}
		return adjustOutputPath(outputPath, determineOutputFormat(record.SourcePath, appConfig.Image.OutputFormat)), nil
	case SourceTypePrint:
		if record.WorldID == "" {
			return outputPath, copyFile(record.SourcePath, outputPath)
		}
		return outputPath, addRMQROnlyCopy(record.SourcePath, outputPath, "https://vrchat.com/home/world/"+record.WorldID)
	default:
		return outputPath, copyFile(record.SourcePath, outputPath)
	}
}

func addRMQROnlyCopy(sourcePath, outputPath, worldURL string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()
	img, format, err := image.Decode(file)
	if err != nil {
		return err
	}
	bounds := img.Bounds()
	outImg := image.NewRGBA(bounds)
	draw.Draw(outImg, bounds, img, bounds.Min, draw.Src)
	if worldURL != "" {
		qrImg, err := generateRMQR(worldURL, false)
		if err == nil {
			qrBounds := qrImg.Bounds()
			scaleFactor := appConfig.Image.QRScaleFactor
			scaledWidth := qrBounds.Dx() * scaleFactor
			scaledHeight := qrBounds.Dy() * scaleFactor
			qrX := bounds.Dx() - scaledWidth - appConfig.Image.QRRightPadding
			if qrX < 0 {
				qrX = 0
			}
			qrY := 4
			scaledQR := image.NewRGBA(image.Rect(0, 0, scaledWidth, scaledHeight))
			xdraw.NearestNeighbor.Scale(scaledQR, scaledQR.Bounds(), qrImg, qrBounds, draw.Src, nil)
			bgRect := image.Rect(qrX, qrY, qrX+scaledWidth, qrY+scaledHeight)
			draw.Draw(outImg, bgRect, &image.Uniform{color.White}, image.Point{}, draw.Src)
			draw.Draw(outImg, bgRect, scaledQR, image.Point{}, draw.Over)
		}
	}
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	var encodeErr error
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		encodeErr = jpeg.Encode(outFile, outImg, &jpeg.Options{Quality: 95})
	case "webp":
		quality := float32(appConfig.Image.WebPCompressionQuality)
		if quality <= 0 || quality > 100 {
			quality = 100
		}
		encodeErr = webp.Encode(outFile, outImg, &webp.Options{Lossless: appConfig.Image.WebPLossless, Quality: quality})
	default:
		encodeErr = png.Encode(outFile, outImg)
	}
	closeErr := outFile.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func amazonOutputDir() string {
	if appConfig.Watcher.AmazonPhotosOutputDir != "" {
		return appConfig.Watcher.AmazonPhotosOutputDir
	}
	if appConfig.Watcher.VRChatPhotoRoot != "" {
		return filepath.Join(appConfig.Watcher.VRChatPhotoRoot, "Annotated")
	}
	return "Annotated"
}

func copyFile(sourcePath, outputPath string) error {
	if sourcePath == outputPath {
		return errors.New("元画像の上書きを防止しました")
	}
	in, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func testEagleConnection() error {
	url := strings.TrimRight(appConfig.Eagle.BaseURL, "/") + "/api/v2/app/info"
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTPステータス %s", resp.Status)
	}
	fmt.Printf("Eagle API 接続成功: %s\n", resp.Status)
	return nil
}

func retryFailed() error {
	entries, err := readStateEntries()
	if err != nil {
		return err
	}
	count := 0
	for _, entry := range entries {
		if entry.EagleStatus == "failed" || entry.AmazonStatus == "failed" {
			if _, err := os.Stat(entry.SourcePath); err != nil {
				continue
			}
			processWatchedFile(entry.SourcePath, true)
			count++
		}
	}
	fmt.Printf("失敗エントリを %d 件再試行しました\n", count)
	return nil
}

func appendState(entry ProcessStateEntry) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	path := appConfig.State.Path
	if path == "" {
		path = "watch-state.jsonl"
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		appendLog(fmt.Sprintf("state write failed: %v", err))
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	if err := enc.Encode(entry); err != nil {
		appendLog(fmt.Sprintf("state encode failed: %v", err))
	}
}

func readStateEntries() ([]ProcessStateEntry, error) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	path := appConfig.State.Path
	if path == "" {
		path = "watch-state.jsonl"
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var entries []ProcessStateEntry
	dec := json.NewDecoder(f)
	for {
		var entry ProcessStateEntry
		if err := dec.Decode(&entry); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return entries, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func alreadyProcessed(path string) bool {
	entries, err := readStateEntries()
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if samePath(entry.SourcePath, path) && entry.Size == info.Size() && entry.ModTimeUnix == info.ModTime().Unix() &&
			isTerminalSuccess(entry.EagleStatus) && isTerminalSuccess(entry.AmazonStatus) {
			return true
		}
	}
	return false
}

func isTerminalSuccess(status string) bool {
	return status == "success" || status == "skipped"
}

func stateEntryForPath(path string) ProcessStateEntry {
	entry := ProcessStateEntry{
		Timestamp:    time.Now().Format(time.RFC3339),
		SourcePath:   path,
		EagleStatus:  "pending",
		AmazonStatus: "pending",
	}
	if info, err := os.Stat(path); err == nil {
		entry.Size = info.Size()
		entry.ModTimeUnix = info.ModTime().Unix()
	}
	return entry
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA == nil {
		a = aa
	}
	if errB == nil {
		b = bb
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func shootMonth(shootDate string) string {
	if len(shootDate) >= 7 {
		return shootDate[:7]
	}
	return ""
}

func joinErrors(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "; " + b
}

func toastEnabled() bool {
	return appConfig.Notifications.ToastEnabled == nil || *appConfig.Notifications.ToastEnabled
}

func notifyFailure(entry ProcessStateEntry) {
	if !toastEnabled() || runtime.GOOS != "windows" {
		return
	}
	title := "VRC ss annotation"
	message := "処理に失敗しました: " + filepath.Base(entry.SourcePath)
	if entry.Error != "" {
		message += " - " + entry.Error
	}
	script := fmt.Sprintf(
		`Add-Type -AssemblyName System.Windows.Forms; $n=New-Object System.Windows.Forms.NotifyIcon; $n.Icon=[System.Drawing.SystemIcons]::Warning; $n.Visible=$true; $n.ShowBalloonTip(5000,'%s','%s',[System.Windows.Forms.ToolTipIcon]::Warning); Start-Sleep -Seconds 6; $n.Dispose()`,
		escapePowerShellSingleQuoted(title),
		escapePowerShellSingleQuoted(message),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	if err := cmd.Start(); err != nil {
		appendLog(fmt.Sprintf("notification failed: %v", err))
	}
}

func escapePowerShellSingleQuoted(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func main() {
	// コンフィグ読み込み
	loadConfig()
	if handled, err := runSubcommand(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
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
	return addMetadataToImageWithWorldIcon(imagePath, date, worldName, authorName, authorID, worldURL, "")
}

func addMetadataToImageWithWorldIcon(imagePath string, date string, worldName string, authorName string, authorID string, worldURL string, worldIconName string) error {
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
			_, err = outFile.Write(buf.Bytes())
			if err != nil {
				_ = outFile.Close()
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}

			// WebP 保存後に XMP メタデータを追加
			xmpAdded := false
			webpXMP, webpErr := extractTextualMetadataFromWebP(origData)
			pngXMP, pngErr := extractTextualMetadataFromPNG(origData)

			fmt.Fprintf(os.Stderr, "  [Metadata] WebP XMP: %s (%d bytes)\n", func() string {
				if webpErr != nil {
					return "ERROR"
				}
				if webpXMP == "" {
					return "NOT_FOUND"
				}
				return "OK"
			}(), len(webpXMP))
			fmt.Fprintf(os.Stderr, "  [Metadata] PNG XMP: %s (%d bytes)\n", func() string {
				if pngErr != nil {
					return "ERROR"
				}
				if pngXMP == "" {
					return "NOT_FOUND"
				}
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
			if err := png.Encode(outFile, outImg); err != nil {
				_ = outFile.Close()
				return err
			}
			if err := outFile.Close(); err != nil {
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
	addTextToImage(newImg, date, worldName, authorName, authorID, worldURL, marginTop, newWidth, newHeight, textColor, bgColor, isDark, worldURL != "", worldIconName)

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
		_, err = outFile.Write(buf.Bytes())
		if err != nil {
			_ = outFile.Close()
			return err
		}
		if err := outFile.Close(); err != nil {
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
		if err := png.Encode(outFile, newImg); err != nil {
			_ = outFile.Close()
			return err
		}
		if err := outFile.Close(); err != nil {
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
		"lock":     "lock_24dp_434343.svg",
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
func addTextToImage(img *image.RGBA, date, worldName, authorName, authorID, worldURL string, marginTop, origWidth, origHeight int, textColor, bgColor color.Color, isDark, needsQR bool, worldIconName string) error {
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
		if worldIconName == "" {
			worldIconName = "location"
		}
		if locIcon, err := loadSVGIcon(worldIconName, colorHex, iconSize); err == nil {
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
			result.Write(webpData[pos:chunkDataEnd])

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
			result.Write(webpData[pos:chunkDataEnd])

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
	chunkBuf.WriteByte(0)           // Null separator after keyword
	chunkBuf.WriteByte(0)           // Compression flag: 0 = uncompressed
	chunkBuf.WriteByte(0)           // Compression method (not used if uncompressed)
	chunkBuf.WriteByte(0)           // Null (language tag is empty)
	chunkBuf.WriteByte(0)           // Null (translated keyword is empty)
	chunkBuf.Write([]byte(xmpData)) // XMP text data
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
	result.Write(data[:iendPos])   // Everything before IEND
	result.Write(newChunk.Bytes()) // New iTXt chunk
	result.Write(data[iendPos:])   // Original IEND chunk

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
			newData.Write(data[offset:nextOffset])
		} else {
			// その他のチャンクはそのままコピー
			newData.Write(data[offset:nextOffset])
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
