-- V8：部分退款（按明细行退数量）
ALTER TABLE t_pos_order_item ADD COLUMN refunded_qty INTEGER NOT NULL DEFAULT 0;
