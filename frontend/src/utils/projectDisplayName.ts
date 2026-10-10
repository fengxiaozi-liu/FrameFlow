import type { Project } from "../api/types";

// Single-draft projects created before title synchronization may have an old name.
export function projectDisplayName(project: Project): string {
  return project.drafts?.length === 1
    ? project.drafts[0]!.name.trim() || project.name
    : project.name;
}
