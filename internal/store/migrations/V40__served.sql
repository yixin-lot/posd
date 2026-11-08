-- V40：出餐扣料（on_serve）所需的行级出餐数量
ALTER TABLE t_pos_order_item ADD COLUMN served_qty INTEGER NOT NULL DEFAULT 0;
