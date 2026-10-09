import { request } from "./client";
import type { components } from "./schema";
import type { Capability, Model } from "./types";
type S = components["schemas"];
export const modelConfigurationApi = {
  protocols: () =>
    request<{ protocols: S["ProtocolDescriptor"][] }>("/api/model-protocols"),
  configure: (
    model: Model,
    operations: Record<string, S["OperationBinding"]>,
    capabilities: Capability[],
  ) =>
    request<Model>(`/api/models/${encodeURIComponent(model.id)}/operations`, {
      method: "PUT",
      body: JSON.stringify({
        expected_revision: model.revision,
        operations,
        capabilities,
      }),
    }),
};
