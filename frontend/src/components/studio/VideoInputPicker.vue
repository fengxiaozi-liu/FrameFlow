<script setup lang="ts">
import { computed } from "vue";
import type { ShotGenerationSettings } from "../../api/bindings";
import type { Material, OperationBinding } from "../../api/types";

const props = defineProps<{
  modelValue: ShotGenerationSettings;
  binding: OperationBinding;
  images: Material[];
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [ShotGenerationSettings] }>();
const modes = computed(
  () =>
    props.binding.input_modes ??
    (props.binding.input_mode ? [props.binding.input_mode] : []),
);
const mode = computed(
  () =>
    props.modelValue.video_input_mode ||
    (modes.value.length === 1 ? modes.value[0] : ""),
);
const slots = computed(() =>
  props.binding.last_frame
    ? (["first", "last"] as const)
    : (["first"] as const),
);
function selected(slot: "first" | "last") {
  return slot === "first"
    ? props.modelValue.first_frame_material_id
    : props.modelValue.last_frame_material_id;
}
function chooseMode(value: "first_frame" | "reference_images") {
  emit("update:modelValue", { ...props.modelValue, video_input_mode: value });
}
function chooseImage(slot: "first" | "last", event: Event) {
  emit("update:modelValue", {
    ...props.modelValue,
    video_input_mode: "first_frame",
    [slot === "first" ? "first_frame_material_id" : "last_frame_material_id"]:
      (event.target as HTMLSelectElement).value || undefined,
  });
}
</script>

<template>
  <fieldset class="video-input-picker" :disabled="disabled">
    <legend>生成模式</legend>
    <div class="video-mode-options">
      <button
        type="button"
        :aria-pressed="mode === 'first_frame'"
        :disabled="!modes.includes('first_frame')"
        @click="chooseMode('first_frame')"
      >
        <strong>{{ binding.last_frame ? "首尾帧模式" : "首帧模式" }}</strong>
        <span>{{
          modes.includes("first_frame")
            ? binding.last_frame
              ? "指定画面的起点与终点"
              : "指定视频开始的画面"
            : "当前模型不支持"
        }}</span>
      </button>
      <button
        type="button"
        :aria-pressed="mode === 'reference_images'"
        :disabled="!modes.includes('reference_images')"
        @click="chooseMode('reference_images')"
      >
        <strong>智能参考模式</strong
        ><span>{{
          modes.includes("reference_images")
            ? "综合人物、场景等引用图片生成"
            : "当前模型不支持"
        }}</span>
      </button>
    </div>
    <p v-if="!mode" role="status">请选择本次视频生成模式。</p>
    <template v-else-if="mode === 'first_frame'">
      <p class="muted">
        从当前分镜引用图片中选择首帧{{
          binding.last_frame
            ? "和尾帧；尾帧可留空，仅使用首帧生成"
            : "；此模型不支持尾帧"
        }}。其他引用图片不会作为画面输入。
      </p>
      <div class="video-frame-slots">
        <div v-for="slot in slots" :key="slot" class="video-frame-slot">
          <label
            >{{ slot === "first" ? "首帧图片（必选）" : "尾帧图片（可选）" }}
            <select
              :aria-label="slot === 'first' ? '首帧图片' : '尾帧图片'"
              :value="selected(slot) || ''"
              @change="chooseImage(slot, $event)"
            >
              <option value="">
                {{ slot === "first" ? "请选择首帧" : "不使用尾帧" }}
              </option>
              <option
                v-if="
                  selected(slot) &&
                  !images.some((image) => image.id === selected(slot))
                "
                :value="selected(slot)"
                disabled
              >
                原图片已移除，请重新选择
              </option>
              <option v-for="image in images" :key="image.id" :value="image.id">
                {{ image.name }}
              </option>
            </select>
          </label>
          <img
            v-if="images.find((image) => image.id === selected(slot))"
            :src="images.find((image) => image.id === selected(slot))!.url"
            :alt="slot === 'first' ? '首帧预览' : '尾帧预览'"
          />
          <div v-else class="frame-placeholder">
            {{ slot === "first" ? "视频从这张图开始" : "可指定视频结束画面" }}
          </div>
        </div>
      </div>
    </template>
    <template v-else>
      <p class="muted">
        已引用 {{ images.length }} 张图片，模型最多支持
        {{ binding.max_references }}
        张。图片用于参考人物与场景，不指定首尾顺序。
      </p>
      <div class="video-reference-strip">
        <img
          v-for="image in images"
          :key="image.id"
          :src="image.url"
          :alt="image.name"
          :title="image.name"
        />
      </div>
    </template>
    <p v-if="!images.length" class="muted">
      请先在上方添加场景或人物，并选择参考图片。
    </p>
  </fieldset>
</template>

<style scoped>
.video-input-picker {
  min-width: 0;
  border: 1px solid var(--border, #dbe3ee);
  border-radius: 8px;
  margin-top: 18px;
  padding: 14px;
}
.video-mode-options,
.video-frame-slots {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.video-mode-options button {
  text-align: left;
  display: grid;
  gap: 5px;
  padding: 12px;
  white-space: normal;
}
.video-mode-options button[aria-pressed="true"] {
  border-color: #0f766e;
  background: #eaf5f2;
  color: #0f766e;
}
.video-mode-options span,
.video-input-picker p {
  font-size: 12px;
}
.video-frame-slot {
  min-width: 0;
}
.video-frame-slot label {
  display: grid;
  gap: 6px;
}
.video-frame-slot select {
  min-width: 0;
  width: 100%;
}
.video-frame-slot img,
.frame-placeholder {
  width: 100%;
  height: 150px;
  object-fit: contain;
  border-radius: 6px;
  background: #f3f6f8;
  margin-top: 8px;
}
.frame-placeholder {
  display: grid;
  place-items: center;
  color: #607080;
  font-size: 12px;
}
.video-reference-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.video-reference-strip img {
  width: 72px;
  height: 54px;
  object-fit: cover;
  border-radius: 4px;
}
@media (max-width: 600px) {
  .video-mode-options,
  .video-frame-slots {
    grid-template-columns: 1fr;
  }
}
</style>
