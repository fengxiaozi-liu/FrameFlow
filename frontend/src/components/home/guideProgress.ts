import { steps } from "./content";

const storageKey = "frameflow-guide-step-v2";

// Stable IDs keep reading progress valid when guide steps are reordered.
export function readGuideStep(): number {
  try {
    const index = steps.findIndex(
      (step) => step.id === localStorage.getItem(storageKey),
    );
    return Math.max(0, index);
  } catch {
    return 0;
  }
}

export function rememberGuideStep(index: number) {
  const step = steps[index];
  if (!step) return;
  try {
    localStorage.setItem(storageKey, step.id);
  } catch {
    // Reading still works when browser storage is unavailable.
  }
}

export function contextualGuideStep(path: string, tab: unknown): number {
  if (path === "/config") return 0;
  if (path === "/materials") return 3;
  if (path === "/tasks" || path.includes("/runs/")) return 4;
  if (path.includes("/shots/")) return 4;
  if (path.endsWith("/shots")) return 2;
  if (tab === "video") return 5;
  if (path.startsWith("/studio/")) return 1;
  return readGuideStep();
}
