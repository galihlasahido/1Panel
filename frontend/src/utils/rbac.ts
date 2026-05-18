// NOTE: do NOT statically import '@/api/modules/user' here. This module
// is pulled in eagerly by routers/index.ts, which main.ts imports
// before app.use(pinia). A static import would drag in the @/api axios
// singleton whose module scope calls GlobalStore() — instantiating a
// Pinia store before Pinia is active ("Cannot read properties of
// undefined (reading '_s')"). The API is lazy-imported at call time
// (runtime, post-pinia) instead.

// Canonical RBAC menu keys — MUST match core/middleware/rbac.go.
const RBAC_MENU_PATH: Record<string, string> = {
    apps: '/apps',
    website: '/websites',
    database: '/databases',
    container: '/containers',
    cron: '/cronjobs',
    host: '/hosts',
    toolbox: '/toolbox',
    ai: '/ai',
    logs: '/logs',
    security: '/security',
    settings: '/settings',
};
const RBAC_MENU_ORDER = [
    'apps',
    'website',
    'database',
    'container',
    'cron',
    'host',
    'toolbox',
    'ai',
    'logs',
    'security',
    'settings',
];
// First URL path segment -> RBAC key. Routes whose segment isn't here
// (home, error, 404, terminal, entrance, ...) are infrastructure and
// never gated client-side; the backend still enforces every API call.
const SEGMENT_KEY: Record<string, string> = {
    apps: 'apps',
    websites: 'website',
    databases: 'database',
    containers: 'container',
    cronjobs: 'cron',
    hosts: 'host',
    toolbox: 'toolbox',
    ai: 'ai',
    logs: 'logs',
    security: 'security',
    settings: 'settings',
};

export interface RbacPerms {
    isSuper: boolean;
    menus: string[];
}

let cache: RbacPerms | null = null;
let inflight: Promise<RbacPerms | null> | null = null;

export const resetRbacPerms = () => {
    cache = null;
    inflight = null;
};

// Resolve the current user's perms, fetching /core/auth/me at most once
// per session. Fails open (returns null) so a transient error never
// locks anyone — including superadmin — out of the UI.
export const loadRbacPerms = async (): Promise<RbacPerms | null> => {
    if (cache) return cache;
    if (inflight) return inflight;
    inflight = (async (): Promise<RbacPerms | null> => {
        try {
            // Lazy import — keeps @/api (module-scope GlobalStore) out of
            // the pre-pinia bootstrap path. See note at top of file.
            const { getCurrentUser } = await import('@/api/modules/user');
            const res = await getCurrentUser();
            const d: any = res?.data;
            if (d && typeof d.isSuper === 'boolean') {
                cache = { isSuper: !!d.isSuper, menus: Array.isArray(d.menus) ? d.menus : [] };
                return cache;
            }
            return null;
        } catch {
            return null;
        } finally {
            inflight = null;
        }
    })();
    return inflight;
};

export const menuKeyForPath = (path: string): string | undefined => {
    const seg = (path || '').split('/').filter(Boolean)[0];
    return seg ? SEGMENT_KEY[seg] : undefined;
};

const isUnrestricted = (perms: RbacPerms) => perms.isSuper || perms.menus.includes('*');

export const isPathAllowed = (perms: RbacPerms, path: string): boolean => {
    if (isUnrestricted(perms)) return true;
    const key = menuKeyForPath(path);
    if (!key) return true; // infrastructure route — not menu-gated
    return perms.menus.includes(key);
};

export const firstAllowedPath = (perms: RbacPerms): string | null => {
    if (isUnrestricted(perms)) return null;
    const key = RBAC_MENU_ORDER.find((k) => perms.menus.includes(k));
    return key ? RBAC_MENU_PATH[key] : null;
};
