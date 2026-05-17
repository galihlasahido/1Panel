<template>
    <div v-loading="loading">
        <LayoutContent :title="$t('setting.users')">
            <template #leftToolBar>
                <el-button type="primary" @click="openCreate">
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
                            Sub-users authenticate against the panel like the admin, but their
                            access is limited to the menus and nodes selected here — enforced on
                            the backend, not just hidden in the UI. The built-in admin account is
                            the superadmin and is managed in Panel settings, not here.
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
                    <el-table-column :label="$t('commons.table.status')" min-width="100" prop="status">
                        <template #default="{ row }">
                            <el-tag :type="row.status === 'Enable' ? 'success' : 'info'">
                                {{ row.status }}
                            </el-tag>
                        </template>
                    </el-table-column>
                    <el-table-column label="Language" min-width="90" prop="language" />
                    <el-table-column label="Menus" min-width="220" prop="menus">
                        <template #default="{ row }">
                            <el-tag v-if="(row.menus || []).includes('*')" type="warning">all</el-tag>
                            <template v-else>
                                <el-tag v-for="m in row.menus" :key="m" class="mr-1 mb-1" size="small">
                                    {{ m }}
                                </el-tag>
                            </template>
                        </template>
                    </el-table-column>
                    <el-table-column label="Nodes" min-width="180" prop="nodes">
                        <template #default="{ row }">
                            <el-tag v-if="(row.nodes || []).includes('*')" type="warning">all</el-tag>
                            <template v-else>
                                <el-tag v-for="n in row.nodes" :key="n" class="mr-1 mb-1" size="small" type="success">
                                    {{ n }}
                                </el-tag>
                            </template>
                        </template>
                    </el-table-column>
                    <el-table-column label="Description" min-width="160" prop="description" show-overflow-tooltip>
                        <template #default="{ row }">{{ row.description || '-' }}</template>
                    </el-table-column>
                    <el-table-column :label="$t('commons.table.operate')" width="260" fixed="right">
                        <template #default="{ row }">
                            <el-button link type="primary" @click="openEdit(row)">
                                {{ $t('commons.button.edit') }}
                            </el-button>
                            <el-button link type="primary" @click="openResetPwd(row)">
                                Password
                            </el-button>
                            <el-button link type="danger" @click="remove(row)">
                                {{ $t('commons.button.delete') }}
                            </el-button>
                        </template>
                    </el-table-column>
                </ComplexTable>
            </template>
        </LayoutContent>

        <el-dialog
            v-model="dialogOpen"
            :title="isEdit ? 'Edit user' : 'Add user'"
            width="42%"
            :close-on-click-modal="false"
        >
            <el-form ref="formRef" :model="form" label-position="top" :rules="rules">
                <el-form-item label="Name" prop="name">
                    <el-input v-model="form.name" :disabled="isEdit" placeholder="login username" />
                </el-form-item>
                <el-form-item v-if="!isEdit" label="Password" prop="password">
                    <el-input v-model="form.password" type="password" show-password placeholder="login password" />
                </el-form-item>
                <el-form-item label="Status">
                    <el-switch
                        v-model="form.status"
                        active-value="Enable"
                        inactive-value="Disable"
                        active-text="Enable"
                        inactive-text="Disable"
                    />
                </el-form-item>
                <el-form-item label="Language">
                    <el-select v-model="form.language" style="width: 160px">
                        <el-option label="English" value="en" />
                        <el-option label="中文" value="zh" />
                    </el-select>
                </el-form-item>
                <el-form-item label="Allowed menus" prop="menus">
                    <el-select
                        v-model="form.menus"
                        multiple
                        collapse-tags
                        collapse-tags-tooltip
                        style="width: 100%"
                        placeholder="select the feature menus this user may use"
                    >
                        <el-option v-for="k in menuKeys" :key="k" :label="k" :value="k" />
                    </el-select>
                </el-form-item>
                <el-form-item label="Allowed nodes" prop="nodes">
                    <el-select
                        v-model="form.nodes"
                        multiple
                        collapse-tags
                        collapse-tags-tooltip
                        style="width: 100%"
                        placeholder="select the nodes this user may operate"
                    >
                        <el-option label="all nodes (*)" value="*" />
                        <el-option v-for="n in nodeOptions" :key="n" :label="n" :value="n" />
                    </el-select>
                </el-form-item>
                <el-form-item label="Description">
                    <el-input v-model="form.description" type="textarea" :rows="2" />
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button @click="dialogOpen = false">{{ $t('commons.button.cancel') }}</el-button>
                <el-button type="primary" :loading="saving" @click="submit">
                    {{ $t('commons.button.confirm') }}
                </el-button>
            </template>
        </el-dialog>

        <el-dialog v-model="pwdOpen" title="Reset password" width="32%" :close-on-click-modal="false">
            <el-form ref="pwdRef" :model="pwdForm" label-position="top" :rules="pwdRules">
                <el-form-item label="New password" prop="password">
                    <el-input v-model="pwdForm.password" type="password" show-password />
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button @click="pwdOpen = false">{{ $t('commons.button.cancel') }}</el-button>
                <el-button type="primary" :loading="saving" @click="submitPwd">
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
import { searchUsers, createUser, updateUser, updateUserPassword, deleteUser } from '@/api/modules/user';
import { UserMgmt, RBAC_MENU_KEYS } from '@/api/interface/user';
import { listNodeOptions } from '@/api/modules/setting';
import { MsgSuccess, MsgError } from '@/utils/message';

const loading = ref(false);
const saving = ref(false);
const data = ref<UserMgmt.UserInfo[]>([]);
const menuKeys = RBAC_MENU_KEYS;
const nodeOptions = ref<string[]>([]);

const paginationConfig = reactive({
    cacheSizeKey: 'user-page-size',
    currentPage: 1,
    pageSize: Number(localStorage.getItem('user-page-size')) || 10,
    total: 0,
    info: '',
});

const search = async () => {
    loading.value = true;
    try {
        const res = await searchUsers({
            page: paginationConfig.currentPage,
            pageSize: paginationConfig.pageSize,
            info: paginationConfig.info,
        });
        data.value = res.data.items || [];
        paginationConfig.total = res.data.total;
    } catch (e: any) {
        MsgError(e?.message || 'Failed to load users');
    } finally {
        loading.value = false;
    }
};

const loadNodeOptions = async () => {
    try {
        const res = await listNodeOptions('all');
        nodeOptions.value = (res.data || []).map((n: any) => n.name).filter((n: string) => n && n !== 'local');
    } catch {
        nodeOptions.value = [];
    }
};

const dialogOpen = ref(false);
const isEdit = ref(false);
const editId = ref(0);
const formRef = ref<FormInstance>();
const blankForm = (): UserMgmt.UserCreate => ({
    name: '',
    password: '',
    status: 'Enable',
    language: 'en',
    menus: [],
    nodes: [],
    description: '',
});
const form = reactive<UserMgmt.UserCreate>(blankForm());
const rules = {
    name: [{ required: true, message: 'Name is required', trigger: 'blur' }],
    password: [{ required: true, message: 'Password is required', trigger: 'blur' }],
    menus: [{ required: true, message: 'Select at least one menu', trigger: 'change' }],
    nodes: [{ required: true, message: 'Select at least one node', trigger: 'change' }],
};

const openCreate = () => {
    Object.assign(form, blankForm());
    isEdit.value = false;
    editId.value = 0;
    dialogOpen.value = true;
};

const openEdit = (row: UserMgmt.UserInfo) => {
    Object.assign(form, {
        name: row.name,
        password: '',
        status: row.status,
        language: row.language,
        menus: [...(row.menus || [])],
        nodes: [...(row.nodes || [])],
        description: row.description,
    });
    isEdit.value = true;
    editId.value = row.id;
    dialogOpen.value = true;
};

const submit = async () => {
    if (!formRef.value) return;
    await formRef.value.validate(async (ok) => {
        if (!ok) return;
        saving.value = true;
        try {
            if (isEdit.value) {
                await updateUser({
                    id: editId.value,
                    status: form.status,
                    language: form.language,
                    menus: form.menus,
                    nodes: form.nodes,
                    description: form.description,
                });
                MsgSuccess('User updated');
            } else {
                await createUser(form);
                MsgSuccess('User created');
            }
            dialogOpen.value = false;
            await search();
        } catch (e: any) {
            MsgError(e?.response?.data?.message || e?.message || 'Save failed');
        } finally {
            saving.value = false;
        }
    });
};

const pwdOpen = ref(false);
const pwdRef = ref<FormInstance>();
const pwdForm = reactive({ id: 0, password: '' });
const pwdRules = {
    password: [{ required: true, message: 'Password is required', trigger: 'blur' }],
};

const openResetPwd = (row: UserMgmt.UserInfo) => {
    pwdForm.id = row.id;
    pwdForm.password = '';
    pwdOpen.value = true;
};

const submitPwd = async () => {
    if (!pwdRef.value) return;
    await pwdRef.value.validate(async (ok) => {
        if (!ok) return;
        saving.value = true;
        try {
            await updateUserPassword(pwdForm.id, pwdForm.password);
            MsgSuccess('Password updated');
            pwdOpen.value = false;
        } catch (e: any) {
            MsgError(e?.response?.data?.message || e?.message || 'Update failed');
        } finally {
            saving.value = false;
        }
    });
};

const remove = async (row: UserMgmt.UserInfo) => {
    try {
        await ElMessageBox.confirm(`Delete user "${row.name}"? This cannot be undone.`, 'Delete', {
            type: 'warning',
        });
    } catch {
        return;
    }
    try {
        await deleteUser(row.id);
        MsgSuccess('Deleted');
        await search();
    } catch (e: any) {
        MsgError(e?.message || 'Delete failed');
    }
};

onMounted(() => {
    search();
    loadNodeOptions();
});
</script>
