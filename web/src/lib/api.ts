const API_BASE = import.meta.env.PUBLIC_API_URL || 'http://localhost:8080';

interface FetchOptions extends RequestInit {
  params?: Record<string, string>;
}

class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    public data?: unknown
  ) {
    super(`API Error: ${status} ${statusText}`);
  }
}

async function request<T>(
  endpoint: string,
  options: FetchOptions = {}
): Promise<T> {
  const { params, ...fetchOptions } = options;

  let url = `${API_BASE}${endpoint}`;
  if (params) {
    const searchParams = new URLSearchParams(params);
    url += `?${searchParams.toString()}`;
  }

  const response = await fetch(url, {
    ...fetchOptions,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...fetchOptions.headers,
    },
  });

  if (!response.ok) {
    const data = await response.json().catch(() => null);
    throw new ApiError(response.status, response.statusText, data);
  }

  return response.json();
}

// Auth API
export const auth = {
  register: (data: { email: string; password: string; name: string }) =>
    request<{ user: User }>('/api/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  login: (data: { email: string; password: string }) =>
    request<{ user: User }>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  logout: () =>
    request<void>('/api/v1/auth/logout', { method: 'POST' }),

  me: () => request<{ user: User }>('/api/v1/auth/me'),
};

// Exercises API
export const exercises = {
  listPacks: () =>
    request<{ packs: Pack[]; total: number }>('/api/v1/exercises'),

  listPackExercises: (packId: string) =>
    request<{ pack: Pack; exercises: ExerciseSummary[]; total: number }>(
      `/api/v1/exercises/${packId}`
    ),

  getExercise: (packId: string, slug: string) =>
    request<Exercise>(`/api/v1/exercises/${packId}/${slug}`),

  startExercise: (packId: string, slug: string) =>
    request<{ workspace: Workspace; exercise: ExerciseSummary }>(
      `/api/v1/exercises/${packId}/${slug}/start`,
      { method: 'POST' }
    ),
};

// Workspaces API
export const workspaces = {
  list: () =>
    request<{ workspaces: Workspace[]; total: number }>('/api/v1/workspaces'),

  create: (data: { name: string; content?: Record<string, string> }) =>
    request<{ workspace: Workspace }>('/api/v1/workspaces', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  get: (id: string) =>
    request<Workspace>(`/api/v1/workspaces/${id}`),

  update: (id: string, data: { name?: string; content?: Record<string, string> }) =>
    request<{ workspace: Workspace }>(`/api/v1/workspaces/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    request<void>(`/api/v1/workspaces/${id}`, { method: 'DELETE' }),

  format: (id: string) =>
    request<{ content: Record<string, string>; message: string }>(
      `/api/v1/workspaces/${id}/format`,
      { method: 'POST' }
    ),
};

// Runs API
export const runs = {
  trigger: (workspaceId: string) =>
    request<Run>('/api/v1/runs', {
      method: 'POST',
      body: JSON.stringify({ workspace_id: workspaceId }),
    }),

  get: (id: string) => request<Run>(`/api/v1/runs/${id}`),

  stream: (id: string) => {
    const url = `${API_BASE}/api/v1/runs/${id}/stream`;
    return new EventSource(url, { withCredentials: true });
  },
};

// Pairing API
export const pairing = {
  startSession: (workspaceId: string) =>
    request<Session>('/api/v1/pairing/sessions', {
      method: 'POST',
      body: JSON.stringify({ workspace_id: workspaceId }),
    }),

  intervene: (data: { session_id: string; intent: string; run_output?: RunOutput }) =>
    request<Intervention>('/api/v1/pairing/intervene', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  stream: (sessionId: string, intent: string) => {
    const url = `${API_BASE}/api/v1/pairing/stream/${sessionId}?intent=${intent}`;
    return new EventSource(url, { withCredentials: true });
  },
};

// Types
export interface User {
  id: string;
  email: string;
  name: string;
  created_at: string;
}

export interface Pack {
  id: string;
  name: string;
  description: string;
  version: string;
}

export interface ExerciseSummary {
  id: string;
  title: string;
  difficulty: 'beginner' | 'intermediate' | 'advanced';
  description: string;
  tags: string[];
}

export interface Exercise extends ExerciseSummary {
  starter_code: Record<string, string>;
  test_code?: Record<string, string>;
}

export interface Workspace {
  id: string;
  name: string;
  exercise_id?: string;
  content?: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface Run {
  id: string;
  workspace_id: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  output?: RunOutput;
  duration?: string;
  created_at: string;
}

export interface RunOutput {
  format_output?: string;
  format_passed: boolean;
  build_output?: string;
  build_passed: boolean;
  test_output?: string;
  test_passed: boolean;
  test_results?: TestResult[];
  duration: string;
}

export interface TestResult {
  name: string;
  package: string;
  passed: boolean;
  elapsed: number;
  output?: string;
}

export interface Session {
  id: string;
  workspace_id: string;
  max_level: number;
  created_at: string;
}

export interface Intervention {
  id: string;
  level: number;
  level_str: string;
  type: string;
  content: string;
}
