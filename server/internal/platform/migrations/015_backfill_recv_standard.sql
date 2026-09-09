-- 015_backfill_recv_standard.sql — 存量单位计提标准回填修复。
-- 背景：v0.13 新增「单位费用自动同步计提标准」仅覆盖新建/编辑后的写入口；
--       更早期创建的单位（如经新增单位弹窗直接填费用）费用已存在 party.expected_*，
--       但 recv_standard 缺失，导致年度计提预览不显示这些单位（表现为"计提空白，
--       手动编辑一次后才出现"）。
-- 说明：按单位类型把已有费用回填为标准；已有标准或费用为 0/空则跳过（幂等）。
-- 标准表迁移 014 已放行 service，此处一并回填流转费(rent)/管理费(service)/投资收益(dividend)。

INSERT INTO recv_standard(org_id, party_id, recv_kind, amount_cents, active, created_at, updated_at)
SELECT p.org_id, p.id, 'rent', p.expected_land_fee_cents, 1, datetime('now'), datetime('now')
FROM party p
WHERE p.type = 'flow' AND p.expected_land_fee_cents > 0
  AND NOT EXISTS (
    SELECT 1 FROM recv_standard s
    WHERE s.org_id = p.org_id AND s.party_id = p.id AND s.recv_kind = 'rent'
  );

INSERT INTO recv_standard(org_id, party_id, recv_kind, amount_cents, active, created_at, updated_at)
SELECT p.org_id, p.id, 'service', p.expected_mgmt_fee_cents, 1, datetime('now'), datetime('now')
FROM party p
WHERE p.type = 'flow' AND p.expected_mgmt_fee_cents > 0
  AND NOT EXISTS (
    SELECT 1 FROM recv_standard s
    WHERE s.org_id = p.org_id AND s.party_id = p.id AND s.recv_kind = 'service'
  );

INSERT INTO recv_standard(org_id, party_id, recv_kind, amount_cents, active, created_at, updated_at)
SELECT p.org_id, p.id, 'dividend', p.expected_return_cents, 1, datetime('now'), datetime('now')
FROM party p
WHERE p.type IN ('invest', 'reinvest') AND p.expected_return_cents > 0
  AND NOT EXISTS (
    SELECT 1 FROM recv_standard s
    WHERE s.org_id = p.org_id AND s.party_id = p.id AND s.recv_kind = 'dividend'
  );
