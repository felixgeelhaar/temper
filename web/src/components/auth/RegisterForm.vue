<script setup lang="ts">
import { ref } from 'vue';
import { auth } from '../../lib/api';

const name = ref('');
const email = ref('');
const password = ref('');
const confirmPassword = ref('');
const error = ref('');
const loading = ref(false);

async function handleSubmit() {
  error.value = '';

  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match';
    return;
  }

  if (password.value.length < 8) {
    error.value = 'Password must be at least 8 characters';
    return;
  }

  loading.value = true;

  try {
    await auth.register({
      name: name.value,
      email: email.value,
      password: password.value,
    });
    // Redirect to exercises on success
    window.location.href = '/exercises';
  } catch (err: any) {
    error.value = err.data?.error || 'Registration failed. Please try again.';
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
      <label for="name" class="block text-sm font-medium text-gray-700">
        Name
      </label>
      <div class="mt-1">
        <input
          id="name"
          v-model="name"
          name="name"
          type="text"
          autocomplete="name"
          required
          class="input"
        />
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
          autocomplete="new-password"
          required
          minlength="8"
          class="input"
        />
      </div>
    </div>

    <div>
      <label for="confirmPassword" class="block text-sm font-medium text-gray-700">
        Confirm Password
      </label>
      <div class="mt-1">
        <input
          id="confirmPassword"
          v-model="confirmPassword"
          name="confirmPassword"
          type="password"
          autocomplete="new-password"
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
        <span v-if="loading">Creating account...</span>
        <span v-else>Create account</span>
      </button>
    </div>
  </form>
</template>
