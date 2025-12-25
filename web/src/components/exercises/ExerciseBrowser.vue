<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { exercises, type Pack, type ExerciseSummary } from '../../lib/api';

const packs = ref<Pack[]>([]);
const selectedPack = ref<Pack | null>(null);
const exerciseList = ref<ExerciseSummary[]>([]);
const loading = ref(true);
const error = ref('');
const starting = ref<string | null>(null);

onMounted(async () => {
  try {
    const data = await exercises.listPacks();
    packs.value = data.packs;
  } catch (err: any) {
    error.value = 'Failed to load exercise packs';
  } finally {
    loading.value = false;
  }
});

async function selectPack(pack: Pack) {
  selectedPack.value = pack;
  loading.value = true;
  error.value = '';

  try {
    const data = await exercises.listPackExercises(pack.id);
    exerciseList.value = data.exercises;
  } catch (err: any) {
    error.value = 'Failed to load exercises';
  } finally {
    loading.value = false;
  }
}

async function startExercise(exercise: ExerciseSummary) {
  if (!selectedPack.value) return;

  starting.value = exercise.id;
  try {
    // Exercise ID format: pack/category/slug (e.g., "go-v1/basics/hello-world")
    const parts = exercise.id.split('/');
    const packId = parts[0];
    const slug = parts.slice(1).join('/'); // "basics/hello-world"
    const data = await exercises.startExercise(packId, slug);
    window.location.href = `/sandbox/${data.workspace.id}`;
  } catch (err: any) {
    error.value = err.data?.error || 'Failed to start exercise';
    starting.value = null;
  }
}

function getDifficultyClass(difficulty: string) {
  switch (difficulty) {
    case 'beginner':
      return 'badge-beginner';
    case 'intermediate':
      return 'badge-intermediate';
    case 'advanced':
      return 'badge-advanced';
    default:
      return '';
  }
}
</script>

<template>
  <div>
    <div v-if="error" class="rounded-md bg-red-50 p-4 mb-4">
      <div class="text-sm text-red-700">{{ error }}</div>
    </div>

    <div v-if="loading && !selectedPack" class="text-center py-12">
      <div class="text-gray-500">Loading exercise packs...</div>
    </div>

    <!-- Pack Selection -->
    <div v-else-if="!selectedPack" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <button
        v-for="pack in packs"
        :key="pack.id"
        @click="selectPack(pack)"
        class="card p-6 text-left hover:border-blue-500 hover:shadow-md transition-all"
      >
        <h3 class="text-lg font-semibold text-gray-900">{{ pack.name }}</h3>
        <p class="mt-2 text-sm text-gray-600">{{ pack.description }}</p>
        <div class="mt-4 text-xs text-gray-400">Version {{ pack.version }}</div>
      </button>
    </div>

    <!-- Exercise List -->
    <div v-else>
      <button
        @click="selectedPack = null; exerciseList = []"
        class="btn btn-ghost mb-4"
      >
        ← Back to Packs
      </button>

      <div class="mb-6">
        <h2 class="text-2xl font-bold text-gray-900">{{ selectedPack.name }}</h2>
        <p class="text-gray-600">{{ selectedPack.description }}</p>
      </div>

      <div v-if="loading" class="text-center py-12">
        <div class="text-gray-500">Loading exercises...</div>
      </div>

      <div v-else class="space-y-4">
        <div
          v-for="exercise in exerciseList"
          :key="exercise.id"
          class="card p-6 flex justify-between items-start"
        >
          <div class="flex-1">
            <div class="flex items-center gap-2">
              <h3 class="text-lg font-medium text-gray-900">{{ exercise.title }}</h3>
              <span :class="['badge', getDifficultyClass(exercise.difficulty)]">
                {{ exercise.difficulty }}
              </span>
            </div>
            <p class="mt-1 text-sm text-gray-600">{{ exercise.description }}</p>
            <div v-if="exercise.tags?.length" class="mt-2 flex gap-1">
              <span
                v-for="tag in exercise.tags"
                :key="tag"
                class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800"
              >
                {{ tag }}
              </span>
            </div>
          </div>
          <button
            @click="startExercise(exercise)"
            :disabled="starting === exercise.id"
            class="btn btn-primary ml-4"
          >
            <span v-if="starting === exercise.id">Starting...</span>
            <span v-else>Start</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
