<script setup lang="ts">
import { computed } from "vue";
import type { OperationBinding } from "../../api/types";
import type { components } from "../../api/schema";

const props = defineProps<{
  modelValue: OperationBinding;
  protocol?: components["schemas"]["ProtocolDescriptor"];
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [OperationBinding] }>();
const mode = computed(
  () =>
    props.modelValue.endpoint_mode ||
    (props.modelValue.endpoint ? "custom" : "default"),
);
const requestPath = computed(() =>
  mode.value === "custom"
    ? props.modelValue.endpoint
    : props.protocol?.default_path,
);

function setMode(next: "default" | "custom") {
  const binding = { ...props.modelValue, endpoint_mode: next };
  if (next === "default") delete binding.endpoint;
  emit("update:modelValue", binding);
}
function setPath(event: Event) {
  emit("update:modelValue", {
    ...props.modelValue,
    endpoint_mode: "custom",
    endpoint: (event.target as HTMLInputElement).value.trim(),
  });
}
</script>

<template>
  <fieldset class="endpoint-settings" :disabled="disabled">
    <legend>请求路径</legend>
    <div class="endpoint-choices">
      <label
        ><input
          type="radio"
          name="endpoint-mode"
          :checked="mode === 'default'"
          @change="setMode('default')"
        />使用协议默认路径</label
      >
      <label
        ><input
          type="radio"
          name="endpoint-mode"
          :checked="mode === 'custom'"
          @change="setMode('custom')"
        />自定义路径</label
      >
    </div>
    <label v-if="mode === 'custom'" class="endpoint-input">
      自定义请求路径
      <input
        aria-label="自定义请求路径"
        :value="modelValue.endpoint || ''"
        required
        placeholder="输入 / 开头的接口路径"
        @input="setPath"
      />
    </label>
    <div class="endpoint-preview" aria-label="实际请求路径" aria-live="polite">
      <span>{{ protocol?.polling_path ? "提交任务" : "生成请求" }} · POST</span>
      <code>{{
        requestPath ||
        (mode === "custom" ? "请填写自定义路径" : "协议路径暂不可用")
      }}</code>
      <template v-if="protocol?.polling_path">
        <span>查询任务 · GET（协议固定路径）</span>
        <code>{{ protocol.polling_path }}</code>
      </template>
    </div>
    <p>
      使用当前厂商连接的域名和密钥。{{
        mode === "custom"
          ? "自定义仅替换生成请求路径，请求格式仍遵循所选协议。"
          : "路径由所选协议提供，无需填写。"
      }}
    </p>
  </fieldset>
</template>

<style scoped>
.endpoint-settings {
  min-width: 0;
  display: grid;
  gap: 12px;
  margin: 0;
  padding: 14px;
  border: 1px solid var(--border, #dbe3ee);
  border-radius: 8px;
}
.endpoint-choices {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 20px;
}
.endpoint-choices label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}
.endpoint-choices input {
  width: auto;
  margin: 0;
}
.endpoint-input {
  display: grid;
  gap: 6px;
  min-width: 0;
}
.endpoint-preview {
  display: grid;
  gap: 6px;
  min-width: 0;
  padding: 12px;
  border-radius: 6px;
  background: #f4f7fa;
}
.endpoint-preview span {
  color: #53657b;
  font-size: 12px;
}
.endpoint-preview code {
  color: #20334b;
  white-space: normal;
  overflow-wrap: anywhere;
  font-size: 12px;
}
.endpoint-preview code + span {
  margin-top: 8px;
}
.endpoint-settings p {
  margin: 0;
  color: #53657b;
  font-size: 12px;
  line-height: 1.6;
}
</style>
