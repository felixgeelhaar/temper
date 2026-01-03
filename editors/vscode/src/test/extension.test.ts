import * as assert from 'assert';
import * as vscode from 'vscode';

suite('Temper Extension Test Suite', () => {
    vscode.window.showInformationMessage('Start Temper extension tests.');

    test('Extension should be present', () => {
        assert.ok(vscode.extensions.getExtension('felixgeelhaar.temper'));
    });

    test('Should register all commands', async () => {
        const commands = await vscode.commands.getCommands(true);

        // Core session commands
        assert.ok(commands.includes('temper.start'), 'temper.start command should be registered');
        assert.ok(commands.includes('temper.stop'), 'temper.stop command should be registered');
        assert.ok(commands.includes('temper.status'), 'temper.status command should be registered');

        // Intervention commands
        assert.ok(commands.includes('temper.hint'), 'temper.hint command should be registered');
        assert.ok(commands.includes('temper.review'), 'temper.review command should be registered');
        assert.ok(commands.includes('temper.stuck'), 'temper.stuck command should be registered');
        assert.ok(commands.includes('temper.next'), 'temper.next command should be registered');
        assert.ok(commands.includes('temper.explain'), 'temper.explain command should be registered');
        assert.ok(commands.includes('temper.escalate'), 'temper.escalate command should be registered');

        // Code commands
        assert.ok(commands.includes('temper.run'), 'temper.run command should be registered');
        assert.ok(commands.includes('temper.format'), 'temper.format command should be registered');

        // Spec commands
        assert.ok(commands.includes('temper.specCreate'), 'temper.specCreate command should be registered');
        assert.ok(commands.includes('temper.specList'), 'temper.specList command should be registered');
        assert.ok(commands.includes('temper.specValidate'), 'temper.specValidate command should be registered');
        assert.ok(commands.includes('temper.specStatus'), 'temper.specStatus command should be registered');
        assert.ok(commands.includes('temper.specLock'), 'temper.specLock command should be registered');
        assert.ok(commands.includes('temper.specDrift'), 'temper.specDrift command should be registered');

        // Stats commands
        assert.ok(commands.includes('temper.stats'), 'temper.stats command should be registered');
        assert.ok(commands.includes('temper.statsSkills'), 'temper.statsSkills command should be registered');
        assert.ok(commands.includes('temper.statsErrors'), 'temper.statsErrors command should be registered');
        assert.ok(commands.includes('temper.statsTrend'), 'temper.statsTrend command should be registered');

        // Patch commands
        assert.ok(commands.includes('temper.patchPreview'), 'temper.patchPreview command should be registered');
        assert.ok(commands.includes('temper.patchApply'), 'temper.patchApply command should be registered');
        assert.ok(commands.includes('temper.patchReject'), 'temper.patchReject command should be registered');
        assert.ok(commands.includes('temper.patches'), 'temper.patches command should be registered');
        assert.ok(commands.includes('temper.patchLog'), 'temper.patchLog command should be registered');
        assert.ok(commands.includes('temper.patchStats'), 'temper.patchStats command should be registered');

        // Utility commands
        assert.ok(commands.includes('temper.panel'), 'temper.panel command should be registered');
        assert.ok(commands.includes('temper.health'), 'temper.health command should be registered');
    });

    test('Should have configuration settings', () => {
        const config = vscode.workspace.getConfiguration('temper');

        // Check default values
        assert.strictEqual(config.get('daemon.host'), '127.0.0.1');
        assert.strictEqual(config.get('daemon.port'), 7432);
        assert.strictEqual(config.get('learningTrack'), 'practice');
        assert.strictEqual(config.get('autoRunOnSave'), false);
    });
});
