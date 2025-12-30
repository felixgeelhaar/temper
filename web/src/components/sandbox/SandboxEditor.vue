<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue';
import { workspaces, runs, pairing, type Workspace, type Run, type Session } from '../../lib/api';
import MonacoEditor from '../editor/MonacoEditor.vue';
import OutputPanel from './OutputPanel.vue';
import PairingFeed from '../pairing/PairingFeed.vue';

const props = defineProps<{
  workspaceId: string;
}>();

// State
const workspace = ref<Workspace | null>(null);
const currentFile = ref<string>('main.go');
const files = ref<Record<string, string>>({});
const currentRun = ref<Run | null>(null);
const session = ref<Session | null>(null);
const loading = ref(true);
const error = ref('');
const saving = ref(false);
const running = ref(false);
const editorRef = ref<InstanceType<typeof MonacoEditor> | null>(null);

// Computed
const currentCode = computed({
  get: () => files.value[currentFile.value] || '',
  set: (value: string) => {
    files.value[currentFile.value] = value;
  },
});

const fileList = computed(() => Object.keys(files.value));

// Load workspace
onMounted(async () => {
  try {
    workspace.value = await workspaces.get(props.workspaceId);
    files.value = workspace.value.content || { 'main.go': '' };

    // Auto-select first file
    const fileNames = Object.keys(files.value);
    if (fileNames.length > 0) {
      currentFile.value = fileNames.includes('main.go') ? 'main.go' : fileNames[0];
    }

    // Start pairing session
    const sessionData = await pairing.startSession(props.workspaceId);
    session.value = sessionData;
  } catch (err: any) {
    error.value = err.data?.error || 'Failed to load workspace';
  } finally {
    loading.value = false;
  }
});

// Save workspace
async function saveWorkspace() {
  if (!workspace.value) return;

  saving.value = true;
  try {
    await workspaces.update(props.workspaceId, {
      content: files.value,
    });
  } catch (err: any) {
    error.value = 'Failed to save workspace';
  } finally {
    saving.value = false;
  }
}

// Run code
async function runCode() {
  if (!workspace.value) return;

  // Save first
  await saveWorkspace();

  running.value = true;
  currentRun.value = null;
  error.value = '';

  try {
    const run = await runs.trigger(props.workspaceId);
    currentRun.value = run;
  } catch (err: any) {
    error.value = err.data?.error || 'Failed to run code';
  } finally {
    running.value = false;
  }
}

// Format code using Monaco's built-in formatter
async function formatCode() {
  if (editorRef.value) {
    await editorRef.value.format();
    // Save after formatting
    await saveWorkspace();
    // Re-run to check if format passes now
    await runCode();
  }
}

// Navigate to next exercise
function goToNextExercise() {
  if (!workspace.value?.exercise_id) {
    window.location.href = '/exercises';
    return;
  }

  // Parse current exercise ID (e.g., "go-v1/basics/hello-world")
  const parts = workspace.value.exercise_id.split('/');
  if (parts.length >= 2) {
    const packId = parts[0];
    // Navigate to exercises page for now - a smarter approach would fetch the next exercise
    window.location.href = `/exercises/${packId}`;
  } else {
    window.location.href = '/exercises';
  }
}

// Keyboard shortcuts
function handleKeyDown(e: KeyboardEvent) {
  // Cmd/Ctrl + S to save
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault();
    saveWorkspace();
  }
  // Cmd/Ctrl + Enter to run
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault();
    runCode();
  }
  // Cmd/Ctrl + Shift + F to format
  if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.key === 'f') {
    e.preventDefault();
    formatCode();
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown);
});

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown);
});
</script>

<template>
  <div class="flex flex-col h-screen bg-gray-900">
    <!-- Header -->
    <header class="flex-shrink-0 bg-gray-800 border-b border-gray-700 px-4 py-2">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-4">
          <a href="/exercises" class="text-gray-400 hover:text-white">
            ← Back
          </a>
          <span class="text-white font-medium">{{ workspace?.name || 'Loading...' }}</span>
        </div>
        <div class="flex items-center gap-2">
          <span v-if="saving" class="text-gray-400 text-sm">Saving...</span>
          <button
            @click="saveWorkspace"
            :disabled="saving"
            class="btn btn-ghost text-gray-300 text-sm"
          >
            Save (⌘S)
          </button>
          <button
            @click="runCode"
            :disabled="running"
            class="btn btn-primary text-sm"
          >
            <span v-if="running">Running...</span>
            <span v-else>Run (⌘↵)</span>
          </button>
        </div>
      </div>
    </header>

    <!-- Loading state -->
    <div v-if="loading" class="flex-1 flex items-center justify-center">
      <div class="text-gray-400">Loading workspace...</div>
    </div>

    <!-- Error state -->
    <div v-else-if="error && !workspace" class="flex-1 flex items-center justify-center">
      <div class="text-center">
        <div class="text-red-400 mb-4">{{ error }}</div>
        <a href="/exercises" class="btn btn-outline">Back to Exercises</a>
      </div>
    </div>

    <!-- Main content -->
    <div v-else class="flex-1 flex overflow-hidden">
      <!-- File tree -->
      <aside class="w-48 flex-shrink-0 bg-gray-800 border-r border-gray-700 overflow-y-auto">
        <div class="p-2">
          <div class="text-xs font-medium text-gray-500 uppercase tracking-wider px-2 py-1">
            Files
          </div>
          <nav class="mt-1 space-y-0.5">
            <button
              v-for="file in fileList"
              :key="file"
              @click="currentFile = file"
              :class="[
                'w-full text-left px-2 py-1 text-sm rounded',
                currentFile === file
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-400 hover:bg-gray-700 hover:text-white',
              ]"
            >
              {{ file }}
            </button>
          </nav>
        </div>
      </aside>

      <!-- Editor + Output -->
      <div class="flex-1 flex flex-col overflow-hidden">
        <!-- Editor -->
        <div class="flex-1 overflow-hidden">
          <MonacoEditor
            ref="editorRef"
            v-model="currentCode"
            :filename="currentFile"
            language="go"
          />
        </div>

        <!-- Output Panel -->
        <div class="h-64 flex-shrink-0 border-t border-gray-700 overflow-hidden">
          <OutputPanel
            :run="currentRun"
            :running="running"
            :exerciseId="workspace?.exercise_id"
            @format="formatCode"
            @nextExercise="goToNextExercise"
          />
        </div>
      </div>

      <!-- Pairing Feed -->
      <aside class="w-80 flex-shrink-0 bg-gray-800 border-l border-gray-700 overflow-hidden">
        <PairingFeed
          v-if="session"
          :session="session"
          :runOutput="currentRun?.output"
        />
      </aside>
    </div>
  </div>
</template>
