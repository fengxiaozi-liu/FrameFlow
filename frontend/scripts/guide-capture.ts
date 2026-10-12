// Regenerate with: npx playwright test --config scripts/guide-capture.config.ts
// Real UI, synthetic fixtures. All API requests are intercepted; no paid calls or user data.
import { test, expect, type Locator } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import { pasteContractJSON } from "../e2e/json-contract-helpers";

test("capture annotated guide images from the application", async ({
  page,
}) => {
  await mkdir("public/guide", { recursive: true });
  await page.setViewportSize({ width: 1280, height: 960 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  const story =
    "雨夜，林夏撑伞走过安静的街道。咖啡馆的暖光映在积水里，她推开玻璃门，看见窗边等候的老朋友。两人相视一笑，在雨声中开始交谈。";
  const videoBinding = {
    protocol: "dashscope.media-video.v1",
    source: "manual",
    endpoint_mode: "default",
    input_modes: ["first_frame", "reference_images"],
    last_frame: true,
    ratios: ["16:9", "9:16"],
    durations: [4, 5, 6],
    duration_min: 4,
    duration_max: 6,
    resolutions: ["720P", "1080P"],
    max_references: 5,
    max_image_bytes: 10485760,
  };
  const imageBinding = {
    protocol: "dashscope.image.v1",
    source: "manual",
    image_modes: ["text_to_image", "image_to_image"],
    min_references: 1,
    max_references: 3,
    max_image_bytes: 10485760,
    ratios: ["16:9", "1:1"],
    image_sizes: { "16:9": "1280*720", "1:1": "1024*1024" },
  };
  const models = [
    ["qwen-plus", "故事模型", "story"],
    ["qwen-image-2.0", "图片模型", "image"],
    ["wan2.7-r2v", "视频模型", "video"],
  ].map(([id, name, cap]) => ({
    id,
    remote_id: id,
    name,
    connection_id: "demo",
    capabilities: [cap],
    enabled: true,
    supported: true,
    default: true,
    operations: {
      [cap]:
        cap === "video"
          ? videoBinding
          : cap === "image"
            ? imageBinding
            : { protocol: "openai.chat.v1" },
    },
    availability: {
      [cap]: {
        available: true,
        binding:
          cap === "video" ? videoBinding : cap === "image" ? imageBinding : {},
      },
    },
  }));
  const scene = {
    id: "shot",
    order: 1,
    title: "雨夜 · 走进咖啡馆",
    visual_prompt:
      "中景，镜头缓慢推进。林夏推开咖啡馆的玻璃门，暖光照亮她的侧脸。",
    narration: "",
    duration_seconds: 4,
    entity_bindings: [
      {
        id: "binding",
        entity_id: "cafe",
        entity_revision: 1,
        role: "location",
      },
    ],
    generation_settings: {
      video_input_mode: "first_frame",
      first_frame_material_id: "photo",
      last_frame_material_id: "photo-end",
      video_model_id: "wan2.7-r2v",
      aspect_ratio: "16:9",
      resolution: "720P",
      duration_seconds: 4,
    },
  };
  const draft = {
    id: "draft",
    name: "雨夜咖啡馆",
    version: 1,
    story: { body: story, summary: "", scenes: [scene] },
    outputs: [],
    candidates: [] as object[],
    video_versions: [
      {
        id: "version",
        task_id: "video-task",
        scene_id: "shot",
        local_media_path: "/guide/demo.mp4",
        duration_seconds: 4,
        created_at: "2026-10-09T08:00:00Z",
      },
    ],
    selected_versions: {} as Record<string, string>,
    compositions: [] as object[],
  };
  const connection = {
    id: "demo",
    name: "创作模型 · 示例连接",
    vendor: "bailian",
    base_url: "https://example.com",
    credential_set: true,
  };
  let showProgress = false;
  await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
  await page.route("**/api/**", (route) => {
    const url = new URL(route.request().url()),
      path = url.pathname;
    if (!path.startsWith("/api/")) return route.continue();
    if (route.request().method() !== "GET")
      throw new Error(`Unexpected write during capture: ${path}`);
    let json: unknown = {};
    if (path === "/api/connections") json = { connections: [connection] };
    else if (path === "/api/model-protocols")
      json = {
        protocols: [
          {
            id: "dashscope.image.v1",
            name: "DashScope 图片生成",
            operation: "image",
            default_path:
              "/api/v1/services/aigc/multimodal-generation/generation",
          },
          {
            id: "custom.json.v1",
            name: "自定义接口（JSON）",
            operation: "video",
            template_variables: [
              "model",
              "prompt",
              "images",
              "first_frame",
              "last_frame",
              "duration",
              "resolution",
              "ratio",
            ],
          },
          {
            id: "dashscope.media-video.v1",
            name: "DashScope 视频生成",
            operation: "video",
            default_path:
              "/api/v1/services/aigc/video-generation/video-synthesis",
            polling_path: "/api/v1/tasks/{task_id}",
          },
        ],
      };
    else if (path.endsWith("/input-capability"))
      json = {
        reference_video: true,
        reference_images: 5,
        durations: [4, 6, 10],
        resolutions: ["720P", "1080P"],
      };
    else if (path === "/api/models")
      json = {
        models: url.searchParams.has("capability")
          ? models.filter((m) =>
              m.capabilities.includes(url.searchParams.get("capability")!),
            )
          : models,
      };
    else if (path.endsWith("/entities"))
      json = {
        catalog_version: 1,
        entities: [
          {
            id: "cafe",
            kind: "location",
            name: "雨夜咖啡馆",
            description: "雨夜街角的咖啡馆，暖黄色灯光，大面积玻璃窗。",
            revision: 1,
            references: [
              { material_id: "photo", order: 0 },
              { material_id: "photo-end", order: 1 },
            ],
          },
        ],
        voice_profiles: [],
      };
    else if (path === "/api/materials")
      json = {
        materials: [
          {
            id: "photo-end",
            kind: "scene",
            name: "结束画面参考图",
            url: "/home/city.png",
            tags: [],
          },
          {
            id: "photo",
            kind: "scene",
            name: "咖啡馆参考图",
            url: "/home/cafe.png",
            tags: [],
          },
        ],
      };
    else if (path === "/api/projects/demo")
      json = { id: "demo", name: "第一支短片 · 演示项目", drafts: [draft] };
    else if (path === "/api/projects/demo/drafts/draft") json = draft;
    else if (path.endsWith("/generation-runs/run"))
      json = {
        id: "run",
        state: "awaiting_video_selection",
        created_at: "2026-10-09T08:00:00Z",
        task_ids: ["video-task"],
        plan: [
          {
            path: "reference_video",
            aspect_ratio: "16:9",
            prompt: scene.visual_prompt,
            entities: [],
          },
        ],
        video_model_id: "wan2.7-r2v",
        resolution: "720P",
        duration_seconds: 4,
        steps: [
          {
            id: "video-task",
            kind: "video",
            status: "succeeded",
            progress: 100,
            result_url: "/guide/demo.mp4",
          },
        ],
      };
    else if (path.endsWith("/generation-runs"))
      json = {
        runs: showProgress
          ? [
              {
                id: "run",
                scene_id: "shot",
                state: "generating_video",
                created_at: "2026-10-10T08:00:00Z",
                plan: [{ path: "reference_video" }],
                steps: [
                  {
                    id: "video-task",
                    kind: "video",
                    status: "running",
                    progress: 45,
                  },
                ],
              },
            ]
          : [],
      };
    else if (path === "/api/tasks") json = { tasks: [], total: 0 };
    else if (path.endsWith("/video-versions"))
      json = { versions: draft.video_versions };
    else if (path === "/api/projects") json = { projects: [] };
    else if (route.request().method() !== "GET")
      throw new Error(`Unexpected write during capture: ${path}`);
    return route.fulfill({ json });
  });
  async function capture(name: string, frame: Locator, marks: Locator[]) {
    const only = process.env.GUIDE_CAPTURE_ONLY?.split(",");
    if (only && !only.includes(name)) return;
    await frame.scrollIntoViewIfNeeded();
    for (const [i, mark] of marks.entries()) {
      await expect(mark).toBeVisible();
      await mark.evaluate((el, n) => {
        el.setAttribute("data-guide-mark", String(n));
        (el as HTMLElement).style.outline = "3px solid #d85b26";
        (el as HTMLElement).style.outlineOffset = "3px";
      }, i + 1);
    }
    // Number labels sit outside controls, so original labels remain readable.
    const frameBox = await frame.boundingBox();
    await page.evaluate((frameLeft) => {
      for (const el of document.querySelectorAll("[data-guide-mark]")) {
        const r = el.getBoundingClientRect(),
          badge = document.createElement("span"),
          modal = el.closest("dialog"),
          parent = modal || document.body,
          origin = modal?.getBoundingClientRect();
        badge.className = "capture-badge";
        badge.textContent = el.getAttribute("data-guide-mark");
        Object.assign(badge.style, {
          position: "absolute",
          left: `${Math.max(r.left - 10, frameLeft + 3) - (origin?.left || 0) + (modal?.scrollLeft || scrollX)}px`,
          top: `${r.top - (origin?.top || 0) + (modal?.scrollTop || scrollY) - 12}px`,
          background: "#a93613",
          color: "white",
          border: "2px solid white",
          borderRadius: "50%",
          width: "24px",
          height: "24px",
          display: "grid",
          placeItems: "center",
          font: "bold 13px sans-serif",
          zIndex: "99999",
          pointerEvents: "none",
        });
        parent.append(badge);
      }
    }, frameBox?.x || 0);
    await frame.screenshot({
      path: `public/guide/${name}.png`,
      animations: "disabled",
    });
    await page.evaluate(() => {
      document.querySelectorAll(".capture-badge").forEach((e) => e.remove());
      document
        .querySelectorAll<HTMLElement>("[data-guide-mark]")
        .forEach((e) => {
          e.style.outline = "";
          e.style.outlineOffset = "";
          e.removeAttribute("data-guide-mark");
        });
    });
  }
  await page.goto("/config");
  await page.getByRole("button", { name: "添加厂商连接", exact: true }).click();
  await page.getByLabel("名称", { exact: true }).fill("我的创作模型");
  await page
    .getByLabel("基础 URL", { exact: true })
    .fill("https://example.com");
  await page
    .getByLabel("API Key", { exact: true })
    .fill("demo-only-not-a-real-key");
  await capture("connection", page.locator(".catalog-settings"), [
    page.getByLabel("基础 URL", { exact: true }),
    page.getByLabel("API Key", { exact: true }),
    page.getByRole("button", { name: "保存连接", exact: true }),
  ]);
  if (process.env.GUIDE_CONNECTION_ONLY === "1") return;
  await page.getByLabel("厂商", { exact: true }).selectOption("custom");
  await page
    .getByLabel("响应 JSON 示例", { exact: true })
    .fill('{\n  "data": [{ "id": "demo-model", "name": "演示模型" }]\n}');
  await page.getByLabel("显示名称 Key（可选）", { exact: true }).fill("name");
  await capture("custom-discovery", page.locator(".discovery-profile"), [
    page.getByLabel("同步路径", { exact: true }),
    page.getByLabel("响应 JSON 示例", { exact: true }),
    page.getByLabel("模型列表 Key", { exact: true }),
  ]);
  await page.goto("/config");
  await expect(page.locator(".catalog-main")).toContainText("故事模型");
  await capture("models", page.locator(".catalog-main"), [
    page.getByRole("button", { name: "测试连接", exact: true }),
    page.getByRole("button", { name: "同步模型", exact: true }),
    page.locator(".catalog-main table").first(),
  ]);
  await page
    .getByRole("row")
    .filter({ hasText: "视频模型" })
    .getByRole("button", { name: "协议与参数" })
    .click();
  await expect(page.getByRole("dialog").getByLabel("接口协议")).toHaveValue(
    "dashscope.media-video.v1",
  );
  await capture("protocol", page.getByRole("dialog"), [
    page.getByLabel("配置能力"),
    page.getByLabel("接口协议"),
    page.getByRole("group", { name: "内置协议请求路径" }),
  ]);
  const drawer = page.getByRole("dialog");
  await drawer.getByLabel("接口协议").selectOption("custom.json.v1");
  await drawer.getByLabel("请求路径", { exact: true }).fill("/v1/videos");
  await pasteContractJSON(
    drawer,
    "请求",
    JSON.stringify(
      {
        model: "example-video-model",
        input: { prompt: "雨夜，镜头缓慢推进咖啡馆", images: [] },
        watermark: false,
      },
      null,
      2,
    ),
  );
  await drawer
    .getByLabel("请求 input.prompt 绑定参数", { exact: true })
    .selectOption("prompt");
  await drawer
    .getByLabel("请求 input.images 绑定参数", { exact: true })
    .selectOption("images");
  await capture(
    "custom-request",
    drawer.getByRole("group", { name: "请求字段", exact: true }),
    [
      drawer.getByLabel("请求 粘贴或修改 JSON", { exact: true }),
      drawer.getByLabel("请求 input.prompt 绑定参数", { exact: true }),
      drawer.getByLabel("请求 input.images 绑定参数", { exact: true }),
    ],
  );
  await pasteContractJSON(
    drawer,
    "响应",
    JSON.stringify(
      {
        output: { url: "https://cdn.example.com/video.mp4" },
        request_id: "example-request",
      },
      null,
      2,
    ),
  );
  await drawer
    .getByLabel("响应 output.url 绑定参数", { exact: true })
    .selectOption("result");
  await capture(
    "custom-response",
    drawer.getByRole("group", { name: "响应字段", exact: true }),
    [
      drawer.getByLabel("响应 粘贴或修改 JSON", { exact: true }),
      drawer.getByLabel("响应 output.url 绑定参数", { exact: true }),
    ],
  );
  await drawer
    .getByLabel("执行方式", { exact: true })
    .selectOption("async_polling");
  await page.setViewportSize({ width: 1280, height: 1500 });
  await drawer
    .getByLabel("查询路径", { exact: true })
    .fill("/v1/tasks/{task_id}");
  await drawer.getByLabel("查询路径", { exact: true }).scrollIntoViewIfNeeded();
  await capture("custom-polling", drawer, [
    drawer.getByLabel("查询路径", { exact: true }),
    drawer.getByRole("group", { name: "查询响应字段", exact: true }),
    drawer.getByRole("group", { name: "等待状态值", exact: true }),
  ]);
  await page.setViewportSize({ width: 1280, height: 1260 });
  await page.goto("/config");
  await page
    .getByRole("row")
    .filter({ hasText: "图片模型" })
    .getByRole("button", { name: "协议与参数" })
    .click();
  await capture("image-modes", page.getByRole("dialog"), [
    page.getByRole("group", { name: "图片生成能力（可多选）" }),
    page.getByLabel("最多参考图片数", { exact: true }),
    page.getByLabel("16:9 输出尺寸", { exact: true }),
  ]);
  const base = "/studio/demo/draft";
  await page.setViewportSize({ width: 1280, height: 560 });
  await page.goto(base);
  await capture("manual-story", page.locator(".editor-card"), [
    page.getByRole("button", { name: "导入文本", exact: true }),
    page.getByLabel("故事正文"),
  ]);
  await page.getByRole("button", { name: "故事联想", exact: true }).click();
  await page
    .getByLabel("你的想法或修改要求")
    .fill("以雨夜咖啡馆的重逢为主题，写一个温暖的短片故事。");
  await capture("story", page.locator(".story-drawer"), [
    page.getByLabel("你的想法或修改要求"),
    page.locator(".story-drawer select"),
    page.getByRole("button", { name: "生成正文", exact: true }),
  ]);
  draft.candidates = [
    {
      id: "story-candidate",
      target: "story",
      body: story,
      status: "preview",
      source_version: 1,
    },
  ];
  await page.goto(base);
  await page.getByRole("button", { name: "故事联想", exact: true }).click();
  await capture(
    "story-preview",
    page.locator(".story-drawer .candidate-card"),
    [
      page.getByRole("button", { name: "追加到正文", exact: true }),
      page.getByRole("button", { name: "替换正文", exact: true }),
    ],
  );
  draft.candidates = [
    {
      id: "shot-candidate",
      target: "storyboard",
      scenes: [scene],
      entities: [],
      status: "preview",
      source_version: 1,
    },
  ];
  await page.setViewportSize({ width: 1280, height: 960 });
  await page.goto(base);
  await capture("storyboard", page.locator(".editor-card .candidate-card"), [
    page.getByRole("button", { name: "应用分镜", exact: true }),
  ]);
  draft.candidates = [];
  await page.setViewportSize({ width: 1280, height: 1000 });
  await page.goto(`${base}/shots`);
  await capture("shot-list", page.locator("main.shot-workspace"), [
    page.getByRole("button", { name: "＋ 新增分镜" }),
    page.getByRole("link", { name: "进入详情 →" }),
  ]);
  await page.goto(`${base}/shots/shot`);
  await page.getByRole("button", { name: "＋ 添加场景", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByLabel("名称", { exact: true })
    .fill("雨夜咖啡馆");
  await page
    .getByRole("dialog")
    .getByLabel("参考描述", { exact: true })
    .fill("雨夜街角，暖黄色灯光，玻璃窗上映着雨滴。");
  await capture("references", page.getByRole("dialog"), [
    page.getByLabel("上传参考图片", { exact: true }).locator(".."),
    page.getByRole("button", { name: "图片生成", exact: true }),
    page.getByRole("button", { name: "保存并引用", exact: true }),
  ]);
  await page
    .getByLabel("上传参考图片", { exact: true })
    .setInputFiles("public/home/cafe.png");
  await capture("upload", page.locator(".reference-pending-upload"), [
    page.getByRole("button", { name: "上传到素材库", exact: true }),
    page.getByRole("button", { name: "使用这张图片", exact: true }),
  ]);
  await page.getByRole("button", { name: "取消上传", exact: true }).click();
  await page.setViewportSize({ width: 1280, height: 1500 });
  await page.getByRole("button", { name: "图片生成", exact: true }).click();
  await page.getByRole("button", { name: "图生图", exact: true }).click();
  await page
    .getByRole("button", { name: "从素材库选择参考图片", exact: true })
    .click();
  await page
    .getByRole("button", { name: "添加编辑参考图 咖啡馆参考图", exact: true })
    .click();
  await page
    .getByRole("button", { name: "添加编辑参考图 结束画面参考图", exact: true })
    .click();
  await page
    .getByRole("button", { name: "从素材库选择参考图片", exact: true })
    .click();
  await page
    .getByLabel("生图提示词", { exact: true })
    .fill(
      "保留图 1 的咖啡馆结构，参考图 2 的暖色灯光，将窗外改为雨夜。保留玻璃窗与木质桌椅。",
    );
  await capture(
    "image-edit",
    page.getByRole("region", { name: "图片生成设置", exact: true }),
    [
      page.getByRole("group", { name: "图片生成模式" }),
      page.getByRole("region", { name: "图生图参考图片", exact: true }),
      page.getByLabel("生图提示词", { exact: true }),
    ],
  );
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await page.setViewportSize({ width: 1280, height: 1000 });
  await expect(
    page.getByRole("button", { name: "生成视频", exact: true }),
  ).toBeEnabled();
  await capture(
    "shot",
    page.getByRole("region", { name: "场景描述", exact: true }),
    [page.getByLabel("镜头描述文本")],
  );
  await capture("frames", page.getByRole("region", { name: "视频设置" }), [
    page.getByRole("combobox", { name: "视频模型", exact: true }),
    page.getByLabel("首帧图片", { exact: true }),
    page.getByLabel("尾帧图片", { exact: true }),
  ]);
  scene.generation_settings.video_input_mode = "reference_images";
  await page.goto(`${base}/shots/shot`);
  await capture("smart", page.getByRole("region", { name: "视频设置" }), [
    page.getByRole("button", { name: "智能参考模式", exact: false }),
    page.locator(".video-reference-strip"),
  ]);
  showProgress = true;
  await page.goto(`${base}/shots/shot`);
  await capture("progress", page.getByRole("region", { name: "任务结构" }), [
    page.locator(".run-video-progress"),
    page.getByRole("link", { name: "进入任务 →" }),
  ]);
  await page.goto(`${base}/shots/shot/runs/run`);
  // Poster is explicitly illustrative; no fabricated playable result is presented.
  await page.locator("video").evaluate((el) => {
    el.setAttribute("poster", "/home/cafe.png");
    el.removeAttribute("src");
    el.setAttribute("preload", "none");
  });
  await capture("result", page.locator("main.shot-workspace"), [
    page.getByRole("link", { name: "返回分镜", exact: true }),
    page.getByRole("button", { name: "选用此结果", exact: true }),
  ]);
  draft.selected_versions = { shot: "version" };
  draft.compositions = [
    {
      id: "composition",
      task_id: "export-task",
      created_at: "2026-10-09T08:00:00Z",
      resolution: "720P",
      aspect_ratio: "16:9",
      music_volume: 0.25,
      source_volume: 1,
      voice_volume: 1,
      local_media_path: "/guide/demo.mp4",
      clip_version_ids: ["version"],
    },
  ];
  await page.goto(`${base}?tab=video`);
  await capture("composition", page.locator(".composition-settings"), [
    page.getByRole("combobox", { name: "背景音乐", exact: true }),
    page.getByRole("combobox", { name: "输出清晰度", exact: true }),
    page.getByRole("button", { name: "导出完整成片", exact: true }),
  ]);
  await capture(
    "download",
    page.locator(".workspace-video > .candidate-card"),
    [
      page.getByRole("button", { name: "播放成片", exact: true }),
      page.getByRole("button", { name: "下载成片", exact: true }),
    ],
  );
});
