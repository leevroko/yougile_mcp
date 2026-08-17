/**
 * yougile-mcp extension for pi
 *
 * Connects pi to the yougile-mcp MCP server (Go binary) over stdio
 * using the official @modelcontextprotocol/sdk Client.
 *
 * Security model (see SECURITY.md in the yougile_mcp repo):
 *   - Credentials from ~/.config/yougile-mcp/config.json (chmod 600), NOT from env
 *   - Tool policy from the same config: allow / confirm / deny (glob patterns)
 *   - "confirm" tools ask the user before every call (ctx.ui.confirm)
 *   - bulk tools run dry-run first, show the plan, then ask to apply
 *   - "deny" tools are not registered at all
 *
 * Commands: /yougile-status — current security state
 */

import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { existsSync, readFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

// ── Config (file-based, no secrets in env) ──────────────────────────────

interface YougileConfig {
  api_key: string;
  base_url?: string;
  memory_dir?: string;
  read_only?: boolean;
  allow_insecure?: boolean;
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
  "track_goals", "compress_reviews",
];
const DEFAULT_CONFIRM = [
  "create_task", "update_task", "audit_board",
  "bulk_move_tasks", "batch_update_stickers",
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
      connectError = `config not found at ${configPath()}. Run: yougile-mcp init (then remove YOUGILE_API_KEY from env)`;
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
        // Only these vars reach the server process; never the full user env.
        // YOUGILE_CONFIG is a PATH (not a secret): server reads the same config file itself.
        // The API key itself never transits through env — server reads it from the file.
        YOUGILE_CONFIG: configPath(),
      },
    });
    client = new Client({ name: "pi-yougile-mcp", version: "0.2.0" });
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

// JSON Schema → typebox (rough mapping, enough for MCP schemas)

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
  const confirmed: string[] = [];

  pi.on("session_start", async (_event, ctx) => {
    await ensureConnected();
    if (connectError) {
      ctx.ui.notify(`yougile-mcp: ${connectError}`, "error");
      return;
    }
    if (toolList.length === 0) {
      ctx.ui.notify("yougile-mcp: connected but no tools found (read-only mode?)", "warning");
      return;
    }

    const perms = {
      allow: cfg?.permissions?.allow ?? DEFAULT_ALLOW,
      confirm: cfg?.permissions?.confirm ?? DEFAULT_CONFIRM,
      deny: cfg?.permissions?.deny ?? [],
    };
    const bulkDryRunFirst = cfg?.bulk_dry_run_first !== false; // default true

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

          if (level === "confirm") {
            // Dry-run-first for bulk tools: show the plan, then ask
            if (bulkDryRunFirst && DRY_RUN_FIRST.has(name) && !args.dryRun) {
              if (name === "audit_board" && !args.autoMove) {
                // audit without autoMove is read-only — skip the dry-run dance
              } else {
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
            // Regular confirm: tool + short args summary
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
      if (level === "confirm") confirmed.push(name);
    }

    const mode = cfg?.read_only ? "read-only" : "normal";
    ctx.ui.notify(
      `yougile-mcp: ${registered.length} tools (${mode}), confirm: ${confirmed.length}, denied: ${denied.length}`,
      "info"
    );
  });

  pi.registerCommand("yougile-status", {
    description: "Security status of yougile-mcp (mode, policy, paths)",
    handler: async (_args, ctx) => {
      await ensureConnected();
      const lines: string[] = ["yougile-mcp security status:"];
      lines.push(`  config:   ${configPath()}`);
      if (connectError) {
        lines.push(`  error:    ${connectError}`);
        ctx.ui.notify(lines.join("\n"), "error");
        return;
      }
      lines.push(`  mode:     ${cfg?.read_only ? "read-only (mutations blocked server-side)" : "normal"}`);
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
