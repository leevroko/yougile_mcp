/**
 * yougile-mcp extension for pi
 *
 * Connects pi to the yougile-mcp MCP server (Go binary) over stdio.
 * Security model (see SECURITY.md in the yougile_mcp repo):
 *   - credentials from ~/.config/yougile-mcp/config.json (chmod 600), NOT env
 *   - tool policy from config: allow / confirm / deny (glob)
 *   - three modes (server-side + reflected here):
 *       read    → only read tools visible (mutations hidden via setActiveTools)
 *       confirm → mutations ask the user (ctx.ui.confirm), bulk = dry-run first
 *       yolo    → everything allowed, no prompts
 *   - set_mode/get_mode MCP tools control the mode through the MCP API
 *
 * Commands: /yougile-status, /yougile-mode
 */

import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { existsSync, readFileSync } from "node:fs";
import { homedir } from "node:os";
import { join, sep } from "node:path";

// ── Project-scope guard (issue #1) ─────────────────────────────────────
// Extension is installed user-scope, but must only activate inside the
// yougile_mcp project. Silent no-op everywhere else.
// Override via env YOUGILE_PI_EXT: "global" (activate everywhere) | "off" (never).

const PROJECT_ROOT = join(homedir(), "wss", "personal", "yougile_mcp");

function inProjectSession(cwd: string): boolean {
  const override = process.env.YOUGILE_PI_EXT;
  if (override === "global") return true;
  if (override === "off") return false;
  return cwd === PROJECT_ROOT || cwd.startsWith(PROJECT_ROOT + sep);
}

// ── Config (file-based, no secrets in env) ──────────────────────────────

interface YougileConfig {
  api_key: string;
  base_url?: string;
  memory_dir?: string;
  mode?: "read" | "confirm" | "yolo";
  read_only?: boolean; // legacy
  bulk_dry_run_first?: boolean;
  permissions?: { allow?: string[]; confirm?: string[]; deny?: string[] };
  audit?: { enabled?: boolean; path?: string };
}

const DEFAULT_CONFIG_PATH = join(homedir(), ".config", "yougile-mcp", "config.json");

function configPath(): string {
  return process.env.YOUGILE_CONFIG || DEFAULT_CONFIG_PATH; // env holds a PATH, not a secret
}

function loadConfig(): YougileConfig | null {
  const p = configPath();
  if (!existsSync(p)) return null;
  try {
    return JSON.parse(readFileSync(p, "utf-8")) as YougileConfig;
  } catch {
    return null;
  }
}

function resolveBinary(): string {
  if (process.env.YOUGILE_MCP_BIN) return process.env.YOUGILE_MCP_BIN;
  const candidates = [
    join(homedir(), ".local", "bin", "yougile-mcp"),
    join(homedir(), "wss", "personal", "yougile_mcp", "bin", "yougile-mcp"),
  ];
  for (const p of candidates) if (existsSync(p)) return p;
  return candidates[0];
}

const DEFAULT_ALLOW = [
  "list_*", "get_*", "get_board_snapshot", "summarize_board",
  "track_goals", "compress_reviews", "get_mode",
];
const DEFAULT_CONFIRM = [
  "create_task", "update_task", "create_board", "create_column", "audit_board",
  "bulk_move_tasks", "batch_update_stickers", "set_mode",
];

// ── Glob matching (minimal, '*' wildcard) ──────────────────────────────

function globMatch(pattern: string, name: string): boolean {
  const parts = pattern.split("*");
  if (parts.length === 1) return pattern === name;
  let idx = 0;
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i];
    if (i === 0) {
      if (!name.startsWith(part)) return false;
      idx = part.length;
    } else if (i === parts.length - 1) {
      if (!name.slice(idx).endsWith(part) || name.length - idx < part.length) return false;
    } else {
      const at = name.indexOf(part, idx);
      if (at === -1) return false;
      idx = at + part.length;
    }
  }
  return true;
}

function matchesAny(patterns: string[] | undefined, name: string): boolean {
  return (patterns ?? []).some((p) => globMatch(p, name));
}

type Level = "allow" | "confirm" | "deny";

function levelFor(name: string, perms: NonNullable<YougileConfig["permissions"]>): Level {
  if (matchesAny(perms.deny, name)) return "deny";
  if (matchesAny(perms.confirm, name)) return "confirm";
  if (matchesAny(perms.allow, name)) return "allow";
  return "deny"; // unknown tool — deny by default
}

// ── MCP client ─────────────────────────────────────────────────────────

let client: Client | null = null;
let toolList: any[] = [];
let cfg: YougileConfig | null = null;
let connectError: string | null = null;
let connecting: Promise<void> | null = null;

async function ensureConnected(): Promise<void> {
  if (client) return;
  if (connecting) return connecting;
  connecting = (async () => {
    cfg = loadConfig();
    if (!cfg) {
      connectError = `config not found at ${configPath()}. Run: yougile-mcp init`;
      return;
    }
    if (!cfg.api_key) {
      connectError = `api_key is empty in ${configPath()}`;
      return;
    }
    const bin = resolveBinary();
    if (!existsSync(bin)) {
      connectError = `binary not found at ${bin}. Build: make build && cp bin/yougile-mcp ~/.local/bin/`;
      return;
    }
    const transport = new StdioClientTransport({
      command: bin,
      args: [],
      env: {
        YOUGILE_CONFIG: configPath(),
      },
    });
    client = new Client({ name: "pi-yougile-mcp", version: "0.3.0" });
    await client.connect(transport);
    const res = await client.listTools();
    toolList = res.tools ?? [];
    connectError = null;
  })();
  try {
    await connecting;
  } finally {
    connecting = null;
  }
}

async function currentMode(): Promise<string> {
  if (!client) return "unknown";
  try {
    const res = await client.callTool({ name: "get_mode", arguments: {} });
    const content = (res?.content ?? []) as any[];
    const t = content.find((c: any) => c?.type === "text")?.text ?? "";
    try { return JSON.parse(t).mode ?? "unknown"; } catch { return "unknown"; }
  } catch {
    return "unknown";
  }
}

// ── Tool result → pi content ────────────────────────────────────────────

function resultToContent(result: any): { type: "text"; text: string }[] {
  const content = result?.content;
  if (Array.isArray(content)) {
    const texts = content
      .filter((c: any) => c?.type === "text")
      .map((c: any) => c.text ?? "")
      .join("\n");
    if (texts) return [{ type: "text", text: texts }];
  }
  return [{ type: "text", text: JSON.stringify(result ?? {}) }];
}

function firstText(result: any): string {
  return resultToContent(result)[0]?.text ?? "";
}

// JSON Schema → typebox (rough mapping)

function jsonSchemaToTypebox(prop: any): any {
  switch (prop?.type) {
    case "string":
      return Type.String({ description: prop.description });
    case "number":
    case "integer":
      return Type.Number({ description: prop.description });
    case "boolean":
      return Type.Boolean({ description: prop.description });
    case "array":
      return Type.Array(jsonSchemaToTypebox(prop.items ?? { type: "string" }));
    default:
      return Type.Any();
  }
}

function buildParamsSchema(tool: any): any {
  const schema = tool?.inputSchema;
  if (!schema?.properties) return Type.Object({});
  const props: Record<string, any> = {};
  for (const [k, v] of Object.entries<any>(schema.properties)) {
    props[k] = jsonSchemaToTypebox(v);
  }
  return Type.Object(props);
}

// Tools that get dry-run-first treatment
const DRY_RUN_FIRST = new Set(["bulk_move_tasks", "batch_update_stickers", "audit_board"]);

// ── Extension ───────────────────────────────────────────────────────────

export default function yougileMcpExtension(pi: ExtensionAPI) {
  const registered: string[] = [];
  const denied: string[] = [];
  // Режим: "off" (локальный, сессия) | "read" | "confirm" | "yolo" (с сервера)
  let mode: string = "unknown";

  const mutatingNames = new Set(
    ["create_task", "update_task", "create_board", "create_column", "audit_board", "bulk_move_tasks", "batch_update_stickers", "set_mode"]
  );

  // Issue #1: setActiveTools() is a FULL replacement of the session toolset.
  // Passing only our names wiped builtin tools (bash/read/edit) and other
  // extensions' tools. Compute the "others" set (everything currently active
  // that this extension does not own) and always keep it intact.
  function applyModeVisibility() {
    if (!pi.setActiveTools || !pi.getActiveTools) return;
    const own = new Set(registered);
    const others = pi.getActiveTools().filter((n) => !own.has(n));
    if (mode === "off") {
      pi.setActiveTools(others); // off: hide YouGile tools only, keep the rest
      return;
    }
    const yougileVisible = mode === "read"
      ? registered.filter((n) => !mutatingNames.has(n))
      : registered;
    pi.setActiveTools([...others, ...yougileVisible]);
  }

  // session_start fires on startup AND on resume; if extensions are NOT
  // reloaded in-process, this handler can run twice — never register the
  // same tools twice.
  let toolsRegistered = false;

  pi.on("session_start", async (_event, ctx) => {
    // cwd-guard: activate only inside the yougile_mcp project (issue #1)
    if (!inProjectSession(ctx.cwd)) return; // silent no-op in other projects

    if (toolsRegistered) {
      // Resume in the same process: just re-apply visibility, keep toolset intact.
      applyModeVisibility();
      return;
    }

    await ensureConnected();
    if (connectError) {
      ctx.ui.notify(`yougile-mcp: ${connectError}`, "error");
      return;
    }
    if (toolList.length === 0) {
      ctx.ui.notify("yougile-mcp: connected but no tools found", "warning");
      return;
    }

    const perms = {
      allow: cfg?.permissions?.allow ?? DEFAULT_ALLOW,
      confirm: cfg?.permissions?.confirm ?? DEFAULT_CONFIRM,
      deny: cfg?.permissions?.deny ?? [],
    };
    const bulkDryRunFirst = cfg?.bulk_dry_run_first !== false; // default true
    mode = await currentMode();
    if (mode === "unknown") mode = cfg?.read_only ? "read" : (cfg?.mode ?? "confirm");

    for (const tool of toolList) {
      const name = tool.name;
      const description = tool.description ?? `YouGile MCP tool: ${name}`;
      const level = levelFor(name, perms);

      if (level === "deny") {
        denied.push(name); // not registered — invisible to the LLM
        continue;
      }

      const callServer = (args: Record<string, unknown>) =>
        client!.callTool({ name, arguments: args });

      pi.registerTool({
        name,
        label: name,
        description,
        promptSnippet: description,
        promptGuidelines: [
          `Use ${name} when the user asks about YouGile projects, boards, columns, tasks, stickers, goals, audits, or reviews.`,
        ],
        parameters: buildParamsSchema(tool),
        async execute(_toolCallId, params, _signal, onUpdate, execCtx) {
          const args = (params ?? {}) as Record<string, unknown>;

          // off: доступ к YouGile полностью отключён
          if (mode === "off") {
            return { content: [{ type: "text", text: "yougile-mcp выключен (off). Включите: /yougile-mode read|confirm|yolo" }], details: { off: true } };
          }

          // set_mode: переключить режим на сервере, обновить локальный режим и видимость
          if (name === "set_mode") {
            const target = String(args.mode ?? "");
            const res = await callServer(args);
            const ok = firstText(res).includes('"ok": true');
            if (ok) {
              mode = target;
              applyModeVisibility();
            }
            return { content: resultToContent(res), details: { mcpTool: name, mode: target } };
          }

          // read-режим: мутации скрыты, но если вызов прошёл — блокировка на сервере
          if (mode === "read" && mutatingNames.has(name)) {
            return { content: [{ type: "text", text: "yougile-mcp в read-режиме: мутации запрещены. Используйте set_mode(confirm|yolo)." }], details: { blocked: true } };
          }

          // confirm-режим: диалоги
          if (mode === "confirm" && level === "confirm" && name !== "set_mode") {
            // Dry-run-first for bulk tools
            if (bulkDryRunFirst && DRY_RUN_FIRST.has(name) && !args.dryRun) {
              if (!(name === "audit_board" && !args.autoMove)) {
                onUpdate?.({ content: [{ type: "text", text: "Считаю план изменений (dry-run)…" }], details: {} });
                const dry = await callServer({ ...args, dryRun: true });
                const plan = firstText(dry).slice(0, 2000);
                const ok = await execCtx.ui.confirm(
                  `yougile-mcp: ${name}`,
                  `План изменений (dry-run):\n\n${plan}\n\nПрименить на самом деле?`
                );
                if (!ok) {
                  return { content: [{ type: "text", text: "Отменено пользователем" }], details: { cancelled: true } };
                }
                const res = await callServer(args);
                return { content: resultToContent(res), details: { mcpTool: name, confirmed: true } };
              }
            }
            const brief = Object.entries(args)
              .map(([k, v]) => `${k}=${String(v).slice(0, 60)}`)
              .join(", ")
              .slice(0, 300);
            const ok = await execCtx.ui.confirm(
              `yougile-mcp: ${name}`,
              `Выполнить \`${name}\`?\n\n${brief}`
            );
            if (!ok) {
              return { content: [{ type: "text", text: "Отменено пользователем" }], details: { cancelled: true } };
            }
          }

          const res = await callServer(args);
          return { content: resultToContent(res), details: { mcpTool: name } };
        },
      });

      registered.push(name);
    }

    toolsRegistered = true;
    applyModeVisibility();
    const visible = mode === "off"
      ? 0
      : mode === "read"
        ? registered.filter((n) => !mutatingNames.has(n)).length
        : registered.length;
    ctx.ui.notify(
      `yougile-mcp: mode=${mode}, tools: ${visible} видимых из ${registered.length}, denied: ${denied.length}`,
      "info"
    );
  });

  pi.registerCommand("yougile-mode", {
    description: "Переключить режим yougile-mcp: /yougile-mode off|read|confirm|yolo",
    handler: async (args, ctx) => {
      const target = (args ?? "").trim().toLowerCase();
      if (!["off", "read", "confirm", "yolo"].includes(target)) {
        ctx.ui.notify(`Текущий режим: ${mode}. Используй: /yougile-mode off|read|confirm|yolo`, "info");
        return;
      }
      // off — локальный режим сессии (сервер/конфиг не трогаем)
      if (target === "off") {
        mode = "off";
        applyModeVisibility();
        ctx.ui.notify("yougile-mcp: режим → off (доступ к YouGile отключён; вернуть: /yougile-mode read|confirm|yolo)", "info");
        return;
      }
      if (!client) {
        ctx.ui.notify("yougile-mcp не активен (нет подключения или сессия вне проекта " + PROJECT_ROOT + ")", "error");
        return;
      }
      const res = await client.callTool({ name: "set_mode", arguments: { mode: target } });
      const ok = firstText(res).includes('"ok": true');
      if (ok) {
        mode = target;
        applyModeVisibility();
        ctx.ui.notify(`yougile-mcp: режим → ${mode}`, "info");
      } else {
        ctx.ui.notify(`Ошибка переключения: ${firstText(res)}`, "error");
      }
    },
  });

  pi.registerCommand("yougile-status", {
    description: "Security status of yougile-mcp (mode, policy, paths)",
    handler: async (_args, ctx) => {
      if (!inProjectSession(ctx.cwd)) {
        ctx.ui.notify(`yougile-mcp: вне проекта ${PROJECT_ROOT} расширение неактивно`, "info");
        return;
      }
      await ensureConnected();
      const lines: string[] = ["yougile-mcp security status:"];
      lines.push(`  config:   ${configPath()}`);
      if (connectError) {
        lines.push(`  error:    ${connectError}`);
        ctx.ui.notify(lines.join("\n"), "error");
        return;
      }
      const liveMode = mode === "off" ? "off (локально)" : await currentMode();
      lines.push(`  mode:     ${liveMode} (в config: ${cfg?.mode ?? (cfg?.read_only ? "read" : "не задан")})`);
      lines.push(`  api key:  loaded from config file (${cfg?.api_key ? "present" : "MISSING"})`);
      lines.push(`  audit:    ${cfg?.audit?.enabled === false ? "disabled" : `enabled → ${cfg?.audit?.path ?? "~/.local/state/yougile-mcp/audit.jsonl"}`}`);
      lines.push(`  allow:    ${(cfg?.permissions?.allow ?? DEFAULT_ALLOW).join(", ")}`);
      lines.push(`  confirm:  ${(cfg?.permissions?.confirm ?? DEFAULT_CONFIRM).join(", ")}`);
      lines.push(`  deny:     ${(cfg?.permissions?.deny ?? []).join(", ") || "(none)"}`);
      lines.push(`  bulk dry-run-first: ${cfg?.bulk_dry_run_first !== false ? "on" : "off"}`);
      lines.push(`  registered: ${registered.length}, denied (hidden): ${denied.length}`);
      ctx.ui.notify(lines.join("\n"), "info");
    },
  });
}
