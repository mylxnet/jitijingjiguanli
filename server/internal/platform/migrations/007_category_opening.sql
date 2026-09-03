-- 007_category_opening.sql — v0.5.1：科目期初录入（用户要求）
-- v0.4(006) 曾移除科目期初；为支持“建账时某二级已有存量”加回。
-- 权益科目期初=开始累计存量；资产科目期初=建账时已在外的投资金额。
-- SQLite ALTER ADD COLUMN NOT NULL 需带 DEFAULT，历史科目期初视为 0。
ALTER TABLE category ADD COLUMN opening_balance_cents INTEGER NOT NULL DEFAULT 0;
