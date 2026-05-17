import { getCurrentUser } from '@/api/modules/user';

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
    inflight = getCurrentUser()
        .then((res) => {
            const d: any = res?.data;
            if (d && typeof d.isSuper === 'boolean') {
                cache = { isSuper: !!d.isSuper, menus: Array.isArray(d.menus) ? d.menus : [] };
                return cache;
            }
            return null;
        })
        .catch(() => null)
        .finally(() => {
            inflight = null;
        });
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
