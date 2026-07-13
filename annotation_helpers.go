package main

import (
	"image"
	"image/color"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/shogo82148/qrcode/rmqr"
)

func extractDateFromFilename(filePath string) string {
	filename := filepath.Base(filePath)
	re1 := regexp.MustCompile(`VRChat_(?:\d+x\d+_)?(\d{4}-\d{2}-\d{2})_(\d{2}-\d{2}-\d{2})`)
	if matches := re1.FindStringSubmatch(filename); len(matches) > 2 {
		return matches[1] + "T" + strings.ReplaceAll(matches[2], "-", ":")
	}
	re2 := regexp.MustCompile(`-(\d{8})-(\d{6})`)
	if matches := re2.FindStringSubmatch(filename); len(matches) > 2 {
		dateStr, timeStr := matches[1], matches[2]
		return dateStr[0:4] + "-" + dateStr[4:6] + "-" + dateStr[6:8] + "T" + timeStr[0:2] + ":" + timeStr[2:4] + ":" + timeStr[4:6]
	}
	return ""
}

func formatDateForDisplay(dateStr string) string {
	layout := strings.TrimSpace(appConfig.DateFormat)
	useUpperWeekday := false
	if layout == "" {
		layout = "2006-01-02 Mon 15:04:05"
		useUpperWeekday = true
	}
	candidates := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02 15:04:05.000", "2006-01-02"}
	for _, pattern := range candidates {
		if parsed, err := time.Parse(pattern, dateStr); err == nil {
			formatted := parsed.Format(layout)
			if useUpperWeekday {
				weekday := parsed.Format("Mon")
				formatted = strings.ReplaceAll(formatted, weekday, strings.ToUpper(weekday))
			}
			return formatted
		}
	}
	return dateStr
}

func generateRMQR(url string, isDark bool) (image.Image, error) {
	qrImage, err := rmqr.Encode([]byte(url), rmqr.WithLevel(rmqr.LevelM), rmqr.WithPriority(rmqr.PriorityHeight))
	if err != nil {
		return nil, err
	}
	if isDark {
		return invertImage(qrImage), nil
	}
	return qrImage, nil
}

func invertImage(img image.Image) image.Image {
	bounds := img.Bounds()
	inverted := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			inverted.SetRGBA(x, y, color.RGBA{R: 255 - uint8(r>>8), G: 255 - uint8(g>>8), B: 255 - uint8(b>>8), A: uint8(a >> 8)})
		}
	}
	return inverted
}
