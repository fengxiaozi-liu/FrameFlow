import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { api } from "../api/client";
import type { Task, TaskEvent } from "../api/types";
import { useTaskStore } from "./tasks";
vi.mock("../api/client", () => ({
  api: {
    socketUrl: () => "ws://test/ws",
    tasks: vi.fn(),
    createTask: vi.fn(),
  },
}));
class Socket {
  readyState = 1;
  send = vi.fn();
  onopen = async () => {};
  onmessage = (_: { data: string }) => {};
  onclose = () => {};
  onerror = () => {};
  close = vi.fn();
}
let sockets: Socket[];
beforeEach(() => {
  setActivePinia(createPinia());
  vi.useFakeTimers();
  sockets = [];
  vi.stubGlobal(
    "WebSocket",
    Object.assign(vi.fn(function () {
      const socket = new Socket();
      sockets.push(socket);
      return socket;
    }), { OPEN: 1 }),
  );
  vi.mocked(api.tasks).mockResolvedValue({
    tasks: [task("a"), task("b")],
    total: 2,
  });
});
afterEach(() => {
  useTaskStore().disconnect();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});
function task(id: string): Task {
  return { id, status: "queued", progress: 0 } as Task;
}
function event(
  id: string,
  sequence: number,
  status: TaskEvent["status"] = "running",
): TaskEvent {
  return {
    task_id: id,
    status,
    progress: sequence * 10,
    stage: status,
    at: `2026-09-16T00:00:${String(sequence).padStart(2, "0")}Z`,
  };
}
function emit(socket: Socket, value: TaskEvent) {
  socket.onmessage({ data: JSON.stringify(value) });
}
it("shares one connection across tasks and keeps it after completion", async () => {
  const store = useTaskStore();
  store.connect();
  store.connect();
  await sockets[0].onopen();
  emit(sockets[0], event("a", 1, "succeeded"));
  emit(sockets[0], event("b", 2));
  expect(sockets).toHaveLength(1);
  expect(store.items[0].status).toBe("succeeded");
  expect(store.items[1].progress).toBe(20);
  expect(sockets[0].close).not.toHaveBeenCalled();
  await vi.advanceTimersByTimeAsync(15000);
  expect(sockets[0].send).toHaveBeenCalledWith("ping");
});
it("buffers live events during HTTP catch-up and ignores old events", async () => {
  let resolve!: (value: { tasks: Task[]; total: number }) => void;
  vi.mocked(api.tasks).mockReturnValue(
    new Promise((done) => {
      resolve = done;
    }),
  );
  const store = useTaskStore();
  store.connect();
  const opening = sockets[0].onopen();
  emit(sockets[0], event("a", 2));
  resolve({ tasks: [task("a")], total: 1 });
  await opening;
  emit(sockets[0], event("a", 1));
  expect(store.items[0].progress).toBe(20);
});
it("reconnects without a client session and refreshes missed state", async () => {
  const store = useTaskStore();
  store.connect();
  await sockets[0].onopen();
  sockets[0].onclose();
  await Promise.resolve();
  expect(api.tasks).toHaveBeenCalledTimes(2);
  await vi.advanceTimersByTimeAsync(1500);
  expect(sockets).toHaveLength(2);
  vi.mocked(api.tasks).mockResolvedValue({
    tasks: [{ ...task("a"), status: "succeeded" }],
    total: 1,
  });
  await sockets[1].onopen();
  expect(store.items[0].status).toBe("succeeded");
  expect(vi.mocked(WebSocket).mock.calls[0][0]).toBe(
    vi.mocked(WebSocket).mock.calls[1][0],
  );
  store.disconnect();
  sockets[1].onclose();
  await vi.advanceTimersByTimeAsync(3000);
  expect(sockets).toHaveLength(2);
});
it("refreshes progress emitted before the create response without opening another socket", async () => {
  const store = useTaskStore();
  store.connect();
  await sockets[0].onopen();
  vi.mocked(api.createTask).mockResolvedValue(task("new"));
  vi.mocked(api.tasks).mockResolvedValue({
    tasks: [{ ...task("new"), status: "succeeded" }],
    total: 1,
  });
  await store.create("video", "", { prompt: "test" });
  expect(store.items[0].status).toBe("succeeded");
  expect(sockets).toHaveLength(1);
});
