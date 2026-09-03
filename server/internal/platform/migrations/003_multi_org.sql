-- 003_multi_org.sql — v0.3.0 多组织 + 资产/应收（结构性重建）
-- 设计依据：docs/03-design.md §9.1；需求文档 v0.3（D9 多组织 / D10 资产 / D11 应收）
-- ⚠️ v0.3 为破坏性升级：旧（单组织）数据作废（需求决策：旧测试库清空重来）。
--    故本迁移 DROP 全部业务表后按新结构重建（全表带 org_id）。
--    迁移执行顺序受外键约束：先删引用方，再删被引用方；重建顺序相反。

DROP TABLE IF EXISTS txn;
DROP TABLE IF EXISTS transfer_leg;
DROP TABLE IF EXISTS transfer;
DROP TABLE IF EXISTS change_log;
DROP TABLE IF EXISTS app_setting;
DROP TABLE IF EXISTS fund_move;
DROP TABLE IF EXISTS party;
DROP TABLE IF EXISTS receivable;
DROP TABLE IF EXISTS receipt;
DROP TABLE IF EXISTS category;
DROP TABLE IF EXISTS session;
DROP TABLE IF EXISTS user;
DROP TABLE IF EXISTS org;

-- 集体经济组织（租户）。一套部署多组织独立记账（D9）。
CREATE TABLE IF NOT EXISTS org (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- 账号：一个组织一个管理员账号（注册制）；org_id 预留组织内多账号扩展（N:1）。
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

-- 科目树（两级）——严格两级：一级 parent_id IS NULL，二级必填 parent_id。
-- balance_type：residual=余粮(看还剩多少) / spending=花费(看累计花了多少)（D6）
-- kind：normal=普通（收支/转账挂它）/ asset=资产型（银行外资产，只走资金划转 fund_move）（D10）
-- preset=1 系统预置（注册时生成，可改名/增删）
CREATE TABLE IF NOT EXISTS category (
    id                         INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id                     INTEGER NOT NULL REFERENCES org(id),
    name                       TEXT    NOT NULL,
    level                      INTEGER NOT NULL CHECK (level IN (1,2)),
    parent_id                  INTEGER REFERENCES category(id),
    status                     TEXT    NOT NULL DEFAULT 'active'
                                       CHECK (status IN ('active','inactive')),
    balance_type               TEXT    NOT NULL DEFAULT 'residual'
                                       CHECK (balance_type IN ('residual','spending')),
    kind                       TEXT    NOT NULL DEFAULT 'normal'
                                       CHECK (kind IN ('normal','asset')),
    opening_balance_cents      INTEGER NOT NULL DEFAULT 0,
    include_in_reconciliation  INTEGER NOT NULL DEFAULT 0
                                       CHECK (include_in_reconciliation IN (0,1)),
    preset                     INTEGER NOT NULL DEFAULT 0 CHECK (preset IN (0,1)),
    sort_order                 INTEGER NOT NULL DEFAULT 0,
    created_at                 DATETIME NOT NULL,
    updated_at                 DATETIME NOT NULL,
    CHECK (NOT (balance_type = 'spending' AND include_in_reconciliation = 1)),
    CHECK (NOT (kind = 'asset' AND include_in_reconciliation = 1)),
    CHECK (NOT (kind = 'asset' AND balance_type = 'spending'))
);
-- 同级重名唯一（同组织内）
CREATE UNIQUE INDEX IF NOT EXISTS idx_category_l1_name
    ON category(org_id, parent_id, name) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_category_l2_name
    ON category(org_id, parent_id, name) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_category_org ON category(org_id);
CREATE INDEX IF NOT EXISTS idx_category_parent ON category(parent_id);

-- 收支流水（进出银行存款，必须挂 normal 科目）
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

-- 科目间转账（不碰银行，1 转出 → N 转入，D7；仅 normal 科目）
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

-- 变更留痕（D0/D3）
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

-- 系统配置（按组织，主键 (org_id,key)）
CREATE TABLE IF NOT EXISTS app_setting (
    org_id INTEGER NOT NULL REFERENCES org(id),
    key    TEXT    NOT NULL,
    value  TEXT    NOT NULL,
    PRIMARY KEY (org_id, key)
);

-- 资金划转（D10）：投资 = 银行→资产科目；收回 = 资产科目→银行
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

-- 往来对象（D11）
CREATE TABLE IF NOT EXISTS party (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id     INTEGER NOT NULL REFERENCES org(id),
    name       TEXT    NOT NULL,
    kind       TEXT    NOT NULL DEFAULT 'household' CHECK (kind IN ('household','unit')),
    note       TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_party_org ON party(org_id);

-- 应收单（D11）：欠款记录；欠款余额 = amount − Σ(未作废 receipt)
CREATE TABLE IF NOT EXISTS receivable (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id             INTEGER NOT NULL REFERENCES org(id),
    party_id           INTEGER NOT NULL REFERENCES party(id),
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

-- 收款核销（D11）：method=cash 现金（自动生成银行收入流水并回写 txn_id）/
--                        offset 抵销（关联一条发放支出流水 txn_id，不产生现金流水）
CREATE TABLE IF NOT EXISTS receipt (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id        INTEGER NOT NULL REFERENCES org(id),
    receivable_id INTEGER NOT NULL REFERENCES receivable(id),
    amount_cents  INTEGER NOT NULL CHECK (amount_cents > 0),
    receipt_date  TEXT    NOT NULL,
    method        TEXT    NOT NULL CHECK (method IN ('cash','offset')),
    txn_id        INTEGER,
    note          TEXT,
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_receipt_org_recv ON receipt(org_id, receivable_id);
