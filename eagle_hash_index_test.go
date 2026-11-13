package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

type fakeEagle struct {
	mu       sync.Mutex
	library  string
	items    []eagleListItem
	addCalls int
	broken   bool
	server   *httptest.Server
}

func newFakeEagle(t *testing.T) *fakeEagle {
	t.Helper()
	library := filepath.Join(t.TempDir(), "Test.library")
	if err := os.MkdirAll(filepath.Join(library, "images"), 0700); err != nil {
		t.Fatal(err)
	}
	fake := &fakeEagle{library: library}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serveHTTP))
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *fakeEagle) addOriginal(t *testing.T, id, name string, content []byte) {
	t.Helper()
	fake.mu.Lock()
	defer fake.mu.Unlock()
	itemDir := filepath.Join(fake.library, "images", id+".info")
	if err := os.MkdirAll(itemDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, name), content, 0600); err != nil {
		t.Fatal(err)
	}
	fake.items = append(fake.items, eagleListItem{ID: id, Name: name[:len(name)-len(filepath.Ext(name))], Ext: filepath.Ext(name)[1:]})
}

func (fake *fakeEagle) serveHTTP(w http.ResponseWriter, r *http.Request) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if fake.broken && r.URL.Path == "/api/v2/item/get" {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	switch r.URL.Path {
	case "/api/v2/library/info":
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"path": fake.library}})
	case "/api/v2/item/get":
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 50
		}
		end := offset + limit
		if end > len(fake.items) {
			end = len(fake.items)
		}
		var page []eagleListItem
		if offset < len(fake.items) {
			page = append(page, fake.items[offset:end]...)
		} else {
			page = []eagleListItem{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"data": page, "total": len(fake.items)}})
	case "/api/v2/item/add":
		var request eagleAddRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fake.addCalls++
		id := fmt.Sprintf("NEW%04d", fake.addCalls)
		original, err := os.ReadFile(request.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		name := filepath.Base(request.Path)
		itemDir := filepath.Join(fake.library, "images", id+".info")
		if err := os.MkdirAll(itemDir, 0700); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(filepath.Join(itemDir, name), original, 0600); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fake.items = append(fake.items, eagleListItem{ID: id, Name: request.Name, Ext: filepath.Ext(name)[1:]})
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"id": id}})
	default:
		http.NotFound(w, r)
	}
}

func newTestIndex(t *testing.T, fake *fakeEagle) *eagleHashIndex {
	t.Helper()
	idx, err := openEagleHashIndex(filepath.Join(t.TempDir(), "index.sqlite"), fake.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = idx.db.Close() })
	appConfig = getDefaultConfig()
	appConfig.Eagle.BaseURL = fake.server.URL
	return idx
}

func writeTestImage(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEagleHashIndexSkipsImportedImagesAcrossPaths(t *testing.T) {
	fake := newFakeEagle(t)
	fake.addOriginal(t, "OLD1", "original.png", []byte("same bytes"))
	idx := newTestIndex(t, fake)

	copyPath := writeTestImage(t, "renamed.png", []byte("same bytes"))
	result, err := idx.Import(PhotoRecord{SourcePath: copyPath, SourceType: SourceTypePhoto})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Duplicate || result.ItemID != "OLD1" || fake.addCalls != 0 {
		t.Fatalf("expected duplicate without Eagle API add: %+v, adds=%d", result, fake.addCalls)
	}
	if !isTerminalSuccess(processingStatusDuplicate) {
		t.Fatal("duplicate status must be considered terminal")
	}
}

func TestEagleHashIndexImportsNewImagesOnce(t *testing.T) {
	fake := newFakeEagle(t)
	idx := newTestIndex(t, fake)
	one := writeTestImage(t, "first.png", []byte("new image"))
	two := writeTestImage(t, "second.png", []byte("new image"))
	var wg sync.WaitGroup
	results := make([]eagleImportResult, 2)
	errs := make([]error, 2)
	for i, path := range []string{one, two} {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			results[i], errs[i] = idx.Import(PhotoRecord{SourcePath: path, SourceType: SourceTypePhoto})
		}(i, path)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || fake.addCalls != 1 {
		t.Fatalf("concurrent import: results=%+v errors=%v adds=%d", results, errs, fake.addCalls)
	}
	if results[0].Duplicate == results[1].Duplicate {
		t.Fatalf("one image should be imported, the other skipped: %+v", results)
	}
}

func TestEagleHashIndexDoesNotConflateSameNameOrSize(t *testing.T) {
	fake := newFakeEagle(t)
	fake.addOriginal(t, "OLD1", "same.png", []byte("AAAA"))
	idx := newTestIndex(t, fake)
	other := writeTestImage(t, "same.png", []byte("BBBB"))
	result, err := idx.Import(PhotoRecord{SourcePath: other, SourceType: SourceTypePhoto})
	if err != nil || result.Duplicate || fake.addCalls != 1 {
		t.Fatalf("different contents must import: %+v, %v, adds=%d", result, err, fake.addCalls)
	}
}

func TestEagleHashIndexReconcilesDeletions(t *testing.T) {
	fake := newFakeEagle(t)
	fake.addOriginal(t, "OLD1", "original.png", []byte("same bytes"))
	idx := newTestIndex(t, fake)
	if err := idx.Sync(false); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	fake.items = nil
	fake.mu.Unlock()
	if err := idx.Sync(true); err != nil {
		t.Fatal(err)
	}
	copyPath := writeTestImage(t, "new.png", []byte("same bytes"))
	result, err := idx.Import(PhotoRecord{SourcePath: copyPath, SourceType: SourceTypePhoto})
	if err != nil || result.Duplicate || fake.addCalls != 1 {
		t.Fatalf("deleted item should no longer block import: %+v, %v, adds=%d", result, err, fake.addCalls)
	}
}

func TestEagleHashIndexFailsClosedWhenRefreshFails(t *testing.T) {
	fake := newFakeEagle(t)
	fake.broken = true
	idx := newTestIndex(t, fake)
	path := writeTestImage(t, "new.png", []byte("new image"))
	if _, err := idx.Import(PhotoRecord{SourcePath: path}); err == nil {
		t.Fatal("expected error on index refresh failure")
	}
	if fake.addCalls != 0 {
		t.Fatal("Eagle add must never happen without a trusted index")
	}
}
