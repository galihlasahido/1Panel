<template>
    <div v-loading="loading">
        <LayoutContent :title="$t('menu.security')">
            <template #rightToolBar>
                <TableRefresh @search="load()" />
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
            </template>
        </LayoutContent>
    </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { getSecurityOverview } from '@/api/modules/security';
import { Security } from '@/api/interface/security';
import { MsgError } from '@/utils/message';

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
        const res = await getSecurityOverview();
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

onMounted(load);
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
</style>
