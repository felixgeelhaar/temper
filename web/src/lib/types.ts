// Domain types mirroring the Go backend

export interface Session {
  id: string;
  exercise_id: string;
  language: string;
  status: string;
  mode: string;
  started_at: string;
  ended_at?: string;
  duration_seconds?: number;
  runs: Run[];
  interventions: Intervention[];
  summary?: SessionSummary;
}

export interface Run {
  id: string;
  session_id: string;
  format_ok: boolean;
  build_ok: boolean;
  tests_passed: number;
  tests_failed: number;
  tests_total: number;
  timestamp: string;
}

export interface Intervention {
  id: string;
  session_id: string;
  level: number;
  type: string;
  content: string;
  timestamp: string;
}

export interface SessionSummary {
  duration_seconds: number;
  total_runs: number;
  max_intervention_level: number;
  skills_demonstrated: string[];
}

export interface Profile {
  id: string;
  overall_skill: number;
  language_skills: Record<string, number>;
  total_sessions: number;
  total_time_seconds: number;
  strengths: string[];
  weaknesses: string[];
}

export interface AnalyticsOverview {
  total_sessions: number;
  total_time_seconds: number;
  average_session_minutes: number;
  sessions_this_week: number;
  sessions_this_month: number;
  most_used_language: string;
  average_intervention_level: number;
}

export interface SkillAnalytics {
  language: string;
  skill_level: number;
  sessions_count: number;
  trend: number; // positive = improving
}

export interface ErrorAnalytics {
  category: string;
  count: number;
  last_seen: string;
}

export interface TrendPoint {
  date: string;
  sessions: number;
  average_skill: number;
  average_intervention_level: number;
}

export interface Exercise {
  id: string;
  pack: string;
  title: string;
  description: string;
  difficulty: string;
  language: string;
  tags: string[];
}

export interface Track {
  id: string;
  name: string;
  description: string;
  preset: string;
  max_intervention_level: number;
  rules: TrackRule[];
}

export interface TrackRule {
  trigger: string;
  action: string;
  threshold: number;
}

export interface IndexStats {
  total_documents: number;
  indexed_documents: number;
  total_sections: number;
  embedded_sections: number;
}

export interface SearchResult {
  section_id: number;
  document_id: string;
  heading: string;
  content: string;
  score: number;
}

export interface HealthStatus {
  status: string;
  timestamp: string;
}

export interface DaemonStatus {
  version: string;
  uptime_seconds: number;
  active_sessions: number;
  llm_providers: string[];
}
