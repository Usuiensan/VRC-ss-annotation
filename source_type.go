package main

import (
	"path/filepath"
	"regexp"
	"strings"
)

type SourceType string

const (
	SourceTypePhoto   SourceType = "photo"
	SourceTypePrint   SourceType = "print"
	SourceTypeSticker SourceType = "sticker"
	SourceTypeStamp   SourceType = "stamp"
	SourceTypeEmoji   SourceType = "emoji"
	SourceTypeUnknown SourceType = "unknown"
)

var vrchatPhotoFilenamePattern = regexp.MustCompile(`(?i)^VRChat_\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}\.\d+_.*`)

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
	if vrchatPhotoFilenamePattern.MatchString(lowerBase) {
		return SourceTypePhoto
	}
	return SourceTypeUnknown
}
