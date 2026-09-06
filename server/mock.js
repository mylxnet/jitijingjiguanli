/**
 * 集体台账 · Mock API Server
 * 端口 8080，替代 Go 后端，供前端 vite proxy 使用。
 * 默认测试账号：admin / admin888（统一）
 *
 * 启动:  node server/mock.js
 */
const http = require('http');
const url = require('url');

const TEST_ACCOUNT = { username: 'admin', password: 'admin888' };

// ========== 模拟数据 ==========
// 预置科目与 Go seedPresetCategoriesTx 保持一致：8 个 L1
//   1 本金(preset=1) / 2 长期投资(preset=1,空) / 3 再投资(preset=1,空)
//   4 经营收入(preset=1,2个L2) / 5 投资收益(preset=1,空)
//   6 土地流转费收入(preset=1,空) / 7 流转管理费(preset=1,空) / 8 分配与支出(preset=1,5个L2)
// 空容器按往来单位 type 自动建 L2：
//   type=invest → 长期投资下建同名 L2
//   type=flow → 土地流转费收入下建同名 L2 + 流转管理费下建同名 L2
// 银行存款不是 category，是 SETTINGS.bankBalanceCents
const CATEGORIES = [
  { id: 1, name: '本金', level: 1, kind: 'equity', preset: true, status: 'active', children: [
    { id: 11, name: '上级补助', level: 2, kind: 'equity', parentId: 1, preset: true, status: 'active', balanceCents: 0 },
    { id: 12, name: '待投资', level: 2, kind: 'equity', parentId: 1, preset: true, status: 'active', balanceCents: 0 },
  ]},
  { id: 2, name: '长期投资', level: 1, kind: 'equity', preset: true, status: 'active', children: [] },
  { id: 3, name: '再投资', level: 1, kind: 'equity', preset: true, status: 'active', children: [] },
  { id: 4, name: '经营收入', level: 1, kind: 'equity', preset: true, status: 'active', children: [
    { id: 41, name: '其他财政收入', level: 2, kind: 'equity', parentId: 4, preset: true, status: 'active', balanceCents: 0 },
    { id: 42, name: '其他收入', level: 2, kind: 'equity', parentId: 4, preset: true, status: 'active', balanceCents: 0 },
  ]},
  { id: 5, name: '投资收益', level: 1, kind: 'equity', preset: true, status: 'active', children: [] },
  { id: 6, name: '土地流转费收入', level: 1, kind: 'equity', preset: true, status: 'active', children: [] },
  { id: 7, name: '流转管理费', level: 1, kind: 'equity', preset: true, status: 'active', children: [] },
  { id: 8, name: '分配与支出', level: 1, kind: 'equity', preset: true, status: 'active', children: [
    { id: 81, name: '土地流转费-转付农户', level: 2, kind: 'equity', parentId: 8, preset: true, status: 'active', balanceCents: 0 },
    { id: 82, name: '成员分红', level: 2, kind: 'equity', parentId: 8, preset: true, status: 'active', balanceCents: 0 },
    { id: 83, name: '福利发放', level: 2, kind: 'equity', parentId: 8, preset: true, status: 'active', balanceCents: 0 },
    { id: 84, name: '公益支出', level: 2, kind: 'equity', parentId: 8, preset: true, status: 'active', balanceCents: 0 },
    { id: 85, name: '管理费支出', level: 2, kind: 'equity', parentId: 8, preset: true, status: 'active', balanceCents: 0 },
  ]},
];

const PARTIES = [];

// 再投资去向明细（ReinvestAllocation 子表）
const REINVEST_ALLOCATIONS = [];

// 532 分配记录（按年存储，每年一条）
const DISTRIBUTIONS_532 = [
  // 2026年数据已清空，用于测试未分配流程
];

// 合同附件（Contract）—— 文件内容存 base64 简化 mock；真实后端应存 OSS/本地磁盘
const CONTRACTS = [];

const TRANSACTIONS = [];

const TRANSFERS = [];

// 应收种子数据
const RECEIVABLES = [];

// ========== 流转管理种子数据 ==========
// 流转类型往来单位（土地流转费 + 管理费）
(function initFlowData() {
  // 3 个 flow 类型单位
  const flowParties = [
    { id: 101, name: '绿野种植合作社', types: ['flow'], landMu: 120, landFeePerMuCents: 60000, expectedLandFeeCents: 7200000, mgmtFeePerMuCents: 6000, expectedMgmtFeeCents: 720000 },
    { id: 102, name: '丰源农业公司',     types: ['flow'], landMu: 85,  landFeePerMuCents: 55000, expectedLandFeeCents: 4675000, mgmtFeePerMuCents: 5500, expectedMgmtFeeCents: 467500 },
    { id: 103, name: '金穗家庭农场',     types: ['flow'], landMu: 60,  landFeePerMuCents: 50000, expectedLandFeeCents: 3000000, mgmtFeePerMuCents: 5000, expectedMgmtFeeCents: 300000 },
  ];
  for (const p of flowParties) {
    if (!PARTIES.find(x => x.id === p.id)) {
      PARTIES.push({ ...p, contactPhone: '', note: null, areaMu: p.landMu, createdAt: '2026-01-01', updatedAt: '2026-01-01', outstandingCents: 0, investAmountCents: 0, returnRateBps: 0, expectedReturnCents: 0 });
    }
  }

  // 2026 年度应收种子数据
  const rentRecv = [
    { partyId: 101, partyName: '绿野种植合作社', amountCents: 7200000, paidCents: 4000000, outstandingCents: 3200000, status: 'partial' },
    { partyId: 102, partyName: '丰源农业公司',     amountCents: 4675000, paidCents: 4675000, outstandingCents: 0,       status: 'paid' },
    { partyId: 103, partyName: '金穗家庭农场',     amountCents: 3000000, paidCents: 0,       outstandingCents: 3000000, status: 'open' },
  ];
  const svcRecv = [
    { partyId: 101, partyName: '绿野种植合作社', amountCents: 720000, paidCents: 500000, outstandingCents: 220000, status: 'partial' },
    { partyId: 102, partyName: '丰源农业公司',     amountCents: 467500, paidCents: 467500, outstandingCents: 0,      status: 'paid' },
    { partyId: 103, partyName: '金穗家庭农场',     amountCents: 300000, paidCents: 0,      outstandingCents: 300000, status: 'open' },
  ];
  let nextRecvId = 100;
  for (const r of rentRecv) {
    RECEIVABLES.push({
      id: nextRecvId++, orgId: 1, partyId: r.partyId, partyName: r.partyName,
      recvYear: 2026, kind: 'rent', recvKind: 'rent',
      title: '2026年度土地流转费',
      amountCents: r.amountCents, incomeCategoryId: null,
      status: r.status, note: null, paidCents: r.paidCents, outstandingCents: r.outstandingCents,
      createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-06-01T00:00:00Z',
    });
  }
  for (const r of svcRecv) {
    RECEIVABLES.push({
      id: nextRecvId++, orgId: 1, partyId: r.partyId, partyName: r.partyName,
      recvYear: 2026, kind: 'service', recvKind: 'service',
      title: '2026年度流转管理费',
      amountCents: r.amountCents, incomeCategoryId: null,
      status: r.status, note: null, paidCents: r.paidCents, outstandingCents: r.outstandingCents,
      createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-06-01T00:00:00Z',
    });
  }

  // 转付农户支出（categoryId: 81）
  let nextTxnId = TRANSACTIONS.length ? Math.max(...TRANSACTIONS.map(t => t.id)) + 1 : 100;
  const farmerTxns = [
    { amountCents: 1500000, txnDate: '2026-03-15', note: '一季度土地流转费转付农户' },
    { amountCents: 1200000, txnDate: '2026-06-20', note: '二季度土地流转费转付农户' },
  ];
  for (const t of farmerTxns) {
    TRANSACTIONS.push({
      id: nextTxnId++, orgId: 1, categoryId: 81, categoryName: '土地流转费-转付农户',
      direction: 'expense', amountCents: t.amountCents, txnDate: t.txnDate,
      note: t.note, status: 'normal', partyId: null, partyName: null,
      createdAt: t.txnDate + 'T00:00:00Z', updatedAt: t.txnDate + 'T00:00:00Z',
    });
  }

  // 管理费支出（categoryId: 85）
  const mgmtTxns = [
    { amountCents: 80000,  txnDate: '2026-04-10', note: '管理费支出-办公用品采购' },
    { amountCents: 120000, txnDate: '2026-07-05', note: '管理费支出-人员工资' },
    { amountCents: 50000,  txnDate: '2026-09-01', note: '管理费支出-其他' },
  ];
  for (const t of mgmtTxns) {
    TRANSACTIONS.push({
      id: nextTxnId++, orgId: 1, categoryId: 85, categoryName: '管理费支出',
      direction: 'expense', amountCents: t.amountCents, txnDate: t.txnDate,
      note: t.note, status: 'normal', partyId: null, partyName: null,
      createdAt: t.txnDate + 'T00:00:00Z', updatedAt: t.txnDate + 'T00:00:00Z',
    });
  }
})();

// 把 CATEGORIES 加工成 SummaryPage 期望的 CategorySummary（带 currentBalanceCents / txnCount / incomeCents / expenseCents）
function buildCategorySummary() {
  return CATEGORIES.map(l1 => {
    const l1Children = (l1.children || []).map(l2 => ({
      id: l2.id,
      name: l2.name,
      level: l2.level,
      parentId: l2.parentId,
      kind: l2.kind,
      currentBalanceCents: l2.balanceCents || 0,
      txnCount: Math.floor(Math.random() * 15) + 1,
      incomeCents: Math.max(0, l2.balanceCents || 0),
      expenseCents: Math.max(0, -(l2.balanceCents || 0)),
    }));
    return {
      id: l1.id,
      name: l1.name,
      level: l1.level,
      currentBalanceCents: l1Children.reduce((s, c) => s + c.currentBalanceCents, 0),
      txnCount: l1Children.reduce((s, c) => s + c.txnCount, 0),
      incomeCents: l1Children.reduce((s, c) => s + c.incomeCents, 0),
      expenseCents: l1Children.reduce((s, c) => s + c.expenseCents, 0),
      children: l1Children,
    };
  });
}

const SUMMARY = {
  incomeTotal: 0,
  expenseTotal: 0,
  balance: 0,
  capital: { bankBalanceCents: 0, assetTotalCents: 0, equityTotalCents: 0 },
  categories: buildCategorySummary(),
};

const SETTINGS = {
  bankOpeningBalanceCents: 0,
  // 再投资比例（基点）：0=不自动；5000=50%。核销时前端根据此比例提示再投资
  reinvestRatioBps: 0,
};

// ========== 工具 ==========
function sendJSON(res, status, data) {
  // 对齐 Go platform.OK: { "data": payload }
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Access-Control-Allow-Origin': 'http://localhost:5173',
    'Access-Control-Allow-Credentials': 'true',
    'Access-Control-Allow-Methods': 'GET,POST,PUT,DELETE,OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type',
  });
  res.end(JSON.stringify({ data }));
}

function sendFail(res, status, code, msg) {
  res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8' });
  res.end(JSON.stringify({ error: { code, message: msg } }));
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let b = '';
    req.on('data', c => b += c);
    req.on('end', () => {
      try { resolve(b ? JSON.parse(b) : {}); } catch (e) { reject(e); }
    });
    req.on('error', reject);
  });
}

function parseCookie(req) {
  const h = req.headers['cookie'] || '';
  const parts = h.split(';').map(s => s.trim()).filter(Boolean);
  const out = {};
  for (const p of parts) {
    const i = p.indexOf('=');
    if (i > 0) out[p.slice(0, i)] = p.slice(i + 1);
  }
  return out;
}

// ========== 路由 ==========
const server = http.createServer(async (req, res) => {
  // CORS preflight
  if (req.method === 'OPTIONS') {
    res.writeHead(204, {
      'Access-Control-Allow-Origin': 'http://localhost:5173',
      'Access-Control-Allow-Credentials': 'true',
      'Access-Control-Allow-Methods': 'GET,POST,PUT,DELETE,OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type',
    });
    res.end();
    return;
  }

  const parsed = url.parse(req.url, true);
  const pathname = parsed.pathname;
  const qs = parsed.query;

  console.log(`[${req.method}] ${pathname}  qs=${JSON.stringify(qs)}`);

  try {
    // 健康检查
    if (pathname === '/api/health') return sendJSON(res, 200, { status: 'ok' });

    // 登录
    if (pathname === '/api/auth/login' && req.method === 'POST') {
      const body = await readBody(req);
      const u = body.username || '';
      const p = body.password || '';
      if (u !== TEST_ACCOUNT.username || p !== TEST_ACCOUNT.password) {
        return sendFail(res, 401, 'BAD_CREDENTIALS', '用户名或密码错误');
      }
      res.setHeader('Set-Cookie', 'session=mock_session; Path=/; HttpOnly; SameSite=Lax');
      return sendJSON(res, 200, { user: { username: TEST_ACCOUNT.username } });
    }
    if (pathname === '/api/auth/register' && req.method === 'POST') {
      const body = await readBody(req);
      const u = body.username || '';
      const p = body.password || '';
      if (u !== TEST_ACCOUNT.username || p !== TEST_ACCOUNT.password) {
        return sendFail(res, 401, 'BAD_CREDENTIALS', '测试账号固定为 admin / admin888');
      }
      res.setHeader('Set-Cookie', 'session=mock_session; Path=/; HttpOnly; SameSite=Lax');
      return sendJSON(res, 200, { user: { username: TEST_ACCOUNT.username } });
    }
    if (pathname === '/api/auth/logout' && req.method === 'POST') {
            res.setHeader('Set-Cookie', 'session=; Path=/; Max-Age=0');
      return sendJSON(res, 200, { ok: true });
    }

    // 测试用：清空所有应收记录（放在鉴权之前，方便测试）
    if (pathname === '/api/test/clear-receivables' && req.method === 'POST') {
      RECEIVABLES.length = 0;
      for (const p of PARTIES) p.outstandingCents = 0;
      return sendJSON(res, 200, { ok: true });
    }

    // 鉴权检查
    const cookies = parseCookie(req); if (cookies.session !== 'mock_session') {
      // GET /categories 和 GET /me 在登录前也会被前端调用触发 checkLogin
      if (pathname === '/api/categories' || pathname === '/api/me') {
        return sendFail(res, 401, 'UNAUTHORIZED', '请先登录');
      }
      // 其他直接 fail
      return sendFail(res, 401, 'UNAUTHORIZED', '请先登录');
    }

    // /api/me
    if (pathname === '/api/me') return sendJSON(res, 200, { userID: 1, orgID: 1, orgName: '新庄村' });

    // categories
    if (pathname === '/api/categories') {
      if (req.method === 'GET') return sendJSON(res, 200, CATEGORIES);
      if (req.method === 'POST') {
        const body = await readBody(req);
        // 验证 parentId 存在
        const parent = CATEGORIES.flatMap(l1 => [l1, ...(l1.children || [])]).find(c => c.id === body.parentId);
        if (!parent) return sendJSON(res, 400, { code: 'BAD_REQUEST', message: '父科目不存在' });
        const newId = CATEGORIES.flatMap(l1 => [l1, ...(l1.children || [])]).reduce((m, c) => Math.max(m, c.id), 0) + 1;
        const newCat = {
          id: newId,
          name: body.name,
          level: body.level || (parent.level === 1 ? 2 : 3),
          parentId: parent.id,
          kind: body.kind || 'equity',
          preset: false,
          status: 'active',
          balanceCents: 0,
        };
        // 挂到父 L1 的 children 下
        if (parent.level === 1) {
          parent.children = parent.children || [];
          parent.children.push(newCat);
        }
        return sendJSON(res, 200, newCat);
      }
    }
    if (pathname.startsWith('/api/categories/')) {
      const id = parseInt(pathname.split('/').pop());
      const allCats = CATEGORIES.flatMap(l1 => [l1, ...(l1.children || [])]);
      const cat = allCats.find(c => c.id === id);
      if (!cat) return sendJSON(res, 404, { code: 'NOT_FOUND', message: '科目不存在' });
      if (req.method === 'GET') return sendJSON(res, 200, cat);
      if (req.method === 'PUT') {
        readBody(req).then(body => {
          // preset 保护
          if (body.name !== undefined && cat.preset) return sendJSON(res, 403, { code: 'CATEGORY_PRESET', message: '预置科目不能删除或重命名' });
          if (body.name !== undefined) cat.name = body.name;
          if (body.status !== undefined) cat.status = body.status;
          if (body.openingBalanceCents !== undefined) cat.openingBalanceCents = body.openingBalanceCents;
          return sendJSON(res, 200, cat);
        });
        return;
      }
      if (req.method === 'DELETE') {
        if (cat.preset) return sendJSON(res, 403, { code: 'CATEGORY_PRESET', message: '预置科目不能删除或重命名' });
        // 从父节点移除
        if (cat.level === 2) {
          const parent = CATEGORIES.find(l1 => l1.id === cat.parentId);
          if (parent) parent.children = parent.children.filter(c => c.id !== cat.id);
        } else {
          const idx = CATEGORIES.findIndex(l1 => l1.id === cat.id);
          if (idx >= 0) CATEGORIES.splice(idx, 1);
        }
        return sendJSON(res, 200, { ok: true });
      }
    }

    // transactions
    if (pathname === '/api/transactions') {
      if (req.method === 'GET') {
        let items = [...TRANSACTIONS];

        // 按 categoryId 筛选：如果是 L1，包含其所有子科目
        if (qs.categoryId) {
          const targetId = parseInt(qs.categoryId, 10);
          // 找到 targetId 对应的 L1（如果它本身就是 L1 则直接用；如果是 L2 则找其父 L1）
          let l1 = CATEGORIES.find(x => x.id === targetId);
          if (!l1) {
            // targetId 是 L2，找其父 L1
            for (const c of CATEGORIES) {
              if (c.children?.some(ch => ch.id === targetId)) { l1 = c; break; }
            }
          }
          if (l1) {
            const allowedIds = new Set([l1.id, ...(l1.children || []).map(ch => ch.id)]);
            items = items.filter(t => allowedIds.has(t.categoryId));
          } else {
            items = items.filter(t => t.categoryId === targetId);
          }
        }

        // 按 direction 筛选
        if (qs.direction) items = items.filter(t => t.direction === qs.direction);
        // 按关键字（note/partyName）
        if (qs.q) {
          const q = qs.q.toLowerCase();
          items = items.filter(t => (t.note || '').toLowerCase().includes(q) || (t.partyName || '').toLowerCase().includes(q));
        }
        // 按日期范围
        if (qs.from) items = items.filter(t => t.txnDate >= qs.from);
        if (qs.to) items = items.filter(t => t.txnDate <= qs.to);

        // 按时间倒序
        items.sort((a, b) => (b.txnDate || '').localeCompare(a.txnDate || ''));

        const pageSize = parseInt(qs.pageSize) || 50;
        return sendJSON(res, 200, { items: items.slice(0, pageSize), total: items.length });
      }
      if (req.method === 'POST') {
        const body = await readBody(req);
        const newId = TRANSACTIONS.length ? Math.max(...TRANSACTIONS.map(t => t.id)) + 1 : 1;
        const txn = { id: newId, status: 'normal', ...body };
        TRANSACTIONS.push(txn);
        return sendJSON(res, 200, txn);
      }
    }
    if (pathname.startsWith('/api/transactions/')) {
      if (req.method === 'PUT') return sendJSON(res, 200, { ok: true });
      if (req.method === 'DELETE') return sendJSON(res, 200, { ok: true });
    }

    // transfers
    if (pathname === '/api/transfers') {
      if (req.method === 'GET') return sendJSON(res, 200, { items: TRANSFERS, total: TRANSFERS.length });
    }
    if (pathname.startsWith('/api/transfers/')) {
      if (req.method === 'PUT') return sendJSON(res, 200, { ok: true });
    }

    // summary
    if (pathname === '/api/summary') {
      // 动态构建，确保后续 POST /api/categories 新建的 L2 能被正确汇总
      const txns = TRANSACTIONS.filter(t => t.status !== 'voided');
      const incomeTotal = txns.filter(t => t.direction === 'income').reduce((s, t) => s + t.amountCents, 0);
      const expenseTotal = txns.filter(t => t.direction === 'expense').reduce((s, t) => s + t.amountCents, 0);
      return sendJSON(res, 200, {
        incomeTotal,
        expenseTotal,
        balance: incomeTotal - expenseTotal,
        capital: {
          bankBalanceCents: SETTINGS.bankOpeningBalanceCents + incomeTotal - expenseTotal,
          assetTotalCents: 0,
          equityTotalCents: SETTINGS.bankOpeningBalanceCents + incomeTotal - expenseTotal,
        },
        categories: buildCategorySummary(),
      });
    }

    // parties
    if (pathname === '/api/parties') {
      if (req.method === 'GET') {
        // 兼容层：确保每个 party 同时有 types 数组（新）和 type 字符串（旧，兼容旧前端）
        const normalized = PARTIES.map(p => {
          const types = (p.types && Array.isArray(p.types)) ? p.types : (p.type && typeof p.type === 'string') ? [p.type] : [];
          const type = p.type || types[0] || null;
          return { ...p, types, type };
        });
        return sendJSON(res, 200, normalized);
      }
      if (req.method === 'POST') {
        const body = await readBody(req);
        const newId = PARTIES.length ? Math.max(...PARTIES.map(p => p.id)) + 1 : 1;
        let types = body.types;
        if (!types && body.type) types = [body.type];
        if (!Array.isArray(types)) types = [];
        const now = new Date().toISOString().slice(0, 10);
        const p = {
          id: newId,
          name: body.name,
          types,
          contactPhone: body.contactPhone || '',
          areaMu: body.areaMu || 0,
          note: body.note || null,
          createdAt: now,
          updatedAt: now,
          outstandingCents: body.outstandingCents || 0,
          // 投资/再投资字段
          investAmountCents: body.investAmountCents || 0,
          returnRateBps: body.returnRateBps || 0,
          expectedReturnCents: body.expectedReturnCents || 0,
          // 土地流转字段
          landMu: body.landMu || 0,
          landFeePerMuCents: body.landFeePerMuCents || 0,
          expectedLandFeeCents: body.expectedLandFeeCents || 0,
          mgmtFeePerMuCents: body.mgmtFeePerMuCents || 0,
          expectedMgmtFeeCents: body.expectedMgmtFeeCents || 0,
        };
        PARTIES.push(p);
        return sendJSON(res, 200, { ...p, type: types[0] || null });
      }
    }
    // PUT /api/parties/:id - 更新往来单位（如 outstandingCents 欠款减少）
    const partyMatch = pathname.match(/^\/api\/parties\/(\d+)$/);
    if (partyMatch && req.method === 'PUT') {
      const body = await readBody(req);
      const id = parseInt(partyMatch[1], 10);
      const p = PARTIES.find(x => x.id === id);
      if (p) {
        Object.assign(p, body);
        return sendJSON(res, 200, p);
      }
      return sendJSON(res, 404, { code: 'NOT_FOUND', message: '往来单位不存在' });
    }

    // reinvest allocations - GET /api/parties/:id/allocations, POST /api/parties/:id/allocations, DELETE /api/allocations/:id
    const allocMatch = pathname.match(/^\/api\/parties\/(\d+)\/allocations$/);
    if (allocMatch) {
      const partyId = parseInt(allocMatch[1], 10);
      if (req.method === 'GET') {
        const list = REINVEST_ALLOCATIONS.filter(a => a.partyId === partyId);
        return sendJSON(res, 200, list);
      }
      if (req.method === 'POST') {
        const body = await readBody(req);
        const newId = REINVEST_ALLOCATIONS.length ? Math.max(...REINVEST_ALLOCATIONS.map(a => a.id)) + 1 : 1;
        const alloc = {
          id: newId, partyId,
          targetName: body.targetName || '',
          amountCents: body.amountCents || 0,
          notes: body.notes || null,
          createdAt: new Date().toISOString().slice(0, 10),
        };
        REINVEST_ALLOCATIONS.push(alloc);
        return sendJSON(res, 200, alloc);
      }
    }
    const delAllocMatch = pathname.match(/^\/api\/allocations\/(\d+)$/);
    if (delAllocMatch && req.method === 'DELETE') {
      const id = parseInt(delAllocMatch[1], 10);
      const idx = REINVEST_ALLOCATIONS.findIndex(a => a.id === id);
      if (idx >= 0) { REINVEST_ALLOCATIONS.splice(idx, 1); return sendJSON(res, 200, { ok: true }); }
      return sendJSON(res, 404, { code: 'NOT_FOUND', message: '再投资去向不存在' });
    }

    // 合同附件
    // GET /api/contracts?partyId=1 → 列表（不含文件内容）
    if (pathname === '/api/contracts' && req.method === 'GET') {
      const q = url.parse(req.url, true).query;
      const partyId = q.partyId ? parseInt(q.partyId, 10) : null;
      let list = CONTRACTS.map(c => ({ ...c, fileData: undefined }));
      if (partyId) list = list.filter(c => c.partyId === partyId);
      return sendJSON(res, 200, list);
    }
    // GET /api/contracts/:id → 含 fileData 的完整对象（下载/预览用）
    const getContractMatch = pathname.match(/^\/api\/contracts\/(\d+)$/);
    if (getContractMatch && req.method === 'GET') {
      const id = parseInt(getContractMatch[1], 10);
      const c = CONTRACTS.find(x => x.id === id);
      if (!c) return sendJSON(res, 404, { code: 'NOT_FOUND', message: '合同不存在' });
      return sendJSON(res, 200, c);
    }
    // POST /api/contracts → 上传
    if (pathname === '/api/contracts' && req.method === 'POST') {
      const body = await readBody(req);
      const partyId = body.partyId;
      if (!partyId) return sendFail(res, 400, 'BAD_REQUEST', '缺少 partyId');
      const newId = CONTRACTS.length ? Math.max(...CONTRACTS.map(c => c.id)) + 1 : 1;
      const contract = {
        id: newId,
        partyId: Number(partyId),
        fileName: body.fileName || '未命名文件',
        fileSize: body.fileSize || 0,
        mimeType: body.mimeType || 'application/octet-stream',
        contractTitle: body.contractTitle || body.fileName || '',
        contractDate: body.contractDate || null,
        expiresAt: body.expiresAt || null,
        fileData: body.fileData || null, // base64 data URL
        createdAt: new Date().toISOString(),
      };
      CONTRACTS.push(contract);
      // 列表接口不返回文件体
      const safe = { ...contract, fileData: undefined };
      return sendJSON(res, 200, safe);
    }
    // DELETE /api/contracts/:id
    const delContractMatch = pathname.match(/^\/api\/contracts\/(\d+)$/);
    if (delContractMatch && req.method === 'DELETE') {
      const id = parseInt(delContractMatch[1], 10);
      const idx = CONTRACTS.findIndex(c => c.id === id);
      if (idx >= 0) { CONTRACTS.splice(idx, 1); return sendJSON(res, 200, { ok: true }); }
      return sendJSON(res, 404, { code: 'NOT_FOUND', message: '合同不存在' });
    }

    // receivables
    if (pathname === '/api/receivables') {
      if (req.method === 'GET') return sendJSON(res, 200, { items: RECEIVABLES, total: RECEIVABLES.length });
    }
    // receipts（核销记录）
    const receiptMatch = pathname.match(/^\/api\/receivables\/(\d+)\/receipts$/);
    if (receiptMatch) {
      const receivableId = parseInt(receiptMatch[1], 10);
      const r = RECEIVABLES.find(x => x.id === receivableId);
      if (!r) return sendFail(res, 404, 'NOT_FOUND', '应收单不存在');
      if (req.method === 'GET') {
        return sendJSON(res, 200, (r.receipts || []).filter(x => x.status !== 'voided'));
      }
      if (req.method === 'POST') {
        const body = await readBody(req);
        const amt = body.amountCents || 0;
        if (amt <= 0) return sendFail(res, 400, 'BAD_REQUEST', '金额必须大于 0');
        if (amt > r.outstandingCents) return sendFail(res, 400, 'BAD_REQUEST', '核销金额超过未收金额');
        // 累积 receipts 数组
        if (!r.receipts) r.receipts = [];
        const nextId = r.receipts.length ? Math.max(...r.receipts.map(x => x.id)) + 1 : 1;
        const receipt = {
          id: nextId,
          receivableId,
          amountCents: amt,
          receiptDate: body.receiptDate || new Date().toISOString().slice(0, 10),
          method: body.method === 'offset' ? 'offset' : 'cash',
          txnId: body.txnId || null,
          note: body.note || null,
          status: 'normal',
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        };
        r.receipts.push(receipt);
        r.paidCents = (r.paidCents || 0) + amt;
        r.outstandingCents = r.amountCents - r.paidCents;
        if (r.outstandingCents <= 0) { r.status = 'closed'; r.outstandingCents = 0; }
        r.updatedAt = new Date().toISOString();
        // 同步更新 PARTIES 的 outstandingCents
        const p = PARTIES.find(x => x.id === r.partyId);
        if (p) {
          p.outstandingCents = RECEIVABLES
            .filter(x => x.partyId === r.partyId)
            .reduce((s, x) => s + (x.outstandingCents || 0), 0);
        }
        return sendJSON(res, 200, receipt);
      }
    }
    // void receipt: DELETE /api/receivables/:id/receipts/:rid
    const voidReceiptMatch = pathname.match(/^\/api\/receivables\/(\d+)\/receipts\/(\d+)$/);
    if (voidReceiptMatch && req.method === 'DELETE') {
      const receivableId = parseInt(voidReceiptMatch[1], 10);
      const rid = parseInt(voidReceiptMatch[2], 10);
      const r = RECEIVABLES.find(x => x.id === receivableId);
      if (!r || !r.receipts) return sendFail(res, 404, 'NOT_FOUND', '记录不存在');
      const rc = r.receipts.find(x => x.id === rid);
      if (!rc) return sendFail(res, 404, 'NOT_FOUND', '核销记录不存在');
      rc.status = 'voided';
      rc.updatedAt = new Date().toISOString();
      // 回退金额
      r.paidCents = Math.max(0, (r.paidCents || 0) - rc.amountCents);
      r.outstandingCents = Math.max(0, r.amountCents - r.paidCents);
      if (r.outstandingCents > 0) r.status = 'open';
      r.updatedAt = new Date().toISOString();
      const p = PARTIES.find(x => x.id === r.partyId);
      if (p) {
        p.outstandingCents = RECEIVABLES
          .filter(x => x.partyId === r.partyId)
          .reduce((s, x) => s + (x.outstandingCents || 0), 0);
      }
      return sendJSON(res, 200, { ok: true });
    }

    // settings
    if (pathname === '/api/settings') {
      if (req.method === 'GET') return sendJSON(res, 200, SETTINGS);
      if (req.method === 'PUT') {
        readBody(req).then(body => {
          if (body.bankOpeningBalanceCents !== undefined) SETTINGS.bankOpeningBalanceCents = body.bankOpeningBalanceCents;
          if (body.reinvestRatioBps !== undefined) SETTINGS.reinvestRatioBps = Math.max(0, Math.min(10000, body.reinvestRatioBps | 0));
          return sendJSON(res, 200, SETTINGS);
        });
        return;
      }
    }

    // changelog
    if (pathname === '/api/changelog') return sendJSON(res, 200, []);

    // backups
    if (pathname === '/api/backups') {
      if (req.method === 'GET') return sendJSON(res, 200, { items: [] });
      if (req.method === 'POST') return sendJSON(res, 200, { name: 'mock-backup.db' });
    }

    // export
    if (pathname === '/api/export') return sendJSON(res, 200, { url: '' });

    // recv-standards 年度结转
    // GET /api/recv-standards → 年度标准列表（从 PARTIES 动态推导，不另存表）
    if (pathname === '/api/recv-standards' && req.method === 'GET') {
      const stds = [];
      for (const p of PARTIES) {
        // dividend：invest/reinvest 类型的 expectedReturnCents
        if ((p.types || []).includes('invest') || (p.types || []).includes('reinvest')) {
          if (p.expectedReturnCents > 0) stds.push({ id: 'd-' + p.id, partyId: p.id, partyName: p.name, recvKind: 'dividend', amountCents: p.expectedReturnCents });
        }
        // rent：flow 类型的 expectedLandFeeCents
        if ((p.types || []).includes('flow')) {
          if (p.expectedLandFeeCents > 0) stds.push({ id: 'r-' + p.id, partyId: p.id, partyName: p.name, recvKind: 'rent', amountCents: p.expectedLandFeeCents });
          if (p.expectedMgmtFeeCents > 0) stds.push({ id: 's-' + p.id, partyId: p.id, partyName: p.name, recvKind: 'service', amountCents: p.expectedMgmtFeeCents });
        }
      }
      return sendJSON(res, 200, stds);
    }
    // GET /api/recv-standards/preview?year=2026
    if (pathname.startsWith('/api/recv-standards/preview') && req.method === 'GET') {
      const q = url.parse(req.url, true).query;
      const year = parseInt(q.year || '0', 10);
      if (!year) return sendFail(res, 400, 'BAD_REQUEST', '缺少年份');
      // 已存在的 receivables: 按 partyId + recvKind + recvYear 匹配
      const existingKeys = new Set(RECEIVABLES.filter(r => r.recvYear === year).map(r => r.partyId + ':' + r.recvKind));
      const items = [];
      for (const p of PARTIES) {
        const types = p.types && p.types.length ? p.types : (p.type ? [p.type] : []);
        if (types.includes('invest') || types.includes('reinvest')) {
          if (p.expectedReturnCents > 0) {
            items.push({ partyId: p.id, partyName: p.name, kind: 'dividend', amountCents: p.expectedReturnCents, exists: existingKeys.has(p.id + ':dividend') });
          }
        }
        if (types.includes('flow')) {
          if (p.expectedLandFeeCents > 0) {
            items.push({ partyId: p.id, partyName: p.name, kind: 'rent', amountCents: p.expectedLandFeeCents, exists: existingKeys.has(p.id + ':rent') });
          }
          if (p.expectedMgmtFeeCents > 0) {
            items.push({ partyId: p.id, partyName: p.name, kind: 'service', amountCents: p.expectedMgmtFeeCents, exists: existingKeys.has(p.id + ':service') });
          }
        }
      }
      return sendJSON(res, 200, { year, items });
    }
    // POST /api/recv-standards/accrue → 确认结转，body: { year, items?: [{ partyId, kind, amountCents }] }
    if (pathname.startsWith('/api/recv-standards/accrue') && req.method === 'POST') {
      const body = await readBody(req);
      const year = parseInt(body.year || '0', 10);
      if (!year) return sendFail(res, 400, 'BAD_REQUEST', '缺少年份');
      const customItems = body.items;
      const existingKeys = new Set(RECEIVABLES.filter(r => r.recvYear === year).map(r => r.partyId + ':' + r.recvKind));
      let created = 0, skipped = 0;
      let nextId = RECEIVABLES.length ? Math.max(...RECEIVABLES.map(r => r.id)) + 1 : 1;
      const addRecv = (p, kind, amountCents) => {
        const key = p.id + ':' + kind;
        if (existingKeys.has(key)) { skipped++; return; }
        RECEIVABLES.push({
          id: nextId++, orgId: 1, partyId: p.id, partyName: p.name,
          recvYear: year, kind, recvKind: kind,
          title: year + '年度' + (kind === 'rent' ? '土地流转费' : kind === 'dividend' ? '投资收益' : kind === 'service' ? '流转管理费' : '应收'),
          amountCents, incomeCategoryId: null,
          status: 'open', note: null,
          createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
          paidCents: 0, outstandingCents: amountCents,
        });
        p.outstandingCents = RECEIVABLES
          .filter(x => x.partyId === p.id)
          .reduce((s, x) => s + (x.outstandingCents || 0), 0);
        created++;
      };
      if (customItems && customItems.length > 0) {
        for (const item of customItems) {
          const p = PARTIES.find(x => x.id === item.partyId);
          if (p) addRecv(p, item.kind, item.amountCents);
        }
      } else {
        for (const p of PARTIES) {
          const types = p.types && p.types.length ? p.types : (p.type ? [p.type] : []);
          if ((types.includes('invest') || types.includes('reinvest')) && p.expectedReturnCents > 0) {
            addRecv(p, 'dividend', p.expectedReturnCents);
          }
          if (types.includes('flow')) {
            if (p.expectedLandFeeCents > 0) addRecv(p, 'rent', p.expectedLandFeeCents);
            if (p.expectedMgmtFeeCents > 0) addRecv(p, 'service', p.expectedMgmtFeeCents);
          }
        }
      }
      return sendJSON(res, 200, { created, skipped });
    }
    if (pathname.startsWith('/api/fund-moves')) return sendJSON(res, 200, []);

    // GET /api/reinvest-allocations
    if (pathname === '/api/reinvest-allocations' && req.method === 'GET') {
      return sendJSON(res, 200, REINVEST_ALLOCATIONS);
    }

    // GET /api/distributions-532?year=2026
    if (pathname === '/api/distributions-532' && req.method === 'GET') {
      const params = url.parse(req.url, true).query;
      const year = params.year ? parseInt(params.year, 10) : null;
      if (year) {
        const dist = DISTRIBUTIONS_532.find(d => d.year === year);
        return sendJSON(res, 200, dist || null);
      }
      return sendJSON(res, 200, DISTRIBUTIONS_532);
    }
    // POST /api/distributions-532 — 创建/更新某年分配
    if (pathname === '/api/distributions-532' && req.method === 'POST') {
      const body = await readBody(req);
      const year = parseInt(body.year, 10);
      if (!year) return sendFail(res, 400, 'BAD_REQUEST', '缺少年份');
      const existing = DISTRIBUTIONS_532.findIndex(d => d.year === year);
      const dist = {
        id: existing >= 0 ? DISTRIBUTIONS_532[existing].id : (DISTRIBUTIONS_532.length ? Math.max(...DISTRIBUTIONS_532.map(d => d.id)) + 1 : 1),
        year,
        totalIncomeCents: body.totalIncomeCents || 0,
        reinvestCents: body.reinvestCents || 0,
        dividendCents: body.dividendCents || 0,
        welfareCents: body.welfareCents || 0,
        createdAt: existing >= 0 ? DISTRIBUTIONS_532[existing].createdAt : new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      if (existing >= 0) {
        DISTRIBUTIONS_532[existing] = dist;
      } else {
        DISTRIBUTIONS_532.push(dist);
      }
      return sendJSON(res, 200, dist);
    }

    // fallback 404
    sendFail(res, 404, 'NOT_FOUND', 'mock 未实现的路径: ' + pathname);
  } catch (e) {
    console.error('mock err:', e);
    sendFail(res, 500, 'INTERNAL', String(e.message || e));
  }
});

// 被 require 时导出数据，直接运行时启 server
if (require.main === module) {
  const PORT = parseInt(process.env.PORT, 10) || 8080;
  server.listen(PORT, () => {
    console.log(`🟢 Mock API Server 运行中: http://localhost:${PORT}`);
    console.log(`   测试账号: ${TEST_ACCOUNT.username} / ${TEST_ACCOUNT.password}`);
  });
} else {
  module.exports = { CATEGORIES, PARTIES, TRANSACTIONS, TRANSFERS, RECEIVABLES, REINVEST_ALLOCATIONS, CONTRACTS, SUMMARY, SETTINGS, buildCategorySummary, server };
}




