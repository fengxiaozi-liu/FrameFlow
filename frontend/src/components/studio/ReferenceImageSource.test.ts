import { mount, flushPromises } from "@vue/test-utils";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import ReferenceImageSource from "./ReferenceImageSource.vue";
import { bindingsApi } from "../../api/bindings";
import { api } from "../../api/client";
import type { Material, Model, Task } from "../../api/types";
vi.mock("../../api/bindings", () => ({
  bindingsApi: { referenceImage: vi.fn(), task: vi.fn() },
}));
vi.mock("../../api/client", () => ({
  api: {
    materials: vi.fn(),
    material: vi.fn(),
    mediaUrl: (s: string) => s,
    retryTask: vi.fn(),
    cancelTask: vi.fn(),
  },
}));
const entity = {
  id: "person",
  kind: "character" as const,
  name: "林夏",
  description: "短发，蓝外套",
  revision: 1,
  references: [],
};
const models = [
  {
    id: "im",
    enabled: true,
    supported: true,
    capabilities: ["image"],
    name: "生图模型",
    connection_id: "c",
    remote_id: "custom",
    source: "manual",
    default: false,
    availability: {
      image: {
        available: true,
        code: "configured",
        reason: "",
        binding: {
          image_modes: ["text_to_image"],
          protocol: "dashscope.image.v1",
          source: "manual",
          ratios: ["16:9", "1:1", "9:16"],
        },
      },
    },
  },
] as Model[];
const props = {
  projectId: "p",
  draftId: "d",
  entity,
  models,
  materials: [] as Material[],
};
describe("reference image sources", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.useFakeTimers();
  });
  afterEach(() => vi.useRealTimers());
  it("filters the library and leaves reference adoption explicit", async () => {
    const person = {
      id: "m",
      name: "人物图",
      kind: "character",
      url: "/media/m.png",
    } as Material;
    vi.mocked(api.materials).mockResolvedValue({
      materials: [person, { ...person, id: "scene", kind: "scene" }],
    });
    const w = mount(ReferenceImageSource, {
      props,
      global: { stubs: { RouterLink: true } },
    });
    await w.get("button").trigger("click");
    await flushPromises();
    expect(w.findAll(".reference-library-grid button")).toHaveLength(1);
    expect(w.emitted("add")).toBeUndefined();
    await w.get(".reference-library-grid button").trigger("click");
    expect(w.emitted("add")![0]).toEqual([person]);
    w.unmount();
  });
  it("replays lost submission with frozen input and ignores late results after unmount", async () => {
    const task = { id: "task", status: "running", progress: 10 } as Task;
    let resolve!: (value: Task) => void;
    vi.mocked(bindingsApi.referenceImage)
      .mockRejectedValueOnce(Error("连接中断"))
      .mockResolvedValueOnce(task);
    vi.mocked(bindingsApi.task).mockImplementation(
      () =>
        new Promise((r) => {
          resolve = r;
        }),
    );
    const w = mount(ReferenceImageSource, {
      props,
      global: { stubs: { RouterLink: true } },
    });
    await w.findAll("button")[1]!.trigger("click");
    await w.get("button.primary").trigger("click");
    await flushPromises();
    expect(w.text()).toContain("恢复提交");
    await w.setProps({ entity: { ...entity, description: "修改后的描述" } });
    await w.get("button.primary").trigger("click");
    await flushPromises();
    const calls = vi.mocked(bindingsApi.referenceImage).mock.calls;
    expect(calls).toHaveLength(2);
    expect(calls[0]![2]).toEqual(calls[1]![2]);
    expect(calls[1]![2].prompt).toBe("短发，蓝外套");
    w.unmount();
    resolve({ ...task, status: "succeeded" });
    await flushPromises();
    await vi.advanceTimersByTimeAsync(4000);
    expect(api.material).not.toHaveBeenCalled();
    expect(w.emitted("add")).toBeUndefined();
  });
  it("requires an enabled configured editing capability and submits ordered reference IDs", async () => {
    const a = {
      id: "a",
      name: "原场景",
      kind: "scene",
      url: "/media/a.png",
      media: { size_bytes: 10 },
    } as Material;
    const b = { ...a, id: "b", name: "光线参考", url: "/media/b.png" };
    vi.mocked(api.materials).mockResolvedValue({ materials: [a, b] });
    vi.mocked(bindingsApi.referenceImage).mockResolvedValue({
      id: "edit",
      status: "running",
      progress: 0,
    } as Task);
    vi.mocked(bindingsApi.task).mockResolvedValue({
      id: "edit",
      status: "running",
      progress: 0,
    } as Task);
    const w = mount(ReferenceImageSource, {
      props: { ...props, materials: [a, b] },
      global: { stubs: { RouterLink: true } },
    });
    await w.findAll("button")[1]!.trigger("click");
    const editButton = () =>
      w.findAll("button").find((x) => x.text() === "图生图")!;
    expect(editButton().attributes("disabled")).toBeDefined();
    expect(w.text()).toContain("图生图尚未配置");
    const configured = structuredClone(models);
    Object.assign(configured[0]!.availability!.image!.binding, {
      image_modes: ["text_to_image", "image_to_image"],
      min_references: 1,
      max_references: 3,
      max_image_bytes: 10485760,
    });
    await w.setProps({ models: configured });
    await editButton().trigger("click");
    await flushPromises();
    await w.get("button.primary").trigger("click");
    expect(bindingsApi.referenceImage).not.toHaveBeenCalled();
    expect(w.text()).toContain("请选择 1–3 张参考图片");
    await w
      .get('[aria-label="图生图参考图片"] button[aria-expanded]')
      .trigger("click");
    await w.get('[aria-label="添加编辑参考图 原场景"]').trigger("click");
    await w.get('[aria-label="添加编辑参考图 光线参考"]').trigger("click");
    await w.get('[aria-label="前移图 2"]').trigger("click");
    await w.get("button.primary").trigger("click");
    await flushPromises();
    expect(
      vi.mocked(bindingsApi.referenceImage).mock.calls[0]![2],
    ).toMatchObject({
      mode: "image_to_image",
      reference_material_ids: ["b", "a"],
    });
    expect(w.emitted("add")).toBeUndefined();
    w.unmount();
  });
  it("does not carry retained editing references into a text-to-image request", async () => {
    const configured = structuredClone(models);
    Object.assign(configured[0]!.availability!.image!.binding, {
      image_modes: ["text_to_image", "image_to_image"],
      min_references: 1,
      max_references: 3,
      max_image_bytes: 10485760,
    });
    vi.mocked(api.materials).mockResolvedValue({ materials: [] });
    vi.mocked(bindingsApi.referenceImage).mockResolvedValue({
      id: "text",
      status: "running",
    } as Task);
    vi.mocked(bindingsApi.task).mockResolvedValue({
      id: "text",
      status: "running",
    } as Task);
    const w = mount(ReferenceImageSource, {
      props: { ...props, models: configured },
      global: { stubs: { RouterLink: true } },
    });
    await w.findAll("button")[1]!.trigger("click");
    await w
      .findAll("button")
      .find((x) => x.text() === "图生图")!
      .trigger("click");
    await w
      .findAll("button")
      .find((x) => x.text() === "文生图")!
      .trigger("click");
    await w.get("button.primary").trigger("click");
    await flushPromises();
    expect(
      vi.mocked(bindingsApi.referenceImage).mock.calls[0]![2],
    ).toMatchObject({ mode: "text_to_image", reference_material_ids: [] });
    w.unmount();
  });
});
