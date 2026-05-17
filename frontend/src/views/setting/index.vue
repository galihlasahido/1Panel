<template>
    <div>
        <RouterButton :buttons="buttons" />
        <LayoutContent>
            <router-view></router-view>
        </LayoutContent>
    </div>
</template>

<script lang="ts" setup>
import i18n from '@/lang';
import { useGlobalStore } from '@/composables/useGlobalStore';
const { isOffLine, isFxplay } = useGlobalStore();

const buttons = [
    {
        label: i18n.global.t('setting.panel'),
        path: '/settings/panel',
    },
    {
        label: i18n.global.t('setting.safe'),
        path: '/settings/safe',
    },
    {
        label: i18n.global.t('xpack.alert.alertNotice'),
        path: '/settings/alert',
    },
    {
        label: i18n.global.t('setting.backupAccount', 2),
        path: '/settings/backupaccount',
    },
    {
        label: i18n.global.t('setting.snapshot', 2),
        path: '/settings/snapshot',
    },
    {
        label: i18n.global.t('setting.nodes'),
        path: '/settings/nodes',
    },
    {
        label: i18n.global.t('setting.license'),
        path: '/settings/license',
    },
    {
        label: i18n.global.t('setting.about'),
        path: '/settings/about',
    },
];

function removeButton(path: string) {
    const i = buttons.findIndex((b) => b.path === path);
    if (i !== -1) buttons.splice(i, 1);
}

onMounted(() => {
    // Path-based removal — index math broke once a tab was inserted.
    if (isOffLine.value) {
        removeButton('/settings/license');
    }
    if (isFxplay.value) {
        removeButton('/settings/about');
    }
});
</script>
