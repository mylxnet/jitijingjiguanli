-- 005_party_unit_only.sql — 需求调整：往来对象只有「往来单位」，去掉农户类型
-- party.kind（household/unit）整列移除；历史行为按单位口径（本版仅冒烟数据）。
-- SQLite 3.35+ 支持 DROP COLUMN；kind 无索引、无表级 CHECK 引用，可安全删除。
ALTER TABLE party DROP COLUMN kind;
