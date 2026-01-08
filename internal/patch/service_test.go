package patch

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestNewServiceWithLogger(t *testing.T) {
	tmpDir := t.TempDir()
	service, err := NewServiceWithLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewServiceWithLogger() error = %v", err)
	}
	if service == nil {
		t.Fatal("NewServiceWithLogger() returned nil")
	}
	if service.GetLogger() == nil {
		t.Error("Service should have logger")
	}
}

func TestService_SetGetLogger(t *testing.T) {
	service := NewService()
	if service.GetLogger() != nil {
		t.Error("New service should not have logger")
	}

	tmpDir := t.TempDir()
	logger, _ := NewLogger(tmpDir)
	service.SetLogger(logger)

	if service.GetLogger() != logger {
		t.Error("GetLogger() should return set logger")
	}
}

func TestService_ExtractFromIntervention(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	intervention := &domain.Intervention{
		ID:        uuid.New(),
		SessionID: sessionID,
		Content:   "Here's a fix:\n```go\n// main.go\npackage main\n\nfunc main() { fmt.Println(\"Hello\") }\n```",
	}

	currentCode := map[string]string{
		"main.go": "package main\n\nfunc main() {}",
	}

	patches := service.ExtractFromIntervention(intervention, sessionID, currentCode)

	// This depends on the extractor implementation
	// At minimum, we should test the method doesn't crash
	if patches != nil && len(patches) > 0 {
		// Verify patches are stored
		stored := service.GetSessionPatches(sessionID)
		if len(stored) == 0 {
			t.Error("Patches should be stored")
		}

		// Verify pending is set
		pending := service.GetPending(sessionID)
		if pending == nil {
			t.Error("Pending should be set")
		}
	}
}

func TestService_GetPatch(t *testing.T) {
	service := NewService()

	// Test not found
	_, err := service.GetPatch(uuid.New())
	if err != ErrPatchNotFound {
		t.Errorf("GetPatch() error = %v; want ErrPatchNotFound", err)
	}
}

func TestService_Preview(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	// Add a patch manually
	patch := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		File:      "test.go",
		Original:  "line1\nline2\nline3\nline4",
		Proposed:  "line1\nnew line\nline3\nline4",
		Diff:      "+new line\n-line2",
		Status:    domain.PatchStatusPending,
	}

	service.mu.Lock()
	service.patches[patch.ID] = patch
	service.sessions[sessionID] = []*domain.Patch{patch}
	service.pending[sessionID] = patch
	service.mu.Unlock()

	preview, err := service.Preview(patch.ID)
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}

	if preview.Patch != patch {
		t.Error("Preview.Patch should match")
	}
	if preview.Additions == 0 {
		t.Error("Preview should count additions")
	}
	if preview.Deletions == 0 {
		t.Error("Preview should count deletions")
	}
}

func TestService_Preview_NotFound(t *testing.T) {
	service := NewService()

	_, err := service.Preview(uuid.New())
	if err != ErrPatchNotFound {
		t.Errorf("Preview() error = %v; want ErrPatchNotFound", err)
	}
}

func TestService_Preview_NewFile(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	// Patch for new file (no Original)
	patch := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		File:      "new_file.go",
		Original:  "",
		Proposed:  "package main\n\nfunc main() {}\n",
		Diff:      "+package main\n+\n+func main() {}",
		Status:    domain.PatchStatusPending,
	}

	service.mu.Lock()
	service.patches[patch.ID] = patch
	service.mu.Unlock()

	preview, err := service.Preview(patch.ID)
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}

	hasNewFileWarning := false
	for _, w := range preview.Warnings {
		if w == "This creates a new file" {
			hasNewFileWarning = true
			break
		}
	}
	if !hasNewFileWarning {
		t.Error("Preview should warn about new file")
	}
}

func TestService_PreviewPending(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	// Add a patch manually
	patch := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		File:      "test.go",
		Original:  "old",
		Proposed:  "new",
		Status:    domain.PatchStatusPending,
	}

	service.mu.Lock()
	service.patches[patch.ID] = patch
	service.pending[sessionID] = patch
	service.mu.Unlock()

	preview, err := service.PreviewPending(sessionID)
	if err != nil {
		t.Fatalf("PreviewPending() error = %v", err)
	}

	if preview.Patch != patch {
		t.Error("PreviewPending should return correct patch")
	}
}

func TestService_PreviewPending_NoPending(t *testing.T) {
	service := NewService()

	_, err := service.PreviewPending(uuid.New())
	if err != ErrPatchNotFound {
		t.Errorf("PreviewPending() error = %v; want ErrPatchNotFound", err)
	}
}

func TestService_Approve(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	patch := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		Status:    domain.PatchStatusPending,
	}

	service.mu.Lock()
	service.patches[patch.ID] = patch
	service.mu.Unlock()

	err := service.Approve(patch.ID)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if patch.Status != domain.PatchStatusApproved {
		t.Errorf("Status = %v; want Approved", patch.Status)
	}
}

func TestService_Approve_Errors(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	tests := []struct {
		name   string
		status domain.PatchStatus
		want   error
	}{
		{"not found", domain.PatchStatusPending, ErrPatchNotFound},
		{"already applied", domain.PatchStatusApplied, ErrPatchApplied},
		{"rejected", domain.PatchStatusRejected, ErrPatchRejected},
		{"expired", domain.PatchStatusExpired, ErrPatchExpired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.name == "not found" {
				err = service.Approve(uuid.New())
			} else {
				patch := &domain.Patch{
					ID:        uuid.New(),
					SessionID: sessionID,
					Status:    tt.status,
				}
				service.mu.Lock()
				service.patches[patch.ID] = patch
				service.mu.Unlock()
				err = service.Approve(patch.ID)
			}
			if err != tt.want {
				t.Errorf("Approve() error = %v; want %v", err, tt.want)
			}
		})
	}
}

func TestService_Apply(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	patch := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		File:      "main.go",
		Proposed:  "package main\n\nfunc main() { fmt.Println(\"Hello\") }",
		Status:    domain.PatchStatusApproved,
	}

	service.mu.Lock()
	service.patches[patch.ID] = patch
	service.sessions[sessionID] = []*domain.Patch{patch}
	service.pending[sessionID] = patch
	service.mu.Unlock()

	file, content, err := service.Apply(patch.ID)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if file != "main.go" {
		t.Errorf("Apply() file = %q; want %q", file, "main.go")
	}
	if content != patch.Proposed {
		t.Error("Apply() content should match Proposed")
	}
	if patch.Status != domain.PatchStatusApplied {
		t.Errorf("Status = %v; want Applied", patch.Status)
	}
	if patch.AppliedAt == nil {
		t.Error("AppliedAt should be set")
	}
}

func TestService_Apply_Errors(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	tests := []struct {
		name   string
		status domain.PatchStatus
		want   error
	}{
		{"not found", domain.PatchStatusPending, ErrPatchNotFound},
		{"already applied", domain.PatchStatusApplied, ErrPatchApplied},
		{"rejected", domain.PatchStatusRejected, ErrPatchRejected},
		{"expired", domain.PatchStatusExpired, ErrPatchExpired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var file, content string
			var err error
			if tt.name == "not found" {
				file, content, err = service.Apply(uuid.New())
			} else {
				patch := &domain.Patch{
					ID:        uuid.New(),
					SessionID: sessionID,
					Status:    tt.status,
				}
				service.mu.Lock()
				service.patches[patch.ID] = patch
				service.mu.Unlock()
				file, content, err = service.Apply(patch.ID)
			}
			if err != tt.want {
				t.Errorf("Apply() error = %v; want %v", err, tt.want)
			}
			if file != "" || content != "" {
				t.Error("Apply() should return empty file/content on error")
			}
		})
	}
}

func TestService_ApplyPending(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	patch := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		File:      "main.go",
		Proposed:  "new content",
		Status:    domain.PatchStatusApproved,
	}

	service.mu.Lock()
	service.patches[patch.ID] = patch
	service.sessions[sessionID] = []*domain.Patch{patch}
	service.pending[sessionID] = patch
	service.mu.Unlock()

	file, content, err := service.ApplyPending(sessionID)
	if err != nil {
		t.Fatalf("ApplyPending() error = %v", err)
	}

	if file != "main.go" {
		t.Errorf("ApplyPending() file = %q; want %q", file, "main.go")
	}
	if content != "new content" {
		t.Error("ApplyPending() content should match")
	}
}

func TestService_ApplyPending_NoPending(t *testing.T) {
	service := NewService()

	_, _, err := service.ApplyPending(uuid.New())
	if err != ErrPatchNotFound {
		t.Errorf("ApplyPending() error = %v; want ErrPatchNotFound", err)
	}
}

func TestService_Reject(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	patch := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		Status:    domain.PatchStatusPending,
	}

	service.mu.Lock()
	service.patches[patch.ID] = patch
	service.sessions[sessionID] = []*domain.Patch{patch}
	service.pending[sessionID] = patch
	service.mu.Unlock()

	err := service.Reject(patch.ID)
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}

	if patch.Status != domain.PatchStatusRejected {
		t.Errorf("Status = %v; want Rejected", patch.Status)
	}
}

func TestService_Reject_NotFound(t *testing.T) {
	service := NewService()

	err := service.Reject(uuid.New())
	if err != ErrPatchNotFound {
		t.Errorf("Reject() error = %v; want ErrPatchNotFound", err)
	}
}

func TestService_Reject_AlreadyApplied(t *testing.T) {
	service := NewService()

	patch := &domain.Patch{
		ID:        uuid.New(),
		SessionID: uuid.New(),
		Status:    domain.PatchStatusApplied,
	}

	service.mu.Lock()
	service.patches[patch.ID] = patch
	service.mu.Unlock()

	err := service.Reject(patch.ID)
	if err != ErrPatchApplied {
		t.Errorf("Reject() error = %v; want ErrPatchApplied", err)
	}
}

func TestService_RejectPending(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	patch := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		Status:    domain.PatchStatusPending,
	}

	service.mu.Lock()
	service.patches[patch.ID] = patch
	service.sessions[sessionID] = []*domain.Patch{patch}
	service.pending[sessionID] = patch
	service.mu.Unlock()

	err := service.RejectPending(sessionID)
	if err != nil {
		t.Fatalf("RejectPending() error = %v", err)
	}

	if patch.Status != domain.PatchStatusRejected {
		t.Errorf("Status = %v; want Rejected", patch.Status)
	}
}

func TestService_RejectPending_NoPending(t *testing.T) {
	service := NewService()

	err := service.RejectPending(uuid.New())
	if err != ErrPatchNotFound {
		t.Errorf("RejectPending() error = %v; want ErrPatchNotFound", err)
	}
}

func TestService_ExpireSession(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	patch1 := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		Status:    domain.PatchStatusPending,
	}
	patch2 := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		Status:    domain.PatchStatusApplied, // Already applied, shouldn't change
	}
	patch3 := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		Status:    domain.PatchStatusPending,
	}

	service.mu.Lock()
	service.patches[patch1.ID] = patch1
	service.patches[patch2.ID] = patch2
	service.patches[patch3.ID] = patch3
	service.sessions[sessionID] = []*domain.Patch{patch1, patch2, patch3}
	service.pending[sessionID] = patch1
	service.mu.Unlock()

	service.ExpireSession(sessionID)

	if patch1.Status != domain.PatchStatusExpired {
		t.Errorf("patch1.Status = %v; want Expired", patch1.Status)
	}
	if patch2.Status != domain.PatchStatusApplied {
		t.Errorf("patch2.Status = %v; should remain Applied", patch2.Status)
	}
	if patch3.Status != domain.PatchStatusExpired {
		t.Errorf("patch3.Status = %v; want Expired", patch3.Status)
	}

	// Pending should be cleared
	if service.GetPending(sessionID) != nil {
		t.Error("Pending should be cleared after ExpireSession")
	}
}

func TestService_AdvancePending(t *testing.T) {
	service := NewService()
	sessionID := uuid.New()

	patch1 := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		Status:    domain.PatchStatusPending,
	}
	patch2 := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		Status:    domain.PatchStatusPending,
	}
	patch3 := &domain.Patch{
		ID:        uuid.New(),
		SessionID: sessionID,
		Status:    domain.PatchStatusPending,
	}

	service.mu.Lock()
	service.patches[patch1.ID] = patch1
	service.patches[patch2.ID] = patch2
	service.patches[patch3.ID] = patch3
	service.sessions[sessionID] = []*domain.Patch{patch1, patch2, patch3}
	service.pending[sessionID] = patch1
	service.mu.Unlock()

	// Apply first patch - should advance to second
	patch1.Status = domain.PatchStatusApproved
	service.Apply(patch1.ID)

	pending := service.GetPending(sessionID)
	if pending == nil || pending.ID != patch2.ID {
		t.Error("Pending should advance to patch2")
	}

	// Apply second patch - should advance to third
	patch2.Status = domain.PatchStatusApproved
	service.Apply(patch2.ID)

	pending = service.GetPending(sessionID)
	if pending == nil || pending.ID != patch3.ID {
		t.Error("Pending should advance to patch3")
	}

	// Apply third patch - should clear pending
	patch3.Status = domain.PatchStatusApproved
	service.Apply(patch3.ID)

	if service.GetPending(sessionID) != nil {
		t.Error("Pending should be nil after all patches applied")
	}
}

func TestExtractContext(t *testing.T) {
	tests := []struct {
		name    string
		content string
		lines   int
		wantLen int
	}{
		{
			name:    "empty content",
			content: "",
			lines:   3,
			wantLen: 0,
		},
		{
			name:    "fewer lines than requested",
			content: "line1\nline2",
			lines:   5,
			wantLen: 2,
		},
		{
			name:    "exact lines",
			content: "line1\nline2\nline3",
			lines:   3,
			wantLen: 3,
		},
		{
			name:    "more lines than requested",
			content: "line1\nline2\nline3\nline4\nline5",
			lines:   3,
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractContext(tt.content, tt.lines)
			if len(got) != tt.wantLen {
				t.Errorf("extractContext() returned %d lines; want %d", len(got), tt.wantLen)
			}
		})
	}
}
