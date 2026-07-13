package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func verifyOutputFormat(path, expected string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	actual := strings.ToLower(detectFileType(data))
	want := strings.ToLower(expected)
	if want == "jpg" || want == "jpeg" {
		want = "jpeg"
	}
	if actual == "webp" {
		actual = "webp"
	}
	if actual == "png" {
		actual = "png"
	}
	if actual == "jpeg" {
		actual = "jpeg"
	}
	if actual != want {
		return errors.New("output format does not match file content")
	}
	return nil
}

func determineOutputFormat(inputPath string, configFormat string) string {
	if configFormat == "" || configFormat == "auto" {
		if strings.HasSuffix(strings.ToLower(inputPath), ".webp") {
			return "webp"
		}
		return "png"
	}
	format := strings.ToLower(configFormat)
	if format == "webp" || format == "png" {
		return format
	}
	return "png"
}

func isSupportedInputFile(filePath string, supportedExts []string) bool {
	if len(supportedExts) == 0 {
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

func adjustOutputPath(outputPath string, outputFormat string) string {
	if outputFormat == "" || outputFormat == "auto" {
		return outputPath
	}

	format := strings.ToLower(outputFormat)
	newExt := ""
	if format == "webp" {
		newExt = ".webp"
	} else if format == "png" {
		newExt = ".png"
	} else {
		return outputPath
	}

	oldExt := filepath.Ext(outputPath)
	if strings.ToLower(oldExt) == newExt {
		return outputPath
	}
	if oldExt != "" {
		return outputPath[:len(outputPath)-len(oldExt)] + newExt
	}
	return outputPath + newExt
}
