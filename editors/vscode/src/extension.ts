import * as vscode from 'vscode';
import { TemperClient, Session, Intervention, RunResult, AuthoringSuggestion, SessionSummary } from './client';

// Global state
let client: TemperClient;
let currentSession: Session | null = null;
let outputChannel: vscode.OutputChannel;
let statusBarItem: vscode.StatusBarItem;
let currentSuggestions: AuthoringSuggestion[] = [];
let currentSpecPath: string | null = null;

export function activate(context: vscode.ExtensionContext) {
    console.log('Temper extension activated');

    // Initialize output channel
    outputChannel = vscode.window.createOutputChannel('Temper');

    // Initialize status bar
    statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    statusBarItem.command = 'temper.status';
    context.subscriptions.push(statusBarItem);

    // Initialize client
    initializeClient();

    // Register commands
    context.subscriptions.push(
        vscode.commands.registerCommand('temper.start', startSession),
        vscode.commands.registerCommand('temper.stop', stopSession),
        vscode.commands.registerCommand('temper.status', showStatus),
        vscode.commands.registerCommand('temper.hint', () => requestIntervention('hint')),
        vscode.commands.registerCommand('temper.review', () => requestIntervention('review')),
        vscode.commands.registerCommand('temper.stuck', () => requestIntervention('stuck')),
        vscode.commands.registerCommand('temper.next', () => requestIntervention('next')),
        vscode.commands.registerCommand('temper.explain', () => requestIntervention('explain')),
        vscode.commands.registerCommand('temper.run', runChecks),
        vscode.commands.registerCommand('temper.format', formatCode),
        vscode.commands.registerCommand('temper.exercises', listExercises),
        vscode.commands.registerCommand('temper.setMode', setLearningMode),
        vscode.commands.registerCommand('temper.health', checkHealth),
        // Spec authoring commands
        vscode.commands.registerCommand('temper.specAuthor', specAuthor),
        vscode.commands.registerCommand('temper.authorDiscover', authorDiscover),
        vscode.commands.registerCommand('temper.authorSuggest', authorSuggest),
        vscode.commands.registerCommand('temper.authorApply', authorApply),
        vscode.commands.registerCommand('temper.authorAsk', authorAsk),
    );

    // Watch for configuration changes
    context.subscriptions.push(
        vscode.workspace.onDidChangeConfiguration(e => {
            if (e.affectsConfiguration('temper')) {
                initializeClient();
            }
        })
    );

    // Auto-run on save (if enabled)
    context.subscriptions.push(
        vscode.workspace.onDidSaveTextDocument(doc => {
            const config = vscode.workspace.getConfiguration('temper');
            if (config.get('autoRunOnSave') && currentSession && doc.languageId === 'go') {
                runChecks();
            }
        })
    );

    updateStatusBar();
}

function initializeClient() {
    const config = vscode.workspace.getConfiguration('temper');
    client = new TemperClient({
        host: config.get('daemon.host', '127.0.0.1'),
        port: config.get('daemon.port', 7432),
    });
}

function updateStatusBar() {
    if (currentSession) {
        statusBarItem.text = `$(mortar-board) Temper: ${currentSession.exercise_id}`;
        statusBarItem.tooltip = `Session: ${currentSession.id}\nRuns: ${currentSession.run_count}\nHints: ${currentSession.hint_count}`;
        statusBarItem.show();
    } else {
        statusBarItem.text = '$(mortar-board) Temper';
        statusBarItem.tooltip = 'No active session';
        statusBarItem.show();
    }
}

function getActiveCode(): Record<string, string> {
    const editor = vscode.window.activeTextEditor;
    if (!editor) {
        return {};
    }

    const document = editor.document;
    const fileName = document.fileName.split('/').pop() || 'main.go';
    return { [fileName]: document.getText() };
}

async function startSession() {
    try {
        // Check if daemon is running
        const running = await client.isRunning();
        if (!running) {
            vscode.window.showErrorMessage('Temper daemon is not running. Start with: temper start');
            return;
        }

        // Get list of exercises
        const exercises = await client.listExercises();
        if (!exercises.packs || exercises.packs.length === 0) {
            vscode.window.showWarningMessage('No exercises found');
            return;
        }

        // Let user select exercise pack
        const packItems = exercises.packs.map(pack => ({
            label: pack.name,
            description: `${pack.language} - ${pack.exercise_count} exercises`,
            detail: pack.description,
            pack: pack,
        }));

        const selectedPack = await vscode.window.showQuickPick(packItems, {
            placeHolder: 'Select an exercise pack',
        });

        if (!selectedPack) {
            return;
        }

        // For now, let user enter exercise ID directly
        const exerciseId = await vscode.window.showInputBox({
            prompt: 'Enter exercise ID (e.g., hello-world)',
            placeHolder: 'exercise-slug',
        });

        if (!exerciseId) {
            return;
        }

        const fullExerciseId = `${selectedPack.pack.id}/${exerciseId}`;
        const config = vscode.workspace.getConfiguration('temper');
        const track = config.get<string>('learningTrack', 'practice');

        // Create session
        currentSession = await client.createSession(fullExerciseId, track);
        updateStatusBar();

        vscode.window.showInformationMessage(`Session started: ${currentSession.id.substring(0, 8)}`);
        outputChannel.appendLine(`Session started: ${currentSession.id}`);
        outputChannel.appendLine(`Exercise: ${fullExerciseId}`);
        outputChannel.appendLine(`Track: ${track}`);
        outputChannel.show();

    } catch (error) {
        vscode.window.showErrorMessage(`Failed to start session: ${error}`);
    }
}

async function stopSession() {
    if (!currentSession) {
        vscode.window.showWarningMessage('No active session');
        return;
    }

    try {
        const result = await client.deleteSession(currentSession.id);
        const sessionId = currentSession.id;
        currentSession = null;
        updateStatusBar();

        // Display session summary
        if (result.summary) {
            showSessionSummary(result.summary, sessionId);
        } else {
            vscode.window.showInformationMessage(`Session ended: ${sessionId.substring(0, 8)}`);
            outputChannel.appendLine(`Session ended: ${sessionId}`);
        }

    } catch (error) {
        vscode.window.showErrorMessage(`Failed to end session: ${error}`);
    }
}

function showSessionSummary(summary: SessionSummary, sessionId: string) {
    outputChannel.clear();
    outputChannel.appendLine('=== Session Summary ===');
    outputChannel.appendLine('');

    // Show accomplishment badge if present
    if (summary.accomplishment) {
        outputChannel.appendLine(`🏆 ${summary.accomplishment}`);
        outputChannel.appendLine('');
    }

    // Show stats
    outputChannel.appendLine(`Duration: ${summary.duration}`);
    outputChannel.appendLine(`Runs: ${summary.run_count}`);
    outputChannel.appendLine(`Hints: ${summary.hint_count}`);

    // Show spec progress for feature guidance sessions
    if (summary.spec_progress) {
        outputChannel.appendLine(`Spec Progress: ${summary.spec_progress}`);
    }

    outputChannel.appendLine('');
    outputChannel.appendLine('---');
    outputChannel.appendLine('');

    // Show motivational message
    outputChannel.appendLine(summary.message);

    outputChannel.appendLine('');
    outputChannel.appendLine(`Session ID: ${sessionId}`);
    outputChannel.show();

    // Show notification with accomplishment or simple message
    if (summary.accomplishment) {
        vscode.window.showInformationMessage(`🏆 ${summary.accomplishment} - ${summary.duration} of practice`);
    } else {
        vscode.window.showInformationMessage(`Session complete - ${summary.duration}`);
    }
}

async function showStatus() {
    if (!currentSession) {
        vscode.window.showInformationMessage('No active session. Use "Temper: Start Session" to begin.');
        return;
    }

    try {
        const session = await client.getSession(currentSession.id);
        currentSession = session;
        updateStatusBar();

        const message = [
            `**Session:** ${session.id.substring(0, 8)}`,
            `**Exercise:** ${session.exercise_id}`,
            `**Status:** ${session.status}`,
            `**Runs:** ${session.run_count}`,
            `**Hints:** ${session.hint_count}`,
            `**Track:** ${session.policy.track}`,
            `**Max Level:** L${session.policy.max_level}`,
        ].join('\n');

        outputChannel.clear();
        outputChannel.appendLine('=== Session Status ===');
        outputChannel.appendLine(`ID: ${session.id}`);
        outputChannel.appendLine(`Exercise: ${session.exercise_id}`);
        outputChannel.appendLine(`Status: ${session.status}`);
        outputChannel.appendLine(`Runs: ${session.run_count}`);
        outputChannel.appendLine(`Hints: ${session.hint_count}`);
        outputChannel.appendLine(`Track: ${session.policy.track}`);
        outputChannel.appendLine(`Max Level: L${session.policy.max_level}`);
        outputChannel.show();

    } catch (error) {
        vscode.window.showErrorMessage(`Failed to get status: ${error}`);
    }
}

async function requestIntervention(intent: 'hint' | 'review' | 'stuck' | 'next' | 'explain') {
    if (!currentSession) {
        vscode.window.showWarningMessage('No active session. Use "Temper: Start Session" to begin.');
        return;
    }

    try {
        const code = getActiveCode();

        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: `Requesting ${intent}...`,
            cancellable: false,
        }, async () => {
            let intervention: Intervention;

            switch (intent) {
                case 'hint':
                    intervention = await client.hint(currentSession!.id, code);
                    break;
                case 'review':
                    intervention = await client.review(currentSession!.id, code);
                    break;
                case 'stuck':
                    intervention = await client.stuck(currentSession!.id, code);
                    break;
                case 'next':
                    intervention = await client.next(currentSession!.id, code);
                    break;
                case 'explain':
                    intervention = await client.explain(currentSession!.id, code);
                    break;
            }

            showIntervention(intervention);
        });

    } catch (error: unknown) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        if (errorMessage.includes('cooldown')) {
            vscode.window.showWarningMessage('Please wait before requesting more detailed help (cooldown active)');
        } else {
            vscode.window.showErrorMessage(`Failed to get ${intent}: ${error}`);
        }
    }
}

function showIntervention(intervention: Intervention) {
    outputChannel.clear();
    outputChannel.appendLine('=== Temper Intervention ===');
    outputChannel.appendLine('');
    outputChannel.appendLine(`Level: L${intervention.level} (${intervention.type})`);
    outputChannel.appendLine(`Intent: ${intervention.intent}`);
    outputChannel.appendLine('');
    outputChannel.appendLine('---');
    outputChannel.appendLine('');
    outputChannel.appendLine(intervention.content);
    outputChannel.show();
}

async function runChecks() {
    if (!currentSession) {
        vscode.window.showWarningMessage('No active session. Use "Temper: Start Session" to begin.');
        return;
    }

    try {
        const code = getActiveCode();

        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: 'Running checks...',
            cancellable: false,
        }, async () => {
            const result = await client.run(currentSession!.id, code);
            showRunResult(result);
        });

    } catch (error) {
        vscode.window.showErrorMessage(`Run failed: ${error}`);
    }
}

function showRunResult(result: RunResult) {
    outputChannel.clear();
    outputChannel.appendLine('=== Run Results ===');
    outputChannel.appendLine('');

    const r = result.result;

    // Format
    const formatStatus = r.format_ok ? '✓' : '✗';
    outputChannel.appendLine(`Format: ${formatStatus}`);
    if (!r.format_ok && r.format_diff) {
        outputChannel.appendLine(r.format_diff);
    }

    // Build
    const buildStatus = r.build_ok ? '✓' : '✗';
    outputChannel.appendLine(`Build: ${buildStatus}`);
    if (!r.build_ok && r.build_output) {
        outputChannel.appendLine(r.build_output);
    }

    // Test
    const testStatus = r.test_ok ? '✓' : '✗';
    outputChannel.appendLine(`Tests: ${testStatus}`);
    if (r.test_output) {
        outputChannel.appendLine('');
        outputChannel.appendLine('--- Test Output ---');
        outputChannel.appendLine(r.test_output);
    }

    outputChannel.show();

    // Show summary notification
    if (r.format_ok && r.build_ok && r.test_ok) {
        vscode.window.showInformationMessage('All checks passed! ✓');
    } else {
        const failed = [];
        if (!r.format_ok) failed.push('format');
        if (!r.build_ok) failed.push('build');
        if (!r.test_ok) failed.push('tests');
        vscode.window.showWarningMessage(`Checks failed: ${failed.join(', ')}`);
    }
}

async function formatCode() {
    if (!currentSession) {
        vscode.window.showWarningMessage('No active session');
        return;
    }

    const editor = vscode.window.activeTextEditor;
    if (!editor) {
        return;
    }

    try {
        const code = getActiveCode();
        const result = await client.format(currentSession.id, code);

        if (result.ok && result.formatted) {
            const fileName = editor.document.fileName.split('/').pop() || 'main.go';
            if (result.formatted[fileName]) {
                const fullRange = new vscode.Range(
                    editor.document.positionAt(0),
                    editor.document.positionAt(editor.document.getText().length)
                );
                await editor.edit(editBuilder => {
                    editBuilder.replace(fullRange, result.formatted[fileName]);
                });
                vscode.window.showInformationMessage('Code formatted');
            }
        }

    } catch (error) {
        vscode.window.showErrorMessage(`Format failed: ${error}`);
    }
}

async function listExercises() {
    try {
        const running = await client.isRunning();
        if (!running) {
            vscode.window.showErrorMessage('Temper daemon is not running');
            return;
        }

        const exercises = await client.listExercises();

        outputChannel.clear();
        outputChannel.appendLine('=== Available Exercises ===');
        outputChannel.appendLine('');

        for (const pack of exercises.packs) {
            outputChannel.appendLine(`## ${pack.name}`);
            outputChannel.appendLine(`   ${pack.description}`);
            outputChannel.appendLine(`   Language: ${pack.language}`);
            outputChannel.appendLine(`   Exercises: ${pack.exercise_count}`);
            outputChannel.appendLine('');
        }

        outputChannel.show();

    } catch (error) {
        vscode.window.showErrorMessage(`Failed to list exercises: ${error}`);
    }
}

async function setLearningMode() {
    const modes = [
        { label: 'Practice', description: 'Normal practice mode (L0-L3)', value: 'practice' },
        { label: 'Interview Prep', description: 'Stricter mode (L0-L2)', value: 'interview-prep' },
    ];

    const selected = await vscode.window.showQuickPick(modes, {
        placeHolder: 'Select learning mode',
    });

    if (selected) {
        const config = vscode.workspace.getConfiguration('temper');
        await config.update('learningTrack', selected.value, vscode.ConfigurationTarget.Global);
        vscode.window.showInformationMessage(`Learning mode set to: ${selected.label}`);

        if (currentSession) {
            vscode.window.showInformationMessage('Note: Mode change will apply to the next session');
        }
    }
}

async function checkHealth() {
    try {
        const running = await client.isRunning();
        if (running) {
            vscode.window.showInformationMessage('Temper daemon is healthy ✓');
        } else {
            vscode.window.showErrorMessage('Temper daemon is not running. Start with: temper start');
        }
    } catch (error) {
        vscode.window.showErrorMessage(`Health check failed: ${error}`);
    }
}

// Spec Authoring Commands

async function specAuthor() {
    try {
        const running = await client.isRunning();
        if (!running) {
            vscode.window.showErrorMessage('Temper daemon is not running. Start with: temper start');
            return;
        }

        // Get list of specs
        const specsResult = await client.listSpecs();
        if (!specsResult.specs || specsResult.specs.length === 0) {
            vscode.window.showWarningMessage('No specs found. Create one first with temper spec create <name>');
            return;
        }

        // Let user select a spec
        const specItems = specsResult.specs.map(spec => ({
            label: spec.name || spec.file_path,
            description: spec.file_path,
            detail: spec.version ? `v${spec.version}` : undefined,
            spec: spec,
        }));

        const selectedSpec = await vscode.window.showQuickPick(specItems, {
            placeHolder: 'Select a spec to author',
        });

        if (!selectedSpec) {
            return;
        }

        currentSpecPath = selectedSpec.spec.file_path;

        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: 'Starting authoring session...',
            cancellable: false,
        }, async () => {
            // Discover docs first
            const docsResult = await client.discoverDocs(currentSpecPath!, ['docs/', 'README.md']);
            const docsPaths = docsResult.documents?.map(d => d.path) || [];

            if (docsResult.documents && docsResult.documents.length > 0) {
                vscode.window.showInformationMessage(`Found ${docsResult.documents.length} documents`);
            }

            // Create authoring session
            currentSession = await client.createAuthoringSession(currentSpecPath!, docsPaths);
            updateStatusBar();
        });

        outputChannel.clear();
        outputChannel.appendLine('=== Spec Authoring Session ===');
        outputChannel.appendLine('');
        outputChannel.appendLine(`Spec: ${selectedSpec.label}`);
        outputChannel.appendLine(`Path: ${currentSpecPath}`);
        outputChannel.appendLine(`Session: ${currentSession?.id.substring(0, 8)}`);
        outputChannel.appendLine('');
        outputChannel.appendLine('Next steps:');
        outputChannel.appendLine('  1. Use "Temper: Suggest Section" to get AI suggestions');
        outputChannel.appendLine('  2. Use "Temper: Apply Suggestion" to apply a suggestion');
        outputChannel.appendLine('  3. Use "Temper: Ask Authoring Question" for help');
        outputChannel.show();

        vscode.window.showInformationMessage(`Authoring session started: ${currentSession?.id.substring(0, 8)}`);

    } catch (error) {
        vscode.window.showErrorMessage(`Failed to start authoring: ${error}`);
    }
}

async function authorDiscover() {
    if (!currentSpecPath) {
        vscode.window.showWarningMessage('No active spec. Use "Temper: Start Spec Authoring" first.');
        return;
    }

    try {
        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: 'Discovering documentation...',
            cancellable: false,
        }, async () => {
            const result = await client.discoverDocs(currentSpecPath!, ['docs/', 'README.md', 'doc/', 'documentation/']);

            outputChannel.clear();
            outputChannel.appendLine('=== Discovered Documents ===');
            outputChannel.appendLine('');

            if (result.documents && result.documents.length > 0) {
                for (const doc of result.documents) {
                    outputChannel.appendLine(`## ${doc.title || doc.path}`);
                    outputChannel.appendLine(`   Type: ${doc.type}`);
                    outputChannel.appendLine(`   Path: ${doc.path}`);
                    if (doc.sections) {
                        outputChannel.appendLine(`   Sections: ${doc.sections.length}`);
                    }
                    outputChannel.appendLine('');
                }
                outputChannel.appendLine(`Total: ${result.documents.length} documents`);
            } else {
                outputChannel.appendLine('No documentation found.');
                outputChannel.appendLine('');
                outputChannel.appendLine('Try adding docs to:');
                outputChannel.appendLine('  - docs/ directory');
                outputChannel.appendLine('  - README.md');
            }

            outputChannel.show();
        });

    } catch (error) {
        vscode.window.showErrorMessage(`Failed to discover docs: ${error}`);
    }
}

async function authorSuggest() {
    if (!currentSession) {
        vscode.window.showWarningMessage('No active session. Use "Temper: Start Spec Authoring" first.');
        return;
    }

    const sections = [
        { label: 'Goals', value: 'goals', description: 'High-level project goals' },
        { label: 'Features', value: 'features', description: 'Feature definitions' },
        { label: 'Acceptance Criteria', value: 'acceptance_criteria', description: 'Verifiable success criteria' },
        { label: 'Non-Functional', value: 'non_functional', description: 'Performance, security, etc.' },
    ];

    const selectedSection = await vscode.window.showQuickPick(sections, {
        placeHolder: 'Select a section to get suggestions for',
    });

    if (!selectedSection) {
        return;
    }

    try {
        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: `Getting ${selectedSection.label} suggestions...`,
            cancellable: false,
        }, async () => {
            const result = await client.authoringSuggest(currentSession!.id, selectedSection.value);
            currentSuggestions = result.suggestions || [];

            outputChannel.clear();
            outputChannel.appendLine(`=== Suggestions: ${selectedSection.label} ===`);
            outputChannel.appendLine('');

            if (currentSuggestions.length > 0) {
                for (let i = 0; i < currentSuggestions.length; i++) {
                    const sug = currentSuggestions[i];
                    outputChannel.appendLine(`[${i + 1}] ${sug.id}`);
                    outputChannel.appendLine('');

                    if (typeof sug.value === 'string') {
                        outputChannel.appendLine(sug.value);
                    } else if (typeof sug.value === 'object' && sug.value !== null) {
                        const v = sug.value as Record<string, unknown>;
                        if (v.title) outputChannel.appendLine(`Title: ${v.title}`);
                        if (v.description) outputChannel.appendLine(`Description: ${v.description}`);
                        if (v.priority) outputChannel.appendLine(`Priority: ${v.priority}`);
                    }

                    outputChannel.appendLine('');
                    if (sug.source) {
                        outputChannel.appendLine(`Source: ${sug.source}`);
                    }
                    if (sug.confidence) {
                        outputChannel.appendLine(`Confidence: ${Math.round(sug.confidence * 100)}%`);
                    }
                    outputChannel.appendLine('---');
                    outputChannel.appendLine('');
                }

                outputChannel.appendLine('Use "Temper: Apply Suggestion" to apply a suggestion');
            } else {
                outputChannel.appendLine('No suggestions available.');
                outputChannel.appendLine('');
                outputChannel.appendLine('Try:');
                outputChannel.appendLine('  - Adding more documentation to your project');
                outputChannel.appendLine('  - Using "Temper: Ask Authoring Question"');
            }

            outputChannel.show();
        });

    } catch (error) {
        vscode.window.showErrorMessage(`Failed to get suggestions: ${error}`);
    }
}

async function authorApply() {
    if (!currentSession) {
        vscode.window.showWarningMessage('No active session. Use "Temper: Start Spec Authoring" first.');
        return;
    }

    if (currentSuggestions.length === 0) {
        vscode.window.showWarningMessage('No suggestions available. Use "Temper: Suggest Section" first.');
        return;
    }

    const items = currentSuggestions.map((sug, i) => ({
        label: `[${i + 1}] ${sug.id}`,
        description: sug.source,
        detail: typeof sug.value === 'string' ? sug.value.substring(0, 100) : JSON.stringify(sug.value).substring(0, 100),
        suggestion: sug,
    }));

    const selected = await vscode.window.showQuickPick(items, {
        placeHolder: 'Select a suggestion to apply',
    });

    if (!selected) {
        return;
    }

    try {
        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: 'Applying suggestion...',
            cancellable: false,
        }, async () => {
            const result = await client.authoringApply(currentSession!.id, selected.suggestion.id);

            if (result.applied) {
                vscode.window.showInformationMessage(`Suggestion applied: ${selected.suggestion.id}`);

                outputChannel.clear();
                outputChannel.appendLine('=== Suggestion Applied ===');
                outputChannel.appendLine('');
                outputChannel.appendLine(`ID: ${selected.suggestion.id}`);
                outputChannel.appendLine(`Section: ${result.section || 'unknown'}`);
                outputChannel.appendLine('');
                outputChannel.appendLine('The spec has been updated.');
                outputChannel.appendLine('');
                outputChannel.appendLine('Next: Use "Temper: Suggest Section" for more suggestions');
                outputChannel.show();
            } else {
                vscode.window.showErrorMessage('Failed to apply suggestion');
            }
        });

    } catch (error) {
        vscode.window.showErrorMessage(`Failed to apply suggestion: ${error}`);
    }
}

async function authorAsk() {
    if (!currentSession) {
        vscode.window.showWarningMessage('No active session. Use "Temper: Start Spec Authoring" first.');
        return;
    }

    const question = await vscode.window.showInputBox({
        prompt: 'Ask a question about your spec',
        placeHolder: 'e.g., What APIs should I define for the user service?',
    });

    if (!question) {
        return;
    }

    try {
        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: 'Thinking...',
            cancellable: false,
        }, async () => {
            const result = await client.authoringHint(currentSession!.id, 'general', question);

            outputChannel.clear();
            outputChannel.appendLine('=== Authoring Help ===');
            outputChannel.appendLine('');
            outputChannel.appendLine(`Question: ${question}`);
            outputChannel.appendLine('');
            outputChannel.appendLine('---');
            outputChannel.appendLine('');
            outputChannel.appendLine(result.content || 'No response received.');
            outputChannel.show();
        });

    } catch (error) {
        vscode.window.showErrorMessage(`Failed to get help: ${error}`);
    }
}

export function deactivate() {
    if (currentSession) {
        // Attempt to end session on deactivation
        client.deleteSession(currentSession.id).catch(() => {});
    }
}
