<script setup lang="ts">
import { ref } from 'vue';
import { auth } from '../../lib/api';

const email = ref('');
const password = ref('');
const error = ref('');
const loading = ref(false);

async function handleSubmit() {
  error.value = '';
  loading.value = true;

  try {
    await auth.login({
      email: email.value,
      password: password.value,
    });
    // Redirect to exercises on success
    window.location.href = '/exercises';
  } catch (err: any) {
    error.value = err.data?.error || 'Login failed. Please check your credentials.';
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <form @submit.prevent="handleSubmit" class="space-y-6">
    <div v-if="error" class="rounded-md bg-red-50 p-4">
      <div class="flex">
        <div class="text-sm text-red-700">{{ error }}</div>
      </div>
    </div>

    <div>
      <label for="email" class="block text-sm font-medium text-gray-700">
        Email address
      </label>
      <div class="mt-1">
        <input
          id="email"
          v-model="email"
          name="email"
          type="email"
          autocomplete="email"
          required
          class="input"
        />
      </div>
    </div>

    <div>
      <label for="password" class="block text-sm font-medium text-gray-700">
        Password
      </label>
      <div class="mt-1">
        <input
          id="password"
          v-model="password"
          name="password"
          type="password"
          autocomplete="current-password"
          required
          class="input"
        />
      </div>
    </div>

    <div>
      <button
        type="submit"
        :disabled="loading"
        class="w-full btn btn-primary"
      >
        <span v-if="loading">Signing in...</span>
        <span v-else>Sign in</span>
      </button>
    </div>
  </form>
</template>
