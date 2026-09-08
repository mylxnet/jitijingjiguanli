-- 010_contract_and_allocation.sql — 补后端缺失接口的落库表（只增不改）
-- 1) contract：合同/附件，文件内容以 BLOB 存 SQLite（单文件备份，天然一致）
-- 2) reinvest_allocation：再投资去向明细（party 子表）

CREATE TABLE IF NOT EXISTS contract (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id         INTEGER NOT NULL REFERENCES org(id),
    party_id       INTEGER NOT NULL REFERENCES party(id),
    file_name      TEXT    NOT NULL,
    file_size      INTEGER NOT NULL DEFAULT 0,
    mime_type      TEXT    NOT NULL DEFAULT 'application/octet-stream',
    contract_title TEXT    NOT NULL DEFAULT '',
    contract_date  TEXT,                      -- nullable
    expires_at     TEXT,                      -- nullable
    file_data      BLOB,                      -- 原始二进制；列表查询不选此列
    created_at     DATETIME NOT NULL,
    updated_at     DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_contract_org_party ON contract(org_id, party_id);

CREATE TABLE IF NOT EXISTS reinvest_allocation (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id          INTEGER NOT NULL REFERENCES org(id),
    party_id        INTEGER NOT NULL REFERENCES party(id),
    target_name     TEXT    NOT NULL,
    target_party_id INTEGER REFERENCES party(id),   -- 可选
    amount_cents    INTEGER NOT NULL CHECK (amount_cents > 0),
    notes           TEXT,
    created_at      DATETIME NOT NULL,
    updated_at      DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_realloc_org_party ON reinvest_allocation(org_id, party_id);