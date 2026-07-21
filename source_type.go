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
var vrcxPrintCameraFilenamePattern = regexp.MustCompile(`(?i)^(.+)_([0-9]{4}-[0-9]{2}-[0-9]{2})_([0-9]{2}-[0-9]{2}-[0-9]{2})\.([0-9]+)_prnt_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}(?:\.[^.]+)?$`)
var vrchatPrintCameraFilenamePattern = regexp.MustCompile(`(?i)^VRChat_.*_2048x1440\.[^.]+$`)

func isVRChatPrintCameraCopy(path string) bool {
	return vrchatPrintCameraFilenamePattern.MatchString(filepath.Base(path))
}

func vrcxPrintCameraFilenameMetadata(path string) (authorName, shootDate string, ok bool) {
	matches := vrcxPrintCameraFilenamePattern.FindStringSubmatch(filepath.Base(path))
	if len(matches) != 5 {
		return "", "", false
	}
	return matches[1], matches[2] + "T" + strings.ReplaceAll(matches[3], "-", ":") + "." + matches[4], true
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
	if vrcxPrintCameraFilenamePattern.MatchString(filepath.Base(path)) {
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
