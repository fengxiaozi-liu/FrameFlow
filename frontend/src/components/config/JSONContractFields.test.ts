import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import type { components } from "../../api/schema";
import JSONContractFields from "./JSONContractFields.vue";
import { updateExampleValue } from "./jsonContractFields";

type Contract = components["schemas"]["JSONContract"];
function setup(
  contract: Contract,
  direction: "request" | "response" = "request",
) {
  const wrapper = mount(JSONContractFields, {
    attachTo: document.body,
    props: {
      title: direction === "request" ? "请求" : "响应",
      direction,
      modelValue: contract,
      fields:
        direction === "request"
          ? [
              { id: "model", label: "模型 ID", required: true },
              { id: "prompt", label: "提示词" },
              { id: "duration", label: "视频时长" },
              { id: "messages", label: "消息列表" },
            ]
          : [{ id: "result", label: "文本结果", required: true }],
      "onUpdate:modelValue": (value: Contract) => {
        void wrapper.setProps({ modelValue: value });
      },
    },
  });
  return wrapper;
}

describe("JSON field table", () => {
  it("nests local keys inside their parent groups and preserves edits when collapsed", async () => {
    const wrapper = setup({
      example:
        '{"model":"demo","list":{"a":{"c":"d"},"b":""},"items":[{"c":"second"}],"empty":{}}',
      bindings: { model: "/model" },
    });
    const list = wrapper.get('[data-field-pointer="/list"]');
    const a = list.get('[data-field-pointer="/list/a"]');
    const c = a.get('[data-field-pointer="/list/a/c"]');
    expect(c.get(".leaf-key").text()).toBe("c");
    expect(list.get('[data-field-pointer="/list/b"] .leaf-key').text()).toBe(
      "b",
    );
    expect(
      wrapper.get('[data-field-pointer="/items/0"] .branch-toggle code').text(),
    ).toBe("[0]");
    expect(wrapper.get('[data-field-pointer="/empty"]').text()).toContain(
      "空对象",
    );
    await c.get('[aria-label="请求 list.a.c 固定值"]').setValue("edited");
    await c.get('[aria-label="请求 list.a.c 绑定参数"]').setValue("prompt");
    await list.get('[aria-label="请求 list.a 收起"]').trigger("click");
    expect(c.isVisible()).toBe(false);
    expect(list.get('[data-field-pointer="/list/b"]').isVisible()).toBe(true);
    await list.get('[aria-label="请求 list.a 展开"]').trigger("click");
    expect(list.get('[data-field-pointer="/list/a/c"]').isVisible()).toBe(true);
    await list.get('[aria-label="请求 list 收起"]').trigger("click");
    expect(c.isVisible()).toBe(false);
    await list.get('[aria-label="请求 list 展开"]').trigger("click");
    expect(wrapper.props("modelValue").bindings.prompt).toBe("/list/a/c");
    expect(JSON.parse(wrapper.props("modelValue").example).list.a.c).toBe(
      "edited",
    );
    expect(JSON.parse(wrapper.props("modelValue").example).items[0].c).toBe(
      "second",
    );
    wrapper.unmount();
  });

  it("generates rows from the example, explicitly binds a field, and allows every binding to be cleared", async () => {
    const wrapper = setup({
      example: '{"model":"demo","input":{"prompt":"hello"},"temperature":0.5}',
      bindings: { model: "/model" },
    });
    const prompt = wrapper.get(
      'select[aria-label="请求 input.prompt 绑定参数"]',
    );
    expect((prompt.element as HTMLSelectElement).value).toBe("");
    expect(
      wrapper.find('select[aria-label="请求 duration 绑定参数"]').exists(),
    ).toBe(false);
    await prompt.setValue("prompt");
    expect(wrapper.props("modelValue").bindings).toEqual({
      model: "/model",
      prompt: "/input/prompt",
    });
    await prompt.setValue("");
    expect(
      wrapper.get('input[aria-label="请求 input.prompt 固定值"]').element,
    ).toHaveProperty("value", "hello");
    await wrapper.get('select[aria-label="请求 model 绑定参数"]').setValue("");
    expect(wrapper.props("modelValue").bindings).toEqual({});
    expect(wrapper.text()).toContain("保存前还需绑定：模型 ID");
    expect(wrapper.findAll("select[required]")).toHaveLength(0);
    wrapper.unmount();
  });

  it("edits fixed values with their original types, retaining other fields and bindings", async () => {
    const wrapper = setup({
      example:
        '{"model":"demo","temperature":0.5,"stream":false,"extra":{"note":"original"}}',
      bindings: { model: "/model" },
    });
    await wrapper.get('[aria-label="请求 temperature 固定值"]').setValue("0.8");
    await wrapper.get('[aria-label="请求 stream 固定值"]').setValue("true");
    await wrapper
      .get('[aria-label="请求 extra.note 固定值"]')
      .setValue("edited");
    expect(JSON.parse(wrapper.props("modelValue").example)).toEqual({
      model: "demo",
      temperature: 0.8,
      stream: true,
      extra: { note: "edited" },
    });
    expect(wrapper.props("modelValue").bindings).toEqual({ model: "/model" });
    const number = wrapper.get('[aria-label="请求 temperature 固定值"]');
    await number.setValue("");
    expect((number.element as HTMLInputElement).validity.valid).toBe(false);
    expect(JSON.parse(wrapper.props("modelValue").example).temperature).toBe(
      0.8,
    );
    await number.setValue("0");
    expect((number.element as HTMLInputElement).validity.valid).toBe(true);
    expect(JSON.parse(wrapper.props("modelValue").example).temperature).toBe(0);
    wrapper.unmount();
  });

  it("keeps invalid drafts and stale bindings until explicitly repaired", async () => {
    const wrapper = setup({
      example: '{"model":"demo","old":"hello"}',
      bindings: { model: "/model", prompt: "/old" },
    });
    await wrapper.get("textarea").setValue('{"model":');
    expect(wrapper.get('[role="alert"]').text()).toContain("已有绑定保留");
    expect(wrapper.props("modelValue").example).toBe('{"model":');
    expect(wrapper.props("modelValue").bindings.prompt).toBe("/old");
    await wrapper.get("textarea").setValue('{"model":"demo","input":"hello"}');
    expect(wrapper.text()).toContain("字段缺失或参数不再支持");
    await wrapper.get('[aria-label="请求 解除提示词绑定"]').trigger("click");
    await wrapper.get('[aria-label="请求 input 绑定参数"]').setValue("prompt");
    expect(wrapper.props("modelValue").bindings.prompt).toBe("/input");
    wrapper.unmount();
  });

  it("prevents duplicate variables, type conflicts and nested request overrides", async () => {
    const wrapper = setup({
      example:
        '{"model":"demo","messages":[{"content":"hello"}],"duration":5,"other":"text"}',
      bindings: { model: "/model" },
    });
    expect(
      wrapper
        .get('[aria-label="请求 duration 绑定参数"] option[value="prompt"]')
        .attributes(),
    ).toHaveProperty("disabled");
    expect(
      wrapper
        .get('[aria-label="请求 other 绑定参数"] option[value="model"]')
        .attributes(),
    ).toHaveProperty("disabled");
    await wrapper
      .get('[aria-label="请求 messages.0.content 绑定参数"]')
      .setValue("prompt");
    expect(
      wrapper
        .get('[aria-label="请求 messages 绑定参数"] option[value="messages"]')
        .attributes(),
    ).toHaveProperty("disabled");
    await wrapper
      .get('[aria-label="请求 messages.0.content 绑定参数"]')
      .setValue("");
    await wrapper
      .get('[aria-label="请求 messages 绑定参数"]')
      .setValue("messages");
    expect(
      wrapper
        .get('[aria-label="请求 messages.0.content 绑定参数"]')
        .attributes(),
    ).toHaveProperty("disabled");
    expect(
      wrapper.find('[aria-label="请求 messages.0.content 固定值"]').exists(),
    ).toBe(false);
    await wrapper.get("textarea").setValue('{"model":7,"messages":[]}');
    expect(wrapper.text()).toContain("需要 string 类型");
    await wrapper.get('[aria-label="请求 model 绑定参数"]').setValue("");
    expect(wrapper.props("modelValue").bindings).toEqual({
      messages: "/messages",
    });
    wrapper.unmount();
  });

  it("leaves response extras unbound and ignores them instead of treating them as constants", async () => {
    const wrapper = setup(
      {
        example: '{"answer":"demo","debug":"info"}',
        bindings: { result: "/answer" },
      },
      "response",
    );
    const select = wrapper.get('[aria-label="响应 debug 绑定参数"]');
    expect((select.element as HTMLSelectElement).value).toBe("");
    expect(select.get('option[value=""]').text()).toBe("不绑定 · 忽略此字段");
    expect(wrapper.find("input").exists()).toBe(false);
    await wrapper.get('[aria-label="响应 answer 绑定参数"]').setValue("");
    expect(wrapper.props("modelValue").bindings).toEqual({});
    expect(wrapper.text()).toContain("保存前还需绑定：文本结果");
    wrapper.unmount();
  });

  it("edits escaped literal keys and array values without traversing inherited properties", () => {
    const example =
      '{"a/b":{"~key":[{"a.b":"old"}]},"__proto__":{"note":"own"}}';
    const edited = updateExampleValue(example, "/a~1b/~0key/0/a.b", "new");
    expect(JSON.parse(edited)["a/b"]["~key"][0]["a.b"]).toBe("new");
    expect(
      JSON.parse(updateExampleValue(edited, "/__proto__/note", "safe"))
        .__proto__.note,
    ).toBe("safe");
    expect(() => updateExampleValue("{}", "/__proto__/note", "bad")).toThrow();
    expect({}).not.toHaveProperty("note");
  });
});
