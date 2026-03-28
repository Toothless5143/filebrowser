package files

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

const (
	trashDirName  = ".trash"
	trashFilesDir = "files"
	trashInfoDir  = "info"
)

// TrashItem represents metadata for a trashed file or directory.
type TrashItem struct {
	TrashID      string    `json:"trashId"`
	OriginalPath string    `json:"originalPath"`
	Name         string    `json:"name"`
	DeletedAt    time.Time `json:"deletedAt"`
	IsDir        bool      `json:"isDir"`
	Size         int64     `json:"size"`
	Source       string    `json:"source"`
}

// trashRoot returns the absolute path of the .trash directory for a source.
func trashRoot(source string) (string, error) {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return "", fmt.Errorf("could not get index for source: %s", source)
	}
	return filepath.Join(idx.Path, trashDirName), nil
}

// trashFilesPath returns the absolute path of the files sub-directory inside trash.
func trashFilesPath(source string) (string, error) {
	root, err := trashRoot(source)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, trashFilesDir), nil
}

// trashInfoPath returns the absolute path of the info sub-directory inside trash.
func trashInfoPath(source string) (string, error) {
	root, err := trashRoot(source)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, trashInfoDir), nil
}

// ensureTrashDirs ensures the trash directory structure exists.
func ensureTrashDirs(source string) error {
	filesPath, err := trashFilesPath(source)
	if err != nil {
		return err
	}
	infoPath, err := trashInfoPath(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filesPath, 0755); err != nil {
		return err
	}
	return os.MkdirAll(infoPath, 0755)
}

// MoveToTrash moves a file or directory into the trash for the given source.
// It returns the generated trashID.
func MoveToTrash(source, absPath string, isDir bool) (string, error) {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return "", fmt.Errorf("could not get index for source: %s", source)
	}

	// Do not allow trashing the source root
	cleanAbs := filepath.Clean(absPath)
	if cleanAbs == filepath.Clean(idx.Path) {
		return "", fmt.Errorf("refusing to trash source root directory: %s", absPath)
	}

	// Do not allow trashing the trash directory itself
	trashRoot := filepath.Join(idx.Path, trashDirName)
	if cleanAbs == filepath.Clean(trashRoot) || isSubPath(trashRoot, cleanAbs) {
		return "", fmt.Errorf("cannot move trash directory to trash")
	}

	if err := ensureTrashDirs(source); err != nil {
		return "", fmt.Errorf("failed to create trash directories: %w", err)
	}

	filesPath, err := trashFilesPath(source)
	if err != nil {
		return "", err
	}
	infoPath, err := trashInfoPath(source)
	if err != nil {
		return "", err
	}

	// Generate a unique trashID based on nanosecond timestamp
	trashID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Determine the size of the item before moving
	size := int64(0)
	if fi, err := os.Stat(absPath); err == nil {
		if isDir {
			size = dirSize(absPath)
		} else {
			size = fi.Size()
		}
	}

	// Destination inside trash
	trashDst := filepath.Join(filesPath, trashID)

	// Move file/directory to trash
	if err := fileutils.MoveFile(absPath, trashDst); err != nil {
		return "", fmt.Errorf("failed to move item to trash: %w", err)
	}

	// Create metadata file
	item := TrashItem{
		TrashID:      trashID,
		OriginalPath: idx.MakeIndexPath(absPath, isDir),
		Name:         filepath.Base(absPath),
		DeletedAt:    time.Now().UTC(),
		IsDir:        isDir,
		Size:         size,
		Source:       source,
	}
	data, err := json.Marshal(item)
	if err != nil {
		// Try to restore the file since writing metadata failed
		_ = fileutils.MoveFile(trashDst, absPath)
		return "", fmt.Errorf("failed to marshal trash metadata: %w", err)
	}

	infoFile := filepath.Join(infoPath, trashID+".json")
	if err := os.WriteFile(infoFile, data, 0644); err != nil {
		// Try to restore the file since writing metadata failed
		_ = fileutils.MoveFile(trashDst, absPath)
		return "", fmt.Errorf("failed to write trash metadata: %w", err)
	}

	// Update source index: remove deleted item and refresh parent
	if !idx.Config.DisableIndexing {
		indexPath := idx.MakeIndexPath(absPath, isDir)
		go func() {
			indexing.RealPathCache.Delete(absPath)
			indexing.IsDirCache.Delete(absPath + ":isdir")
			idx.DeleteMetadata(indexPath, isDir, isDir)
			if err := RefreshIndex(source, filepath.Dir(absPath), true, false); err != nil {
				logger.Errorf("Failed to refresh parent after trash move: %v", err)
			}
		}()
	}

	return trashID, nil
}

// ListTrash returns all items currently in the trash for a source.
func ListTrash(source string) ([]TrashItem, error) {
	infoPath, err := trashInfoPath(source)
	if err != nil {
		return nil, err
	}

	// If trash info directory doesn't exist yet, return empty list
	if _, err := os.Stat(infoPath); os.IsNotExist(err) {
		return []TrashItem{}, nil
	}

	entries, err := os.ReadDir(infoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read trash info directory: %w", err)
	}

	items := make([]TrashItem, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		infoFile := filepath.Join(infoPath, entry.Name())
		data, err := os.ReadFile(infoFile)
		if err != nil {
			logger.Errorf("Failed to read trash info file %s: %v", infoFile, err)
			continue
		}
		var item TrashItem
		if err := json.Unmarshal(data, &item); err != nil {
			logger.Errorf("Failed to parse trash info file %s: %v", infoFile, err)
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

// RestoreFromTrash restores a trashed item to its original path.
func RestoreFromTrash(source, trashID string) error {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return fmt.Errorf("could not get index for source: %s", source)
	}

	filesPath, err := trashFilesPath(source)
	if err != nil {
		return err
	}
	infoPath, err := trashInfoPath(source)
	if err != nil {
		return err
	}

	infoFile := filepath.Join(infoPath, trashID+".json")
	data, err := os.ReadFile(infoFile)
	if err != nil {
		return fmt.Errorf("trash item not found: %w", err)
	}

	var item TrashItem
	if err := json.Unmarshal(data, &item); err != nil {
		return fmt.Errorf("failed to parse trash metadata: %w", err)
	}

	trashSrc := filepath.Join(filesPath, trashID)
	if _, err := os.Stat(trashSrc); os.IsNotExist(err) {
		return fmt.Errorf("trash data file not found for trashID: %s", trashID)
	}

	// Restore to original real path
	originalRealPath := filepath.Join(idx.Path, item.OriginalPath)
	// Remove trailing slash that MakeIndexPath may have added for dirs
	originalRealPath = filepath.Clean(originalRealPath)

	// Ensure parent directory exists
	parentDir := filepath.Dir(originalRealPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory for restore: %w", err)
	}

	// Check for conflicts at the restore destination
	if _, err := os.Stat(originalRealPath); err == nil {
		return fmt.Errorf("a file or directory already exists at the restore destination: %s", originalRealPath)
	}

	// Move the file back to its original location
	if err := fileutils.MoveFile(trashSrc, originalRealPath); err != nil {
		return fmt.Errorf("failed to restore item from trash: %w", err)
	}

	// Remove the metadata file
	if err := os.Remove(infoFile); err != nil {
		logger.Errorf("Failed to remove trash metadata file %s: %v", infoFile, err)
	}

	// Update index
	if !idx.Config.DisableIndexing {
		go func() {
			if err := RefreshIndex(source, originalRealPath, item.IsDir, item.IsDir); err != nil {
				logger.Errorf("Failed to refresh index after trash restore: %v", err)
			}
			if err := RefreshIndex(source, parentDir, true, false); err != nil {
				logger.Errorf("Failed to refresh parent after trash restore: %v", err)
			}
		}()
	}

	return nil
}

// DeleteFromTrash permanently deletes an item from the trash.
func DeleteFromTrash(source, trashID string) error {
	filesPath, err := trashFilesPath(source)
	if err != nil {
		return err
	}
	infoPath, err := trashInfoPath(source)
	if err != nil {
		return err
	}

	trashSrc := filepath.Join(filesPath, trashID)
	infoFile := filepath.Join(infoPath, trashID+".json")

	// Remove the actual file/directory
	if err := os.RemoveAll(trashSrc); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to permanently delete trash item: %w", err)
	}

	// Remove the metadata file
	if err := os.Remove(infoFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove trash metadata: %w", err)
	}

	return nil
}

// EmptyTrash permanently deletes all items in the trash for a source.
func EmptyTrash(source string) error {
	root, err := trashRoot(source)
	if err != nil {
		return err
	}

	filesPath := filepath.Join(root, trashFilesDir)
	infoPath := filepath.Join(root, trashInfoDir)

	// Remove all files in the files directory
	if err := clearDir(filesPath); err != nil {
		return fmt.Errorf("failed to empty trash files: %w", err)
	}

	// Remove all files in the info directory
	if err := clearDir(infoPath); err != nil {
		return fmt.Errorf("failed to empty trash info: %w", err)
	}

	return nil
}

// clearDir removes all entries within a directory without removing the directory itself.
func clearDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

// dirSize returns the total size of all files in a directory tree.
func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total
}

// isSubPath reports whether child is a sub-path of parent.
func isSubPath(parent, child string) bool {
	parent = filepath.Clean(parent) + string(filepath.Separator)
	child = filepath.Clean(child)
	return strings.HasPrefix(child, parent)
}
