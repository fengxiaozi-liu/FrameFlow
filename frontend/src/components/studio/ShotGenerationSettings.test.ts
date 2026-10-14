import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ShotGenerationSettings from "./ShotGenerationSettings.vue";
import type { Model, Material } from "../../api/types";
import type { ShotGenerationSettings as Settings } from "../../api/bindings";
const base: Model = {
  id: "m",
  connection_id: "c",
  remote_id: "arbitrary",
  name: "自定义视频",
  capabilities: ["video"],
  source: "manual",
  enabled: true,
  supported: true,
  default: false,
};
describe("model-driven video settings", () => {
  it("selects adaptive verbatim and requires re-selection when the model removes it", async () => {
    const binding = {
      protocol: "custom.json.v1",
      source: "manual",
      input_modes: ["first_frame"] as const,
      ratios: ["adaptive", "21:9"],
      durations: [5],
      resolutions: ["720P"],
      max_references: 1,
    };
    const model: Model = {
      ...base,
      availability: {
        video: {
          available: true,
          code: "configured",
          reason: "",
          binding: { ...binding, input_modes: ["first_frame"] },
        },
      },
    };
    const w = mount(ShotGenerationSettings, {
      props: {
        modelValue: {
          video_model_id: "m",
          video_input_mode: "first_frame" as const,
          first_frame_material_id: "a",
          aspect_ratio: "21:9",
          duration_seconds: 5,
          resolution: "720P",
        },
        models: [model],
        images: [
          {
            id: "a",
            kind: "scene" as const,
            name: "A",
            url: "/media/a.png",
            created_at: "",
          },
        ],
        "onUpdate:modelValue": (value: Settings) =>
          w.setProps({ modelValue: value }),
      },
      global: { stubs: { RouterLink: true } },
    });
    await w.get('[aria-label="画面比例"]').setValue("adaptive");
    expect(w.props("modelValue").aspect_ratio).toBe("adaptive");
    expect(w.text()).toContain("自适应（adaptive）");
    expect(w.emitted("ready")?.slice(-1)[0]).toEqual([true]);
    await w.setProps({
      models: [
        {
          ...model,
          availability: {
            video: {
              ...model.availability!.video!,
              binding: {
                ...binding,
                input_modes: ["first_frame"],
                ratios: ["21:9"],
              },
            },
          },
        },
      ],
    });
    expect(w.props("modelValue").aspect_ratio).toBe("adaptive");
    expect(w.emitted("ready")?.slice(-1)[0]).toEqual([false]);
    expect(w.text()).toContain("请重新选择");
    w.unmount();
  });
  it("explains missing configuration instead of hiding the model", () => {
    const w = mount(ShotGenerationSettings, {
      props: {
        modelValue: {
          video_model_id: "m",
          aspect_ratio: "16:9",
          duration_seconds: 6,
          resolution: "720P",
        },
        models: [
          {
            ...base,
            availability: {
              video: {
                available: false,
                code: "configuration_missing",
                reason: "请配置协议",
                binding: { protocol: "", source: "" },
              },
            },
          },
        ],
      },
      global: { stubs: { RouterLink: true } },
    });
    expect(w.text()).toContain("请配置协议");
    expect(w.findAll("select")[1]!.attributes()).toHaveProperty("disabled");
    expect(w.emitted("ready")?.[0]).toEqual([false]);
  });
  it("changes incompatible values to declared choices when a model is selected", async () => {
    const w = mount(ShotGenerationSettings, {
      props: {
        modelValue: {
          video_model_id: "",
          aspect_ratio: "16:9",
          duration_seconds: 6,
          resolution: "720P",
        },
        models: [
          {
            ...base,
            availability: {
              video: {
                available: true,
                code: "configured",
                reason: "",
                binding: {
                  protocol: "dashscope.media-video.v1",
                  source: "manual",
                  input_mode: "first_frame",
                  durations: [4, 8],
                  resolutions: ["1080P"],
                  ratios: ["9:16"],
                  max_references: 1,
                },
              },
            },
          },
        ],
      },
      global: { stubs: { RouterLink: true } },
    });
    await w.findAll("select")[0]!.setValue("m");
    expect(w.emitted("update:modelValue")?.[0]?.[0]).toMatchObject({
      video_model_id: "m",
      duration_seconds: 4,
      resolution: "1080P",
      aspect_ratio: "9:16",
    });
  });
});

it("requires an explicit mode, retains frame choices and blocks removed frames", async () => {
  const model: Model = {
    ...base,
    availability: {
      video: {
        available: true,
        code: "configured",
        reason: "",
        binding: {
          protocol: "dashscope.media-video.v1",
          source: "manual",
          input_modes: ["first_frame", "reference_images"],
          last_frame: true,
          max_references: 3,
          durations: [4],
          resolutions: ["720P"],
          ratios: ["16:9"],
        },
      },
    },
  };
  const images: Material[] = [
    {
      id: "a",
      kind: "scene",
      name: "A",
      url: "/media/a.png",
      created_at: "2026-10-10T00:00:00Z",
    },
    {
      id: "b",
      kind: "scene",
      name: "B",
      url: "/media/b.png",
      created_at: "2026-10-10T00:00:00Z",
    },
  ];
  const w = mount(ShotGenerationSettings, {
    props: {
      modelValue: {
        video_model_id: "m",
        aspect_ratio: "16:9",
        duration_seconds: 4,
        resolution: "720P",
      },
      models: [model],
      images,
      "onUpdate:modelValue": (value: Settings) =>
        w.setProps({ modelValue: value }),
    },
    global: { stubs: { RouterLink: true } },
  });
  expect(w.emitted("ready")?.slice(-1)[0]).toEqual([false]);
  const buttons = w.findAll(".video-mode-options button");
  await buttons[0]!.trigger("click");
  await w.get('[aria-label="首帧图片"]').setValue("b");
  await w.get('[aria-label="尾帧图片"]').setValue("a");
  expect(w.emitted("ready")?.slice(-1)[0]).toEqual([true]);
  await buttons[1]!.trigger("click");
  expect(w.find('[aria-label="首帧图片"]').exists()).toBe(false);
  await buttons[0]!.trigger("click");
  expect(
    (w.get('[aria-label="首帧图片"]').element as HTMLSelectElement).value,
  ).toBe("b");
  await w.setProps({ images: images.slice(1) });
  expect(w.emitted("ready")?.slice(-1)[0]).toEqual([false]);
  expect(w.text()).toContain("尾帧不可用");
  await w.get('[aria-label="尾帧图片"]').setValue("");
  expect(w.emitted("ready")?.slice(-1)[0]).toEqual([true]);
  await w.setProps({ images: [] });
  expect(w.emitted("ready")?.slice(-1)[0]).toEqual([false]);
  expect(w.text()).toContain("请选择当前分镜引用中的首帧图片");
});
