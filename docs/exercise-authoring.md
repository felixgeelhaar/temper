# Exercise Authoring Guide

Create custom exercise packs for Temper to teach any programming concept.

## Quick Start

```bash
# Create a new pack directory
mkdir -p exercises/my-pack/basics

# Create pack manifest
cat > exercises/my-pack/pack.yaml << 'EOF'
id: my-pack
name: My Learning Pack
version: 1.0.0
description: Learn through deliberate practice
language: go  # or python, typescript, rust
difficulty_range: [beginner, intermediate]
default_policy:
  max_level: 3
  patching_enabled: false
exercises:
  - basics/first-exercise
EOF

# Create your first exercise
# (see exercise format below)
```

## Directory Structure

```
exercises/
  my-pack/
    pack.yaml              # Pack manifest
    basics/
      first-exercise.yaml  # Exercise definition
      second-exercise.yaml
    intermediate/
      harder-exercise.yaml
```

## Pack Manifest (`pack.yaml`)

```yaml
id: my-pack                    # Unique identifier (lowercase, hyphens)
name: My Learning Pack         # Human-readable name
version: 1.0.0                 # Semantic version
description: |
  Multi-line description of what this pack teaches.

language: go                   # go | python | typescript | rust
difficulty_range:
  - beginner
  - intermediate
  - advanced

default_policy:
  max_level: 3                 # Default max intervention level (0-5)
  patching_enabled: false      # Allow code patches?
  cooldown_seconds: 60         # Time between hint requests
  track: practice              # practice | interview-prep

exercises:
  - basics/hello-world         # Relative paths to exercise yamls
  - basics/variables
  - intermediate/structs
```

## Exercise Format

Each exercise is a YAML file with these sections:

### Metadata

```yaml
id: hello-world                # Unique within pack
title: Hello World             # Display name
description: |
  ## Learning Objectives
  - First objective
  - Second objective

  ## Instructions
  What the user should do.

difficulty: beginner           # beginner | intermediate | advanced
tags:
  - basics
  - functions
prerequisites:                 # Other exercise IDs (optional)
  - basics/variables
```

### Starter Code

Code the user begins with. Use meaningful TODOs.

```yaml
starter:
  main.go: |
    package main

    // Hello returns a greeting for the given name.
    // TODO: Implement this function
    func Hello(name string) string {
        return ""
    }

  helper.go: |                 # Multiple files supported
    package main

    func helper() {}
```

### Tests

Validation tests that must pass. Hidden from user by default.

```yaml
tests:
  main_test.go: |
    package main

    import "testing"

    func TestHello(t *testing.T) {
        tests := []struct {
            name     string
            input    string
            expected string
        }{
            {"empty", "", "Hello, World!"},
            {"named", "Go", "Hello, Go!"},
        }

        for _, tc := range tests {
            t.Run(tc.name, func(t *testing.T) {
                got := Hello(tc.input)
                if got != tc.expected {
                    t.Errorf("got %q, want %q", got, tc.expected)
                }
            })
        }
    }
```

### Check Recipe

How to validate the exercise.

```yaml
check_recipe:
  format: true                 # Run formatter (gofmt, black, etc)
  build: true                  # Compile the code
  test: true                   # Run tests
  test_flags:
    - "-v"                     # Verbose test output
    - "-race"                  # Race detector (Go)
  lint: false                  # Run linter
  timeout: 30                  # Seconds before timeout
```

### Rubric

Scoring criteria for feedback.

```yaml
rubric:
  criteria:
    - id: correctness
      name: Correctness
      description: All tests pass
      weight: 0.5              # Must sum to 1.0
      signals:
        - all_tests_pass

    - id: style
      name: Code Style
      description: Follows conventions
      weight: 0.2
      signals:
        - gofmt_clean
        - no_unused_imports

    - id: idioms
      name: Idiomatic Code
      description: Uses language best practices
      weight: 0.3
      signals:
        - uses_table_tests
        - proper_error_handling
```

### Hints (Intervention Levels)

Progressive hints from vague to specific.

```yaml
hints:
  L0:                          # Clarifying questions only
    - "What value should be returned when the input is empty?"
    - "Have you considered edge cases?"

  L1:                          # Category/direction hints
    - "Look at string formatting functions in the standard library"
    - "The fmt package has what you need"

  L2:                          # Location + concept
    - "Use fmt.Sprintf to format strings with placeholders"
    - "Check if name is empty with name == \"\""

  L3:                          # Constrained snippets
    - |
      The structure should look like:
      ```go
      if name == "" {
          name = "World"
      }
      return fmt.Sprintf("Hello, %s!", name)
      ```
```

### Solution

Reference solution (gated behind L5).

```yaml
solution:
  main.go: |
    package main

    import "fmt"

    func Hello(name string) string {
        if name == "" {
            name = "World"
        }
        return fmt.Sprintf("Hello, %s!", name)
    }
```

## Language-Specific Details

### Go

```yaml
check_recipe:
  format: true      # gofmt
  build: true       # go build
  test: true        # go test
  lint: true        # golangci-lint (if available)
```

### Python

```yaml
check_recipe:
  format: true      # black
  build: false      # No compilation
  test: true        # pytest
  lint: true        # ruff (if available)
```

### TypeScript

```yaml
check_recipe:
  format: true      # prettier
  build: true       # tsc
  test: true        # vitest or jest
  lint: true        # eslint (if available)
```

### Rust

```yaml
check_recipe:
  format: true      # rustfmt
  build: true       # cargo build
  test: true        # cargo test
  lint: true        # clippy (if available)
```

## Complete Example

Here's a complete intermediate exercise:

```yaml
id: error-handling
title: Custom Error Types
description: |
  ## Learning Objectives
  - Define custom error types
  - Implement the error interface
  - Use errors.Is and errors.As

  ## Instructions
  Create a `ValidationError` type that implements the `error` interface.
  The `Validate` function should return this error for invalid inputs.

difficulty: intermediate
tags:
  - errors
  - interfaces
prerequisites:
  - intermediate/interfaces

starter:
  validator.go: |
    package validator

    // ValidationError represents a validation failure.
    // TODO: Add fields for Field and Message
    type ValidationError struct {
    }

    // Error implements the error interface.
    // TODO: Return a formatted error message
    func (e *ValidationError) Error() string {
        return ""
    }

    // Validate checks if the input is valid.
    // Returns ValidationError if:
    // - input is empty (field: "input", message: "cannot be empty")
    // - input is longer than 100 chars (field: "input", message: "too long")
    func Validate(input string) error {
        // TODO: Implement validation
        return nil
    }

tests:
  validator_test.go: |
    package validator

    import (
        "errors"
        "testing"
    )

    func TestValidate(t *testing.T) {
        t.Run("valid input returns nil", func(t *testing.T) {
            err := Validate("hello")
            if err != nil {
                t.Errorf("expected nil, got %v", err)
            }
        })

        t.Run("empty input returns ValidationError", func(t *testing.T) {
            err := Validate("")
            var ve *ValidationError
            if !errors.As(err, &ve) {
                t.Error("expected ValidationError")
            }
            if ve.Field != "input" {
                t.Errorf("Field = %q, want %q", ve.Field, "input")
            }
        })

        t.Run("long input returns ValidationError", func(t *testing.T) {
            long := string(make([]byte, 101))
            err := Validate(long)
            var ve *ValidationError
            if !errors.As(err, &ve) {
                t.Error("expected ValidationError")
            }
        })
    }

check_recipe:
  format: true
  build: true
  test: true
  test_flags: ["-v"]
  timeout: 30

rubric:
  criteria:
    - id: correctness
      name: Correctness
      description: All tests pass
      weight: 0.5
      signals:
        - all_tests_pass
    - id: error_interface
      name: Error Interface
      description: Properly implements error interface
      weight: 0.3
      signals:
        - implements_error
    - id: struct_design
      name: Struct Design
      description: ValidationError has appropriate fields
      weight: 0.2
      signals:
        - has_field_and_message

hints:
  L0:
    - "What fields does a validation error need to be useful?"
    - "What interface must error types implement?"
  L1:
    - "The error interface requires an Error() string method"
    - "Your struct needs Field and Message string fields"
  L2:
    - "Return &ValidationError{Field: \"input\", Message: \"...\"}"
    - "Use len(input) to check length"
  L3:
    - |
      ```go
      type ValidationError struct {
          Field   string
          Message string
      }

      func (e *ValidationError) Error() string {
          return fmt.Sprintf("%s: %s", e.Field, e.Message)
      }
      ```

solution:
  validator.go: |
    package validator

    import "fmt"

    type ValidationError struct {
        Field   string
        Message string
    }

    func (e *ValidationError) Error() string {
        return fmt.Sprintf("%s: %s", e.Field, e.Message)
    }

    func Validate(input string) error {
        if input == "" {
            return &ValidationError{Field: "input", Message: "cannot be empty"}
        }
        if len(input) > 100 {
            return &ValidationError{Field: "input", Message: "too long"}
        }
        return nil
    }
```

## Best Practices

### Exercise Design

1. **One concept per exercise** - Focus on a single learning objective
2. **Progressive difficulty** - Use prerequisites to build knowledge
3. **Clear instructions** - State exactly what needs to be implemented
4. **Meaningful tests** - Tests should guide toward the solution
5. **Good hints** - Each level should add value without giving away the answer

### Hint Quality

| Level | Purpose | Example |
|-------|---------|---------|
| L0 | Clarify requirements | "What should happen when X?" |
| L1 | Point to resources | "Look at the fmt package" |
| L2 | Specific guidance | "Use fmt.Sprintf with %s placeholder" |
| L3 | Partial structure | Code skeleton without full solution |

### Testing Your Exercises

```bash
# Start a session with your exercise
temper start my-pack/basics/first-exercise

# Test the starter code fails
temper run

# Implement the solution
# Test it passes
temper run

# Verify hints make sense
temper hint
temper hint  # Get next level
```

## Contributing Exercises

1. Fork the repository
2. Create your pack in `exercises/`
3. Test all exercises locally
4. Submit a pull request

### Review Checklist

- [ ] Pack manifest is valid YAML
- [ ] All exercises load without errors
- [ ] Tests pass with provided solutions
- [ ] Hints are progressive and helpful
- [ ] Difficulty ratings are accurate
- [ ] Prerequisites are correct
