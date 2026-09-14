import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { api } from "../api/client";
import type { Task, TaskEvent, TaskInput } from "../api/types";
export const useTaskStore = defineStore("tasks", () => {
  const items = ref<Task[]>([]),
    loading = ref(false),
    error = ref("");
  const running = computed(() =>
    items.value.filter((v) => v.status === "queued" || v.status === "running"),
  );
  async function load() {
    loading.value = true;
    try {
      items.value = (await api.tasks()).tasks;
      error.value = "";
    } catch (e) {
      error.value = (e as Error).message;
    } finally {
      loading.value = false;
    }
  }
  function patch(event: TaskEvent) {
    const item = items.value.find((v) => v.id === event.task_id);
    if (item)
      Object.assign(item, {
        status: event.status,
        progress: event.progress,
        stage: event.stage,
        updated_at: event.at,
      });
  }
  function watch(id: string) {
    let after = 0,
      timer: number | undefined,
      closed = false;
    const ws = new WebSocket(api.socketUrl(id));
    ws.onmessage = (e) => {
      const event = JSON.parse(e.data) as TaskEvent;
      after = event.sequence;
      patch(event);
    };
    ws.onerror = () => ws.close();
    ws.onclose = () => {
      if (closed) return;
      timer = window.setInterval(async () => {
        const events = (await api.taskEvents(id, after)).events;
        events.forEach((e) => {
          after = e.sequence;
          patch(e);
        });
        if (
          events.some((e) =>
            ["succeeded", "failed", "cancelled"].includes(e.status),
          ) &&
          timer
        )
          clearInterval(timer);
      }, 1500);
    };
    return () => {
      closed = true;
      ws.close();
      if (timer) clearInterval(timer);
    };
  }
  async function create(
    kind: Task["kind"],
    provider: string,
    input: TaskInput,
  ) {
    const value = await api.createTask(kind, provider, input);
    items.value.unshift(value);
    watch(value.id);
    return value;
  }
  return { items, loading, error, running, load, create, watch };
});
