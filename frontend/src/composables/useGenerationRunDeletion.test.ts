import { mount, flushPromises } from "@vue/test-utils";
import { defineComponent } from "vue";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { bindingsApi, type GenerationRun } from "../api/bindings";
import { useGenerationRuns } from "./useGenerationRuns";
import { useTaskStore } from "../stores/tasks";
vi.mock("../api/bindings", () => ({
  bindingsApi: { runs: vi.fn(), getRun: vi.fn(), deleteRun: vi.fn() },
}));
vi.mock("../stores/tasks", () => ({
  useTaskStore: () => ({
    subscribe: vi.fn().mockReturnValue(() => {}),
    load: load,
  }),
}));
const load = vi.hoisted(() => vi.fn().mockResolvedValue(undefined));
const failed: GenerationRun = {
  id: "r",
  scene_id: "s",
  state: "failed",
  task_ids: ["r-0"],
  snapshot_id: "snap",
};
const other: GenerationRun = { ...failed, id: "other", state: "completed" };
function setup() {
  let state!: ReturnType<typeof useGenerationRuns>;
  const wrapper = mount(
    defineComponent({
      setup() {
        state = useGenerationRuns({ project: "p", draft: "d", scene: "s" });
        return () => null;
      },
    }),
  );
  return { state, wrapper };
}
describe("failed generation history deletion", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    sessionStorage.clear();
    vi.mocked(bindingsApi.runs).mockResolvedValue({ runs: [failed, other] });
  });
  afterEach(() => vi.useRealTimers());
  it("retains the record on server rejection and supports retry", async () => {
    const { state, wrapper } = setup();
    await flushPromises();
    vi.mocked(bindingsApi.deleteRun).mockRejectedValueOnce(
      new Error("仍有执行中的步骤"),
    );
    expect(await state.remove(failed)).toBe(false);
    expect(state.runs.value.map((r) => r.id)).toEqual(["r", "other"]);
    expect(state.error.value).toContain("执行中");
    expect(state.deleting.value).toBe("");
    vi.mocked(bindingsApi.deleteRun).mockResolvedValueOnce(undefined);
    expect(await state.remove(failed)).toBe(true);
    expect(state.runs.value.map((r) => r.id)).toEqual(["other"]);
    expect(useTaskStore().load).toHaveBeenCalledTimes(1);
    wrapper.unmount();
  });
  it("deduplicates deletion and ignores stale poll/list responses after success", async () => {
    const { state, wrapper } = setup();
    await flushPromises();
    let resolvePoll!: (v: GenerationRun) => void;
    vi.mocked(bindingsApi.getRun).mockReturnValueOnce(
      new Promise((r) => (resolvePoll = r)),
    );
    await vi.advanceTimersByTimeAsync(4000);
    let resolveDelete!: () => void;
    vi.mocked(bindingsApi.deleteRun).mockReturnValueOnce(
      new Promise((r) => (resolveDelete = r)),
    );
    const deleting = state.remove(failed);
    expect(state.deleting.value).toBe("r");
    expect(await state.remove(failed)).toBe(false);
    expect(bindingsApi.deleteRun).toHaveBeenCalledTimes(1);
    resolveDelete();
    await deleting;
    resolvePoll(failed);
    await flushPromises();
    await state.refresh();
    expect(state.runs.value.map((r) => r.id)).toEqual(["other"]);
    expect(state.error.value).toBe("");
    wrapper.unmount();
  });
  it("does not offer deletion for other states and removes history deleted in another window", async () => {
    const { state, wrapper } = setup();
    await flushPromises();
    expect(await state.remove(other)).toBe(false);
    expect(bindingsApi.deleteRun).not.toHaveBeenCalled();
    vi.mocked(bindingsApi.getRun).mockRejectedValueOnce(
      Object.assign(new Error("已不存在"), {
        status: 400,
        payload: { error: { code: "run_not_found" } },
      }),
    );
    await vi.advanceTimersByTimeAsync(4000);
    await flushPromises();
    expect(state.runs.value.map((r) => r.id)).toEqual(["other"]);
    expect(state.error.value).toBe("");
    wrapper.unmount();
  });
});
