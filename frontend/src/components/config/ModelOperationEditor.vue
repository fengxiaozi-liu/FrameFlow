<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import ModelOptionTags from "./ModelOptionTags.vue";
import DurationRangeInput from "./DurationRangeInput.vue";
import CustomInterfaceSettings from "./CustomInterfaceSettings.vue";
import ProtocolEndpointSettings from "./ProtocolEndpointSettings.vue";
import EntityEditorDrawer from "../studio/EntityEditorDrawer.vue";
import { modelConfigurationApi } from "../../api/modelConfiguration";
import type { Capability, Model, OperationBinding } from "../../api/types";
import type { components } from "../../api/schema";
const props = defineProps<{ model: Model; baseUrl: string }>();
const emit = defineEmits<{ close: []; saved: [Model] }>();
const labels: Record<Capability, string> = {
  story: "文本生成",
  image: "图片生成",
  video: "视频生成",
  speech: "语音合成",
};
const capabilityOptions: Capability[] = ["story", "image", "video", "speech"];
const selected = ref<Capability | "">(props.model.capabilities[0] || "");
const capabilities = ref<Capability[]>([...props.model.capabilities]);
const bindings = ref<Record<string, OperationBinding>>(
  JSON.parse(JSON.stringify(props.model.operations || {})),
);
const protocols = ref<components["schemas"]["ProtocolDescriptor"][]>([]);
const busy = ref(false),
  error = ref(""),
  loading = ref(true);
const current = computed(() => bindings.value[selected.value]);
const choices = computed(() =>
  protocols.value.filter((p) => p.operation === selected.value),
);
function chooseProtocol(event: Event) {
  if (!selected.value) return;
  const protocol = (event.target as HTMLSelectElement).value;
  if (!protocol) {
    delete bindings.value[selected.value];
    return;
  }
  if (!capabilities.value.includes(selected.value))
    capabilities.value.push(selected.value);
  bindings.value[selected.value] = {
    protocol,
    source: "manual",
    endpoint_mode: "default",
    ratios: [],
    image_sizes: {},
    voices: [],
  };
  if (protocol === "custom.json.v1") {
    const body =
      selected.value === "speech"
        ? {
            model: "{{model}}",
            text: "{{text}}",
            voice: "{{voice}}",
            language: "{{language}}",
          }
        : selected.value === "story"
          ? { model: "{{model}}", messages: "{{messages}}" }
          : { model: "{{model}}", prompt: "{{prompt}}", images: "{{images}}" };
    bindings.value[selected.value]!.custom = {
      path: "",
      body_template: JSON.stringify(body, null, 2),
      auth_mode: "bearer",
      mode: "sync",
      result_pointer: "",
    };
  }
}
function chooseMode(event: Event) {
  if (!current.value) return;
  current.value.input_mode = (event.target as HTMLSelectElement)
    .value as OperationBinding["input_mode"];
  if (current.value.input_mode === "first_frame")
    current.value.max_references = 1;
  else {
    current.value.last_frame = false;
    current.value.driving_audio = false;
  }
}
function setSize(ratio: string, event: Event) {
  if (current.value)
    current.value.image_sizes = {
      ...current.value.image_sizes,
      [ratio]: (event.target as HTMLInputElement).value.trim(),
    };
}
function setImageMB(event: Event) {
  if (current.value)
    current.value.max_image_bytes =
      Number((event.target as HTMLInputElement).value) * 1024 * 1024;
}
async function save() {
  busy.value = true;
  error.value = "";
  try {
    const next = JSON.parse(JSON.stringify(bindings.value)) as Record<
      string,
      OperationBinding
    >;
    emit(
      "saved",
      await modelConfigurationApi.configure(
        props.model,
        next,
        capabilities.value,
      ),
    );
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}
onMounted(async () => {
  try {
    protocols.value = (await modelConfigurationApi.protocols()).protocols;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <EntityEditorDrawer
    :title="`${model.name} · 协议与参数`"
    :busy="busy"
    @close="emit('close')"
  >
    <form class="model-operation-form" @submit.prevent="save">
      <p class="muted">
        模型 ID：{{
          model.remote_id
        }}。按服务商接口文档配置；配置完成不代表已通过实际生成验证。
      </p>
      <p v-if="!model.capabilities.length">
        同步结果未声明模型能力，请在下面选择并配置，保存时会一并添加该能力。
      </p>
      <p v-if="!model.enabled" class="muted">
        此模型尚未启用，保存后请在模型列表勾选“启用”。
      </p>
      <label
        >配置能力<select
          aria-label="配置能力"
          v-model="selected"
          :disabled="busy || loading"
        >
          <option disabled value="">请选择需要配置的能力</option>
          <option v-for="cap in capabilityOptions" :key="cap" :value="cap">
            {{ labels[cap] }}
          </option>
        </select></label
      >
      <label
        >接口协议<select
          aria-label="接口协议"
          :value="current?.protocol || ''"
          :disabled="busy || loading || !selected"
          @change="chooseProtocol"
        >
          <option value="">暂不配置</option>
          <option
            v-for="protocol in choices"
            :key="protocol.id"
            :value="protocol.id"
          >
            {{ protocol.name }}
          </option>
        </select></label
      >
      <p v-if="selected && !loading" class="muted">
        内置协议提供固定请求格式与路径；其他 JSON
        接口请选择“自定义接口（JSON）”，按文档配置请求数据和结果提取。
      </p>
      <p v-if="selected && !loading && !choices.length" role="status">
        暂无可用协议，请重新加载或接入对应协议适配器。
      </p>
      <template v-if="current">
        <CustomInterfaceSettings
          v-if="current.protocol === 'custom.json.v1' && current.custom"
          v-model="current.custom"
          :capability="selected as Capability"
          :variables="
            choices.find((p) => p.id === current.protocol)
              ?.template_variables || []
          "
          :base-url="baseUrl"
          :disabled="busy"
        />
        <ProtocolEndpointSettings
          v-else
          :base-url="baseUrl"
          :key="selected + current.protocol"
          v-model="bindings[selected]!"
          :protocol="
            choices.find((protocol) => protocol.id === current.protocol)
          "
          :disabled="busy"
        />
        <fieldset
          v-if="selected === 'image' || selected === 'video'"
          :disabled="busy"
        >
          <legend>支持的画面比例</legend>
          <div class="operation-ratios">
            <label v-for="ratio in ['16:9', '9:16', '1:1']" :key="ratio"
              ><input
                v-model="current.ratios"
                type="checkbox"
                :value="ratio"
              />{{ ratio }}</label
            >
          </div>
        </fieldset>
        <template v-if="selected === 'image'">
          <label v-for="ratio in current.ratios" :key="ratio"
            >{{ ratio }} 输出尺寸<input
              :value="current.image_sizes?.[ratio]"
              :disabled="busy"
              placeholder="宽*高，例如 1664*928"
              @input="setSize(ratio, $event)"
          /></label>
          <label
            >最多参考图片数<input
              v-model.number="current.max_references"
              type="number"
              min="0"
              max="100"
              placeholder="纯文生图填 0"
              :disabled="busy"
          /></label>
        </template>
        <template v-if="selected === 'video'">
          <label
            >图片输入方式<select
              aria-label="图片输入方式"
              :value="current.input_mode || ''"
              :disabled="busy"
              @change="chooseMode"
            >
              <option value="">请选择</option>
              <option value="first_frame">单图首帧</option>
              <option value="reference_images">多图参考</option>
            </select></label
          >
          <div class="operation-grid">
            <DurationRangeInput
              :key="current.protocol + '-durations'"
              :model-value="current.durations || []"
              :disabled="busy"
              @update:model-value="current.durations = $event"
            />
            <ModelOptionTags
              :key="current.protocol + '-resolutions'"
              label="可选清晰度"
              kind="resolution"
              :model-value="current.resolutions || []"
              :disabled="busy"
              @update:model-value="current.resolutions = $event"
            />
            <label
              >最多参考图片数<input
                v-model.number="current.max_references"
                type="number"
                min="1"
                max="100"
                :disabled="busy || current.input_mode === 'first_frame'"
            /></label>
            <label
              >单张图片上限（MB）<input
                :value="
                  current.max_image_bytes
                    ? current.max_image_bytes / 1048576
                    : ''
                "
                type="number"
                min="0.1"
                step="0.1"
                :disabled="busy"
                placeholder="按文档填写"
                @input="setImageMB"
            /></label>
          </div>
          <p v-if="current.input_mode === 'first_frame'" class="muted">
            生成时使用分镜中唯一的引用图片作为首帧；多张引用时会要求你调整。
          </p>
          <details v-if="current.input_mode === 'first_frame'">
            <summary>首尾帧与驱动音频（可选）</summary>
            <label class="operation-check"
              ><input
                v-model="current.last_frame"
                type="checkbox"
                :disabled="busy"
              />允许尾帧</label
            >
            <label
              v-if="current.protocol !== 'custom.json.v1'"
              class="operation-check"
              ><input
                v-model="current.driving_audio"
                type="checkbox"
                :disabled="busy"
              />允许驱动音频</label
            >
            <template v-if="current.driving_audio">
              <label
                >音频文件上限（字节）<input
                  v-model.number="current.max_audio_bytes"
                  type="number"
                  min="1"
                  :disabled="busy"
              /></label>
              <label
                >音频最短时长（秒）<input
                  v-model.number="current.min_audio_duration"
                  type="number"
                  min="1"
                  :disabled="busy"
              /></label>
              <label
                >音频最长时长（秒）<input
                  v-model.number="current.max_audio_duration"
                  type="number"
                  min="1"
                  :disabled="busy"
              /></label>
            </template>
          </details>
        </template>
        <template v-if="selected === 'speech'">
          <label
            >文字长度上限<input
              v-model.number="current.max_text_length"
              type="number"
              min="1"
              :disabled="busy"
          /></label>
          <fieldset
            v-for="(voice, index) in current.voices"
            :key="index"
            :disabled="busy"
          >
            <legend>音色 {{ index + 1 }}</legend>
            <label>音色 ID<input v-model.trim="voice.id" /></label
            ><label>显示名称<input v-model.trim="voice.name" /></label>
            <ModelOptionTags
              label="音色语言"
              kind="language"
              :model-value="voice.languages"
              :disabled="busy"
              @update:model-value="voice.languages = $event"
            />
            <button type="button" @click="current.voices?.splice(index, 1)">
              移除此音色
            </button>
          </fieldset>
          <button
            type="button"
            :disabled="busy"
            @click="
              (current.voices ||= []).push({ id: '', name: '', languages: [] })
            "
          >
            添加音色
          </button>
        </template>
      </template>
      <p v-if="error" role="alert" class="error">{{ error }}</p>
      <footer class="operation-footer">
        <button type="button" :disabled="busy" @click="emit('close')">
          取消</button
        ><button
          class="primary"
          type="submit"
          :disabled="busy || loading || !capabilities.length"
        >
          {{ busy ? "保存中…" : "保存配置" }}
        </button>
      </footer>
    </form>
  </EntityEditorDrawer>
</template>

<style scoped>
.model-operation-form {
  display: grid;
  gap: 16px;
  padding: 20px 24px;
}
.model-operation-form label {
  display: grid;
  gap: 6px;
}
.model-operation-form fieldset {
  padding: 12px;
  border: 1px solid var(--border, #dbe3ee);
  border-radius: 8px;
}
.operation-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}
.operation-ratios {
  display: flex;
  gap: 20px;
}
.operation-ratios label,
.model-operation-form .operation-check {
  display: flex;
  align-items: center;
  gap: 6px;
}
.operation-ratios input,
.operation-check input {
  width: auto;
}
.operation-footer {
  position: sticky;
  bottom: 0;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 0;
  background: white;
}
@media (max-width: 480px) {
  .operation-grid {
    grid-template-columns: 1fr;
  }
}
</style>
