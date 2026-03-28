package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
)

// trashListHandler lists items in the trash for a given source.
// @Summary List trash items
// @Description Returns all items currently in the trash for the specified source.
// @Tags Trash
// @Produce json
// @Param source query string true "Source name"
// @Success 200 {array} files.TrashItem "List of trash items"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/trash [get]
func trashListHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Delete {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to access trash")
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		return http.StatusBadRequest, fmt.Errorf("source parameter is required")
	}

	items, err := files.ListTrash(source)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, items)
}

// TrashRestoreRequest represents a request to restore items from trash.
type TrashRestoreRequest struct {
	Source   string   `json:"source"`
	TrashIDs []string `json:"trashIds"`
}

// TrashRestoreResult represents the result of a trash restore operation.
type TrashRestoreResult struct {
	Succeeded []string           `json:"succeeded"`
	Failed    []TrashItemFailure `json:"failed"`
}

// TrashItemFailure represents a failed trash operation on a specific item.
type TrashItemFailure struct {
	TrashID string `json:"trashId"`
	Message string `json:"message"`
}

// trashRestoreHandler restores items from the trash.
// @Summary Restore trash items
// @Description Restores one or more items from the trash to their original locations.
// @Tags Trash
// @Accept json
// @Produce json
// @Param body body TrashRestoreRequest true "Restore request"
// @Success 200 {object} TrashRestoreResult "All items restored"
// @Success 207 {object} TrashRestoreResult "Partial success"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/trash/restore [post]
func trashRestoreHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Delete {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to access trash")
	}

	var req TrashRestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
	}

	if req.Source == "" {
		return http.StatusBadRequest, fmt.Errorf("source is required")
	}

	if len(req.TrashIDs) == 0 {
		return http.StatusBadRequest, fmt.Errorf("trashIds array cannot be empty")
	}

	result := TrashRestoreResult{
		Succeeded: make([]string, 0),
		Failed:    make([]TrashItemFailure, 0),
	}

	for _, trashID := range req.TrashIDs {
		if err := files.RestoreFromTrash(req.Source, trashID); err != nil {
			result.Failed = append(result.Failed, TrashItemFailure{
				TrashID: trashID,
				Message: err.Error(),
			})
		} else {
			result.Succeeded = append(result.Succeeded, trashID)
		}
	}

	statusCode := http.StatusOK
	if len(result.Failed) > 0 && len(result.Succeeded) == 0 {
		statusCode = http.StatusInternalServerError
	} else if len(result.Failed) > 0 {
		statusCode = http.StatusMultiStatus
	}

	return renderJSON(w, r, result, statusCode)
}

// TrashDeleteRequest represents a request to permanently delete items from trash.
type TrashDeleteRequest struct {
	Source   string   `json:"source"`
	TrashIDs []string `json:"trashIds"`
}

// trashDeleteHandler permanently deletes items from the trash.
// @Summary Delete trash items permanently
// @Description Permanently deletes one or more items from the trash.
// @Tags Trash
// @Accept json
// @Produce json
// @Param body body TrashDeleteRequest true "Delete request"
// @Success 200 {object} TrashRestoreResult "All items deleted"
// @Success 207 {object} TrashRestoreResult "Partial success"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/trash [delete]
func trashDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Delete {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to access trash")
	}

	var req TrashDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
	}

	if req.Source == "" {
		return http.StatusBadRequest, fmt.Errorf("source is required")
	}

	if len(req.TrashIDs) == 0 {
		return http.StatusBadRequest, fmt.Errorf("trashIds array cannot be empty")
	}

	result := TrashRestoreResult{
		Succeeded: make([]string, 0),
		Failed:    make([]TrashItemFailure, 0),
	}

	for _, trashID := range req.TrashIDs {
		if err := files.DeleteFromTrash(req.Source, trashID); err != nil {
			result.Failed = append(result.Failed, TrashItemFailure{
				TrashID: trashID,
				Message: err.Error(),
			})
		} else {
			result.Succeeded = append(result.Succeeded, trashID)
		}
	}

	statusCode := http.StatusOK
	if len(result.Failed) > 0 && len(result.Succeeded) == 0 {
		statusCode = http.StatusInternalServerError
	} else if len(result.Failed) > 0 {
		statusCode = http.StatusMultiStatus
	}

	return renderJSON(w, r, result, statusCode)
}

// trashEmptyHandler empties the entire trash for a given source.
// @Summary Empty trash
// @Description Permanently deletes all items in the trash for the specified source.
// @Tags Trash
// @Produce json
// @Param source query string true "Source name"
// @Success 200 "Trash emptied successfully"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/trash/empty [delete]
func trashEmptyHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Delete {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to access trash")
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		return http.StatusBadRequest, fmt.Errorf("source parameter is required")
	}

	if err := files.EmptyTrash(source); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}
