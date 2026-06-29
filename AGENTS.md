<!-- CODEGRAPH_START -->
## CodeGraph

This project has a CodeGraph MCP server (`codegraph_*` tools) configured. CodeGraph is a tree-sitter-parsed knowledge graph of every symbol, edge, and file. Reads are sub-millisecond and return structural information grep cannot.

### When to prefer codegraph over native search

Use codegraph for **structural** questions: what calls what, what would break, where X is defined, and what X's signature is. Use native grep/read only for **literal text** queries, such as string contents, comments, log messages, or after you already have a specific file open.

| Question | Tool |
|---|---|
| "Where is X defined?" / "Find symbol named X" | `codegraph_search` |
| "What calls function Y?" | `codegraph_callers` |
| "What does Y call?" | `codegraph_callees` |
| "How does X reach/become Y? / trace the flow from X to Y" | `codegraph_trace` |
| "What would break if I changed Z?" | `codegraph_impact` |
| "Show me Y's signature / source / docstring" | `codegraph_node` |
| "Give me focused context for a task/area" | `codegraph_context` |
| "See several related symbols' source at once" | `codegraph_explore` |
| "What files exist under path/" | `codegraph_files` |
| "Is the index healthy?" | `codegraph_status` |

### Rules of thumb

- Answer directly. For "how does X work" / architecture questions, use `codegraph_context` first, then one `codegraph_explore` for the source of surfaced symbols.
- For a specific flow, start with `codegraph_trace` from -> to, then one `codegraph_explore` for the relevant bodies.
- Trust codegraph results. Do not re-verify them with grep.
- Do not grep first when looking up a symbol by name. Use `codegraph_search`.
- Do not chain `codegraph_search` + `codegraph_node` when broader context is needed. Use `codegraph_context`.
- Do not loop `codegraph_node` over many symbols. Use one `codegraph_explore`.
- If a codegraph response starts with a staleness banner, read only the pending files listed there for accurate content.

### If `.codegraph/` doesn't exist

Ask the user: "I notice this project doesn't have CodeGraph initialized. Want me to run `codegraph init -i` to build the index?"
<!-- CODEGRAPH_END -->

## Project Memory

1. 一定要优先考虑干净、优雅的代码架构。加代码前先思考这样加是否合理，避免为了快速实现而破坏边界、抽象和可维护性。
2. 加注释时使用中文注释；必要字段可以使用英文表述。常量、配置项等必须加注释，说明用途、含义或取值约束。
3. 当用户说“开始迭代”时，需要先检查当前 diff，在之前版本基础上迭代一个新版本。例如之前是 `v0.7.3`，新版本应为 `v0.7.4`。同时修改版本元数据信息，检查并更新相关文档（不只是 `CHANGELOG.md`），执行编译和测试；确认没问题后提交、打 tag，并 push 当前分支与 tag。
4. 迭代新版本时，`CHANGELOG.md` 和 `CHANGELOG.en.md` 只能新增当前版本记录，严禁修改任何历史版本记录。
