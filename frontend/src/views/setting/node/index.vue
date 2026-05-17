<template>
    <div v-loading="loading">
        <LayoutContent :title="$t('setting.nodes')">
            <template #leftToolBar>
                <el-button type="primary" @click="openAdd">
                    {{ $t('commons.button.add') }}
                </el-button>
            </template>
            <template #rightToolBar>
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
                    @sort-change="search"
                    @search="search"
                    :data="data"
                >
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
                    <el-table-column label="Message" min-width="200" prop="lastMessage" show-overflow-tooltip>
                        <template #default="{ row }">{{ row.lastMessage || '-' }}</template>
                    </el-table-column>
                    <el-table-column :label="$t('commons.table.operate')" width="200" fixed="right">
                        <template #default="{ row }">
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
    </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import type { FormInstance } from 'element-plus';
import { ElMessageBox } from 'element-plus';
import { searchNodes, addNode, deleteNode, recheckNode } from '@/api/modules/node';
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

const search = async () => {
    loading.value = true;
    try {
        const res = await searchNodes({
            page: paginationConfig.currentPage,
            pageSize: paginationConfig.pageSize,
            info: paginationConfig.info,
        });
        data.value = res.data.items || [];
        paginationConfig.total = res.data.total;
    } catch (e: any) {
        MsgError(e?.message || 'Failed to load nodes');
    } finally {
        loading.value = false;
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
