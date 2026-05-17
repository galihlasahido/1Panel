import http from '@/api';
import { Security } from '../interface/security';

export const getSecurityOverview = () => {
    return http.get<Security.Overview>(`/core/security/overview`);
};

export const getSecurityActivity = (limit = 200, kind = '') => {
    return http.get<Security.ActivityEntry[]>(`/core/security/activity`, { limit, kind });
};
