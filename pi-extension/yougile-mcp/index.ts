/**
 * yougile-mcp extension for pi
 *
 * Connects pi to the yougile-mcp MCP server (Go binary) over stdio
 * using the official @modelcontextprotocol/sdk Client.
 *
 * Registers all 15 tools exposed by the server via pi.registerTool().
 *
 * Setup:
 *   - Build the binary:  make build  (output: bin/yougile-mcp)
 *   - Place binary at:   ~/.local/bin/yougile-mcp  (or set YOUGILE_MCP_BIN)
 *   - Export YOUGILE_API_KEY (required by the server)
 */

import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

// ── Config ──────────────────────────────────────────────────────────────

function resolveBinary(): string {
  if (process.env.YOUGILE_MCP_BIN) return process.env.YOUGILE_MCP_BIN;
  const candidates = [
    join(homedir(), ".local", "bin", "yougile-mcp"),
    join(homedir(), "wss", "personal", "yougile_mcp", "bin", "yougile-mcp"),
  ];
  for (const p of candidates) {
    if (existsSync(p)) return p;
  }
  return candidates[0];
}

const BIN = resolveBinary();
const API_KEY = process.env.YOUGILE_API_KEY ?? "";
const BASE_URL = process.env.YOUGILE_BASE_URL ?? "https://ru.yougile.com/api-v2";
const MEMORY_DIR = process.env.YOUGILE_MEMORY_DIR ?? join(homedir(), ".yougile-mcp", "memory", "reviews");

// ── MCP client ─────────────────────────────────────────────────────────

let client: Client | null = null;
let transport: StdioClientTransport | null = null;
let toolList: any[] = [];
let connecting: Promise<void> | null = null;
let connectError: string | null = null;

async function ensureConnected(): Promise<void> {
  if (client) return;
  if (connecting) return connecting;
  connecting = (async () => {
    if (!existsSync(BIN)) {
      connectError = `Binary not found at ${BIN}. Build with: make build, then copy to ~/.local/bin/yougile-mcp`;
      return;
    }
    if (!API_KEY) {
      connectError = "YOUGILE_API_KEY is not set. Export it before starting pi.";
      return;
    }
    transport = new StdioClientTransport({
      command: BIN,
      args: [],
      env: {
        YOUGILE_API_KEY: API_KEY,
        YOUGILE_BASE_URL: BASE_URL,
        YOUGILE_MEMORY_DIR: MEMORY_DIR,
      },
    });
    client = new Client({ name: "pi-yougile-mcp", version: "0.1.0" });
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

// ── JSON Schema → typebox ───────────────────────────────────────────────

function jsonSchemaToTypebox(prop: any): any {
  switch (prop?.type) {
    case "string": {
      if (prop.enum) return Type.String({ description: prop.description, enum: prop.enum });
      return Type.String({ description: prop.description });
    }
    case "number":
    case "integer":
      return Type.Number({ description: prop.description });
    case "boolean":
      return Type.Boolean({ description: prop.description });
    case "array":
      return Type.Array(jsonSchemaToTypebox(prop.items ?? { type: "string" }));
    case "object":
      return Type.Object({});
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

// ── Extension ───────────────────────────────────────────────────────────

export default function yougileMcpExtension(pi: ExtensionAPI) {
  pi.on("session_start", async (_event, ctx) => {
    await ensureConnected();
    if (connectError) {
      ctx.ui.notify(`yougile-mcp: ${connectError}`, "error");
      return;
    }
    if (toolList.length === 0) {
      ctx.ui.notify("yougile-mcp: connected but no tools found", "warning");
      return;
    }

    for (const tool of toolList) {
      const name = tool.name;
      const description = tool.description ?? `YouGile MCP tool: ${name}`;

      pi.registerTool({
        name,
        label: name,
        description,
        promptSnippet: description,
        promptGuidelines: [
          `Use ${name} when the user asks about YouGile projects, boards, columns, tasks, stickers, goals, audits, or reviews.`,
        ],
        parameters: buildParamsSchema(tool),
        async execute(_toolCallId, params) {
          const args = (params ?? {}) as Record<string, unknown>;
          const res = await client!.callTool({ name, arguments: args });
          return { content: resultToContent(res), details: { mcpTool: name } };
        },
      });
    }

    ctx.ui.notify(`yougile-mcp: registered ${toolList.length} tools`, "info");
  });
}
