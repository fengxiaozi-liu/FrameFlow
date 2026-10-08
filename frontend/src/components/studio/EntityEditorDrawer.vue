<script setup lang="ts">
import {onMounted,onBeforeUnmount,ref} from 'vue';
const props=defineProps<{title:string;busy?:boolean}>();
const emit=defineEmits<{close:[]}>();
const dialog=ref<HTMLDialogElement>();
let previous:HTMLElement|null=null,overflow='';
function close(){if(!props.busy)emit('close')}
onMounted(()=>{previous=document.activeElement as HTMLElement;overflow=document.body.style.overflow;document.body.style.overflow='hidden';dialog.value?.showModal();dialog.value?.querySelector<HTMLInputElement>('input:not([type=file])')?.focus()});
onBeforeUnmount(()=>{dialog.value?.close();document.body.style.overflow=overflow;if(previous?.isConnected)previous.focus()});
</script>
<template><Teleport to="body"><dialog ref="dialog" class="binding-workbench shot-configuration entity-editor-drawer" aria-labelledby="entity-editor-title" @cancel.prevent="close"><header class="entity-drawer-heading"><h2 id="entity-editor-title">{{title}}</h2><button type="button" aria-label="关闭素材侧栏" :disabled="busy" @click="close">×</button></header><slot /></dialog></Teleport></template>
