<script setup lang="ts">
import type { VoiceProfile } from '../../api/bindings';
defineProps<{modelValue?:string;voices:VoiceProfile[];inherit?:boolean;disabled?:boolean}>();
const emit=defineEmits<{'update:modelValue':[string]}>();
</script>
<template><select aria-label="选择音色" :value="modelValue||''" :disabled="disabled" @change="emit('update:modelValue',($event.target as HTMLSelectElement).value)"><option value="">{{inherit?'继承人物音色':'请选择音色'}}</option><option v-for="voice in voices" :key="voice.id+':'+voice.revision" :value="voice.id+':'+voice.revision">{{voice.name}} · v{{voice.revision}}</option></select></template>
