<template>
    <el-select
        :model-value="modelValue"
        @update:model-value="handleChange"
        class="p-w-200"
        filterable
        remote
        :remote-method="remoteSearch"
        :loading="loading"
        reserve-keyword
        :placeholder="$t('setting.selectNode')"
        @visible-change="onVisible"
    >
        <template #prefix>{{ $t('xpack.node.node') }}</template>
        <el-option
            v-for="item in nodes"
            :key="item.id + '-' + item.name"
            :label="item.name === 'local' ? globalStore.getMasterAlias() : item.name"
            :value="item.name"
        ></el-option>
    </el-select>
</template>

<script setup>
import { searchNodeOptions } from '@/api/modules/node';
import { useGlobalStore } from '@/composables/useGlobalStore';
const { globalStore } = useGlobalStore();

defineProps({
    modelValue: {
        type: String,
        default: '',
    },
});

const nodes = ref([]);
const loading = ref(false);
const emit = defineEmits(['update:modelValue', 'change']);

const handleChange = (value) => {
    emit('update:modelValue', value);
    emit('change', value);
};

// Server-side typeahead — never loads the whole fleet (scales to
// thousands of nodes). Mirrors the sidebar picker (FLEET P1).
const fetchNodes = async (info) => {
    loading.value = true;
    try {
        const res = await searchNodeOptions({ page: 1, pageSize: 20, info: info || '' });
        nodes.value = res?.data?.items || [];
    } catch (error) {
        nodes.value = [];
    } finally {
        loading.value = false;
    }
};

const remoteSearch = (q) => fetchNodes(q);
const onVisible = (open) => {
    if (open && nodes.value.length === 0) fetchNodes('');
};

onMounted(() => {
    fetchNodes('');
});
</script>
