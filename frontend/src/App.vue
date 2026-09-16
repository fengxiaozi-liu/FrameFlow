<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import { useTaskStore } from "./stores/tasks";
import AppHeader from "./components/AppHeader.vue";
const tasks = useTaskStore();
onMounted(tasks.connect);
onUnmounted(tasks.disconnect);
</script>
<template><AppHeader /><RouterView /></template>
