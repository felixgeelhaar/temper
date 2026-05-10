// Package metrics is a tiny in-process metrics registry that emits the
// Prometheus text exposition format. Hand-rolled rather than depending on
// prometheus/client_golang to keep the daemon zero-extra-dep; the surface
// area we need (counters, gauges, simple histograms) is small.
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Registry holds the live metrics. Safe for concurrent use; counters and
// gauges use atomics on their hot path.
type Registry struct {
	mu       sync.RWMutex
	counters map[string]*counter
	gauges   map[string]*gauge
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{
		counters: make(map[string]*counter),
		gauges:   make(map[string]*gauge),
	}
}

// Counter returns (creating if needed) the counter for the given name.
// Counters are monotonic; use AddCounter to increment.
func (r *Registry) Counter(name, help string) *counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &counter{name: name, help: help, series: map[string]*counterSeries{}}
	r.counters[name] = c
	return c
}

// Gauge returns (creating if needed) the gauge for the given name. Gauges
// can go up and down; use Set/Add.
func (r *Registry) Gauge(name, help string) *gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g := &gauge{name: name, help: help}
	r.gauges[name] = g
	return g
}

// Format emits the registry contents in Prometheus text exposition format.
// Output is deterministic (metrics + label combinations sorted) so tests
// can assert exact wire shape.
func (r *Registry) Format() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	names := make([]string, 0, len(r.counters)+len(r.gauges))
	for n := range r.counters {
		names = append(names, n)
	}
	for n := range r.gauges {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, n := range names {
		if c, ok := r.counters[n]; ok {
			fmt.Fprintf(&sb, "# HELP %s %s\n", c.name, c.help)
			fmt.Fprintf(&sb, "# TYPE %s counter\n", c.name)
			c.format(&sb)
		}
		if g, ok := r.gauges[n]; ok {
			fmt.Fprintf(&sb, "# HELP %s %s\n", g.name, g.help)
			fmt.Fprintf(&sb, "# TYPE %s gauge\n", g.name)
			fmt.Fprintf(&sb, "%s %d\n", g.name, g.value.Load())
		}
	}
	return sb.String()
}

type counter struct {
	name   string
	help   string
	mu     sync.RWMutex
	series map[string]*counterSeries
}

type counterSeries struct {
	labelKey string
	labels   map[string]string
	value    atomic.Int64
}

// Add increments the counter by n for the given label set. Pass an empty
// map for an unlabeled counter.
func (c *counter) Add(labels map[string]string, n int64) {
	key := encodeLabels(labels)
	c.mu.RLock()
	s, ok := c.series[key]
	c.mu.RUnlock()
	if !ok {
		c.mu.Lock()
		if s, ok = c.series[key]; !ok {
			s = &counterSeries{labelKey: key, labels: copyLabels(labels)}
			c.series[key] = s
		}
		c.mu.Unlock()
	}
	s.value.Add(n)
}

// Inc adds 1 to the counter for the given label set.
func (c *counter) Inc(labels map[string]string) {
	c.Add(labels, 1)
}

func (c *counter) format(sb *strings.Builder) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.series))
	for k := range c.series {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := c.series[k]
		if len(s.labels) == 0 {
			fmt.Fprintf(sb, "%s %d\n", c.name, s.value.Load())
			continue
		}
		fmt.Fprintf(sb, "%s{%s} %d\n", c.name, formatLabelPairs(s.labels), s.value.Load())
	}
}

type gauge struct {
	name  string
	help  string
	value atomic.Int64
}

// Set replaces the gauge value.
func (g *gauge) Set(v int64) { g.value.Store(v) }

// Add adjusts the gauge by delta (may be negative).
func (g *gauge) Add(delta int64) { g.value.Add(delta) }

// encodeLabels produces a stable key per label set used as the series
// map key. The empty label set produces an empty string key.
func encodeLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(labels[k])
		sb.WriteByte(',')
	}
	return sb.String()
}

func copyLabels(labels map[string]string) map[string]string {
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		out[k] = v
	}
	return out
}

func formatLabelPairs(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, labels[k]))
	}
	return strings.Join(parts, ",")
}
