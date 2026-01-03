# Specifications (Specular)

Spec-driven development using the Specular format.

## Creating a Spec

```bash
temper spec create feature-name
```

Creates a YAML file:

```yaml
intent: What you're building and why
goals:
  - Goal 1
  - Goal 2
acceptance:
  - Criteria 1
  - Criteria 2
```

## Validation

```bash
temper spec validate
```

Checks for:
- Completeness
- Placeholder text
- Vague language

## Progress Tracking

```bash
temper spec status
```

Shows acceptance criteria completion.

## Drift Detection

```bash
temper spec lock   # Generate hash
temper spec drift  # Check for changes
```

Ensures spec stays aligned with implementation.
