package archtest

import "testing"

// TestDomainHasNoInternalDependencies asserts the clean-architecture rule
// stated in docs/architecture.md: internal/domain is the innermost layer
// and must not import any other internal package. Violations represent
// upward dependencies that break the dependency rule.
func TestDomainHasNoInternalDependencies(t *testing.T) {
	violations, err := AllowedInternalImports(
		"github.com/felixgeelhaar/temper/internal/domain",
		nil, // empty allowlist
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("internal/domain must not import other internal packages, but imports: %v", violations)
	}
}

// TestPairingDoesNotImportDaemon asserts that the pairing engine stays
// oblivious to the HTTP layer. If a future change drags daemon types
// into pairing the test fails immediately, before review.
func TestPairingDoesNotImportDaemon(t *testing.T) {
	hits, err := ForbidImport(
		"github.com/felixgeelhaar/temper/internal/pairing",
		"github.com/felixgeelhaar/temper/internal/daemon",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("internal/pairing must not import internal/daemon, found: %v", hits)
	}
}

// TestLLMDoesNotImportDaemonOrPairing keeps the provider layer reusable
// outside the daemon/pairing machinery. The reverse direction is fine
// (pairing imports llm) but llm should depend only on domain + helpers.
func TestLLMDoesNotImportDaemonOrPairing(t *testing.T) {
	for _, forbidden := range []string{
		"github.com/felixgeelhaar/temper/internal/daemon",
		"github.com/felixgeelhaar/temper/internal/pairing",
	} {
		hits, err := ForbidImport(
			"github.com/felixgeelhaar/temper/internal/llm",
			forbidden,
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 0 {
			t.Errorf("internal/llm must not import %s, found: %v", forbidden, hits)
		}
	}
}

// TestDomainDoesNotImportSpecOrDaemon catches another common drift —
// the domain "leaking" into framework-flavored packages.
func TestDomainDoesNotImportSpecOrDaemon(t *testing.T) {
	for _, forbidden := range []string{
		"github.com/felixgeelhaar/temper/internal/daemon",
		"github.com/felixgeelhaar/temper/internal/spec",
		"github.com/felixgeelhaar/temper/internal/runner",
		"github.com/felixgeelhaar/temper/internal/sandbox",
	} {
		hits, err := ForbidImport(
			"github.com/felixgeelhaar/temper/internal/domain",
			forbidden,
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 0 {
			t.Errorf("internal/domain must not import %s, found: %v", forbidden, hits)
		}
	}
}

// TestCorrelationIsLeaf — the new internal/correlation package is meant
// to be a leaf with no dependencies on other internal packages.
func TestCorrelationIsLeaf(t *testing.T) {
	violations, err := AllowedInternalImports(
		"github.com/felixgeelhaar/temper/internal/correlation",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("internal/correlation must remain a leaf, but imports: %v", violations)
	}
}

// TestMetricsIsLeaf — same rule for the metrics registry.
func TestMetricsIsLeaf(t *testing.T) {
	violations, err := AllowedInternalImports(
		"github.com/felixgeelhaar/temper/internal/metrics",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("internal/metrics must remain a leaf, but imports: %v", violations)
	}
}
