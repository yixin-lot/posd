-- V33：套餐可选项（同一个 group_id 的多个组件互为替代，下单时选一个）
ALTER TABLE t_pos_product_bundle ADD COLUMN group_id INTEGER NOT NULL DEFAULT 0;
