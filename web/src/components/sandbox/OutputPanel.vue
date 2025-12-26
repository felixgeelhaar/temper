<script setup lang="ts">
import { computed } from 'vue';
import type { Run } from '../../lib/api';

const props = defineProps<{
  run: Run | null;
  running: boolean;
}>();

// Format duration from nanoseconds to human readable
function formatDuration(ns: number): string {
  if (ns < 1000000) {
    return `${(ns / 1000).toFixed(2)}µs`;
  } else if (ns < 1000000000) {
    return `${(ns / 1000000).toFixed(2)}ms`;
  } else {
    return `${(ns / 1000000000).toFixed(2)}s`;
  }
}

// Parse go test -json output into readable format
function parseTestOutput(jsonOutput: string): string {
  const lines = jsonOutput.split('\n').filter(l => l.trim());
  const testOutputs: Map<string, string[]> = new Map();
  const testResults: Map<string, { passed: boolean; elapsed: number }> = new Map();

  for (const line of lines) {
    try {
      const event = JSON.parse(line);
      const testKey = event.Test || '';

      if (event.Action === 'output' && event.Output) {
        // Collect output for each test
        const output = event.Output.replace(/\n$/, '');
        if (output && !output.startsWith('===') && !output.startsWith('---')) {
          const existing = testOutputs.get(testKey) || [];
          existing.push(output);
          testOutputs.set(testKey, existing);
        }
      } else if (event.Action === 'pass' || event.Action === 'fail') {
        if (testKey) {
          testResults.set(testKey, {
            passed: event.Action === 'pass',
            elapsed: event.Elapsed || 0
          });
        }
      }
    } catch {
      // Skip non-JSON lines
    }
  }

  const parts: string[] = [];

  // Show test results
  for (const [test, result] of testResults) {
    const icon = result.passed ? '✓' : '✗';
    const color = result.passed ? '' : '';
    parts.push(`${icon} ${test}`);

    // Show failure output
    if (!result.passed) {
      const outputs = testOutputs.get(test) || [];
      for (const out of outputs) {
        if (out.includes('want') || out.includes('got') || out.includes('Error')) {
          parts.push(`    ${out.trim()}`);
        }
      }
    }
  }

  if (parts.length === 0) {
    return jsonOutput; // Fallback to raw output
  }

  return parts.join('\n');
}

const output = computed(() => {
  if (!props.run?.output) return '';

  const parts: string[] = [];

  // Show format issues if any
  if (props.run.output.format_output) {
    parts.push('═══ Format Issues ═══');
    parts.push('Code formatting does not match gofmt standards.');
    parts.push('');
  }

  // Show build errors if any
  if (props.run.output.build_output && !props.run.output.build_passed) {
    parts.push('═══ Build Errors ═══');
    parts.push(props.run.output.build_output);
    parts.push('');
  }

  // Show test results
  if (props.run.output.test_output) {
    parts.push('═══ Test Results ═══');
    const passed = props.run.output.tests_passed || 0;
    const failed = props.run.output.tests_failed || 0;
    parts.push(`Passed: ${passed}  Failed: ${failed}`);
    parts.push('');
    parts.push(parseTestOutput(props.run.output.test_output));
  } else if (props.run.output.build_passed === false) {
    parts.push('═══ Tests ═══');
    parts.push('Tests skipped due to build errors.');
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
    duration: o.duration ? formatDuration(o.duration) : '',
    testsPassed: o.tests_passed || 0,
    testsFailed: o.tests_failed || 0,
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
            Test {{ summary.testsPassed }}/{{ summary.testsPassed + summary.testsFailed }}
          </span>
        </div>
      </div>
      <div v-if="summary && summary.duration" class="text-xs text-gray-400">
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
