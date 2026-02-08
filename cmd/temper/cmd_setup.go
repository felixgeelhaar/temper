package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/felixgeelhaar/temper/internal/config"
)

// cmdInit initializes Temper for first-time use
func cmdInit() error {
	fmt.Println("Temper - First-Time Setup")
	fmt.Println("==========================")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// 1. Create directory structure
	fmt.Print("Creating ~/.temper directory structure... ")
	temperDir, err := config.EnsureTemperDir()
	if err != nil {
		return fmt.Errorf("create directories: %w", err)
	}
	fmt.Println("✓")

	// 2. Create default config if it doesn't exist
	configPath := filepath.Join(temperDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Print("Creating default configuration... ")
		if err := config.SaveLocalConfig(config.DefaultLocalConfig()); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Println("✓")
	} else {
		fmt.Println("Configuration already exists ✓")
	}

	// 3. Copy bundled exercises
	fmt.Print("Setting up exercise packs... ")
	exercisesDest := filepath.Join(temperDir, "exercises")

	// Check if exercises exist in current directory (dev mode)
	if _, err := os.Stat("./exercises"); err == nil {
		// Copy from local
		if err := copyDir("./exercises", exercisesDest); err != nil {
			fmt.Println("⚠ (manual copy required)")
		} else {
			fmt.Println("✓")
		}
	} else {
		// Create placeholder
		goPackDir := filepath.Join(exercisesDest, "go-fundamentals")
		if err := os.MkdirAll(goPackDir, 0755); err == nil {
			fmt.Println("✓ (placeholder created)")
		}
	}

	// 4. Configure LLM provider
	fmt.Println()
	fmt.Println("LLM Provider Setup")
	fmt.Println("------------------")
	fmt.Println("Temper supports: Claude (Anthropic), OpenAI, and Ollama (local)")
	fmt.Println()

	// Load current config to check existing keys
	cfg, _ := config.LoadLocalConfig()

	// Claude
	if cfg != nil && cfg.LLM.Providers["claude"] != nil && cfg.LLM.Providers["claude"].APIKey != "" {
		fmt.Println("Claude API key: already configured ✓")
	} else {
		fmt.Print("Enter Claude API key (or press Enter to skip): ")
		key, _ := reader.ReadString('\n')
		key = strings.TrimSpace(key)
		if key != "" {
			secrets := map[string]string{"claude": key}
			if err := config.SaveSecrets(secrets); err != nil {
				fmt.Printf("  ⚠ Failed to save: %v\n", err)
			} else {
				fmt.Println("  ✓ Saved")
			}
		}
	}

	// 5. Check Docker
	fmt.Println()
	fmt.Print("Checking Docker... ")
	if err := checkDocker(); err != nil {
		fmt.Println("⚠ Not available (local execution will be used)")
	} else {
		fmt.Println("✓")
	}

	// 6. Summary
	fmt.Println()
	fmt.Println("Setup Complete!")
	fmt.Println("===============")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. temper start          # Start the daemon")
	fmt.Println("  2. temper doctor         # Verify configuration")
	fmt.Println("  3. temper exercise list  # See available exercises")
	fmt.Println()
	fmt.Println("For IDE integration:")
	fmt.Println("  - VS Code: Install the Temper extension from editors/vscode/")
	fmt.Println("  - Neovim:  Add the plugin from editors/nvim/")
	fmt.Println("  - Cursor:  Configure MCP with 'temper mcp' command")

	return nil
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// cmdDoctor checks system requirements
func cmdDoctor() error {
	fmt.Println("Checking system requirements...")

	allGood := true

	// Check Docker
	fmt.Print("Docker:    ")
	if err := checkDocker(); err != nil {
		fmt.Printf("✗ %v\n", err)
		allGood = false
	} else {
		fmt.Println("✓ available")
	}

	// Check temper directory
	fmt.Print("Directory: ")
	temperDir, err := config.TemperDir()
	if err != nil {
		fmt.Printf("✗ %v\n", err)
		allGood = false
	} else if _, err := os.Stat(temperDir); os.IsNotExist(err) {
		fmt.Println("✗ not created (run 'temper start' to create)")
		allGood = false
	} else {
		fmt.Printf("✓ %s\n", temperDir)
	}

	// Check config
	fmt.Print("Config:    ")
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		fmt.Printf("✗ %v\n", err)
		allGood = false
	} else {
		fmt.Println("✓ loaded")

		// Check LLM providers
		fmt.Println("\nLLM Providers:")
		for name, provider := range cfg.LLM.Providers {
			if !provider.Enabled {
				continue
			}

			fmt.Printf("  %s: ", name)
			if name == "ollama" {
				// Check Ollama connectivity
				if err := checkOllama(provider.URL); err != nil {
					fmt.Printf("✗ %v\n", err)
				} else {
					fmt.Printf("✓ available (model: %s)\n", provider.Model)
				}
			} else if provider.APIKey != "" {
				fmt.Printf("✓ configured (model: %s)\n", provider.Model)
			} else {
				fmt.Printf("✗ no API key (run 'temper provider set-key %s')\n", name)
			}
		}
	}

	// Check daemon status
	fmt.Print("\nDaemon:    ")
	if isRunning() {
		fmt.Println("✓ running")
	} else {
		fmt.Println("✗ not running (run 'temper start')")
	}

	fmt.Println()
	if allGood {
		fmt.Println("All checks passed! ✓")
	} else {
		fmt.Println("Some checks failed. Please fix the issues above.")
	}

	return nil
}

// cmdConfig shows current configuration
func cmdConfig() error {
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Println("Temper Configuration")

	fmt.Println("Daemon:")
	fmt.Printf("  bind: %s:%d\n", cfg.Daemon.Bind, cfg.Daemon.Port)
	fmt.Printf("  log_level: %s\n", cfg.Daemon.LogLevel)

	fmt.Println("\nLLM:")
	fmt.Printf("  default_provider: %s\n", cfg.LLM.DefaultProvider)
	for name, provider := range cfg.LLM.Providers {
		if provider.Enabled {
			hasKey := provider.APIKey != "" || name == "ollama"
			keyStatus := "✗"
			if hasKey {
				keyStatus = "✓"
			}
			fmt.Printf("  %s: enabled=%t model=%s key=%s\n", name, provider.Enabled, provider.Model, keyStatus)
		}
	}

	fmt.Println("\nLearning Contract:")
	fmt.Printf("  default_track: %s\n", cfg.Learning.DefaultTrack)
	for name, track := range cfg.Learning.Tracks {
		fmt.Printf("  %s: max_level=L%d cooldown=%ds\n", name, track.MaxLevel, track.CooldownSeconds)
	}

	fmt.Println("\nRunner:")
	fmt.Printf("  executor: %s\n", cfg.Runner.Executor)
	if cfg.Runner.Executor == "docker" {
		fmt.Printf("  image: %s\n", cfg.Runner.Docker.Image)
		fmt.Printf("  memory: %dMB\n", cfg.Runner.Docker.MemoryMB)
		fmt.Printf("  timeout: %ds\n", cfg.Runner.Docker.TimeoutSeconds)
	}

	temperDir, _ := config.TemperDir()
	fmt.Printf("\nConfig path: %s/config.yaml\n", temperDir)

	return nil
}

// cmdProvider manages LLM provider API keys
func cmdProvider(args []string) error {
	if len(args) < 1 {
		fmt.Println(`Provider management commands:

  temper provider list              List configured providers
  temper provider set-key <name>    Set API key for a provider`)
		return nil
	}

	switch args[0] {
	case "list":
		return cmdProviderList()
	case "set-key":
		if len(args) < 2 {
			return fmt.Errorf("provider name required")
		}
		return cmdProviderSetKey(args[1])
	default:
		return fmt.Errorf("unknown provider command: %s", args[0])
	}
}

func cmdProviderList() error {
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Println("Configured LLM Providers:")
	for name, provider := range cfg.LLM.Providers {
		status := "disabled"
		if provider.Enabled {
			if provider.APIKey != "" || name == "ollama" {
				status = "ready"
			} else {
				status = "needs API key"
			}
		}

		isDefault := ""
		if name == cfg.LLM.DefaultProvider {
			isDefault = " (default)"
		}

		fmt.Printf("  %s%s\n", name, isDefault)
		fmt.Printf("    status: %s\n", status)
		fmt.Printf("    model:  %s\n", provider.Model)
		if name == "ollama" && provider.URL != "" {
			fmt.Printf("    url:    %s\n", provider.URL)
		}
		fmt.Println()
	}

	return nil
}

func cmdProviderSetKey(provider string) error {
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Check if provider exists
	if _, ok := cfg.LLM.Providers[provider]; !ok {
		return fmt.Errorf("unknown provider: %s (valid: claude, openai, ollama)", provider)
	}

	if provider == "ollama" {
		fmt.Println("Ollama doesn't require an API key.")
		return nil
	}

	// Prompt for API key
	fmt.Printf("Enter %s API key: ", provider)
	reader := bufio.NewReader(os.Stdin)
	key, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	key = strings.TrimSpace(key)

	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Load existing secrets and update
	secrets := make(map[string]string)
	secrets[provider] = key

	if err := config.SaveSecrets(secrets); err != nil {
		return fmt.Errorf("save secrets: %w", err)
	}

	fmt.Printf("✓ API key saved for %s\n", provider)
	fmt.Println("Restart the daemon for changes to take effect.")
	return nil
}
