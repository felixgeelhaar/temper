package appreciation

import (
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
