package daemon

import (
	"fmt"
	"net/http"

	"github.com/felixgeelhaar/temper/internal/pairing"
)

// handleMetrics emits Prometheus-style text from the in-process registry,
// merging pairing.ClampViolations() (counter that lives at package scope
// in pairing for easy increment from Validate) into the body so the SLO
// "clamp_violation_rate" can be computed from a single endpoint.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	if s.metrics != nil {
		fmt.Fprint(w, s.metrics.Format())
	}

	// Surface clamp violations alongside other metrics. The pairing
	// validator increments a package-level atomic; mirror it here so a
	// single /v1/metrics scrape captures it.
	fmt.Fprintf(w,
		"# HELP clamp_violations_total Output-side clamp validator rejections, including retries.\n"+
			"# TYPE clamp_violations_total counter\n"+
			"clamp_violations_total %d\n",
		pairing.ClampViolations(),
	)
}
