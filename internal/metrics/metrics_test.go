package metrics

import (
	"strings"
	"sync"
	"testing"
)

func TestCounter_Inc_DefaultLabels(t *testing.T) {
	r := New()
	c := r.Counter("hint_requests_total", "total hint requests")

	c.Inc(nil)
	c.Inc(nil)
	c.Inc(nil)

	out := r.Format()
	if !strings.Contains(out, "hint_requests_total 3\n") {
		t.Errorf("expected counter=3, got:\n%s", out)
	}
	if !strings.Contains(out, "# TYPE hint_requests_total counter") {
		t.Errorf("missing TYPE line:\n%s", out)
	}
}

func TestCounter_AddWithLabels(t *testing.T) {
	r := New()
	c := r.Counter("requests_total", "")

	c.Inc(map[string]string{"intent": "hint", "level": "1"})
	c.Inc(map[string]string{"intent": "hint", "level": "1"})
	c.Inc(map[string]string{"intent": "stuck", "level": "2"})

	out := r.Format()
	if !strings.Contains(out, `requests_total{intent="hint",level="1"} 2`) {
		t.Errorf("missing hint/L1=2:\n%s", out)
	}
	if !strings.Contains(out, `requests_total{intent="stuck",level="2"} 1`) {
		t.Errorf("missing stuck/L2=1:\n%s", out)
	}
}

func TestGauge_SetAndAdd(t *testing.T) {
	r := New()
	g := r.Gauge("sandbox_active", "currently active sandbox count")

	g.Set(5)
	g.Add(2)
	g.Add(-1)

	out := r.Format()
	if !strings.Contains(out, "sandbox_active 6") {
		t.Errorf("expected gauge=6, got:\n%s", out)
	}
	if !strings.Contains(out, "# TYPE sandbox_active gauge") {
		t.Errorf("missing TYPE gauge:\n%s", out)
	}
}

func TestRegistry_DeterministicOutput(t *testing.T) {
	r := New()
	c1 := r.Counter("a_total", "a help")
	c1.Inc(nil)
	c2 := r.Counter("z_total", "z help")
	c2.Inc(nil)
	g := r.Gauge("m_value", "m help")
	g.Set(42)

	a := r.Format()
	b := r.Format()
	if a != b {
		t.Errorf("Format() not deterministic")
	}

	// Lines should be sorted: a_total first, m_value second, z_total last.
	idxA := strings.Index(a, "a_total")
	idxM := strings.Index(a, "m_value")
	idxZ := strings.Index(a, "z_total")
	if !(idxA < idxM && idxM < idxZ) {
		t.Errorf("Format() not sorted: a=%d m=%d z=%d", idxA, idxM, idxZ)
	}
}

func TestCounter_ConcurrentSafe(t *testing.T) {
	r := New()
	c := r.Counter("concurrent_total", "")

	const goroutines = 50
	const incs = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incs; j++ {
				c.Inc(map[string]string{"k": "v"})
			}
		}()
	}
	wg.Wait()

	want := goroutines * incs
	out := r.Format()
	expected := strings.Contains(out, "concurrent_total{k=\"v\"} 10000")
	if !expected {
		t.Errorf("expected concurrent_total=%d, got:\n%s", want, out)
	}
}

func TestEmptyRegistry_FormatIsEmpty(t *testing.T) {
	r := New()
	if got := r.Format(); got != "" {
		t.Errorf("empty registry should format empty, got:\n%s", got)
	}
}
