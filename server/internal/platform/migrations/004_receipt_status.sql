-- 004_receipt_status.sql — v0.3.4 应收/往来
-- 背景：003_multi_org.sql 的 receipt 建表漏了 status 列（设计文档 §9.1 有，D11 需要按作废/正常区分），
-- 此处补充（SQLite ADD COLUMN + NOT NULL 必须带 DEFAULT，历史行为全部视为 normal）。
ALTER TABLE receipt ADD COLUMN status TEXT NOT NULL DEFAULT 'normal'
    CHECK (status IN ('normal','voided'));
