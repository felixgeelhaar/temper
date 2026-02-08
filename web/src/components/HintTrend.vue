<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue';
import * as d3 from 'd3';

const props = defineProps<{
  trend: { date: string; average_intervention_level: number; sessions: number }[];
}>();

const svgRef = ref<SVGSVGElement | null>(null);

function draw() {
  const el = svgRef.value;
  if (!el || props.trend.length === 0) return;

  const width = el.clientWidth || 600;
  const height = 250;
  const margin = { top: 20, right: 50, bottom: 30, left: 40 };
  const innerW = width - margin.left - margin.right;
  const innerH = height - margin.top - margin.bottom;

  const svg = d3.select(el);
  svg.selectAll('*').remove();
  svg.attr('viewBox', `0 0 ${width} ${height}`);

  const g = svg
    .append('g')
    .attr('transform', `translate(${margin.left},${margin.top})`);

  const data = props.trend.map((d) => ({
    date: new Date(d.date),
    level: d.average_intervention_level,
    sessions: d.sessions,
  }));

  // Scales
  const x = d3
    .scaleTime()
    .domain(d3.extent(data, (d) => d.date) as [Date, Date])
    .range([0, innerW]);

  const yLevel = d3.scaleLinear().domain([0, 5]).range([innerH, 0]);

  const maxSessions = d3.max(data, (d) => d.sessions) ?? 5;
  const ySessions = d3
    .scaleLinear()
    .domain([0, Math.max(maxSessions, 1)])
    .range([innerH, 0]);

  // Grid
  g.append('g')
    .attr('class', 'grid')
    .call(
      d3
        .axisLeft(yLevel)
        .tickSize(-innerW)
        .tickFormat(() => '')
        .ticks(5)
    )
    .selectAll('line')
    .attr('stroke', '#2a2d3d');

  g.selectAll('.grid .domain').remove();

  // Axes
  g.append('g')
    .attr('transform', `translate(0,${innerH})`)
    .call(
      d3
        .axisBottom(x)
        .ticks(6)
        .tickFormat((d) => d3.timeFormat('%-m/%-d')(d as Date))
    )
    .selectAll('text')
    .attr('fill', '#6e738d')
    .attr('font-size', '10px');

  g.selectAll('.domain').attr('stroke', '#2a2d3d');
  g.selectAll('.tick line').attr('stroke', '#2a2d3d');

  g.append('g')
    .call(d3.axisLeft(yLevel).ticks(5))
    .selectAll('text')
    .attr('fill', '#6e738d')
    .attr('font-size', '10px');

  g.append('g')
    .attr('transform', `translate(${innerW},0)`)
    .call(d3.axisRight(ySessions).ticks(5))
    .selectAll('text')
    .attr('fill', '#6e738d')
    .attr('font-size', '10px');

  // Axis labels
  g.append('text')
    .attr('x', -10)
    .attr('y', -8)
    .attr('fill', '#6e738d')
    .attr('font-size', '9px')
    .text('Level');

  g.append('text')
    .attr('x', innerW + 10)
    .attr('y', -8)
    .attr('fill', '#6e738d')
    .attr('font-size', '9px')
    .text('Sessions');

  // Intervention level area + line
  const areaLevel = d3
    .area<(typeof data)[0]>()
    .x((d) => x(d.date))
    .y0(innerH)
    .y1((d) => yLevel(d.level))
    .curve(d3.curveMonotoneX);

  g.append('path')
    .datum(data)
    .attr('d', areaLevel)
    .attr('fill', 'rgba(198, 160, 246, 0.1)');

  const lineLevel = d3
    .line<(typeof data)[0]>()
    .x((d) => x(d.date))
    .y((d) => yLevel(d.level))
    .curve(d3.curveMonotoneX);

  g.append('path')
    .datum(data)
    .attr('d', lineLevel)
    .attr('fill', 'none')
    .attr('stroke', 'rgba(198, 160, 246, 0.8)')
    .attr('stroke-width', 2);

  // Sessions area + line
  const areaSessions = d3
    .area<(typeof data)[0]>()
    .x((d) => x(d.date))
    .y0(innerH)
    .y1((d) => ySessions(d.sessions))
    .curve(d3.curveMonotoneX);

  g.append('path')
    .datum(data)
    .attr('d', areaSessions)
    .attr('fill', 'rgba(166, 218, 149, 0.08)');

  const lineSessions = d3
    .line<(typeof data)[0]>()
    .x((d) => x(d.date))
    .y((d) => ySessions(d.sessions))
    .curve(d3.curveMonotoneX);

  g.append('path')
    .datum(data)
    .attr('d', lineSessions)
    .attr('fill', 'none')
    .attr('stroke', 'rgba(166, 218, 149, 0.8)')
    .attr('stroke-width', 2);

  // Data points
  g.selectAll('.dot-level')
    .data(data)
    .join('circle')
    .attr('cx', (d) => x(d.date))
    .attr('cy', (d) => yLevel(d.level))
    .attr('r', 3)
    .attr('fill', '#c6a0f6');

  g.selectAll('.dot-sessions')
    .data(data)
    .join('circle')
    .attr('cx', (d) => x(d.date))
    .attr('cy', (d) => ySessions(d.sessions))
    .attr('r', 3)
    .attr('fill', '#a6da95');
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

watch(() => props.trend, draw, { deep: true });
</script>

<template>
  <div class="hint-trend card">
    <h3 class="trend-title">Learning Progress</h3>
    <div class="trend-chart">
      <svg v-if="trend.length > 0" ref="svgRef" class="trend-svg" />
      <p v-else class="empty-state">
        No trend data yet. Complete some sessions to see your progress.
      </p>
    </div>
  </div>
</template>

<style scoped>
.hint-trend {
  min-height: 300px;
}

.trend-title {
  font-size: 0.875rem;
  font-weight: 600;
  margin-bottom: 1rem;
  color: var(--color-text);
}

.trend-chart {
  height: 250px;
}

.trend-svg {
  width: 100%;
  height: 100%;
}

.empty-state {
  color: var(--color-text-muted);
  text-align: center;
  padding: 3rem 0;
  font-size: 0.875rem;
}
</style>
