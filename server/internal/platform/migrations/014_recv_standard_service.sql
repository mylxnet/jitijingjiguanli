-- 014_recv_standard_service.sql — 放宽计提标准类别，纳入「管理费 service」。
-- 背景：recv_standard 的 CHECK 仅允许 rent/dividend/other，但管理费（service）也应支持
--       按年计提：引导页可填流转费+管理费，需同步写入标准表；否则管理费标准无法保存、
--       年度计提向导「管理费」步恒为空。ALTER 无法改 CHECK，重建表并保留原数据。
PRAGMA defer_foreign_keys=ON;

CREATE TABLE recv_standard_new (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id       INTEGER NOT NULL REFERENCES org(id),
    party_id     INTEGER NOT NULL REFERENCES party(id),
    recv_kind    TEXT    NOT NULL CHECK (recv_kind IN ('rent','dividend','service','other')),
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
