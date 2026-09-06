/**
 * 端到端测试：spawn 启动 mock server → 登录 → GET /api/summary → 验证 → kill。
 * 这样避免了 require mock.js 的 server 在 before/after 中行为不可控的问题。
 * 跑测试:  node --test server/tests/mock-e2e.test.js
 */
const { test, before, after } = require('node:test');
const assert = require('node:assert/strict');
const http = require('http');
const { spawn } = require('child_process');
const path = require('path');

const PORT = 18081;
let child;
let cookieJar = '';

// 简单轮询等 server 就绪
async function waitServer(timeoutMs = 5000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      await new Promise((resolve, reject) => {
        http.get({ hostname: 'localhost', port: PORT, path: '/api/health' }, res => {
          res.resume();
          res.statusCode === 200 ? resolve() : reject(new Error('bad status'));
        }).on('error', reject);
      });
      return;
    } catch { await new Promise(r => setTimeout(r, 100)); }
  }
  throw new Error(`mock server 在 ${timeoutMs}ms 内未就绪`);
}

function request(method, path, body) {
  return new Promise((resolve, reject) => {
    const headers = { 'Content-Type': 'application/json' };
    if (cookieJar) headers['Cookie'] = cookieJar;
    const req = http.request({ hostname: 'localhost', port: PORT, path, method, headers }, res => {
      const chunks = [];
      res.on('data', c => chunks.push(c));
      res.on('end', () => {
        const raw = Buffer.concat(chunks).toString('utf8');
        const sc = res.headers['set-cookie'];
        if (sc && sc.length > 0) {
          const m = sc[0].match(/^([^=]+=[^;]+)/);
          if (m) cookieJar = m[1];
        }
        let parsed = null;
        try { parsed = JSON.parse(raw); } catch {}
        resolve({ status: res.statusCode, data: parsed, raw });
      });
    });
    req.on('error', reject);
    if (body) req.write(JSON.stringify(body));
    req.end();
  });
}

before(async () => {
  // 启动 mock.js，传 PORT 作为 argv[2]
  child = spawn(process.execPath, [path.join(__dirname, '..', 'mock.js')], {
    stdio: ['pipe', 'pipe', 'pipe'],
    env: { ...process.env, PORT },
  });
  child.stdout.on('data', () => {});  // 静默启动日志
  child.stderr.on('data', d => process.stderr.write(d));
  await waitServer(6000);
});

after(async () => {
  if (child && !child.killed) {
    child.kill('SIGKILL');
    await new Promise(r => child.on('exit', r));
  }
});

// ---- 登录前置 ----
test('POST /api/auth/login 返回 200 + Set-Cookie', async () => {
  const res = await request('POST', '/api/auth/login', { username: 'e2e_user', password: 'any' });
  assert.equal(res.status, 200);
  assert.ok(res.data.data.user.username.includes('e2e_user'));
  assert.ok(cookieJar.includes('session=mock_session'), `应收到 session cookie，实际: ${cookieJar}`);
});

// ---- 鉴权保护 ----
test('未登录访问 /api/summary 返回 401', async () => {
  const saved = cookieJar; cookieJar = '';
  try {
    const res = await request('GET', '/api/summary');
    assert.equal(res.status, 401);
  } finally { cookieJar = saved; }
});

// ---- 核心：/api/summary 响应结构完整 ----
test('GET /api/summary 响应包含全部 SummaryResponse 字段', async () => {
  const res = await request('GET', '/api/summary');
  assert.equal(res.status, 200);
  assert.ok('data' in res.data);
  const d = res.data.data;

  for (const key of ['incomeTotal', 'expenseTotal', 'balance', 'capital', 'categories']) {
    assert.ok(key in d, `缺少顶层字段: ${key}`);
  }
  for (const key of ['bankBalanceCents', 'assetTotalCents', 'equityTotalCents']) {
    assert.ok(key in d.capital, `capital 缺少: ${key}`);
  }
  assert.ok(Array.isArray(d.categories), 'categories 应为数组');
  assert.ok(d.categories.length > 0, 'categories 不应为空');

  const firstL2 = d.categories[0]?.children?.[0];
  assert.ok(firstL2, '应有至少一个二级科目');
  assert.ok('kind' in firstL2, `L2 ${firstL2.name} 缺少 kind`);
  assert.ok('txnCount' in firstL2);
  assert.ok('currentBalanceCents' in firstL2);
});

test('带 query 参数的 GET /api/summary 正常返回', async () => {
  const res = await request('GET', '/api/summary?from=2026-01-01&to=2026-09-04');
  assert.equal(res.status, 200);
  assert.ok(Array.isArray(res.data.data.categories));
});

// ---- balance 正确性 ----
test('GET /api/summary balance = incomeTotal - expenseTotal', async () => {
  const res = await request('GET', '/api/summary');
  const d = res.data.data;
  assert.equal(d.balance, d.incomeTotal - d.expenseTotal);
});
