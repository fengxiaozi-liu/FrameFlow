# ADR 0001: MVP architecture

Status: Accepted

FrameFlow uses Go for the API and worker, Vue 3 with TypeScript for the UI, SQLite for persistence, and an in-process queue for the MVP. Domain packages do not import transport or storage packages. HTTP handlers and WebSocket handlers live in separate files and packages. Story, image, and video generation use separate provider ports; character and opening/ending frame generation share the image provider selection.

The API returns a task ID immediately. Workers update durable task events. WebSocket is the primary update channel and HTTP event polling is the fallback.
