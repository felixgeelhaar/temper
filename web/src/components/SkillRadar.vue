<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue';
import * as d3 from 'd3';

const props = defineProps<{
  skills: Record<string, number>;
}>();

const svgRef = ref<SVGSVGElement | null>(null);

function draw() {
  const el = svgRef.value;
  if (!el) return;

  const entries = Object.entries(props.skills);
  if (entries.length === 0) return;

  const width = el.clientWidth || 360;
  const height = width;
  const margin = 40;
  const radius = (Math.min(width, height) - margin * 2) / 2;
  const cx = width / 2;
  const cy = height / 2;
  const maxVal = 10;
  const levels = 5;

  const svg = d3.select(el);
  svg.selectAll('*').remove();
  svg.attr('viewBox', `0 0 ${width} ${height}`);

  const g = svg.append('g').attr('transform', `translate(${cx},${cy})`);

  const angleSlice = (Math.PI * 2) / entries.length;

  // Grid circles
  for (let lvl = 1; lvl <= levels; lvl++) {
    const r = (radius / levels) * lvl;
    g.append('circle')
      .attr('r', r)
      .attr('fill', 'none')
      .attr('stroke', '#2a2d3d')
      .attr('stroke-width', 1);

    g.append('text')
      .attr('x', 4)
      .attr('y', -r + 3)
      .text(String((maxVal / levels) * lvl))
      .attr('fill', '#6e738d')
      .attr('font-size', '9px')
      .attr('font-family', 'var(--font-mono)');
  }

  // Axis lines + labels
  entries.forEach(([label], i) => {
    const angle = angleSlice * i - Math.PI / 2;
    const x = Math.cos(angle) * radius;
    const y = Math.sin(angle) * radius;

    g.append('line')
      .attr('x1', 0)
      .attr('y1', 0)
      .attr('x2', x)
      .attr('y2', y)
      .attr('stroke', '#2a2d3d')
      .attr('stroke-width', 1);

    const labelX = Math.cos(angle) * (radius + 18);
    const labelY = Math.sin(angle) * (radius + 18);

    g.append('text')
      .attr('x', labelX)
      .attr('y', labelY)
      .attr('text-anchor', 'middle')
      .attr('dominant-baseline', 'central')
      .text(label)
      .attr('fill', '#cad3f5')
      .attr('font-size', '11px');
  });

  // Data polygon
  const rScale = d3.scaleLinear().domain([0, maxVal]).range([0, radius]);

  const points = entries.map(([, val], i) => {
    const angle = angleSlice * i - Math.PI / 2;
    return [
      Math.cos(angle) * rScale(val),
      Math.sin(angle) * rScale(val),
    ] as [number, number];
  });

  const line = d3
    .lineRadial<[number, number]>()
    .angle((_, i) => angleSlice * i)
    .radius((d) => {
      const angle = angleSlice * points.indexOf(d) - Math.PI / 2;
      return Math.sqrt(d[0] ** 2 + d[1] ** 2);
    })
    .curve(d3.curveLinearClosed);

  // Fill area
  const pathData =
    'M' +
    points.map((p) => `${p[0]},${p[1]}`).join('L') +
    'Z';

  g.append('path')
    .attr('d', pathData)
    .attr('fill', 'rgba(138, 173, 244, 0.15)')
    .attr('stroke', 'rgba(138, 173, 244, 0.8)')
    .attr('stroke-width', 2);

  // Data points
  points.forEach((p) => {
    g.append('circle')
      .attr('cx', p[0])
      .attr('cy', p[1])
      .attr('r', 4)
      .attr('fill', '#8aadf4');
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

watch(() => props.skills, draw, { deep: true });
</script>

<template>
  <div class="skill-radar card">
    <h3 class="radar-title">Skill Map</h3>
    <div class="radar-chart">
      <svg v-if="Object.keys(skills).length > 0" ref="svgRef" class="radar-svg" />
      <p v-else class="empty-state">No skill data yet. Start a session!</p>
    </div>
  </div>
</template>

<style scoped>
.skill-radar {
  min-height: 300px;
}

.radar-title {
  font-size: 0.875rem;
  font-weight: 600;
  margin-bottom: 1rem;
  color: var(--color-text);
}

.radar-chart {
  max-width: 400px;
  margin: 0 auto;
}

.radar-svg {
  width: 100%;
  height: auto;
}

.empty-state {
  color: var(--color-text-muted);
  text-align: center;
  padding: 3rem 0;
  font-size: 0.875rem;
}
</style>
