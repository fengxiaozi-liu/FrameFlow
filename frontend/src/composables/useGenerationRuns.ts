import { ref, onBeforeUnmount, onMounted } from "vue";
import { useTaskStore } from "../stores/tasks";
import {
  bindingsApi,
  type GenerationRun,
  type GenerationRequest,
} from "../api/bindings";

export function useGenerationRuns(scope: {
  project: string;
  draft: string;
  scene: string;
}) {
  const runs = ref<GenerationRun[]>([]),
    submitting = ref(false),
    error = ref("");
  const pendingSubmission = ref(false);
  const deleting = ref(""),
    deleted = new Set<string>();
  const key = `frameflow-submit:${scope.project}:${scope.draft}:${scope.scene}`;
  let alive = true,
    pending: GenerationRequest | undefined,
    submission: Promise<GenerationRun | undefined> | undefined;
  let timer: ReturnType<typeof setInterval> | undefined;
  const tasks = useTaskStore();
  const unsubscribe = tasks.subscribe((event) => {
    if (event.project_id !== scope.project || event.draft_id !== scope.draft)
      return;
    const run = runs.value.find((r) => r.task_ids.includes(event.task_id));
    if (run)
      void bindingsApi
        .getRun(scope.project, scope.draft, scope.scene, run.id)
        .then(merge)
        .catch((e) => {
          handleRunError(run.id, e);
        });
  });
  function handleRunError(id: string, e: unknown) {
    if (!alive || deleted.has(id)) return;
    const failure = e as Error & {
      status?: number;
      payload?: { error?: { code?: string } };
    };
    if (
      failure.status === 404 ||
      failure.payload?.error?.code === "run_not_found"
    ) {
      deleted.add(id);
      runs.value = runs.value.filter((r) => r.id !== id);
      return;
    }
    error.value = failure.message;
  }
  try {
    const stored = sessionStorage.getItem(key);
    if (stored) {
      pending = JSON.parse(stored);
      pendingSubmission.value = !!pending;
    }
  } catch {
    /* storage unavailable */
  }
  function merge(run: GenerationRun) {
    if (!alive || deleted.has(run.id)) return;
    const i = runs.value.findIndex((r) => r.id === run.id);
    if (i < 0) runs.value.push(run);
    else {
      const current = runs.value[i]!;
      if (
        current.updated_at &&
        run.updated_at &&
        Date.parse(run.updated_at) < Date.parse(current.updated_at)
      )
        return;
      runs.value[i] = run;
    }
    runs.value.sort(
      (a, b) =>
        Date.parse(a.created_at || "1970-01-01") -
        Date.parse(b.created_at || "1970-01-01"),
    );
  }
  async function remove(run: GenerationRun) {
    if (deleting.value || run.state !== "failed") return false;
    deleting.value = run.id;
    error.value = "";
    try {
      await bindingsApi.deleteRun(
        scope.project,
        scope.draft,
        scope.scene,
        run.id,
      );
      deleted.add(run.id);
      runs.value = runs.value.filter((r) => r.id !== run.id);
      await tasks.load();
      return true;
    } catch (e) {
      if (alive) error.value = (e as Error).message;
      return false;
    } finally {
      if (alive) deleting.value = "";
    }
  }
  function clearPending() {
    pending = undefined;
    pendingSubmission.value = false;
    try {
      sessionStorage.removeItem(key);
    } catch {
      /* storage unavailable */
    }
  }
  async function refresh() {
    try {
      const result = await bindingsApi.runs(
        scope.project,
        scope.draft,
        scope.scene,
      );
      if (alive) result.runs.forEach(merge);
    } catch (e) {
      if (alive) error.value = (e as Error).message;
    }
  }
  async function submit(
    input: Omit<GenerationRequest, "idempotency_key">,
  ): Promise<GenerationRun | undefined> {
    if (submission) return submission;
    if (!pending) pending = { ...input, idempotency_key: crypto.randomUUID() };
    pendingSubmission.value = true;
    try {
      sessionStorage.setItem(key, JSON.stringify(pending));
    } catch {
      /* retain key in memory */
    }
    submitting.value = true;
    error.value = "";
    submission = (async () => {
      try {
        const run = await bindingsApi.run(
          scope.project,
          scope.draft,
          scope.scene,
          pending!,
        );
        merge(run);
        clearPending();
        return run;
      } catch (e) {
        const failure = e as Error & { status?: number };
        if (failure.status && failure.status < 500) {
          clearPending();
        }
        if (alive) error.value = failure.message;
        return undefined;
      } finally {
        if (alive) submitting.value = false;
        submission = undefined;
      }
    })();
    return submission;
  }
  async function poll() {
    for (const r of [...runs.value]) {
      if (!alive) return;
      if (!deleted.has(r.id) && !["completed", "cancelled"].includes(r.state)) {
        try {
          merge(
            await bindingsApi.getRun(
              scope.project,
              scope.draft,
              scope.scene,
              r.id,
            ),
          );
        } catch (e) {
          handleRunError(r.id, e);
        }
      }
    }
  }
  onMounted(() => {
    void refresh();
    timer = setInterval(() => void poll(), 4000);
  });
  onBeforeUnmount(() => {
    alive = false;
    unsubscribe();
    clearInterval(timer);
  });
  return {
    runs,
    submitting,
    error,
    submit,
    refresh,
    merge,
    pendingSubmission,
    remove,
    deleting,
  };
}
