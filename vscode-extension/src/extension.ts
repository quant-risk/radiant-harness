import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';

// Extension activation: register commands, tree views, status bar, and a
// file watcher that keeps the harness state UI live.
export function activate(context: vscode.ExtensionContext) {
    // ── Commands ──
    context.subscriptions.push(
        vscode.commands.registerCommand('radiant.init', () => runRadiant('init')),
        vscode.commands.registerCommand('radiant.validate', () => runRadiant('validate')),
        vscode.commands.registerCommand('radiant.run', (uri?: vscode.Uri) =>
            runRadiantOnSpec(uri ? vscode.Uri.file(uri.fsPath) : undefined)),
        vscode.commands.registerCommand('radiant.runGate', (uri: vscode.Uri) => runGate(uri))
    );

    // ── Tree views ──
    const specsProvider = new SpecsTreeProvider();
    const tasksProvider = new TasksTreeProvider();
    const progressProvider = new ProgressTreeProvider();

    vscode.window.registerTreeDataProvider('radiant.specs', specsProvider);
    vscode.window.registerTreeDataProvider('radiant.tasks', tasksProvider);
    vscode.window.registerTreeDataProvider('radiant.progress', progressProvider);

    // ── Status bar ──
    const statusBar = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    statusBar.text = '$(loading~spin) radiant: idle';
    statusBar.tooltip = 'Radiant Harness — click to view progress';
    statusBar.command = 'radiant.showProgress';
    statusBar.show();
    context.subscriptions.push(statusBar);

    // ── Watchers ──
    const mdWatcher = vscode.workspace.createFileSystemWatcher('**/*.md');
    mdWatcher.onDidChange(() => {
        specsProvider.refresh();
        tasksProvider.refresh();
    });
    const progressWatcher = vscode.workspace.createFileSystemWatcher('**/.radiant-harness/progress.json');
    progressWatcher.onDidChange(() => {
        progressProvider.refresh();
        updateStatusBar(statusBar);
    });
    progressWatcher.onDidCreate(() => {
        progressProvider.refresh();
        updateStatusBar(statusBar);
    });
    context.subscriptions.push(mdWatcher, progressWatcher);

    // ── CodeLens on tasks.md ──
    // Show a "▶ Run gate" inline action above any task row whose last
    // table cell contains a backtick-quoted shell command. Click it and
    // the command runs in a terminal — no copy/paste needed.
    const codeLensProvider = new TasksCodeLensProvider();
    context.subscriptions.push(
        vscode.languages.registerCodeLensProvider(
            { scheme: 'file', pattern: '**/specs/*/tasks.md' },
            codeLensProvider
        )
    );

    // First-paint refresh.
    specsProvider.refresh();
    tasksProvider.refresh();
    progressProvider.refresh();
    updateStatusBar(statusBar);
}

async function runRadiant(command: string) {
    const folder = vscode.workspace.workspaceFolders?.[0];
    if (!folder) {
        vscode.window.showErrorMessage('No workspace folder open');
        return;
    }
    const terminal = vscode.window.createTerminal('Radiant Harness');
    terminal.sendText(`radiant ${command} .`);
    terminal.show();
}

async function runRadiantOnSpec(uri?: vscode.Uri) {
    const folder = vscode.workspace.workspaceFolders?.[0];
    if (!folder) {
        vscode.window.showErrorMessage('No workspace folder open');
        return;
    }
    const specDir = uri?.fsPath ?? path.join(folder.uri.fsPath, 'specs');
    const terminal = vscode.window.createTerminal('Radiant Harness');
    terminal.sendText(`radiant run ${shellQuote(specDir)}`);
    terminal.show();
}

async function runGate(uri: vscode.Uri) {
    // Reads the gate command from the tasks.md row under cursor and runs
    // it in a terminal. This is a CodeLens target so the user can click
    // directly from the tasks file without leaving it.
    const folder = vscode.workspace.workspaceFolders?.[0];
    if (!folder) {
        return;
    }
    const line = fs.readFileSync(uri.fsPath, 'utf-8').split('\n')[uri.line] ?? '';
    const m = /`([^`]+)`/.exec(line);
    if (!m) {
        vscode.window.showInformationMessage('No gate command found on this line.');
        return;
    }
    const terminal = vscode.window.createTerminal('Radiant Gate');
    terminal.sendText(`cd ${shellQuote(folder.uri.fsPath)} && ${m[1]}`);
    terminal.show();
}

function shellQuote(s: string): string {
    return `'${s.replace(/'/g, `'\\''`)}'`;
}

// ── Status bar ──

function updateStatusBar(bar: vscode.StatusBarItem) {
    const folder = vscode.workspace.workspaceFolders?.[0];
    if (!folder) {
        bar.hide();
        return;
    }
    const progressPath = path.join(folder.uri.fsPath, '.radiant-harness', 'progress.json');
    if (!fs.existsSync(progressPath)) {
        bar.text = '$(circle-outline) radiant: not initialized';
        bar.tooltip = 'Run `radiant init` to set up the harness.';
        bar.show();
        return;
    }
    try {
        const p = JSON.parse(fs.readFileSync(progressPath, 'utf-8'));
        const state = String(p.State ?? 'idle');
        const feature = String(p.Feature ?? '');
        const current = Number(p.CurrentTask ?? 0);
        const total = Number(p.TotalTasks ?? 0);
        const pct = total > 0 ? Math.round((current / total) * 100) : 0;
        const icon = iconForState(state);
        bar.text = `${icon} radiant: ${state} — ${feature} (${current}/${total}, ${pct}%)`;
        bar.tooltip = `State: ${state}\nFeature: ${feature}\nClick to view progress`;
        bar.show();
    } catch (err) {
        bar.text = '$(warning) radiant: progress unreadable';
        bar.tooltip = String(err);
        bar.show();
    }
}

function iconForState(state: string): string {
    switch (state) {
        case 'implement': return '$(gear~spin)';
        case 'validate': return '$(checklist)';
        case 'correcting': return '$(warning)';
        case 'done': return '$(check)';
        case 'failed': return '$(error)';
        case 'research': return '$(search)';
        case 'plan': return '$(edit)';
        default: return '$(circle-outline)';
    }
}

// ── Tree data providers ──

class SpecsTreeProvider implements vscode.TreeDataProvider<SpecItem> {
    private _onDidChangeTreeData = new vscode.EventEmitter<SpecItem | undefined>();
    readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

    refresh(): void { this._onDidChangeTreeData.fire(undefined); }

    getTreeItem(element: SpecItem): vscode.TreeItem { return element; }

    getChildren(): SpecItem[] {
        const folder = vscode.workspace.workspaceFolders?.[0];
        if (!folder) return [];

        const specsDir = path.join(folder.uri.fsPath, 'specs');
        if (!fs.existsSync(specsDir)) return [];

        return fs.readdirSync(specsDir, { withFileTypes: true })
            .filter(d => d.isDirectory() && /^\d{4}-/.test(d.name))
            .sort((a, b) => a.name.localeCompare(b.name))
            .map(d => {
                const item = new SpecItem(d.name, vscode.TreeItemCollapsibleState.Collapsed);
                item.command = {
                    command: 'vscode.open',
                    title: 'Open spec',
                    arguments: [vscode.Uri.file(path.join(specsDir, d.name, 'spec.md'))]
                };
                return item;
            });
    }
}

class TasksTreeProvider implements vscode.TreeDataProvider<TasksGroupItem | TaskItem> {
    private _onDidChangeTreeData = new vscode.EventEmitter<TasksGroupItem | TaskItem | undefined>();
    readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

    refresh(): void { this._onDidChangeTreeData.fire(undefined); }

    getTreeItem(element: TasksGroupItem | TaskItem): vscode.TreeItem { return element; }

    getChildren(element?: TasksGroupItem | TaskItem): (TasksGroupItem | TaskItem)[] {
        const folder = vscode.workspace.workspaceFolders?.[0];
        if (!folder) return [];
        if (!element) {
            // Root level: one group per feature directory.
            const specsDir = path.join(folder.uri.fsPath, 'specs');
            if (!fs.existsSync(specsDir)) return [];
            return fs.readdirSync(specsDir, { withFileTypes: true })
                .filter(d => d.isDirectory() && /^\d{4}-/.test(d.name))
                .sort((a, b) => a.name.localeCompare(b.name))
                .map(d => new TasksGroupItem(d.name, path.join(specsDir, d.name)));
        }
        if (element instanceof TasksGroupItem) {
            const tasksPath = path.join(element.specDir, 'tasks.md');
            if (!fs.existsSync(tasksPath)) return [];
            const lines = fs.readFileSync(tasksPath, 'utf-8').split('\n');
            const items: TaskItem[] = [];
            for (let i = 0; i < lines.length; i++) {
                const line = lines[i].trim();
                if (!line.startsWith('|')) continue;
                if (line.includes('---')) continue;
                if (line.toLowerCase().includes('task') && line.toLowerCase().includes('covers')) continue;
                const cols = line.split('|').map(c => c.trim());
                if (cols.length < 6) continue;
                const id = cols[1];
                const name = cols[2];
                if (!/^\d+$/.test(id)) continue;
                items.push(new TaskItem(`#${id}: ${name}`, tasksPath, i));
            }
            return items;
        }
        return [];
    }
}

class ProgressTreeProvider implements vscode.TreeDataProvider<ProgressItem> {
    private _onDidChangeTreeData = new vscode.EventEmitter<ProgressItem | undefined>();
    readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

    refresh(): void { this._onDidChangeTreeData.fire(undefined); }

    getTreeItem(element: ProgressItem): vscode.TreeItem { return element; }

    getChildren(): ProgressItem[] {
        const folder = vscode.workspace.workspaceFolders?.[0];
        if (!folder) return [];
        const progressPath = path.join(folder.uri.fsPath, '.radiant-harness', 'progress.json');
        if (!fs.existsSync(progressPath)) return [];
        try {
            const p = JSON.parse(fs.readFileSync(progressPath, 'utf-8'));
            const items: ProgressItem[] = [];
            items.push(new ProgressItem(`State: ${p.State ?? 'idle'}`));
            if (p.Feature) items.push(new ProgressItem(`Feature: ${p.Feature}`));
            if (p.TotalTasks) {
                const completed = Array.isArray(p.Log)
                    ? p.Log.filter((e: { Action?: string }) => e.Action === 'completed').length
                    : 0;
                items.push(new ProgressItem(`Tasks: ${completed}/${p.TotalTasks}`));
            }
            if (Array.isArray(p.Log)) {
                for (const entry of p.Log.slice(-10)) {
                    const ts = new Date(entry.Timestamp ?? Date.now()).toLocaleTimeString();
                    items.push(new ProgressItem(`${ts} — ${entry.Action}${entry.TaskID ? ` task ${entry.TaskID}` : ''}`));
                }
            }
            return items;
        } catch {
            return [];
        }
    }
}

// ── Tree items ──

class SpecItem extends vscode.TreeItem {
    constructor(label: string, collapsibleState: vscode.TreeItemCollapsibleState) {
        super(label, collapsibleState);
        this.tooltip = label;
        this.description = 'spec';
        this.iconPath = new vscode.ThemeIcon('file-text');
        this.contextValue = 'spec';
    }
}

class TasksGroupItem extends vscode.TreeItem {
    constructor(label: string, public readonly specDir: string) {
        super(label, vscode.TreeItemCollapsibleState.Collapsed);
        this.iconPath = new vscode.ThemeIcon('list-unordered');
        this.contextValue = 'specTasks';
    }
}

class TaskItem extends vscode.TreeItem {
    constructor(label: string, public readonly tasksPath: string, public readonly line: number) {
        super(label, vscode.TreeItemCollapsibleState.None);
        this.iconPath = new vscode.ThemeIcon('tasklist');
        this.command = {
            command: 'vscode.open',
            title: 'Open tasks.md',
            arguments: [vscode.Uri.file(tasksPath), { selection: [line, 0, line, 0] }]
        };
    }
}

class ProgressItem extends vscode.TreeItem {
    constructor(label: string) {
        super(label, vscode.TreeItemCollapsibleState.None);
        this.iconPath = new vscode.ThemeIcon('pulse');
    }
}

// ── CodeLens ──

// TasksCodeLensProvider surfaces a "▶ Run gate" CodeLens above any
// task row in tasks.md whose last table cell contains a backtick-quoted
// shell command. Matches the `\`...\`` convention used in `radiant init`
// templates. The CodeLens calls the existing `radiant.runGate` command,
// which already handles the shell-quoting + terminal plumbing.
class TasksCodeLensProvider implements vscode.CodeLensProvider {
    private _onDidChangeCodeLenses = new vscode.EventEmitter<void>();
    readonly onDidChangeCodeLenses = this._onDidChangeCodeLenses.event;

    constructor() {
        // Re-evaluate CodeLenses whenever a markdown file changes — the
        // gate commands are often the first thing a user edits while
        // iterating on a spec.
        vscode.workspace.onDidChangeTextDocument((e) => {
            if (e.document.fileName.endsWith('/tasks.md')) {
                this._onDidChangeCodeLenses.fire();
            }
        });
    }

    provideCodeLenses(document: vscode.TextDocument): vscode.CodeLens[] {
        const lenses: vscode.CodeLens[] = [];
        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i).text;
            // Only consider table rows; first cell numeric (task id),
            // last cell contains a backtick-quoted command.
            const trimmed = line.trim();
            if (!trimmed.startsWith('|')) continue;
            const cells = trimmed.split('|').map(c => c.trim());
            if (cells.length < 6) continue;
            if (!/^\d+$/.test(cells[1])) continue;
            const cmdMatch = /`([^`]+)`/.exec(line);
            if (!cmdMatch) continue;

            const range = new vscode.Range(i, 0, i, 0);
            const lens = new vscode.CodeLens(range, {
                title: `▶ Run gate  (${cmdMatch[1]})`,
                command: 'radiant.runGate',
                arguments: [document.uri]
            });
            // Trick: `uri.line` is undefined here, so runGate falls back
            // to reading the line from the file. We rewrite uri.line
            // by wrapping the command via a closure-bound argument:
            (lens.command as vscode.Command).arguments = [
                { fsPath: document.uri.fsPath, line: i } as unknown as vscode.Uri
            ];
            lenses.push(lens);
        }
        return lenses;
    }
}
