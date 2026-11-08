-- V38：退单双模式、库存流水、损耗台账（PRD P0，第一步只落表结构）
-- 说明：本迁移只建表/加列，业务读写由后续提交接入，保证可回滚。

-- 退单类型：cancel_unmade=未制作取消（回补库存）；waste_made=已制作退品（记损耗）
ALTER TABLE t_pos_refund ADD COLUMN refund_mode TEXT NOT NULL DEFAULT 'waste_made';
ALTER TABLE t_pos_refund ADD COLUMN loss_cost INTEGER NOT NULL DEFAULT 0;

-- 售出时成本快照：后续原料涨价不影响历史毛利
ALTER TABLE t_pos_order_item ADD COLUMN cost_minor INTEGER NOT NULL DEFAULT 0;

-- 库存流水：所有库存变动的唯一真相（qty_delta 正入负出）
CREATE TABLE IF NOT EXISTS t_pos_stock_move (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  product_id  INTEGER NOT NULL,
  qty_delta   INTEGER NOT NULL,
  cost_minor  INTEGER NOT NULL DEFAULT 0,
  move_type   TEXT    NOT NULL,
  ref_type    TEXT    NOT NULL DEFAULT '',
  ref_id      TEXT    NOT NULL DEFAULT '',
  reason      TEXT    NOT NULL DEFAULT '',
  operator    TEXT    NOT NULL DEFAULT '',
  created_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_move_product_time ON t_pos_stock_move(product_id, created_at);
CREATE INDEX IF NOT EXISTS idx_move_ref ON t_pos_stock_move(ref_type, ref_id);

-- 损耗台账：售后作废 / 仓储报损 / 赠送试吃 / 员工餐
CREATE TABLE IF NOT EXISTS t_pos_loss (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  product_id  TEXT    NOT NULL,
  qty         INTEGER NOT NULL,
  cost_minor  INTEGER NOT NULL DEFAULT 0,
  loss_type   TEXT    NOT NULL,             -- refund_waste / storage / gift / employee
  ref_type    TEXT    NOT NULL DEFAULT '',
  ref_id      TEXT    NOT NULL DEFAULT '',
  reason      TEXT    NOT NULL DEFAULT '',
  operator    TEXT    NOT NULL DEFAULT '',
  created_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_loss_type_time ON t_pos_loss(loss_type, created_at);
