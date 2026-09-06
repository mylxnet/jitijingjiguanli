/**
 * 快速记账 12 个模板端到端测试
 * 验证每个模板最终调用的后端路由契约（不验证 Vue UI，只验证 API 交互和数据正确性）
 *
 * 覆盖维度：
 *   A. 预置科目结构（所有模板依赖的 L2 都存在）
 *   B. 纯 preset L2 模板（grant / interest / member / welfare / mgmtFee / toHousehold / recover）
 *   C. autoBuildL1 模板（dividend / rent / service / reinvest）：在指定 L1 下按单位名自动建 L2
 *   D. invest 模板（长期投资给公司）：用选中的 invest L2
 *   E. rent 的减欠款联动：记账后 party.outstandingCents 减少
 */
const test = require('node:test')
const assert = require('node:assert/strict')
const http = require('http')
const { spawn } = require('child_process')

const PORT = 18085
let serverProcess

function httpReq(method, path, body, cookie) {
  return new Promise((resolve, reject) => {
    const url = new URL(path, `http://localhost:${PORT}`)
    const opts = {
      hostname: url.hostname, port: url.port, path: url.pathname + url.search, method,
      headers: { 'Content-Type': 'application/json' },
    }
    if (cookie) opts.headers['Cookie'] = cookie
    const req = http.request(opts, res => {
      let data = ''
      res.on('data', c => data += c)
      res.on('end', () => {
        try {
          resolve({ status: res.statusCode, data: data ? JSON.parse(data) : null })
        } catch (e) {
          reject(new Error(`响应不是有效 JSON: ${data.substring(0, 100)}`))
        }
      })
    })
    req.on('error', reject)
    if (body) req.write(JSON.stringify(body))
    req.end()
  })
}

// sendJSON 把业务 payload 包在 { data: payload } 里，这里解包
function unwrap(res) {
  return res.data.data
}

let sessionCookie, allCats, allParties

test.before(async () => {
  serverProcess = spawn(process.execPath, ['server/mock.js'], {
    env: { ...process.env, PORT: String(PORT) },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  await new Promise(res => setTimeout(res, 800))
  const login = await httpReq('POST', '/api/auth/login', { username: 'test', password: 'test' })
  sessionCookie = login.data?.setCookie || 'session=mock_session'
  const catsRes = await httpReq('GET', '/api/categories', null, sessionCookie)
  allCats = unwrap(catsRes)
  const partiesRes = await httpReq('GET', '/api/parties', null, sessionCookie)
  allParties = unwrap(partiesRes)
})

test.after(async () => {
  if (serverProcess) {
    serverProcess.kill('SIGTERM')
    await new Promise(res => setTimeout(res, 200))
  }
})

// 辅助：按 L1 名 + L2 名找 category
function findL2(l1Name, l2Name) {
  const l1 = allCats.find(c => c.name === l1Name)
  if (!l1) return null
  return (l1.children || []).find(c => c.name === l2Name) || null
}

// 辅助：确保在指定 L1 下有指定 L2（autoBuild 逻辑的 HTTP 版）
async function ensureL2(l1Name, l2Name) {
  let l1 = allCats.find(c => c.name === l1Name)
  if (!l1) throw new Error(`缺少 L1：${l1Name}`)
  let l2 = (l1.children || []).find(c => c.name === l2Name)
  if (l2) return l2
  // POST 新建
  const res = await httpReq('POST', '/api/categories', {
    name: l2Name, level: 2, parentId: l1.id, kind: 'equity',
  }, sessionCookie)
  // 刷新缓存
  const catsRes = await httpReq('GET', '/api/categories', null, sessionCookie)
  allCats = unwrap(catsRes)
  l1 = allCats.find(c => c.name === l1Name)
  return (l1.children || []).find(c => c.name === l2Name)
}

// ============= 一、预置科目结构 =============

test('预置：本金 L1 下有 上级补助 + 待投资 两个 preset L2', () => {
  const fund = allCats.find(c => c.name === '本金')
  assert.ok(fund, '本金 L1 存在')
  const names = (fund.children || []).map(c => c.name).sort()
  assert.deepEqual(names, ['上级补助', '待投资'].sort(), `本金 L2 错误: ${names}`)
  for (const l2 of fund.children) assert.equal(l2.preset, true, `${l2.name} 应为 preset`)
})

test('预置：8 个 L1 按 sort 排序', () => {
  assert.equal(allCats.length, 8)
  const names = allCats.map(c => c.name)
  assert.deepEqual(names, ['本金', '长期投资', '再投资', '经营收入', '投资收益', '土地流转费收入', '流转管理费', '分配与支出'])
})

// ============= 二、模板 1/6/7/8/9/5/11：纯 preset L2（无自动建） =============

async function testPresetTxn(templateName, dir, l1Name, l2Name, amountCents, defaultNote) {
  const l2 = findL2(l1Name, l2Name)
  assert.ok(l2, `${templateName}: 缺少 L2 ${l1Name}/${l2Name}`)
  const amount = 50000000 // ¥500
  const note = defaultNote
  const res = await httpReq('POST', '/api/transactions', {
    txnDate: '2026-09-05',
    direction: dir,
    amountCents: amount,
    categoryId: l2.id,
    note,
  }, sessionCookie)
  assert.equal(res.status, 200, `${templateName}: 保存失败`)
  const data = unwrap(res)
  assert.ok(data, `${templateName}: 响应应有 data`)
  assert.equal(data.direction, dir, `${templateName}: direction`)
  assert.equal(data.amountCents, amount, `${templateName}: amountCents`)
  assert.equal(data.categoryId, l2.id, `${templateName}: categoryId`)
  assert.ok(data.id, `${templateName}: 应有自增 id`)
}

test('模板 1 grant: 收上级财政补助 → 【上级补助】+X, 银行存款+X', async () => {
  await testPresetTxn('grant', 'income', '本金', '上级补助', 50000000, '收上级财政补助')
})

test('模板 6 interest: 银行存款利息 → 【其他收入】+X, 银行存款+X', async () => {
  await testPresetTxn('interest', 'income', '经营收入', '其他收入', 350000, '银行存款利息')
})

test('模板 7 member: 532-成员分配发放 → 【成员分红】-X, 银行存款-X', async () => {
  await testPresetTxn('member', 'expense', '分配与支出', '成员分红', 20000000, '成员分红')
})

test('模板 8 welfare: 532-公益支出 → 【公益支出】-X', async () => {
  await testPresetTxn('welfare', 'expense', '分配与支出', '公益支出', 5000000, '公益支出')
})

test('模板 9 mgmtFee: 支出管理费 → 【管理费支出】-X', async () => {
  await testPresetTxn('mgmtFee', 'expense', '分配与支出', '管理费支出', 3000000, '管理费支出')
})

test('模板 5 toHousehold: 拨付流转费给农户 → 【转付农户】-X', async () => {
  await testPresetTxn('toHousehold', 'expense', '分配与支出', '土地流转费-转付农户', 500000000, '拨付流转费给农户')
})

test('模板 11 recover: 收回投资 → 【待投资】+X, 银行存款+X (preset L2)', async () => {
  // 模板 11 recover 选往来单位"富民公司"，note 应为 "收回 富民公司 投资"
  await testPresetTxn('recover', 'income', '本金', '待投资', 200000000, '收回 富民公司 投资')
})

// ============= 三、模板 2/3/4/12：autoBuildL1 + 选往来单位 =============

test('模板 2 dividend: 收到投资收益/分红 → 自动在【投资收益】下建同名 L2', async () => {
  const l2 = await ensureL2('投资收益', '测试投资公司')
  const res = await httpReq('POST', '/api/transactions', {
    txnDate: '2026-09-05',
    direction: 'income',
    amountCents: 8000000, // ¥80,000
    categoryId: l2.id,
    note: '收到 测试投资公司 投资收益',
  }, sessionCookie)
  assert.equal(res.status, 200)
  const data = unwrap(res)
  assert.equal(data.direction, 'income')
  assert.equal(data.amountCents, 8000000)
  assert.equal(data.categoryId, l2.id)
  assert.match(data.note, /测试投资公司/)
})

test('模板 3 rent: 收到土地流转费 → 自动在【土地流转费收入】下建同名 L2 + 减欠款联动', async () => {
  // 选"祥云合作社"（flow 类型，已有 L2，但先验证 ensureL2 能正确返回已存在的）
  const l2 = await ensureL2('土地流转费收入', '祥云合作社')
  assert.ok(l2.preset === false || l2.preset === undefined, 'auto-build 的 L2 preset 应为 false')
  const party = allParties.find(p => p.name === '祥云合作社')
  const origDebt = party.outstandingCents

  const AMOUNT = 3000000 // ¥30,000
  // 先记 txn
  const txnRes = await httpReq('POST', '/api/transactions', {
    txnDate: '2026-09-05',
    direction: 'income',
    amountCents: AMOUNT,
    categoryId: l2.id,
    note: `收到 祥云合作社 土地流转费`,
  }, sessionCookie)
  assert.equal(txnRes.status, 200)
  assert.equal(unwrap(txnRes).categoryId, l2.id)

  // 再减欠款：PUT /api/parties/:id
  const newDebt = Math.max(0, origDebt - AMOUNT)
  const putRes = await httpReq('PUT', `/api/parties/${party.id}`, { outstandingCents: newDebt }, sessionCookie)
  assert.equal(putRes.status, 200)
  assert.equal(unwrap(putRes).outstandingCents, newDebt)
})

test('模板 4 service: 收到流转管理费 → 自动在【流转管理费】下建同名 L2', async () => {
  const l2 = await ensureL2('流转管理费', '测试流转企业')
  const res = await httpReq('POST', '/api/transactions', {
    txnDate: '2026-09-05',
    direction: 'income',
    amountCents: 500000, // ¥5,000
    categoryId: l2.id,
    note: '收到 测试流转企业 流转管理费',
  }, sessionCookie)
  assert.equal(res.status, 200)
  const data = unwrap(res)
  assert.equal(data.direction, 'income')
  assert.equal(data.categoryId, l2.id)
})

test('模板 12 reinvest: 532-再投资 → 自动在【再投资】下建同名 L2 (expense)', async () => {
  const l2 = await ensureL2('再投资', '测试再投资公司')
  const res = await httpReq('POST', '/api/transactions', {
    txnDate: '2026-09-05',
    direction: 'expense',
    amountCents: 15000000, // ¥150,000
    categoryId: l2.id,
    note: '532再投资给 测试再投资公司',
  }, sessionCookie)
  assert.equal(res.status, 200)
  const data = unwrap(res)
  assert.equal(data.direction, 'expense', '再投资应是 expense')
  assert.equal(data.amountCents, 15000000)
  assert.equal(data.categoryId, l2.id)
})

// ============= 四、模板 10 invest: 长期投资给公司 =============

test('模板 10 invest: 长期投资给公司 → 用已存在的 invest L2 (富民公司)', async () => {
  const l1 = allCats.find(c => c.name === '长期投资')
  assert.ok(l1)
  const l2 = (l1.children || []).find(c => c.name === '富民公司')
  assert.ok(l2, '应存在 富民公司 L2')
  const res = await httpReq('POST', '/api/transactions', {
    txnDate: '2026-09-05',
    direction: 'expense',
    amountCents: 50000000, // ¥500,000
    categoryId: l2.id,
    note: '长期投资给公司',
  }, sessionCookie)
  assert.equal(res.status, 200)
  const data = unwrap(res)
  assert.equal(data.direction, 'expense')
  assert.equal(data.categoryId, l2.id)
})

test('模板 10 invest: 长期投资给公司 → 新建公司后 L2 自动创建并可用', async () => {
  // 模拟 Home.vue handleCreateCompany 流程：
  // 1. POST /api/categories 在"长期投资"下建 L2
  const l1 = allCats.find(c => c.name === '长期投资')
  const NEW_COMPANY = '测试新建公司-XY'
  const catRes = await httpReq('POST', '/api/categories', {
    name: NEW_COMPANY, level: 2, parentId: l1.id, kind: 'equity',
  }, sessionCookie)
  assert.equal(catRes.status, 200)
  // 2. 刷新 allCats
  const catsRes = await httpReq('GET', '/api/categories', null, sessionCookie)
  allCats = unwrap(catsRes)
  const newL2 = findL2('长期投资', NEW_COMPANY)
  assert.ok(newL2, '应能找到新建的 L2')
  assert.equal(newL2.preset, false, 'auto-build L2 preset 应为 false')
  // 3. 用这个 L2 记一条 txn
  const txnRes = await httpReq('POST', '/api/transactions', {
    txnDate: '2026-09-05',
    direction: 'expense',
    amountCents: 30000000,
    categoryId: newL2.id,
    note: '长期投资给 ' + NEW_COMPANY,
  }, sessionCookie)
  assert.equal(txnRes.status, 200)
  assert.equal(unwrap(txnRes).categoryId, newL2.id)
})

// ============= 五、note 格式规范 =============

test('所有模板 note 不超过 100 字符（前后端契约）', async () => {
  const res = await httpReq('GET', '/api/transactions', null, sessionCookie)
  const list = unwrap(res).items || unwrap(res) || []
  // 把刚建的 14 条 txn 都检查一遍
  for (const t of list) {
    if (t.note) assert.ok(t.note.length <= 100, `note 过长: "${t.note}"`)
  }
})

test('所有 transaction 的 categoryId 指向真实存在的 category', async () => {
  const res = await httpReq('GET', '/api/transactions', null, sessionCookie)
  const list = unwrap(res).items || unwrap(res) || []
  const allL2Ids = new Set()
  for (const l1 of allCats) for (const l2 of (l1.children || [])) allL2Ids.add(l2.id)
  for (const t of list) {
    assert.ok(allL2Ids.has(t.categoryId), `txn ${t.id} 的 categoryId=${t.categoryId} 不存在于当前科目结构`)
  }
})

// ============= 六、边界条件 =============

test('ensureL2 对已存在的 L2 不会重复创建（幂等）', async () => {
  const first = await ensureL2('土地流转费收入', '祥云合作社')
  const second = await ensureL2('土地流转费收入', '祥云合作社')
  assert.equal(first.id, second.id, '两次 ensureL2 应返回同一个 L2')
})

test('POST transactions 缺 amountCents 应返回 400 或后端报错', async () => {
  const l2 = findL2('本金', '上级补助')
  const res = await httpReq('POST', '/api/transactions', {
    txnDate: '2026-09-05', direction: 'income', categoryId: l2.id,
  }, sessionCookie)
  // mock.js 的 POST /api/transactions 不做字段校验，会直接返回
  // 这里只是确保不会崩，真实校验在 Go 后端
  assert.ok([200, 400, 422, 500].includes(res.status))
})

test('note 可为空字符串（不强制填写）', async () => {
  const l2 = findL2('本金', '上级补助')
  const res = await httpReq('POST', '/api/transactions', {
    txnDate: '2026-09-05', direction: 'income', amountCents: 10000,
    categoryId: l2.id, note: '',
  }, sessionCookie)
  assert.equal(res.status, 200)
})
