<script lang="ts">
    import {onMount} from 'svelte';
    import {getSettings, updateSettings, isAppIncluded, setAppIncluded} from '$lib/stores/settings.svelte';
    import {showToast, getActiveTab, setActiveTab} from '$lib/stores/ui.svelte';
    import {getIsWindows} from '$lib/stores/platform.svelte';
    import Switch from '$lib/components/shared/Switch.svelte';
    import type {
        main,
        scheduler as schedulerModels,
    } from '../../../../wailsjs/go/models';

    const appTargets = [
        {key: 'system', label: 'Windows System Theme'},
        {key: 'browser', label: 'Browser'},
        {key: 'win11-start-menu', label: 'Start Menu'},
        {key: 'win11-notifications', label: 'Notifications'},
        {key: 'win11-taskbar', label: 'Taskbar'},
        {key: 'win11-file-explorer', label: 'File Explorer'},
        {key: 'windows-terminal', label: 'Windows Terminal'},
        {key: 'vscode', label: 'VS Code / Cursor'},
        {key: 'vencord', label: 'Discord / Vencord'},
        {key: 'obsidian', label: 'Obsidian'},
        {key: 'yasb', label: 'YASB'},
        {key: 'glazewm', label: 'GlazeWM'},
    ];

    const aiHarnessTargets = [
        {key: 'opencode', label: 'OpenCode'},
        {key: 'hermes', label: 'Hermes'},
        {key: 'pi', label: 'Pi'},
        {key: 'claude-code', label: 'Claude Code'},
        {key: 'jan', label: 'Jan'},
        {key: 'aider', label: 'Aider'},
    ];

    let choosingFolder = $state(false);
    let wallpaperFolder = $derived(
        getSettings().wallpaperFolder || '~/Wallpapers'
    );

    // Ecosystem state
    let aetherDirStatus = $state<main.AetherDirStatus | null>(null);
    let activeThemeName = $state('');
    let cliCommand = $state('');
    let isWindows = $state(false);

    // Light/Dark scheduler â€” auto day/night blueprint switching.
    let scheduler = $state<schedulerModels.Settings>({
        enabled: false,
        light_blueprint: '',
        dark_blueprint: '',
        mode: 'system',
    });
    let schedulerLoaded = $state(false);
    let schedulerBusy = $state(false);

    async function loadScheduler() {
        try {
            const {SchedulerLoad, ListBlueprints} = await import(
                '../../../../wailsjs/go/main/App'
            );
            const [settings, bps] = await Promise.all([
                SchedulerLoad(),
                ListBlueprints(),
            ]);
            scheduler = {...(settings ?? {})} as unknown as schedulerModels.Settings;
            blueprintNames = (bps ?? [])
                .map((bp: Record<string, any>) => String(bp?.name ?? ''))
                .filter(Boolean);
            schedulerLoaded = true;
        } catch {
            schedulerLoaded = false;
        }
    }

    async function saveScheduler() {
        schedulerBusy = true;
        try {
            const {SchedulerSave} = await import(
                '../../../../wailsjs/go/main/App'
            );
            await SchedulerSave({
                enabled: !!scheduler.enabled,
                light_blueprint: scheduler.light_blueprint ?? '',
                dark_blueprint: scheduler.dark_blueprint ?? '',
                mode: scheduler.mode === 'schedule' ? 'schedule' : 'system',
            });
            showToast('Scheduler settings saved');
        } catch (e: any) {
            showToast('Scheduler save failed: ' + (e?.message || ''));
        } finally {
            schedulerBusy = false;
        }
    }

    onMount(async () => {
        isWindows = getIsWindows();
        try {
            const {GetAetherDirStatus, GetHeadlessCLICommand} = await import(
                '../../../../wailsjs/go/main/App'
            );
            const status = await GetAetherDirStatus() as unknown as main.AetherDirStatus;
            aetherDirStatus = status;
            cliCommand = await GetHeadlessCLICommand() as string;
        } catch {}
        loadScheduler();
    });

    async function chooseWallpaperFolder() {
        choosingFolder = true;
        try {
            const {ChooseWallpaperFolder} = await import(
                '../../../../wailsjs/go/main/App'
            );
            const folder = await ChooseWallpaperFolder();
            if (!folder) return;

            updateSettings({wallpaperFolder: folder});
            showToast('Wallpaper folder updated');
        } catch (err) {
            console.error('ChooseWallpaperFolder failed', err);
            showToast('Failed to set wallpaper folder');
        } finally {
            choosingFolder = false;
        }
    }

    async function openInExplorer(path: string) {
        try {
            const {OpenFolderInExplorer} = await import(
                '../../../../wailsjs/go/main/App'
            );
            await OpenFolderInExplorer(path);
        } catch {
            showToast('Could not open folder');
        }
    }

    async function copyCLICommand() {
        if (!cliCommand) return;
        try {
            await navigator.clipboard.writeText(cliCommand);
            showToast('Copied CLI command to clipboard');
        } catch {
            showToast('Failed to copy');
        }
    }
</script>

<div class="h-full overflow-y-auto">
    <div class="mx-auto max-w-[760px] px-8 py-11">
        <header class="mb-9">
            <p
                class="text-accent mb-2 text-[10px] font-semibold uppercase tracking-[0.18em]"
            >
                Preferences
            </p>
            <h1
                class="text-fg-primary text-[26px] font-semibold tracking-[-0.01em]"
            >
                Settings
            </h1>
            <p class="text-fg-secondary mt-2 text-[13px] leading-relaxed">
                Configure where Waifer finds local wallpapers and manage the
                ecosystem integration.
            </p>
        </header>

        <section aria-labelledby="wallpaper-library-heading">
            <h2
                id="wallpaper-library-heading"
                class="text-fg-dimmed mb-2.5 text-[10px] font-semibold uppercase tracking-[0.14em]"
            >
                Wallpaper library
            </h2>

            <div
                class="border-border bg-bg-secondary grid grid-cols-[auto_minmax(0,1fr)] items-start gap-4 border p-[18px] sm:grid-cols-[auto_minmax(0,1fr)_auto]"
            >
                <div
                    class="bg-accent-muted text-accent flex h-9 w-9 shrink-0 items-center justify-center"
                >
                    <svg
                        class="h-4 w-4"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        aria-hidden="true"
                    >
                        <path
                            d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"
                        ></path>
                    </svg>
                </div>

                <div class="min-w-0">
                    <div class="text-fg-primary text-[13px] font-semibold">
                        Wallpaper folder
                    </div>
                    <p class="text-fg-dimmed mb-3 mt-[3px] text-[12px]">
                        The Local page scans this folder and all of its
                        subfolders for images.
                    </p>
                    <div
                        class="bg-bg-primary border-border text-fg-secondary truncate border px-[11px] py-[9px] font-mono text-[12px] font-medium"
                        title={wallpaperFolder}
                    >
                        {wallpaperFolder}
                    </div>
                </div>

                <button
                    type="button"
                    class="bg-accent text-accent-fg hover:bg-accent-hover col-span-2 h-8 w-full shrink-0 px-3.5 text-[12px] font-semibold transition-colors disabled:cursor-wait disabled:opacity-60 sm:col-span-1 sm:w-auto"
                    onclick={chooseWallpaperFolder}
                    disabled={choosingFolder}
                >
                    {choosingFolder ? 'Choosingâ€¦' : 'Choose folderâ€¦'}
                </button>
            </div>
        </section>

        <section aria-labelledby="scheduler-heading" class="mt-9">
            <h2
                id="scheduler-heading"
                class="text-fg-dimmed mb-2.5 text-[10px] font-semibold uppercase tracking-[0.14em]"
            >
                Light/Dark scheduler
            </h2>
            {#if schedulerLoaded}
                <div
                    class="border-border bg-bg-secondary grid gap-3 border p-[18px]"
                >
                    <div class="flex items-center justify-between gap-4">
                        <div class="min-w-0">
                            <div
                                class="text-fg-primary text-[13px] font-semibold"
                            >
                                Automatic day/night switching
                            </div>
                            <p class="text-fg-dimmed mt-[3px] text-[12px]">
                                Switch between a light and a dark blueprint
                                automatically. 06:00â€“18:00 local time is light.
                            </p>
                        </div>
                        <Switch
                            checked={!!scheduler.enabled}
                            onchange={v => {
                                scheduler.enabled = v;
                            }}
                            size="sm"
                        />
                    </div>
                    <div class="grid gap-3 sm:grid-cols-2">
                        <label class="text-fg-secondary text-[12px]">
                            Light blueprint
                            <select
                                class="bg-bg-primary border-border text-fg-primary mt-1 w-full border px-2 py-1.5 text-[12px]"
                                bind:value={scheduler.light_blueprint}
                            >
                                <option value="">None</option>
                                {#each blueprintNames as name}
                                    <option value={name}>{name}</option>
                                {/each}
                            </select>
                        </label>
                        <label class="text-fg-secondary text-[12px]">
                            Dark blueprint
                            <select
                                class="bg-bg-primary border-border text-fg-primary mt-1 w-full border px-2 py-1.5 text-[12px]"
                                bind:value={scheduler.dark_blueprint}
                            >
                                <option value="">None</option>
                                {#each blueprintNames as name}
                                    <option value={name}>{name}</option>
                                {/each}
                            </select>
                        </label>
                    </div>
                    <div class="flex items-center justify-between gap-4">
                        <p class="text-fg-dimmed text-[11.5px]">
                            {scheduler.mode === 'schedule'
                                ? 'Following the fixed 06:00â€“18:00 schedule.'
                                : 'Following the Windows system light/dark mode.'}
                        </p>
                        <button
                            type="button"
                            class="bg-accent text-accent-fg hover:bg-accent-hover h-8 shrink-0 px-3.5 text-[12px] font-semibold transition-colors disabled:opacity-60"
                            onclick={saveScheduler}
                            disabled={schedulerBusy}
                        >
                            {schedulerBusy ? 'Savingâ€¦' : 'Save'}
                        </button>
                    </div>
                </div>
            {:else}
                <p class="text-fg-dimmed text-[12px]">
                    Scheduler settings unavailable.
                </p>
            {/if}
        </section>

        {#if isWindows}
        <section aria-labelledby="ecosystem-heading" class="mt-9">
            <h2
                id="ecosystem-heading"
                class="text-fg-dimmed mb-2.5 text-[10px] font-semibold uppercase tracking-[0.14em]"
            >
                Omarchy Ecosystem
            </h2>
            <p class="text-fg-secondary mb-4 text-[12px]">
                Waifer stores themes, user drop-in templates, and post-apply
                hooks under <code class="text-fg-primary">%LOCALAPPDATA%\Waifer\</code>.
            </p>

            <div class="grid gap-2">
                <!-- Templates folder -->
                <div
                    class="border-border bg-bg-secondary flex items-center justify-between border p-3"
                >
                    <div class="min-w-0">
                        <span class="text-fg-primary text-[13px] font-medium">
                            Templates
                        </span>
                        <p class="text-fg-dimmed mt-0.5 text-[11.5px]">
                            {aetherDirStatus?.templateCount ?? 0} active
                            {aetherDirStatus?.templateCount === 1 ? '.tpl file' : '.tpl files'}
                            {#if aetherDirStatus?.templatesDir}
                                in <code class="text-fg-secondary break-all font-mono text-[11px]">{aetherDirStatus.templatesDir}</code>
                            {/if}
                        </p>
                    </div>
                    <button
                        type="button"
                        class="text-accent hover:bg-accent-muted flex h-7 w-7 shrink-0 items-center justify-center transition-colors"
                        onclick={() => openInExplorer(aetherDirStatus?.templatesDir ?? '')}
                        title="Open templates folder"
                        aria-label="Open templates folder"
                    >
                        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
                        </svg>
                    </button>
                </div>

                <!-- Hooks folder -->
                <div
                    class="border-border bg-bg-secondary flex items-center justify-between border p-3"
                >
                    <div class="min-w-0">
                        <span class="text-fg-primary text-[13px] font-medium">
                            Hooks
                        </span>
                        <p class="text-fg-dimmed mt-0.5 text-[11.5px]">
                            {#if aetherDirStatus?.hasHookScript}
                                <span class="text-success">Active:</span> theme-set.ps1 / .bat
                            {:else}
                                No post-apply hook script â€” add
                                <code class="text-fg-secondary font-mono text-[11px]">theme-set.ps1</code>
                                or
                                <code class="text-fg-secondary font-mono text-[11px]">theme-set.bat</code>
                            {/if}
                            in <code class="text-fg-secondary break-all font-mono text-[11px]">{aetherDirStatus?.hooksDir ?? ''}</code>
                        </p>
                    </div>
                    <button
                        type="button"
                        class="text-accent hover:bg-accent-muted flex h-7 w-7 shrink-0 items-center justify-center transition-colors"
                        onclick={() => openInExplorer(aetherDirStatus?.hooksDir ?? '')}
                        title="Open hooks folder"
                        aria-label="Open hooks folder"
                    >
                        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
                        </svg>
                    </button>
                </div>

                <!-- Headless CLI command -->
                <div
                    class="border-border bg-bg-secondary flex items-center justify-between border p-3"
                >
                    <div class="min-w-0 flex-1">
                        <span class="text-fg-primary text-[13px] font-medium">
                            Headless CLI command
                        </span>
                        <p class="text-fg-dimmed mt-0.5 text-[11.5px]">
                            Use in Flow Launcher, PowerToys Run, or AutoHotkey
                            to apply the active theme.
                        </p>
                        {#if cliCommand}
                            <div
                                class="bg-bg-primary border-border text-fg-secondary mt-2 truncate border px-[11px] py-[9px] font-mono text-[12px] font-medium"
                                title={cliCommand}
                            >
                                {cliCommand}
                            </div>
                        {/if}
                    </div>
                    <button
                        type="button"
                        class="bg-accent text-accent-fg hover:bg-accent-hover ml-3 h-8 shrink-0 px-3.5 text-[12px] font-semibold transition-colors"
                        onclick={copyCLICommand}
                        title="Copy command to clipboard"
                    >
                        Copy
                    </button>
                </div>
            </div>
        </section>
        {/if}

        <section aria-labelledby="app-targets-heading" class="mt-9">
            <h2
                id="app-targets-heading"
                class="text-fg-dimmed mb-2.5 text-[10px] font-semibold uppercase tracking-[0.14em]"
            >
                App targets
            </h2>
            <p class="text-fg-secondary mb-4 text-[12px]">
                Choose which apps Waifer updates when you apply a theme.
            </p>
            <div class="grid gap-2">
                {#each appTargets as target}
                    <div
                        class="border-border bg-bg-secondary flex items-center justify-between border p-3"
                    >
                        <span class="text-fg-primary text-[13px] font-medium">
                            {target.label}
                        </span>
                        <Switch
                            checked={isAppIncluded(target.key)}
                            onchange={v => setAppIncluded(target.key, v)}
                            label={target.label}
                        />
                    </div>
                {/each}
            </div>
        </section>

        {#if isWindows}
        <section aria-labelledby="ai-harnesses-heading" class="mt-9">
            <h2
                id="ai-harnesses-heading"
                class="text-fg-dimmed mb-2.5 text-[10px] font-semibold uppercase tracking-[0.14em]"
            >
                AI Harnesses
            </h2>
            <p class="text-fg-secondary mb-4 text-[12px]">
                Theme AI agent harnesses safely. Configs are backed up and only theme keys are touched.
            </p>
            <div class="grid gap-2">
                {#each aiHarnessTargets as target}
                    <div
                        class="border-border bg-bg-secondary flex items-center justify-between border p-3"
                    >
                        <span class="text-fg-primary text-[13px] font-medium">
                            {target.label}
                        </span>
                        <Switch
                            checked={isAppIncluded(target.key)}
                            onchange={v => setAppIncluded(target.key, v)}
                            label={target.label}
                        />
                    </div>
                {/each}
            </div>
        </section>
        {/if}
    </div>
</div>
