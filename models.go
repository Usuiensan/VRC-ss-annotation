package main

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
