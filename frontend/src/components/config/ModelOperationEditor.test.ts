import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ModelOperationEditor from "./ModelOperationEditor.vue";
import { modelConfigurationApi } from "../../api/modelConfiguration";
import type { Model } from "../../api/types";

vi.mock("../../api/modelConfiguration", () => ({
  modelConfigurationApi: { protocols: vi.fn(), configure: vi.fn() },
}));
const model: Model = {
  id: "m",
  connection_id: "c",
  remote_id: "unknown",
  name: "待配置模型",
  capabilities: [],
  source: "discovered",
  enabled: false,
  supported: false,
  default: false,
  revision: 1,
};
const protocols = [
  {
    id: "openai.chat.v1",
    operation: "story",
    name: "OpenAI Chat Completions",
    default_path: "/compatible-mode/v1/chat/completions",
  },
  {
    id: "dashscope.image.v1",
    operation: "image",
    name: "DashScope 图片",
    default_path: "/api/v1/services/aigc/multimodal-generation/generation",
  },
  {
    id: "dashscope.media-video.v1",
    operation: "video",
    name: "DashScope 视频",
    default_path: "/api/v1/services/aigc/video-generation/video-synthesis",
    polling_path: "/api/v1/tasks/{task_id}",
  },
  {
    id: "dashscope.speech.v1",
    operation: "speech",
    name: "DashScope 语音",
    default_path: "/api/v1/services/aigc/multimodal-generation/generation",
  },
] as const;
async function editor() {
  const wrapper = mount(ModelOperationEditor, {
    props: { model, baseUrl: "https://example.com" },
    global: {
      stubs: { EntityEditorDrawer: { template: "<div><slot /></div>" } },
    },
  });
  await flushPromises();
  return wrapper;
}
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(modelConfigurationApi.protocols).mockResolvedValue({
    protocols: [...protocols],
  });
  vi.mocked(modelConfigurationApi.configure).mockResolvedValue(model);
});
describe("unclassified model configuration", () => {
  it("offers all capabilities and never defaults an empty selection to the text protocol", async () => {
    const wrapper = await editor();
    const capability = wrapper.get('select[aria-label="配置能力"]');
    const protocol = wrapper.get('select[aria-label="接口协议"]');
    expect(capability.findAll("option")).toHaveLength(5);
    expect(protocol.attributes("disabled")).toBeDefined();
    expect(protocol.findAll("option")).toHaveLength(1);
    for (const entry of protocols) {
      await capability.setValue(entry.operation);
      expect(
        protocol.findAll("option").map((option) => option.attributes("value")),
      ).toEqual(["", entry.id]);
    }
    await wrapper.get('button[type="button"]').trigger("click");
    expect(modelConfigurationApi.configure).not.toHaveBeenCalled();
    expect(model.capabilities).toEqual([]);
  });
  it("saves capability and binding together and retains input after a failed save", async () => {
    const wrapper = await editor();
    await wrapper.get('select[aria-label="配置能力"]').setValue("story");
    await wrapper
      .get('select[aria-label="接口协议"]')
      .setValue("openai.chat.v1");
    vi.mocked(modelConfigurationApi.configure).mockRejectedValueOnce(
      new Error("保存失败"),
    );
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toBe("保存失败");
    expect(
      (
        wrapper.get('select[aria-label="接口协议"]')
          .element as HTMLSelectElement
      ).value,
    ).toBe("openai.chat.v1");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(modelConfigurationApi.configure).toHaveBeenLastCalledWith(
      model,
      { story: expect.objectContaining({ protocol: "openai.chat.v1" }) },
      ["story"],
    );
    expect(wrapper.emitted("saved")).toHaveLength(1);
    expect(model.capabilities).toEqual([]);
  });
});
