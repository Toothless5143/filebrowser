package files

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

// setupTrashTestIndex creates a temporary directory and registers it as an index source.
// Returns the source name, the real temp directory path, and a cleanup function.
func setupTrashTestIndex(t *testing.T, sourceName string) (string, string) {
	t.Helper()

	tmpDir := t.TempDir()
	originalCacheDir := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = tmpDir
	t.Cleanup(func() {
		settings.Config.Server.CacheDir = originalCacheDir
	})

	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0644, 0755)
	}

	if indexing.GetIndexDB() == nil {
		db, _, err := dbsql.NewIndexDB(sourceName, "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
		indexing.SetIndexDBForTesting(db)
	}

	realTmpDir := t.TempDir()

	indexing.Initialize(&settings.Source{
		Name: sourceName,
		Path: realTmpDir,
	}, false, false)

	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		t.Fatal("Failed to get test index")
	}

	// Wait for scanner to be ready
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		status := idx.GetScannerStatus()
		if status["status"] == "ready" || status["status"] == "unavailable" {
			break
		}
	}

	return sourceName, realTmpDir
}

func TestMoveToTrash_File(t *testing.T) {
	source, root := setupTrashTestIndex(t, "test_trash_file")

	// Create a test file
	testFile := filepath.Join(root, "testfile.txt")
	if err := os.WriteFile(testFile, []byte("hello trash"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Move to trash
	trashID, err := MoveToTrash(source, testFile, false)
	if err != nil {
		t.Fatalf("MoveToTrash failed: %v", err)
	}

	if trashID == "" {
		t.Error("Expected non-empty trashID")
	}

	// Original file should no longer exist
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Original file should not exist after moving to trash")
	}

	// Trash file should exist
	filesPath := filepath.Join(root, trashDirName, trashFilesDir, trashID)
	if _, err := os.Stat(filesPath); os.IsNotExist(err) {
		t.Errorf("Trash file should exist at %s", filesPath)
	}

	// Trash info file should exist
	infoFile := filepath.Join(root, trashDirName, trashInfoDir, trashID+".json")
	if _, err := os.Stat(infoFile); os.IsNotExist(err) {
		t.Errorf("Trash info file should exist at %s", infoFile)
	}
}

func TestMoveToTrash_Directory(t *testing.T) {
	source, root := setupTrashTestIndex(t, "test_trash_dir")

	// Create a test directory with a file inside
	testDir := filepath.Join(root, "testdir")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "inner.txt"), []byte("inner"), 0644); err != nil {
		t.Fatalf("Failed to create inner file: %v", err)
	}

	trashID, err := MoveToTrash(source, testDir, true)
	if err != nil {
		t.Fatalf("MoveToTrash failed: %v", err)
	}

	if trashID == "" {
		t.Error("Expected non-empty trashID")
	}

	// Original directory should no longer exist
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Error("Original directory should not exist after moving to trash")
	}

	// Trash data directory should exist
	filesPath := filepath.Join(root, trashDirName, trashFilesDir, trashID)
	if fi, err := os.Stat(filesPath); os.IsNotExist(err) {
		t.Errorf("Trash directory should exist at %s", filesPath)
	} else if !fi.IsDir() {
		t.Error("Trash data should be a directory")
	}
}

func TestListTrash(t *testing.T) {
	source, root := setupTrashTestIndex(t, "test_trash_list")

	// Empty trash should return empty list
	items, err := ListTrash(source)
	if err != nil {
		t.Fatalf("ListTrash failed on empty trash: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items in empty trash, got %d", len(items))
	}

	// Add a file to trash
	file1 := filepath.Join(root, "file1.txt")
	if err := os.WriteFile(file1, []byte("file1"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	_, err = MoveToTrash(source, file1, false)
	if err != nil {
		t.Fatalf("MoveToTrash failed: %v", err)
	}

	items, err = ListTrash(source)
	if err != nil {
		t.Fatalf("ListTrash failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("Expected 1 item in trash, got %d", len(items))
	}

	if items[0].Name != "file1.txt" {
		t.Errorf("Expected item name 'file1.txt', got '%s'", items[0].Name)
	}
	if items[0].IsDir {
		t.Error("Item should not be a directory")
	}
	if items[0].Source != source {
		t.Errorf("Expected source '%s', got '%s'", source, items[0].Source)
	}
}

func TestRestoreFromTrash(t *testing.T) {
	source, root := setupTrashTestIndex(t, "test_trash_restore")

	// Create and trash a file
	testFile := filepath.Join(root, "restore_me.txt")
	content := []byte("restore me")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	trashID, err := MoveToTrash(source, testFile, false)
	if err != nil {
		t.Fatalf("MoveToTrash failed: %v", err)
	}

	// Restore the file
	if err := RestoreFromTrash(source, trashID); err != nil {
		t.Fatalf("RestoreFromTrash failed: %v", err)
	}

	// File should be back at original location
	restoredContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Restored file should exist at original location: %v", err)
	}

	if string(restoredContent) != string(content) {
		t.Errorf("Restored file content mismatch: got %q, want %q", restoredContent, content)
	}

	// Trash metadata should be removed
	infoFile := filepath.Join(root, trashDirName, trashInfoDir, trashID+".json")
	if _, err := os.Stat(infoFile); !os.IsNotExist(err) {
		t.Error("Trash info file should have been removed after restore")
	}
}

func TestDeleteFromTrash(t *testing.T) {
	source, root := setupTrashTestIndex(t, "test_trash_delete")

	// Create and trash a file
	testFile := filepath.Join(root, "delete_from_trash.txt")
	if err := os.WriteFile(testFile, []byte("delete me"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	trashID, err := MoveToTrash(source, testFile, false)
	if err != nil {
		t.Fatalf("MoveToTrash failed: %v", err)
	}

	// Permanently delete from trash
	if err := DeleteFromTrash(source, trashID); err != nil {
		t.Fatalf("DeleteFromTrash failed: %v", err)
	}

	// Trash file should no longer exist
	filesPath := filepath.Join(root, trashDirName, trashFilesDir, trashID)
	if _, err := os.Stat(filesPath); !os.IsNotExist(err) {
		t.Error("Trash file should have been permanently deleted")
	}

	// Trash metadata should be removed
	infoFile := filepath.Join(root, trashDirName, trashInfoDir, trashID+".json")
	if _, err := os.Stat(infoFile); !os.IsNotExist(err) {
		t.Error("Trash info file should have been removed")
	}
}

func TestEmptyTrash(t *testing.T) {
	source, root := setupTrashTestIndex(t, "test_trash_empty")

	// Add multiple files to trash
	for i := 0; i < 3; i++ {
		fname := fmt.Sprintf("file%d.txt", i+1)
		f := filepath.Join(root, fname)
		if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		// Small sleep to ensure unique trashIDs (nanosecond timestamps)
		time.Sleep(time.Millisecond)
		_, err := MoveToTrash(source, f, false)
		if err != nil {
			t.Fatalf("MoveToTrash failed: %v", err)
		}
	}

	items, err := ListTrash(source)
	if err != nil {
		t.Fatalf("ListTrash failed: %v", err)
	}
	if len(items) < 1 {
		t.Error("Expected items in trash before emptying")
	}

	// Empty the trash
	if err := EmptyTrash(source); err != nil {
		t.Fatalf("EmptyTrash failed: %v", err)
	}

	// Trash should be empty now
	items, err = ListTrash(source)
	if err != nil {
		t.Fatalf("ListTrash failed after empty: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items after emptying trash, got %d", len(items))
	}
}

func TestMoveToTrash_PreventTrashingTrashDir(t *testing.T) {
	source, root := setupTrashTestIndex(t, "test_trash_prevent_self")

	trashDir := filepath.Join(root, trashDirName)
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		t.Fatalf("Failed to create trash dir: %v", err)
	}

	_, err := MoveToTrash(source, trashDir, true)
	if err == nil {
		t.Error("Expected error when trying to move trash directory to trash")
	}
}
