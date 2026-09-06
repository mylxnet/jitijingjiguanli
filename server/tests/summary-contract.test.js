/**
 * 验证 /api/summary 返回结构满足前端 SummaryResponse 契约。
 * 历史 bug：mock 只返回 { incomeTotal, expenseTotal, capital }，
 * 缺 balance 和 categories → 前端访问 res.data.categories.length 抛 TypeError → "加载失败"。
 *
 * 跑测试:  node --test server/tests/summary-contract.test.js
 */
const { test } = require('node:test');
const assert = require('node:assert/strict');

const { SUMMARY, CATEGORIES, buildCategorySummary } = require('../mock.js');

// ---- 顶层 SUMMARY 结构 ----
test('SummaryResponse 顶层必须包含所有必需字段', () => {
  for (const key of ['incomeTotal', 'expenseTotal', 'balance', 'capital', 'categories']) {
    assert.ok(key in SUMMARY, `SUMMARY 缺少字段: ${key}`);
  }
});

test('balance = incomeTotal - expenseTotal', () => {
  assert.equal(SUMMARY.balance, SUMMARY.incomeTotal - SUMMARY.expenseTotal,
    `balance 应等于 incomeTotal(${SUMMARY.incomeTotal}) - expenseTotal(${SUMMARY.expenseTotal})`);
});

test('capital 对象包含三项资产', () => {
  for (const key of ['bankBalanceCents', 'assetTotalCents', 'equityTotalCents']) {
    assert.ok(key in SUMMARY.capital, `capital 缺少字段: ${key}`);
  }
  assert.equal(typeof SUMMARY.capital.bankBalanceCents, 'number');
  assert.equal(typeof SUMMARY.capital.assetTotalCents, 'number');
  assert.equal(typeof SUMMARY.capital.equityTotalCents, 'number');
});

// ---- categories 树结构 (就是之前缺的这个字段!) ----
test('categories 是非空数组', () => {
  assert.ok(Array.isArray(SUMMARY.categories), 'categories 必须是数组');
  assert.ok(SUMMARY.categories.length > 0, 'categories 不能为空');
});

test('每个一级科目包含 SummaryResponse CategorySummary 必需字段', () => {
  const L1_REQUIRED = ['id', 'name', 'level', 'currentBalanceCents', 'txnCount', 'incomeCents', 'expenseCents', 'children'];
  for (const l1 of SUMMARY.categories) {
    for (const key of L1_REQUIRED) {
      assert.ok(key in l1, `L1 ${l1.name} 缺少字段: ${key}`);
    }
  }
});

test('每个二级科目包含 CategorySummary 必需字段 + kind', () => {
  const L2_REQUIRED = ['id', 'name', 'level', 'parentId', 'kind', 'currentBalanceCents', 'txnCount'];
  for (const l1 of SUMMARY.categories) {
    for (const l2 of (l1.children || [])) {
      for (const key of L2_REQUIRED) {
        assert.ok(key in l2, `L2 ${l2.name} 缺少字段: ${key}`);
      }
      assert.ok(['equity', 'asset'].includes(l2.kind), `L2 ${l2.name} kind 必须是 equity 或 asset`);
    }
  }
});

// ---- 余额累加一致性 ----
test('一级科目余额 = 所有二级科目余额之和', () => {
  for (const l1 of SUMMARY.categories) {
    const sum = (l1.children || []).reduce((s, c) => s + (c.currentBalanceCents || 0), 0);
    assert.equal(l1.currentBalanceCents, sum,
      `L1 [${l1.name}] currentBalanceCents(${l1.currentBalanceCents}) != children 之和(${sum})`);
  }
});

test('buildCategorySummary 对空 children 不崩', () => {
  const empty = buildCategorySummary();
  assert.ok(Array.isArray(empty));
  // CATEGORIES 里本金只有 1 个子科目，其他都应该能正确走 children 路径
  for (const c of empty) {
    assert.equal(typeof c.children, 'object');
    assert.ok(Array.isArray(c.children));
  }
});

test('原始 CATEGORIES 是 SummaryResponse categories 的数据来源', () => {
  // 每个原始 L1 都应该在 SUMMARY.categories 中存在
  const summaryIds = new Set(SUMMARY.categories.map(c => c.id));
  for (const l1 of CATEGORIES) {
    assert.ok(summaryIds.has(l1.id), `原始 L1 [${l1.name}] 在 summary.categories 中找不到`);
  }
});

// ---- 数值合理性 ----
test('所有金额字段是整数（分，不含小数）', () => {
  const checkInt = (v, path) => {
    assert.equal(typeof v, 'number', `${path} 应为 number`);
    assert.ok(Number.isInteger(v), `${path} 应为整数，实际: ${v}`);
  };

  checkInt(SUMMARY.incomeTotal, 'incomeTotal');
  checkInt(SUMMARY.expenseTotal, 'expenseTotal');
  checkInt(SUMMARY.balance, 'balance');
  checkInt(SUMMARY.capital.bankBalanceCents, 'capital.bankBalanceCents');
  checkInt(SUMMARY.capital.assetTotalCents, 'capital.assetTotalCents');
  checkInt(SUMMARY.capital.equityTotalCents, 'capital.equityTotalCents');

  for (const l1 of SUMMARY.categories) {
    checkInt(l1.currentBalanceCents, `categories[${l1.name}].currentBalanceCents`);
    for (const l2 of (l1.children || [])) {
      checkInt(l2.currentBalanceCents, `categories[${l1.name}].children[${l2.name}].currentBalanceCents`);
      checkInt(l2.txnCount, `categories[${l1.name}].children[${l2.name}].txnCount`);
    }
  }
});
