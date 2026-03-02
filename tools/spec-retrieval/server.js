import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  ListToolsRequestSchema,
  CallToolRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { readdir, readFile } from "node:fs/promises";
import { join, basename } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const SPECS_DIR = join(__dirname, "../../docs/specs");

// Cached specs — loaded once at startup, not on every tool call
let cachedSpecs = null;

async function loadSpecs() {
  const files = await readdir(SPECS_DIR);
  const specs = [];
  for (const file of files) {
    if (!file.endsWith(".md")) continue;
    try {
      const content = await readFile(join(SPECS_DIR, file), "utf-8");
      const name = basename(file, ".md");
      const firstLine = content.split("\n").find((l) => l.startsWith("# "));
      const description = firstLine ? firstLine.replace("# ", "") : name;
      specs.push({ name, description, file, content });
    } catch (err) {
      console.error(`Warning: failed to read spec file ${file}: ${err.message}`);
    }
  }
  return specs;
}

async function getSpecs() {
  if (!cachedSpecs) {
    cachedSpecs = await loadSpecs();
  }
  return cachedSpecs;
}

const server = new Server(
  { name: "north-cloud-specs", version: "1.0.0" },
  { capabilities: { tools: {} } }
);

server.setRequestHandler(ListToolsRequestSchema, async () => ({
  tools: [
    {
      name: "list_specs",
      description:
        "List all available North Cloud subsystem specs with names and descriptions",
      inputSchema: { type: "object", properties: {} },
    },
    {
      name: "get_spec",
      description:
        "Get the full content of a specific North Cloud subsystem spec by name",
      inputSchema: {
        type: "object",
        properties: {
          name: {
            type: "string",
            description:
              "Spec name (e.g., content-acquisition, classification, content-routing)",
          },
        },
        required: ["name"],
      },
    },
    {
      name: "search_specs",
      description:
        "Search across all North Cloud specs for matching sections by keyword",
      inputSchema: {
        type: "object",
        properties: {
          query: {
            type: "string",
            description: "Search query (keyword substring match)",
          },
        },
        required: ["query"],
      },
    },
  ],
}));

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  let specs;
  try {
    specs = await getSpecs();
  } catch (err) {
    return {
      content: [{ type: "text", text: `Failed to load specs: ${err.message}` }],
      isError: true,
    };
  }

  if (name === "list_specs") {
    const list = specs.map((s) => ({
      name: s.name,
      description: s.description,
      file: s.file,
    }));
    return { content: [{ type: "text", text: JSON.stringify(list, null, 2) }] };
  }

  if (name === "get_spec") {
    if (!args?.name || typeof args.name !== "string") {
      return {
        content: [{ type: "text", text: 'Missing required argument "name" (string).' }],
        isError: true,
      };
    }
    const spec = specs.find((s) => s.name === args.name);
    if (!spec) {
      const available = specs.map((s) => s.name).join(", ");
      return {
        content: [
          {
            type: "text",
            text: `Spec "${args.name}" not found. Available: ${available}`,
          },
        ],
        isError: true,
      };
    }
    return { content: [{ type: "text", text: spec.content }] };
  }

  if (name === "search_specs") {
    if (!args?.query || typeof args.query !== "string") {
      return {
        content: [{ type: "text", text: 'Missing required argument "query" (string).' }],
        isError: true,
      };
    }
    const query = args.query.toLowerCase();
    const results = [];
    for (const spec of specs) {
      const sections = spec.content.split(/^## /m);
      for (const section of sections) {
        if (section.toLowerCase().includes(query)) {
          const title = section.split("\n")[0].trim();
          const preview = section.slice(0, 500);
          results.push({
            spec: spec.name,
            section: title,
            preview: preview.trim(),
          });
        }
      }
    }
    if (results.length === 0) {
      return {
        content: [
          { type: "text", text: `No matches found for "${args.query}"` },
        ],
      };
    }
    return {
      content: [{ type: "text", text: JSON.stringify(results, null, 2) }],
    };
  }

  return {
    content: [{ type: "text", text: `Unknown tool: ${name}` }],
    isError: true,
  };
});

async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main().catch((err) => {
  console.error(`Failed to start spec-retrieval MCP server: ${err.message}`);
  process.exit(1);
});
