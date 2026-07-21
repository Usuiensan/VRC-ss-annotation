package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

	if !parseLeavingRoom("2026.07.10 00:03:00 Log -  [Behaviour] Leaving Room") {
		t.Fatal("parseLeavingRoom did not match Leaving Room")
	}
	if !parseLeavingRoom("2026.07.10 00:03:00 Log -  [Behaviour] OnLeftRoom") {
		t.Fatal("parseLeavingRoom did not match OnLeftRoom")
	}
}

func TestFormatDateForDisplayLegacyUppercaseWeekday(t *testing.T) {
	oldConfig := appConfig
	t.Cleanup(func() { appConfig = oldConfig })
	appConfig = getDefaultConfig()
	appConfig.DateFormat = "2006-01-02 MON"

	if got := formatDateForDisplay("2026-07-14T12:00:00+09:00"); got != "2026-07-14 TUE" {
		t.Fatalf("formatDateForDisplay = %q; want %q", got, "2026-07-14 TUE")
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

func TestPhotoTimeWithoutZoneUsesLocalTime(t *testing.T) {
	tm, ok := parsePhotoTime("2026-07-09T23:59:30")
	if !ok {
		t.Fatal("parsePhotoTime did not match local timestamp")
	}
	if tm.Location() != time.Local {
		t.Fatalf("location = %v; want time.Local", tm.Location())
	}
	if tm.Format("2006-01-02T15:04:05") != "2026-07-09T23:59:30" {
		t.Fatalf("time = %s", tm.Format(time.RFC3339))
	}
}

func TestBuildPhotoRecordUsesLocalPhotoTimeForSnapshot(t *testing.T) {
	oldContext := vrchatContext
	defer func() { vrchatContext = oldContext }()

	before, err := time.ParseInLocation("2006-01-02T15:04:05", "2026-07-09T23:59:00", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	after, err := time.ParseInLocation("2006-01-02T15:04:05", "2026-07-10T00:01:00", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	vrchatContext = &VRChatLogTracker{
		presentUsers: make(map[string]bool),
		history: []VRChatContextSnapshot{
			{
				At:        before,
				WorldID:   "wrld_before",
				WorldName: "Before World",
			},
			{
				At:        after,
				WorldID:   "wrld_after",
				WorldName: "After World",
			},
		},
	}
	path := filepath.Join(t.TempDir(), "VRChat_2026-07-09_23-59-30.000_3840x2160.png")
	if err := os.WriteFile(path, []byte("not a png"), 0644); err != nil {
		t.Fatal(err)
	}

	record := buildPhotoRecord(path, SourceTypePhoto)
	if record.WorldID != "wrld_before" || record.WorldName != "Before World" {
		t.Fatalf("record should use local-time snapshot before photo: %#v", record)
	}
}

func TestBuildPhotoRecordLoadsContextFromVisitLogs(t *testing.T) {
	oldConfig := appConfig
	oldContext := vrchatContext
	defer func() {
		appConfig = oldConfig
		vrchatContext = oldContext
	}()

	appConfig = getDefaultConfig()
	visitLogDir := t.TempDir()
	appConfig.Watcher.VisitLogDir = visitLogDir

	logPath := filepath.Join(visitLogDir, "vrchat-visits-2026-07-10.jsonl")
	events := []VRChatVisitEvent{
		{
			Timestamp:      "2026-07-10T01:00:00+09:00",
			Event:          "world_join",
			WorldID:        "wrld_67013904-2013-4dab-9c54-5e68b04f6a05",
			WorldName:      "ﾁｬﾝﾙｰﾑ",
			InstanceID:     "32692~friends(usr_owner)",
			InstanceType:   "friends",
			PresentUsers:   []string{},
			VisitStartedAt: "2026-07-10T01:00:00+09:00",
		},
		{
			Timestamp:    "2026-07-10T01:05:00+09:00",
			Event:        "player_join",
			UserName:     "うすいえんさん",
			PresentUsers: []string{"うすいえんさん"},
		},
		{
			Timestamp:    "2026-07-10T01:06:00+09:00",
			Event:        "player_join",
			UserName:     "エフギア",
			PresentUsers: []string{"うすいえんさん", "エフギア"},
		},
		{
			Timestamp:    "2026-07-10T01:07:00+09:00",
			Event:        "player_join",
			UserName:     "よるつき、",
			PresentUsers: []string{"うすいえんさん", "エフギア", "よるつき、"},
		},
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(logPath, b.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "VRChat_2026-07-10_01-08-46.000_3840x2160.png")
	if err := os.WriteFile(path, []byte("not a png"), 0644); err != nil {
		t.Fatal(err)
	}

	record := buildPhotoRecord(path, SourceTypePhoto)
	if record.WorldID != "wrld_67013904-2013-4dab-9c54-5e68b04f6a05" || record.WorldName != "ﾁｬﾝﾙｰﾑ" {
		t.Fatalf("record world from visit logs = %#v", record)
	}
	if !record.WorldFilledFromLog {
		t.Fatal("WorldFilledFromLog should be true")
	}
	if !reflect.DeepEqual(record.PresentUsers, []string{"うすいえんさん", "よるつき、", "エフギア"}) {
		t.Fatalf("record users = %#v", record.PresentUsers)
	}
	req := buildEagleRequest(record)
	wantTags := map[string]bool{
		"VRChat":       true,
		"type:photo":   true,
		"wrld:ﾁｬﾝﾙｰﾑ":  true,
		"2026-07":      true,
		"user:うすいえんさん": true,
		"user:エフギア":    true,
		"user:よるつき、":   true,
	}
	for _, tag := range req.Tags {
		delete(wantTags, tag)
	}
	if len(wantTags) > 0 {
		t.Fatalf("missing tags: %#v; got %#v", wantTags, req.Tags)
	}
	if req.Website != "https://vrchat.com/home/world/wrld_67013904-2013-4dab-9c54-5e68b04f6a05" {
		t.Fatalf("website = %q", req.Website)
	}
	if req.Annotation != "World: ﾁｬﾝﾙｰﾑ\nInstance: 32692 (Friends)" {
		t.Fatalf("annotation = %q", req.Annotation)
	}
}

func TestBuildPhotoRecordLoadsUsersFromRawOutputLogs(t *testing.T) {
	oldConfig := appConfig
	oldContext := vrchatContext
	defer func() {
		appConfig = oldConfig
		vrchatContext = oldContext
	}()

	appConfig = getDefaultConfig()
	logDir := t.TempDir()
	appConfig.Watcher.VRChatLogDir = logDir
	appConfig.Watcher.VisitLogDir = filepath.Join(t.TempDir(), "missing-visit-logs")

	logPath := filepath.Join(logDir, "output_log_2026-07-10_01-00-00.txt")
	logText := strings.Join([]string{
		"2026.07.10 01:00:00 Log -  [Behaviour] Joining wrld_raw:32692~friends(usr_owner)",
		"2026.07.10 01:00:03 Log -  [Behaviour] Entering Room: Raw Log World",
		`2026.07.10 01:01:00 Log -  [Behaviour] OnPlayerJoined "Alice" (usr_1)`,
		`2026.07.10 01:02:00 Log -  [Behaviour] OnPlayerJoined "Bob" (usr_2)`,
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(logText), 0644); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "VRChat_2026-07-10_01-03-00.000_3840x2160.png")
	if err := os.WriteFile(path, []byte("not a png"), 0644); err != nil {
		t.Fatal(err)
	}

	record := buildPhotoRecord(path, SourceTypePhoto)
	if record.WorldID != "wrld_raw" || record.WorldName != "Raw Log World" {
		t.Fatalf("record world from raw output logs = %#v", record)
	}
	if !reflect.DeepEqual(record.PresentUsers, []string{"Alice", "Bob"}) {
		t.Fatalf("record users = %#v", record.PresentUsers)
	}
	req := buildEagleRequest(record)
	wantTags := map[string]bool{
		"user:Alice": true,
		"user:Bob":   true,
	}
	for _, tag := range req.Tags {
		delete(wantTags, tag)
	}
	if len(wantTags) > 0 {
		t.Fatalf("missing user tags: %#v; got %#v", wantTags, req.Tags)
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

func TestBuildEagleRequestNormalizesAndDeduplicatesUserTags(t *testing.T) {
	oldConfig := appConfig
	defer func() { appConfig = oldConfig }()
	appConfig = getDefaultConfig()
	req := buildEagleRequest(PhotoRecord{
		SourcePath:   "photo.png",
		SourceType:   SourceTypePhoto,
		PresentUsers: []string{"user:/ player=うすいえんさん(local)", "うすいえんさん", "/ player=よるみや", "user:/ player=エフギア"},
	})
	want := []string{"user:うすいえんさん", "user:よるみや", "user:エフギア"}
	for _, tag := range want {
		found := false
		for _, got := range req.Tags {
			if got == tag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing normalized tag %q in %#v", tag, req.Tags)
		}
	}
	for _, got := range req.Tags {
		if strings.Contains(got, "player=") || strings.Contains(got, "/") {
			t.Errorf("raw player prefix leaked into tag %q", got)
		}
	}
}

func TestProcessStateForRecordPreservesRecordContext(t *testing.T) {
	record := PhotoRecord{
		SourcePath:         filepath.Join(t.TempDir(), "photo.png"),
		SourceType:         SourceTypePhoto,
		WorldID:            "wrld_test",
		WorldName:          "Test World",
		InstanceID:         "12345~friends(usr_owner)",
		InstanceType:       "friends",
		PresentUsers:       []string{"Alice"},
		WorldFilledFromLog: true,
	}

	entry := processStateForRecord(record)
	if entry.SourcePath != record.SourcePath || entry.SourceType != record.SourceType {
		t.Fatalf("source mapping = %#v", entry)
	}
	if entry.WorldID != record.WorldID || entry.WorldName != record.WorldName ||
		entry.InstanceID != record.InstanceID || entry.InstanceType != record.InstanceType {
		t.Fatalf("world mapping = %#v", entry)
	}
	if !reflect.DeepEqual(entry.PresentUsers, record.PresentUsers) || !entry.WorldFilledFromLog {
		t.Fatalf("context mapping = %#v", entry)
	}
}

func TestSkippedProcessStateUsesTerminalStatuses(t *testing.T) {
	entry := skippedProcessState(filepath.Join(t.TempDir(), "photo.png"))
	if entry.EagleStatus != processingStatusSkipped || entry.AmazonStatus != processingStatusSkipped {
		t.Fatalf("skipped state = %#v", entry)
	}
	if !isTerminalSuccess(entry.EagleStatus) || !isTerminalSuccess(entry.AmazonStatus) {
		t.Fatalf("skipped state should be terminal: %#v", entry)
	}
}

func TestWorldURLForID(t *testing.T) {
	if got := worldURLForID(""); got != "" {
		t.Fatalf("empty world ID URL = %q", got)
	}
	want := "https://vrchat.com/home/world/wrld_test"
	if got := worldURLForID("wrld_test"); got != want {
		t.Fatalf("world URL = %q; want %q", got, want)
	}
}

func TestResolveOutputDirPrefersExplicitOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "photo.png")
	if got := resolveOutputDir(path, "C:/export"); got != "C:/export" {
		t.Fatalf("override output dir = %q", got)
	}
}

func TestOutputFormatHelpers(t *testing.T) {
	if got := determineOutputFormat("photo.webp", "auto"); got != "webp" {
		t.Fatalf("auto WebP format = %q", got)
	}
	if got := determineOutputFormat("photo.png", "auto"); got != "png" {
		t.Fatalf("auto PNG format = %q", got)
	}
	if got := determineOutputFormat("photo.jpg", "invalid"); got != "png" {
		t.Fatalf("invalid format fallback = %q", got)
	}
	if !isSupportedInputFile("PHOTO.PNG", nil) || isSupportedInputFile("photo.gif", nil) {
		t.Fatal("supported extension detection mismatch")
	}
	if got := adjustOutputPath("photo.JPG", "webp"); got != "photo.webp" {
		t.Fatalf("adjusted output path = %q", got)
	}
	pngPath := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(pngPath, []byte{137, 80, 78, 71, 13, 10, 26, 10}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifyOutputFormat(pngPath, "png"); err != nil {
		t.Fatalf("PNG format verification failed: %v", err)
	}
	if err := verifyOutputFormat(pngPath, "webp"); err == nil {
		t.Fatal("mismatched output format was accepted")
	}
}

func TestClassifySourceType(t *testing.T) {
	cases := []struct {
		path string
		want SourceType
	}{
		{`C:\VRChat\VRChat_2026-07-13_12-34-56.123_3840x2160.png`, SourceTypePhoto},
		{`C:\VRCX\Tokiame_2026-07-18_02-02-22.384_prnt_9b9f76b4-f45c-4fe1-bae0-dc2ac05e425f.png`, SourceTypePrint},
		{`C:\VRChat\Print\image.png`, SourceTypePrint},
		{`C:\VRChat\Stickers\image.png`, SourceTypeSticker},
		{`C:\VRChat\emoji\smile.png`, SourceTypeEmoji},
		{`C:\VRChat\other\image.png`, SourceTypeUnknown},
	}
	for _, tc := range cases {
		if got := classifySourceType(tc.path); got != tc.want {
			t.Errorf("classifySourceType(%q) = %q; want %q", tc.path, got, tc.want)
		}
	}
}

func TestVRChatPrintCameraCopyIsExcludedFromScan(t *testing.T) {
	oldConfig := appConfig
	defer func() { appConfig = oldConfig }()
	appConfig = getDefaultConfig()

	root := t.TempDir()
	vrcxName := "Tokiame_2026-07-18_02-02-22.384_prnt_9b9f76b4-f45c-4fe1-bae0-dc2ac05e425f.png"
	vrchatName := "VRChat_2026-07-18_02-02-22.384_2048x1440.png"
	for _, name := range []string{vrcxName, vrchatName} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("image"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if !isVRChatPrintCameraCopy(vrchatName) {
		t.Fatal("VRChat Print Camera filename was not recognized")
	}
	paths, err := scanImageFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != vrcxName {
		t.Fatalf("scan paths = %#v; want only %q", paths, vrcxName)
	}
}

func TestVRCXPrintCameraFilenameMetadata(t *testing.T) {
	name := "Tokiame_2026-07-18_02-02-22.384_prnt_9b9f76b4-f45c-4fe1-bae0-dc2ac05e425f.png"
	author, shootDate, ok := vrcxPrintCameraFilenameMetadata(name)
	if !ok || author != "Tokiame" || shootDate != "2026-07-18T02:02:22.384" {
		t.Fatalf("metadata = (%q, %q, %v)", author, shootDate, ok)
	}
}

func TestPrintCameraUsesMetadataWorldIDBeforeLog(t *testing.T) {
	record := PhotoRecord{
		SourceType: SourceTypePrint,
		WorldID:    "wrld_metadata",
		WorldName:  "Metadata World",
		AuthorName: "Photographer",
	}
	applyLogSnapshotToRecord(&record, VRChatContextSnapshot{
		WorldID:      "wrld_log",
		WorldName:    "Log World",
		PresentUsers: []string{"Photographer"},
	})
	if record.WorldID != "wrld_metadata" || record.WorldName != "Metadata World" {
		t.Fatalf("metadata world was overwritten: %#v", record)
	}
	if record.WorldFilledFromLog {
		t.Fatal("metadata world was incorrectly marked as log-filled")
	}
}

func TestPrintCameraLogFallbackRequiresPhotographerPresence(t *testing.T) {
	snap := VRChatContextSnapshot{
		WorldID:      "wrld_log",
		WorldName:    "Log World",
		PresentUsers: []string{"Alice"},
	}

	absent := PhotoRecord{SourceType: SourceTypePrint, AuthorName: "Bob"}
	applyLogSnapshotToRecord(&absent, snap)
	if absent.WorldID != "" {
		t.Fatalf("world was filled without photographer presence: %#v", absent)
	}

	present := PhotoRecord{SourceType: SourceTypePrint, AuthorName: " alice "}
	applyLogSnapshotToRecord(&present, snap)
	if present.WorldID != "wrld_log" || !present.WorldFilledFromLog {
		t.Fatalf("world was not filled for present photographer: %#v", present)
	}
}

func TestVisitLogWorldLeaveClearsHistoricalUsers(t *testing.T) {
	tracker := &VRChatLogTracker{presentUsers: make(map[string]bool)}
	joined := time.Date(2026, 7, 18, 2, 0, 0, 0, time.Local)
	left := joined.Add(5 * time.Minute)
	tracker.applyVisitEvent(joined, VRChatVisitEvent{
		Event:        "world_join",
		WorldID:      "wrld_test",
		PresentUsers: []string{"Already Left"},
	})
	tracker.applyVisitEvent(left, VRChatVisitEvent{
		Event:        "world_leave",
		WorldID:      "wrld_test",
		PresentUsers: []string{"Already Left"},
	})

	snap := tracker.SnapshotAt(left.Add(time.Second))
	if snap.WorldID != "" || len(snap.PresentUsers) != 0 {
		t.Fatalf("leave snapshot retained stale context: %#v", snap)
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

func TestLeavingRoomEndsVisitAndClearsSnapshot(t *testing.T) {
	visitLogDir := t.TempDir()
	tracker := &VRChatLogTracker{
		visitLogDir:  visitLogDir,
		day:          "2026-07-09",
		presentUsers: make(map[string]bool),
	}
	logPath := filepath.Join(t.TempDir(), "output_log_2026-07-09.txt")

	tracker.handleLogLine(logPath, "2026.07.09 23:58:01 Log -  [Behaviour] Joining wrld_leave:12345~private(usr_owner)")
	tracker.handleLogLine(logPath, "2026.07.09 23:58:05 Log -  [Behaviour] Entering Room: Leave World")
	tracker.handleLogLine(logPath, `2026.07.09 23:58:10 Log -  [Behaviour] OnPlayerJoined "Alice" (usr_1)`)
	tracker.handleLogLine(logPath, "2026.07.09 23:59:00 Log -  [Behaviour] Leaving Room")

	snap := tracker.SnapshotAt(time.Now())
	if snap.WorldID != "" || snap.WorldName != "" || len(snap.PresentUsers) != 0 {
		t.Fatalf("snapshot after leaving room should be empty: %#v", snap)
	}
	events := readAllVisitEvents(t, visitLogDir)
	if countVisitEvents(events, "world_leave") != 1 {
		t.Fatalf("world_leave event count mismatch: %#v", events)
	}
	var leaveEvent VRChatVisitEvent
	for _, event := range events {
		if event.Event == "world_leave" {
			leaveEvent = event
			break
		}
	}
	if leaveEvent.WorldID != "wrld_leave" || leaveEvent.WorldName != "Leave World" {
		t.Fatalf("world_leave should keep previous world context: %#v", leaveEvent)
	}
	if leaveEvent.VisitEndedAt == "" || leaveEvent.DurationSeconds <= 0 {
		t.Fatalf("world_leave should include end time and duration: %#v", leaveEvent)
	}
}

func TestPollReadsOnlyAppendedLogLines(t *testing.T) {
	logDir := t.TempDir()
	visitLogDir := t.TempDir()
	logPath := filepath.Join(logDir, "output_log_2026-07-09_23-58-00.txt")
	initial := "2026.07.09 23:58:01 Log -  [Behaviour] Joining wrld_tail:12345~friends(usr_owner)\n" +
		"2026.07.09 23:58:05 Log -  [Behaviour] Entering Room: Tail World\n"
	if err := os.WriteFile(logPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	tracker := &VRChatLogTracker{
		logDir:       logDir,
		visitLogDir:  visitLogDir,
		day:          "2026-07-09",
		presentUsers: make(map[string]bool),
	}
	tracker.poll()
	firstEvents := readAllVisitEvents(t, visitLogDir)
	if !containsVisitEvent(firstEvents, "world_join", false) || !containsVisitEvent(firstEvents, "world_name", false) {
		t.Fatalf("initial events missing: %#v", firstEvents)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`2026.07.09 23:58:10 Log -  [Behaviour] OnPlayerJoined "Alice" (usr_1)` + "\n"); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	tracker.poll()

	events := readAllVisitEvents(t, visitLogDir)
	if countVisitEvents(events, "world_join") != 1 {
		t.Fatalf("world_join should not be reprocessed: %#v", events)
	}
	if countVisitEvents(events, "player_join") != 1 {
		t.Fatalf("appended player_join was not processed once: %#v", events)
	}
	snap := tracker.SnapshotAt(time.Now())
	if !reflect.DeepEqual(snap.PresentUsers, []string{"Alice"}) {
		t.Fatalf("snapshot users after appended line = %#v", snap.PresentUsers)
	}
}

func TestInitialPollUsesLogDateForVisitLog(t *testing.T) {
	logDir := t.TempDir()
	visitLogDir := t.TempDir()
	logPath := filepath.Join(logDir, "output_log_2026-07-09_23-58-00.txt")
	if err := os.WriteFile(logPath, []byte("2026.07.09 23:58:01 Log -  [Behaviour] Joining wrld_initial:12345~friends(usr_owner)\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tracker := &VRChatLogTracker{
		logDir:       logDir,
		visitLogDir:  visitLogDir,
		presentUsers: make(map[string]bool),
	}
	tracker.poll()

	if _, err := os.Stat(filepath.Join(visitLogDir, "vrchat-visits-2026-07-09.jsonl")); err != nil {
		t.Fatalf("visit log should use log timestamp date: %v", err)
	}
	if tracker.day != "2026-07-09" {
		t.Fatalf("tracker day = %q; want log date", tracker.day)
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

func countVisitEvents(events []VRChatVisitEvent, eventName string) int {
	count := 0
	for _, event := range events {
		if event.Event == eventName {
			count++
		}
	}
	return count
}
