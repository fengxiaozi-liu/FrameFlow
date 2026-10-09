import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ModelOptionTags from "./ModelOptionTags.vue";

function editor(
  kind: "duration" | "resolution" | "language",
  values: string[] = [],
) {
  const wrapper = mount(ModelOptionTags, {
    props: {
      kind,
      label: "选项",
      modelValue: values,
      "onUpdate:modelValue": (value: string[]) =>
        wrapper.setProps({ modelValue: value }),
    },
  });
  return wrapper;
}
describe("model option tags", () => {
  it("sorts integer durations, rejects invalid input and deduplicates without mutating the original", async () => {
    const original = ["8"];
    const w = editor("duration", original);
    const input = w.get("input");
    for (const value of ["0", "4.5", "3601", "4,6"]) {
      await input.setValue(value);
      await input.trigger("keydown", { key: "Enter" });
      expect(w.props("modelValue")).toEqual(["8"]);
      expect((input.element as HTMLInputElement).validity.customError).toBe(
        true,
      );
    }
    await input.setValue("04");
    await input.trigger("keydown", { key: "Enter" });
    expect(w.props("modelValue")).toEqual(["4", "8"]);
    await input.setValue("4");
    await input.trigger("keydown", { key: "Enter" });
    expect(w.props("modelValue")).toEqual(["4", "8"]);
    expect(w.get('[role="status"]').text()).toContain("已添加");
    expect((input.element as HTMLInputElement).validity.customError).toBe(
      false,
    );
    await w.get('button[aria-label="移除4 秒"]').trigger("click");
    expect(w.props("modelValue")).toEqual(["8"]);
    expect(original).toEqual(["8"]);
  });
  it("starts with no preselected resolution, supports toggles and custom values, and keeps IME input pending", async () => {
    const w = editor("resolution");
    expect(w.findAll('[aria-pressed="true"]')).toHaveLength(0);
    const button = w.findAll("button").find((b) => b.text() === "720P")!;
    await button.trigger("click");
    expect(w.props("modelValue")).toEqual(["720P"]);
    await w.get("input").setValue("720p");
    await w.get("input").trigger("blur");
    expect(w.props("modelValue")).toEqual(["720P"]);
    await button.trigger("click");
    expect(w.props("modelValue")).toEqual([]);
    await w.get("input").setValue("1440p");
    await w
      .get("input")
      .trigger("keydown", { key: "Enter", isComposing: true });
    expect(w.props("modelValue")).toEqual([]);
    await w.get("input").trigger("blur");
    expect(w.props("modelValue")).toEqual(["1440P"]);
    await w.setProps({ disabled: true });
    await w.get('button[aria-label="移除1440P"]').trigger("click");
    expect(w.props("modelValue")).toEqual(["1440P"]);
  });
  it("preserves language identifiers as individual values", async () => {
    const w = editor("language", ["Chinese"]);
    await w.get("input").setValue("English");
    await w.get("input").trigger("blur");
    expect(w.props("modelValue")).toEqual(["Chinese", "English"]);
    await w.get("input").setValue("Chinese,English");
    await w.get("input").trigger("blur");
    expect(w.props("modelValue")).toEqual(["Chinese", "English"]);
    expect(w.get('[role="status"]').text()).toBe("请一次添加一个值");
  });
});
