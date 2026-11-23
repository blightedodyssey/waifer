import {showToast} from '$lib/stores/ui.svelte';
import {
    setPalette,
    setExtendedColors,
    setNativeColors,
    setIconTheme,
    setWallpaperPath,
    setWallpaperBlur,
    setLightMode,
} from '$lib/stores/theme.svelte';

export type ImportFileType = 'base16' | 'base24' | 'pywal' | 'toml' | 'blueprint';

// Opens a file dialog and loads the chosen theme file into the editor.
// Nothing is applied. Returns true when the editor received new colors.
export async function importThemeFile(
    fileType: ImportFileType
): Promise<boolean> {
    try {
        const {ImportFileDialog} = await import('../../../wailsjs/go/main/App');
        const result = await ImportFileDialog(fileType);
        if (!(result?.colors?.length >= 16)) {
            showToast('Import returned no colors');
            return false;
        }
        setPalette(result.colors);
        setExtendedColors(result.extendedColors ?? {});
        setNativeColors(result.nativeColors ?? {});
        setIconTheme(result.iconTheme, true);
        if (result.wallpaperPath) setWallpaperPath(result.wallpaperPath);
        setWallpaperBlur(!!result.wallpaperBlur, true);
        if (result.lightMode !== undefined) setLightMode(result.lightMode);
        showToast(`Imported: ${result.name || fileType}`);
        return true;
    } catch (e: any) {
        // The dialog reports a cancel as an error. That is not a failure.
        if (e?.message?.includes('cancelled')) return false;
        console.error('Import error:', e);
        showToast('Import failed: ' + (e?.message || JSON.stringify(e)));
        return false;
    }
}

// Opens a save dialog and writes the current theme (colors.toml, wallpaper,
// custom.css) as a portable .waifer bundle zip.
export async function exportWaiferBundle(): Promise<boolean> {
    try {
        const {BundleExportDialog} = await import(
            '../../../wailsjs/go/main/App'
        );
        const path = await BundleExportDialog();
        if (!path) return false;
        showToast(`Bundle exported to ${path}`);
        return true;
    } catch (e: any) {
        // The dialog reports a cancel as an error. That is not a failure.
        if (e?.message?.includes('cancelled')) return false;
        console.error('Bundle export error:', e);
        showToast(
            'Bundle export failed: ' + (e?.message || JSON.stringify(e))
        );
        return false;
    }
}

// Opens a file dialog, extracts a .waifer bundle and loads its colors,
// wallpaper and settings into the editor. Nothing is applied.
export async function importWaiferBundle(): Promise<boolean> {
    try {
        const {BundleImportDialog} = await import(
            '../../../wailsjs/go/main/App'
        );
        const result = await BundleImportDialog();
        if (!(result?.colors?.length >= 16)) {
            showToast('Bundle import returned no colors');
            return false;
        }
        setPalette(result.colors);
        setExtendedColors(result.extendedColors ?? {});
        setNativeColors(result.nativeColors ?? {});
        setIconTheme(result.iconTheme, true);
        if (result.wallpaperPath) setWallpaperPath(result.wallpaperPath);
        setWallpaperBlur(!!result.wallpaperBlur, true);
        if (result.lightMode !== undefined) setLightMode(result.lightMode);
        showToast(`Imported bundle: ${result.name || 'waifer'}`);
        return true;
    } catch (e: any) {
        // The dialog reports a cancel as an error. That is not a failure.
        if (e?.message?.includes('cancelled')) return false;
        console.error('Bundle import error:', e);
        showToast(
            'Bundle import failed: ' + (e?.message || JSON.stringify(e))
        );
        return false;
    }
}
