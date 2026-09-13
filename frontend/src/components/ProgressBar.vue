<script setup lang="ts">
defineProps<{ value: number }>();
</script>
<template>
  <div
    class="progress"
    role="progressbar"
    aria-label="任务进度"
    :aria-valuenow="value"
    aria-valuemin="0"
    aria-valuemax="100"
  >
    <i :style="{ width: `${value}%` }" />
  </div>
</template>
