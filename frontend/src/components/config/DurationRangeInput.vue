<script setup lang="ts">
import { computed, ref, watch, watchEffect } from "vue";
import ModelOptionTags from "./ModelOptionTags.vue";

const props = defineProps<{ modelValue: number[]; disabled?: boolean }>();
const emit = defineEmits<{ "update:modelValue": [number[]] }>();
const mode = ref("range");
const minimum = ref<number | string>("");
const maximum = ref<number | string>("");
const maximumInput = ref<HTMLInputElement>();
let lastEmitted: number[] | undefined;

watch(
  () => props.modelValue,
  (values) => {
    // Parent echoes must not erase an incomplete range while the user types.
    if (lastEmitted && JSON.stringify(values) === JSON.stringify(lastEmitted))
      return;
    const sorted = [...new Set(values)].sort((a, b) => a - b);
    mode.value = sorted.some(
      (value, index) => index > 0 && value !== sorted[index - 1]! + 1,
    )
      ? "specific"
      : "range";
    minimum.value = sorted[0] ?? "";
    maximum.value = sorted[sorted.length - 1] ?? "";
  },
  { immediate: true, deep: true },
);

const valid = computed(
  () =>
    [minimum.value, maximum.value].every(
      (value) =>
        value !== "" &&
        Number.isInteger(Number(value)) &&
        Number(value) >= 1 &&
        Number(value) <= 3600,
    ) && Number(minimum.value) <= Number(maximum.value),
);
const reversed = computed(
  () =>
    minimum.value !== "" &&
    maximum.value !== "" &&
    Number(minimum.value) > Number(maximum.value),
);
watchEffect(() =>
  maximumInput.value?.setCustomValidity(
    reversed.value ? "最长时长不能小于最短时长" : "",
  ),
);

function publish(values: number[]) {
  lastEmitted = values;
  emit("update:modelValue", values);
}
function updateRange() {
  publish(
    valid.value
      ? Array.from(
          { length: Number(maximum.value) - Number(minimum.value) + 1 },
          (_, i) => Number(minimum.value) + i,
        )
      : [],
  );
}
function changeMode() {
  if (mode.value === "range") updateRange();
}
function updateSpecific(values: string[]) {
  const seconds = values.map(Number);
  minimum.value = seconds[0] ?? "";
  maximum.value = seconds[seconds.length - 1] ?? "";
  publish(seconds);
}
</script>

<template>
  <fieldset class="duration-range" :disabled="disabled">
    <legend>可选时长</legend>
    <select v-model="mode" aria-label="时长填写方式" @change="changeMode">
      <option value="range">时长范围</option>
      <option value="specific">指定时长</option>
    </select>
    <template v-if="mode === 'range'">
      <div class="duration-bounds">
        <label
          >最短（秒）<input
            v-model="minimum"
            type="number"
            aria-label="最短时长（秒）"
            min="1"
            max="3600"
            step="1"
            required
            placeholder="如 4"
            @input="updateRange"
        /></label>
        <span aria-hidden="true">—</span>
        <label
          >最长（秒）<input
            ref="maximumInput"
            v-model="maximum"
            type="number"
            aria-label="最长时长（秒）"
            min="1"
            max="3600"
            step="1"
            required
            placeholder="如 10"
            @input="updateRange"
        /></label>
      </div>
      <p v-if="reversed" role="alert" class="error">
        最长时长不能小于最短时长。
      </p>
      <p v-else class="muted">
        {{
          valid
            ? `${minimum}–${maximum} 秒，含起止值，每 1 秒可选。`
            : "按模型文档填写，支持 1–3600 秒的整数。"
        }}起止相同表示固定时长。
      </p>
    </template>
    <template v-else>
      <ModelOptionTags
        label="指定秒数"
        kind="duration"
        :model-value="modelValue.map(String)"
        :disabled="disabled"
        @update:model-value="updateSpecific"
      />
      <p class="muted">仅允许已添加的秒数；适用于只支持固定档位的模型。</p>
    </template>
  </fieldset>
</template>

<style scoped>
.duration-range {
  min-width: 0;
  display: grid;
  align-content: start;
  gap: 12px;
}
.duration-range select,
.duration-range input {
  min-width: 0;
  width: 100%;
}
.duration-bounds {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: end;
  gap: 6px;
}
.duration-bounds label {
  display: grid;
  gap: 6px;
}
.duration-bounds span {
  align-self: end;
  padding-bottom: 12px;
}
.duration-range p {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
}
</style>
