import http from '@/api';
import { ResPage } from '../interface';
import { NodeMgmt } from '../interface/node';
import { TimeoutEnum } from '@/enums/http-enum';

export const searchNodes = (params: NodeMgmt.NodeSearch) => {
    return http.post<ResPage<NodeMgmt.NodeInfo>>(`/core/nodes/search`, params);
};

// Server-paginated typeahead for the node picker — never loads the
// whole fleet (scales to thousands of nodes).
export const searchNodeOptions = (params: NodeMgmt.NodeSearch) => {
    return http.post<ResPage<NodeMgmt.NodeOption>>(`/core/nodes/options`, params);
};

export const getNode = (id: number) => {
    return http.get<NodeMgmt.NodeInfo>(`/core/nodes/${id}`);
};

export const addNode = (params: NodeMgmt.NodeCreate) => {
    return http.post<NodeMgmt.NodeInfo>(`/core/nodes/add`, params, TimeoutEnum.T_5M);
};

export const updateNode = (params: NodeMgmt.NodeUpdate) => {
    return http.post(`/core/nodes/update`, params);
};

export const deleteNode = (id: number) => {
    return http.post(`/core/nodes/del/${id}`, {});
};

export const recheckNode = (id: number) => {
    return http.post<NodeMgmt.NodeInfo>(`/core/nodes/healthcheck/${id}`, {});
};

export const getNodeLabels = (id: number) => {
    return http.get<NodeMgmt.NodeLabel[]>(`/core/nodes/labels/get/${id}`);
};

export const setNodeLabels = (nodeID: number, labels: NodeMgmt.NodeLabel[]) => {
    return http.post(`/core/nodes/labels/set`, { nodeID, labels });
};

export const bulkNodeLabel = (nodeIDs: number[], key: string, value: string, op: 'add' | 'remove') => {
    return http.post(`/core/nodes/labels/bulk`, { nodeIDs, key, value, op });
};

export const nodeLabelKeys = () => {
    return http.get<string[]>(`/core/nodes/labels/keys`);
};

export const nodeLabelValues = (key: string) => {
    return http.post<string[]>(`/core/nodes/labels/values`, { key });
};

export const nodeStats = (params: NodeMgmt.NodeSearch) => {
    return http.post<NodeMgmt.NodeStats>(`/core/nodes/stats`, params);
};

export const listScopes = () => {
    return http.get<NodeMgmt.NodeScope[]>(`/core/scopes/search`);
};

export const createScope = (params: { name: string; labels: string[]; status: string; description: string }) => {
    return http.post(`/core/scopes/create`, params);
};

export const updateScope = (params: {
    id: number;
    labels: string[];
    status: string;
    description: string;
}) => {
    return http.post(`/core/scopes/update`, params);
};

export const deleteScope = (id: number) => {
    return http.post(`/core/scopes/del/${id}`, {});
};

export const bulkNodeOp = (params: {
    scopeID?: number;
    labels?: string[];
    nodeIDs?: number[];
    action: 'healthcheck' | 'security-collect';
}) => {
    return http.post<{
        total: number;
        succeeded: number;
        failed: number;
        results: { node: string; ok: boolean; message: string }[];
    }>(`/core/nodes/bulk`, params);
};
