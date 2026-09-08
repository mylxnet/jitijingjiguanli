-- 011: 往来单位年度数据列
-- 此前这些字段仅在 Party 结构体声明，未落库导致"年度数据"表单保存即丢。
ALTER TABLE party ADD COLUMN invest_amount_cents      INTEGER NOT NULL DEFAULT 0; -- 投资本金（分）
ALTER TABLE party ADD COLUMN return_rate_bps          INTEGER NOT NULL DEFAULT 0; -- 收益率基点（500 = 5.00%）
ALTER TABLE party ADD COLUMN expected_return_cents    INTEGER NOT NULL DEFAULT 0; -- 年收益（分）
ALTER TABLE party ADD COLUMN land_mu                   REAL    NOT NULL DEFAULT 0; -- 流转亩数
ALTER TABLE party ADD COLUMN land_fee_per_mu_cents     INTEGER NOT NULL DEFAULT 0; -- 每亩年流转费（分）
ALTER TABLE party ADD COLUMN expected_land_fee_cents   INTEGER NOT NULL DEFAULT 0; -- 总流转费（分）
ALTER TABLE party ADD COLUMN mgmt_fee_per_mu_cents     INTEGER NOT NULL DEFAULT 0; -- 每亩年管理费（分）
ALTER TABLE party ADD COLUMN expected_mgmt_fee_cents   INTEGER NOT NULL DEFAULT 0; -- 总管理费（分）