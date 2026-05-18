import http from '@/api';
import { Security } from '../interface/security';

export const getSecurityOverview = (scope = 0) => {
    return http.get<Security.Overview>(`/core/security/overview`, scope ? { scope } : {});
};

export const getSecurityActivity = (limit = 200, kind = '', scope = 0) => {
    const params: any = { limit, kind };
    if (scope) params.scope = scope;
    return http.get<Security.ActivityEntry[]>(`/core/security/activity`, params);
};
