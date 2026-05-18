<template>
    <div>
        <el-popover
            placement="right-end"
            :show-arrow="false"
            :offset="0"
            :width="200"
            trigger="click"
            @before-enter="showPopover"
            popper-class="custom-popover-dropdown"
        >
            <template #reference>
                <div class="el-dropdown-link" v-if="!menuStore.isCollapse">
                    <el-badge is-dot :value="taskCount" :show-zero="false" :offset="[5, 5]">
                        <el-button link>
                            <SvgIcon class="icon" iconName="p-pcm" />
                            <span class="ellipsis-text">{{ loadCurrentName() }}</span>
                        </el-button>
                    </el-badge>
                </div>
                <div v-else class="el-dropdown-link">
                    <el-badge is-dot :value="taskCount" :show-zero="false" :offset="[-5, 5]">
                        <SvgIcon class="icon" iconName="p-pcm" />
                    </el-badge>
                </div>
            </template>
            <div class="dropdown-menu" v-loading="loading">
                <div class="dropdown-item" @click="openTask">
                    <div class="node">
                        <SvgIcon class="icon" iconName="p-renwuzhongxin1" />
                        {{ $t('menu.msgCenter') }}
                    </div>
                    <el-tag class="msg-tag" v-if="taskCount !== 0" size="small" round>{{ taskCount }}</el-tag>
                </div>
                <el-divider v-if="showNodes()" class="divider" />
                <div class="dropdown-item" @click="openNodeDashboard" v-if="isMasterPro">
                    <div class="node">
                        <SvgIcon class="icon" iconName="p-gailan1" />
                        {{ $t('xpack.node.multiOverview') }}
                    </div>
                </div>
                <el-divider v-if="isMasterPro" class="divider" />

                <div v-if="showNodes()">
                    <el-input
                        suffix-icon="Search"
                        v-model="query"
                        @input="onQueryInput"
                        class="w-full filter-input"
                        size="small"
                        clearable
                        placeholder="Search nodes"
                    />
                    <el-scrollbar :max-height="isMasterPro ? '230px' : '195px'" :noresize="true">
                        <template v-if="!query">
                            <div v-if="favorites.length" class="section-label">Favorites</div>
                            <div
                                class="dropdown-item"
                                v-for="item in favorites"
                                :key="'fav-' + item.name"
                                @click="changeNode(item)"
                            >
                                <div class="node">
                                    <SvgIcon class="icon" iconName="p-zhuji" />
                                    <span class="node-name">
                                        {{ item.name === 'local' ? globalStore.getMasterAlias() : item.name }}
                                    </span>
                                    <el-icon class="fav-star" @click.stop="toggleFavorite(item)">
                                        <StarFilled />
                                    </el-icon>
                                </div>
                            </div>
                            <div v-if="recents.length" class="section-label">Recent</div>
                            <div
                                class="dropdown-item"
                                v-for="item in recents"
                                :key="'rec-' + item.name"
                                @click="changeNode(item)"
                            >
                                <div class="node">
                                    <SvgIcon class="icon" iconName="p-zhuji" />
                                    <span class="node-name">
                                        {{ item.name === 'local' ? globalStore.getMasterAlias() : item.name }}
                                    </span>
                                </div>
                            </div>
                            <div class="section-label">All nodes</div>
                        </template>
                        <div
                            class="dropdown-item"
                            @click="changeNode(item)"
                            v-for="item in items"
                            :key="item.name"
                        >
                            <div class="node">
                                <SvgIcon class="icon" iconName="p-zhuji" />
                                <span class="node-name">
                                    {{ item.name === 'local' ? globalStore.getMasterAlias() : item.name }}
                                </span>
                                <el-icon class="fav-star" @click.stop="toggleFavorite(item)">
                                    <StarFilled v-if="isFavorite(item)" />
                                    <Star v-else />
                                </el-icon>
                                <el-tooltip
                                    v-if="item.status !== 'Healthy' || !item.isBound"
                                    :content="
                                        item.isBound ? $t('xpack.node.nodeUnhealthy') : $t('xpack.node.nodeUnbind')
                                    "
                                    placement="right"
                                >
                                    <el-icon class="icon-status" type="danger">
                                        <Warning />
                                    </el-icon>
                                </el-tooltip>
                            </div>
                        </div>
                        <div v-if="!items.length && !loading" class="section-label">No matching nodes</div>
                        <div v-if="hasMore" class="dropdown-item load-more" @click.stop="loadMore">
                            {{ loadingMore ? '…' : 'Load more' }}
                        </div>
                    </el-scrollbar>
                </div>
                <el-divider class="divider" />
                <div class="dropdown-item" @click="logout">
                    <SvgIcon class="icon" iconName="p-tuichudenglu3" />
                    {{ $t('commons.login.logout') }}
                </div>
            </div>
        </el-popover>
    </div>
</template>

<script setup lang="ts">
import { GlobalStore, MenuStore } from '@/store';
import { countExecutingTask } from '@/api/modules/log';
import { MsgError, MsgSuccess } from '@/utils/message';
import i18n from '@/lang';
import { getAgentSettingInfo } from '@/api/modules/setting';
import { searchNodeOptions } from '@/api/modules/node';
import { ref, watch } from 'vue';
import bus from '@/global/bus';
import { logOutApi } from '@/api/modules/auth';
import router from '@/routers';
import { loadProductProFromDB } from '@/utils/xpack';
import { routerToNameWithQuery } from '@/utils/router';
import { setDefaultNodeInfo } from '@/utils/node';

const query = ref('');
const globalStore = GlobalStore();
const menuStore = MenuStore();
const items = ref<any[]>([]);
const recents = ref<any[]>([]);
const favorites = ref<any[]>([]);
const loading = ref(false);
const loadingMore = ref(false);
const page = ref(1);
const pageSize = 20;
const total = ref(0);
const hasEnrolled = ref(false);
const props = defineProps({
    version: String,
});
const isMasterPro = computed(() => {
    return globalStore.isMasterPro();
});
const hasMore = computed(() => items.value.length < total.value);
watch(
    () => globalStore.isMasterPro(),
    () => {
        searchFirst();
    },
);

const emit = defineEmits(['openTask']);
bus.on('refreshTask', () => {
    checkTask();
});

const loadCurrentName = () => {
    if (globalStore.currentNode) {
        if (globalStore.currentNode === 'local') {
            return globalStore.getMasterAlias();
        }
        return globalStore.currentNode;
    }
    return globalStore.getMasterAlias();
};

const REC_KEY = 'node-recents';
const FAV_KEY = 'node-favorites';
const readLS = (k: string) => {
    try {
        return JSON.parse(localStorage.getItem(k) || '[]');
    } catch {
        return [];
    }
};
const writeLS = (k: string, v: any) => localStorage.setItem(k, JSON.stringify(v));
const snap = (i: any) => ({
    name: i.name,
    addr: i.addr,
    status: i.status,
    version: i.version,
    isBound: i.isBound,
});

const loadPersisted = () => {
    recents.value = readLS(REC_KEY);
    favorites.value = readLS(FAV_KEY);
};
const isFavorite = (item: any) => favorites.value.some((f) => f.name === item.name);
const toggleFavorite = (item: any) => {
    favorites.value = isFavorite(item)
        ? favorites.value.filter((f) => f.name !== item.name)
        : [snap(item), ...favorites.value].slice(0, 20);
    writeLS(FAV_KEY, favorites.value);
};
const recordRecent = (item: any) => {
    if (item.name === 'local') return;
    recents.value = [snap(item), ...recents.value.filter((r) => r.name !== item.name)].slice(0, 5);
    writeLS(REC_KEY, recents.value);
};

// Server-side typeahead: never loads the whole fleet. Scales to
// thousands of nodes — the SQL pages + filters, the client only ever
// holds one page (+ recents/favorites snapshots).
const fetchPage = async (reset: boolean) => {
    if (reset) {
        page.value = 1;
        loading.value = true;
    } else {
        loadingMore.value = true;
    }
    try {
        const res = await searchNodeOptions({ page: page.value, pageSize, info: query.value || '' });
        const data = res?.data?.items || [];
        total.value = res?.data?.total || 0;
        items.value = reset ? data : items.value.concat(data);
        if (!query.value && reset) {
            hasEnrolled.value = total.value > 1;
            if (total.value <= 1) setDefaultNodeInfo();
        }
    } catch {
        if (reset) items.value = [];
    } finally {
        loading.value = false;
        loadingMore.value = false;
    }
};
const searchFirst = () => fetchPage(true);
const loadMore = () => {
    if (items.value.length >= total.value) return;
    page.value += 1;
    fetchPage(false);
};

let debounceTimer: any;
const onQueryInput = () => {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => searchFirst(), 300);
};

const showPopover = () => {
    query.value = '';
    loadPersisted();
    searchFirst();
};

const changeNode = (item: any) => {
    if (!item || globalStore.currentNode === item.name) {
        return;
    }
    if (item.name === 'local') {
        globalStore.currentNode = 'local';
        globalStore.currentNodeAddr = item.addr;
        loadGlobalSetting();
        localStorage.removeItem('dashboardCache');
        localStorage.removeItem('upgradeChecked');
        loadProductProFromDB();
        routerToNameWithQuery('home', { t: Date.now() });
        return;
    }
    if (item.isBound === false) {
        MsgError(i18n.global.t('xpack.node.nodeUnbindHelper'));
        return;
    }
    if (item.status && item.status !== 'Healthy') {
        MsgError(i18n.global.t('xpack.node.nodeUnhealthyHelper'));
        return;
    }
    if (item.version && props.version != item.version) {
        MsgError(i18n.global.t('setting.versionNotSame'));
        return;
    }
    loadGlobalSetting();
    localStorage.removeItem('dashboardCache');
    localStorage.removeItem('upgradeChecked');
    globalStore.currentNode = item.name || 'local';
    globalStore.currentNodeAddr = item.addr;
    loadProductProFromDB();
    recordRecent(item);
    routerToNameWithQuery('home', { t: Date.now() });
};

const loadGlobalSetting = async () => {
    await getAgentSettingInfo().then((res) => {
        globalStore.defaultNetwork = res.data.defaultNetwork;
    });
};

const showNodes = () => hasEnrolled.value;

const taskCount = ref(0);
const checkTask = async () => {
    try {
        const res = await countExecutingTask();
        taskCount.value = res.data;
    } catch (error) {}
};

const openTask = () => {
    emit('openTask');
};

const openNodeDashboard = () => {
    routerToNameWithQuery('NodeDashboard', { uncached: 'true' });
};

const logout = () => {
    ElMessageBox.confirm(i18n.global.t('commons.msg.sureLogOut'), i18n.global.t('commons.msg.infoTitle'), {
        confirmButtonText: i18n.global.t('commons.button.confirm'),
        cancelButtonText: i18n.global.t('commons.button.cancel'),
        type: 'warning',
    })
        .then(async () => {
            await logOutApi();
            router.push({ name: 'entrance', params: { code: globalStore.entrance } });
            globalStore.isLogin = false;
            MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
        })
        .catch(() => {});
};

onMounted(() => {
    loadPersisted();
    searchFirst();
    checkTask();
});
</script>

<style scoped lang="scss">
@use '../index';

.el-dropdown-link {
    display: flex;
    align-items: center;
    box-sizing: border-box;
    border-top: 1px solid var(--panel-footer-border);
    height: 48px;
    .icon {
        margin-left: 25px;
        font-size: 8px;
        margin-right: 7px;
        color: var(--panel-main-bg-color-1);
    }
    &:hover {
        .icon {
            color: var(--el-color-primary);
        }
        .el-button {
            color: var(--el-color-primary);
        }
    }
}
.custom-popover-dropdown {
    padding: 0 !important;
    border: 1px solid #e4e7ed !important;
    box-shadow: 0 2px 8px 0 rgba(0, 0, 0, 0.1) !important;
    background-color: var(--el-menu-item-bg-color);
    .divider {
        display: block;
        height: 1px;
        width: 91%;
        margin: 3px 8px;
        border-top: 1px var(--el-border-color) var(--el-border-style);
    }
}

.dropdown-menu {
    min-width: 120px;
}

.dropdown-item {
    display: flex;
    align-items: center;
    padding: 2px 8px;
    cursor: pointer;
    min-height: 32px;
    transition: background 0.3s;
    .icon {
        font-size: 8px;
    }
    .icon-status {
        font-size: 16px;
        margin-left: auto;
    }
    .node {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 3px 0;
        width: 100%;
    }
    .node-name {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .msg-tag {
        margin-left: auto;
        background-color: transparent;
        color: var(--panel-main-bg-color-1);
    }
    &:hover {
        color: var(--el-color-primary);
        .icon {
            color: var(--el-color-primary);
        }
        .msg-tag {
            color: var(--el-color-primary);
        }
    }
}
.filter-input {
    padding: 0 8px;
    margin-bottom: 4px;
}
.dropdown-item:hover {
    background: var(--el-menu-item-bg-color-active);
}
.ellipsis-text {
    display: inline-block;
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
</style>
