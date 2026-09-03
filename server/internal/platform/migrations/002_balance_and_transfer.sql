-- 002_balance_and_transfer.sql — v0.2.0 新增表
-- 设计依据：docs/03-design.md §4.1；需求文档 D6（余额模型）与 D7（转账模型）
-- 金额一律为「分」的整数，禁止浮点。

-- 系统配置（键值对，见 D6 银行存款期初余额）
CREATE TABLE IF NOT EXISTS app_setting (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- 转账（1 转出 → N 转入，不涉及银行存款，见 D7）
CREATE TABLE IF NOT EXISTS transfer (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    txn_date            TEXT    NOT NULL,                -- YYYY-MM-DD
    source_category_id  INTEGER NOT NULL REFERENCES category(id),
    source_amount_cents INTEGER NOT NULL CHECK (source_amount_cents > 0),
    note                TEXT,
    status              TEXT    NOT NULL DEFAULT 'normal'
                                CHECK (status IN ('normal','voided')),
    created_at          DATETIME NOT NULL,
    updated_at          DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_transfer_date ON transfer(txn_date DESC);
CREATE INDEX IF NOT EXISTS idx_transfer_source ON transfer(source_category_id);

-- 转账转入明细（R2：Σ(leg.amount) = source_amount；R4：花费型不得作转入方，应用层校验）
CREATE TABLE IF NOT EXISTS transfer_leg (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    transfer_id  INTEGER NOT NULL REFERENCES transfer(id) ON DELETE CASCADE,
    category_id  INTEGER NOT NULL REFERENCES category(id),
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0)
);
CREATE INDEX IF NOT EXISTS idx_transfer_leg_transfer ON transfer_leg(transfer_id);
CREATE INDEX IF NOT EXISTS idx_transfer_leg_category ON transfer_leg(category_id);

-- 变更留痕（D0 可追溯优先，D3 编辑自动留痕）
CREATE TABLE IF NOT EXISTS change_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT    NOT NULL,          -- 'transaction' / 'category' / 'transfer'
    entity_id   INTEGER NOT NULL,
    action      TEXT    NOT NULL,          -- 'create' / 'update' / 'void' / 'unvoid'
    field       TEXT,                      -- 变更字段名（action=update 时有效）
    old_value   TEXT,
    new_value   TEXT,
    changed_at  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_changelog_entity ON change_log(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_changelog_time ON change_log(changed_at);