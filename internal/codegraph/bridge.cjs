// 通过官方 SDK 扫描与查询；打开索引前读取同版本 schema 常量，避免隐式迁移。
const fs = require('node:fs');
const path = require('node:path');
const { DatabaseSync } = require('node:sqlite');
const { CodeGraph, setLogger, silentLogger } = require(path.join(process.argv[2], 'lib/dist/index.js'));
const { CURRENT_SCHEMA_VERSION } = require(path.join(process.argv[2], 'lib/dist/db/migrations.js'));
setLogger(silentLogger);

function checkSchema(root) {
  const database = path.join(root, '.codegraph', 'codegraph.db');
  if (!fs.existsSync(database)) return;
  const db = new DatabaseSync(database, {readOnly:true});
  try {
    const row = db.prepare('SELECT MAX(version) AS version FROM schema_versions').get();
    if (row.version !== CURRENT_SCHEMA_VERSION) {
      throw new Error(`CodeGraph index schema ${row.version} differs from selected runtime schema ${CURRENT_SCHEMA_VERSION}; rebuild the shared index with the selected CodeGraph version before running skills-seed`);
    }
  } finally { db.close(); }
}

async function main() {
  const request = JSON.parse(fs.readFileSync(0, 'utf8'));
  const root = process.cwd();
  checkSchema(root);
  let graph;
  try {
    if (request.action === 'scan') {
      const allowed = new Set(request.paths);
      graph = CodeGraph.isInitialized(root) ? await CodeGraph.open(root, {sync:false}) : await CodeGraph.init(root, {index:false});
      // 共享标准索引时只筛选本轮快照，不删除其他工具已索引的文件。
      const result = await graph.sync({paths:request.paths});
      if (result.failedFilePaths?.length) throw new Error(`CodeGraph could not parse: ${result.failedFilePaths.join(', ')}`);
      const files = graph.getFiles().filter(file => allowed.has(file.path));
      const failed = files.filter(file => file.errors?.length);
      if (failed.length) throw new Error(`CodeGraph extraction errors: ${failed.map(file => file.path).join(', ')}`);
      const indexed = new Set(files.map(file => file.path));
      const nodes = graph.getNodesInFiles(files.map(file => file.path));
      const nodeIds = new Set(nodes.map(node => node.id));
      const callable = nodes.filter(node => node.kind === 'function' || node.kind === 'method' || node.kind === 'route');
      const parents = graph.getIncomingEdgesTo(callable.map(node => node.id), ['contains']);
      const parentNodes = graph.getNodesByIds(parents.map(edge => edge.source));
      const declarations = new Set(parents.filter(edge => ['interface', 'trait', 'protocol'].includes(parentNodes.get(edge.source)?.kind)).map(edge => edge.target));
      const calls = new Map();
      for (const edge of graph.getOutgoingEdgesFrom(callable.map(node => node.id), ['calls'])) {
        if (nodeIds.has(edge.target)) calls.set(`${edge.source}\0${edge.target}`, {source:edge.source, target:edge.target});
      }
      return {
        files: files.map(({path, language, contentHash}) => ({path, language, contentHash})).sort((a,b) => a.path.localeCompare(b.path)),
        nodes: nodes.map(({id, name, kind, filePath, startLine, endLine, signature, isExported, isAbstract}) => ({id, name, kind, filePath, startLine, endLine, signature, isExported, declarationOnly: isAbstract || declarations.has(id)})).sort((a,b) => a.filePath.localeCompare(b.filePath) || a.startLine-b.startLine || a.id.localeCompare(b.id)),
        calls: [...calls.values()].sort((a,b) => a.source.localeCompare(b.source) || a.target.localeCompare(b.target)),
        dependencies: graph.getFileDependencyPairs().filter(pair => indexed.has(pair.source) && indexed.has(pair.target)).sort((a,b) => a.source.localeCompare(b.source) || a.target.localeCompare(b.target)),
        unindexed: request.paths.filter(file => !indexed.has(file)).sort()
      };
    }
    if (!CodeGraph.isInitialized(root)) throw new Error('Run skills-seed learn current to build the local index first');
    graph = await CodeGraph.open(root, {sync:false, readOnly:true});
    const [query, target] = request.args || [];
    switch (request.action) {
      case 'query': return graph.searchNodes(query, {limit:30});
      case 'node': return graph.getNode(query);
      case 'callers': return graph.getCallers(query, 2);
      case 'callees': return graph.getCallees(query, 2);
      case 'impact': return graph.getImpactRadius(query, 2);
      case 'trace': return graph.findPath(query, target, ['calls']);
      default: throw new Error(`Unsupported query: ${request.action}`);
    }
  } finally { graph?.close(); }
}
main().then(result => process.stdout.write(JSON.stringify(result)+'\n')).catch(error => { console.error(error.message); process.exitCode=1; });
