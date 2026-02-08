package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// cmdStats shows learning statistics
func cmdStats(args []string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	subCmd := "overview"
	if len(args) > 0 {
		subCmd = args[0]
	}

	switch subCmd {
	case "overview", "":
		return cmdStatsOverview()
	case "skills":
		return cmdStatsSkills()
	case "errors":
		return cmdStatsErrors()
	case "trend":
		return cmdStatsTrend()
	default:
		return fmt.Errorf("unknown stats command: %s (valid: overview, skills, errors, trend)", subCmd)
	}
}

func cmdStatsOverview() error {
	resp, err := http.Get(daemonAddr + "/v1/analytics/overview")
	if err != nil {
		return fmt.Errorf("get overview: %w", err)
	}
	defer resp.Body.Close()

	var overview struct {
		TotalSessions       int     `json:"total_sessions"`
		CompletedSessions   int     `json:"completed_sessions"`
		TotalRuns           int     `json:"total_runs"`
		TotalHints          int     `json:"total_hints"`
		TotalExercises      int     `json:"total_exercises"`
		HintDependency      float64 `json:"hint_dependency"`
		AvgTimeToGreen      string  `json:"avg_time_to_green"`
		CompletionRate      float64 `json:"completion_rate"`
		MostPracticedTopics []struct {
			Topic    string  `json:"topic"`
			Attempts int     `json:"attempts"`
			Level    float64 `json:"level"`
			Trend    string  `json:"trend"`
		} `json:"most_practiced_topics"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&overview); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("Learning Statistics")
	fmt.Println("==================")
	fmt.Printf("Total Sessions:     %d\n", overview.TotalSessions)
	fmt.Printf("Completed:          %d (%.1f%%)\n", overview.CompletedSessions, overview.CompletionRate*100)
	fmt.Printf("Total Exercises:    %d\n", overview.TotalExercises)
	fmt.Printf("Total Runs:         %d\n", overview.TotalRuns)
	fmt.Printf("Total Hints:        %d\n", overview.TotalHints)
	fmt.Printf("Hint Dependency:    %.1f%%\n", overview.HintDependency*100)
	fmt.Printf("Avg Time to Green:  %s\n", overview.AvgTimeToGreen)

	if len(overview.MostPracticedTopics) > 0 {
		fmt.Println("\nMost Practiced Topics")
		fmt.Println("---------------------")
		for _, topic := range overview.MostPracticedTopics {
			bar := renderProgressBar(topic.Level, 20)
			fmt.Printf("%-20s %s %.0f%% (%d attempts) %s\n",
				topic.Topic, bar, topic.Level*100, topic.Attempts, topic.Trend)
		}
	}

	return nil
}

func cmdStatsSkills() error {
	resp, err := http.Get(daemonAddr + "/v1/analytics/skills")
	if err != nil {
		return fmt.Errorf("get skills: %w", err)
	}
	defer resp.Body.Close()

	var breakdown struct {
		Skills map[string]struct {
			Topic      string  `json:"topic"`
			Level      float64 `json:"level"`
			Attempts   int     `json:"attempts"`
			Trend      string  `json:"trend"`
			Confidence float64 `json:"confidence"`
		} `json:"skills"`
		Progression []struct {
			Date         string  `json:"date"`
			AvgSkill     float64 `json:"avg_skill"`
			TopicsActive int     `json:"topics_active"`
		} `json:"progression"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&breakdown); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("Skills by Topic")
	fmt.Println("===============")

	if len(breakdown.Skills) == 0 {
		fmt.Println("No skills tracked yet. Start practicing!")
		return nil
	}

	for topic, skill := range breakdown.Skills {
		bar := renderProgressBar(skill.Level, 20)
		fmt.Printf("%-20s %s %.0f%% (%d attempts) %s\n",
			topic, bar, skill.Level*100, skill.Attempts, skill.Trend)
	}

	if len(breakdown.Progression) > 0 {
		fmt.Println("\nProgression (Last 30 days)")
		fmt.Println("--------------------------")
		for _, point := range breakdown.Progression {
			miniBar := renderProgressBar(point.AvgSkill, 10)
			fmt.Printf("%s: %s %.0f%% (%d topics)\n",
				point.Date, miniBar, point.AvgSkill*100, point.TopicsActive)
		}
	}

	return nil
}

func cmdStatsErrors() error {
	resp, err := http.Get(daemonAddr + "/v1/analytics/errors")
	if err != nil {
		return fmt.Errorf("get errors: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Patterns []struct {
			Pattern  string `json:"pattern"`
			Count    int    `json:"count"`
			Category string `json:"category"`
		} `json:"patterns"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("Common Error Patterns")
	fmt.Println("====================")

	if len(result.Patterns) == 0 {
		fmt.Println("No errors tracked yet. Keep coding!")
		return nil
	}

	for _, pattern := range result.Patterns {
		fmt.Printf("  [%s] %s (%d occurrences)\n",
			pattern.Category, pattern.Pattern, pattern.Count)
	}

	return nil
}

func cmdStatsTrend() error {
	resp, err := http.Get(daemonAddr + "/v1/analytics/trend")
	if err != nil {
		return fmt.Errorf("get trend: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Trend []struct {
			Timestamp  string  `json:"timestamp"`
			Dependency float64 `json:"dependency"`
			RunWindow  int     `json:"run_window"`
		} `json:"trend"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("Hint Dependency Trend")
	fmt.Println("====================")

	if len(result.Trend) == 0 {
		fmt.Println("Not enough data yet. Keep practicing!")
		return nil
	}

	// Show last 10 data points
	start := 0
	if len(result.Trend) > 10 {
		start = len(result.Trend) - 10
	}

	for _, point := range result.Trend[start:] {
		bar := renderProgressBar(point.Dependency, 20)
		fmt.Printf("%s: %s %.1f%%\n",
			point.Timestamp[:10], bar, point.Dependency*100)
	}

	// Calculate trend direction
	if len(result.Trend) >= 2 {
		first := result.Trend[0].Dependency
		last := result.Trend[len(result.Trend)-1].Dependency
		if last < first-0.05 {
			fmt.Println("\n↓ Your hint dependency is decreasing - great progress!")
		} else if last > first+0.05 {
			fmt.Println("\n↑ Your hint dependency is increasing - try solving more on your own")
		} else {
			fmt.Println("\n→ Your hint dependency is stable")
		}
	}

	return nil
}
