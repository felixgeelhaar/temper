// Package archtest provides architectural-fitness helpers usable from
// `go test` so the design rules in docs/architecture.md are enforced
// continuously rather than by code review alone. Hand-rolled because
// the rules we need (a handful of import-graph constraints) are small
// and the alternative dependency (go-arch-lint) carries more weight than
// it earns.
package archtest

import (
	"fmt"
	"go/build"
	"strings"
)

// PackageImports returns the unique set of import paths that the given
// package's non-test files declare. Test files are excluded so build
// constraints like //go:build integration do not change the result.
func PackageImports(pkgPath string) ([]string, error) {
	pkg, err := build.Default.Import(pkgPath, "", 0)
	if err != nil {
		return nil, fmt.Errorf("import %s: %w", pkgPath, err)
	}
	seen := make(map[string]struct{}, len(pkg.Imports))
	for _, imp := range pkg.Imports {
		seen[imp] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for imp := range seen {
		out = append(out, imp)
	}
	return out, nil
}

// ForbidImport asserts that pkgPath does not import any path that
// matches forbidden. Returns the offending import paths (empty when
// the rule holds). The matcher uses strings.HasPrefix so callers can
// pass a module-relative prefix like
// "github.com/felixgeelhaar/temper/internal/daemon".
func ForbidImport(pkgPath, forbiddenPrefix string) ([]string, error) {
	imports, err := PackageImports(pkgPath)
	if err != nil {
		return nil, err
	}
	var hits []string
	for _, imp := range imports {
		if strings.HasPrefix(imp, forbiddenPrefix) {
			hits = append(hits, imp)
		}
	}
	return hits, nil
}

// AllowedInternalImports asserts that the only internal/* packages the
// pkgPath imports are within the allowed set. Returns offending paths.
// Used for clean-architecture rules like "domain depends on no other
// internal package".
func AllowedInternalImports(pkgPath string, allowed []string) ([]string, error) {
	imports, err := PackageImports(pkgPath)
	if err != nil {
		return nil, err
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = struct{}{}
	}
	var violations []string
	const internalPrefix = "github.com/felixgeelhaar/temper/internal/"
	for _, imp := range imports {
		if !strings.HasPrefix(imp, internalPrefix) {
			continue
		}
		if _, ok := allowedSet[imp]; !ok {
			violations = append(violations, imp)
		}
	}
	return violations, nil
}
