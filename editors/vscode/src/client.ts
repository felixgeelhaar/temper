import * as http from 'http';

export interface Config {
    host: string;
    port: number;
}

export interface Session {
    id: string;
    exercise_id: string;
    code: Record<string, string>;
    policy: LearningPolicy;
    status: string;
    run_count: number;
    hint_count: number;
    created_at: string;
    updated_at: string;
}

export interface LearningPolicy {
    max_level: number;
    patching_enabled: boolean;
    cooldown_seconds: number;
    track: string;
}

export interface RunResult {
    id: string;
    session_id: string;
    result: {
        format_ok: boolean;
        format_diff?: string;
        build_ok: boolean;
        build_output?: string;
        test_ok: boolean;
        test_output?: string;
        duration: number;
    };
}

export interface Intervention {
    id: string;
    intent: string;
    level: number;
    type: string;
    content: string;
}

export interface ExercisePack {
    id: string;
    name: string;
    description: string;
    language: string;
    exercise_count: number;
}

export interface Exercise {
    id: string;
    title: string;
    description: string;
    difficulty: string;
    starter_code: Record<string, string>;
    test_code: Record<string, string>;
}

export class TemperClient {
    private config: Config;

    constructor(config: Config) {
        this.config = config;
    }

    private request<T>(method: string, path: string, body?: unknown): Promise<T> {
        return new Promise((resolve, reject) => {
            const options: http.RequestOptions = {
                hostname: this.config.host,
                port: this.config.port,
                path: path,
                method: method,
                headers: {
                    'Content-Type': 'application/json',
                },
            };

            const req = http.request(options, (res) => {
                let data = '';

                res.on('data', (chunk) => {
                    data += chunk;
                });

                res.on('end', () => {
                    try {
                        const parsed = JSON.parse(data);
                        if (res.statusCode && res.statusCode >= 400) {
                            reject(new Error(parsed.error || `Request failed with status ${res.statusCode}`));
                        } else {
                            resolve(parsed as T);
                        }
                    } catch (e) {
                        reject(new Error(`Failed to parse response: ${data}`));
                    }
                });
            });

            req.on('error', (e) => {
                reject(new Error(`Connection failed: ${e.message}`));
            });

            req.setTimeout(30000, () => {
                req.destroy();
                reject(new Error('Request timeout'));
            });

            if (body) {
                req.write(JSON.stringify(body));
            }

            req.end();
        });
    }

    async health(): Promise<{ status: string }> {
        return this.request('GET', '/v1/health');
    }

    async status(): Promise<{ status: string; version: string; llm_providers: string[]; runner: string }> {
        return this.request('GET', '/v1/status');
    }

    async listExercises(): Promise<{ packs: ExercisePack[] }> {
        return this.request('GET', '/v1/exercises');
    }

    async getExercise(pack: string, slug: string): Promise<Exercise> {
        return this.request('GET', `/v1/exercises/${pack}/${slug}`);
    }

    async createSession(exerciseId: string, track?: string): Promise<Session> {
        const body: { exercise_id: string; track?: string } = { exercise_id: exerciseId };
        if (track) {
            body.track = track;
        }
        return this.request('POST', '/v1/sessions', body);
    }

    async getSession(sessionId: string): Promise<Session> {
        return this.request('GET', `/v1/sessions/${sessionId}`);
    }

    async deleteSession(sessionId: string): Promise<{ deleted: boolean }> {
        return this.request('DELETE', `/v1/sessions/${sessionId}`);
    }

    async run(sessionId: string, code: Record<string, string>, opts?: { format?: boolean; build?: boolean; test?: boolean }): Promise<RunResult> {
        return this.request('POST', `/v1/sessions/${sessionId}/runs`, {
            code,
            format: opts?.format ?? true,
            build: opts?.build ?? true,
            test: opts?.test ?? true,
        });
    }

    async hint(sessionId: string, code?: Record<string, string>): Promise<Intervention> {
        return this.request('POST', `/v1/sessions/${sessionId}/hint`, code ? { code } : {});
    }

    async review(sessionId: string, code?: Record<string, string>): Promise<Intervention> {
        return this.request('POST', `/v1/sessions/${sessionId}/review`, code ? { code } : {});
    }

    async stuck(sessionId: string, code?: Record<string, string>): Promise<Intervention> {
        return this.request('POST', `/v1/sessions/${sessionId}/stuck`, code ? { code } : {});
    }

    async next(sessionId: string, code?: Record<string, string>): Promise<Intervention> {
        return this.request('POST', `/v1/sessions/${sessionId}/next`, code ? { code } : {});
    }

    async explain(sessionId: string, code?: Record<string, string>): Promise<Intervention> {
        return this.request('POST', `/v1/sessions/${sessionId}/explain`, code ? { code } : {});
    }

    async format(sessionId: string, code: Record<string, string>): Promise<{ ok: boolean; formatted: Record<string, string> }> {
        return this.request('POST', `/v1/sessions/${sessionId}/format`, { code });
    }

    async isRunning(): Promise<boolean> {
        try {
            const result = await this.health();
            return result.status === 'healthy';
        } catch {
            return false;
        }
    }

    // Spec Authoring

    async createAuthoringSession(specPath: string, docsPaths?: string[]): Promise<Session> {
        return this.request('POST', '/v1/sessions', {
            spec_path: specPath,
            intent: 'spec_authoring',
            docs_paths: docsPaths || [],
        });
    }

    async discoverDocs(specPath: string, docsPaths?: string[]): Promise<{ documents: Document[] }> {
        return this.request('POST', '/v1/authoring/discover', {
            spec_path: specPath,
            docs_paths: docsPaths || ['docs/', 'README.md'],
        });
    }

    async authoringSuggest(sessionId: string, section: string): Promise<{ suggestions: AuthoringSuggestion[] }> {
        return this.request('POST', `/v1/sessions/${sessionId}/authoring/suggest`, { section });
    }

    async authoringApply(sessionId: string, suggestionId: string): Promise<{ applied: boolean; section?: string }> {
        return this.request('POST', `/v1/sessions/${sessionId}/authoring/apply`, { suggestion_id: suggestionId });
    }

    async authoringHint(sessionId: string, section: string, question: string): Promise<Intervention> {
        return this.request('POST', `/v1/sessions/${sessionId}/authoring/hint`, { section, question });
    }

    async listSpecs(): Promise<{ specs: Spec[] }> {
        return this.request('GET', '/v1/specs');
    }
}

export interface Document {
    path: string;
    title: string;
    type: string;
    sections?: { heading: string; level: number; content: string }[];
}

export interface AuthoringSuggestion {
    id: string;
    section: string;
    value: unknown;
    source: string;
    confidence: number;
    reasoning?: string;
}

export interface Spec {
    name: string;
    file_path: string;
    version?: string;
    goals?: string[];
    acceptance_criteria?: { id: string; description: string; satisfied: boolean }[];
}

export interface PatchPreview {
    has_patch: boolean;
    preview?: {
        patch: {
            id: string;
            file: string;
            description: string;
            diff: string;
            status: string;
        };
        additions: number;
        deletions: number;
        warnings?: string[];
    };
    message?: string;
}
