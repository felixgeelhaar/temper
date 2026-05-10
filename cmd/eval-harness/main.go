// eval-harness runs the pairing evaluation golden set against a configured
// LLM provider and reports clamp adherence + custom assertions.
//
// Usage:
//   eval-harness -dir eval/cases [-provider claude]
//
// Requires the same secrets.yaml the daemon reads (~/.temper/secrets.yaml)
// for API keys.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/felixgeelhaar/temper/internal/config"
	"github.com/felixgeelhaar/temper/internal/eval"
	"github.com/felixgeelhaar/temper/internal/llm"
)

func main() {
	dir := flag.String("dir", "eval/cases", "directory containing case .yaml files")
	providerName := flag.String("provider", "", "provider override (default: config default_provider)")
	threshold := flag.Float64("threshold", 0.9, "minimum pass rate (0..1) for non-zero exit")
	timeout := flag.Duration("timeout", 5*time.Minute, "overall run timeout")
	flag.Parse()

	if err := run(*dir, *providerName, *threshold, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "eval failed: %v\n", err)
		os.Exit(1)
	}
}

func run(dir, providerOverride string, threshold float64, timeout time.Duration) error {
	cases, err := eval.LoadCases(dir)
	if err != nil {
		return fmt.Errorf("load cases: %w", err)
	}
	if len(cases) == 0 {
		return fmt.Errorf("no cases found in %s", dir)
	}
	fmt.Printf("loaded %d cases from %s\n", len(cases), dir)

	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	registry := llm.NewRegistry()
	for name, p := range cfg.LLM.Providers {
		if !p.Enabled || p.APIKey == "" {
			continue
		}
		switch name {
		case "claude":
			registry.Register(name, llm.NewClaudeProvider(llm.ClaudeConfig{
				APIKey: p.APIKey,
				Model:  p.Model,
			}))
		case "openai":
			registry.Register(name, llm.NewOpenAIProvider(llm.OpenAIConfig{
				APIKey: p.APIKey,
				Model:  p.Model,
			}))
		}
	}

	if providerOverride != "" {
		if err := registry.SetDefault(providerOverride); err != nil {
			return fmt.Errorf("set provider %s: %w", providerOverride, err)
		}
	} else if cfg.LLM.DefaultProvider != "" && cfg.LLM.DefaultProvider != "auto" {
		_ = registry.SetDefault(cfg.LLM.DefaultProvider)
	}

	provider, err := registry.Default()
	if err != nil {
		return fmt.Errorf("no LLM provider available: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	gen := &eval.LLMGenerator{Provider: provider}
	result, err := eval.Run(ctx, cases, gen)
	if err != nil {
		return fmt.Errorf("run cases: %w", err)
	}

	fmt.Println(eval.FormatReport(result))

	if result.PassRate() < threshold {
		return fmt.Errorf("pass rate %.1f%% below threshold %.1f%%",
			result.PassRate()*100, threshold*100)
	}
	return nil
}
