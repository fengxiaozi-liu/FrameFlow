<script setup lang="ts">
import { computed } from "vue";
import ModelOptionTags from "./ModelOptionTags.vue";
import type { Capability } from "../../api/types";
import type { components } from "../../api/schema";
type CustomInterface = components["schemas"]["CustomInterface"];
const props = defineProps<{
  modelValue: CustomInterface;
  capability: Capability;
  variables: string[];
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [CustomInterface] }>();
function update<K extends keyof CustomInterface>(
  key: K,
  value: CustomInterface[K],
) {
  emit("update:modelValue", { ...props.modelValue, [key]: value });
}
function text(event: Event) {
  return (event.target as HTMLInputElement).value;
}
function setMode(event: Event) {
  const mode = text(event) as CustomInterface["mode"];
  const next = { ...props.modelValue, mode };
  if (mode === "async")
    next.polling ||= {
      path: "",
      task_id_pointer: "",
      status_pointer: "",
      pending: [],
      succeeded: [],
      failed: [],
    };
  else delete next.polling;
  emit("update:modelValue", next);
}
function updatePolling(key: string, value: string | string[]) {
  if (props.modelValue.polling)
    update("polling", { ...props.modelValue.polling, [key]: value });
}
function bodyInput(event: Event) {
  const input = event.target as HTMLTextAreaElement;
  update("body_template", input.value);
  input.setCustomValidity(templateError(input.value));
}
function templateError(value: string) {
  try {
    const parsed = JSON.parse(value);
    return !parsed || typeof parsed !== "object" || Array.isArray(parsed)
      ? "请求模板必须是 JSON 对象"
      : "";
  } catch {
    return "请输入有效的 JSON 对象";
  }
}
const bodyError = computed(() => templateError(props.modelValue.body_template));
const requestExample = '"prompt": "{{prompt}}"';
const states = [
  { key: "pending" as const, label: "等待或进行中状态" },
  { key: "succeeded" as const, label: "成功状态" },
  { key: "failed" as const, label: "失败或取消状态" },
];
</script>
<template>
  <fieldset class="custom-interface" :disabled="disabled">
    <legend>自定义接口格式</legend>
    <p>
      按服务商文档配置 JSON 接口。以下示例仅说明填写方式，不代表服务商默认参数。
    </p>
    <section>
      <h4>1. 请求与认证</h4>
      <label
        >生成请求路径 · POST<input
          aria-label="生成请求路径"
          :value="modelValue.path"
          required
          placeholder="例如 /v1/generate"
          @input="update('path', text($event).trim())"
      /></label>
      <p>
        使用当前厂商连接的域名。填写从 /
        开始的完整路径，替换连接地址中的路径部分。
      </p>
      <label
        >认证方式<select
          aria-label="认证方式"
          :value="modelValue.auth_mode"
          @change="
            update('auth_mode', text($event) as CustomInterface['auth_mode'])
          "
        >
          <option value="bearer">Bearer Token（使用连接密钥）</option>
          <option value="header">自定义认证请求头（使用连接密钥）</option>
          <option value="none">无需认证</option>
        </select></label
      >
      <template v-if="modelValue.auth_mode === 'header'">
        <label
          >认证请求头名称<input
            :value="modelValue.auth_header"
            required
            placeholder="例如 X-API-Key"
            @input="update('auth_header', text($event).trim())"
        /></label>
        <label
          >密钥前缀（可选）<input
            :value="modelValue.auth_prefix"
            placeholder="可留空，按文档填写，保留末尾空格"
            @input="update('auth_prefix', text($event))"
        /></label>
      </template>
      <p>密钥从厂商连接读取，请勿写入模板。</p>
    </section>
    <section>
      <h4>2. 请求数据</h4>
      <label
        >JSON 请求模板<textarea
          aria-label="JSON 请求模板"
          :value="modelValue.body_template"
          required
          rows="9"
          spellcheck="false"
          @input="bodyInput"
        />
      </label>
      <p v-if="bodyError" role="alert">{{ bodyError }}</p>
      <div class="variables">
        <span>可用变量</span
        ><code
          v-for="variable in variables"
          :key="variable"
          v-text="'{{' + variable + '}}'"
        ></code>
      </div>
      <p>
        变量须占据整个字符串值，例如
        <code v-text="requestExample"></code
        >。提交时自动转义文字，时长保留数字，images 为图片地址数组，messages
        为角色和内容组成的消息数组。固定参数可直接写入 JSON。
      </p>
    </section>
    <section>
      <h4>3. 返回结果</h4>
      <label
        >返回方式<select
          aria-label="返回方式"
          :value="modelValue.mode"
          @change="setMode"
        >
          <option value="sync">同步返回结果</option>
          <option v-if="capability === 'video'" value="async">
            异步任务，轮询查询
          </option>
        </select></label
      >
      <label
        >{{ capability === "story" ? "文本结果字段" : "媒体 URL 字段"
        }}<input
          aria-label="结果字段路径"
          :value="modelValue.result_pointer"
          required
          placeholder="例如 /data/0/url"
          @input="update('result_pointer', text($event).trim())"
      /></label>
      <p>
        字段路径按层级填写：<code>/data/0/url</code> 表示 data 数组第一项的
        url。{{
          modelValue.mode === "async"
            ? "填写查询成功响应中的结果字段。"
            : "填写生成响应中的结果字段。"
        }}媒体结果须为 HTTP(S) 地址。
      </p>
    </section>
    <section v-if="modelValue.mode === 'async' && modelValue.polling">
      <h4>4. 任务查询与状态</h4>
      <label
        >提交响应的任务 ID 字段<input
          :value="modelValue.polling.task_id_pointer"
          required
          placeholder="例如 /data/id"
          @input="updatePolling('task_id_pointer', text($event).trim())"
      /></label>
      <label
        >查询请求路径 · GET<input
          :value="modelValue.polling.path"
          required
          placeholder="例如 /v1/tasks/{task_id}"
          @input="updatePolling('path', text($event).trim())"
      /></label>
      <label
        >查询响应的状态字段<input
          :value="modelValue.polling.status_pointer"
          required
          placeholder="例如 /data/status"
          @input="updatePolling('status_pointer', text($event).trim())"
      /></label>
      <p>
        查询沿用同一连接和认证方式。状态值区分大小写，每组至少填写一个，不能重复。
      </p>
      <ModelOptionTags
        v-for="state in states"
        :key="state.key"
        :label="state.label"
        kind="state"
        :model-value="modelValue.polling[state.key]"
        :disabled="disabled"
        @update:model-value="updatePolling(state.key, $event)"
      />
    </section>
    <p>
      当前支持 JSON POST 与视频 GET
      轮询；不支持文件表单、流式响应、签名脚本及异步图片或语音接口。
    </p>
  </fieldset>
</template>
<style scoped>
.custom-interface {
  min-width: 0;
  display: grid;
  gap: 18px;
  padding: 14px;
  border: 1px solid var(--border, #dbe3ee);
  border-radius: 8px;
}
section {
  min-width: 0;
  display: grid;
  gap: 10px;
}
section + section {
  border-top: 1px solid var(--border, #dbe3ee);
  padding-top: 16px;
}
h4,
p {
  margin: 0;
}
p {
  font-size: 12px;
  color: #53657b;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
label {
  display: grid;
  gap: 6px;
  min-width: 0;
}
input,
select,
textarea {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}
textarea {
  font-family: Consolas, monospace;
  font-size: 12px;
  resize: vertical;
}
.variables {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  font-size: 12px;
}
.variables code {
  border-radius: 4px;
  background: #eef4f6;
  padding: 2px 5px;
}
</style>
