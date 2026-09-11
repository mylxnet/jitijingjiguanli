-- v0.19：科目间结转(transfer) 关联来源流水，支持作废/恢复流水时联动作废相应结转。
ALTER TABLE transfer ADD COLUMN txn_id INTEGER REFERENCES txn(id);
CREATE INDEX IF NOT EXISTS idx_transfer_txn ON transfer(txn_id);