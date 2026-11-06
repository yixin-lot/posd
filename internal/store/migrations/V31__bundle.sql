-- V31：商品套餐（组合商品）
-- 套餐本身是一条商品（有自己的价格），组件用于后厨分单与将来的库存扣减。
ALTER TABLE t_pos_product ADD COLUMN kind TEXT NOT NULL DEFAULT 'single'; -- single / bundle

CREATE TABLE IF NOT EXISTS t_pos_product_bundle (
  product_id   INTEGER NOT NULL,   -- 套餐本身
  component_id INTEGER NOT NULL,   -- 组成商品（必须是 single）
  qty          INTEGER NOT NULL,
  PRIMARY KEY (product_id, component_id)
);

-- 订单明细记录套餐组件快照（后厨按组件分单；商品改名/改价不影响历史单）
ALTER TABLE t_pos_order_item ADD COLUMN components TEXT NOT NULL DEFAULT '';
