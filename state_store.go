package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var stateMutex sync.Mutex

func appendState(entry ProcessStateEntry) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	path := appConfig.State.Path
	if path == "" {
		path = "watch-state.jsonl"
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		appendLog("state write failed: " + err.Error())
		return
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(entry); err != nil {
		appendLog("state encode failed: " + err.Error())
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
		if samePath(entry.SourcePath, path) && entry.Size == info.Size() && entry.ModTimeUnix == info.ModTime().Unix() && isTerminalSuccess(entry.EagleStatus) && isTerminalSuccess(entry.AmazonStatus) {
			return true
		}
	}
	return false
}

func isTerminalSuccess(status string) bool {
	return status == processingStatusSuccess || status == processingStatusSkipped
}

func stateEntryForPath(path string) ProcessStateEntry {
	entry := ProcessStateEntry{Timestamp: time.Now().Format(time.RFC3339), SourcePath: path, EagleStatus: processingStatusPending, AmazonStatus: processingStatusPending}
	if info, err := os.Stat(path); err == nil {
		entry.Size = info.Size()
		entry.ModTimeUnix = info.ModTime().Unix()
	}
	return entry
}

func samePath(a, b string) bool {
	if aa, err := filepath.Abs(a); err == nil {
		a = aa
	}
	if bb, err := filepath.Abs(b); err == nil {
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
