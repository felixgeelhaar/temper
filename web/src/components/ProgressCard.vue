<script setup lang="ts">
defineProps<{
  label: string;
  value: string | number;
  subtitle?: string;
  trend?: number;
  color?: string;
}>();
</script>

<template>
  <div class="progress-card card">
    <div class="card-label">{{ label }}</div>
    <div class="card-value" :style="color ? { color } : {}">{{ value }}</div>
    <div v-if="subtitle" class="card-subtitle">{{ subtitle }}</div>
    <div v-if="trend !== undefined" class="card-trend" :class="trend >= 0 ? 'trend-up' : 'trend-down'">
      {{ trend >= 0 ? '+' : '' }}{{ trend.toFixed(1) }}%
    </div>
  </div>
</template>

<style scoped>
.progress-card {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.card-label {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.card-value {
  font-size: 2rem;
  font-weight: 700;
  font-family: var(--font-mono);
  line-height: 1.2;
}

.card-subtitle {
  font-size: 0.8rem;
  color: var(--color-text-muted);
}

.card-trend {
  font-size: 0.75rem;
  font-weight: 600;
  font-family: var(--font-mono);
}

.trend-up {
  color: var(--color-success);
}

.trend-down {
  color: var(--color-danger);
}
</style>
