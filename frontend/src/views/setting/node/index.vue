<template>
    <div v-loading="loading">
        <LayoutContent :title="$t('setting.nodes')">
            <template #leftToolBar>
                <el-button type="primary" @click="openAdd">
                    {{ $t('commons.button.add') }}
                </el-button>
                <el-button :disabled="selected.length === 0" @click="openBulk">
                    Bulk label ({{ selected.length }})
                </el-button>
            </template>
            <template #rightToolBar>
                <el-input
                    v-model="labelFilter"
                    size="small"
                    clearable
                    style="width: 220px; margin-right: 8px"
                    placeholder="label filter e.g. env=prod,role=db"
                    @clear="search()"
                    @keyup.enter="search()"
                />
                <TableSearch @search="search()" v-model:searchName="paginationConfig.info" />
                <TableRefresh @search="search()" />
            </template>
            <template #main>
                <el-alert type="info" :closable="false" class="common-div">
                    <template #title>
                        <span class="text-xs">
                            Install <code>1panel-agent</code> on the target host first, then add it
                            here over SSH — the master issues an mTLS cert bundle and proxies the
                            panel to that node. Agent auto-install and SSH host-key pinning are not
                            wired in this build.
                        </span>
                    </template>
                </el-alert>

                <ComplexTable
                    :pagination-config="paginationConfig"
                    v-model:selects="selected"
                    @sort-change="search"
                    @search="search"
                    :data="data"
                >
                    <el-table-column type="selection" fix />
                    <el-table-column :label="$t('commons.table.name')" min-width="120" prop="name" show-overflow-tooltip />
                    <el-table-column label="Address" min-width="160" prop="addr">
                        <template #default="{ row }">{{ row.addr }}:{{ row.port }}</template>
                    </el-table-column>
                    <el-table-column label="Scope" min-width="90" prop="scope">
                        <template #default="{ row }">
                            <el-tag>{{ row.scope || 'slave' }}</el-tag>
                        </template>
                    </el-table-column>
                    <el-table-column :label="$t('commons.table.status')" min-width="120" prop="status">
                        <template #default="{ row }">
                            <el-tag :type="statusType(row.status)">{{ row.status }}</el-tag>
                        </template>
                    </el-table-column>
                    <el-table-column label="Version" min-width="100" prop="version" show-overflow-tooltip />
                    <el-table-column label="Last check" min-width="160" prop="lastCheck">
                        <template #default="{ row }">
                            {{ row.lastCheck ? dateFormatSimple(row.lastCheck) : '-' }}
                        </template>
                    </el-table-column>
                    <el-table-column label="Message" min-width="160" prop="lastMessage" show-overflow-tooltip>
                        <template #default="{ row }">{{ row.lastMessage || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="Labels" min-width="180">
                        <template #default="{ row }">
                            <el-tag
                                v-for="l in row.labels || []"
                                :key="l.key"
                                size="small"
                                class="mr-1 mb-1"
                            >
                                {{ l.key }}={{ l.value }}
                            </el-tag>
                            <span v-if="!(row.labels && row.labels.length)">-</span>
                        </template>
                    </el-table-column>
                    <el-table-column :label="$t('commons.table.operate')" width="260" fixed="right">
                        <template #default="{ row }">
                            <el-button link type="primary" @click="openLabels(row)">Labels</el-button>
                            <el-button link type="primary" :loading="row._checking" @click="recheck(row)">
                                Recheck
                            </el-button>
                            <el-button link type="danger" @click="remove(row)">
                                {{ $t('commons.button.delete') }}
                            </el-button>
                        </template>
                    </el-table-column>
                </ComplexTable>
            </template>
        </LayoutContent>

        <el-dialog v-model="addOpen" title="Add node" width="40%" :close-on-click-modal="false">
            <el-form ref="formRef" :model="form" label-position="top" :rules="rules">
                <el-form-item label="Name" prop="name">
                    <el-input v-model="form.name" placeholder="unique node name (used as the cert CN)" />
                </el-form-item>
                <el-form-item label="Address" prop="addr">
                    <el-input v-model="form.addr" placeholder="reachable IP / host of the agent" />
                </el-form-item>
                <el-form-item label="Agent port" prop="port">
                    <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" />
                </el-form-item>
                <el-form-item label="SSH user" prop="sshUser">
                    <el-input v-model="form.sshUser" placeholder="root" />
                </el-form-item>
                <el-form-item label="SSH port" prop="sshPort">
                    <el-input-number v-model="form.sshPort" :min="1" :max="65535" controls-position="right" />
                </el-form-item>
                <el-form-item label="SSH auth">
                    <el-radio-group v-model="authMode">
                        <el-radio label="password">Password</el-radio>
                        <el-radio label="key">Private key</el-radio>
                    </el-radio-group>
                </el-form-item>
                <el-form-item v-if="authMode === 'password'" label="SSH password" prop="sshPassword">
                    <el-input v-model="form.sshPassword" type="password" show-password />
                </el-form-item>
                <template v-else>
                    <el-form-item label="Private key" prop="sshPrivateKey">
                        <el-input v-model="form.sshPrivateKey" type="textarea" :rows="4"
                            placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" />
                    </el-form-item>
                    <el-form-item label="Key passphrase">
                        <el-input v-model="form.sshPassPhrase" type="password" show-password />
                    </el-form-item>
                </template>
                <el-form-item label="Description">
                    <el-input v-model="form.description" type="textarea" :rows="2" />
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button @click="addOpen = false">{{ $t('commons.button.cancel') }}</el-button>
                <el-button type="primary" :loading="adding" @click="submit">
                    {{ $t('commons.button.confirm') }}
                </el-button>
            </template>
        </el-dialog>

        <el-dialog v-model="labelsOpen" title="Node labels" width="38%" :close-on-click-modal="false">
            <div v-for="(l, i) in labelRows" :key="i" class="label-row">
                <el-input v-model="l.key" placeholder="key (e.g. env)" style="width: 40%" />
                <span class="eq">=</span>
                <el-input v-model="l.value" placeholder="value (e.g. prod)" style="width: 40%" />
                <el-button link type="danger" @click="removeLabelRow(i)">
                    {{ $t('commons.button.delete') }}
                </el-button>
            </div>
            <el-button link type="primary" @click="addLabelRow">+ Add label</el-button>
            <template #footer>
                <el-button @click="labelsOpen = false">{{ $t('commons.button.cancel') }}</el-button>
                <el-button type="primary" :loading="labelsSaving" @click="saveLabels">
                    {{ $t('commons.button.confirm') }}
                </el-button>
            </template>
        </el-dialog>

        <el-dialog v-model="bulkOpen" title="Bulk label" width="34%" :close-on-click-modal="false">
            <el-form :model="bulkForm" label-position="top">
                <el-form-item label="Operation">
                    <el-radio-group v-model="bulkForm.op">
                        <el-radio label="add">Add / set</el-radio>
                        <el-radio label="remove">Remove key</el-radio>
                    </el-radio-group>
                </el-form-item>
                <el-form-item label="Key">
                    <el-input v-model="bulkForm.key" placeholder="env" />
                </el-form-item>
                <el-form-item v-if="bulkForm.op === 'add'" label="Value">
                    <el-input v-model="bulkForm.value" placeholder="prod" />
                </el-form-item>
                <span class="text-xs">
                    Applies to {{ selected.length }} selected node(s).
                </span>
            </el-form>
            <template #footer>
                <el-button @click="bulkOpen = false">{{ $t('commons.button.cancel') }}</el-button>
                <el-button type="primary" :loading="bulkSaving" @click="applyBulk">
                    {{ $t('commons.button.confirm') }}
                </el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import type { FormInstance } from 'element-plus';
import { ElMessageBox } from 'element-plus';
import { searchNodes, addNode, deleteNode, recheckNode, setNodeLabels, bulkNodeLabel } from '@/api/modules/node';
import { NodeMgmt } from '@/api/interface/node';
import { dateFormatSimple } from '@/utils/date';
import { MsgSuccess, MsgError } from '@/utils/message';

const loading = ref(false);
const data = ref<NodeMgmt.NodeInfo[]>([]);
const paginationConfig = reactive({
    cacheSizeKey: 'node-page-size',
    currentPage: 1,
    pageSize: Number(localStorage.getItem('node-page-size')) || 10,
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

const labelFilter = ref('');
const selected = ref<NodeMgmt.NodeInfo[]>([]);

const parseLabelFilter = (): string[] =>
    labelFilter.value
        .split(',')
        .map((s) => s.trim())
        .filter((s) => s.includes('='));

const search = async () => {
    loading.value = true;
    try {
        const res = await searchNodes({
            page: paginationConfig.currentPage,
            pageSize: paginationConfig.pageSize,
            info: paginationConfig.info,
            labels: parseLabelFilter(),
        });
        data.value = res.data.items || [];
        paginationConfig.total = res.data.total;
    } catch (e: any) {
        MsgError(e?.message || 'Failed to load nodes');
    } finally {
        loading.value = false;
    }
};

// --- per-node label editor ---
const labelsOpen = ref(false);
const labelsSaving = ref(false);
const labelsNodeID = ref(0);
const labelRows = ref<NodeMgmt.NodeLabel[]>([]);
const openLabels = (row: NodeMgmt.NodeInfo) => {
    labelsNodeID.value = row.id;
    labelRows.value = (row.labels || []).map((l) => ({ ...l }));
    labelsOpen.value = true;
};
const addLabelRow = () => labelRows.value.push({ key: '', value: '' });
const removeLabelRow = (i: number) => labelRows.value.splice(i, 1);
const saveLabels = async () => {
    labelsSaving.value = true;
    try {
        const clean = labelRows.value.filter((l) => l.key.trim() !== '');
        await setNodeLabels(labelsNodeID.value, clean);
        MsgSuccess('Labels updated');
        labelsOpen.value = false;
        await search();
    } catch (e: any) {
        MsgError(e?.response?.data?.message || e?.message || 'Save failed');
    } finally {
        labelsSaving.value = false;
    }
};

// --- bulk label across selected nodes ---
const bulkOpen = ref(false);
const bulkSaving = ref(false);
const bulkForm = reactive({ key: '', value: '', op: 'add' as 'add' | 'remove' });
const openBulk = () => {
    if (selected.value.length === 0) {
        MsgError('Select at least one node');
        return;
    }
    bulkForm.key = '';
    bulkForm.value = '';
    bulkForm.op = 'add';
    bulkOpen.value = true;
};
const applyBulk = async () => {
    if (!bulkForm.key.trim()) {
        MsgError('Label key is required');
        return;
    }
    bulkSaving.value = true;
    try {
        await bulkNodeLabel(
            selected.value.map((n) => n.id).filter((id) => id > 0),
            bulkForm.key.trim(),
            bulkForm.value.trim(),
            bulkForm.op,
        );
        MsgSuccess('Labels applied');
        bulkOpen.value = false;
        await search();
    } catch (e: any) {
        MsgError(e?.response?.data?.message || e?.message || 'Bulk label failed');
    } finally {
        bulkSaving.value = false;
    }
};

const addOpen = ref(false);
const adding = ref(false);
const authMode = ref<'password' | 'key'>('password');
const formRef = ref<FormInstance>();
const blankForm = (): NodeMgmt.NodeCreate => ({
    name: '',
    addr: '',
    port: 9999,
    sshUser: 'root',
    sshPort: 22,
    sshPassword: '',
    sshPrivateKey: '',
    sshPassPhrase: '',
    description: '',
});
const form = reactive<NodeMgmt.NodeCreate>(blankForm());
const rules = {
    name: [{ required: true, message: 'Name is required', trigger: 'blur' }],
    addr: [{ required: true, message: 'Address is required', trigger: 'blur' }],
    sshUser: [{ required: true, message: 'SSH user is required', trigger: 'blur' }],
};

const openAdd = () => {
    Object.assign(form, blankForm());
    authMode.value = 'password';
    addOpen.value = true;
};

const submit = async () => {
    if (!formRef.value) return;
    await formRef.value.validate(async (ok) => {
        if (!ok) return;
        if (authMode.value === 'password') {
            form.sshPrivateKey = '';
            form.sshPassPhrase = '';
        } else {
            form.sshPassword = '';
        }
        adding.value = true;
        try {
            await addNode(form);
            MsgSuccess('Node enrolled');
            addOpen.value = false;
            await search();
        } catch (e: any) {
            MsgError(e?.response?.data?.message || e?.message || 'Enrollment failed');
        } finally {
            adding.value = false;
        }
    });
};

const recheck = async (row: NodeMgmt.NodeInfo & { _checking?: boolean }) => {
    row._checking = true;
    try {
        await recheckNode(row.id);
        await search();
    } catch (e: any) {
        MsgError(e?.message || 'Health check failed');
    } finally {
        row._checking = false;
    }
};

const remove = async (row: NodeMgmt.NodeInfo) => {
    try {
        await ElMessageBox.confirm(
            `Delete node "${row.name}"? The master stops proxying to it; the agent on the host is not uninstalled.`,
            'Delete',
            { type: 'warning' },
        );
    } catch {
        return;
    }
    try {
        await deleteNode(row.id);
        MsgSuccess('Deleted');
        await search();
    } catch (e: any) {
        MsgError(e?.message || 'Delete failed');
    }
};

onMounted(search);
</script>

<style scoped lang="scss">
.label-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
    .eq {
        color: var(--el-text-color-secondary);
    }
}
.mr-1 {
    margin-right: 4px;
}
.mb-1 {
    margin-bottom: 4px;
}
</style>
