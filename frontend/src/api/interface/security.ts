export namespace Security {
    export interface SSHEntry {
        node: string;
        dateStr: string;
        user: string;
        address: string;
        authMode: string;
        message: string;
    }
    export interface NodeSummary {
        nodeName: string;
        reachable: boolean;
        error?: string;
        fail2banActive: boolean;
        bannedIPs: string[];
        failedSSHCount: number;
        listeningPorts: number;
        firewallStatus: string;
    }
    export interface Overview {
        generatedAt: string;
        nodesTotal: number;
        nodesReachable: number;
        totalBannedIPs: number;
        totalFailedSSH: number;
        nodes: NodeSummary[];
        recentFailedSSH: SSHEntry[];
    }
}
