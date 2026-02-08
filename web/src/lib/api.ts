// Temper Daemon API client
// All requests proxy through Vite to the daemon at localhost:7533

import type {
  AnalyticsOverview,
  ErrorAnalytics,
  Exercise,
  HealthStatus,
  IndexStats,
  Profile,
  SearchResult,
  Session,
  SkillAnalytics,
  Track,
  TrendPoint,
} from './types';

const BASE = '/v1';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`API ${res.status}: ${body}`);
  }
  return res.json();
}

// Health & Status

export async function getHealth(): Promise<HealthStatus> {
  return request('/health');
}

export async function getStatus(): Promise<Record<string, unknown>> {
  return request('/status');
}

// Profile & Analytics

export async function getProfile(): Promise<Profile> {
  return request('/profile');
}

export async function getAnalyticsOverview(): Promise<AnalyticsOverview> {
  return request('/analytics/overview');
}

export async function getAnalyticsSkills(): Promise<SkillAnalytics[]> {
  return request<{ skills: SkillAnalytics[] }>('/analytics/skills').then(
    (r) => r.skills
  );
}

export async function getAnalyticsErrors(): Promise<ErrorAnalytics[]> {
  return request<{ errors: ErrorAnalytics[] }>('/analytics/errors').then(
    (r) => r.errors
  );
}

export async function getAnalyticsTrend(
  days: number = 30
): Promise<TrendPoint[]> {
  return request<{ trend: TrendPoint[] }>(
    `/analytics/trend?days=${days}`
  ).then((r) => r.trend);
}

// Sessions

export async function getSession(id: string): Promise<Session> {
  return request(`/sessions/${id}`);
}

// Exercises

export async function listExercises(): Promise<Exercise[]> {
  return request<{ exercises: Exercise[] }>('/exercises').then(
    (r) => r.exercises ?? []
  );
}

export async function listPackExercises(pack: string): Promise<Exercise[]> {
  return request<{ exercises: Exercise[] }>(`/exercises/${pack}`).then(
    (r) => r.exercises ?? []
  );
}

// Tracks

export async function listTracks(): Promise<Track[]> {
  return request<{ tracks: Track[] }>('/tracks').then((r) => r.tracks ?? []);
}

export async function getTrack(id: string): Promise<Track> {
  return request(`/tracks/${id}`);
}

// Document Index

export async function getDocIndexStatus(): Promise<IndexStats> {
  return request('/docindex/status');
}

export async function searchDocs(
  query: string,
  topK: number = 5
): Promise<SearchResult[]> {
  return request<{ results: SearchResult[] }>('/docindex/search', {
    method: 'POST',
    body: JSON.stringify({ query, top_k: topK }),
  }).then((r) => r.results ?? []);
}

export async function indexDocs(
  path?: string
): Promise<{ documents_found: number; documents_indexed: number }> {
  return request('/docindex/index', {
    method: 'POST',
    body: JSON.stringify({ path: path ?? '' }),
  });
}

// SSE Helpers

export function subscribeSession(
  sessionId: string,
  onEvent: (event: string, data: string) => void
): EventSource {
  const source = new EventSource(`${BASE}/stream/sessions/${sessionId}`);

  source.addEventListener('content', (e) => onEvent('content', e.data));
  source.addEventListener('metadata', (e) => onEvent('metadata', e.data));
  source.addEventListener('error', (e) => {
    if (e instanceof MessageEvent) {
      onEvent('error', e.data);
    }
  });
  source.addEventListener('done', (e) => {
    onEvent('done', e.data);
    source.close();
  });

  return source;
}
