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

// 备份数据存储
const BACKUPS = [];

// 操作日志
const OPERATION_LOGS = [];
let nextOpLogId = 1;

// 资金划转记录
const FUND_MOVES = [];

// 单位类型 → L1 科目映射
const PARTY_TYPE_L1_MAP = {
  invest: [{ name: '长期投资', id: 2 }],
  reinvest: [{ name: '再投资', id: 3 }],
  flow: [{ name: '土地流转费收入', id: 6 }, { name: '流转管理费', id: 7 }],
};

// 根据单位类型自动创建 L2 科目
function ensurePartyL2Categories(party) {
  const types = party.types && party.types.length ? party.types : (party.type ? [party.type] : []);
  const created = [];
  for (const t of types) {
    const l1s = PARTY_TYPE_L1_MAP[t];
    if (!l1s) continue;
    for (const l1Ref of l1s) {
      const l1 = CATEGORIES.find(c => c.id === l1Ref.id);
      if (!l1) continue;
      if (!l1.children) l1.children = [];
      const existing = l1.children.find(c => c.name === party.name);
      if (existing) {
        created.push(existing);
      } else {
        const newId = CATEGORIES.flatMap(c => [c, ...(c.children || [])]).reduce((m, c) => Math.max(m, c.id), 0) + 1;
        const newCat = {
          id: newId,
          name: party.name,
          level: 2,
          parentId: l1.id,
          kind: 'equity',
          preset: false,
          status: 'active',
          balanceCents: 0,
        };
        l1.children.push(newCat);
        created.push(newCat);
      }
    }
  }
  return created;
}

// 查找单位对应的 L2 科目
function findPartyL2Categories(party) {
  const types = party.types && party.types.length ? party.types : (party.type ? [party.type] : []);
  const result = [];
  for (const t of types) {
    const l1s = PARTY_TYPE_L1_MAP[t];
    if (!l1s) continue;
    for (const l1Ref of l1s) {
      const l1 = CATEGORIES.find(c => c.id === l1Ref.id);
      if (!l1 || !l1.children) continue;
      const cat = l1.children.find(c => c.name === party.name);
      if (cat) result.push(cat);
    }
  }
  return result;
}

// 单位名称模糊查重
function findDuplicateParties(name, type, excludeId) {
  if (!name || name.length < 2) return [];
  const q = name.toLowerCase();
  return PARTIES.filter(p => {
    if (excludeId && p.id === excludeId) return false;
    // 名称 + 类型同时匹配才判重
    const pTypes = (p.types && p.types.length ? p.types : (p.type ? [p.type] : []));
    const typeMatch = type ? pTypes.includes(type) : true;
    if (!typeMatch) return false;
    return p.name.toLowerCase().includes(q) || q.includes(p.name.toLowerCase());
  });
}

function recordOp(operation, summary, effects) {
  const now = Date.now();
  OPERATION_LOGS.push({
    id: nextOpLogId++,
    time: new Date().toISOString(),
    operation,
    summary,
    effects: effects || [],
  });
  // 清理超过 48 小时的记录
  const cutoff = now - 48 * 60 * 60 * 1000;
  while (OPERATION_LOGS.length > 0 && new Date(OPERATION_LOGS[0].time).getTime() < cutoff) {
    OPERATION_LOGS.shift();
  }
}

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

  // 创建 L2 子科目：流转管理费（category 7）下为每个 flow 单位创建同名 L2
  const mgmtFeeCat = CATEGORIES.find(c => c.id === 7);
  if (mgmtFeeCat && mgmtFeeCat.children.length === 0) {
    for (const p of flowParties) {
      mgmtFeeCat.children.push({
        id: 700 + p.id, name: p.name, level: 2, kind: 'equity', parentId: 7,
        preset: false, status: 'active', balanceCents: 0,
      });
    }
  }

  // 管理费已收金额转为收入流水（categoryId 7 的子科目）
  let nextTxnId = TRANSACTIONS.length ? Math.max(...TRANSACTIONS.map(t => t.id)) + 1 : 100;
  const svcL2 = CATEGORIES.find(c => c.id === 7)?.children || [];
  for (const r of svcRecv) {
    if (r.paidCents > 0) {
      const party = flowParties.find(p => p.id === r.partyId);
      const l2 = svcL2.find(c => c.name === (party?.name || ''));
      if (l2) {
        l2.balanceCents += r.paidCents;
      }
      TRANSACTIONS.push({
        id: nextTxnId++, orgId: 1, categoryId: l2?.id || 7, categoryName: (flowParties.find(p => p.id === r.partyId)?.name || '') + '-管理费',
        direction: 'income', amountCents: r.paidCents, txnDate: '2026-06-30',
        note: '收到' + (flowParties.find(p => p.id === r.partyId)?.name || '') + '2026年度流转管理费',
        status: 'normal', partyId: r.partyId, partyName: flowParties.find(p => p.id === r.partyId)?.name || '',
        createdAt: '2026-06-30T00:00:00Z', updatedAt: '2026-06-30T00:00:00Z',
      });
    }
  }

  // 转付农户支出（categoryId: 81）
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
    const chunks = [];
    req.on('data', c => chunks.push(c));
    req.on('end', () => {
      const b = Buffer.concat(chunks).toString('utf8');
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
	      recordOp('登录', '用户 ' + u + ' 登录系统', []);
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
	      recordOp('注册组织', '注册组织 ' + body.orgName, [{ entity: '组织', desc: '创建组织「' + (body.orgName || '') + '」' }]);
	      return sendJSON(res, 200, { user: { username: TEST_ACCOUNT.username } });
	    }
	    if (pathname === '/api/auth/logout' && req.method === 'POST') {
	            res.setHeader('Set-Cookie', 'session=; Path=/; Max-Age=0');
	      recordOp('登出', '用户登出', []);
	      return sendJSON(res, 200, { ok: true });
	    }

    // 测试用：清空所有应收记录（放在鉴权之前，方便测试）
	    if (pathname === '/api/test/clear-receivables' && req.method === 'POST') {
	      const oldCount = RECEIVABLES.length;
	      RECEIVABLES.length = 0;
	      for (const p of PARTIES) p.outstandingCents = 0;
	      recordOp('清空测试数据', '清空 ' + oldCount + ' 条应收记录', [{ entity: '应收单', desc: '清空全部 ' + oldCount + ' 条应收记录' }]);
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
	        recordOp('创建科目', '创建科目「' + newCat.name + '」', [
	          { entity: '科目', entityId: newCat.id, field: 'id', newValue: String(newCat.id), desc: '创建科目 ' + newCat.id },
	          { entity: '科目', entityId: newCat.id, field: '名称', newValue: newCat.name, desc: '科目名称: ' + newCat.name },
	          { entity: '科目', entityId: newCat.id, field: '类型', newValue: newCat.kind, desc: '科目类型: ' + (newCat.kind === 'asset' ? '资产' : '权益') },
	        ]);
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
	          const effects = [];
	          if (body.name !== undefined) {
	            effects.push({ entity: '科目', entityId: cat.id, field: '名称', oldValue: cat.name, newValue: body.name, desc: '名称: ' + cat.name + ' → ' + body.name });
	            cat.name = body.name;
	          }
	          if (body.status !== undefined) {
	            effects.push({ entity: '科目', entityId: cat.id, field: '状态', oldValue: cat.status, newValue: body.status, desc: '状态: ' + cat.status + ' → ' + body.status });
	            cat.status = body.status;
	          }
	          if (body.openingBalanceCents !== undefined) {
            effects.push({ entity: '科目', entityId: cat.id, field: '期初余额', oldValue: String(cat.openingBalanceCents || 0), newValue: String(body.openingBalanceCents), desc: '期初余额: ¥' + ((cat.openingBalanceCents || 0)/100).toFixed(2) + ' → ¥' + (body.openingBalanceCents/100).toFixed(2) });
            cat.openingBalanceCents = body.openingBalanceCents;
            cat.balanceCents = body.openingBalanceCents; // 期初余额同步到当前余额
          }
	          if (effects.length > 0) recordOp('修改科目', '修改科目「' + cat.name + '」', effects);
	          return sendJSON(res, 200, cat);
	        });
	        return;
	      }
	      if (req.method === 'DELETE') {
	        if (cat.preset) return sendJSON(res, 403, { code: 'CATEGORY_PRESET', message: '预置科目不能删除或重命名' });
	        const catName = cat.name;
	        const catId = cat.id;
	        // 从父节点移除
	        if (cat.level === 2) {
	          const parent = CATEGORIES.find(l1 => l1.id === cat.parentId);
	          if (parent) parent.children = parent.children.filter(c => c.id !== cat.id);
	        } else {
	          const idx = CATEGORIES.findIndex(l1 => l1.id === cat.id);
	          if (idx >= 0) CATEGORIES.splice(idx, 1);
	        }
	        recordOp('删除科目', '删除科目「' + catName + '」', [{ entity: '科目', entityId: catId, desc: '删除科目 #' + catId + ' ' + catName }]);
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
		        if (body.direction === 'expense') {
		          // 余额防负校验
		          const txns = TRANSACTIONS.filter(t => t.status !== 'voided');
		          const incomeTotal = txns.filter(t => t.direction === 'income').reduce((s, t) => s + t.amountCents, 0);
		          const expenseTotal = txns.filter(t => t.direction === 'expense').reduce((s, t) => s + t.amountCents, 0);
		          const currentBalance = (SETTINGS.bankOpeningBalanceCents || 0) + incomeTotal - expenseTotal;
		          if (body.amountCents > currentBalance) {
		            return sendFail(res, 400, 'INSUFFICIENT_BALANCE', '银行存款余额不足，无法完成支出');
		          }
		        }
		        const newId = TRANSACTIONS.length ? Math.max(...TRANSACTIONS.map(t => t.id)) + 1 : 1;
	        const txn = { id: newId, status: 'normal', ...body };
	        TRANSACTIONS.push(txn);
	        const dirLabel = txn.direction === 'income' ? '收入' : '支出';
	        const catName = txn.categoryName || '科目#' + txn.categoryId;
	        recordOp('记一笔' + dirLabel, '¥' + (txn.amountCents/100).toFixed(2) + ' ' + dirLabel + ' → ' + catName, [
	          { entity: '流水', entityId: txn.id, field: '创建', newValue: txn.note || '', desc: '创建流水 #' + txn.id + ', ¥' + (txn.amountCents/100).toFixed(2) + ' ' + dirLabel },
	          { entity: '科目', entityId: catName, field: '余额', desc: catName + ' 余额' + (txn.direction === 'income' ? ' +' : ' -') + '¥' + (txn.amountCents/100).toFixed(2) },
	          { entity: '银行存款', field: '余额', desc: '银行存款' + (txn.direction === 'income' ? ' +' : ' -') + '¥' + (txn.amountCents/100).toFixed(2) },
	        ]);
	        return sendJSON(res, 200, txn);
	      }
	    }
	    if (pathname.startsWith('/api/transactions/')) {
		      if (req.method === 'PUT') {
			        const id = parseInt(pathname.split('/').pop());
			        const txn = TRANSACTIONS.find(t => t.id === id);
			        if (txn) {
			          const body = await readBody(req);
			          // 只允许作废操作，不允许修改金额/摘要等字段
			          if (body.status === 'voided' && txn.status !== 'voided') {
			            const oldStatus = txn.status;
			            txn.status = 'voided';
			            const effects = [
			              { entity: '流水', entityId: txn.id, field: '状态', oldValue: oldStatus, newValue: 'voided', desc: '已作废流水 #' + txn.id + ' (¥' + (txn.amountCents/100).toFixed(2) + ' ' + (txn.direction === 'income' ? '收入' : '支出') + ')' },
			              { entity: '银行存款', field: '余额', desc: '回滚银行存款' + (txn.direction === 'income' ? ' -' : ' +') + '¥' + (txn.amountCents/100).toFixed(2) },
			              { entity: '科目', entityId: txn.categoryId, field: '余额', desc: '回滚科目余额' + (txn.direction === 'income' ? ' -' : ' +') + '¥' + (txn.amountCents/100).toFixed(2) },
			            ];
			            recordOp('作废流水', '作废流水 #' + txn.id, effects);
			            return sendJSON(res, 200, txn);
			          }
			          // 其他修改操作禁止
			          return sendJSON(res, 403, { code: 'TXN_READONLY', message: '流水不允许编辑，只能作废' });
			        }
			        return sendJSON(res, 404, { code: 'NOT_FOUND', message: '流水不存在' });
			      }
		      if (req.method === 'DELETE') {
		        const id = parseInt(pathname.split('/').pop());
		        const txn = TRANSACTIONS.find(t => t.id === id);
		        if (txn && txn.status !== 'voided') {
		          txn.status = 'voided';
		          recordOp('作废流水', '作废流水 #' + id + ' (¥' + (txn.amountCents/100).toFixed(2) + ' ' + (txn.direction === 'income' ? '收入' : '支出') + ')', [
		            { entity: '流水', entityId: txn.id, field: '状态', oldValue: 'normal', newValue: 'voided', desc: '已作废流水 #' + txn.id },
		            { entity: '银行存款', field: '余额', desc: '回滚银行存款' + (txn.direction === 'income' ? ' -' : ' +') + '¥' + (txn.amountCents/100).toFixed(2) },
		            { entity: '科目', entityId: txn.categoryId, field: '余额', desc: '回滚科目余额' + (txn.direction === 'income' ? ' -' : ' +') + '¥' + (txn.amountCents/100).toFixed(2) },
		          ]);
		        }
		        return sendJSON(res, 200, { ok: true });
		      }
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
		        // 重名校验（名称 + 类型一致才判重）
		        const dupes = findDuplicateParties(body.name, body.types?.[0] || body.type);
		        if (dupes.length > 0) {
		          return sendJSON(res, 409, { code: 'DUPLICATE_NAME', message: '存在重名单位：' + dupes.map(d => d.name).join('、'), dupes: dupes.map(d => ({ id: d.id, name: d.name })) });
		        }
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
		        // 自动创建 L2 科目
		        const createdCats = ensurePartyL2Categories(p);
		        const typeLabels = { invest: '投资公司', flow: '流转企业', reinvest: '再投资', longterm: '长期投资', other: '其它单位' };
		        const effects = [
		          { entity: '往来单位', entityId: p.id, field: '名称', newValue: p.name, desc: '单位名称: ' + p.name },
		          { entity: '往来单位', entityId: p.id, field: '类型', newValue: types[0] || '', desc: '单位类型: ' + (typeLabels[types[0]] || types[0] || '') },
		        ];
		        for (const cat of createdCats) {
		          const l1 = CATEGORIES.find(c => c.id === cat.parentId);
		          effects.push({ entity: '科目', entityId: cat.id, field: '创建', newValue: cat.name, desc: '创建科目「' + cat.name + '」' + (l1 ? ' 在 ' + l1.name + ' 下' : '') });
		        }
		        recordOp('创建单位', '创建单位「' + p.name + '」' + (types[0] ? ' (' + (typeLabels[types[0]] || types[0]) + ')' : '') + (createdCats.length > 0 ? ', 自动创建 ' + createdCats.length + ' 个科目' : ''), effects);
		        return sendJSON(res, 200, { ...p, type: types[0] || null, createdCats: createdCats.map(c => ({ id: c.id, name: c.name, parentId: c.parentId })) });
		      }
	    }
	    // PUT /api/parties/:id - 已禁用（单位不允许编辑）
		    const partyMatch = pathname.match(/^\/api\/parties\/(\d+)$/);
		    if (partyMatch && req.method === 'PUT') {
		      return sendJSON(res, 403, { code: 'PARTY_READONLY', message: '单位不允许编辑' });
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
	      recordOp('上传合同', '上传合同「' + contract.fileName + '」', [
	        { entity: '合同', entityId: contract.id, field: '创建', newValue: contract.fileName, desc: '创建合同 #' + contract.id + ': ' + contract.fileName },
	      ]);
	      // 列表接口不返回文件体
	      const safe = { ...contract, fileData: undefined };
	      return sendJSON(res, 200, safe);
	    }
	    // DELETE /api/contracts/:id
	    const delContractMatch = pathname.match(/^\/api\/contracts\/(\d+)$/);
	    if (delContractMatch && req.method === 'DELETE') {
	      const id = parseInt(delContractMatch[1], 10);
	      const idx = CONTRACTS.findIndex(c => c.id === id);
	      if (idx >= 0) {
	        const c = CONTRACTS[idx];
	        CONTRACTS.splice(idx, 1);
	        recordOp('删除合同', '删除合同「' + c.fileName + '」', [{ entity: '合同', entityId: id, desc: '删除合同 #' + id + ': ' + c.fileName }]);
	        return sendJSON(res, 200, { ok: true });
	      }
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
	        const oldPaid = r.paidCents || 0;
	        const oldStatus = r.status;
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
	        const methodLabel = receipt.method === 'cash' ? '现金收款' : '抵销';
	        recordOp(methodLabel, '¥' + (amt/100).toFixed(2) + ' → ' + (r.partyName || '单位#' + r.partyId), [
	          { entity: '应收单', entityId: r.id, field: '已收金额', oldValue: String(oldPaid), newValue: String(r.paidCents), desc: '已收: ¥' + (oldPaid/100).toFixed(2) + ' → ¥' + (r.paidCents/100).toFixed(2) },
	          { entity: '应收单', entityId: r.id, field: '状态', oldValue: oldStatus, newValue: r.status, desc: '状态: ' + oldStatus + ' → ' + r.status },
	          { entity: '核销记录', entityId: receipt.id, field: '创建', newValue: methodLabel, desc: '创建核销记录 #' + receipt.id + ', ¥' + (amt/100).toFixed(2) + ' ' + methodLabel },
	        ]);
	        return sendJSON(res, 200, receipt);
	      }
    }
    // PUT /api/receipts/:id - 作废核销（前端 ContactsPage 调用）
		    const receiptPutMatch = pathname.match(/^\/api\/receipts\/(\d+)$/);
		    if (receiptPutMatch && req.method === 'PUT') {
		      const rid = parseInt(receiptPutMatch[1], 10);
		      // 在所有应收单中找该核销记录
		      let foundR, foundRc;
		      for (const recv of RECEIVABLES) {
		        if (recv.receipts) {
		          const rc = recv.receipts.find(x => x.id === rid);
		          if (rc) { foundR = recv; foundRc = rc; break; }
		        }
		      }
		      if (!foundR || !foundRc) return sendFail(res, 404, 'NOT_FOUND', '核销记录不存在');
		      const body = await readBody(req);
		      if (body.status === 'voided') {
		        const oldPaid = foundR.paidCents || 0;
		        const oldStatus = foundR.status;
		        foundRc.status = 'voided';
		        foundRc.updatedAt = new Date().toISOString();
		        // 回退金额
		        foundR.paidCents = Math.max(0, (foundR.paidCents || 0) - foundRc.amountCents);
		        foundR.outstandingCents = Math.max(0, foundR.amountCents - foundR.paidCents);
		        if (foundR.outstandingCents > 0) foundR.status = 'open';
		        foundR.updatedAt = new Date().toISOString();
		        const p = PARTIES.find(x => x.id === foundR.partyId);
		        if (p) {
		          p.outstandingCents = RECEIVABLES
		            .filter(x => x.partyId === foundR.partyId)
		            .reduce((s, x) => s + (x.outstandingCents || 0), 0);
		        }
		        const effects = [
		          { entity: '核销记录', entityId: rid, field: '状态', oldValue: 'normal', newValue: 'voided', desc: '核销记录 #' + rid + ': normal → voided' },
		          { entity: '应收单', entityId: foundR.id, field: '已收金额', oldValue: String(oldPaid), newValue: String(foundR.paidCents), desc: '已收: ¥' + (oldPaid/100).toFixed(2) + ' → ¥' + (foundR.paidCents/100).toFixed(2) },
		          { entity: '应收单', entityId: foundR.id, field: '状态', oldValue: oldStatus, newValue: foundR.status, desc: '状态: ' + oldStatus + ' → ' + foundR.status },
		        ];
		        // 现金核销 → 连带作废对应的银行收入流水
		        if (foundRc.method === 'cash' && foundRc.txnId) {
		          const txn = TRANSACTIONS.find(t => t.id === foundRc.txnId);
		          if (txn && txn.status !== 'voided') {
		            txn.status = 'voided';
		            effects.push({ entity: '流水', entityId: txn.id, field: '状态', oldValue: 'normal', newValue: 'voided', desc: '连带作废流水 #' + txn.id + ' (¥' + (txn.amountCents/100).toFixed(2) + ')' });
		          }
		        }
		        recordOp('作废核销', '作废核销 #' + rid, effects);
		      }
		      return sendJSON(res, 200, { ok: true });
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
	      const oldPaid = r.paidCents || 0;
	      const oldStatus = r.status;
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
	      recordOp('作废核销', '作废核销 #' + rid + ' (应收#' + receivableId + ')', [
	        { entity: '核销记录', entityId: rid, field: '状态', oldValue: 'normal', newValue: 'voided', desc: '核销记录 #' + rid + ': normal → voided' },
	        { entity: '应收单', entityId: r.id, field: '已收金额', oldValue: String(oldPaid), newValue: String(r.paidCents), desc: '已收: ¥' + (oldPaid/100).toFixed(2) + ' → ¥' + (r.paidCents/100).toFixed(2) },
	        { entity: '应收单', entityId: r.id, field: '状态', oldValue: oldStatus, newValue: r.status, desc: '状态: ' + oldStatus + ' → ' + r.status },
	      ]);
	      return sendJSON(res, 200, { ok: true });
	    }

    // settings
	    if (pathname === '/api/settings') {
	      if (req.method === 'GET') return sendJSON(res, 200, SETTINGS);
	      if (req.method === 'PUT') {
	        readBody(req).then(body => {
	          const effects = [];
	          if (body.bankOpeningBalanceCents !== undefined) {
	            effects.push({ entity: '设置', field: '银行存款期初余额', oldValue: String(SETTINGS.bankOpeningBalanceCents), newValue: String(body.bankOpeningBalanceCents), desc: '期初余额: ¥' + (SETTINGS.bankOpeningBalanceCents/100).toFixed(2) + ' → ¥' + (body.bankOpeningBalanceCents/100).toFixed(2) });
	            SETTINGS.bankOpeningBalanceCents = body.bankOpeningBalanceCents;
	          }
	          if (body.reinvestRatioBps !== undefined) {
	            const old = SETTINGS.reinvestRatioBps;
	            SETTINGS.reinvestRatioBps = Math.max(0, Math.min(10000, body.reinvestRatioBps | 0));
	            effects.push({ entity: '设置', field: '再投资比例', oldValue: String(old), newValue: String(SETTINGS.reinvestRatioBps), desc: '再投资比例: ' + (old/100).toFixed(1) + '% → ' + (SETTINGS.reinvestRatioBps/100).toFixed(1) + '%' });
	          }
	          if (effects.length > 0) recordOp('保存设置', '更新 ' + effects.length + ' 项设置', effects);
	          return sendJSON(res, 200, SETTINGS);
	        });
	        return;
	      }
	    }

    // operation-logs
		if (pathname === '/api/operation-logs' && req.method === 'GET') {
		  return sendJSON(res, 200, { items: [...OPERATION_LOGS].reverse() });
		}

		// changelog
		if (pathname === '/api/changelog') return sendJSON(res, 200, []);

		// backups
		if (pathname === '/api/backups') {
		  if (req.method === 'GET') return sendJSON(res, 200, { items: BACKUPS });
		  if (req.method === 'POST') {
		    const now = new Date();
		    const pad = (n) => String(n).padStart(2, '0');
		    const id = 'jz-backup-' + now.getFullYear() + pad(now.getMonth()+1) + pad(now.getDate()) + '-' + pad(now.getHours()) + pad(now.getMinutes()) + pad(now.getSeconds()) + '.db';
		    const entry = { id, sizeBytes: Math.floor(Math.random() * 500000) + 100000, createdAt: id.replace(/^jz-backup-|\.db$/g, '') };
		    BACKUPS.push(entry);
		    recordOp('立即备份', '创建备份 ' + id, [{ entity: '备份', desc: '创建备份: ' + id }]);
		    return sendJSON(res, 200, { name: id });
		  }
		}
		if (pathname.startsWith('/api/backups/') && pathname.endsWith('/restore') && req.method === 'POST') {
		  const id = pathname.replace('/api/backups/', '').replace('/restore', '');
		  const exists = BACKUPS.some(b => b.id === id);
		  if (!exists) return sendFail(res, 404, 'NOT_FOUND', '备份不存在');
		  recordOp('恢复备份', '从备份恢复 ' + id, [{ entity: '备份', desc: '从备份恢复: ' + id }]);
		  return sendJSON(res, 200, { ok: true });
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
	      const createdDetails = [];
	      const addRecv = (p, kind, amountCents) => {
	        const key = p.id + ':' + kind;
	        if (existingKeys.has(key)) { skipped++; return; }
	        const recv = {
	          id: nextId++, orgId: 1, partyId: p.id, partyName: p.name,
	          recvYear: year, kind, recvKind: kind,
	          title: year + '年度' + (kind === 'rent' ? '土地流转费' : kind === 'dividend' ? '投资收益' : kind === 'service' ? '流转管理费' : '应收'),
	          amountCents, incomeCategoryId: null,
	          status: 'open', note: null,
	          createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
	          paidCents: 0, outstandingCents: amountCents,
	        };
	        RECEIVABLES.push(recv);
	        p.outstandingCents = RECEIVABLES
	          .filter(x => x.partyId === p.id)
	          .reduce((s, x) => s + (x.outstandingCents || 0), 0);
	        created++;
	        createdDetails.push({ entity: '应收单', entityId: recv.id, desc: '创建应收 #' + recv.id + ': ' + p.name + ' ' + recv.title + ' ¥' + (amountCents/100).toFixed(2) });
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
	      recordOp('年度结转', year + '年度结转: 创建 ' + created + ' 条, 跳过 ' + skipped + ' 条', createdDetails);
	      return sendJSON(res, 200, { created, skipped });
	    }
    // fund-moves
		    if (pathname === '/api/fund-moves') {
		      if (req.method === 'GET') return sendJSON(res, 200, FUND_MOVES);
		      if (req.method === 'POST') {
		        const body = await readBody(req);
		        const dir = body.direction; // 'invest' 投资(银行→资产), 'recover' 收回(资产→银行)
		        const amt = body.amountCents || 0;
		        if (amt <= 0) return sendFail(res, 400, 'BAD_REQUEST', '金额必须大于 0');
		        const newId = FUND_MOVES.length ? Math.max(...FUND_MOVES.map(f => f.id)) + 1 : 1;
		        const fm = {
		          id: newId,
		          direction: dir,
		          amountCents: amt,
		          partyId: body.partyId || null,
		          note: body.note || '',
		          status: 'normal',
		          createdAt: new Date().toISOString(),
		          updatedAt: new Date().toISOString(),
		        };
		        FUND_MOVES.push(fm);
		        const dirLabel = dir === 'invest' ? '投资' : '收回';
		        recordOp('资金划转', dirLabel + ' ¥' + (amt/100).toFixed(2), [
		          { entity: '资金划转', entityId: fm.id, field: '方向', newValue: dirLabel, desc: dirLabel + ' ¥' + (amt/100).toFixed(2) },
		          { entity: '银行存款', field: '余额', desc: '银行存款' + (dir === 'invest' ? ' -' : ' +') + '¥' + (amt/100).toFixed(2) },
		          { entity: '资产科目', field: '余额', desc: '资产科目余额' + (dir === 'invest' ? ' +' : ' -') + '¥' + (amt/100).toFixed(2) },
		        ]);
		        return sendJSON(res, 200, fm);
		      }
		    }
		    if (pathname.startsWith('/api/fund-moves/')) {
		      const id = parseInt(pathname.split('/').pop(), 10);
		      const fm = FUND_MOVES.find(f => f.id === id);
		      if (!fm) return sendFail(res, 404, 'NOT_FOUND', '资金划转记录不存在');
		      if (req.method === 'PUT') {
		        const body = await readBody(req);
		        if (body.status === 'voided') {
		          const oldStatus = fm.status;
		          fm.status = 'voided';
		          fm.updatedAt = new Date().toISOString();
		          recordOp('作废资金划转', '作废资金划转#' + fm.id, [
		            { entity: '资金划转', entityId: fm.id, field: '状态', oldValue: oldStatus, newValue: 'voided', desc: '状态: ' + oldStatus + ' → voided' },
		            { entity: '银行存款', field: '余额', desc: '回滚银行存款' + (fm.direction === 'invest' ? ' +' : ' -') + '¥' + (fm.amountCents/100).toFixed(2) },
		            { entity: '资产科目', field: '余额', desc: '回滚资产科目余额' + (fm.direction === 'invest' ? ' -' : ' +') + '¥' + (fm.amountCents/100).toFixed(2) },
		          ]);
		        }
		        return sendJSON(res, 200, fm);
		      }
		    }

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
		      // 如果已存在分配方案且已有相关流水，禁止修改
		      if (existing >= 0) {
		        const hasTxns = TRANSACTIONS.some(t =>
		          t.status !== 'voided' &&
		          t.txnDate && t.txnDate.startsWith(String(year)) &&
		          t.note && (t.note.includes('532-') || t.note.includes('—再投资'))
		        );
		        if (hasTxns) {
		          return sendJSON(res, 403, { code: 'DIST_LOCKED', message: '该年度分配方案已有支出记录，无法修改' });
		        }
		      }
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
	      const action = existing >= 0 ? '更新' : '创建';
	      if (existing >= 0) {
	        DISTRIBUTIONS_532[existing] = dist;
	      } else {
	        DISTRIBUTIONS_532.push(dist);
	      }
	      recordOp(action + '532分配', year + '年度 532 分配方案', [
	        { entity: '532分配', entityId: dist.id, field: '再投资', newValue: String(dist.reinvestCents), desc: '再投资: ¥' + (dist.reinvestCents/100).toFixed(2) },
	        { entity: '532分配', entityId: dist.id, field: '成员分红', newValue: String(dist.dividendCents), desc: '成员分红: ¥' + (dist.dividendCents/100).toFixed(2) },
	        { entity: '532分配', entityId: dist.id, field: '公益支出', newValue: String(dist.welfareCents), desc: '公益支出: ¥' + (dist.welfareCents/100).toFixed(2) },
	      ]);
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




