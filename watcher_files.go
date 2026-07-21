package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// waitForStableFile waits until a newly-created image stops changing size and
// modification time. This prevents processing a file while VRChat is still
// writing it.
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
		if isVRChatPrintCameraCopy(path) {
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
