<script lang="ts">
    import {
        setWallpaperPath,
        addAdditionalImage,
        getAdditionalImages,
    } from '$lib/stores/theme.svelte';
    import {setActiveTab, showToast} from '$lib/stores/ui.svelte';
    import {applyWallpaperOnly, extractColors} from '$lib/actions/themeActions';
    import {getIsApplying} from '$lib/stores/theme.svelte';
    import {openURL} from '$lib/utils/browser';
    import {observeIntersection} from '$lib/utils/intersection';
    import {isFavorite, toggleFavorite} from '$lib/stores/favorites.svelte';
    import {formatFileSize} from '$lib/utils/format';
    import WallpaperTile from '$lib/components/shared/WallpaperTile.svelte';

    let {wallpaper, onpreview}: {wallpaper: any; onpreview: () => void} =
        $props();
    let isDownloading = $state(false);
    let favoriteKey = $derived(wallpaper.path || wallpaper.id);
    let isFavorited = $derived(isFavorite(favoriteKey));
    let cardEl = $state<HTMLDivElement | null>(null);
    let inView = $state(false);

    // Gate <img> on viewport so scrolled-past cards release decoded-image
    // memory.
    $effect(() => {
        if (!cardEl) return;
        return observeIntersection(
            cardEl,
            entry => {
                inView = entry.isIntersecting;
            },
            {rootMargin: '600px 0px'}
        );
    });

    async function handleUse() {
        if (isDownloading) return;
        isDownloading = true;
        showToast('Downloading wallpaper…');
        try {
            // Full-res data[].path saved into Pictures\Wallpapers as
            // wallhaven-<id>.<ext>; existing files are reused.
            const {DownloadWallhaven} = await import(
                '../../../../wailsjs/go/main/App'
            );
            const localPath = await DownloadWallhaven(
                wallpaper.id,
                wallpaper.path
            );
            setWallpaperPath(localPath);
            setActiveTab('editor');
            // Auto-populate the 16-color palette from the new wallpaper.
            await extractColors();
        } catch (e: unknown) {
            const detail =
                typeof e === 'string'
                    ? e
                    : e instanceof Error
                      ? e.message
                      : '';
            showToast(
                detail
                    ? `Failed to download wallpaper: ${detail}`
                    : 'Failed to download wallpaper'
            );
        } finally {
            isDownloading = false;
        }
    }

    async function handleFavorite() {
        try {
            await toggleFavorite(favoriteKey, 'wallhaven', {
                id: wallpaper.id,
                thumbUrl: wallpaper.thumbs?.small,
                resolution: wallpaper.resolution,
            });
        } catch (err) {
            console.error('ToggleFavorite failed', err);
            showToast('Could not update favorites');
        }
    }

    async function handleAddExtra() {
        try {
            showToast('Downloading wallpaper...');
            const {DownloadWallpaper} = await import(
                '../../../../wailsjs/go/main/App'
            );
            const localPath = await DownloadWallpaper(wallpaper.path);
            if (getAdditionalImages().includes(localPath)) {
                showToast('Already in additional images');
                return;
            }
            showToast(
                addAdditionalImage(localPath)
                    ? 'Added to additional images'
                    : 'Skipped: the theme already has a wallpaper with that filename'
            );
        } catch {
            showToast('Failed to download wallpaper');
        }
    }

    function handleVisit() {
        openURL(`https://wallhaven.cc/w/${wallpaper.id}`);
    }
</script>

<div bind:this={cardEl}>
    <WallpaperTile
        path={wallpaper.path}
        name={wallpaper.id}
        {isFavorited}
        busy={isDownloading}
        applying={getIsApplying()}
        useLabel={isDownloading ? 'Loading…' : 'Use'}
        useTitle="Download to Pictures\Wallpapers, set as wallpaper, and extract palette"
        visitTitle="Open on wallhaven.cc"
        onuse={handleUse}
        onwallpaperonly={() => applyWallpaperOnly(wallpaper.path)}
        {onpreview}
        onaddextra={handleAddExtra}
        onfavorite={handleFavorite}
        onvisit={handleVisit}
    >
        {#snippet thumb()}
            {#if inView}
                <img
                    src={wallpaper.thumbs?.large ||
                        wallpaper.thumbs?.original ||
                        wallpaper.thumbs?.small}
                    alt={wallpaper.id}
                    class="h-full w-full object-cover"
                    loading="lazy"
                />
            {/if}
        {/snippet}
        {#snippet meta()}
            <span class="text-fg-secondary flex-1 truncate font-mono"
                >{wallpaper.resolution?.replace('x', ' × ')}</span
            >
            <span class="text-fg-dimmed shrink-0"
                >{formatFileSize(wallpaper.file_size)}</span
            >
        {/snippet}
    </WallpaperTile>
</div>
