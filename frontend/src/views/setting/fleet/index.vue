<template>
    <div v-loading="loading">
        <LayoutContent title="Fleet">
            <template #rightToolBar>
                <TableRefresh @search="refreshAll()" />
            </template>
            <template #main>
                <el-alert type="info" :closable="false" class="common-div">
                    <template #title>
                        <span class="text-xs">
                            Fleet-wide view across every node. Filter by status, free text, or
                            <code>key=value</code> labels (AND); save a filter as a named scope to
                            reuse it for scoped fan-out and bulk operations.
                        </span>
                    </template>
                </el-alert>

                <el-row :gutter="12" class="mt-2">
                    <el-col :span="5">
                        <el-card shadow="never">
                            <div class="stat"><div class="num">{{ stats.total }}</div><div class="lbl">Total</div></div>
                        </el-card>
                    </el-col>
                    <el-col :span="5">
                        <el-card shadow="never">
                            <div class="stat"><div class="num ok">{{ stats.healthy }}</div><div class="lbl">Healthy</div></div>
                        </el-card>
                    </el-col>
                    <el-col :span="5">
                        <el-card shadow="never">
                            <div class="stat"><div class="num bad">{{ stats.unhealthy }}</div><div class="lbl">Unhealthy</div></div>
                        </el-card>
                    </el-col>
                    <el-col :span="5">
                        <el-card shadow="never">
                            <div class="stat"><div class="num warn">{{ stats.pending }}</div><div class="lbl">Pending</div></div>
                        </el-card>
                    </el-col>
                    <el-col :span="4">
                        <el-card shadow="never">
                            <div class="stat"><div class="num">{{ stats.other }}</div><div class="lbl">Other</div></div>
                        </el-card>
                    </el-col>
                </el-row>

                <div class="toolbar">
                    <el-select
                        v-model="activeScope"
                        placeholder="Saved scope"
                        clearable
                        style="width: 200px"
                        @change="applyScope"
                    >
                        <el-option v-for="s in scopes" :key="s.id" :label="s.name" :value="s.id" />
                    </el-select>
                    <el-select v-model="filterStatus" placeholder="Status" clearable style="width: 150px" @change="reload">
                        <el-option label="Healthy" value="Healthy" />
                        <el-option label="Unhealthy" value="Unhealthy" />
                        <el-option label="Pending" value="Pending" />
                        <el-option label="VersionMismatch" value="VersionMismatch" />
                    </el-select>
                    <el-input
                        v-model="filterLabels"
                        clearable
                        style="width: 240px"
                        placeholder="labels e.g. env=prod,role=db"
                        @clear="reload"
                        @keyup.enter="reload"
                    />
                    <el-input
                        v-model="filterText"
                        clearable
                        style="width: 180px"
                        placeholder="name / addr"
                        @clear="reload"
                        @keyup.enter="reload"
                    />
                    <el-button type="primary" @click="reload">Apply</el-button>
                    <el-button @click="openSaveScope">Save as scope</el-button>
                    <el-button v-if="activeScope" type="danger" plain @click="removeScope">Delete scope</el-button>
                </div>

                <ComplexTable :pagination-config="paginationConfig" @search="reload" :data="data">
                    <el-table-column :label="$t('commons.table.name')" min-width="120" prop="name" show-overflow-tooltip />
                    <el-table-column label="Address" min-width="150">
                        <template #default="{ row }">{{ row.addr }}:{{ row.port }}</template>
                    </el-table-column>
                    <el-table-column :label="$t('commons.table.status')" min-width="110">
                        <template #default="{ row }">
                            <el-tag :type="statusType(row.status)">{{ row.status }}</el-tag>
                        </template>
                    </el-table-column>
                    <el-table-column label="Version" min-width="90" prop="version" />
                    <el-table-column label="Labels" min-width="200">
                        <template #default="{ row }">
                            <el-tag v-for="l in row.labels || []" :key="l.key" size="small" class="mr-1 mb-1">
                                {{ l.key }}={{ l.value }}
                            </el-tag>
                            <span v-if="!(row.labels && row.labels.length)">-</span>
                        </template>
                    </el-table-column>
                </ComplexTable>
            </template>
        </LayoutContent>

        <el-dialog v-model="saveOpen" title="Save scope" width="32%" :close-on-click-modal="false">
            <el-form :model="saveForm" label-position="top">
                <el-form-item label="Name">
                    <el-input v-model="saveForm.name" placeholder="e.g. prod-db" />
                </el-form-item>
                <el-form-item label="Description">
                    <el-input v-model="saveForm.description" type="textarea" :rows="2" />
                </el-form-item>
                <span class="text-xs">
                    Captures the current filters: status="{{ filterStatus || 'any' }}",
                    labels=[{{ parseLabels().join(', ') || 'none' }}].
                </span>
            </el-form>
            <template #footer>
                <el-button @click="saveOpen = false">{{ $t('commons.button.cancel') }}</el-button>
                <el-button type="primary" :loading="saving" @click="doSaveScope">
                    {{ $t('commons.button.confirm') }}
                </el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessageBox } from 'element-plus';
import { searchNodes, nodeStats, listScopes, createScope, deleteScope } from '@/api/modules/node';
import { NodeMgmt } from '@/api/interface/node';
import { MsgSuccess, MsgError } from '@/utils/message';

const loading = ref(false);
const data = ref<NodeMgmt.NodeInfo[]>([]);
const scopes = ref<NodeMgmt.NodeScope[]>([]);
const activeScope = ref<number | undefined>(undefined);
const filterStatus = ref('');
const filterLabels = ref('');
const filterText = ref('');
const stats = reactive<NodeMgmt.NodeStats>({ total: 0, healthy: 0, unhealthy: 0, pending: 0, other: 0 });

const paginationConfig = reactive({
    cacheSizeKey: 'fleet-page-size',
    currentPage: 1,
    pageSize: Number(localStorage.getItem('fleet-page-size')) || 10,
    total: 0,
    info: '',
});

const statusType = (s: string) => {
    switch (s) {
        case 'Healthy':
            return 'success';
        case 'Unhealthy':
            return 'danger';
        case 'VersionMismatch':
            return 'warning';
        default:
            return 'info';
    }
};

const parseLabels = (): string[] =>
    filterLabels.value
        .split(',')
        .map((s) => s.trim())
        .filter((s) => s.includes('='));

const buildReq = (): NodeMgmt.NodeSearch => ({
    page: paginationConfig.currentPage,
    pageSize: paginationConfig.pageSize,
    info: filterText.value,
    status: filterStatus.value,
    labels: parseLabels(),
});

const reload = async () => {
    loading.value = true;
    try {
        const [nodesRes, statsRes] = await Promise.all([searchNodes(buildReq()), nodeStats(buildReq())]);
        data.value = nodesRes.data.items || [];
        paginationConfig.total = nodesRes.data.total;
        Object.assign(stats, statsRes.data);
    } catch (e: any) {
        MsgError(e?.message || 'Failed to load fleet');
    } finally {
        loading.value = false;
    }
};

const loadScopes = async () => {
    try {
        const res = await listScopes();
        scopes.value = res.data || [];
    } catch {
        scopes.value = [];
    }
};

const applyScope = (id: number | undefined) => {
    const s = scopes.value.find((x) => x.id === id);
    if (!s) {
        return;
    }
    filterStatus.value = s.status || '';
    filterLabels.value = (s.labels || []).join(',');
    reload();
};

const refreshAll = () => {
    loadScopes();
    reload();
};

const saveOpen = ref(false);
const saving = ref(false);
const saveForm = reactive({ name: '', description: '' });
const openSaveScope = () => {
    saveForm.name = '';
    saveForm.description = '';
    saveOpen.value = true;
};
const doSaveScope = async () => {
    if (!saveForm.name.trim()) {
        MsgError('Scope name is required');
        return;
    }
    saving.value = true;
    try {
        await createScope({
            name: saveForm.name.trim(),
            labels: parseLabels(),
            status: filterStatus.value,
            description: saveForm.description,
        });
        MsgSuccess('Scope saved');
        saveOpen.value = false;
        await loadScopes();
    } catch (e: any) {
        MsgError(e?.response?.data?.message || e?.message || 'Save failed');
    } finally {
        saving.value = false;
    }
};

const removeScope = async () => {
    if (!activeScope.value) return;
    try {
        await ElMessageBox.confirm('Delete this saved scope?', 'Delete', { type: 'warning' });
    } catch {
        return;
    }
    try {
        await deleteScope(activeScope.value);
        activeScope.value = undefined;
        MsgSuccess('Deleted');
        await loadScopes();
    } catch (e: any) {
        MsgError(e?.message || 'Delete failed');
    }
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
        &.ok {
            color: var(--el-color-success);
        }
        &.bad {
            color: var(--el-color-danger);
        }
        &.warn {
            color: var(--el-color-warning);
        }
    }
    .lbl {
        font-size: 12px;
        color: var(--el-text-color-secondary);
        margin-top: 4px;
    }
}
.toolbar {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    margin: 12px 0;
}
.mr-1 {
    margin-right: 4px;
}
.mb-1 {
    margin-bottom: 4px;
}
</style>
