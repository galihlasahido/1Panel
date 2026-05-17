<template>
    <div>
        <LayoutContent title="Nodes">
            <template #main>
                <el-empty
                    description="Multi-node management is a Pro feature and is not available in this open-source build."
                />
            </template>
        </LayoutContent>
    </div>
</template>

<script setup lang="ts">
// OSS stub. The upstream node manager depends on xpack-only components
// (table-search / table-refresh) that are not part of the open-source
// tree, so it cannot compile here. The route is kept; the page is an
// honest placeholder. LayoutContent is globally registered
// (src/components/index.ts).
</script>
