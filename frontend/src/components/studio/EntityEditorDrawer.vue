<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, useId } from "vue";

const props = withDefaults(
  defineProps<{
    title: string;
    busy?: boolean;
    resizable?: boolean;
    initialWidth?: number;
    closeLabel?: string;
  }>(),
  { initialWidth: 720, closeLabel: "关闭素材侧栏" },
);
const emit = defineEmits<{ close: [] }>();
const dialog = ref<HTMLDialogElement>();
const titleID = useId();
const viewportWidth = ref(window.innerWidth);
const preferredWidth = ref(props.initialWidth);
const resizing = ref(false);
const narrow = computed(() => viewportWidth.value <= 640);
const maxWidth = computed(() =>
  narrow.value ? viewportWidth.value : Math.min(1100, viewportWidth.value - 32),
);
const minWidth = computed(() => Math.min(480, maxWidth.value));
const width = computed(() =>
  narrow.value
    ? viewportWidth.value
    : Math.max(minWidth.value, Math.min(maxWidth.value, preferredWidth.value)),
);
let previous: HTMLElement | null = null;
let overflow = "";
let drag: { id: number; x: number; width: number } | undefined;

function close() {
  if (!props.busy) emit("close");
}
function updateViewport() {
  viewportWidth.value = window.innerWidth;
  if (narrow.value) endResize();
}
function startResize(event: PointerEvent) {
  if (event.button !== 0 || narrow.value) return;
  event.preventDefault();
  const handle = event.currentTarget as HTMLElement;
  handle.focus();
  handle.setPointerCapture(event.pointerId);
  drag = { id: event.pointerId, x: event.clientX, width: width.value };
  resizing.value = true;
}
function resize(event: PointerEvent) {
  if (!drag || drag.id !== event.pointerId) return;
  preferredWidth.value = Math.max(
    minWidth.value,
    Math.min(maxWidth.value, drag.width + drag.x - event.clientX),
  );
}
function endResize() {
  drag = undefined;
  resizing.value = false;
}
function resizeWithKeyboard(event: KeyboardEvent) {
  const step = event.shiftKey ? 80 : 20;
  const target = {
    ArrowLeft: width.value + step,
    ArrowRight: width.value - step,
    Home: minWidth.value,
    End: maxWidth.value,
  }[event.key];
  if (target === undefined) return;
  event.preventDefault();
  preferredWidth.value = Math.max(
    minWidth.value,
    Math.min(maxWidth.value, target),
  );
}
onMounted(() => {
  previous = document.activeElement as HTMLElement;
  overflow = document.body.style.overflow;
  document.body.style.overflow = "hidden";
  window.addEventListener("resize", updateViewport);
  dialog.value?.showModal();
  dialog.value
    ?.querySelector<HTMLInputElement>("input:not([type=file])")
    ?.focus();
});
onBeforeUnmount(() => {
  endResize();
  window.removeEventListener("resize", updateViewport);
  dialog.value?.close();
  document.body.style.overflow = overflow;
  if (previous?.isConnected) previous.focus();
});
</script>

<template>
  <Teleport to="body">
    <dialog
      ref="dialog"
      class="binding-workbench shot-configuration entity-editor-drawer"
      :class="{
        'entity-drawer-resizable': resizable,
        'entity-drawer-resizing': resizing,
      }"
      :style="resizable ? { width: `${width}px` } : undefined"
      :aria-labelledby="titleID"
      @cancel.self.prevent="close"
    >
      <div
        v-if="resizable && !narrow"
        class="entity-drawer-resize"
        role="separator"
        tabindex="0"
        aria-label="调整协议与参数面板宽度"
        aria-orientation="vertical"
        :aria-valuemin="minWidth"
        :aria-valuemax="maxWidth"
        :aria-valuenow="width"
        :aria-valuetext="`${width} 像素`"
        title="拖动调整宽度，或使用左右方向键"
        @pointerdown="startResize"
        @pointermove="resize"
        @pointerup="endResize"
        @pointercancel="endResize"
        @lostpointercapture="endResize"
        @keydown="resizeWithKeyboard"
      />
      <header class="entity-drawer-heading">
        <h2 :id="titleID">{{ title }}</h2>
        <button
          type="button"
          :aria-label="closeLabel"
          :disabled="busy"
          @click="close"
        >
          ×
        </button>
      </header>
      <div v-if="resizable" class="entity-drawer-content"><slot /></div>
      <slot v-else />
    </dialog>
  </Teleport>
</template>

<style scoped>
.binding-workbench.shot-configuration.entity-editor-drawer.entity-drawer-resizable[open] {
  position: fixed;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.entity-drawer-resizable .entity-drawer-heading {
  flex-shrink: 0;
  min-width: 0;
}
.entity-drawer-heading h2 {
  overflow-wrap: anywhere;
}
.entity-drawer-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  overscroll-behavior: contain;
  container-type: inline-size;
}
.entity-drawer-resize {
  position: absolute;
  inset: 0 auto 0 0;
  width: 10px;
  z-index: 5;
  cursor: ew-resize;
  touch-action: none;
}
.entity-drawer-resize::after {
  content: "";
  position: absolute;
  left: 2px;
  top: 50%;
  width: 3px;
  height: 48px;
  border-radius: 3px;
  background: #9aafb4;
}
.entity-drawer-resize:hover,
.entity-drawer-resize:focus-visible {
  background: #08796f20;
}
.entity-drawer-resizing {
  user-select: none;
  cursor: ew-resize;
}
</style>
