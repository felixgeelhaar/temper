<script setup lang="ts">
import { computed } from 'vue';
import type { Run } from '../../lib/api';

const props = defineProps<{
  run: Run | null;
  running: boolean;
  exerciseId?: string;
}>();

const emit = defineEmits<{
  (e: 'format'): void;
  (e: 'nextExercise'): void;
}>();

// Check if all checks passed
const allPassed = computed(() => {
  if (!props.run?.output) return false;
  const o = props.run.output;
  return o.format_passed && o.build_passed && o.test_passed;
});

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

// Check specific failure states
const formatFailed = computed(() => {
  return props.run?.output && !props.run.output.format_passed;
});

const buildFailed = computed(() => {
  return props.run?.output && !props.run.output.build_passed;
});

const output = computed(() => {
  if (!props.run?.output) return '';

  const parts: string[] = [];

  // Show format issues if any
  if (props.run.output.format_output && !props.run.output.format_passed) {
    parts.push('═══ Format Issues ═══');
    parts.push('Code formatting does not match gofmt standards.');
    parts.push('Click "Format Code" below to auto-fix.');
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
      <template v-else>
        <!-- Success state -->
        <div v-if="allPassed" class="mb-4 p-4 bg-green-900/30 border border-green-700 rounded-lg">
          <div class="flex items-center gap-3">
            <span class="text-3xl">🎉</span>
            <div>
              <div class="text-green-400 font-semibold">All checks passed!</div>
              <div class="text-green-300/80 text-xs mt-1">Great work! You've completed this exercise.</div>
            </div>
          </div>
          <div class="mt-3 flex gap-2">
            <button
              @click="emit('nextExercise')"
              class="btn btn-sm bg-green-600 hover:bg-green-500 text-white"
            >
              Next Exercise →
            </button>
            <a href="/exercises" class="btn btn-sm btn-ghost text-green-400">
              Browse All
            </a>
          </div>
        </div>

        <pre class="text-gray-300 whitespace-pre-wrap">{{ output }}</pre>

        <!-- Action buttons -->
        <div v-if="formatFailed || buildFailed" class="mt-4 pt-4 border-t border-gray-700">
          <div v-if="formatFailed" class="mb-2">
            <button
              @click="emit('format')"
              class="btn btn-sm bg-yellow-600 hover:bg-yellow-500 text-white"
            >
              🔧 Format Code
            </button>
            <span class="text-xs text-gray-400 ml-2">Auto-fix formatting issues</span>
          </div>
          <div v-if="buildFailed" class="text-xs text-gray-400">
            💡 Tip: Check the error messages above and fix the syntax errors in your code.
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
