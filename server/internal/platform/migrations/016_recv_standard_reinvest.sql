-- 016_recv_standard_reinvest.sql — 计提标准放行「再投资收益 reinvest_dividend」。
-- 背景：再投资单位的收益此前被统一写成 dividend 标准，而 dividend 在预览/结转中只匹配
--       invest 单位，导致再投资收益无法进入年度计提（整条链路断裂）。
-- 说明：重建 recv_standard 放行 reinvest_dividend；并把再投资单位既有的 dividend 标准
--       迁移为 reinvest_dividend（历史应收单保持原类别不动）。
PRAGMA defer_foreign_keys=ON;

CREATE TABLE recv_standard_new (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id       INTEGER NOT NULL REFERENCES org(id),
    party_id     INTEGER NOT NULL REFERENCES party(id),
    recv_kind    TEXT    NOT NULL CHECK (recv_kind IN ('rent','dividend','service','reinvest_dividend','other')),
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    active       INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1)),
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

INSERT INTO recv_standard_new(id, org_id, party_id, recv_kind, amount_cents, active, created_at, updated_at)
SELECT id, org_id, party_id, recv_kind, amount_cents, active, created_at, updated_at FROM recv_standard;

DROP TABLE recv_standard;
ALTER TABLE recv_standard_new RENAME TO recv_standard;
CREATE UNIQUE INDEX IF NOT EXISTS idx_recv_standard_org_party_kind
    ON recv_standard(org_id, party_id, recv_kind);

-- 先删除会与目标类别冲突的既有 reinvest_dividend 行（极少数重复场景）
DELETE FROM recv_standard
WHERE recv_kind = 'dividend'
  AND party_id IN (SELECT id FROM party WHERE type = 'reinvest')
  AND EXISTS (
      SELECT 1 FROM recv_standard s2
      WHERE s2.org_id = recv_standard.org_id
        AND s2.party_id = recv_standard.party_id
        AND s2.recv_kind = 'reinvest_dividend'
  );

-- 再投资单位的 dividend 标准 → reinvest_dividend
UPDATE recv_standard
SET recv_kind = 'reinvest_dividend', updated_at = datetime('now')
WHERE recv_kind = 'dividend'
  AND party_id IN (SELECT id FROM party WHERE type = 'reinvest');
