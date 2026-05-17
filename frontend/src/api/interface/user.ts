import { ReqPage } from '.';

export namespace UserMgmt {
    export interface UserInfo {
        id: number;
        name: string;
        status: string;
        language: string;
        menus: string[];
        nodes: string[];
        description: string;
        createdAt: string;
    }
    export interface UserSearch extends ReqPage {
        info?: string;
        status?: string;
    }
    export interface UserCreate {
        name: string;
        password: string;
        status: string;
        language: string;
        menus: string[];
        nodes: string[];
        description: string;
    }
    export interface UserUpdate {
        id: number;
        status: string;
        language: string;
        menus: string[];
        nodes: string[];
        description: string;
    }
}

// Canonical RBAC menu keys — MUST match core/middleware/rbac.go.
export const RBAC_MENU_KEYS = [
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
