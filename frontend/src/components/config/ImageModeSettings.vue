<script setup lang="ts">
import type { OperationBinding } from "../../api/types";
const binding = defineModel<OperationBinding>({ required: true });
defineProps<{ disabled?: boolean }>();
function setLimit(event: Event) {
  binding.value.max_image_bytes =
    Number((event.target as HTMLInputElement).value) * 1048576;
}
function useBailianExample() {
  binding.value = {
    ...binding.value,
    image_modes: ["text_to_image", "image_to_image"],
    min_references: 1,
    max_references: 3,
    max_image_bytes: 10 * 1048576,
    ratios: ["16:9", "9:16", "1:1"],
    image_sizes: { "16:9": "1280*720", "9:16": "720*1280", "1:1": "1024*1024" },
  };
}
</script>

<template>
  <fieldset :disabled="disabled" class="image-mode-settings">
    <legend>图片生成能力（可多选）</legend>
    <div class="mode-options">
      <label
        ><input
          v-model="binding.image_modes"
          type="checkbox"
          value="text_to_image"
        />文生图</label
      >
      <label
        ><input
          v-model="binding.image_modes"
          type="checkbox"
          value="image_to_image"
        />图生图</label
      >
    </div>
    <p class="muted">
      先在这里启用并配置能力，创作时才能选择对应模式。请以模型实际支持范围为准。
    </p>
    <div v-if="binding.image_modes?.includes('image_to_image')" class="limits">
      <label
        >最少参考图片数<input
          v-model.number="binding.min_references"
          type="number"
          min="1"
          max="100"
          placeholder="按文档填写"
      /></label>
      <label
        >最多参考图片数<input
          v-model.number="binding.max_references"
          type="number"
          min="1"
          max="100"
          placeholder="按文档填写"
      /></label>
      <label
        >单张图片上限（MB）<input
          :value="
            binding.max_image_bytes ? binding.max_image_bytes / 1048576 : ''
          "
          type="number"
          min="0.1"
          max="20"
          step="0.1"
          placeholder="按文档填写"
          @input="setLimit"
      /></label>
    </div>
    <p
      v-if="
        binding.image_modes?.includes('image_to_image') &&
        binding.protocol === 'custom.json.v1'
      "
      class="muted"
    >
      在请求 JSON 字段中绑定“参考图片（多图）”参数
      images，运行时按选择顺序传入图片地址数组。
    </p>
    <details v-if="binding.protocol === 'dashscope.image.v1'">
      <summary>百炼配置示例</summary>
      <p class="muted">
        qwen-image-2.0-pro：文生图 / 图生图，1–3 张参考图，单张不超过
        10MB。其他模型请按对应文档调整。
      </p>
      <button type="button" @click="useBailianExample">填入百炼示例参数</button>
      <a
        href="https://help.aliyun.com/zh/model-studio/qwen-image-edit-api"
        target="_blank"
        rel="noreferrer"
        >查看官方图像编辑文档</a
      >
    </details>
  </fieldset>
</template>

<style scoped>
.image-mode-settings {
  display: grid;
  gap: 12px;
  min-width: 0;
}
.mode-options {
  display: flex;
  flex-wrap: wrap;
  gap: 20px;
}
.mode-options label {
  display: flex;
  align-items: center;
  gap: 8px;
}
.mode-options input {
  width: auto;
}
.limits {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 12px;
}
.limits label {
  display: grid;
  gap: 6px;
}
details a {
  display: block;
  margin-top: 8px;
}
</style>
