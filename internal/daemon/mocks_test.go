package daemon

import (
	"context"
	"errors"
	"net/http"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/patch"
	"github.com/felixgeelhaar/temper/internal/profile"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/felixgeelhaar/temper/internal/spec"
	"github.com/google/uuid"
)

var errNotImplemented = errors.New("mock: not implemented")

// mockSessionService implements session.SessionService for testing
type mockSessionService struct {
	createFn             func(ctx context.Context, req session.CreateRequest) (*session.Session, error)
	getFn                func(ctx context.Context, id string) (*session.Session, error)
	deleteFn             func(ctx context.Context, id string) error
	runCodeFn            func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error)
	updateCodeFn         func(ctx context.Context, id string, code map[string]string) (*session.Session, error)
	recordInterventionFn func(ctx context.Context, intervention *session.Intervention) error
}

func (m *mockSessionService) Create(ctx context.Context, req session.CreateRequest) (*session.Session, error) {
	if m.createFn != nil {
		return m.createFn(ctx, req)
	}
	return nil, errNotImplemented
}

func (m *mockSessionService) Get(ctx context.Context, id string) (*session.Session, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, errNotImplemented
}

func (m *mockSessionService) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return errNotImplemented
}

func (m *mockSessionService) RunCode(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
	if m.runCodeFn != nil {
		return m.runCodeFn(ctx, sessionID, req)
	}
	return nil, errNotImplemented
}

func (m *mockSessionService) UpdateCode(ctx context.Context, id string, code map[string]string) (*session.Session, error) {
	if m.updateCodeFn != nil {
		return m.updateCodeFn(ctx, id, code)
	}
	return nil, errNotImplemented
}

func (m *mockSessionService) RecordIntervention(ctx context.Context, intervention *session.Intervention) error {
	if m.recordInterventionFn != nil {
		return m.recordInterventionFn(ctx, intervention)
	}
	return errNotImplemented
}

var _ session.SessionService = (*mockSessionService)(nil)

// mockPairingService implements pairing.PairingService for testing
type mockPairingService struct {
	interveneFn         func(ctx context.Context, req pairing.InterventionRequest) (*domain.Intervention, error)
	interveneStreamFn   func(ctx context.Context, req pairing.InterventionRequest) (<-chan pairing.StreamChunk, error)
	suggestForSectionFn func(ctx context.Context, authCtx pairing.AuthoringContext) ([]domain.AuthoringSuggestion, error)
	authoringHintFn     func(ctx context.Context, authCtx pairing.AuthoringContext) (*domain.Intervention, error)
}

func (m *mockPairingService) Intervene(ctx context.Context, req pairing.InterventionRequest) (*domain.Intervention, error) {
	if m.interveneFn != nil {
		return m.interveneFn(ctx, req)
	}
	return nil, errNotImplemented
}

func (m *mockPairingService) IntervenStream(ctx context.Context, req pairing.InterventionRequest) (<-chan pairing.StreamChunk, error) {
	if m.interveneStreamFn != nil {
		return m.interveneStreamFn(ctx, req)
	}
	return nil, errNotImplemented
}

func (m *mockPairingService) SuggestForSection(ctx context.Context, authCtx pairing.AuthoringContext) ([]domain.AuthoringSuggestion, error) {
	if m.suggestForSectionFn != nil {
		return m.suggestForSectionFn(ctx, authCtx)
	}
	return nil, errNotImplemented
}

func (m *mockPairingService) AuthoringHint(ctx context.Context, authCtx pairing.AuthoringContext) (*domain.Intervention, error) {
	if m.authoringHintFn != nil {
		return m.authoringHintFn(ctx, authCtx)
	}
	return nil, errNotImplemented
}

var _ pairing.PairingService = (*mockPairingService)(nil)

// mockPatchService implements patch.PatchService for testing
type mockPatchService struct {
	extractFromInterventionFn func(intervention *domain.Intervention, sessionID uuid.UUID, currentCode map[string]string) []*domain.Patch
	previewPendingFn          func(sessionID uuid.UUID) (*domain.PatchPreview, error)
	applyPendingFn            func(sessionID uuid.UUID) (file string, content string, err error)
	rejectPendingFn           func(sessionID uuid.UUID) error
	getSessionPatchesFn       func(sessionID uuid.UUID) []*domain.Patch
	getLoggerFn               func() *patch.Logger
}

func (m *mockPatchService) ExtractFromIntervention(intervention *domain.Intervention, sessionID uuid.UUID, currentCode map[string]string) []*domain.Patch {
	if m.extractFromInterventionFn != nil {
		return m.extractFromInterventionFn(intervention, sessionID, currentCode)
	}
	return nil
}

func (m *mockPatchService) PreviewPending(sessionID uuid.UUID) (*domain.PatchPreview, error) {
	if m.previewPendingFn != nil {
		return m.previewPendingFn(sessionID)
	}
	return nil, errNotImplemented
}

func (m *mockPatchService) ApplyPending(sessionID uuid.UUID) (file string, content string, err error) {
	if m.applyPendingFn != nil {
		return m.applyPendingFn(sessionID)
	}
	return "", "", errNotImplemented
}

func (m *mockPatchService) RejectPending(sessionID uuid.UUID) error {
	if m.rejectPendingFn != nil {
		return m.rejectPendingFn(sessionID)
	}
	return errNotImplemented
}

func (m *mockPatchService) GetSessionPatches(sessionID uuid.UUID) []*domain.Patch {
	if m.getSessionPatchesFn != nil {
		return m.getSessionPatchesFn(sessionID)
	}
	return nil
}

func (m *mockPatchService) GetLogger() *patch.Logger {
	if m.getLoggerFn != nil {
		return m.getLoggerFn()
	}
	return nil
}

var _ patch.PatchService = (*mockPatchService)(nil)

// mockSpecService implements spec.SpecService for testing
type mockSpecService struct {
	createFn                 func(ctx context.Context, name string) (*domain.ProductSpec, error)
	loadFn                   func(ctx context.Context, path string) (*domain.ProductSpec, error)
	listFn                   func(ctx context.Context) ([]*domain.ProductSpec, error)
	validateFn               func(ctx context.Context, path string) (*domain.SpecValidation, error)
	markCriterionSatisfiedFn func(ctx context.Context, path, criterionID, evidence string) error
	lockFn                   func(ctx context.Context, path string) (*domain.SpecLock, error)
	getProgressFn            func(ctx context.Context, path string) (*domain.SpecProgress, error)
	getDriftFn               func(ctx context.Context, path string) (*spec.DriftReport, error)
	saveFn                   func(ctx context.Context, spec *domain.ProductSpec) error
	getWorkspaceRootFn       func() string
}

func (m *mockSpecService) Create(ctx context.Context, name string) (*domain.ProductSpec, error) {
	if m.createFn != nil {
		return m.createFn(ctx, name)
	}
	return nil, errNotImplemented
}

func (m *mockSpecService) Load(ctx context.Context, path string) (*domain.ProductSpec, error) {
	if m.loadFn != nil {
		return m.loadFn(ctx, path)
	}
	return nil, errNotImplemented
}

func (m *mockSpecService) List(ctx context.Context) ([]*domain.ProductSpec, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return nil, errNotImplemented
}

func (m *mockSpecService) Validate(ctx context.Context, path string) (*domain.SpecValidation, error) {
	if m.validateFn != nil {
		return m.validateFn(ctx, path)
	}
	return nil, errNotImplemented
}

func (m *mockSpecService) MarkCriterionSatisfied(ctx context.Context, path, criterionID, evidence string) error {
	if m.markCriterionSatisfiedFn != nil {
		return m.markCriterionSatisfiedFn(ctx, path, criterionID, evidence)
	}
	return errNotImplemented
}

func (m *mockSpecService) Lock(ctx context.Context, path string) (*domain.SpecLock, error) {
	if m.lockFn != nil {
		return m.lockFn(ctx, path)
	}
	return nil, errNotImplemented
}

func (m *mockSpecService) GetProgress(ctx context.Context, path string) (*domain.SpecProgress, error) {
	if m.getProgressFn != nil {
		return m.getProgressFn(ctx, path)
	}
	return nil, errNotImplemented
}

func (m *mockSpecService) GetDrift(ctx context.Context, path string) (*spec.DriftReport, error) {
	if m.getDriftFn != nil {
		return m.getDriftFn(ctx, path)
	}
	return nil, errNotImplemented
}

func (m *mockSpecService) Save(ctx context.Context, spec *domain.ProductSpec) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, spec)
	}
	return errNotImplemented
}

func (m *mockSpecService) GetWorkspaceRoot() string {
	if m.getWorkspaceRootFn != nil {
		return m.getWorkspaceRootFn()
	}
	return ""
}

var _ spec.SpecService = (*mockSpecService)(nil)

// mockProfileService implements profile.ProfileService for testing
type mockProfileService struct {
	getProfileFn        func(ctx context.Context) (*profile.StoredProfile, error)
	getOverviewFn       func(ctx context.Context) (*profile.AnalyticsOverview, error)
	getSkillBreakdownFn func(ctx context.Context) (*profile.SkillBreakdown, error)
	getErrorPatternsFn  func(ctx context.Context) ([]profile.ErrorPattern, error)
	getHintTrendFn      func(ctx context.Context) ([]profile.HintDependencyPoint, error)
	onSessionStartFn    func(ctx context.Context, sess profile.SessionInfo) error
	onSessionCompleteFn func(ctx context.Context, sess profile.SessionInfo) error
	onRunCompleteFn     func(ctx context.Context, sess profile.SessionInfo, run profile.RunInfo) error
	onHintDeliveredFn   func(ctx context.Context, sess profile.SessionInfo) error
}

func (m *mockProfileService) GetProfile(ctx context.Context) (*profile.StoredProfile, error) {
	if m.getProfileFn != nil {
		return m.getProfileFn(ctx)
	}
	return nil, errNotImplemented
}

func (m *mockProfileService) GetOverview(ctx context.Context) (*profile.AnalyticsOverview, error) {
	if m.getOverviewFn != nil {
		return m.getOverviewFn(ctx)
	}
	return nil, errNotImplemented
}

func (m *mockProfileService) GetSkillBreakdown(ctx context.Context) (*profile.SkillBreakdown, error) {
	if m.getSkillBreakdownFn != nil {
		return m.getSkillBreakdownFn(ctx)
	}
	return nil, errNotImplemented
}

func (m *mockProfileService) GetErrorPatterns(ctx context.Context) ([]profile.ErrorPattern, error) {
	if m.getErrorPatternsFn != nil {
		return m.getErrorPatternsFn(ctx)
	}
	return nil, errNotImplemented
}

func (m *mockProfileService) GetHintTrend(ctx context.Context) ([]profile.HintDependencyPoint, error) {
	if m.getHintTrendFn != nil {
		return m.getHintTrendFn(ctx)
	}
	return nil, errNotImplemented
}

func (m *mockProfileService) OnSessionStart(ctx context.Context, sess profile.SessionInfo) error {
	if m.onSessionStartFn != nil {
		return m.onSessionStartFn(ctx, sess)
	}
	return errNotImplemented
}

func (m *mockProfileService) OnSessionComplete(ctx context.Context, sess profile.SessionInfo) error {
	if m.onSessionCompleteFn != nil {
		return m.onSessionCompleteFn(ctx, sess)
	}
	return errNotImplemented
}

func (m *mockProfileService) OnRunComplete(ctx context.Context, sess profile.SessionInfo, run profile.RunInfo) error {
	if m.onRunCompleteFn != nil {
		return m.onRunCompleteFn(ctx, sess, run)
	}
	return errNotImplemented
}

func (m *mockProfileService) OnHintDelivered(ctx context.Context, sess profile.SessionInfo) error {
	if m.onHintDeliveredFn != nil {
		return m.onHintDeliveredFn(ctx, sess)
	}
	return errNotImplemented
}

var _ profile.ProfileService = (*mockProfileService)(nil)

// mockLLMRegistry implements llm.LLMRegistry for testing
type mockLLMRegistry struct {
	listFn       func() []string
	defaultFn    func() (llm.Provider, error)
	getFn        func(name string) (llm.Provider, error)
	setDefaultFn func(name string) error
	registerFn   func(name string, p llm.Provider)
}

func (m *mockLLMRegistry) List() []string {
	if m.listFn != nil {
		return m.listFn()
	}
	return nil
}

func (m *mockLLMRegistry) Default() (llm.Provider, error) {
	if m.defaultFn != nil {
		return m.defaultFn()
	}
	return nil, errNotImplemented
}

func (m *mockLLMRegistry) Get(name string) (llm.Provider, error) {
	if m.getFn != nil {
		return m.getFn(name)
	}
	return nil, errNotImplemented
}

func (m *mockLLMRegistry) SetDefault(name string) error {
	if m.setDefaultFn != nil {
		return m.setDefaultFn(name)
	}
	return errNotImplemented
}

func (m *mockLLMRegistry) Register(name string, p llm.Provider) {
	if m.registerFn != nil {
		m.registerFn(name, p)
	}
}

var _ llm.LLMRegistry = (*mockLLMRegistry)(nil)

// mockExecutor implements runner.Executor for testing
type mockExecutor struct {
	runFormatFn    func(ctx context.Context, code map[string]string) (*runner.FormatResult, error)
	runFormatFixFn func(ctx context.Context, code map[string]string) (map[string]string, error)
	runBuildFn     func(ctx context.Context, code map[string]string) (*runner.BuildResult, error)
	runTestsFn     func(ctx context.Context, code map[string]string, flags []string) (*runner.TestResult, error)
}

func (m *mockExecutor) RunFormat(ctx context.Context, code map[string]string) (*runner.FormatResult, error) {
	if m.runFormatFn != nil {
		return m.runFormatFn(ctx, code)
	}
	return nil, errNotImplemented
}

func (m *mockExecutor) RunFormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
	if m.runFormatFixFn != nil {
		return m.runFormatFixFn(ctx, code)
	}
	return nil, errNotImplemented
}

func (m *mockExecutor) RunBuild(ctx context.Context, code map[string]string) (*runner.BuildResult, error) {
	if m.runBuildFn != nil {
		return m.runBuildFn(ctx, code)
	}
	return nil, errNotImplemented
}

func (m *mockExecutor) RunTests(ctx context.Context, code map[string]string, flags []string) (*runner.TestResult, error) {
	if m.runTestsFn != nil {
		return m.runTestsFn(ctx, code, flags)
	}
	return nil, errNotImplemented
}

var _ runner.Executor = (*mockExecutor)(nil)

// serverWithMocks holds a server configured with mock dependencies
type serverWithMocks struct {
	server   *Server
	sessions *mockSessionService
	pairing  *mockPairingService
	patches  *mockPatchService
	specs    *mockSpecService
	profiles *mockProfileService
	registry *mockLLMRegistry
	executor *mockExecutor
}

// newServerWithMocks creates a minimal Server with all mock dependencies injected
// This enables isolated testing of handlers without real service dependencies
func newServerWithMocks() *serverWithMocks {
	sessions := &mockSessionService{}
	pairingMock := &mockPairingService{}
	patches := &mockPatchService{}
	specs := &mockSpecService{}
	profiles := &mockProfileService{}
	registry := &mockLLMRegistry{}
	executor := &mockExecutor{}

	router := http.NewServeMux()

	srv := &Server{
		router:         router,
		sessionService: sessions,
		pairingService: pairingMock,
		patchService:   patches,
		specService:    specs,
		profileService: profiles,
		llmRegistry:    registry,
		runnerExecutor: executor,
	}

	// Register routes (need to set up routes manually for isolated testing)
	srv.setupRoutes()

	return &serverWithMocks{
		server:   srv,
		sessions: sessions,
		pairing:  pairingMock,
		patches:  patches,
		specs:    specs,
		profiles: profiles,
		registry: registry,
		executor: executor,
	}
}

// mockExerciseLoader implements a minimal exercise loader for testing
type mockExerciseLoader struct {
	packs []string
}
