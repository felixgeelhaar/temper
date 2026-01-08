package domain

import (
	"time"
)

// SkillEvaluator is a domain service for cross-cutting skill analysis.
// It evaluates learning patterns and progress across exercises and topics.
type SkillEvaluator struct{}

// NewSkillEvaluator creates a new skill evaluator
func NewSkillEvaluator() *SkillEvaluator {
	return &SkillEvaluator{}
}

// SkillGrowthRate represents the velocity of skill improvement
type SkillGrowthRate string

const (
	GrowthRapid     SkillGrowthRate = "rapid"
	GrowthSteady    SkillGrowthRate = "steady"
	GrowthSlow      SkillGrowthRate = "slow"
	GrowthPlateaued SkillGrowthRate = "plateaued"
)

// SkillAssessment provides a comprehensive evaluation of a user's skill
type SkillAssessment struct {
	// Overall metrics
	OverallLevel   float64         // 0.0 - 1.0 aggregate skill level
	GrowthRate     SkillGrowthRate // velocity of improvement
	HintDependency float64         // 0.0 - 1.0 reliance on hints

	// Topic analysis
	StrongestTopics []string // top 3 strongest areas
	WeakestTopics   []string // top 3 areas needing work
	BlindSpots      []string // common error patterns

	// Recommendations
	SuggestedFocus      string            // topic to focus on next
	RecommendedLevel    InterventionLevel // suggested intervention cap
	ReadyForAdvancement bool              // ready for harder exercises
}

// EvaluateSkill performs a comprehensive skill assessment
func (e *SkillEvaluator) EvaluateSkill(profile *LearningProfile) *SkillAssessment {
	if profile == nil {
		return &SkillAssessment{
			RecommendedLevel: L2LocationConcept,
		}
	}

	assessment := &SkillAssessment{
		HintDependency:   profile.HintDependency(),
		RecommendedLevel: profile.SuggestMaxLevel(),
	}

	// Calculate overall level
	assessment.OverallLevel = e.calculateOverallLevel(profile)

	// Determine growth rate
	assessment.GrowthRate = e.determineGrowthRate(profile)

	// Analyze topics
	assessment.StrongestTopics, assessment.WeakestTopics = e.analyzeTopics(profile)

	// Identify blind spots from common errors
	assessment.BlindSpots = e.identifyBlindSpots(profile)

	// Suggest focus area
	assessment.SuggestedFocus = e.suggestFocus(profile, assessment)

	// Determine readiness for advancement
	assessment.ReadyForAdvancement = e.isReadyForAdvancement(assessment)

	return assessment
}

// calculateOverallLevel computes a weighted average skill level
func (e *SkillEvaluator) calculateOverallLevel(profile *LearningProfile) float64 {
	if len(profile.TopicSkills) == 0 {
		return 0.0
	}

	var totalWeight float64
	var weightedSum float64

	for _, skill := range profile.TopicSkills {
		// Weight by recency and attempts
		weight := float64(skill.Attempts)
		if time.Since(skill.LastSeen) < 7*24*time.Hour {
			weight *= 1.5 // boost recent activity
		}

		weightedSum += skill.Level * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0.0
	}

	return weightedSum / totalWeight
}

// determineGrowthRate analyzes skill improvement velocity
func (e *SkillEvaluator) determineGrowthRate(profile *LearningProfile) SkillGrowthRate {
	// Heuristics based on profile metrics
	if profile.TotalRuns < 10 {
		return GrowthSteady // Not enough data
	}

	dependency := profile.HintDependency()
	avgSkill := e.calculateOverallLevel(profile)

	// Rapid: high skill with low dependency
	if avgSkill > 0.7 && dependency < 0.3 {
		return GrowthRapid
	}

	// Plateaued: moderate skill but high dependency
	if avgSkill > 0.4 && avgSkill < 0.6 && dependency > 0.5 {
		return GrowthPlateaued
	}

	// Slow: low skill with high dependency
	if avgSkill < 0.4 && dependency > 0.5 {
		return GrowthSlow
	}

	return GrowthSteady
}

// analyzeTopics identifies strongest and weakest topics
func (e *SkillEvaluator) analyzeTopics(profile *LearningProfile) (strongest, weakest []string) {
	if len(profile.TopicSkills) == 0 {
		return nil, nil
	}

	type topicScore struct {
		topic string
		score float64
	}

	var scores []topicScore
	for topic, skill := range profile.TopicSkills {
		// Adjust score by recency
		recencyFactor := 1.0
		daysSince := time.Since(skill.LastSeen).Hours() / 24
		if daysSince > 14 {
			recencyFactor = 0.8
		} else if daysSince > 30 {
			recencyFactor = 0.5
		}

		scores = append(scores, topicScore{
			topic: topic,
			score: skill.Level * recencyFactor,
		})
	}

	// Sort by score (simple insertion sort for small slices)
	for i := 1; i < len(scores); i++ {
		j := i
		for j > 0 && scores[j-1].score < scores[j].score {
			scores[j-1], scores[j] = scores[j], scores[j-1]
			j--
		}
	}

	// Extract top 3 strongest
	for i := 0; i < len(scores) && i < 3; i++ {
		strongest = append(strongest, scores[i].topic)
	}

	// Extract bottom 3 weakest (those with attempts but low skill)
	for i := len(scores) - 1; i >= 0 && len(weakest) < 3; i-- {
		if profile.TopicSkills[scores[i].topic].Attempts > 0 {
			weakest = append(weakest, scores[i].topic)
		}
	}

	return strongest, weakest
}

// identifyBlindSpots finds common error patterns
func (e *SkillEvaluator) identifyBlindSpots(profile *LearningProfile) []string {
	// CommonErrors is already populated by the profile
	if len(profile.CommonErrors) > 3 {
		return profile.CommonErrors[:3]
	}
	return profile.CommonErrors
}

// suggestFocus recommends the next topic to work on
func (e *SkillEvaluator) suggestFocus(profile *LearningProfile, assessment *SkillAssessment) string {
	// Prioritize weakest topics that have recent activity
	for _, topic := range assessment.WeakestTopics {
		skill := profile.TopicSkills[topic]
		if time.Since(skill.LastSeen) < 14*24*time.Hour {
			return topic
		}
	}

	// If no weak recent topics, suggest continuing strongest
	if len(assessment.StrongestTopics) > 0 {
		return assessment.StrongestTopics[0]
	}

	return ""
}

// isReadyForAdvancement determines if user is ready for harder exercises
func (e *SkillEvaluator) isReadyForAdvancement(assessment *SkillAssessment) bool {
	// Ready if:
	// - Overall level is high
	// - Low hint dependency
	// - Growth rate is not slow/plateaued
	return assessment.OverallLevel > 0.6 &&
		assessment.HintDependency < 0.3 &&
		assessment.GrowthRate != GrowthSlow &&
		assessment.GrowthRate != GrowthPlateaued
}

// CalculateSkillDelta computes skill change for a topic based on outcome
func (e *SkillEvaluator) CalculateSkillDelta(currentLevel float64, success bool, difficulty Difficulty) float64 {
	baseDelta := 0.05 // base improvement

	// Adjust based on difficulty
	switch difficulty {
	case DifficultyBeginner:
		baseDelta *= 0.8 // less gain from easy exercises
	case DifficultyIntermediate:
		// standard delta
	case DifficultyAdvanced:
		baseDelta *= 1.5 // more gain from hard exercises
	}

	if success {
		// Diminishing returns at high skill
		if currentLevel > 0.8 {
			baseDelta *= 0.5
		}
		return baseDelta
	}

	// Failure: smaller penalty
	return -baseDelta * 0.4
}

// ShouldSuggestBreak determines if user should take a break
func (e *SkillEvaluator) ShouldSuggestBreak(signals *ProfileSignals) bool {
	if signals == nil {
		return false
	}

	// Suggest break if:
	// - Many runs without success
	// - Long time on exercise
	// - High frustration indicators
	return signals.RunsThisSession > 10 ||
		signals.TimeOnExercise > 30*time.Minute ||
		(signals.RunsThisSession > 5 && len(signals.ErrorsEncountered) > 5)
}

// EscalationThreshold determines when automatic escalation should occur
func (e *SkillEvaluator) EscalationThreshold(profile *LearningProfile, difficulty Difficulty) int {
	// Base threshold: 3 attempts before escalation
	threshold := 3

	// More patient with beginners
	if difficulty == DifficultyBeginner {
		threshold = 4
	}

	// Less patient with advanced (they should get it faster)
	if difficulty == DifficultyAdvanced {
		threshold = 2
	}

	// Adjust based on profile
	if profile != nil {
		dependency := profile.HintDependency()
		// More dependent users get escalation sooner
		if dependency > 0.5 {
			threshold = max(2, threshold-1)
		}
	}

	return threshold
}
