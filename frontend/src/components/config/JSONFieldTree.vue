<script setup lang="ts">
import { ref } from "vue";
import type { JSONFieldNode } from "./jsonContractFields";
import { pointerToKey } from "./jsonKeyPath";

defineProps<{
  nodes: JSONFieldNode[];
  title: string;
  bindings: Record<string, string>;
  depth?: number;
}>();
defineSlots<{
  value(props: { field: JSONFieldNode }): unknown;
  binding(props: { field: JSONFieldNode }): unknown;
}>();
const collapsed = ref(new Set<string>());
function toggle(pointer: string) {
  if (collapsed.value.has(pointer)) collapsed.value.delete(pointer);
  else collapsed.value.add(pointer);
}
</script>

<template>
  <ul class="json-field-tree" :aria-label="`${title}字段层级`">
    <li
      v-for="field in nodes"
      :key="field.pointer"
      :data-field-pointer="field.pointer"
      class="field-node"
    >
      <div
        class="field-row"
        :class="{
          'is-container': field.type === 'object' || field.type === 'array',
          'is-bound': Object.values(bindings).includes(field.pointer),
        }"
      >
        <div class="field-key">
          <button
            v-if="field.children.length"
            type="button"
            class="branch-toggle"
            :aria-label="`${title} ${pointerToKey(field.pointer)} ${collapsed.has(field.pointer) ? '展开' : '收起'}`"
            :aria-expanded="!collapsed.has(field.pointer)"
            @click="toggle(field.pointer)"
          >
            <span class="chevron" aria-hidden="true">{{
              collapsed.has(field.pointer) ? "▸" : "▾"
            }}</span>
            <code>{{ field.key }}</code>
          </button>
          <code v-else class="leaf-key">{{ field.key }}</code>
          <span class="type-badge">{{ field.type }}</span>
        </div>
        <div class="field-value"><slot name="value" :field="field" /></div>
        <div class="field-binding"><slot name="binding" :field="field" /></div>
      </div>
      <div
        v-if="field.children.length"
        v-show="!collapsed.has(field.pointer)"
        class="field-children"
        :class="{ 'deep-children': (depth || 0) >= 5 }"
      >
        <JSONFieldTree
          :nodes="field.children"
          :title="title"
          :bindings="bindings"
          :depth="(depth || 0) + 1"
        >
          <template #value="slotProps"
            ><slot name="value" v-bind="slotProps"
          /></template>
          <template #binding="slotProps"
            ><slot name="binding" v-bind="slotProps"
          /></template>
        </JSONFieldTree>
      </div>
    </li>
  </ul>
</template>

<style scoped>
.json-field-tree {
  list-style: none;
  padding: 0;
  margin: 0;
  min-width: 0;
}
.field-node {
  min-width: 0;
}
.field-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1.4fr);
  gap: 12px;
  padding: 10px;
  align-items: start;
  border-top: 1px solid var(--border, #e2e8f0);
}
.field-key {
  display: grid;
  gap: 5px;
  min-width: 0;
}
.field-key code {
  font-size: 12px;
  overflow-wrap: anywhere;
  line-height: 1.5;
  text-align: left;
}
.branch-toggle {
  display: flex;
  gap: 5px;
  align-items: baseline;
  justify-content: flex-start;
  padding: 0;
  min-height: 24px;
  border: 0;
  border-radius: 3px;
  background: transparent;
  color: inherit;
  cursor: pointer;
}
.branch-toggle:focus-visible {
  outline: 2px solid #00857a;
  outline-offset: 2px;
}
.chevron {
  width: 12px;
  flex-shrink: 0;
  color: #64748b;
}
.leaf-key {
  padding-left: 17px;
}
.type-badge {
  padding-left: 17px;
  font-size: 10px;
  color: var(--muted);
}
.field-value,
.field-binding {
  min-width: 0;
}
.field-binding {
  display: grid;
  gap: 5px;
}
.is-bound {
  background: #f8fbff;
}
.is-container {
  background: #f3f6fa;
}
.field-children {
  margin-left: 16px;
  border-left: 2px solid #d6e0ec;
}
.deep-children {
  margin-left: 0;
}
@container (max-width: 500px) {
  .field-row {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.5fr);
    gap: 8px;
  }
  .field-key {
    grid-column: 1 / -1;
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .type-badge {
    padding-left: 0;
  }
  .field-children {
    margin-left: 10px;
  }
  .deep-children {
    margin-left: 0;
  }
}
</style>
