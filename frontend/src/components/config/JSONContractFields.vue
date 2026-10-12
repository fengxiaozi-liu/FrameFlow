<script setup lang="ts">
import { computed, ref, nextTick } from "vue";
import type { components } from "../../api/schema";
import { jsonFields, type JSONField } from "./jsonExample";
import JSONFieldTree from "./JSONFieldTree.vue";
import { pointerToKey } from "./jsonKeyPath";
import {
  jsonFieldTree,
  bindingType,
  bindingUnavailable,
  exampleValue,
  updateExampleValue,
  type BusinessField,
  type ContractDirection,
} from "./jsonContractFields";

type Contract = components["schemas"]["JSONContract"];
const props = defineProps<{
  modelValue: Contract;
  title: string;
  direction: ContractDirection;
  fields: BusinessField[];
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [Contract] }>();
const editorOpen = ref(false);
const editor = ref<HTMLTextAreaElement>();
const parsed = computed(() => jsonFields(props.modelValue.example));
const root = computed<unknown>(() =>
  parsed.value.error ? null : JSON.parse(props.modelValue.example),
);
const tree = computed(() => jsonFieldTree(parsed.value.fields));
const staleBindings = computed(() =>
  parsed.value.error
    ? []
    : Object.entries(props.modelValue.bindings).filter(
        ([id, path]) =>
          !props.fields.some((field) => field.id === id) ||
          !parsed.value.fields.some((field) => field.pointer === path),
      ),
);
const missingRequired = computed(() =>
  props.fields.filter(
    (business) =>
      business.required &&
      !parsed.value.fields.some(
        (field) =>
          field.pointer === props.modelValue.bindings[business.id] &&
          field.type === bindingType(business.id, props.direction),
      ),
  ),
);
function update(patch: Partial<Contract>) {
  emit("update:modelValue", {
    ...props.modelValue,
    ...patch,
    schema: undefined,
  });
}
function boundIDs(pointer: string): string[] {
  return Object.entries(props.modelValue.bindings)
    .filter(([, path]) => path === pointer)
    .map(([id]) => id);
}
function label(id: string): string {
  return props.fields.find((field) => field.id === id)?.label || id;
}
function coveredBy(pointer: string): string | undefined {
  if (props.direction !== "request") return;
  return Object.values(props.modelValue.bindings).find((path) =>
    pointer.startsWith(`${path}/`),
  );
}
function bindingProblem(field: JSONField): string {
  const ids = boundIDs(field.pointer);
  if (ids.length > 1) return "同一字段绑定了多个参数，请先解除后重新选择";
  const business = props.fields.find((item) => item.id === ids[0]);
  return business
    ? bindingUnavailable(
        field,
        business,
        props.modelValue.bindings,
        props.direction,
      )
    : "";
}
function bind(field: JSONField, event: Event) {
  const select = event.target as HTMLSelectElement;
  const id = select.value;
  const business = props.fields.find((item) => item.id === id);
  if (
    id &&
    (!business ||
      bindingUnavailable(
        field,
        business,
        props.modelValue.bindings,
        props.direction,
      ))
  ) {
    select.value = boundIDs(field.pointer)[0] || "";
    return;
  }
  const bindings = { ...props.modelValue.bindings };
  for (const oldID of boundIDs(field.pointer)) delete bindings[oldID];
  if (id) bindings[id] = field.pointer;
  update({ bindings });
}
function unbind(id: string) {
  const bindings = { ...props.modelValue.bindings };
  delete bindings[id];
  update({ bindings });
}
function sample(field: JSONField): string {
  const value = exampleValue(root.value, field.pointer);
  return typeof value === "string" ? value : (JSON.stringify(value) ?? "");
}
function editFixed(field: JSONField, event: Event) {
  const input = event.target as HTMLInputElement;
  let value: string | number | boolean = input.value;
  if (field.type === "number") {
    if (!value.trim() || !Number.isFinite(Number(value))) {
      input.setCustomValidity("请输入有效数字，不能留空");
      input.reportValidity();
      return;
    }
    value = Number(value);
    input.setCustomValidity("");
  } else if (field.type === "boolean") value = value === "true";
  update({
    example: updateExampleValue(props.modelValue.example, field.pointer, value),
  });
}
async function openEditor() {
  editorOpen.value = true;
  await nextTick();
  editor.value?.focus();
}
</script>

<template>
  <fieldset class="json-contract-fields" :disabled="disabled">
    <legend>{{ title }}字段</legend>
    <details
      :open="editorOpen || !!parsed.error"
      @toggle="editorOpen = ($event.target as HTMLDetailsElement).open"
    >
      <summary :aria-label="`${title} 粘贴或修改 JSON`">
        粘贴 / 修改 JSON <span class="muted">自动解析字段</span>
      </summary>
      <label class="json-editor">
        {{ title }} JSON 示例
        <textarea
          ref="editor"
          :value="modelValue.example"
          rows="6"
          spellcheck="false"
          @input="
            update({ example: ($event.target as HTMLTextAreaElement).value })
          "
        />
      </label>
    </details>
    <p v-if="parsed.error" class="error" role="alert">
      {{ parsed.error }}；已有绑定保留，修正后继续编辑。
    </p>
    <template v-else>
      <p class="field-hint">
        {{
          direction === "request"
            ? "绑定后使用本次生成的参数；不绑定则保留示例值，可直接修改固定值。"
            : "绑定后提取实际响应中的值；不绑定的字段会被忽略。"
        }}
      </p>
      <div v-if="staleBindings.length" class="mapping-warning" role="status">
        <strong>以下绑定需要处理</strong>
        <div
          v-for="[id, path] in staleBindings"
          :key="id"
          class="stale-binding"
        >
          <span
            >{{ label(id) }} → <code>{{ pointerToKey(path) }}</code
            >（字段缺失或参数不再支持）</span
          >
          <button
            type="button"
            :aria-label="`${title} 解除${label(id)}绑定`"
            @click="unbind(id)"
          >
            解除绑定
          </button>
        </div>
      </div>
      <div
        v-if="tree.length"
        class="field-table"
        role="group"
        :aria-label="`${title}字段列表`"
      >
        <div class="field-header" aria-hidden="true">
          <span>字段 Key / 类型</span>
          <span>{{
            direction === "request" ? "示例值 / 固定值" : "示例值"
          }}</span>
          <span>{{ direction === "request" ? "生成时取值" : "提取为" }}</span>
        </div>
        <JSONFieldTree
          :nodes="tree"
          :title="title"
          :bindings="modelValue.bindings"
        >
          <template #value="{ field }">
            <div
              v-if="field.type === 'object' || field.type === 'array'"
              class="container-summary"
            >
              <span>{{
                field.children.length
                  ? field.children.length +
                    (field.type === "array" ? " 项" : " 个字段")
                  : field.type === "array"
                    ? "空数组"
                    : "空对象"
              }}</span>
              <button
                v-if="
                  direction === 'request' &&
                  !boundIDs(field.pointer).length &&
                  !coveredBy(field.pointer)
                "
                type="button"
                class="structure-edit"
                :aria-label="`${title} ${pointerToKey(field.pointer)} 在 JSON 中编辑`"
                @click="openEditor"
              >
                编辑 JSON
              </button>
            </div>
            <template
              v-else-if="
                direction === 'request' &&
                !boundIDs(field.pointer).length &&
                !coveredBy(field.pointer)
              "
            >
              <input
                v-if="field.type === 'string' || field.type === 'number'"
                :type="field.type === 'number' ? 'number' : 'text'"
                step="any"
                :value="sample(field)"
                :aria-label="`${title} ${pointerToKey(field.pointer)} 固定值`"
                @change="editFixed(field, $event)"
              />
              <select
                v-else-if="field.type === 'boolean'"
                :value="sample(field)"
                :aria-label="`${title} ${pointerToKey(field.pointer)} 固定值`"
                @change="editFixed(field, $event)"
              >
                <option value="true">true</option>
                <option value="false">false</option>
              </select>
              <button
                v-else
                type="button"
                class="json-value-button"
                :aria-label="`${title} ${pointerToKey(field.pointer)} 在 JSON 中编辑`"
                @click="openEditor"
              >
                {{ sample(field).slice(0, 80) }} <span>编辑 JSON</span>
              </button>
            </template>
            <code v-else class="sample-value" :title="sample(field)">{{
              sample(field).slice(0, 120) || '""'
            }}</code>
          </template>
          <template #binding="{ field }">
            <span
              v-if="field.type === 'object' && !boundIDs(field.pointer).length"
              class="structure-note"
              >{{
                coveredBy(field.pointer)
                  ? "随父级参数传入"
                  : field.children.length
                    ? "在下级字段中设置"
                    : "暂无子字段"
              }}</span
            >
            <select
              v-else
              :value="boundIDs(field.pointer)[0] || ''"
              :aria-label="`${title} ${pointerToKey(field.pointer)} 绑定参数`"
              :disabled="
                !!coveredBy(field.pointer) && !boundIDs(field.pointer).length
              "
              @change="bind(field, $event)"
            >
              <option value="">
                {{
                  coveredBy(field.pointer)
                    ? "随父级参数传入"
                    : direction === "request"
                      ? "不绑定 · 保留示例值"
                      : "不绑定 · 忽略此字段"
                }}
              </option>
              <option
                v-for="business in fields"
                :key="business.id"
                :value="business.id"
                :disabled="
                  !!bindingUnavailable(
                    field,
                    business,
                    modelValue.bindings,
                    direction,
                  )
                "
              >
                {{ business.label
                }}{{
                  bindingUnavailable(
                    field,
                    business,
                    modelValue.bindings,
                    direction,
                  )
                    ? `（${bindingUnavailable(field, business, modelValue.bindings, direction)}）`
                    : ""
                }}
              </option>
            </select>
            <span v-if="bindingProblem(field)" class="error">{{
              bindingProblem(field)
            }}</span>
            <span v-else-if="coveredBy(field.pointer)" class="muted"
              >由 {{ pointerToKey(coveredBy(field.pointer)!) }} 整体替换</span
            >
          </template>
        </JSONFieldTree>
      </div>
      <p v-else class="muted">暂无可配置字段，请在 JSON 中添加字段。</p>
      <p v-if="missingRequired.length" class="mapping-warning" role="status">
        保存前还需绑定：{{
          missingRequired.map((field) => field.label).join("、")
        }}。每个字段均可不绑定，但协议需要的参数必须有对应字段。
      </p>
    </template>
  </fieldset>
</template>

<style scoped>
.json-contract-fields {
  display: grid;
  gap: 12px;
  border: 1px solid var(--border);
  padding: 14px;
  border-radius: 8px;
  min-width: 0;
  container-type: inline-size;
}
legend {
  font-weight: 600;
  padding: 0 5px;
}
summary {
  cursor: pointer;
  font-size: 13px;
}
summary span {
  margin-left: 8px;
  font-size: 12px;
}
.json-editor {
  display: grid;
  gap: 6px;
  margin-top: 12px;
}
.json-editor textarea {
  width: 100%;
  min-width: 0;
  font-family: Consolas, monospace;
  resize: vertical;
}
.field-hint {
  color: var(--muted);
  font-size: 12px;
  margin: 0;
  line-height: 1.6;
}
.field-table {
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}
.field-header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1.4fr);
  gap: 12px;
  padding: 10px;
  border-top: 0;
  background: #f5f7fa;
  color: var(--muted);
  font-size: 12px;
}

.sample-value {
  display: block;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
  font-size: 12px;
  line-height: 1.6;
  max-height: 80px;
  overflow: auto;
}
.field-table input,
.field-table select {
  width: 100%;
  min-width: 0;
  font-size: 12px;
  padding: 7px 6px;
}
.field-table span.error,
.field-table span.muted {
  font-size: 12px;
  overflow-wrap: anywhere;
}

.mapping-warning {
  background: #fff8e8;
  color: #795214;
  padding: 10px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.6;
  margin: 0;
}
.stale-binding {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-top: 6px;
}
.stale-binding span {
  overflow-wrap: anywhere;
  min-width: 0;
}
.stale-binding button {
  flex-shrink: 0;
  font-size: 12px;
}
.json-value-button {
  display: grid;
  width: 100%;
  text-align: left;
  overflow-wrap: anywhere;
  font-size: 12px;
}
.json-value-button span {
  color: var(--muted);
}
@container (max-width: 500px) {
  .field-header {
    display: none;
  }
}
.container-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 10px;
  align-items: center;
  font-size: 12px;
  color: var(--muted);
}
.structure-edit {
  font-size: 12px;
  padding: 2px 4px;
  background: transparent;
  color: #007b72;
  border: 0;
}
.structure-note {
  font-size: 12px;
  color: var(--muted);
}
</style>
