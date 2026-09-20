-- 020_reinvest_income_cat.sql — v0.24：新增预置 L1「再投资收益」，并把再投资收益的现金入账
-- 从「再投资」本金容器中迁出（方案见 docs/25）。
-- 背景：recvKindToL1 曾把 reinvest_dividend 的 cash 核销流水记到「再投资/<单位>」，
--       导致再投资收益虚增该单位的再投资本金（实测 org6 羊鸡福：期初 16,834.45 + 收益 505.03
--       被当成本金 17,339.48 展示），且这笔收益永远进不了 532 可分配池。
-- 说明：① 一级容器 sort_order≥6 整体后移一位，逐组织补建「再投资收益」(preset=1, sort=6)。
--         L1 唯一索引含 NULL 列，SQLite 视 NULL 互不相等 → 不强制唯一，必须显式 NOT EXISTS 判重。
--       ② 依 receipt.txn_id ⨝ receivable.recv_kind='reinvest_dividend' 精确定位被错记的收益流水：
--         先在「再投资收益」下补建同名二级，再把流水 category_id 挪过去。
--         仅认 method='cash'（offset 的 txn_id 指向的是发放支出流水，方向亦为 expense）；
--         作废流水一并挪走，避免本金科目下钻明细里残留收益痕迹。
PRAGMA defer_foreign_keys=ON;

UPDATE category
   SET sort_order = sort_order + 1, updated_at = datetime('now')
 WHERE level = 1 AND sort_order >= 6
   AND NOT EXISTS (
       SELECT 1 FROM category n
        WHERE n.org_id = category.org_id AND n.level = 1 AND n.name = '再投资收益'
   );

INSERT INTO category(org_id, name, level, parent_id, status, kind, preset, sort_order, created_at, updated_at)
SELECT o.id, '再投资收益', 1, NULL, 'active', 'equity', 1, 6, datetime('now'), datetime('now')
  FROM org o
 WHERE NOT EXISTS (
     SELECT 1 FROM category c
      WHERE c.org_id = o.id AND c.level = 1 AND c.name = '再投资收益'
 );

-- 目标二级容器：再投资收益/<原「再投资」下的同名单位二级>
INSERT INTO category(org_id, name, level, parent_id, status, kind, preset, sort_order, created_at, updated_at)
SELECT DISTINCT t.org_id, src.name, 2, nl.id, 'active', 'equity', 0, 0, datetime('now'), datetime('now')
  FROM txn t
  JOIN receipt    rc  ON rc.txn_id = t.id AND rc.org_id = t.org_id AND rc.method = 'cash'
  JOIN receivable r   ON r.id = rc.receivable_id AND r.org_id = t.org_id
                     AND r.recv_kind = 'reinvest_dividend'
  JOIN category   src ON src.id = t.category_id
  JOIN category   rl  ON rl.id = src.parent_id AND rl.level = 1 AND rl.name = '再投资'
  JOIN category   nl  ON nl.org_id = t.org_id AND nl.level = 1 AND nl.name = '再投资收益'
 WHERE t.direction = 'income'
   AND NOT EXISTS (
       SELECT 1 FROM category x
        WHERE x.org_id = t.org_id AND x.parent_id = nl.id AND x.name = src.name
   );

UPDATE txn
   SET category_id = COALESCE((
         SELECT dst.id
           FROM receipt rc
           JOIN receivable r   ON r.id = rc.receivable_id AND r.recv_kind = 'reinvest_dividend'
           JOIN category   src ON src.id = txn.category_id
           JOIN category   nl  ON nl.org_id = txn.org_id AND nl.level = 1 AND nl.name = '再投资收益'
           JOIN category   dst ON dst.org_id = txn.org_id AND dst.parent_id = nl.id AND dst.name = src.name
          WHERE rc.txn_id = txn.id AND rc.org_id = txn.org_id AND rc.method = 'cash'
          LIMIT 1
       ), txn.category_id),
       updated_at = datetime('now')
 WHERE direction = 'income'
   AND EXISTS (
         SELECT 1
           FROM receipt rc
           JOIN receivable r   ON r.id = rc.receivable_id AND r.recv_kind = 'reinvest_dividend'
           JOIN category   src ON src.id = txn.category_id
           JOIN category   rl  ON rl.id = src.parent_id AND rl.level = 1 AND rl.name = '再投资'
          WHERE rc.txn_id = txn.id AND rc.org_id = txn.org_id AND rc.method = 'cash'
   );
