<script setup lang="ts">
import { computed } from "vue";
import type { components } from "../../api/schema";
import { jsonFields } from "./jsonExample";
import { keyToPointer, pointerToKey } from "./jsonKeyPath";
import RequestAddressPreview from "./RequestAddressPreview.vue";
type Profile = components["schemas"]["DiscoveryProfile"];
const props = defineProps<{
  modelValue: Profile;
  baseUrl: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [Profile] }>();
const parsed = computed(() =>
  jsonFields(props.modelValue.response_json_example || ""),
);
const itemFields = computed(() => {
  try {
    let value: unknown = JSON.parse(
      props.modelValue.response_json_example || "",
    );
    for (const token of props.modelValue.model_list_pointer
      .slice(1)
      .split("/")) {
      if (!value || typeof value !== "object") return [];
      value = (value as Record<string, unknown>)[
        token.replaceAll("~1", "/").replaceAll("~0", "~")
      ];
    }
    return Array.isArray(value) && value.length
      ? jsonFields(JSON.stringify(value[0])).fields.filter(
          (item) => item.type === "string",
        )
      : [];
  } catch {
    return [];
  }
});
function set<K extends keyof Profile>(key: K, value: Profile[K]) {
  const next = { ...props.modelValue, schema: undefined, [key]: value };
  if (key === "method" && value === "GET") next.request_json = undefined;
  emit("update:modelValue", next);
}
function fieldValue(key: "model_list_pointer" | "model_id_pointer" | "model_name_pointer") {
  return pointerToKey(props.modelValue[key]);
}
function setField(key: "model_list_pointer" | "model_id_pointer" | "model_name_pointer", value: string) {
  set(key, keyToPointer(value));
}
function paginationFieldValue(key: "has_more_pointer" | "next_cursor_pointer") {
  return pointerToKey(props.modelValue.pagination?.[key] || "");
}
function setPaginationField(key: "has_more_pointer" | "next_cursor_pointer", value: string) {
  pagination(key, keyToPointer(value));
}
function pagination(key: string, value: string | number) {
  emit("update:modelValue", {
    ...props.modelValue,
    pagination: { ...props.modelValue.pagination, [key]: value },
  });
}
</script>

<template>
  <fieldset class="discovery-profile" :disabled="disabled">
    <legend>模型同步接口</legend>
    <p class="muted">
      仅同步模型 ID 和名称。文生图、视频等能力在单个模型的“协议与参数”中设置。
    </p>
    <div class="discovery-grid">
      <label
        >请求方式<select
          :value="modelValue.method"
          @change="
            set(
              'method',
              ($event.target as HTMLSelectElement).value as Profile['method'],
            )
          "
        >
          <option value="GET">GET</option>
          <option value="POST">POST</option>
        </select></label
      >
      <label
        >同步路径<input
          :value="modelValue.path"
          required
          placeholder="/v1/models"
          @input="set('path', ($event.target as HTMLInputElement).value)"
      /></label>
    </div>
    <label v-if="modelValue.method === 'POST'"
      >请求 JSON 示例<textarea
        :value="modelValue.request_json"
        rows="3"
        @input="
          set('request_json', ($event.target as HTMLTextAreaElement).value)
        "
      />
    </label>
    <label
      >响应 JSON 示例<textarea
        :value="modelValue.response_json_example"
        rows="5"
        placeholder='{"data":[{"id":"model-id","name":"模型名称"}]}'
        spellcheck="false"
        @input="
          set(
            'response_json_example',
            ($event.target as HTMLTextAreaElement).value,
          )
        "
      />
    </label>
    <p
      v-if="modelValue.response_json_example && parsed.error"
      role="alert"
      class="error"
    >
      {{ parsed.error }}
    </p>
    <p v-else-if="modelValue.response_json_example" role="status" class="muted">
      已解析 {{ parsed.fields.length }} 个字段，下方映射将用于实际同步响应。
    </p>
    <div class="discovery-grid">
      <label
        >模型列表 Key<input
          :value="fieldValue('model_list_pointer')"
          list="discovery-list-fields"
          required
          placeholder="data"
          @input="
            setField('model_list_pointer', ($event.target as HTMLInputElement).value)
          "
      /></label>
      <label
        >模型 ID Key<input
          :value="fieldValue('model_id_pointer')"
          list="discovery-item-fields"
          required
          placeholder="id"
          @input="
            setField('model_id_pointer', ($event.target as HTMLInputElement).value)
          "
      /></label>
      <label
        >显示名称 Key（可选）<input
          :value="fieldValue('model_name_pointer')"
          list="discovery-item-fields"
          placeholder="name"
          @input="
            setField('model_name_pointer', ($event.target as HTMLInputElement).value)
          "
      /></label>
      <label
        >分页方式<select
          :value="modelValue.pagination?.mode || 'none'"
          @change="
            pagination('mode', ($event.target as HTMLSelectElement).value)
          "
        >
          <option value="none">不分页</option>
          <option value="page">页码分页</option>
          <option value="cursor">游标分页</option>
        </select></label
      >
    </div>
    <datalist id="discovery-list-fields">
      <option
        v-for="field in parsed.fields.filter((item) => item.type === 'array')"
        :key="field.pointer"
        :value="pointerToKey(field.pointer)"
      />
    </datalist>
    <datalist id="discovery-item-fields">
      <option
        v-for="field in itemFields"
        :key="field.pointer"
        :value="pointerToKey(field.pointer)"
      />
    </datalist>
    <p class="muted">
      列表 Key 从响应根节点读取；ID 和名称 Key 相对于每个模型条目，例如
      <code>id</code>。支持用点号表示嵌套 Key，例如 <code>result.models</code>。名称未配置或为空时使用模型 ID。
    </p>
    <div v-if="modelValue.pagination?.mode === 'page'" class="discovery-grid">
      <label
        >页码参数<input
          :value="modelValue.pagination.page_param"
          required
          placeholder="page"
          @input="
            pagination('page_param', ($event.target as HTMLInputElement).value)
          "
      /></label>
      <label
        >每页大小<input
          :value="modelValue.pagination.page_size"
          required
          type="number"
          min="1"
          max="2000"
          @input="
            pagination(
              'page_size',
              Number(($event.target as HTMLInputElement).value),
            )
          "
      /></label>
      <label
        >页面大小参数（可选）<input
          :value="modelValue.pagination.page_size_param"
          placeholder="page_size"
          @input="
            pagination(
              'page_size_param',
              ($event.target as HTMLInputElement).value,
            )
          "
      /></label>
    </div>
    <div v-if="modelValue.pagination?.mode === 'cursor'" class="discovery-grid">
      <label
        >游标参数<input
          :value="modelValue.pagination.cursor_param"
          required
          placeholder="after"
          @input="
            pagination(
              'cursor_param',
              ($event.target as HTMLInputElement).value,
            )
          "
      /></label>
      <label
        >是否有更多 Key<input
          :value="paginationFieldValue('has_more_pointer')"
          required
          placeholder="has_more"
          @input="
            setPaginationField('has_more_pointer', ($event.target as HTMLInputElement).value)
          "
      /></label>
      <label
        >下一游标 Key<input
          :value="paginationFieldValue('next_cursor_pointer')"
          required
          placeholder="last_id"
          @input="
            setPaginationField('next_cursor_pointer', ($event.target as HTMLInputElement).value)
          "
      /></label>
    </div>
    <RequestAddressPreview :base-url="baseUrl" :path="modelValue.path" :method="modelValue.method" />
  </fieldset>
</template>

<style scoped>
.discovery-profile {
  display: grid;
  gap: 12px;
  min-width: 0;
  border: 1px solid var(--border);
  padding: 12px;
  border-radius: 8px;
}
.discovery-profile label {
  display: grid;
  gap: 6px;
  min-width: 0;
}
.discovery-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
.discovery-profile textarea {
  width: 100%;
  font-family: Consolas, monospace;
  resize: vertical;
}
@media (max-width: 640px) {
  .discovery-grid {
    grid-template-columns: 1fr;
  }
}
</style>
