<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, shallowRef } from 'vue';
import * as monaco from 'monaco-editor';

const props = defineProps<{
  modelValue: string;
  language: string;
  filename: string;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void;
}>();

const containerRef = ref<HTMLDivElement>();
const editor = shallowRef<monaco.editor.IStandaloneCodeEditor>();

onMounted(() => {
  if (!containerRef.value) return;

  // Create editor
  editor.value = monaco.editor.create(containerRef.value, {
    value: props.modelValue,
    language: props.language,
    theme: 'vs-dark',
    fontSize: 14,
    fontFamily: '"JetBrains Mono", "Fira Code", monospace',
    lineNumbers: 'on',
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    automaticLayout: true,
    tabSize: 4,
    insertSpaces: false,
    wordWrap: 'off',
    renderWhitespace: 'selection',
    bracketPairColorization: { enabled: true },
    padding: { top: 8 },
  });

  // Emit changes
  editor.value.onDidChangeModelContent(() => {
    const value = editor.value?.getValue() || '';
    emit('update:modelValue', value);
  });
});

// Update content when prop changes
watch(
  () => props.modelValue,
  (newValue) => {
    if (editor.value && editor.value.getValue() !== newValue) {
      editor.value.setValue(newValue);
    }
  }
);

// Update language when filename changes
watch(
  () => props.filename,
  () => {
    if (editor.value) {
      const model = editor.value.getModel();
      if (model) {
        monaco.editor.setModelLanguage(model, props.language);
      }
    }
  }
);

onUnmounted(() => {
  editor.value?.dispose();
});
</script>

<template>
  <div ref="containerRef" class="w-full h-full"></div>
</template>
