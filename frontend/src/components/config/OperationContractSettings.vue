<script setup lang="ts">
import { computed } from "vue";
import JSONContractFields from "./JSONContractFields.vue";
import ModelOptionTags from "./ModelOptionTags.vue";
import RequestAddressPreview from "./RequestAddressPreview.vue";
import type { Capability, OperationBinding } from "../../api/types";
import type { components } from "../../api/schema";

type Contract = components["schemas"]["OperationContract"];
const props = defineProps<{
  modelValue: OperationBinding;
  capability: Capability;
  variables: string[];
  baseUrl: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [OperationBinding] }>();
const labels: Record<string, string> = {
  model: "模型 ID",
  prompt: "提示词",
  system_prompt: "系统提示词",
  messages: "消息列表",
  images: "参考图片（多图）",
  first_frame: "首帧图片",
  last_frame: "尾帧图片",
  input_mode: "输入方式",
  duration: "视频时长",
  resolution: "清晰度",
  ratio: "画面比例",
  size: "图片尺寸",
  text: "合成文字",
  voice: "音色 ID",
  language: "语言",
  instructions: "语音指令",
};
const contract = computed(() => props.modelValue.contract!);
const requestFields = computed(() =>
  props.variables.map((id) => ({
    id,
    label:
      id === "images" && props.capability === "video"
        ? "智能参考图片（多图）"
        : labels[id] || id,
    required: id === "model",
  })),
);
const responseFields = computed(() =>
  contract.value.execution === "async_polling"
    ? [{ id: "task_id", label: "远端任务 ID", required: true }]
    : [
        {
          id: "result",
          label:
            props.capability === "story"
              ? "文本结果 / 文本片段"
              : "媒体结果 URL",
          required: true,
        },
      ],
);
function updateContract(next: Contract) {
  emit("update:modelValue", { ...props.modelValue, contract: next });
}
function mode(event: Event) {
  const execution = (event.target as HTMLSelectElement)
    .value as Contract["execution"];
  const next = { ...props.modelValue };
  next.custom = { ...next.custom! };
  if (execution === "async_polling") {
    next.custom.polling ||= {
      path: "",
      task_id_pointer: "",
      status_pointer: "",
      pending: ["queued", "running"],
      succeeded: ["succeeded"],
      failed: ["failed", "cancelled"],
    };
  }
  const previousAsync = contract.value.execution === "async_polling";
  const nextAsync = execution === "async_polling";
  const response =
    previousAsync === nextAsync
      ? contract.value.response
      : nextAsync
        ? { example: '{"id":"example-task"}', bindings: { task_id: "/id" } }
        : {
            example:
              props.capability === "story"
                ? '{"result":"示例文字"}'
                : '{"url":"https://cdn.example.com/result.mp4"}',
            bindings: {
              result: props.capability === "story" ? "/result" : "/url",
            },
          };
  next.contract = {
    ...contract.value,
    execution,
    response,
    polling_response: nextAsync
      ? contract.value.polling_response || {
          example:
            '{\n  "status": "succeeded",\n  "output": { "url": "https://cdn.example.com/video.mp4" }\n}',
          bindings: { status: "/status", result: "/output/url" },
        }
      : undefined,
  };
  emit("update:modelValue", next);
}
function path(event: Event) {
  emit("update:modelValue", {
    ...props.modelValue,
    custom: {
      ...props.modelValue.custom!,
      path: (event.target as HTMLInputElement).value,
    },
  });
}
function polling(key: string, value: string | string[]) {
  emit("update:modelValue", {
    ...props.modelValue,
    custom: {
      ...props.modelValue.custom!,
      polling: { ...props.modelValue.custom!.polling!, [key]: value },
    },
  });
}
</script>

<template>
  <section class="operation-contract-settings">
    <p class="muted">
      粘贴服务商文档中的 JSON 示例，系统会解析出字段列表。 按需将字段绑定到模型
      ID、提示词等参数，也可选择不绑定。不要粘贴密钥。
    </p>
    <label
      >请求路径<input
        :value="modelValue.custom?.path"
        required
        placeholder="/v1/generate"
        :disabled="disabled"
        @input="path"
    /></label>
    <label
      >执行方式<select
        aria-label="执行方式"
        :value="contract.execution"
        :disabled="disabled"
        @change="mode"
      >
        <option value="sync_json">同步 JSON</option>
        <option v-if="capability === 'video'" value="async_polling">
          异步任务 + 查询
        </option>
        <option value="sse_stream">SSE 流式响应</option>
        <option value="ndjson_stream">NDJSON 流式响应</option>
      </select></label
    >
    <JSONContractFields
      :model-value="contract.request"
      title="请求"
      direction="request"
      :fields="requestFields"
      :disabled="disabled"
      @update:model-value="updateContract({ ...contract, request: $event })"
    />
    <p v-if="capability === 'video'" class="muted">
      智能参考（多图）：将服务商的图片数组字段绑定为“智能参考图片（多图）”，
      生成时会传入本次选择的全部参考图片。示例需包含数组，如
      <code>"images": []</code>，也可放在嵌套对象中；请绑定整个数组。
      字段名称与层级以服务商文档为准，首帧图片单独绑定。
    </p>
    <JSONContractFields
      :model-value="contract.response"
      direction="response"
      :title="contract.execution === 'async_polling' ? '受理响应' : '响应'"
      :fields="responseFields"
      :disabled="disabled"
      @update:model-value="updateContract({ ...contract, response: $event })"
    />
    <template
      v-if="
        contract.execution === 'async_polling' &&
        modelValue.custom?.polling &&
        contract.polling_response
      "
    >
      <label
        >查询路径<input
          :value="modelValue.custom.polling.path"
          required
          placeholder="/v1/tasks/{task_id}"
          :disabled="disabled"
          @input="polling('path', ($event.target as HTMLInputElement).value)"
      /></label>
      <JSONContractFields
        :model-value="contract.polling_response"
        direction="response"
        title="查询响应"
        :fields="[
          { id: 'status', label: '状态字段', required: true },
          { id: 'result', label: '视频 URL', required: true },
        ]"
        :disabled="disabled"
        @update:model-value="
          updateContract({ ...contract, polling_response: $event })
        "
      />
      <ModelOptionTags
        v-for="state in ['pending', 'succeeded', 'failed'] as const"
        :key="state"
        kind="state"
        :model-value="modelValue.custom.polling[state]"
        :label="
          {
            pending: '等待状态值',
            succeeded: '成功状态值',
            failed: '失败状态值',
          }[state]
        "
        :disabled="disabled"
        @update:model-value="polling(state, $event)"
      />
    </template>
    <template
      v-if="
        contract.execution === 'sse_stream' ||
        contract.execution === 'ndjson_stream'
      "
    >
      <label
        >完成标记<input
          :value="contract.done_marker"
          placeholder="例如 [DONE]；留空表示正常 EOF 完成"
          :disabled="disabled"
          @input="
            updateContract({
              ...contract,
              done_marker: ($event.target as HTMLInputElement).value,
            })
          "
      /></label>
      <p class="muted">
        {{
          capability === "story"
            ? "文本片段将依次合并。"
            : "使用最后一个媒体 URL 作为结果。"
        }}设置完成标记时，断开连接且未收到标记会报错。
      </p>
    </template>
    <RequestAddressPreview
      :base-url="baseUrl"
      :path="modelValue.custom?.path || ''"
      :polling-path="
        contract.execution === 'async_polling'
          ? modelValue.custom?.polling?.path
          : undefined
      "
    />
  </section>
</template>

<style scoped>
.operation-contract-settings {
  display: grid;
  gap: 14px;
}
.operation-contract-settings > label {
  display: grid;
  gap: 6px;
}
</style>
