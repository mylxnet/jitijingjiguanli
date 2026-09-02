-- 001_init.sql — v0.1.0 首批表
-- 设计依据：docs/03-design.md §4.1；金额一律为「分」的整数，禁止浮点。

CREATE TABLE IF NOT EXISTS user (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    created_at    DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS session (
    id         TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_user ON session(user_id);

-- 严格两级科目：一级 parent_id IS NULL，二级必填 parent_id。
-- balance_type：residual=余粮型(收增支减) / spending=花费型(支增收减)（需求文档 D6）
CREATE TABLE IF NOT EXISTS category (
    id                         INTEGER PRIMARY KEY AUTOINCREMENT,
    name                       TEXT    NOT NULL,
    level                      INTEGER NOT NULL CHECK (level IN (1,2)),
    parent_id                  INTEGER REFERENCES category(id),
    status                     TEXT    NOT NULL DEFAULT 'active'
                                       CHECK (status IN ('active','inactive')),
    balance_type               TEXT    NOT NULL DEFAULT 'residual'
                                       CHECK (balance_type IN ('residual','spending')),
    opening_balance_cents      INTEGER NOT NULL DEFAULT 0,
    include_in_reconciliation  INTEGER NOT NULL DEFAULT 0
                                       CHECK (include_in_reconciliation IN (0,1)),
    sort_order                 INTEGER NOT NULL DEFAULT 0,
    created_at                 DATETIME NOT NULL,
    updated_at                 DATETIME NOT NULL,
    -- 花费型科目不得参与资金勾稽（否则「未分配」被二次扣减，见 D6 强制校验）
    CHECK (NOT (balance_type = 'spending' AND include_in_reconciliation = 1))
);
-- 同级重名唯一（一级：parent_id IS NULL 单独索引）
CREATE UNIQUE INDEX IF NOT EXISTS idx_category_l1_name
    ON category(parent_id, name) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_category_l2_name
    ON category(parent_id, name) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_category_parent ON category(parent_id);

-- 收支流水。金额以分为单位的正整数。direction 收=income 支=expense。
-- ⚠️ 表名用 txn：transaction 是 SQLite 保留字。对应领域实体「transaction（流水）」，见 03-design §4.1。
CREATE TABLE IF NOT EXISTS txn (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    txn_date      TEXT    NOT NULL,                -- YYYY-MM-DD
    direction     TEXT    NOT NULL CHECK (direction IN ('income','expense')),
    amount_cents  INTEGER NOT NULL CHECK (amount_cents > 0),
    category_id   INTEGER NOT NULL REFERENCES category(id),
    note          TEXT,
    status        TEXT    NOT NULL DEFAULT 'normal' CHECK (status IN ('normal','voided')),
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_txn_date      ON txn(txn_date DESC);
CREATE INDEX IF NOT EXISTS idx_txn_category  ON txn(category_id);
CREATE INDEX IF NOT EXISTS idx_txn_status    ON txn(status);
CREATE INDEX IF NOT EXISTS idx_txn_date_status ON txn(txn_date, status);
