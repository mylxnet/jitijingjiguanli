/**
 * 预置科目 + preset 守卫 + 引导页路由 测试
 * 覆盖：
 *   1. 预置科目数据结构（9 个 L1，preset 字段正确）
 *   2. /api/categories 返回结构中 preset 字段传递给前端
 *   3. mock.js 中 GET/POST/PUT/DELETE categories 路由响应契约
 *   4. 空 L1 容器（投资收益/再投资/再投资收益）的 children 是空数组而非 undefined
 *   5. 快速记账依赖的 L2 科目名都存在
 *   6. 引导页路由 /api/categories /api/parties /api/settings 契约
 */
const test = require('node:test')
const assert = require('node:assert/strict')
const http = require('http')

const PORT = 18083

let serverProcess, baseURL

function httpReq(method, path, body) {
  return new Promise((resolve, reject) => {
    const url = new URL(path, `http://localhost:${PORT}`)
    const opts = {
      hostname: url.hostname,
      port: url.port,
      path: url.pathname + url.search,
      method,
      headers: {
        'Content-Type': 'application/json',
        'Cookie': 'session=mock_session',
      },
    }
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

test.before(async () => {
  const { spawn } = require('child_process')
  serverProcess = spawn(process.execPath, ['server/mock.js'], {
    env: { ...process.env, PORT: String(PORT) },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  // 等待 server 起来
  await new Promise(res => setTimeout(res, 800))
  // 先登录拿 cookie
  await httpReq('POST', '/api/auth/login', { username: 'test', password: 'test' })
})

test.after(async () => {
  if (serverProcess) {
    serverProcess.kill('SIGTERM')
    await new Promise(res => setTimeout(res, 200))
  }
})

// ================ 一、预置科目数据结构 ================

test('预置科目：CATEGORIES 有 9 个 L1（本金/长期投资/再投资/经营收入/投资收益/再投资收益/土地流转费收入/流转管理费/分配与支出）', () => {
  const { CATEGORIES } = require('../mock')
  const l1Names = CATEGORIES.map(c => c.name).sort()
  assert.deepEqual(
    l1Names,
    ['本金', '分配与支出', '再投资', '再投资收益', '投资收益', '经营收入', '长期投资', '土地流转费收入', '流转管理费'].sort(),
    `9 个 L1 名称错误，实际: ${l1Names.join(', ')}`
  )
})

test('预置科目：全部 9 个 L1 的 level=1, kind=equity, preset=true', () => {
  const { CATEGORIES } = require('../mock')
  for (const l1 of CATEGORIES) {
    assert.equal(l1.level, 1, `L1 ${l1.name} level 应为 1`)
    assert.equal(l1.kind, 'equity', `L1 ${l1.name} kind 应为 equity`)
    assert.equal(l1.preset, true, `L1 ${l1.name} preset 应为 true`)
  }
})

test('预置科目："长期投资"下的富民公司/祥云合作社 preset=false（auto-build 示例）', () => {
  const { CATEGORIES } = require('../mock')
  const invest = CATEGORIES.find(c => c.name === '长期投资')
  assert.ok(invest, '应存在"长期投资"L1')
  for (const l2 of invest.children || []) {
    assert.equal(l2.preset, false, `L2 ${l2.name} preset 应为 false`)
  }
})

test('预置科目：空容器（再投资/投资收益/再投资收益）的 children 是空数组', () => {
  const { CATEGORIES } = require('../mock')
  const invInc = CATEGORIES.find(c => c.name === '投资收益')
  const reinv = CATEGORIES.find(c => c.name === '再投资')
  const reinvInc = CATEGORIES.find(c => c.name === '再投资收益')
  assert.ok(Array.isArray(invInc.children), '投资收益 children 应是数组')
  assert.equal(invInc.children.length, 0, '投资收益应为空容器')
  assert.ok(Array.isArray(reinv.children), '再投资 children 应是数组')
  assert.equal(reinv.children.length, 0, '再投资应为空容器')
  assert.ok(Array.isArray(reinvInc.children), '再投资收益 children 应是数组')
  assert.equal(reinvInc.children.length, 0, '再投资收益应为空容器')
})

test('预置科目：经营收入 L1 下的 2 个二级名正确（其他财政收入/其他收入）', () => {
  const { CATEGORIES } = require('../mock')
  const income = CATEGORIES.find(c => c.name === '经营收入')
  const names = (income.children || []).map(c => c.name).sort()
  assert.deepEqual(
    names,
    ['其他财政收入', '其他收入'].sort(),
    `经营收入 L2 错误，实际: ${names.join(', ')}`
  )
  // 经营收入下 4 个 L2 全是 preset
  for (const l2 of income.children) {
    assert.equal(l2.preset, true, `L2 ${l2.name} preset 应为 true`)
  }
})

test('预置科目：分配与支出 L1 下 5 个二级名正确', () => {
  const { CATEGORIES } = require('../mock')
  const dist = CATEGORIES.find(c => c.name === '分配与支出')
  const names = (dist.children || []).map(c => c.name).sort()
  assert.deepEqual(
    names,
    ['土地流转费-转付农户', '成员分红', '福利发放', '公益支出', '管理费支出'].sort(),
    `分配与支出 L2 错误，实际: ${names.join(', ')}`
  )
})

// ================ 二、快速记账依赖的 L2 科目名 ================

test('快速记账：所有启用中的快速记账模板依赖的二级科目名都在 CATEGORIES 中存在', () => {
  const { CATEGORIES } = require('../mock')
  const allL2 = new Set()
  for (const l1 of CATEGORIES) {
    for (const l2 of (l1.children || [])) {
      allL2.add(l2.name)
    }
  }
  // 注意：dividend(投资收益) 模板暂注释，后续再调；recover(收回投资) 依赖"上级补助"
  const needed = [
    '上级补助',        // grant 收上级财政补助
    // '土地流转费收入'  已提升为 L1 容器，rent 模板后续按公司名自动建 L2
    // '流转管理费'      已提升为 L1 容器，service 模板后续按公司名自动建 L2
    '土地流转费-转付农户', // toHousehold
    '其他收入',        // interest 银行利息
    '成员分红',        // member
    '公益支出',        // welfare
    '管理费支出',      // mgmtFee
  ]
  for (const name of needed) {
    assert.ok(allL2.has(name), `快速记账依赖的科目 "${name}" 在 CATEGORIES 中不存在`)
  }
})

// ================ 三、HTTP 路由契约 ================

test('GET /api/categories 返回 9 个 L1，preset 字段存在', async () => {
  const res = await httpReq('GET', '/api/categories')
  assert.equal(res.status, 200)
  const data = res.data.data // sendJSON 包了一层
  assert.ok(Array.isArray(data), 'categories 应是数组')
  assert.equal(data.length, 9, `应返回 9 个 L1，实际 ${data.length}`)
  for (const l1 of data) {
    assert.ok('preset' in l1, `L1 ${l1.name} 应有 preset 字段`)
    // preset 是 boolean
    assert.equal(typeof l1.preset, 'boolean', `preset 应为 boolean，实际 ${typeof l1.preset}`)
    // 每个 L2 也有 preset
    for (const l2 of (l1.children || [])) {
      assert.ok('preset' in l2, `L2 ${l2.name} 应有 preset 字段`)
    }
  }
})

test('GET /api/categories?categoryId=2 返回长期投资（含 auto-build 示例二级）', async () => {
  // mock 不支持 categoryId 筛选，这里只测基础 GET
  const res = await httpReq('GET', '/api/categories')
  const l1Invest = res.data.data.find(c => c.id === 2)
  assert.ok(l1Invest, '应存在 id=2 长期投资')
  assert.equal(l1Invest.name, '长期投资')
  assert.equal(l1Invest.preset, true)
  assert.ok(Array.isArray(l1Invest.children))
  // mock 里有富民公司和祥云合作社两个 preset=false 的示例
  const autoChildren = l1Invest.children.filter(c => !c.preset)
  assert.ok(autoChildren.length >= 2, '应至少有 2 个 auto-build 示例二级')
})

test('GET /api/parties 返回往来单位列表（引导页第一页数据源）', async () => {
  const res = await httpReq('GET', '/api/parties')
  assert.equal(res.status, 200)
  const list = res.data.data
  assert.ok(Array.isArray(list), 'parties 应是数组')
  // 应包含 type 字段
  for (const p of list) {
    assert.ok('type' in p, `party ${p.name} 应有 type 字段`)
  }
})

test('POST /api/parties 可以新建往来单位（引导页核心动作）', async () => {
  const res = await httpReq('POST', '/api/parties', {
    name: '测试公司',
    type: 'invest',
  })
  assert.equal(res.status, 200)
  assert.ok(res.data, '应有响应')
})

test('GET /api/settings 返回银行期初余额（引导页第二页数据源）', async () => {
  const res = await httpReq('GET', '/api/settings')
  assert.equal(res.status, 200)
  const s = res.data.data
  assert.ok(s, 'settings 应有数据')
  assert.ok('bankBalanceCents' in s, '应有 bankBalanceCents 字段')
})

test('PUT /api/settings 可以更新银行期初', async () => {
  const res = await httpReq('PUT', '/api/settings', {
    bankBalanceCents: 50000000,
  })
  assert.equal(res.status, 200)
})

test('PUT /api/categories/:id 可以更新期初余额（引导页第二页核心动作）', async () => {
  const { CATEGORIES } = require('../mock')
  // 找一个 preset 的二级科目
  const presetL2 = CATEGORIES.flatMap(l1 => l1.children || []).find(c => c.preset)
  assert.ok(presetL2, '应能找到一个 preset L2')
  const res = await httpReq('PUT', `/api/categories/${presetL2.id}`, {
    openingBalanceCents: 1234567,
  })
  assert.equal(res.status, 200, `更新期初应成功，响应: ${JSON.stringify(res.data).substring(0, 100)}`)
})

test('POST /api/categories 可以在长期投资下新建二级科目（引导页 invest 单位自动建）', async () => {
  const { CATEGORIES } = require('../mock')
  const investL1 = CATEGORIES.find(c => c.name === '长期投资')
  assert.ok(investL1, '应存在长期投资 L1')
  const res = await httpReq('POST', '/api/categories', {
    name: '测试自动建的公司',
    level: 2,
    parentId: investL1.id,
    kind: 'equity',
  })
  assert.equal(res.status, 200)
})

// ================ 四、GET /api/summary 结构验证（回归） ================

test('GET /api/summary categories 是 9 个 L1 的 buildCategorySummary 结果', async () => {
  const res = await httpReq('GET', '/api/summary')
  assert.equal(res.status, 200)
  const s = res.data.data
  assert.ok(s, 'summary 应有数据')
  assert.ok(Array.isArray(s.categories), 'categories 应是数组')
  assert.equal(s.categories.length, 9, `summary.categories 应有 9 个 L1`)
  // 投资收益、再投资收益和再投资是空 children
  for (const cat of s.categories) {
    assert.ok(Array.isArray(cat.children), `${cat.name} children 应是数组（空容器也应为 []）`)
  }
})

