package appreciation

import (
	"fmt"
	"sync"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/google/uuid"
)

// Service handles appreciation detection and generation
type Service struct {
	detector  *Detector
	generator *Generator

	// Track last appreciation per user to avoid spam
	mu               sync.RWMutex
	lastAppreciation map[uuid.UUID]time.Time
}

// NewService creates a new appreciation service
func NewService() *Service {
	return &Service{
		detector:         NewDetector(),
		generator:        NewGenerator(),
		lastAppreciation: make(map[uuid.UUID]time.Time),
	}
}

// CheckSession evaluates a completed session for appreciation
func (s *Service) CheckSession(userID uuid.UUID, sess *session.Session, output *domain.RunOutput) *Message {
	moments := s.detector.DetectSessionMoments(sess, output)
	if len(moments) == 0 {
		return nil
	}

	best := s.detector.SelectBest(moments)
	if best == nil {
		return nil
	}

	// Check if we should show appreciation based on timing
	if !s.shouldShow(userID, best.Type) {
		return nil
	}

	msg := s.generator.Generate(best)
	if msg != nil {
		s.recordAppreciation(userID)
	}

	return msg
}

// CheckProgress evaluates profile changes for appreciation
func (s *Service) CheckProgress(userID uuid.UUID, current, previous *domain.LearningProfile) *Message {
	moments := s.detector.DetectProfileMoments(current, previous)
	if len(moments) == 0 {
		return nil
	}

	best := s.detector.SelectBest(moments)
	if best == nil {
		return nil
	}

	if !s.shouldShow(userID, best.Type) {
		return nil
	}

	msg := s.generator.Generate(best)
	if msg != nil {
		s.recordAppreciation(userID)
	}

	return msg
}

// CheckSkill evaluates skill achievement for appreciation
func (s *Service) CheckSkill(userID uuid.UUID, skill *Skill, isFirst bool) *Message {
	moments := s.detector.DetectTopicMoments(skill, isFirst)
	if len(moments) == 0 {
		return nil
	}

	best := s.detector.SelectBest(moments)
	if best == nil {
		return nil
	}

	if !s.shouldShow(userID, best.Type) {
		return nil
	}

	msg := s.generator.Generate(best)
	if msg != nil {
		s.recordAppreciation(userID)
	}

	return msg
}

// CheckSpec evaluates spec progress for appreciation
func (s *Service) CheckSpec(userID uuid.UUID, spec *domain.ProductSpec, criterion *domain.AcceptanceCriterion) *Message {
	moments := s.detector.DetectSpecMoments(spec, criterion)
	if len(moments) == 0 {
		return nil
	}

	best := s.detector.SelectBest(moments)
	if best == nil {
		return nil
	}

	// Spec moments are always shown (they're significant)
	msg := s.generator.Generate(best)
	if msg != nil {
		s.recordAppreciation(userID)
	}

	return msg
}

// GetMomentPriority returns the priority of a moment type
func (s *Service) GetMomentPriority(momentType MomentType) int {
	priority := map[MomentType]int{
		MomentSpecComplete:       10,
		MomentTopicMastery:       9,
		MomentFirstTrySuccess:    8,
		MomentNoHintsNeeded:      7,
		MomentReducedDependency:  6,
		MomentCriterionSatisfied: 5,
		MomentNoEscalation:       4,
		MomentMinimalHints:       3,
		MomentConsistentSuccess:  3,
		MomentQuickResolution:    2,
		MomentAllTestsPassing:    1,
		MomentFirstInTopic:       1,
	}
	return priority[momentType]
}

func (s *Service) shouldShow(userID uuid.UUID, momentType MomentType) bool {
	s.mu.RLock()
	lastTime, exists := s.lastAppreciation[userID]
	s.mu.RUnlock()

	if !exists {
		return true // First appreciation for this user
	}

	minutesSince := int(time.Since(lastTime).Minutes())
	priority := s.GetMomentPriority(momentType)

	return ShouldAppreciate(minutesSince, priority)
}

func (s *Service) recordAppreciation(userID uuid.UUID) {
	s.mu.Lock()
	s.lastAppreciation[userID] = time.Now()
	s.mu.Unlock()
}

// MinutesSinceLastAppreciation returns how long since last appreciation for a user
func (s *Service) MinutesSinceLastAppreciation(userID uuid.UUID) int {
	s.mu.RLock()
	lastTime, exists := s.lastAppreciation[userID]
	s.mu.RUnlock()

	if !exists {
		return -1 // Never shown
	}

	return int(time.Since(lastTime).Minutes())
}

// GenerateSessionSummary creates a motivational summary for a completed session
func (s *Service) GenerateSessionSummary(sess *session.Session, specProgress string) *SessionSummary {
	if sess == nil {
		return nil
	}

	duration := time.Since(sess.CreatedAt)
	durationStr := formatSessionDuration(duration)

	// Create the base summary
	summary := &SessionSummary{
		Duration:     durationStr,
		RunCount:     sess.RunCount,
		HintCount:    sess.HintCount,
		Intent:       string(sess.Intent),
		SpecPath:     sess.SpecPath,
		SpecProgress: specProgress,
	}

	// Determine accomplishment and message based on session performance
	accomplishment, message := s.determineAccomplishment(sess, specProgress, duration)
	summary.Accomplishment = accomplishment
	summary.Message = message

	// Add evidence
	summary.Evidence = &Evidence{
		HintCount:       sess.HintCount,
		RunCount:        sess.RunCount,
		SessionDuration: durationStr,
		SpecProgress:    specProgress,
	}

	return summary
}

// determineAccomplishment selects the right accomplishment and message based on session metrics
func (s *Service) determineAccomplishment(sess *session.Session, specProgress string, duration time.Duration) (accomplishment, message string) {
	durationStr := formatSessionDuration(duration)

	// Check for spec completion
	if specProgress == "100%" {
		return "Spec Complete", "You completed all acceptance criteria. The feature is done."
	}

	// Check for no hints needed
	if sess.HintCount == 0 && sess.RunCount > 0 {
		return "Self-Reliant", "No hints needed. Your independent problem-solving is strong."
	}

	// Check for minimal hints
	if sess.HintCount > 0 && sess.HintCount <= 2 {
		return "Focused Learning", "Minimal guidance needed. You're building real understanding."
	}

	// Check for quick session (less than 15 minutes)
	if duration < 15*time.Minute && sess.RunCount > 0 {
		return "Quick Progress", "Efficient session. You knew what to do and did it."
	}

	// Check for spec progress
	if specProgress != "" && specProgress != "0%" {
		return "Making Progress", "Steady progress on the feature. Keep building."
	}

	// Check for effort (multiple runs)
	if sess.RunCount >= 3 {
		return "Persistent Effort", "Multiple iterations show determination. That's how skills are built."
	}

	// Default motivational message
	moment := &Moment{
		Type: MomentSessionEnd,
		Evidence: Evidence{
			SessionDuration: durationStr,
		},
	}
	msg := s.generator.Generate(moment)
	if msg != nil {
		return "Session Complete", msg.Text
	}

	return "Session Complete", "Every session is practice. Keep going."
}

// formatSessionDuration formats a duration in a human-readable way
func formatSessionDuration(d time.Duration) string {
	if d < time.Minute {
		return "under a minute"
	}

	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60

	if hours == 0 {
		if mins == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", mins)
	}

	if hours == 1 {
		if mins == 0 {
			return "1 hour"
		}
		return fmt.Sprintf("1 hour %d minutes", mins)
	}

	if mins == 0 {
		return fmt.Sprintf("%d hours", hours)
	}
	return fmt.Sprintf("%d hours %d minutes", hours, mins)
}
