<script setup lang="ts">
import { nextTick, ref } from 'vue';
import type { Material } from '../../api/types';
const props=defineProps<{ids:string[];materials:Material[];disabled?:boolean;replaceable?:boolean}>();
const emit=defineEmits<{'update:ids':[string[]];upload:[File[]];replace:[string,File]}>();
const preview=ref<Material>();const closeButton=ref<HTMLButtonElement>();let opener:HTMLElement|undefined;
function open(id:string,event:Event){opener=event.currentTarget as HTMLElement;preview.value=props.materials.find(m=>m.id===id);void nextTick(()=>closeButton.value?.focus())}
function close(){preview.value=undefined;void nextTick(()=>opener?.focus())}
function move(i:number,offset:number){const next=[...props.ids];const j=i+offset;if(j<0||j>=next.length)return;[next[i],next[j]]=[next[j]!,next[i]!];emit('update:ids',next)}
function replace(id:string,event:Event){const el=event.target as HTMLInputElement;const file=el.files?.[0];if(file)emit('replace',id,file);el.value=''}
function upload(event:Event){const el=event.target as HTMLInputElement;emit('upload',Array.from(el.files||[]));el.value=''}
</script>
<template><div class="reference-list"><div v-for="(id,i) in ids" :key="id" class="reference-tile"><button type="button" :disabled="disabled" :aria-label="`预览参考图 ${i+1}`" @click="open(id,$event)"><img v-if="materials.find(m=>m.id===id)" :src="materials.find(m=>m.id===id)?.url" :alt="materials.find(m=>m.id===id)?.name" /><span v-else>素材已失效</span></button><span>{{materials.find(m=>m.id===id)?.name||id}}</span><div><button type="button" :disabled="disabled||i===0" aria-label="参考图前移" @click="move(i,-1)">←</button><button type="button" :disabled="disabled||i===ids.length-1" aria-label="参考图后移" @click="move(i,1)">→</button><button type="button" :disabled="disabled" aria-label="移除参考图绑定" @click="emit('update:ids',ids.filter(x=>x!==id))">×</button></div><label v-if="replaceable" class="reference-replace">替换参考图<input type="file" accept="image/*" :disabled="disabled" @change="replace(id,$event)" /></label></div><label class="upload-tile">＋ 添加图片<input type="file" accept="image/*" multiple :disabled="disabled" @change="upload" /></label><div v-if="preview" class="image-lightbox" role="dialog" aria-modal="true" aria-label="参考图预览" @keydown.esc.prevent="close" @keydown.tab.prevent="closeButton?.focus()"><button ref="closeButton" type="button" @click="close">关闭预览</button><img :src="preview.url" :alt="preview.name" /></div></div></template>
