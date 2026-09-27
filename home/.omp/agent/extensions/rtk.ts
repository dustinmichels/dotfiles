// @ts-nocheck
import { spawnSync } from "node:child_process";
import type { ExtensionAPI } from "@oh-my-pi/pi-coding-agent";

export default function rtkExtension(pi: ExtensionAPI): void {
  pi.on("tool_call", async (event) => {
    if (event.toolName !== "bash") return;

    const command = event.input?.command;
    if (!command || typeof command !== "string") return;

    const trimmed = command.trim();

    // 1. Skip if already calling rtk or empty
    if (!trimmed || trimmed.startsWith("rtk ")) return;

    // 2. Skip commands using omp internal virtual protocols (artifact://, omp://, local://)
    if (trimmed.includes("://")) return;

    try {
      const res = spawnSync("rtk", ["hook", "check", trimmed], {
        encoding: "utf-8",
        timeout: 2000,
      });

      // rtk hook check exits 0 with rewritten command on stdout if supported
      if (res.status === 0 && res.stdout) {
        const rewritten = res.stdout.trim();
        if (rewritten && rewritten !== trimmed) {
          return {
            input: {
              ...event.input,
              command: rewritten,
            },
          };
        }
      }
    } catch {
      // Fail open: leave command untouched if rtk fails or is unavailable
    }
  });
}
