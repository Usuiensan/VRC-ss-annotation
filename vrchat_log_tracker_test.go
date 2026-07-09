package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestVRChatLogParsers(t *testing.T) {
	worldID, instanceID, ok := parseJoiningWorld("2026.07.09 23:58:01 Log -  [Behaviour] Joining wrld_12345678-abcd-ef00-1111-222233334444:12345~private(usr_abc)~region(jp)")
	if !ok {
		t.Fatal("parseJoiningWorld did not match")
	}
	if worldID != "wrld_12345678-abcd-ef00-1111-222233334444" {
		t.Fatalf("worldID = %q", worldID)
	}
	if instanceID != "12345~private(usr_abc)~region(jp)" {
		t.Fatalf("instanceID = %q", instanceID)
	}
	if got := classifyInstanceType(instanceID); got != "private" {
		t.Fatalf("classifyInstanceType = %q", got)
	}

	worldName, ok := parseEnteringRoom("2026.07.09 23:58:05 Log -  [Behaviour] Entering Room: Test World")
	if !ok || worldName != "Test World" {
		t.Fatalf("parseEnteringRoom = %q, %v", worldName, ok)
	}

	userName, ok := parsePlayerJoined(`2026.07.09 23:58:10 Log -  [Behaviour] OnPlayerJoined "Alice" (usr_1)`)
	if !ok || userName != "Alice" {
		t.Fatalf("parsePlayerJoined = %q, %v", userName, ok)
	}

	userName, ok = parsePlayerLeft(`2026.07.09 23:59:10 Log -  [Behaviour] OnPlayerLeft: Bob (usr_2)`)
	if !ok || userName != "Bob" {
		t.Fatalf("parsePlayerLeft = %q, %v", userName, ok)
	}
}

func TestVRChatLogTrackerSnapshotAndVisitLogs(t *testing.T) {
	visitLogDir := t.TempDir()
	tracker := &VRChatLogTracker{
		visitLogDir:  visitLogDir,
		day:          "2026-07-09",
		presentUsers: make(map[string]bool),
	}
	logPath := filepath.Join(t.TempDir(), "output_log_2026-07-09.txt")

	tracker.handleLogLine(logPath, "2026.07.09 23:58:01 Log -  [Behaviour] Joining wrld_test:12345~friends(usr_owner)~region(jp)")
	tracker.handleLogLine(logPath, "2026.07.09 23:58:05 Log -  [Behaviour] Entering Room: Midnight World")
	tracker.handleLogLine(logPath, `2026.07.09 23:58:10 Log -  [Behaviour] OnPlayerJoined "Alice" (usr_1)`)
	tracker.handleLogLine(logPath, `2026.07.09 23:59:10 Log -  [Behaviour] OnPlayerJoined "Bob" (usr_2)`)
	tracker.handleLogLine(logPath, `2026.07.10 00:00:05 Log -  [Behaviour] OnPlayerLeft: Alice (usr_1)`)

	at, err := time.ParseInLocation("2006.01.02 15:04:05", "2026.07.09 23:59:30", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	snap := tracker.SnapshotAt(at)
	if snap.WorldID != "wrld_test" || snap.WorldName != "Midnight World" || snap.InstanceType != "friends" {
		t.Fatalf("snapshot world = %#v", snap)
	}
	if !reflect.DeepEqual(snap.PresentUsers, []string{"Alice", "Bob"}) {
		t.Fatalf("snapshot users = %#v", snap.PresentUsers)
	}

	oldEvents := readVisitEvents(t, filepath.Join(visitLogDir, "vrchat-visits-2026-07-09.jsonl"))
	newEvents := readVisitEvents(t, filepath.Join(visitLogDir, "vrchat-visits-2026-07-10.jsonl"))
	if !containsVisitEvent(oldEvents, "day_rollover_end", true) {
		t.Fatalf("old day events do not contain day_rollover_end: %#v", oldEvents)
	}
	if !containsVisitEvent(newEvents, "day_rollover_start", true) {
		t.Fatalf("new day events do not contain day_rollover_start: %#v", newEvents)
	}
	if !containsVisitEvent(newEvents, "player_left", false) {
		t.Fatalf("new day events do not contain player_left: %#v", newEvents)
	}
}

func TestBuildPhotoRecordFillsWorldAndUsersFromLogSnapshot(t *testing.T) {
	oldContext := vrchatContext
	defer func() { vrchatContext = oldContext }()

	at, err := time.ParseInLocation("2006-01-02T15:04:05", "2026-07-09T23:59:30", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	vrchatContext = &VRChatLogTracker{
		presentUsers: map[string]bool{"Alice": true, "Bob": true},
		history: []VRChatContextSnapshot{
			{
				At:           at.Add(-time.Minute),
				WorldID:      "wrld_from_log",
				WorldName:    "Logged World",
				InstanceID:   "12345~hidden(usr_owner)",
				InstanceType: "hidden",
				PresentUsers: []string{"Alice", "Bob"},
			},
		},
	}
	path := filepath.Join(t.TempDir(), "VRChat_2026-07-09_23-59-30.000_3840x2160.png")
	if err := os.WriteFile(path, []byte("not a png"), 0644); err != nil {
		t.Fatal(err)
	}

	record := buildPhotoRecord(path, SourceTypePhoto)
	if record.WorldID != "wrld_from_log" || record.WorldName != "Logged World" {
		t.Fatalf("record world = %#v", record)
	}
	if !record.WorldFilledFromLog {
		t.Fatal("WorldFilledFromLog is false")
	}
	if !reflect.DeepEqual(record.PresentUsers, []string{"Alice", "Bob"}) {
		t.Fatalf("record users = %#v", record.PresentUsers)
	}
	if record.InstanceType != "hidden" {
		t.Fatalf("record instance type = %q", record.InstanceType)
	}
}

func TestBuildEagleRequestAddsPresentUserTags(t *testing.T) {
	oldConfig := appConfig
	defer func() { appConfig = oldConfig }()
	appConfig = getDefaultConfig()

	req := buildEagleRequest(PhotoRecord{
		SourcePath:   filepath.Join(t.TempDir(), "photo.png"),
		SourceType:   SourceTypePhoto,
		WorldID:      "wrld_test",
		WorldName:    "Test World",
		ShootDate:    "2026-07-09T23:59:30",
		InstanceID:   "12345~group(grp_1)",
		InstanceType: "group",
		PresentUsers: []string{"Alice", " Bob ", ""},
	})

	wantTags := map[string]bool{
		"VRChat":          true,
		"type:photo":      true,
		"wrld:Test World": true,
		"2026-07":         true,
		"user:Alice":      true,
		"user:Bob":        true,
	}
	for _, tag := range req.Tags {
		delete(wantTags, tag)
	}
	if len(wantTags) > 0 {
		t.Fatalf("missing tags: %#v; got %#v", wantTags, req.Tags)
	}
	if req.Website != "https://vrchat.com/home/world/wrld_test" {
		t.Fatalf("website = %q", req.Website)
	}
}

func TestLogFileChangeEndsPreviousVisitAndClearsSnapshot(t *testing.T) {
	logDir := t.TempDir()
	visitLogDir := t.TempDir()
	firstLog := filepath.Join(logDir, "output_log_2026-07-09_23-58-00.txt")
	secondLog := filepath.Join(logDir, "output_log_2026-07-10_00-05-00.txt")
	if err := os.WriteFile(firstLog, []byte("2026.07.09 23:58:01 Log -  [Behaviour] Joining wrld_old:12345~private(usr_owner)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(firstLog, time.Now().Add(-time.Minute), time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	tracker := &VRChatLogTracker{
		logDir:       logDir,
		visitLogDir:  visitLogDir,
		day:          time.Now().Format("2006-01-02"),
		presentUsers: make(map[string]bool),
	}
	tracker.poll()
	if snap := tracker.SnapshotAt(time.Now()); snap.WorldID != "wrld_old" {
		t.Fatalf("initial snapshot = %#v", snap)
	}

	if err := os.WriteFile(secondLog, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(secondLog, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	tracker.poll()
	if snap := tracker.SnapshotAt(time.Now().Add(time.Second)); snap.WorldID != "" || snap.InstanceID != "" {
		t.Fatalf("snapshot after log change should be empty: %#v", snap)
	}

	events := readAllVisitEvents(t, visitLogDir)
	if !containsVisitEvent(events, "log_file_changed", false) {
		t.Fatalf("events do not contain log_file_changed: %#v", events)
	}
}

func readVisitEvents(t *testing.T, path string) []VRChatVisitEvent {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var events []VRChatVisitEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event VRChatVisitEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func readAllVisitEvents(t *testing.T, dir string) []VRChatVisitEvent {
	t.Helper()
	var events []VRChatVisitEvent
	matches, err := filepath.Glob(filepath.Join(dir, "vrchat-visits-*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range matches {
		events = append(events, readVisitEvents(t, path)...)
	}
	return events
}

func containsVisitEvent(events []VRChatVisitEvent, eventName string, continues bool) bool {
	for _, event := range events {
		if event.Event != eventName {
			continue
		}
		if continues && !event.Continues {
			continue
		}
		return true
	}
	return false
}
