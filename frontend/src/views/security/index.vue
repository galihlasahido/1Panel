<template>
    <div v-loading="loading">
        <LayoutContent :title="$t('menu.security')">
            <template #rightToolBar>
                <el-select
                    v-model="scopeID"
                    placeholder="All nodes (no scope)"
                    clearable
                    size="small"
                    style="width: 200px; margin-right: 8px"
                    @change="refreshAll()"
                >
                    <el-option v-for="s in scopes" :key="s.id" :label="s.name" :value="s.id" />
                </el-select>
                <TableRefresh @search="refreshAll()" />
            </template>
            <template #main>
                <el-alert type="info" :closable="false" class="common-div">
                    <template #title>
                        <span class="text-xs">
                            Aggregated security posture across every node (local + managed). Data
                            is pulled live from each node's agent; an unreachable node is shown
                            with its error rather than silently dropped.
                        </span>
                    </template>
                </el-alert>

                <el-row :gutter="12" class="mt-2">
                    <el-col :span="6">
                        <el-card shadow="never">
                            <div class="stat">
                                <div class="num">{{ data.nodesReachable }}/{{ data.nodesTotal }}</div>
                                <div class="lbl">Nodes reachable</div>
                            </div>
                        </el-card>
                    </el-col>
                    <el-col :span="6">
                        <el-card shadow="never">
                            <div class="stat">
                                <div class="num" :class="{ warn: data.totalBannedIPs > 0 }">
                                    {{ data.totalBannedIPs }}
                                </div>
                                <div class="lbl">Banned IPs (Fail2Ban)</div>
                            </div>
                        </el-card>
                    </el-col>
                    <el-col :span="6">
                        <el-card shadow="never">
                            <div class="stat">
                                <div class="num" :class="{ danger: data.totalFailedSSH > 0 }">
                                    {{ data.totalFailedSSH }}
                                </div>
                                <div class="lbl">Failed SSH logins</div>
                            </div>
                        </el-card>
                    </el-col>
                    <el-col :span="6">
                        <el-card shadow="never">
                            <div class="stat">
                                <div class="num">{{ generatedAt }}</div>
                                <div class="lbl">Generated</div>
                            </div>
                        </el-card>
                    </el-col>
                </el-row>

                <el-divider content-position="left">Per-node posture</el-divider>
                <el-table :data="data.nodes" border>
                    <el-table-column label="Node" prop="nodeName" min-width="120" />
                    <el-table-column label="Reachable" min-width="100">
                        <template #default="{ row }">
                            <el-tag :type="row.reachable ? 'success' : 'danger'">
                                {{ row.reachable ? 'Yes' : 'No' }}
                            </el-tag>
                        </template>
                    </el-table-column>
                    <el-table-column label="Fail2Ban" min-width="100">
                        <template #default="{ row }">
                            <el-tag :type="row.fail2banActive ? 'success' : 'info'">
                                {{ row.fail2banActive ? 'Active' : 'Off' }}
                            </el-tag>
                        </template>
                    </el-table-column>
                    <el-table-column label="Banned IPs" min-width="110">
                        <template #default="{ row }">
                            <el-tag v-if="row.bannedIPs.length" type="warning">
                                {{ row.bannedIPs.length }}
                            </el-tag>
                            <span v-else>0</span>
                        </template>
                    </el-table-column>
                    <el-table-column label="Failed SSH" prop="failedSSHCount" min-width="100" />
                    <el-table-column label="Listening ports" prop="listeningPorts" min-width="120" />
                    <el-table-column label="Firewall" prop="firewallStatus" min-width="110">
                        <template #default="{ row }">{{ row.firewallStatus || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="Error" min-width="160" show-overflow-tooltip>
                        <template #default="{ row }">
                            <span class="danger">{{ row.error || '' }}</span>
                        </template>
                    </el-table-column>
                </el-table>

                <el-divider content-position="left">Recent failed SSH logins</el-divider>
                <el-table :data="data.recentFailedSSH" border max-height="420">
                    <el-table-column label="Node" prop="node" min-width="100" />
                    <el-table-column label="Time" prop="dateStr" min-width="160" />
                    <el-table-column label="User" prop="user" min-width="110" />
                    <el-table-column label="From" prop="address" min-width="140" />
                    <el-table-column label="Auth" prop="authMode" min-width="100" />
                    <el-table-column label="Message" prop="message" min-width="220" show-overflow-tooltip />
                    <template #empty>No failed SSH logins reported.</template>
                </el-table>

                <el-divider content-position="left">Host activity timeline</el-divider>
                <div class="activity-toolbar">
                    <span class="lbl">Kind</span>
                    <el-select
                        v-model="kindFilter"
                        size="small"
                        style="width: 170px"
                        @change="loadActivity"
                    >
                        <el-option label="All kinds" value="" />
                        <el-option label="SSH login" value="ssh_login" />
                        <el-option label="SSH failed" value="ssh_failed" />
                        <el-option label="SSH brute-force" value="ssh_bruteforce" />
                        <el-option label="Fail2Ban ban" value="fail2ban_ban" />
                        <el-option label="Fail2Ban unban" value="fail2ban_unban" />
                        <el-option label="File change" value="file_change" />
                    </el-select>
                </div>
                <el-table :data="activity" border max-height="520">
                    <el-table-column label="Node" prop="node" min-width="90" />
                    <el-table-column label="Time" min-width="160">
                        <template #default="{ row }">{{ fmtTime(row.eventTime) }}</template>
                    </el-table-column>
                    <el-table-column label="Severity" min-width="100">
                        <template #default="{ row }">
                            <el-tag :type="sevType(row.severity)">{{ row.severity }}</el-tag>
                        </template>
                    </el-table-column>
                    <el-table-column label="Kind" prop="kind" min-width="130" />
                    <el-table-column label="Actor" prop="actor" min-width="100" />
                    <el-table-column label="Source" prop="source" min-width="130" />
                    <el-table-column label="Target" prop="target" min-width="140" show-overflow-tooltip />
                    <el-table-column label="Detail" prop="detail" min-width="220" show-overflow-tooltip />
                    <template #empty>No host activity recorded yet.</template>
                </el-table>
            </template>
        </LayoutContent>
    </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { getSecurityOverview, getSecurityActivity } from '@/api/modules/security';
import { listScopes } from '@/api/modules/node';
import { Security } from '@/api/interface/security';
import { NodeMgmt } from '@/api/interface/node';
import { MsgError } from '@/utils/message';

const scopes = ref<NodeMgmt.NodeScope[]>([]);
const scopeID = ref<number>(0);
const loadScopes = async () => {
    try {
        const res = await listScopes();
        scopes.value = res.data || [];
    } catch {
        scopes.value = [];
    }
};

const activity = ref<Security.ActivityEntry[]>([]);
const kindFilter = ref('');

const fmtTime = (t: string) => (t ? new Date(t).toLocaleString() : '-');
const sevType = (s: string) => (s === 'crit' ? 'danger' : s === 'warn' ? 'warning' : 'info');

const loadActivity = async () => {
    try {
        const res = await getSecurityActivity(200, kindFilter.value, scopeID.value || 0);
        activity.value = res.data || [];
    } catch (e: any) {
        MsgError(e?.message || 'Failed to load activity timeline');
    }
};

const loading = ref(false);
const generatedAt = ref('-');
const data = reactive<Security.Overview>({
    generatedAt: '',
    nodesTotal: 0,
    nodesReachable: 0,
    totalBannedIPs: 0,
    totalFailedSSH: 0,
    nodes: [],
    recentFailedSSH: [],
});

const load = async () => {
    loading.value = true;
    try {
        const res = await getSecurityOverview(scopeID.value || 0);
        Object.assign(data, res.data);
        generatedAt.value = res.data.generatedAt
            ? new Date(res.data.generatedAt).toLocaleTimeString()
            : '-';
    } catch (e: any) {
        MsgError(e?.message || 'Failed to load security overview');
    } finally {
        loading.value = false;
    }
};

const refreshAll = () => {
    loadScopes();
    load();
    loadActivity();
};

onMounted(refreshAll);
</script>

<style scoped lang="scss">
.stat {
    text-align: center;
    padding: 6px 0;
    .num {
        font-size: 22px;
        font-weight: 600;
        &.warn {
            color: var(--el-color-warning);
        }
        &.danger {
            color: var(--el-color-danger);
        }
    }
    .lbl {
        font-size: 12px;
        color: var(--el-text-color-secondary);
        margin-top: 4px;
    }
}
.danger {
    color: var(--el-color-danger);
}
.activity-toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 8px 0 12px;
    .lbl {
        font-size: 12px;
        color: var(--el-text-color-secondary);
    }
}
</style>
