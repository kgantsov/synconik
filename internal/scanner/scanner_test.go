package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/kgantsov/synconik/internal/uploader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// stubScanStorage makes GetStorage return a storage with scanning and read
// access enabled so NewScanner's startup settings load is satisfied.
func stubScanStorage(mockClient *client.MockClient) {
	mockClient.On("GetStorage", mock.Anything, mock.Anything).Return(
		&client.Storage{Settings: map[string]interface{}{"scan": true, "read": true}}, nil,
	)
}

// newStabilityScanner builds a scanner rooted at a fresh temp dir with the given
// stability window, returning the scanner, the scan root, its upload queue, and a
// cleanup func. The mock client accepts collection creation for the root dir.
func newStabilityScanner(t *testing.T, stabilityWindow int32) (*Scanner, string, chan uploader.Job, func()) {
	tmpDir, err := os.MkdirTemp("", "scanner-stability-*")
	assert.NoError(t, err)
	tmpDbDir, err := os.MkdirTemp("", "scanner-stability-db-*")
	assert.NoError(t, err)

	testDir := filepath.Join(tmpDir, "testdir")
	err = os.MkdirAll(testDir, 0755)
	assert.NoError(t, err)

	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Dir:             testDir,
			Interval:        10,
			StabilityWindow: stabilityWindow,
		},
	}

	st, err := store.NewBadgerStore(tmpDbDir)
	assert.NoError(t, err)

	uploadQueue := make(chan uploader.Job, 100)
	mockClient := client.NewMockClient()
	mockClient.On("CreateCollection", mock.Anything, mock.Anything).Return(
		&client.Collection{ID: "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F", Title: "testdir"}, nil,
	)
	stubScanStorage(mockClient)

	scanner, err := NewScanner(cfg, st, mockClient, uploadQueue)
	assert.NoError(t, err)

	cleanup := func() {
		scanner.Stop()
		os.RemoveAll(tmpDir)
		os.RemoveAll(tmpDbDir)
	}

	return scanner, testDir, uploadQueue, cleanup
}

func setupTestScanner(t *testing.T) (*Scanner, string, func()) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	assert.NoError(t, err)
	tmpDbDir, err := os.MkdirTemp("", "scanner-test-db-*")
	assert.NoError(t, err)

	testDir := filepath.Join(tmpDir, "testdir")
	err = os.MkdirAll(testDir, 0755)
	assert.NoError(t, err)

	// Create test config
	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Dir:      testDir,
			Interval: 10,
		},
	}

	store, err := store.NewBadgerStore(tmpDbDir)
	assert.NoError(t, err)

	// Create test scanner
	uploadQueue := make(chan uploader.Job, 100)
	mockClient := client.NewMockClient()

	mockClient.On(
		"CreateCollection",
		mock.Anything,
		&client.Collection{ID: "", Title: "testdir", ParentID: "", StorageID: ""},
	).Return(&client.Collection{ID: "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F", Title: "testdir", ParentID: "", StorageID: ""}, nil)
	stubScanStorage(mockClient)

	scanner, err := NewScanner(cfg, store, mockClient, uploadQueue)
	assert.NoError(t, err)

	cleanup := func() {
		scanner.Stop()
		os.RemoveAll(tmpDir)
		os.RemoveAll(tmpDbDir)
	}

	return scanner, tmpDir, cleanup
}

func TestScanner_StartStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	assert.NoError(t, err)
	tmpDbDir, err := os.MkdirTemp("", "scanner-test-db-*")
	assert.NoError(t, err)

	testDir := filepath.Join(tmpDir, "testdir")
	err = os.MkdirAll(testDir, 0755)
	assert.NoError(t, err)

	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Dir:      testDir,
			Interval: 10,
		},
	}

	store, err := store.NewBadgerStore(tmpDbDir)
	assert.NoError(t, err)

	// Create test scanner
	uploadQueue := make(chan uploader.Job, 100)
	mockClient := client.NewMockClient()

	mockClient.On(
		"CreateCollection",
		mock.Anything,
		&client.Collection{ID: "", Title: "testdir", ParentID: "", StorageID: ""},
	).Return(&client.Collection{ID: "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F", Title: "testdir", ParentID: "", StorageID: ""}, nil)
	stubScanStorage(mockClient)

	scanner, err := NewScanner(cfg, store, mockClient, uploadQueue)
	assert.NoError(t, err)

	// Test Start
	scanner.Start()
	assert.NotNil(t, scanner.done)

	// Test Stop
	scanner.Stop()
	_, ok := <-scanner.done
	assert.False(t, ok, "done channel should be closed")
}

// drainQueue empties the upload queue, signalling WG.Done for each job so that a
// later scanner.Stop() (which waits on the WaitGroup) does not block.
func drainQueue(queue chan uploader.Job) []uploader.Job {
	var jobs []uploader.Job
	for {
		select {
		case job := <-queue:
			job.Payload.WG.Done()
			jobs = append(jobs, job)
		default:
			return jobs
		}
	}
}

func writeFile(t *testing.T, dir, name, content string, mtime time.Time) (relPath string, size int64) {
	fullPath := filepath.Join(dir, name)
	err := os.WriteFile(fullPath, []byte(content), 0644)
	assert.NoError(t, err)
	err = os.Chtimes(fullPath, mtime, mtime)
	assert.NoError(t, err)

	info, err := os.Stat(fullPath)
	assert.NoError(t, err)

	// Scanner keys are the path with the scan-root prefix trimmed. Dir has no
	// trailing slash here, so the key retains a leading separator.
	return fullPath[len(dir):], info.Size()
}

// A file seen for the first time has not yet been stable for the window, so it
// must not be enqueued.
func TestScanner_StabilityWindow_DefersNewFile(t *testing.T) {
	scanner, dir, queue, cleanup := newStabilityScanner(t, 30)
	defer cleanup()

	relPath, _ := writeFile(t, dir, "movie.mov", "partial", time.Now())

	scanner.Scan()

	assert.Len(t, drainQueue(queue), 0, "freshly seen file should be deferred")
	_, tracked := scanner.stability[relPath]
	assert.True(t, tracked, "file should be tracked for the next scan")
}

// A file whose (size, mtime) has been unchanged since before the window elapsed
// is considered settled and gets enqueued.
func TestScanner_StabilityWindow_EnqueuesStableFile(t *testing.T) {
	scanner, dir, queue, cleanup := newStabilityScanner(t, 30)
	defer cleanup()

	mtime := time.Now().Add(-time.Hour)
	relPath, size := writeFile(t, dir, "movie.mov", "complete", mtime)

	// Simulate having first observed this exact size+mtime well before the window.
	scanner.stability[relPath] = fileState{
		size:      size,
		mtime:     mtime,
		firstSeen: time.Now().Add(-time.Minute),
	}

	scanner.Scan()

	jobs := drainQueue(queue)
	assert.Len(t, jobs, 1, "settled file should be enqueued")
	assert.Equal(t, relPath, jobs[0].Payload.Path)
}

// A file whose size changed since the last scan (i.e. still growing) has its
// stability clock reset and must not be enqueued.
func TestScanner_StabilityWindow_DefersGrowingFile(t *testing.T) {
	scanner, dir, queue, cleanup := newStabilityScanner(t, 30)
	defer cleanup()

	mtime := time.Now().Add(-time.Hour)
	relPath, size := writeFile(t, dir, "movie.mov", "grown-bigger", mtime)

	// Pretend we previously saw an old first-seen time but a smaller size.
	scanner.stability[relPath] = fileState{
		size:      size - 1,
		mtime:     mtime,
		firstSeen: time.Now().Add(-time.Minute),
	}

	scanner.Scan()

	assert.Len(t, drainQueue(queue), 0, "growing file should reset and be deferred")
	assert.Equal(t, time.Now().Truncate(time.Second),
		scanner.stability[relPath].firstSeen.Truncate(time.Second),
		"firstSeen should reset to now when the file changes")
}

// With the window disabled (0), files are enqueued immediately as before.
func TestScanner_StabilityWindow_Disabled(t *testing.T) {
	scanner, dir, queue, cleanup := newStabilityScanner(t, 0)
	defer cleanup()

	writeFile(t, dir, "movie.mov", "whatever", time.Now())

	scanner.Scan()

	assert.Len(t, drainQueue(queue), 1, "disabled window should enqueue immediately")
}

// Files whose base name matches a scan_ignore glob are skipped, while others are
// still enqueued.
func TestScanner_ScanIgnore_SkipsMatchingFiles(t *testing.T) {
	scanner, dir, queue, cleanup := newStabilityScanner(t, 0)
	defer cleanup()

	scanner.scanIgnore = []string{"*.arw", "*.tmp"}

	mtime := time.Now().Add(-time.Hour)
	writeFile(t, dir, "photo.arw", "raw", mtime)
	writeFile(t, dir, "scratch.tmp", "temp", mtime)
	keepPath, _ := writeFile(t, dir, "movie.mov", "keep", mtime)

	scanner.Scan()

	jobs := drainQueue(queue)
	assert.Len(t, jobs, 1, "ignored files should not be enqueued")
	assert.Equal(t, keepPath, jobs[0].Payload.Path)
}

// A directory matching a scan_ignore pattern is skipped along with its subtree.
func TestScanner_ScanIgnore_SkipsMatchingDir(t *testing.T) {
	scanner, dir, queue, cleanup := newStabilityScanner(t, 0)
	defer cleanup()

	scanner.scanIgnore = []string{"media cache"}

	ignored := filepath.Join(dir, "media cache")
	assert.NoError(t, os.MkdirAll(ignored, 0755))
	writeFile(t, ignored, "inside.mov", "nested", time.Now().Add(-time.Hour))
	keepPath, _ := writeFile(t, dir, "keep.mov", "keep", time.Now().Add(-time.Hour))

	scanner.Scan()

	jobs := drainQueue(queue)
	assert.Len(t, jobs, 1, "files under an ignored dir should be skipped")
	assert.Equal(t, keepPath, jobs[0].Payload.Path)
}

// When the storage's scan flag is off, nothing is enqueued.
func TestScanner_ScanDisabled_SkipsScan(t *testing.T) {
	scanner, dir, queue, cleanup := newStabilityScanner(t, 0)
	defer cleanup()

	scanner.scanEnabled = false
	writeFile(t, dir, "movie.mov", "whatever", time.Now().Add(-time.Hour))

	scanner.Scan()

	assert.Len(t, drainQueue(queue), 0, "scan-disabled storage should enqueue nothing")
}

// When the storage's read flag is off, nothing is enqueued.
func TestScanner_ReadDisabled_SkipsScan(t *testing.T) {
	scanner, dir, queue, cleanup := newStabilityScanner(t, 0)
	defer cleanup()

	scanner.readEnabled = false
	writeFile(t, dir, "movie.mov", "whatever", time.Now().Add(-time.Hour))

	scanner.Scan()

	assert.Len(t, drainQueue(queue), 0, "read-disabled storage should enqueue nothing")
}

func TestMatchesIgnore(t *testing.T) {
	patterns := []string{"*.arw", "media cache", "cache/*"}

	assert.True(t, matchesIgnore(patterns, "photos/a.arw", "a.arw"))
	assert.True(t, matchesIgnore(patterns, "media cache", "media cache"))
	assert.True(t, matchesIgnore(patterns, "cache/thumb.jpg", "thumb.jpg"))
	assert.False(t, matchesIgnore(patterns, "photos/a.jpg", "a.jpg"))
	assert.False(t, matchesIgnore(nil, "photos/a.arw", "a.arw"))
}
