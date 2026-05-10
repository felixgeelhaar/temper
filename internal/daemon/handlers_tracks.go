package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/felixgeelhaar/temper/internal/domain"
	sqlitestore "github.com/felixgeelhaar/temper/internal/storage/sqlite"
)

// Track handlers (Learning Contract Presets)

func (s *Server) handleListTracks(w http.ResponseWriter, r *http.Request) {
	if s.trackStore == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "tracks require sqlite storage", nil)
		return
	}

	tracks, err := s.trackStore.List()
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to list tracks", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"tracks": tracks,
		"count":  len(tracks),
	})
}

func (s *Server) handleGetTrack(w http.ResponseWriter, r *http.Request) {
	if s.trackStore == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "tracks require sqlite storage", nil)
		return
	}

	id := r.PathValue("id")
	track, err := s.trackStore.Get(id)
	if err != nil {
		if err == sqlitestore.ErrTrackNotFound {
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeTrackNotFound, "track not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get track", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, track)
}

func (s *Server) handleCreateTrack(w http.ResponseWriter, r *http.Request) {
	if s.trackStore == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "tracks require sqlite storage", nil)
		return
	}

	var track domain.Track
	if err := json.NewDecoder(r.Body).Decode(&track); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := track.Validate(); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid track", err)
		return
	}

	// Check for duplicate ID
	if s.trackStore.Exists(track.ID) {
		s.jsonError(w, http.StatusConflict, "track with this ID already exists", nil)
		return
	}

	if track.Preset == "" {
		track.Preset = "custom"
	}

	if err := s.trackStore.Save(&track); err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to create track", err)
		return
	}

	s.jsonResponse(w, http.StatusCreated, track)
}

func (s *Server) handleUpdateTrack(w http.ResponseWriter, r *http.Request) {
	if s.trackStore == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "tracks require sqlite storage", nil)
		return
	}

	id := r.PathValue("id")

	// Verify track exists
	existing, err := s.trackStore.Get(id)
	if err != nil {
		if err == sqlitestore.ErrTrackNotFound {
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeTrackNotFound, "track not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get track", err)
		return
	}

	var update domain.Track
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Preserve ID and created_at
	update.ID = id
	update.CreatedAt = existing.CreatedAt

	if err := update.Validate(); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid track", err)
		return
	}

	if err := s.trackStore.Save(&update); err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to update track", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, update)
}

func (s *Server) handleDeleteTrack(w http.ResponseWriter, r *http.Request) {
	if s.trackStore == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "tracks require sqlite storage", nil)
		return
	}

	id := r.PathValue("id")

	if err := s.trackStore.Delete(id); err != nil {
		if err == sqlitestore.ErrTrackNotFound {
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeTrackNotFound, "track not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to delete track", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"deleted": true,
	})
}

func (s *Server) handleExportTrack(w http.ResponseWriter, r *http.Request) {
	if s.trackStore == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "tracks require sqlite storage", nil)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	track, err := s.trackStore.Get(req.ID)
	if err != nil {
		if err == sqlitestore.ErrTrackNotFound {
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeTrackNotFound, "track not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get track", err)
		return
	}

	yamlData, err := track.MarshalYAML()
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to export track", err)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.yaml", track.ID))
	w.WriteHeader(http.StatusOK)
	w.Write(yamlData)
}

func (s *Server) handleImportTrack(w http.ResponseWriter, r *http.Request) {
	if s.trackStore == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "tracks require sqlite storage", nil)
		return
	}

	// Read YAML body (limit to 64KB)
	body := http.MaxBytesReader(w, r.Body, 64*1024)
	data, err := io.ReadAll(body)
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, "failed to read request body", err)
		return
	}

	track, err := domain.UnmarshalTrackYAML(data)
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid track YAML", err)
		return
	}

	// Save imported track
	if err := s.trackStore.Save(track); err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to import track", err)
		return
	}

	s.jsonResponse(w, http.StatusCreated, track)
}
