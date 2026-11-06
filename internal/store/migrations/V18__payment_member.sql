-- V18：支付记录关联会员（余额支付时记 member_id，便于对账）
ALTER TABLE t_pos_payment ADD COLUMN member_id INTEGER NOT NULL DEFAULT 0;
