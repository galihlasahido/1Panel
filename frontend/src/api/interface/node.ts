import { ReqPage } from '.';

export namespace NodeMgmt {
    export interface NodeInfo {
        id: number;
        name: string;
        addr: string;
        port: number;
        scope: string;
        status: string;
        version: string;
        groupID: number;
        lastCheck: string | null;
        lastMessage: string;
        description: string;
        createdAt: string;
    }

    export interface NodeSearch extends ReqPage {
        info?: string;
        status?: string;
        groupID?: number;
    }

    export interface NodeOption {
        id: number;
        name: string;
        addr: string;
        status: string;
        version: string;
        isXpack: boolean;
        isBound: boolean;
    }

    export interface NodeCreate {
        name: string;
        addr: string;
        port: number;
        groupID?: number;
        description?: string;
        sshUser: string;
        sshPort: number;
        sshPassword?: string;
        sshPrivateKey?: string;
        sshPassPhrase?: string;
    }

    export interface NodeUpdate {
        id: number;
        groupID: number;
        description: string;
    }
}
