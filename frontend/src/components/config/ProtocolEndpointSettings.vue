<script setup lang="ts">
import RequestAddressPreview from "./RequestAddressPreview.vue";
import { computed } from "vue";
import type { OperationBinding } from "../../api/types";
import type { components } from "../../api/schema";
const props = defineProps<{
  baseUrl?: string;
  modelValue: OperationBinding;
  protocol?: components["schemas"]["ProtocolDescriptor"];
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [OperationBinding] }>();
const legacy = computed(
  () =>
    props.modelValue.endpoint_mode !== "default" && !!props.modelValue.endpoint,
);
const requestPath = computed(() =>
  legacy.value ? props.modelValue.endpoint : props.protocol?.default_path,
);
function restoreDefault() {
  const next = { ...props.modelValue, endpoint_mode: "default" as const };
  delete next.endpoint;
  emit("update:modelValue", next);
}
</script>
<template>
  <fieldset class="endpoint-settings" :disabled="disabled">
    <legend>内置协议请求路径</legend>
    <div class="endpoint-preview" aria-label="实际请求路径">
      <span>{{ protocol?.polling_path ? "提交任务" : "生成请求" }} · POST</span>
      <code>{{ requestPath || "协议路径暂不可用" }}</code>
      <template v-if="protocol?.polling_path">
        <span>查询任务 · GET</span><code>{{ protocol.polling_path }}</code>
      </template>
    </div>
    <RequestAddressPreview :base-url="baseUrl || ''" :path="requestPath" :polling-path="protocol?.polling_path" />
    <template v-if="legacy">
      <p>
        历史路径配置：继续使用原路径及所选内置协议的数据格式。需要自定义数据格式时，请在上方选择“自定义接口（JSON）”。
      </p>
      <button type="button" @click="restoreDefault">恢复协议默认路径</button>
    </template>
    <p v-else>
      请求格式与路径由内置协议提供，使用当前厂商连接的域名和密钥。其他接口格式请在上方选择“自定义接口（JSON）”。
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
.endpoint-preview {
  display: grid;
  gap: 6px;
  min-width: 0;
  padding: 12px;
  border-radius: 6px;
  background: #f4f7fa;
}
.endpoint-preview span,
p {
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
p {
  margin: 0;
  line-height: 1.6;
}
button {
  justify-self: start;
}
</style>
