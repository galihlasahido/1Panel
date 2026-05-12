<template>
    <el-drawer v-model="open" :title="$t('commons.button.add') + ' Node'" size="40%" :close-on-click-modal="false">
        <el-form ref="formRef" :model="form" label-position="top" :rules="rules">
            <el-form-item label="Name" prop="name">
                <el-input v-model="form.name" placeholder="prod-web-1" />
                <span class="input-help">RFC 1035 hostname label (a-z, 0-9, hyphen). Used as TLS SAN.</span>
            </el-form-item>
            <el-form-item label="Address" prop="addr">
                <el-input v-model="form.addr" placeholder="10.0.0.5 or node.example.com" />
            </el-form-item>
            <el-form-item label="Agent Port" prop="port">
                <el-input-number v-model="form.port" :min="1" :max="65535" />
            </el-form-item>
            <el-divider content-position="left">SSH credentials (master → slave)</el-divider>
            <el-form-item label="SSH User" prop="sshUser">
                <el-input v-model="form.sshUser" placeholder="root" />
            </el-form-item>
            <el-form-item label="SSH Port" prop="sshPort">
                <el-input-number v-model="form.sshPort" :min="1" :max="65535" />
            </el-form-item>
            <el-form-item label="Auth Mode">
                <el-radio-group v-model="authMode">
                    <el-radio value="password">Password</el-radio>
                    <el-radio value="key">Private Key</el-radio>
                </el-radio-group>
            </el-form-item>
            <el-form-item v-if="authMode === 'password'" label="Password" prop="sshPassword">
                <el-input v-model="form.sshPassword" type="password" show-password />
            </el-form-item>
            <el-form-item v-else label="Private Key" prop="sshPrivateKey">
                <el-input v-model="form.sshPrivateKey" type="textarea" :rows="6" />
            </el-form-item>
            <el-form-item v-if="authMode === 'key'" label="Passphrase (optional)">
                <el-input v-model="form.sshPassPhrase" type="password" show-password />
            </el-form-item>
            <el-form-item label="Description">
                <el-input v-model="form.description" type="textarea" :rows="2" />
            </el-form-item>
        </el-form>

        <el-alert type="info" :closable="false" class="mt-4">
            <p>Enrollment will SSH into the slave host, drop a TLS cert bundle into
            <code>/etc/1panel/bootstrap/</code>, then restart the agent service.
            Make sure <code>1panel-agent</code> is already installed on the slave —
            the installer-side auto-install integration (see <code>INSTALLER_TODO.md</code>)
            is not yet shipped.</p>
        </el-alert>

        <template #footer>
            <el-button @click="open = false">{{ $t('commons.button.cancel') }}</el-button>
            <el-button type="primary" :loading="loading" @click="submit">
                {{ $t('commons.button.confirm') }}
            </el-button>
        </template>
    </el-drawer>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue';
import { addNode } from '@/api/modules/node';
import { Rules } from '@/global/form-rules';
import i18n from '@/lang';
import { MsgSuccess } from '@/utils/message';
import type { FormInstance } from 'element-plus';
import type { NodeMgmt } from '@/api/interface/node';

const emit = defineEmits<{ (e: 'search'): void }>();

const open = ref(false);
const loading = ref(false);
const authMode = ref<'password' | 'key'>('password');
const formRef = ref<FormInstance>();

const form = reactive<NodeMgmt.NodeCreate>({
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

const rules = reactive({
    name: [Rules.requiredInput, { pattern: /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$/, message: 'Invalid hostname label', trigger: 'blur' }],
    addr: [Rules.requiredInput],
    port: [Rules.requiredInput],
    sshUser: [Rules.requiredInput],
    sshPort: [Rules.requiredInput],
});

function acceptParams() {
    form.name = '';
    form.addr = '';
    form.port = 9999;
    form.sshUser = 'root';
    form.sshPort = 22;
    form.sshPassword = '';
    form.sshPrivateKey = '';
    form.sshPassPhrase = '';
    form.description = '';
    authMode.value = 'password';
    open.value = true;
}

async function submit() {
    const valid = await formRef.value?.validate().catch(() => false);
    if (!valid) return;
    loading.value = true;
    try {
        const payload = { ...form };
        if (authMode.value === 'password') {
            payload.sshPrivateKey = '';
            payload.sshPassPhrase = '';
        } else {
            payload.sshPassword = '';
        }
        await addNode(payload);
        MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
        open.value = false;
        emit('search');
    } finally {
        loading.value = false;
    }
}

defineExpose({ acceptParams });
</script>
