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
  let socket: WebSocket | undefined;
  let reconnect: number | undefined;
  let heartbeat: number | undefined;
  let fallback: number | undefined;
  let stopped = true;
  let pending: TaskEvent[] = [];
  let loadingRequest: Promise<void> | undefined;
  const listeners = new Set<(event: TaskEvent) => void>();
  function notify(event: TaskEvent) {
    listeners.forEach((listener) => listener(event));
  }
  function patch(event: TaskEvent) {
    const item = items.value.find((v) => v.id === event.task_id);
    if (!item) return;
    // A snapshot may already be newer than an event buffered during its request.
    if (item.updated_at && Date.parse(event.at) < Date.parse(item.updated_at))
      return;
    Object.assign(item, {
      status: event.status,
      progress: event.progress,
      stage: event.stage,
      updated_at: event.at,
    });
    notify(event);
  }
  function load(projectId = "", draftId = ""): Promise<void> {
    if (loadingRequest) return loadingRequest;
    loading.value = true;
    loadingRequest = (async () => {
      try {
        const previous = new Map(
          items.value.map((item) => [item.id, item.status]),
        );
        items.value = (await api.tasks(projectId, draftId)).tasks;
        for (const item of items.value) {
          if (
            item.status === "succeeded" &&
            previous.has(item.id) &&
            previous.get(item.id) !== "succeeded"
          ) {
            notify({
              task_id: item.id,
              project_id: item.project_id,
              draft_id: item.draft_id,
              status: item.status,
              progress: item.progress,
              stage: item.stage,
              at: item.updated_at,
            });
          }
        }
        error.value = "";
      } catch (e) {
        error.value = (e as Error).message;
      } finally {
        pending
          .sort((a, b) => Date.parse(a.at) - Date.parse(b.at))
          .forEach(patch);
        pending = [];
        loading.value = false;
        loadingRequest = undefined;
      }
    })();
    return loadingRequest;
  }
  function open() {
    if (stopped || socket) return;
    const ws = new WebSocket(api.socketUrl());
    socket = ws;
    ws.onopen = async () => {
      clearInterval(fallback);
      fallback = undefined;
      heartbeat = window.setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) ws.send("ping");
      }, 15000);
      // Wait for any older request, then query after the connection is established.
      if (loadingRequest) await loadingRequest;
      if (socket === ws) await load();
    };
    ws.onmessage = (message) => {
      if (socket !== ws) return;
      try {
        const event = JSON.parse(message.data) as TaskEvent;
        if (!event.task_id || !Number.isFinite(Date.parse(event.at))) return;
        if (loading.value) pending.push(event);
        else patch(event);
      } catch {
        /* Ignore malformed messages. */
      }
    };
    ws.onerror = () => ws.close();
    ws.onclose = () => {
      if (socket !== ws) return;
      clearInterval(heartbeat);
      heartbeat = undefined;
      socket = undefined;
      if (!stopped) {
        void load();
        if (fallback === undefined)
          fallback = window.setInterval(() => void load(), 2000);
        reconnect = window.setTimeout(open, 1500);
      }
    };
  }
  function connect() {
    if (!stopped) return;
    stopped = false;
    open();
  }
  function disconnect() {
    stopped = true;
    clearTimeout(reconnect);
    clearInterval(heartbeat);
    clearInterval(fallback);
    const previous = socket;
    socket = undefined;
    previous?.close();
  }
  async function create(
    projectId: string,
    draftId: string,
    kind: Task["kind"],
    provider: string,
    input: TaskInput,
  ) {
    const value = await api.createTask(
      projectId,
      draftId,
      kind,
      provider,
      input,
    );
    items.value.unshift(value);
    // Catch events emitted before the creation response arrived.
    if (loadingRequest) await loadingRequest;
    await load();
    return value;
  }
  function subscribe(listener: (event: TaskEvent) => void) {
    listeners.add(listener);
    return () => listeners.delete(listener);
  }
  return {
    items,
    loading,
    error,
    running,
    load,
    create,
    connect,
    disconnect,
    subscribe,
  };
});
