package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// cmdStatsExport emits an anonymized JSONL stream of profile activity
// suitable for opt-in research sharing. Stays local-only by default —
// the user controls whether to share the resulting file. Strips UUIDs
// (replaced with deterministic short hashes), excludes code content, and
// excludes LLM responses to keep the export privacy-safe.
//
//	temper stats export                      # writes to stdout
//	temper stats export -out usage.jsonl     # writes to file
//	temper stats export -since 2026-01-01    # only attempts after date
//	temper stats export -salt my-cohort-id   # change anonymization salt
func cmdStatsExport(args []string) error {
	fs := flag.NewFlagSet("stats export", flag.ContinueOnError)
	out := fs.String("out", "", "output file (default: stdout)")
	since := fs.String("since", "", "only export attempts after this date (YYYY-MM-DD)")
	salt := fs.String("salt", "temper-default-cohort", "anonymization salt for hashed IDs")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	resp, err := daemonGet(daemonAddr + "/v1/profile")
	if err != nil {
		return fmt.Errorf("get profile: %w", err)
	}
	defer resp.Body.Close()

	if err := authError(resp); err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("get profile: status=%d body=%s", resp.StatusCode, string(body))
	}

	var profile struct {
		ID                  string `json:"id"`
		TopicSkills         map[string]struct {
			Level    float64   `json:"level"`
			Attempts int       `json:"attempts"`
			LastSeen time.Time `json:"last_seen"`
		} `json:"topic_skills"`
		TotalSessions       int            `json:"total_sessions"`
		TotalRuns           int            `json:"total_runs"`
		HintRequests        int            `json:"hint_requests"`
		AvgTimeToGreenMs    int64          `json:"avg_time_to_green_ms"`
		ErrorPatterns       map[string]int `json:"error_patterns"`
		ExerciseHistory     []struct {
			ExerciseID       string     `json:"exercise_id"`
			SessionID        string     `json:"session_id"`
			StartedAt        time.Time  `json:"started_at"`
			CompletedAt      *time.Time `json:"completed_at,omitempty"`
			RunCount         int        `json:"run_count"`
			HintCount        int        `json:"hint_count"`
			TimeToCompleteMs int64      `json:"time_to_complete_ms,omitempty"`
			Success          bool       `json:"success"`
		} `json:"exercise_history"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return fmt.Errorf("parse profile: %w", err)
	}

	var sinceTime time.Time
	if *since != "" {
		t, err := time.Parse("2006-01-02", *since)
		if err != nil {
			return fmt.Errorf("parse -since: %w", err)
		}
		sinceTime = t
	}

	w := os.Stdout
	if *out != "" {
		f, err := os.OpenFile(*out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("open %s: %w", *out, err)
		}
		defer f.Close()
		w = f
	}

	enc := json.NewEncoder(w)

	if err := enc.Encode(map[string]any{
		"event":               "summary",
		"profile_id":          anonymize(profile.ID, *salt),
		"total_sessions":      profile.TotalSessions,
		"total_runs":          profile.TotalRuns,
		"hint_requests":       profile.HintRequests,
		"avg_time_to_green_ms": profile.AvgTimeToGreenMs,
		"topic_skills":        profile.TopicSkills,
		"error_patterns":      profile.ErrorPatterns,
		"exported_at":         time.Now().UTC(),
	}); err != nil {
		return err
	}

	wrote := 0
	for _, attempt := range profile.ExerciseHistory {
		if !sinceTime.IsZero() && attempt.StartedAt.Before(sinceTime) {
			continue
		}
		if err := enc.Encode(map[string]any{
			"event":               "attempt",
			"session":             anonymize(attempt.SessionID, *salt),
			"exercise_id":         attempt.ExerciseID,
			"topic":               topicFromExerciseID(attempt.ExerciseID),
			"started_at":          attempt.StartedAt,
			"completed_at":        attempt.CompletedAt,
			"run_count":           attempt.RunCount,
			"hint_count":          attempt.HintCount,
			"time_to_complete_ms": attempt.TimeToCompleteMs,
			"success":             attempt.Success,
		}); err != nil {
			return err
		}
		wrote++
	}

	if *out != "" {
		fmt.Fprintf(os.Stderr, "exported %d attempts + 1 summary line to %s\n", wrote, *out)
	}
	return nil
}

// anonymize hashes an ID with a per-cohort salt and returns the first
// 12 hex chars. Deterministic per (id, salt) so attempts can be grouped
// without revealing the underlying UUID.
func anonymize(id, salt string) string {
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(salt + ":" + id))
	return hex.EncodeToString(sum[:6])
}

// topicFromExerciseID mirrors profile.ExtractTopic without requiring the
// internal package import. "go-v1/basics/hello-world" → "go/basics".
func topicFromExerciseID(id string) string {
	parts := strings.Split(id, "/")
	if len(parts) < 2 {
		return "general"
	}
	pack := parts[0]
	if idx := strings.Index(pack, "-"); idx > 0 {
		pack = pack[:idx]
	}
	if len(parts) >= 3 {
		return pack + "/" + parts[1]
	}
	return pack
}
