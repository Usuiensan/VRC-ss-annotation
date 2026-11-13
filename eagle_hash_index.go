package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const eagleIndexSyncInterval = 60 * time.Second

type eagleIndexedItem struct {
	ID       string
	Hash     string
	Path     string
	Size     int64
	Modified int64
}

type eagleListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Ext       string `json:"ext"`
	FilePath  string `json:"filePath"`
	IsDeleted bool   `json:"isDeleted"`
}

type eagleImportResult struct {
	Hash      string
	ItemID    string
	Duplicate bool
}

// eagleHashIndex serializes imports and index synchronization. In particular,
// two newly-created copies of the same image cannot both pass the hash check.
type eagleHashIndex struct {
	mu          sync.Mutex
	db          *sql.DB
	client      *http.Client
	baseURL     string
	libraryPath string
	lastSync    time.Time
}

var eagleIndexSingleton struct {
	sync.Mutex
	index *eagleHashIndex
	path  string
	url   string
}

func configuredEagleIndexPath() (string, error) {
	if appConfig.Eagle.HashIndexPath != "" {
		return filepath.Abs(appConfig.Eagle.HashIndexPath)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "VRCSSAnnotationTool", "eagle-hash-index.sqlite"), nil
}

func openEagleHashIndex(path, baseURL string) (*eagleHashIndex, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"PRAGMA busy_timeout = 5000",
		`CREATE TABLE IF NOT EXISTS eagle_items (
			library_path TEXT NOT NULL,
			item_id TEXT NOT NULL,
			sha256 TEXT NOT NULL,
			file_path TEXT NOT NULL,
			size INTEGER NOT NULL,
			mtime_ns INTEGER NOT NULL,
			PRIMARY KEY (library_path, item_id)
		)`,
		"CREATE INDEX IF NOT EXISTS eagle_items_hash ON eagle_items(library_path, sha256)",
	} {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("Eagleハッシュ索引の初期化: %w", err)
		}
	}
	return &eagleHashIndex{db: db, client: &http.Client{Timeout: 15 * time.Second}, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func getEagleHashIndex() (*eagleHashIndex, error) {
	path, err := configuredEagleIndexPath()
	if err != nil {
		return nil, err
	}
	eagleIndexSingleton.Lock()
	defer eagleIndexSingleton.Unlock()
	baseURL := strings.TrimRight(appConfig.Eagle.BaseURL, "/")
	if eagleIndexSingleton.index != nil && eagleIndexSingleton.path == path && eagleIndexSingleton.url == baseURL {
		return eagleIndexSingleton.index, nil
	}
	idx, err := openEagleHashIndex(path, baseURL)
	if err != nil {
		return nil, err
	}
	if eagleIndexSingleton.index != nil {
		_ = eagleIndexSingleton.index.db.Close()
	}
	eagleIndexSingleton.index = idx
	eagleIndexSingleton.path = path
	eagleIndexSingleton.url = baseURL
	return idx, nil
}

func (idx *eagleHashIndex) getJSON(endpoint string, result any) error {
	resp, err := idx.client.Get(idx.baseURL + endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Eagle API %s: %s", endpoint, resp.Status)
	}
	var envelope struct {
		Status  string          `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Status != "success" {
		return fmt.Errorf("Eagle API %s: %s", endpoint, envelope.Message)
	}
	return json.Unmarshal(envelope.Data, result)
}

func eagleLibraryKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

func (idx *eagleHashIndex) currentLibrary() (string, error) {
	var info struct {
		Path string `json:"path"`
	}
	if err := idx.getJSON("/api/v2/library/info", &info); err != nil {
		return "", err
	}
	if strings.TrimSpace(info.Path) == "" {
		return "", errors.New("Eagle APIがライブラリのパスを返しませんでした")
	}
	if _, err := os.Stat(filepath.Join(info.Path, "images")); err != nil {
		return "", fmt.Errorf("Eagleライブラリにアクセスできません: %w", err)
	}
	return eagleLibraryKey(info.Path), nil
}

func (idx *eagleHashIndex) listItems() ([]eagleListItem, error) {
	const limit = 1000
	var items []eagleListItem
	seen := make(map[string]bool)
	for offset := 0; ; {
		values := url.Values{}
		values.Set("limit", fmt.Sprint(limit))
		values.Set("offset", fmt.Sprint(offset))
		values.Set("fields", "id,name,ext,filePath,isDeleted")
		var page struct {
			Items []eagleListItem `json:"data"`
			Total int             `json:"total"`
		}
		if err := idx.getJSON("/api/v2/item/get?"+values.Encode(), &page); err != nil {
			return nil, err
		}
		if page.Total > 0 && len(page.Items) == 0 && offset < page.Total {
			return nil, errors.New("Eagleアイテム一覧の途中で空のページが返されました")
		}
		for _, item := range page.Items {
			if item.ID == "" || seen[item.ID] {
				return nil, fmt.Errorf("EagleアイテムIDの欠落または重複: %q", item.ID)
			}
			seen[item.ID] = true
			items = append(items, item)
		}
		if offset+len(page.Items) >= page.Total {
			return items, nil
		}
		offset += len(page.Items)
	}
}

func supportedEagleImage(ext string) bool {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "png", "jpg", "jpeg", "webp":
		return true
	default:
		return false
	}
}

func fileInsideDir(filePath, dir string) bool {
	rel, err := filepath.Rel(dir, filePath)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolveEagleOriginal never guesses between multiple possible originals.
// The API's filePath is preferred, but older builds may omit it.
func resolveEagleOriginal(library string, item eagleListItem) (string, error) {
	itemDir := filepath.Join(library, "images", item.ID+".info")
	if item.FilePath != "" && fileInsideDir(item.FilePath, itemDir) {
		if info, err := os.Stat(item.FilePath); err == nil && info.Mode().IsRegular() {
			return item.FilePath, nil
		}
	}
	if item.Name != "" {
		candidate := filepath.Join(itemDir, item.Name+"."+strings.TrimPrefix(item.Ext, "."))
		if fileInsideDir(candidate, itemDir) {
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
				return candidate, nil
			}
		}
	}
	files, err := os.ReadDir(itemDir)
	if err != nil {
		return "", err
	}
	var candidates []string
	for _, file := range files {
		if !file.Type().IsRegular() {
			continue
		}
		name := strings.ToLower(file.Name())
		if strings.HasSuffix(name, "_thumbnail.png") || strings.Contains(name, "_preview.") {
			continue
		}
		if strings.EqualFold(filepath.Ext(name), "."+strings.TrimPrefix(item.Ext, ".")) {
			candidates = append(candidates, filepath.Join(itemDir, file.Name()))
		}
	}
	if len(candidates) != 1 {
		return "", fmt.Errorf("元画像の候補が%d件です (item=%s)", len(candidates), item.ID)
	}
	return candidates[0], nil
}

// hashStableFile avoids recording a hash of a file being written or replaced.
func hashStableFile(path string) (string, int64, int64, error) {
	for attempt := 0; attempt < 3; attempt++ {
		before, err := os.Stat(path)
		if err != nil {
			return "", 0, 0, err
		}
		if !before.Mode().IsRegular() {
			return "", 0, 0, fmt.Errorf("通常ファイルではありません: %s", path)
		}
		f, err := os.Open(path)
		if err != nil {
			return "", 0, 0, err
		}
		hasher := sha256.New()
		_, copyErr := io.Copy(hasher, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", 0, 0, copyErr
		}
		if closeErr != nil {
			return "", 0, 0, closeErr
		}
		after, err := os.Stat(path)
		if err != nil {
			return "", 0, 0, err
		}
		if before.Size() == after.Size() && before.ModTime().Equal(after.ModTime()) {
			return hex.EncodeToString(hasher.Sum(nil)), after.Size(), after.ModTime().UnixNano(), nil
		}
	}
	return "", 0, 0, fmt.Errorf("ハッシュ計算中にファイルが変更されました: %s", path)
}

func (idx *eagleHashIndex) syncLocked(force bool) error {
	library, err := idx.currentLibrary()
	if err != nil {
		return err
	}
	if !force && library == idx.libraryPath && time.Since(idx.lastSync) < eagleIndexSyncInterval {
		return nil
	}
	items, err := idx.listItems()
	if err != nil {
		return err
	}
	rows, err := idx.db.Query("SELECT item_id, sha256, file_path, size, mtime_ns FROM eagle_items WHERE library_path = ?", library)
	if err != nil {
		return err
	}
	previous := make(map[string]eagleIndexedItem)
	for rows.Next() {
		var item eagleIndexedItem
		if err := rows.Scan(&item.ID, &item.Hash, &item.Path, &item.Size, &item.Modified); err != nil {
			_ = rows.Close()
			return err
		}
		previous[item.ID] = item
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()

	updated := make([]eagleIndexedItem, 0)
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if item.IsDeleted || !supportedEagleImage(item.Ext) {
			continue
		}
		path, err := resolveEagleOriginal(library, item)
		if err != nil {
			return fmt.Errorf("Eagle画像 %s: %w", item.ID, err)
		}
		seen[item.ID] = true
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if old, ok := previous[item.ID]; !force && ok && old.Path == path && old.Size == info.Size() && old.Modified == info.ModTime().UnixNano() {
			continue
		}
		hash, size, modified, err := hashStableFile(path)
		if err != nil {
			return err
		}
		updated = append(updated, eagleIndexedItem{ID: item.ID, Hash: hash, Path: path, Size: size, Modified: modified})
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range updated {
		if _, err := tx.Exec(`INSERT INTO eagle_items (library_path, item_id, sha256, file_path, size, mtime_ns)
			VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(library_path, item_id) DO UPDATE SET
			sha256=excluded.sha256, file_path=excluded.file_path, size=excluded.size, mtime_ns=excluded.mtime_ns`,
			library, item.ID, item.Hash, item.Path, item.Size, item.Modified); err != nil {
			return err
		}
	}
	for id := range previous {
		if !seen[id] {
			if _, err := tx.Exec("DELETE FROM eagle_items WHERE library_path = ? AND item_id = ?", library, id); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	idx.libraryPath = library
	idx.lastSync = time.Now()
	return nil
}

func (idx *eagleHashIndex) Sync(force bool) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.syncLocked(force)
}

func (idx *eagleHashIndex) Import(record PhotoRecord) (eagleImportResult, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	var result eagleImportResult
	if err := idx.syncLocked(false); err != nil {
		return result, fmt.Errorf("Eagleハッシュ索引を同期できません: %w", err)
	}
	hash, size, modified, err := hashStableFile(record.SourcePath)
	if err != nil {
		return result, err
	}
	result.Hash = hash
	lookup := func() error {
		result.ItemID = ""
		return idx.db.QueryRow("SELECT item_id FROM eagle_items WHERE library_path = ? AND sha256 = ? LIMIT 1", idx.libraryPath, hash).Scan(&result.ItemID)
	}
	err = lookup()
	if err == nil {
		result.Duplicate = true
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	// A miss must be checked against the latest Eagle contents, including
	// imports performed manually since the last periodic sync. Existing hashes
	// are reused, so this does not rehash the entire library.
	idx.lastSync = time.Time{}
	if err := idx.syncLocked(false); err != nil {
		return result, fmt.Errorf("インポート直前のEagle再照合に失敗: %w", err)
	}
	err = lookup()
	if err == nil {
		result.Duplicate = true
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	itemID, err := exportToEagle(record)
	if err != nil {
		return result, err
	}
	result.ItemID = itemID
	if itemID != "" {
		if _, err := idx.db.Exec(`INSERT INTO eagle_items (library_path, item_id, sha256, file_path, size, mtime_ns)
			VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(library_path, item_id) DO UPDATE SET
			sha256=excluded.sha256, file_path=excluded.file_path, size=excluded.size, mtime_ns=excluded.mtime_ns`,
			idx.libraryPath, itemID, hash, record.SourcePath, size, modified); err != nil {
			// The import succeeded: an index write failure must not cause a retry
			// that would send the same image to Eagle again.
			idx.lastSync = time.Time{}
			appendLog("Eagle登録は成功しましたが索引への保存に失敗しました: " + err.Error())
		}
	} else {
		idx.lastSync = time.Time{}
		appendLog("Eagle登録応答にアイテムIDがありません。次回索引同期で補完します")
	}
	return result, nil
}

func (idx *eagleHashIndex) Status() (string, int, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	library, err := idx.currentLibrary()
	if err != nil {
		return "", 0, err
	}
	var count int
	err = idx.db.QueryRow("SELECT COUNT(*) FROM eagle_items WHERE library_path = ?", library).Scan(&count)
	return library, count, err
}

// The initial sync runs in the background so the file watcher can retain
// new arrivals even when the Eagle library takes minutes to hash initially.
func startEagleIndexSync() {
	idx, err := getEagleHashIndex()
	if err != nil {
		appendLog("Eagleハッシュ索引を開けません: " + err.Error())
		return
	}
	if err := idx.Sync(false); err != nil {
		appendLog("Eagleハッシュ索引の初回同期失敗: " + err.Error())
	} else {
		appendLog("Eagleハッシュ索引の初回同期完了")
	}
	ticker := time.NewTicker(eagleIndexSyncInterval)
	defer ticker.Stop()
	for range ticker.C {
		if err := idx.Sync(false); err != nil {
			appendLog("Eagleハッシュ索引の定期同期失敗: " + err.Error())
		}
	}
}
