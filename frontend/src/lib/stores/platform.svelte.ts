let isWindows = $state(false);
let isMac = $state(false);
let loaded = $state(false);

loadPlatform();

async function loadPlatform() {
    try {
        const {IsWindows, IsMacOS} = await import('../../../wailsjs/go/main/App');
        const [win, mac] = await Promise.all([IsWindows(), IsMacOS()]);
        isWindows = win;
        isMac = mac;
    } catch {
        // Running in a plain browser (vite dev without Wails): assume other.
    } finally {
        loaded = true;
    }
}

export function getIsWindows(): boolean {
    return isWindows;
}

export function getIsMac(): boolean {
    return isMac;
}

export function getPlatformLoaded(): boolean {
    return loaded;
}
