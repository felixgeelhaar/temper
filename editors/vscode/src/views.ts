import * as vscode from 'vscode';
import { TemperClient, Session, ExercisePack } from './client';

// Session Tree View
export class SessionTreeProvider implements vscode.TreeDataProvider<SessionItem> {
    private _onDidChangeTreeData = new vscode.EventEmitter<SessionItem | undefined | null | void>();
    readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

    private client: TemperClient;
    private session: Session | null = null;

    constructor(client: TemperClient) {
        this.client = client;
    }

    refresh(): void {
        this._onDidChangeTreeData.fire();
    }

    setSession(session: Session | null): void {
        this.session = session;
        this.refresh();
    }

    getTreeItem(element: SessionItem): vscode.TreeItem {
        return element;
    }

    getChildren(element?: SessionItem): Thenable<SessionItem[]> {
        if (!this.session) {
            return Promise.resolve([
                new SessionItem(
                    'No Active Session',
                    'Start a session with Temper: Start Session',
                    vscode.TreeItemCollapsibleState.None,
                    'info'
                ),
            ]);
        }

        if (!element) {
            // Root level
            return Promise.resolve([
                new SessionItem(
                    'Session Info',
                    '',
                    vscode.TreeItemCollapsibleState.Expanded,
                    'session'
                ),
                new SessionItem(
                    'Policy',
                    '',
                    vscode.TreeItemCollapsibleState.Expanded,
                    'policy'
                ),
                new SessionItem(
                    'Statistics',
                    '',
                    vscode.TreeItemCollapsibleState.Expanded,
                    'stats'
                ),
            ]);
        }

        // Children based on parent type
        switch (element.contextValue) {
            case 'session':
                return Promise.resolve([
                    new SessionItem(
                        `ID: ${this.session.id.substring(0, 8)}...`,
                        this.session.id,
                        vscode.TreeItemCollapsibleState.None,
                        'field'
                    ),
                    new SessionItem(
                        `Exercise: ${this.session.exercise_id}`,
                        '',
                        vscode.TreeItemCollapsibleState.None,
                        'field'
                    ),
                    new SessionItem(
                        `Status: ${this.session.status}`,
                        '',
                        vscode.TreeItemCollapsibleState.None,
                        'field',
                        this.session.status === 'active' ? 'testing-passed-icon' : 'circle-outline'
                    ),
                ]);

            case 'policy':
                return Promise.resolve([
                    new SessionItem(
                        `Track: ${this.session.policy.track}`,
                        '',
                        vscode.TreeItemCollapsibleState.None,
                        'field'
                    ),
                    new SessionItem(
                        `Max Level: L${this.session.policy.max_level}`,
                        '',
                        vscode.TreeItemCollapsibleState.None,
                        'field'
                    ),
                    new SessionItem(
                        `Patching: ${this.session.policy.patching_enabled ? 'Enabled' : 'Disabled'}`,
                        '',
                        vscode.TreeItemCollapsibleState.None,
                        'field'
                    ),
                    new SessionItem(
                        `Cooldown: ${this.session.policy.cooldown_seconds}s`,
                        '',
                        vscode.TreeItemCollapsibleState.None,
                        'field'
                    ),
                ]);

            case 'stats':
                return Promise.resolve([
                    new SessionItem(
                        `Runs: ${this.session.run_count}`,
                        '',
                        vscode.TreeItemCollapsibleState.None,
                        'field',
                        'play'
                    ),
                    new SessionItem(
                        `Hints: ${this.session.hint_count}`,
                        '',
                        vscode.TreeItemCollapsibleState.None,
                        'field',
                        'lightbulb'
                    ),
                ]);

            default:
                return Promise.resolve([]);
        }
    }
}

class SessionItem extends vscode.TreeItem {
    constructor(
        public readonly label: string,
        public readonly tooltip: string,
        public readonly collapsibleState: vscode.TreeItemCollapsibleState,
        public readonly contextValue: string,
        iconId?: string
    ) {
        super(label, collapsibleState);
        this.tooltip = tooltip || label;

        if (iconId) {
            this.iconPath = new vscode.ThemeIcon(iconId);
        }
    }
}

// Exercises Tree View
export class ExercisesTreeProvider implements vscode.TreeDataProvider<ExerciseItem> {
    private _onDidChangeTreeData = new vscode.EventEmitter<ExerciseItem | undefined | null | void>();
    readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

    private client: TemperClient;
    private packs: ExercisePack[] = [];
    private isLoading = false;

    constructor(client: TemperClient) {
        this.client = client;
    }

    refresh(): void {
        this._onDidChangeTreeData.fire();
    }

    async loadExercises(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;
        try {
            const result = await this.client.listExercises();
            this.packs = result.packs || [];
        } catch (error) {
            this.packs = [];
        } finally {
            this.isLoading = false;
            this.refresh();
        }
    }

    getTreeItem(element: ExerciseItem): vscode.TreeItem {
        return element;
    }

    async getChildren(element?: ExerciseItem): Promise<ExerciseItem[]> {
        if (!element) {
            // Root level - show packs
            if (this.packs.length === 0 && !this.isLoading) {
                // Try to load
                await this.loadExercises();
            }

            if (this.packs.length === 0) {
                return [
                    new ExerciseItem(
                        'No exercises found',
                        'Check daemon connection',
                        vscode.TreeItemCollapsibleState.None,
                        'info',
                        undefined
                    ),
                ];
            }

            return this.packs.map(pack =>
                new ExerciseItem(
                    pack.name,
                    `${pack.language} - ${pack.exercise_count} exercises\n${pack.description}`,
                    vscode.TreeItemCollapsibleState.Collapsed,
                    'pack',
                    pack,
                    this.getLanguageIcon(pack.language)
                )
            );
        }

        // Pack children - for now show a placeholder
        // In a full implementation, we'd fetch the exercise list from the pack
        if (element.contextValue === 'pack' && element.pack) {
            return [
                new ExerciseItem(
                    `${element.pack.exercise_count} exercises available`,
                    'Select pack to start a session',
                    vscode.TreeItemCollapsibleState.None,
                    'exercise-count',
                    undefined,
                    'list-ordered'
                ),
            ];
        }

        return [];
    }

    private getLanguageIcon(language: string): string {
        switch (language.toLowerCase()) {
            case 'go':
                return 'symbol-method';
            case 'python':
                return 'symbol-misc';
            case 'typescript':
            case 'javascript':
                return 'symbol-variable';
            case 'rust':
                return 'symbol-struct';
            default:
                return 'code';
        }
    }
}

class ExerciseItem extends vscode.TreeItem {
    constructor(
        public readonly label: string,
        public readonly tooltip: string,
        public readonly collapsibleState: vscode.TreeItemCollapsibleState,
        public readonly contextValue: string,
        public readonly pack?: ExercisePack,
        iconId?: string
    ) {
        super(label, collapsibleState);
        this.tooltip = tooltip || label;

        if (iconId) {
            this.iconPath = new vscode.ThemeIcon(iconId);
        }

        // Add command to start session when clicking a pack
        if (contextValue === 'pack' && pack) {
            this.command = {
                command: 'temper.startFromPack',
                title: 'Start Session',
                arguments: [pack],
            };
        }
    }
}
