// remove_telegram.js — удаляет Telegram-ноды из n8n-воркфлоу.
//
// Запуск: node remove_telegram.js
// Подключение к Postgres выполняется через docker exec psql (см. README).
const { execFileSync } = require('child_process');

const DB = 'AS_news';
const USER = 'peter';

// Воркфлоу, из которых удаляем Telegram-ноды.
const TARGETS = [
  'Get my YouTube channals',
  'Проверка кук YouTube',
  'Gen my videos',
];

function psql(sql) {
  return execFileSync('docker', [
    'exec', 'postgres', 'psql', '-U', USER, '-d', DB, '-t', '-A', '-c', sql,
  ], { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 });
}

// Возвращает { nodes, connections } в виде объектов.
function loadWorkflow(name) {
  const raw = psql(
    `SELECT nodes::text || E'\\n@@@SPLIT@@@\\n' || connections::text ` +
    `FROM workflow_entity WHERE name = '${name.replace(/'/g, "''")}'`
  );
  const [nodesRaw, connRaw] = raw.split('\n@@@SPLIT@@@\n');
  return {
    nodes: JSON.parse(nodesRaw.trim()),
    connections: JSON.parse(connRaw.trim()),
  };
}

function saveWorkflow(name, nodes, connections) {
  const nodesJson = JSON.stringify(nodes);
  const connJson = JSON.stringify(connections);
  // Экранируем одинарные кавычки для SQL-строки.
  const esc = (s) => s.replace(/'/g, "''");
  const sql =
    `UPDATE workflow_entity SET nodes = '${esc(nodesJson)}'::json, ` +
    `connections = '${esc(connJson)}'::json WHERE name = '${name.replace(/'/g, "''")}'`;
  psql(sql);
  console.log(`saved: ${name}`);
}

for (const name of TARGETS) {
  const { nodes, connections } = loadWorkflow(name);

  const telegramNodes = nodes.filter((n) =>
    String(n.type || '').toLowerCase().includes('telegram')
  );
  if (telegramNodes.length === 0) {
    console.log(`skip (no telegram nodes): ${name}`);
    continue;
  }

  const removedNames = new Set(telegramNodes.map((n) => n.name));
  console.log(`${name}: removing telegram nodes ->`, [...removedNames].join(', '));

  // Убираем Telegram-ноды.
  const keptNodes = nodes.filter((n) => !removedNames.has(n.name));

  // Убираем связи, ведущие к удалённым нодам.
  for (const source of Object.keys(connections)) {
    if (removedNames.has(source)) {
      delete connections[source];
      continue;
    }
    const branches = connections[source].main || [];
    const newBranches = [];
    for (const branch of branches) {
      if (!Array.isArray(branch)) {
        newBranches.push(branch);
        continue;
      }
      const filtered = branch.filter((c) => !removedNames.has(c.node));
      // Пустую ветку удаляем целиком.
      if (filtered.length > 0) newBranches.push(filtered);
    }
    if (newBranches.length === 0) {
      delete connections[source];
    } else {
      connections[source].main = newBranches;
    }
  }

  // Удаляем оси (position) удалённых нод, если n8n их хранит.
  saveWorkflow(name, keptNodes, connections);
}
