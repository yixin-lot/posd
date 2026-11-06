-- V29：会员流水关联班次（积分/余额统计不再依赖时间窗近似）
ALTER TABLE t_pos_member_txn ADD COLUMN shift_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_member_txn_shift ON t_pos_member_txn(shift_id);
