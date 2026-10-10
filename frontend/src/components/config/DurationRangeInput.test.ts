import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import DurationRangeInput from "./DurationRangeInput.vue";

function editor(values: number[] = []) {
  const wrapper = mount(DurationRangeInput, {
    props: {
      modelValue: values,
      "onUpdate:modelValue": (modelValue: number[]) =>
        wrapper.setProps({ modelValue }),
    },
  });
  return wrapper;
}
const min = '[aria-label="最短时长（秒）"]';
const max = '[aria-label="最长时长（秒）"]';

describe("duration range", () => {
  it("retains incomplete edits, includes both endpoints and restores saved ranges", async () => {
    const w = editor();
    await w.get(min).setValue("4");
    expect((w.get(min).element as HTMLInputElement).value).toBe("4");
    expect(w.props("modelValue")).toEqual([]);
    await w.get(max).setValue("8");
    expect(w.props("modelValue")).toEqual([4, 5, 6, 7, 8]);
    const reopened = editor(w.props("modelValue"));
    expect((reopened.get(min).element as HTMLInputElement).value).toBe("4");
    expect((reopened.get(max).element as HTMLInputElement).value).toBe("8");
    await w.get(max).setValue("4");
    expect(w.props("modelValue")).toEqual([4]);
  });
  it("rejects reversed, fractional, empty and out-of-bounds ranges", async () => {
    const w = editor([4, 5, 6]);
    await w.get(max).setValue("3");
    expect((w.get(max).element as HTMLInputElement).validity.customError).toBe(
      true,
    );
    expect(w.props("modelValue")).toEqual([]);
    for (const value of ["", "0", "4.5", "3601"]) {
      await w.get(max).setValue(value);
      expect((w.get(max).element as HTMLInputElement).checkValidity()).toBe(
        false,
      );
      expect(w.props("modelValue")).toEqual([]);
    }
    await w.get(min).setValue("1");
    await w.get(max).setValue("3600");
    expect(w.props("modelValue")).toHaveLength(3600);
    expect((w.get(max).element as HTMLInputElement).checkValidity()).toBe(true);
  });
  it("preserves discrete choices until a range is explicitly selected", async () => {
    const w = editor([4, 8]);
    expect(w.get("select").element.value).toBe("specific");
    expect(w.emitted("update:modelValue")).toBeUndefined();
    await w.get('[aria-label="添加指定秒数"]').setValue("12");
    await w
      .get('[aria-label="添加指定秒数"]')
      .trigger("keydown", { key: "Enter" });
    expect(w.props("modelValue")).toEqual([4, 8, 12]);
    await w.get("select").setValue("range");
    expect(w.props("modelValue")).toEqual([4, 5, 6, 7, 8, 9, 10, 11, 12]);
  });
});
