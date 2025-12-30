<script setup lang="ts">
import { ref, onUnmounted, computed } from 'vue';
import { marked } from 'marked';
import { pairing, type Session, type RunOutput, type Intervention } from '../../lib/api';

// Configure marked for safe rendering
marked.setOptions({
  breaks: true,
  gfm: true,
});

// Render markdown to HTML
function renderMarkdown(content: string): string {
  return marked.parse(content) as string;
}

const props = defineProps<{
  session: Session;
  runOutput?: RunOutput;
}>();

type Intent = 'hint' | 'stuck' | 'review' | 'explain';

const interventions = ref<Intervention[]>([]);
const currentContent = ref('');
const loading = ref(false);
const error = ref('');
let eventSource: EventSource | null = null;

const intents: { id: Intent; label: string; description: string }[] = [
  { id: 'hint', label: '💡 Hint', description: 'Get a hint' },
  { id: 'stuck', label: '🆘 Stuck', description: 'I\'m stuck' },
  { id: 'review', label: '👀 Review', description: 'Review my code' },
  { id: 'explain', label: '📚 Explain', description: 'Explain this' },
];

async function requestIntervention(intent: Intent) {
  if (loading.value) return;

  loading.value = true;
  error.value = '';
  currentContent.value = '';

  // Close any existing stream
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }

  try {
    eventSource = pairing.stream(props.session.id, intent);

    eventSource.addEventListener('connected', () => {
      // Connected
    });

    eventSource.addEventListener('chunk', (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      currentContent.value += data.content;
    });

    eventSource.addEventListener('done', () => {
      // Save the intervention
      if (currentContent.value) {
        interventions.value.unshift({
          id: Date.now().toString(),
          level: 2,
          level_str: 'L2',
          type: intent,
          content: currentContent.value,
        });
      }
      currentContent.value = '';
      loading.value = false;
      eventSource?.close();
      eventSource = null;
    });

    eventSource.addEventListener('error', (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      error.value = data.error || 'Failed to get response';
      loading.value = false;
      eventSource?.close();
      eventSource = null;
    });

    eventSource.onerror = () => {
      if (eventSource?.readyState === EventSource.CLOSED) {
        loading.value = false;
      }
    };
  } catch (err: any) {
    error.value = 'Failed to start intervention';
    loading.value = false;
  }
}

function getLevelClass(level: number): string {
  return `badge-l${level}`;
}

onUnmounted(() => {
  eventSource?.close();
});
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Header -->
    <div class="flex-shrink-0 px-4 py-3 bg-gray-800 border-b border-gray-700">
      <h3 class="text-sm font-medium text-white flex items-center gap-2">
        🤖 AI Pairing
        <span class="text-xs text-gray-400">Max: L{{ session.maxLevel }}</span>
      </h3>
    </div>

    <!-- Intent Buttons -->
    <div class="flex-shrink-0 p-3 border-b border-gray-700 grid grid-cols-2 gap-2">
      <button
        v-for="intent in intents"
        :key="intent.id"
        @click="requestIntervention(intent.id)"
        :disabled="loading"
        class="btn btn-ghost text-gray-300 text-xs justify-start"
        :title="intent.description"
      >
        {{ intent.label }}
      </button>
    </div>

    <!-- Error -->
    <div v-if="error" class="p-3 bg-red-900/50 text-red-300 text-sm">
      {{ error }}
    </div>

    <!-- Current streaming response -->
    <div v-if="loading || currentContent" class="p-4 border-b border-gray-700">
      <div class="flex items-center gap-2 mb-2">
        <span class="badge badge-l2">Thinking...</span>
      </div>
      <div class="text-sm text-gray-300 prose prose-invert prose-sm max-w-none">
        <div v-html="renderMarkdown(currentContent)"></div>
        <span v-if="loading" class="animate-pulse">▋</span>
      </div>
    </div>

    <!-- Interventions Feed -->
    <div class="flex-1 overflow-y-auto">
      <div v-if="interventions.length === 0 && !loading" class="p-4 text-center text-gray-500 text-sm">
        <p class="mb-2">Need help?</p>
        <p>Click a button above to get AI assistance.</p>
      </div>

      <div v-for="intervention in interventions" :key="intervention.id" class="p-4 border-b border-gray-700">
        <div class="flex items-center gap-2 mb-2">
          <span :class="['badge', getLevelClass(intervention.level)]">
            {{ intervention.level_str }}
          </span>
          <span class="text-xs text-gray-400 capitalize">{{ intervention.type }}</span>
        </div>
        <div class="text-sm text-gray-300 prose prose-invert prose-sm max-w-none" v-html="renderMarkdown(intervention.content)"></div>
      </div>
    </div>

    <!-- Footer -->
    <div class="flex-shrink-0 px-4 py-2 bg-gray-800 border-t border-gray-700">
      <p class="text-xs text-gray-500 text-center">
        You remain the author. Understanding beats speed.
      </p>
    </div>
  </div>
</template>
