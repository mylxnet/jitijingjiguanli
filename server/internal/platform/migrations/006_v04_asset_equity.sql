-- 006_v04_asset_equity.sql — v0.4 模型重置（破坏性升级，仅测试数据）
-- 需求决策（用户逐案确认）：
--  1) 科目二级类型只有「资产类 asset / 权益类 equity」，删除余额类型(balance_type)、
--     参与勾稽(include_in_reconciliation)、科目期初(opening_balance_cents) 与“专项资金/未分配”口径
--  2) equity 记收支与科目间转账；asset 只能资金划转（fund_move）
--  3) 银行存款期初保留在 app_setting（银行独立资金池）
--  4) receivable 增加 recv_year（批量计提按年度结转、防重）
--  5) 532 仅按比例记账，不建方案台账
-- 重建顺序：先删引用方，再删被引用方；重建相反（与 003 同法）。

DROP TABLE IF EXISTS receipt;
DROP TABLE IF EXISTS receivable;
DROP TABLE IF EXISTS txn;
DROP TABLE IF EXISTS transfer_leg;
DROP TABLE IF EXISTS transfer;
DROP TABLE IF EXISTS fund_move;
DROP TABLE IF EXISTS change_log;
DROP TABLE IF EXISTS app_setting;
DROP TABLE IF EXISTS category;
DROP TABLE IF EXISTS party;
DROP TABLE IF EXISTS session;
DROP TABLE IF EXISTS user;
DROP TABLE IF EXISTS org;

CREATE TABLE IF NOT EXISTS org (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS user (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id        INTEGER NOT NULL REFERENCES org(id),
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    created_at    DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_user_org ON user(org_id);

CREATE TABLE IF NOT EXISTS session (
    id         TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_user ON session(user_id);

-- 科目树（两级）。kind：asset 资产 / equity 权益（v0.4）
-- asset 仅二级、只能资金划转；equity 二级记收支/转账。无期初/无勾稽/无余额类型。
CREATE TABLE IF NOT EXISTS category (
    id                         INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id                     INTEGER NOT NULL REFERENCES org(id),
    name                       TEXT    NOT NULL,
    level                      INTEGER NOT NULL CHECK (level IN (1,2)),
    parent_id                  INTEGER REFERENCES category(id),
    status                     TEXT    NOT NULL DEFAULT 'active'
                                       CHECK (status IN ('active','inactive')),
    kind                       TEXT    NOT NULL DEFAULT 'equity'
                                       CHECK (kind IN ('asset','equity')),
    preset                     INTEGER NOT NULL DEFAULT 0 CHECK (preset IN (0,1)),
    sort_order                 INTEGER NOT NULL DEFAULT 0,
    created_at                 DATETIME NOT NULL,
    updated_at                 DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_category_l1_name
    ON category(org_id, parent_id, name) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_category_l2_name
    ON category(org_id, parent_id, name) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_category_org ON category(org_id);
CREATE INDEX IF NOT EXISTS idx_category_parent ON category(parent_id);

-- 收支流水（进出银行存款；只挂 equity 二级）
CREATE TABLE IF NOT EXISTS txn (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id        INTEGER NOT NULL REFERENCES org(id),
    txn_date      TEXT    NOT NULL,
    direction     TEXT    NOT NULL CHECK (direction IN ('income','expense')),
    amount_cents  INTEGER NOT NULL CHECK (amount_cents > 0),
    category_id   INTEGER NOT NULL REFERENCES category(id),
    note          TEXT,
    status        TEXT    NOT NULL DEFAULT 'normal' CHECK (status IN ('normal','voided')),
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_txn_org_date ON txn(org_id, txn_date DESC);
CREATE INDEX IF NOT EXISTS idx_txn_category ON txn(category_id);

-- 科目间转账（equity 之间；1 转出 → N 转入）
CREATE TABLE IF NOT EXISTS transfer (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id              INTEGER NOT NULL REFERENCES org(id),
    txn_date            TEXT    NOT NULL,
    source_category_id  INTEGER NOT NULL REFERENCES category(id),
    source_amount_cents INTEGER NOT NULL CHECK (source_amount_cents > 0),
    note                TEXT,
    status              TEXT    NOT NULL DEFAULT 'normal'
                                CHECK (status IN ('normal','voided')),
    created_at          DATETIME NOT NULL,
    updated_at          DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_transfer_org_date ON transfer(org_id, txn_date DESC);
CREATE INDEX IF NOT EXISTS idx_transfer_source ON transfer(source_category_id);

CREATE TABLE IF NOT EXISTS transfer_leg (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id       INTEGER NOT NULL REFERENCES org(id),
    transfer_id  INTEGER NOT NULL REFERENCES transfer(id) ON DELETE CASCADE,
    category_id  INTEGER NOT NULL REFERENCES category(id),
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0)
);
CREATE INDEX IF NOT EXISTS idx_transfer_leg_transfer ON transfer_leg(transfer_id);
CREATE INDEX IF NOT EXISTS idx_transfer_leg_org ON transfer_leg(org_id);

-- 资金划转（D10 保留）：银行 ↔ asset 二级
CREATE TABLE IF NOT EXISTS fund_move (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id            INTEGER NOT NULL REFERENCES org(id),
    move_date         TEXT    NOT NULL,
    kind              TEXT    NOT NULL CHECK (kind IN ('invest','recover')),
    asset_category_id INTEGER NOT NULL REFERENCES category(id),
    amount_cents      INTEGER NOT NULL CHECK (amount_cents > 0),
    note              TEXT,
    status            TEXT    NOT NULL DEFAULT 'normal'
                              CHECK (status IN ('normal','voided')),
    created_at        DATETIME NOT NULL,
    updated_at        DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_fundmove_org_date ON fund_move(org_id, move_date DESC);
CREATE INDEX IF NOT EXISTS idx_fundmove_asset ON fund_move(asset_category_id);

-- 往来单位（只有单位；无 kind 农户）
CREATE TABLE IF NOT EXISTS party (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id     INTEGER NOT NULL REFERENCES org(id),
    name       TEXT    NOT NULL,
    note       TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_party_org ON party(org_id);

-- 应收单：recv_year=归属年度（批量计提结转用，防重复）；income_category_id 收款自动入账 equity 二级
CREATE TABLE IF NOT EXISTS receivable (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id             INTEGER NOT NULL REFERENCES org(id),
    party_id           INTEGER NOT NULL REFERENCES party(id),
    recv_year          INTEGER NOT NULL DEFAULT 0,
    recv_kind          TEXT    NOT NULL CHECK (recv_kind IN ('rent','dividend','other')),
    title              TEXT    NOT NULL,
    amount_cents       INTEGER NOT NULL CHECK (amount_cents > 0),
    income_category_id INTEGER REFERENCES category(id),
    status             TEXT    NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
    note               TEXT,
    created_at         DATETIME NOT NULL,
    updated_at         DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_receivable_org_party ON receivable(org_id, party_id);

-- 年度计提标准（如土地流转费每年金额；10月一键结转生成应收单，可按 party+kind 防重）
CREATE TABLE IF NOT EXISTS recv_standard (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id       INTEGER NOT NULL REFERENCES org(id),
    party_id     INTEGER NOT NULL REFERENCES party(id),
    recv_kind    TEXT    NOT NULL CHECK (recv_kind IN ('rent','dividend','other')),
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    active       INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1)),
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_recv_standard_org_party_kind
    ON recv_standard(org_id, party_id, recv_kind);

-- 收款核销（cash=现金入账自动生成银行收入流水；offset=抵销关联支出流水；status 由 004 并入）
CREATE TABLE IF NOT EXISTS receipt (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id        INTEGER NOT NULL REFERENCES org(id),
    receivable_id INTEGER NOT NULL REFERENCES receivable(id),
    amount_cents  INTEGER NOT NULL CHECK (amount_cents > 0),
    receipt_date  TEXT    NOT NULL,
    method        TEXT    NOT NULL CHECK (method IN ('cash','offset')),
    txn_id        INTEGER,
    note          TEXT,
    status        TEXT    NOT NULL DEFAULT 'normal'
                          CHECK (status IN ('normal','voided')),
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_receipt_org_recv ON receipt(org_id, receivable_id);

-- 留痕
CREATE TABLE IF NOT EXISTS change_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id      INTEGER NOT NULL REFERENCES org(id),
    entity_type TEXT    NOT NULL,
    entity_id   INTEGER NOT NULL,
    action      TEXT    NOT NULL,
    field       TEXT,
    old_value   TEXT,
    new_value   TEXT,
    changed_at  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_changelog_org_entity ON change_log(org_id, entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_changelog_time ON change_log(changed_at);

-- 系统配置（银行期初等，主键 org+key）
CREATE TABLE IF NOT EXISTS app_setting (
    org_id INTEGER NOT NULL REFERENCES org(id),
    key    TEXT    NOT NULL,
    value  TEXT    NOT NULL,
    PRIMARY KEY (org_id, key)
);
