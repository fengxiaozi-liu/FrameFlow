<script setup lang="ts">
import { ref } from "vue";
import { formatAspectRatio } from "../../utils/aspectRatio";

const props = defineProps<{
  modelValue: string[];
  label: string;
  kind: "duration" | "resolution" | "language" | "state" | "ratio";
  presets?: string[];
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [string[]] }>();
const draft = ref("");
const message = ref("");
const input = ref<HTMLInputElement>();
const resolutions = ["480P", "720P", "1080P", "4K"];

function display(value: string) {
  if (props.kind === "ratio") return formatAspectRatio(value);
  return props.kind === "duration" ? `${value} 秒` : value;
}
function contains(value: string) {
  return props.modelValue.some((item) =>
    props.kind === "state" || props.kind === "ratio"
      ? item === value
      : item.toLocaleLowerCase() === value.toLocaleLowerCase(),
  );
}
function clearError() {
  message.value = "";
  input.value?.setCustomValidity("");
}
function add() {
  if (props.disabled || !draft.value.trim()) return;
  clearError();
  let value = draft.value.trim();
  if (props.kind === "duration") {
    if (!/^\d+$/.test(value) || Number(value) < 1 || Number(value) > 3600) {
      message.value = "请输入 1–3600 之间的整数秒数";
      input.value?.setCustomValidity(message.value);
      return;
    }
    value = String(Number(value));
  } else if (/[,，\n\r]/.test(value)) {
    message.value = "请一次添加一个值";
    input.value?.setCustomValidity(message.value);
    return;
  } else if (
    props.kind === "ratio" &&
    ([...value].length > 64 || /[\s\p{Cc}]/u.test(value))
  ) {
    message.value = "比例最多 64 个字符，不能包含空白或控制字符";
    input.value?.setCustomValidity(message.value);
    return;
  } else if (props.kind === "resolution" && /^\d+[pk]$/i.test(value)) {
    value = value.toUpperCase();
  }
  if (contains(value)) {
    message.value = `${display(value)} 已添加`;
  } else {
    if (props.kind === "ratio" && props.modelValue.length >= 64) {
      message.value = "最多添加 64 个比例选项";
      return;
    }
    const next = [...props.modelValue, value];
    if (props.kind === "duration") next.sort((a, b) => Number(a) - Number(b));
    emit("update:modelValue", next);
  }
  draft.value = "";
}
function toggleRatio(value: string) {
  if (props.disabled) return;
  if (contains(value)) remove(value);
  else if (props.modelValue.length < 64)
    emit("update:modelValue", [...props.modelValue, value]);
}
function remove(value: string) {
  if (!props.disabled)
    emit(
      "update:modelValue",
      props.modelValue.filter((item) => item !== value),
    );
}
function toggleResolution(value: string) {
  if (props.disabled) return;
  if (contains(value)) {
    emit(
      "update:modelValue",
      props.modelValue.filter((item) => item.toUpperCase() !== value),
    );
  } else {
    emit("update:modelValue", [...props.modelValue, value]);
  }
}
function onEnter(event: KeyboardEvent) {
  if (event.isComposing) return;
  event.preventDefault();
  add();
}
</script>

<template>
  <fieldset class="option-tags" :disabled="disabled">
    <legend>{{ label }}</legend>
    <div
      v-if="kind === 'ratio'"
      class="option-presets"
      aria-label="常用画面比例"
    >
      <label v-for="value in presets || ['16:9', '9:16', '1:1']" :key="value">
        <input
          type="checkbox"
          :aria-label="value"
          :checked="contains(value)"
          :value="value"
          @change="toggleRatio(value)"
        />{{ display(value) }}
      </label>
    </div>
    <div
      v-if="kind === 'resolution'"
      class="option-presets"
      aria-label="常用清晰度"
    >
      <button
        v-for="value in resolutions"
        :key="value"
        type="button"
        :aria-pressed="contains(value)"
        @click="toggleResolution(value)"
      >
        {{ value }}
      </button>
    </div>
    <ul
      v-if="modelValue.length"
      class="option-values"
      :aria-label="`已选${label}`"
    >
      <li v-for="value in modelValue" :key="value">
        <span>{{ display(value) }}</span>
        <button
          type="button"
          :aria-label="`移除${display(value)}`"
          @click="remove(value)"
        >
          ×
        </button>
      </li>
    </ul>
    <p v-else class="option-empty">尚未添加</p>
    <div class="option-entry">
      <input
        ref="input"
        v-model="draft"
        :aria-label="`添加${label}`"
        :inputmode="kind === 'duration' ? 'numeric' : 'text'"
        :placeholder="
          kind === 'duration'
            ? '输入秒数'
            : kind === 'resolution'
              ? '自定义，如 1440P'
              : kind === 'ratio'
                ? '按模型文档填写，如 adaptive、21:9'
                : kind === 'state'
                  ? '按接口文档逐个添加状态值'
                  : '语言标识，如 Chinese'
        "
        :disabled="disabled"
        @input="clearError"
        @keydown.enter="onEnter"
        @blur="add"
      />
      <button
        type="button"
        :disabled="disabled || !draft.trim()"
        :aria-label="`添加${label}值`"
        @click="add"
      >
        添加
      </button>
    </div>
    <p v-if="message" role="status" class="option-message">{{ message }}</p>
    <p class="option-hint">
      {{
        kind === "resolution"
          ? "按模型文档选择，也可逐个添加自定义值。"
          : "输入后按回车或点添加，点击 × 移除。"
      }}
    </p>
  </fieldset>
</template>

<style scoped>
.option-tags {
  min-width: 0;
  margin: 0;
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border, #dbe3ee);
  border-radius: 8px;
}
.option-presets,
.option-values {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.option-presets button {
  padding: 5px 9px;
  font-size: 12px;
}
.option-presets label {
  display: flex;
  align-items: center;
  gap: 5px;
}
.option-presets input {
  width: auto;
}
.option-presets button[aria-pressed="true"] {
  color: #08746c;
  border-color: #62b5ac;
  background: #e7f4f1;
}
.option-values {
  list-style: none;
  margin: 0;
  padding: 0;
}
.option-values li {
  display: flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  padding: 3px 5px 3px 9px;
  border: 1px solid #c4dfdc;
  border-radius: 6px;
  background: #eef7f5;
  color: #155e59;
  font-size: 13px;
}
.option-values span {
  overflow-wrap: anywhere;
  min-width: 0;
}
.option-values button {
  padding: 0;
  width: 24px;
  height: 24px;
  border: 0;
  background: transparent;
  color: inherit;
}
.option-entry {
  display: flex;
  gap: 6px;
}
.option-entry input {
  min-width: 0;
  flex: 1;
  width: 0;
}
.option-entry button {
  flex-shrink: 0;
  padding-inline: 12px;
}
.option-empty,
.option-hint,
.option-message {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
}
.option-empty,
.option-hint {
  color: #64748b;
}
.option-message {
  color: #9a4d16;
}
</style>
