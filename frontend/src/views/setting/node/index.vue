<template>
    <div v-loading="loading">
        <LayoutContent title="Nodes">
            <template #leftToolBar>
                <el-button type="primary" @click="onOpenAdd">
                    {{ $t('commons.button.add') }}
                </el-button>
            </template>
            <template #rightToolBar>
                <TableSearch @search="search()" v-model:searchName="paginationConfig.info" />
                <TableRefresh @search="search()" />
            </template>
            <template #main>
                <ComplexTable
                    :pagination-config="paginationConfig"
                    @sort-change="search"
                    @search="search"
                    :data="data"
                >
                    <el-table-column :label="$t('commons.table.name')" prop="name" show-overflow-tooltip>
                        <template #default="{ row }">
                            <el-text type="primary">{{ row.name }}</el-text>
                        </template>
                    </el-table-column>
                    <el-table-column label="Address" prop="addr" show-overflow-tooltip>
                        <template #default="{ row }">{{ row.addr }}:{{ row.port }}</template>
                    </el-table-column>
                    <el-table-column :label="$t('commons.table.status')" prop="status">
                        <template #default="{ row }">
                            <el-tag :type="statusType(row.status)">
                                {{ row.status }}
                            </el-tag>
                        </template>
                    </el-table-column>
                    <el-table-column label="Version" prop="version" show-overflow-tooltip />
                    <el-table-column label="Last Check" prop="lastCheck" show-overflow-tooltip>
                        <template #default="{ row }">
                            {{ row.lastCheck ? formatDate(row.lastCheck) : '-' }}
                        </template>
                    </el-table-column>
                    <el-table-column label="Message" prop="lastMessage" show-overflow-tooltip />
                    <fu-table-operations
                        width="240px"
                        :buttons="buttons"
                        :label="$t('commons.table.operate')"
                        fix
                    />
                </ComplexTable>
            </template>
        </LayoutContent>

        <Operate ref="dialogRef" @search="search" />
        <OpDialog ref="opRef" @search="search" />
    </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import LayoutContent from '@/layout/layout-content.vue';
import ComplexTable from '@/components/complex-table/index.vue';
import TableSearch from '@/components/table-setting/table-search.vue';
import TableRefresh from '@/components/table-setting/table-refresh.vue';
import OpDialog from '@/components/del-dialog/index.vue';
import Operate from './operate.vue';
import { searchNodes, deleteNode, recheckNode } from '@/api/modules/node';
import { dateFormatSimple } from '@/utils/util';
import i18n from '@/lang';
import { MsgSuccess } from '@/utils/message';
import type { NodeMgmt } from '@/api/interface/node';

const loading = ref(false);
const data = ref<NodeMgmt.NodeInfo[]>([]);
const paginationConfig = reactive({
    cacheSizeKey: 'node-page-size',
    currentPage: 1,
    pageSize: 10,
    total: 0,
    info: '',
});

const dialogRef = ref();
const opRef = ref();

const buttons = [
    {
        label: i18n.global.t('commons.button.refresh'),
        click: (row: NodeMgmt.NodeInfo) => onRecheck(row),
    },
    {
        label: i18n.global.t('commons.button.delete'),
        click: (row: NodeMgmt.NodeInfo) => onDelete(row),
    },
];

function statusType(s: string) {
    if (s === 'Healthy') return 'success';
    if (s === 'VersionMismatch') return 'warning';
    if (s === 'Pending') return 'info';
    return 'danger';
}

function formatDate(v: string) {
    return dateFormatSimple(v);
}

async function search() {
    loading.value = true;
    try {
        const res = await searchNodes({
            page: paginationConfig.currentPage,
            pageSize: paginationConfig.pageSize,
            info: paginationConfig.info,
        });
        data.value = res.data.items || [];
        paginationConfig.total = res.data.total || 0;
    } finally {
        loading.value = false;
    }
}

function onOpenAdd() {
    dialogRef.value?.acceptParams();
}

async function onRecheck(row: NodeMgmt.NodeInfo) {
    loading.value = true;
    try {
        await recheckNode(row.id);
        MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
        await search();
    } finally {
        loading.value = false;
    }
}

function onDelete(row: NodeMgmt.NodeInfo) {
    opRef.value?.acceptParams({
        title: i18n.global.t('commons.msg.delete'),
        names: [row.name],
        msg: i18n.global.t('commons.msg.operatorHelper', [
            i18n.global.t('commons.button.delete'),
            row.name,
        ]),
        api: deleteNode,
        params: row.id,
    });
}

onMounted(() => {
    search();
});
</script>
