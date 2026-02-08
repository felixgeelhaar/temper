<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue';
import * as d3 from 'd3';

interface TimelineEvent {
  type: 'run' | 'intervention';
  timestamp: string;
  label: string;
  level?: number;
  passed?: boolean;
}

const props = defineProps<{
  events: TimelineEvent[];
}>();

const svgRef = ref<SVGSVGElement | null>(null);

function draw() {
  const el = svgRef.value;
  if (!el || props.events.length === 0) return;

  const width = el.clientWidth || 600;
  const rowH = 36;
  const height = Math.max(props.events.length * rowH + 40, 100);
  const margin = { top: 10, right: 20, bottom: 10, left: 100 };
  const innerW = width - margin.left - margin.right;

  const svg = d3.select(el);
  svg.selectAll('*').remove();
  svg.attr('viewBox', `0 0 ${width} ${height}`);

  const g = svg
    .append('g')
    .attr('transform', `translate(${margin.left},${margin.top})`);

  const data = props.events.map((e, i) => ({
    ...e,
    date: new Date(e.timestamp),
    index: i,
  }));

  const colorMap: Record<string, string> = {
    run: '#a6da95',
    intervention: '#c6a0f6',
  };

  // Timeline line
  g.append('line')
    .attr('x1', 0)
    .attr('y1', 0)
    .attr('x2', 0)
    .attr('y2', data.length * rowH)
    .attr('stroke', '#2a2d3d')
    .attr('stroke-width', 2);

  // Events
  data.forEach((d, i) => {
    const y = i * rowH + rowH / 2;
    const color = colorMap[d.type] ?? '#8aadf4';

    // Dot
    g.append('circle')
      .attr('cx', 0)
      .attr('cy', y)
      .attr('r', 5)
      .attr('fill', color)
      .attr('stroke', '#161822')
      .attr('stroke-width', 2);

    // Time label (left)
    const fmt = d3.timeFormat('%H:%M:%S');
    g.append('text')
      .attr('x', -12)
      .attr('y', y)
      .attr('text-anchor', 'end')
      .attr('dominant-baseline', 'central')
      .attr('fill', '#6e738d')
      .attr('font-size', '10px')
      .attr('font-family', 'var(--font-mono)')
      .text(fmt(d.date));

    // Event label (right)
    g.append('text')
      .attr('x', 16)
      .attr('y', y)
      .attr('dominant-baseline', 'central')
      .attr('fill', '#cad3f5')
      .attr('font-size', '12px')
      .text(d.label);

    // Level badge for interventions
    if (d.type === 'intervention' && d.level !== undefined) {
      const textLen = d.label.length * 7 + 24;
      g.append('rect')
        .attr('x', textLen)
        .attr('y', y - 8)
        .attr('width', 24)
        .attr('height', 16)
        .attr('rx', 8)
        .attr('fill', `${color}20`);

      g.append('text')
        .attr('x', textLen + 12)
        .attr('y', y)
        .attr('text-anchor', 'middle')
        .attr('dominant-baseline', 'central')
        .attr('fill', color)
        .attr('font-size', '9px')
        .attr('font-weight', '700')
        .text(`L${d.level}`);
    }

    // Pass/fail indicator for runs
    if (d.type === 'run') {
      const indicator = d.passed ? '\u2713' : '\u2717';
      const indColor = d.passed ? '#a6da95' : '#ed8796';
      const textLen = d.label.length * 7 + 24;
      g.append('text')
        .attr('x', textLen)
        .attr('y', y)
        .attr('dominant-baseline', 'central')
        .attr('fill', indColor)
        .attr('font-size', '14px')
        .attr('font-weight', '700')
        .text(indicator);
    }
  });
}

let observer: ResizeObserver | null = null;

onMounted(() => {
  draw();
  if (svgRef.value) {
    observer = new ResizeObserver(draw);
    observer.observe(svgRef.value);
  }
});

onUnmounted(() => {
  observer?.disconnect();
});

watch(() => props.events, draw, { deep: true });
</script>

<template>
  <div class="session-timeline card">
    <h3 class="timeline-title">Session Timeline</h3>
    <div class="timeline-chart">
      <svg v-if="events.length > 0" ref="svgRef" class="timeline-svg" />
      <p v-else class="empty-state">No events in this session yet.</p>
    </div>
  </div>
</template>

<style scoped>
.session-timeline {
  min-height: 200px;
}

.timeline-title {
  font-size: 0.875rem;
  font-weight: 600;
  margin-bottom: 1rem;
  color: var(--color-text);
}

.timeline-chart {
  overflow-y: auto;
  max-height: 400px;
}

.timeline-svg {
  width: 100%;
  height: auto;
}

.empty-state {
  color: var(--color-text-muted);
  text-align: center;
  padding: 2rem 0;
  font-size: 0.875rem;
}
</style>
