<script setup lang="ts">
defineProps<{busy:boolean;status:string;error?:string;referenceCount:number;referenceLimit?:number}>();
const emit=defineEmits<{preview:[];image:[];video:[]}>();
</script>
<template><div class="generation-toolbar"><span role="status">{{status}}</span><span>已选 {{referenceCount}} 张参考图{{referenceLimit!==undefined?` / 模型上限 ${referenceLimit}`:''}}</span><p v-if="error" role="alert" class="error">{{error}}</p><button type="button" :disabled="busy" @click="emit('preview')">查看实际生成输入</button><button type="button" :disabled="busy" @click="emit('image')">生成分镜图</button><button type="button" class="primary" :disabled="busy" @click="emit('video')">生成视频 ↑</button></div></template>
