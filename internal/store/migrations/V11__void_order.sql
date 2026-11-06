-- V11：整单作废（点错/重复下单，不算销售；与"退款"语义分开）
ALTER TABLE t_pos_order ADD COLUMN voided_at TEXT;
ALTER TABLE t_pos_order ADD COLUMN void_reason TEXT;
ALTER TABLE t_pos_order ADD COLUMN void_operator TEXT;
