-- 018_receipt_writeoff.sql — 坏账核销：receipt.method 增加 'writeoff'。
-- 背景：坏账需把应收单「剩余待收」清零，且不影响账目（不生成任何流水），
--       作为一种独立的核销方式留痕（可撤销），与 cash（现金入账）/offset（抵销）并列。
-- 说明：ALTER 无法改 CHECK，需重建 receipt 表；receipt 外键引用 receivable，
--       用 PRAGMA defer_foreign_keys=ON 将 FK 校验延迟到提交；重建后保留原 id，引用依旧成立。
PRAGMA defer_foreign_keys=ON;

CREATE TABLE receipt_new (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id        INTEGER NOT NULL REFERENCES org(id),
    receivable_id INTEGER NOT NULL REFERENCES receivable(id),
    amount_cents  INTEGER NOT NULL CHECK (amount_cents > 0),
    receipt_date  TEXT    NOT NULL,
    method        TEXT    NOT NULL CHECK (method IN ('cash','offset','writeoff')),
    txn_id        INTEGER,
    note          TEXT,
    status        TEXT    NOT NULL DEFAULT 'normal'
                          CHECK (status IN ('normal','voided')),
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);

INSERT INTO receipt_new(id, org_id, receivable_id, amount_cents, receipt_date, method, txn_id, note, status, created_at, updated_at)
SELECT id, org_id, receivable_id, amount_cents, receipt_date, method, txn_id, note, status, created_at, updated_at FROM receipt;

DROP TABLE receipt;
ALTER TABLE receipt_new RENAME TO receipt;

CREATE INDEX IF NOT EXISTS idx_receipt_org_recv ON receipt(org_id, receivable_id);

PRAGMA defer_foreign_keys=OFF;
