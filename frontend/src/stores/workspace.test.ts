import { beforeEach, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import type { Draft, Project } from "../api/types";
import { api } from "../api/client";
import { useWorkspaceStore } from "./workspace";

vi.mock("../api/client", () => ({
  api: {
    createProject: vi.fn(),
    saveDraft: vi.fn(),
    project: vi.fn(),
  },
}));

function draft(body: string): Draft {
  return {
    id: "draft",
    project_id: "project",
    name: "Draft",
    story: {
      summary: "",
      body,
      scenes: [],
      updated_at: "2026-09-22T00:00:00Z",
    },
    outputs: [],
  };
}

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
});

it("loads a route-scoped draft", async () => {
  const project: Project = {
    id: "project",
    name: "Demo",
    drafts: [draft("saved")],
    created_at: "2026-09-22T00:00:00Z",
  };
  vi.mocked(api.project).mockResolvedValue(project);
  const store = useWorkspaceStore();
  await store.load("project", "draft");
  expect(store.projectId).toBe("project");
  expect(store.draft.story.body).toBe("saved");
});

it("protects local edits when a generated draft arrives", () => {
  const store = useWorkspaceStore();
  store.applyDraft(draft("submitted"));
  store.draft.story.body = "local edit";
  const conflicted = store.mergeGeneratedDraft(draft("generated"), "submitted");
  expect(conflicted).toBe(true);
  expect(store.draft.story.body).toBe("local edit");
  expect(store.mergeGeneratedDraft(draft("generated"), "local edit")).toBe(
    false,
  );
  expect(store.draft.story.body).toBe("generated");
});
