<template>
    <div>
        <RouterButton :buttons="buttons" />
        <LayoutContent>
            <router-view></router-view>
        </LayoutContent>
    </div>
</template>

<script lang="ts" setup>
import { reactive } from 'vue';
import i18n from '@/lang';
import { useGlobalStore } from '@/composables/useGlobalStore';
import { getCurrentUser } from '@/api/modules/user';
const { isOffLine, isFxplay } = useGlobalStore();

const buttons = reactive([
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
]);

function removeButton(path: string) {
    const i = buttons.findIndex((b) => b.path === path);
    if (i !== -1) buttons.splice(i, 1);
}

onMounted(async () => {
    // Path-based removal — index math broke once a tab was inserted.
    if (isOffLine.value) {
        removeButton('/settings/license');
    }
    if (isFxplay.value) {
        removeButton('/settings/about');
    }
    // The Users (RBAC) tab is superadmin-only. Backend enforces this via
    // middleware.SuperAdmin regardless; hiding the tab just keeps it out
    // of a sub-user's UI. Insert it right after the Nodes tab.
    try {
        const me = await getCurrentUser();
        if (me.data?.isSuper) {
            const at = buttons.findIndex((b) => b.path === '/settings/nodes');
            buttons.splice(at === -1 ? buttons.length : at + 1, 0, {
                label: i18n.global.t('setting.users'),
                path: '/settings/users',
            });
        }
    } catch {
        /* not resolvable → leave the tab hidden */
    }
});
</script>
