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
