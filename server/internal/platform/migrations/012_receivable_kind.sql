-- 012_receivable_kind.sql — 加宽应收单类别，支持「管理费 service」与「再投资收益 reinvest_dividend」。
-- 背景：recv_kind 原 CHECK 仅允许 rent/dividend/other，年度计提引导页会生成 service 应收，插入被拒。
-- 说明：需重建 receivable 表（ALTER 无法改 CHECK）。receipt 外键引用 receivable，
--       用 PRAGMA defer_foreign_keys=ON 将 FK 校验延迟到提交；重建后保留原 id，引用依旧成立。
PRAGMA defer_foreign_keys=ON;

CREATE TABLE receivable_new (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id             INTEGER NOT NULL REFERENCES org(id),
    party_id           INTEGER NOT NULL REFERENCES party(id),
    recv_year          INTEGER NOT NULL DEFAULT 0,
    recv_kind          TEXT    NOT NULL CHECK (recv_kind IN ('rent','dividend','service','reinvest_dividend','other')),
    title              TEXT    NOT NULL,
    amount_cents       INTEGER NOT NULL CHECK (amount_cents > 0),
    income_category_id INTEGER REFERENCES category(id),
    status             TEXT    NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
    note               TEXT,
    created_at         DATETIME NOT NULL,
    updated_at         DATETIME NOT NULL
);

INSERT INTO receivable_new(id, org_id, party_id, recv_year, recv_kind, title, amount_cents, income_category_id, status, note, created_at, updated_at)
SELECT id, org_id, party_id, recv_year, recv_kind, title, amount_cents, income_category_id, status, note, created_at, updated_at FROM receivable;

DROP TABLE receivable;
ALTER TABLE receivable_new RENAME TO receivable;

CREATE INDEX IF NOT EXISTS idx_receivable_org_party ON receivable(org_id, party_id);

PRAGMA defer_foreign_keys=OFF;