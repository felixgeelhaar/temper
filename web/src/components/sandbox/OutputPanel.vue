<script setup lang="ts">
import { computed } from 'vue';
import type { Run } from '../../lib/api';

const props = defineProps<{
  run: Run | null;
  running: boolean;
}>();

const tabs = ['output', 'tests'] as const;
const activeTab = computed(() => 'output');

const output = computed(() => {
  if (!props.run?.output) return '';

  const parts: string[] = [];

  if (props.run.output.format_output) {
    parts.push('=== Format ===');
    parts.push(props.run.output.format_output);
    parts.push('');
  }

  if (props.run.output.build_output) {
    parts.push('=== Build ===');
    parts.push(props.run.output.build_output);
    parts.push('');
  }

  if (props.run.output.test_output) {
    parts.push('=== Tests ===');
    parts.push(props.run.output.test_output);
  }

  return parts.join('\n');
});

const summary = computed(() => {
  if (!props.run?.output) return null;

  const o = props.run.output;
  return {
    format: o.format_passed,
    build: o.build_passed,
    test: o.test_passed,
    duration: o.duration,
  };
});
</script>

<template>
  <div class="h-full flex flex-col bg-gray-900">
    <!-- Header -->
    <div class="flex-shrink-0 flex items-center justify-between px-4 py-2 bg-gray-800">
      <div class="flex items-center gap-4">
        <h3 class="text-sm font-medium text-white">Output</h3>
        <div v-if="summary" class="flex items-center gap-2 text-xs">
          <span :class="summary.format ? 'text-green-400' : 'text-red-400'">
            Format {{ summary.format ? '✓' : '✗' }}
          </span>
          <span :class="summary.build ? 'text-green-400' : 'text-red-400'">
            Build {{ summary.build ? '✓' : '✗' }}
          </span>
          <span :class="summary.test ? 'text-green-400' : 'text-red-400'">
            Test {{ summary.test ? '✓' : '✗' }}
          </span>
        </div>
      </div>
      <div v-if="summary" class="text-xs text-gray-400">
        {{ summary.duration }}
      </div>
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-auto p-4 font-mono text-sm">
      <div v-if="running" class="text-gray-400">
        Running...
      </div>
      <div v-else-if="!run" class="text-gray-500">
        Press ⌘+Enter to run your code
      </div>
      <pre v-else class="text-gray-300 whitespace-pre-wrap">{{ output }}</pre>
    </div>
  </div>
</template>
