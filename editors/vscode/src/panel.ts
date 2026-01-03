import * as vscode from 'vscode';
import { Session, RunResult, Intervention, PatchPreview } from './client';

export class TemperPanel {
    public static currentPanel: TemperPanel | undefined;
    private static readonly viewType = 'temperPanel';

    private readonly _panel: vscode.WebviewPanel;
    private readonly _extensionUri: vscode.Uri;
    private _disposables: vscode.Disposable[] = [];

    private _session: Session | null = null;
    private _interventions: Intervention[] = [];
    private _lastRun: RunResult | null = null;
    private _pendingPatch: PatchPreview | null = null;

    public static createOrShow(extensionUri: vscode.Uri): TemperPanel {
        const column = vscode.ViewColumn.Two;

        if (TemperPanel.currentPanel) {
            TemperPanel.currentPanel._panel.reveal(column);
            return TemperPanel.currentPanel;
        }

        const panel = vscode.window.createWebviewPanel(
            TemperPanel.viewType,
            'Temper',
            column,
            {
                enableScripts: true,
                retainContextWhenHidden: true,
                localResourceRoots: [extensionUri],
            }
        );

        TemperPanel.currentPanel = new TemperPanel(panel, extensionUri);
        return TemperPanel.currentPanel;
    }

    public static revive(panel: vscode.WebviewPanel, extensionUri: vscode.Uri): void {
        TemperPanel.currentPanel = new TemperPanel(panel, extensionUri);
    }

    private constructor(panel: vscode.WebviewPanel, extensionUri: vscode.Uri) {
        this._panel = panel;
        this._extensionUri = extensionUri;

        this._update();

        this._panel.onDidDispose(() => this.dispose(), null, this._disposables);

        this._panel.onDidChangeViewState(
            () => {
                if (this._panel.visible) {
                    this._update();
                }
            },
            null,
            this._disposables
        );

        this._panel.webview.onDidReceiveMessage(
            async (message) => {
                switch (message.command) {
                    case 'applyPatch':
                        vscode.commands.executeCommand('temper.patchApply');
                        break;
                    case 'rejectPatch':
                        vscode.commands.executeCommand('temper.patchReject');
                        break;
                    case 'requestHint':
                        vscode.commands.executeCommand('temper.hint');
                        break;
                    case 'runChecks':
                        vscode.commands.executeCommand('temper.run');
                        break;
                }
            },
            null,
            this._disposables
        );
    }

    public setSession(session: Session | null): void {
        this._session = session;
        if (!session) {
            this._interventions = [];
            this._lastRun = null;
            this._pendingPatch = null;
        }
        this._update();
    }

    public addIntervention(intervention: Intervention): void {
        this._interventions.unshift(intervention);
        if (this._interventions.length > 50) {
            this._interventions = this._interventions.slice(0, 50);
        }
        this._update();
    }

    public setRunResult(result: RunResult): void {
        this._lastRun = result;
        this._update();
    }

    public setPendingPatch(patch: PatchPreview | null): void {
        this._pendingPatch = patch;
        this._update();
    }

    public dispose(): void {
        TemperPanel.currentPanel = undefined;

        this._panel.dispose();

        while (this._disposables.length) {
            const d = this._disposables.pop();
            if (d) {
                d.dispose();
            }
        }
    }

    private _update(): void {
        this._panel.webview.html = this._getHtmlContent();
    }

    private _getHtmlContent(): string {
        const session = this._session;
        const interventions = this._interventions;
        const lastRun = this._lastRun;
        const pendingPatch = this._pendingPatch;

        return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Temper</title>
    <style>
        :root {
            --bg-primary: var(--vscode-editor-background);
            --bg-secondary: var(--vscode-sideBar-background);
            --fg-primary: var(--vscode-editor-foreground);
            --fg-secondary: var(--vscode-descriptionForeground);
            --border: var(--vscode-panel-border);
            --accent: var(--vscode-textLink-foreground);
            --success: var(--vscode-terminal-ansiGreen);
            --error: var(--vscode-terminal-ansiRed);
            --warning: var(--vscode-terminal-ansiYellow);
        }

        * {
            box-sizing: border-box;
        }

        body {
            font-family: var(--vscode-font-family);
            font-size: var(--vscode-font-size);
            color: var(--fg-primary);
            background: var(--bg-primary);
            padding: 16px;
            margin: 0;
        }

        h1, h2, h3 {
            margin: 0 0 12px 0;
            font-weight: 500;
        }

        h1 { font-size: 1.4em; }
        h2 { font-size: 1.2em; }
        h3 { font-size: 1em; }

        .section {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 6px;
            padding: 16px;
            margin-bottom: 16px;
        }

        .session-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 12px;
        }

        .status-badge {
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 0.85em;
            font-weight: 500;
        }

        .status-active { background: var(--success); color: black; }
        .status-inactive { background: var(--fg-secondary); color: white; }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));
            gap: 12px;
        }

        .stat-item {
            text-align: center;
        }

        .stat-value {
            font-size: 1.5em;
            font-weight: 600;
            color: var(--accent);
        }

        .stat-label {
            font-size: 0.85em;
            color: var(--fg-secondary);
        }

        .run-result {
            margin-top: 8px;
        }

        .run-status {
            display: flex;
            gap: 8px;
            margin-bottom: 8px;
        }

        .check-badge {
            padding: 2px 8px;
            border-radius: 3px;
            font-size: 0.8em;
        }

        .check-pass { background: var(--success); color: black; }
        .check-fail { background: var(--error); color: white; }

        .collapsible {
            cursor: pointer;
            user-select: none;
            display: flex;
            align-items: center;
            gap: 6px;
        }

        .collapsible::before {
            content: '▶';
            font-size: 0.7em;
            transition: transform 0.2s;
        }

        .collapsible.open::before {
            transform: rotate(90deg);
        }

        .collapsible-content {
            display: none;
            margin-top: 8px;
        }

        .collapsible-content.open {
            display: block;
        }

        pre {
            background: var(--bg-primary);
            border: 1px solid var(--border);
            border-radius: 4px;
            padding: 12px;
            overflow-x: auto;
            font-family: var(--vscode-editor-font-family);
            font-size: var(--vscode-editor-font-size);
            margin: 0;
        }

        code {
            font-family: var(--vscode-editor-font-family);
        }

        .intervention {
            border-left: 3px solid var(--accent);
            padding-left: 12px;
            margin-bottom: 16px;
        }

        .intervention-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 8px;
        }

        .intervention-level {
            font-size: 0.8em;
            padding: 2px 6px;
            border-radius: 3px;
            background: var(--accent);
            color: white;
        }

        .intervention-type {
            color: var(--fg-secondary);
            font-size: 0.9em;
        }

        .intervention-content {
            line-height: 1.6;
        }

        .intervention-content p {
            margin: 0 0 8px 0;
        }

        .intervention-content p:last-child {
            margin-bottom: 0;
        }

        .patch-section {
            border: 2px solid var(--warning);
            border-radius: 6px;
            padding: 16px;
            margin-bottom: 16px;
        }

        .patch-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 12px;
        }

        .patch-actions {
            display: flex;
            gap: 8px;
        }

        button {
            padding: 6px 12px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.9em;
        }

        .btn-primary {
            background: var(--accent);
            color: white;
        }

        .btn-secondary {
            background: var(--bg-primary);
            color: var(--fg-primary);
            border: 1px solid var(--border);
        }

        .btn-danger {
            background: var(--error);
            color: white;
        }

        button:hover {
            opacity: 0.9;
        }

        .diff-add { color: var(--success); }
        .diff-del { color: var(--error); }

        .quick-actions {
            display: flex;
            gap: 8px;
            flex-wrap: wrap;
        }

        .no-session {
            text-align: center;
            padding: 40px 20px;
            color: var(--fg-secondary);
        }

        .no-session h2 {
            color: var(--fg-primary);
        }
    </style>
</head>
<body>
    ${session ? this._renderSession(session, lastRun, pendingPatch, interventions) : this._renderNoSession()}

    <script>
        const vscode = acquireVsCodeApi();

        document.querySelectorAll('.collapsible').forEach(el => {
            el.addEventListener('click', () => {
                el.classList.toggle('open');
                const content = el.nextElementSibling;
                if (content) {
                    content.classList.toggle('open');
                }
            });
        });

        document.querySelectorAll('[data-command]').forEach(btn => {
            btn.addEventListener('click', () => {
                vscode.postMessage({ command: btn.dataset.command });
            });
        });
    </script>
</body>
</html>`;
    }

    private _renderNoSession(): string {
        return `
            <div class="no-session">
                <h2>No Active Session</h2>
                <p>Start a pairing session to begin learning.</p>
                <p>Use the command palette (Cmd+Shift+P) and search for "Temper: Start Session"</p>
            </div>
        `;
    }

    private _renderSession(
        session: Session,
        lastRun: RunResult | null,
        pendingPatch: PatchPreview | null,
        interventions: Intervention[]
    ): string {
        return `
            <div class="section">
                <div class="session-header">
                    <h2>Session</h2>
                    <span class="status-badge ${session.status === 'active' ? 'status-active' : 'status-inactive'}">
                        ${session.status}
                    </span>
                </div>
                <p style="color: var(--fg-secondary); margin: 0 0 12px 0;">
                    Exercise: ${session.exercise_id}
                </p>
                <div class="stats-grid">
                    <div class="stat-item">
                        <div class="stat-value">${session.run_count}</div>
                        <div class="stat-label">Runs</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">${session.hint_count}</div>
                        <div class="stat-label">Hints</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">L${session.policy.max_level}</div>
                        <div class="stat-label">Max Level</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">${session.policy.patching_enabled ? 'On' : 'Off'}</div>
                        <div class="stat-label">Patching</div>
                    </div>
                </div>
                <div class="quick-actions" style="margin-top: 16px;">
                    <button class="btn-primary" data-command="runChecks">Run Checks</button>
                    <button class="btn-secondary" data-command="requestHint">Get Hint</button>
                </div>
            </div>

            ${lastRun ? this._renderRunResult(lastRun) : ''}
            ${pendingPatch?.has_patch ? this._renderPatch(pendingPatch) : ''}
            ${interventions.length > 0 ? this._renderInterventions(interventions) : ''}
        `;
    }

    private _renderRunResult(run: RunResult): string {
        const result = run.result;
        return `
            <div class="section">
                <h3>Last Run</h3>
                <div class="run-status">
                    <span class="check-badge ${result.format_ok ? 'check-pass' : 'check-fail'}">
                        Format ${result.format_ok ? '✓' : '✗'}
                    </span>
                    <span class="check-badge ${result.build_ok ? 'check-pass' : 'check-fail'}">
                        Build ${result.build_ok ? '✓' : '✗'}
                    </span>
                    <span class="check-badge ${result.test_ok ? 'check-pass' : 'check-fail'}">
                        Test ${result.test_ok ? '✓' : '✗'}
                    </span>
                </div>
                <p style="color: var(--fg-secondary); margin: 8px 0 0 0; font-size: 0.85em;">
                    Duration: ${result.duration}ms
                </p>
                ${!result.format_ok && result.format_diff ? `
                    <div class="run-result">
                        <h4 class="collapsible">Format Diff</h4>
                        <div class="collapsible-content">
                            <pre>${this._escapeHtml(result.format_diff)}</pre>
                        </div>
                    </div>
                ` : ''}
                ${!result.build_ok && result.build_output ? `
                    <div class="run-result">
                        <h4 class="collapsible">Build Output</h4>
                        <div class="collapsible-content">
                            <pre>${this._escapeHtml(result.build_output)}</pre>
                        </div>
                    </div>
                ` : ''}
                ${!result.test_ok && result.test_output ? `
                    <div class="run-result">
                        <h4 class="collapsible open">Test Output</h4>
                        <div class="collapsible-content open">
                            <pre>${this._escapeHtml(result.test_output)}</pre>
                        </div>
                    </div>
                ` : ''}
            </div>
        `;
    }

    private _renderPatch(patch: PatchPreview): string {
        if (!patch.preview) return '';

        const preview = patch.preview;
        return `
            <div class="patch-section">
                <div class="patch-header">
                    <div>
                        <h3>Pending Patch</h3>
                        <p style="color: var(--fg-secondary); margin: 4px 0 0 0; font-size: 0.9em;">
                            ${preview.patch.file}: ${preview.patch.description}
                        </p>
                    </div>
                    <div class="patch-actions">
                        <button class="btn-primary" data-command="applyPatch">Apply</button>
                        <button class="btn-danger" data-command="rejectPatch">Reject</button>
                    </div>
                </div>
                <p style="color: var(--fg-secondary); margin: 0 0 8px 0; font-size: 0.85em;">
                    +${preview.additions} / -${preview.deletions} lines
                </p>
                ${preview.warnings && preview.warnings.length > 0 ? `
                    <p style="color: var(--warning); margin: 0 0 8px 0; font-size: 0.85em;">
                        ⚠ ${preview.warnings.join(', ')}
                    </p>
                ` : ''}
                <pre>${this._formatDiff(preview.patch.diff)}</pre>
            </div>
        `;
    }

    private _renderInterventions(interventions: Intervention[]): string {
        return `
            <div class="section">
                <h3>Interventions</h3>
                ${interventions.map(int => `
                    <div class="intervention">
                        <div class="intervention-header">
                            <span class="intervention-type">${int.type}</span>
                            <span class="intervention-level">L${int.level}</span>
                        </div>
                        <div class="intervention-content">
                            ${this._renderMarkdown(int.content)}
                        </div>
                    </div>
                `).join('')}
            </div>
        `;
    }

    private _escapeHtml(text: string): string {
        return text
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    }

    private _formatDiff(diff: string): string {
        return diff
            .split('\n')
            .map(line => {
                const escaped = this._escapeHtml(line);
                if (line.startsWith('+') && !line.startsWith('+++')) {
                    return `<span class="diff-add">${escaped}</span>`;
                } else if (line.startsWith('-') && !line.startsWith('---')) {
                    return `<span class="diff-del">${escaped}</span>`;
                }
                return escaped;
            })
            .join('\n');
    }

    private _renderMarkdown(content: string): string {
        // Simple markdown rendering
        let html = this._escapeHtml(content);

        // Code blocks
        html = html.replace(/```(\w+)?\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>');

        // Inline code
        html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

        // Bold
        html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');

        // Italic
        html = html.replace(/\*([^*]+)\*/g, '<em>$1</em>');

        // Line breaks to paragraphs
        html = html
            .split('\n\n')
            .filter(p => p.trim())
            .map(p => `<p>${p.replace(/\n/g, '<br>')}</p>`)
            .join('');

        return html;
    }
}
