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
  let stopped = true;
  let pending: TaskEvent[] = [];
  let loadingRequest: Promise<void> | undefined;
  const sequences = new Map<string, number>();
  function patch(event: TaskEvent) {
    if (event.sequence <= (sequences.get(event.task_id) || 0)) return;
    const item = items.value.find((v) => v.id === event.task_id);
    if (!item) return;
    sequences.set(event.task_id, event.sequence);
    // A snapshot may already be newer than an event buffered during its request.
    if (item.updated_at && Date.parse(event.at) < Date.parse(item.updated_at))
      return;
    Object.assign(item, {
      status: event.status,
      progress: event.progress,
      stage: event.stage,
      updated_at: event.at,
    });
  }
  function load(): Promise<void> {
    if (loadingRequest) return loadingRequest;
    loading.value = true;
    loadingRequest = (async () => {
      try {
        items.value = (await api.tasks()).tasks;
        const ids = new Set(items.value.map((item) => item.id));
        for (const id of sequences.keys())
          if (!ids.has(id)) sequences.delete(id);
        error.value = "";
      } catch (e) {
        error.value = (e as Error).message;
      } finally {
        pending.sort((a, b) => a.sequence - b.sequence).forEach(patch);
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
      // Wait for any older request, then query after the connection is established.
      if (loadingRequest) await loadingRequest;
      if (socket === ws) await load();
    };
    ws.onmessage = (message) => {
      if (socket !== ws) return;
      try {
        const event = JSON.parse(message.data) as TaskEvent;
        if (!event.task_id || !Number.isFinite(event.sequence)) return;
        if (loading.value) pending.push(event);
        else patch(event);
      } catch {
        /* Ignore malformed messages. */
      }
    };
    ws.onerror = () => ws.close();
    ws.onclose = () => {
      if (socket !== ws) return;
      socket = undefined;
      if (!stopped) reconnect = window.setTimeout(open, 1500);
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
    const previous = socket;
    socket = undefined;
    previous?.close();
  }
  async function create(
    kind: Task["kind"],
    provider: string,
    input: TaskInput,
  ) {
    const value = await api.createTask(kind, provider, input);
    items.value.unshift(value);
    // Catch events emitted before the creation response arrived.
    if (loadingRequest) await loadingRequest;
    await load();
    return value;
  }
  return { items, loading, error, running, load, create, connect, disconnect };
});
