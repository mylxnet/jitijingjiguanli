-- 013_distribution_532.sql — v0.11：532 分配方案表。
-- 年度收益按 50% 再投资 / 30% 分红福利 / 20% 管理公益 分配，按年一条（org+year 唯一）。
-- total_income_cents 记录分配基准总投资收益余额（写入时快照）。
CREATE TABLE IF NOT EXISTS distribution_532 (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id             INTEGER NOT NULL REFERENCES org(id),
    year               INTEGER NOT NULL,
    total_income_cents INTEGER NOT NULL DEFAULT 0, -- 分配基准总收益（分）
    reinvest_cents     INTEGER NOT NULL DEFAULT 0, -- 再投资（50%）
    dividend_cents     INTEGER NOT NULL DEFAULT 0, -- 成员分红福利（30%）
    welfare_cents      INTEGER NOT NULL DEFAULT 0, -- 管理公益支出（20%）
    created_at         DATETIME NOT NULL,
    updated_at         DATETIME NOT NULL,
    UNIQUE(org_id, year)
);
CREATE INDEX IF NOT EXISTS idx_dist532_org ON distribution_532(org_id);