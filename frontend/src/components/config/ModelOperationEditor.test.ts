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
  it("allows incomplete saved ratio configuration to be repaired", async () => {
    const wrapper = mount(ModelOperationEditor, {
      props: {
        model: {
          ...model,
          capabilities: ["video"],
          operations: {
            video: { protocol: "dashscope.media-video.v1", source: "manual" },
          },
        },
        baseUrl: "https://example.com",
      },
      global: {
        stubs: { EntityEditorDrawer: { template: "<div><slot /></div>" } },
      },
    });
    await flushPromises();
    await wrapper.get('[aria-label="16:9"]').setValue(true);
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(
      vi.mocked(modelConfigurationApi.configure).mock.calls[0]?.[1].video
        ?.ratios,
    ).toEqual(["16:9"]);
    wrapper.unmount();
  });
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
  it("saves image modes independently and can disable editing after applying an example", async () => {
    const w = await editor();
    await w.get('select[aria-label="配置能力"]').setValue("image");
    await w.get('select[aria-label="接口协议"]').setValue("dashscope.image.v1");
    expect(
      (w.get('input[value="text_to_image"]').element as HTMLInputElement)
        .checked,
    ).toBe(true);
    expect(
      (w.get('input[value="image_to_image"]').element as HTMLInputElement)
        .checked,
    ).toBe(false);
    await w
      .findAll("button")
      .find((b) => b.text() === "填入百炼示例参数")!
      .trigger("click");
    await w.get("form").trigger("submit");
    await flushPromises();
    expect(
      vi.mocked(modelConfigurationApi.configure).mock.calls[0]![1].image,
    ).toMatchObject({
      image_modes: ["text_to_image", "image_to_image"],
      min_references: 1,
      max_references: 3,
      max_image_bytes: 10485760,
    });
    await w.get('input[value="image_to_image"]').setValue(false);
    await w.get("form").trigger("submit");
    await flushPromises();
    expect(
      vi.mocked(modelConfigurationApi.configure).mock.calls[1]![1].image!
        .image_modes,
    ).toEqual(["text_to_image"]);
    w.unmount();
  });
});
