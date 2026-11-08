-- V39：BOM 配方（两级：原料/半成品），支持损耗系数与成本卷算
CREATE TABLE IF NOT EXISTS t_pos_bom_item (
  id                  INTEGER PRIMARY KEY AUTOINCREMENT,
  parent_product_id   INTEGER NOT NULL,
  component_product_id INTEGER NOT NULL,
  qty_milli           INTEGER NOT NULL,              -- 用量 ×1000（0.5g = 500）
  loss_bp             INTEGER NOT NULL DEFAULT 0,    -- 投入损耗万分比（500 = 5%）
  sort                INTEGER NOT NULL DEFAULT 0,
  UNIQUE(parent_product_id, component_product_id)
);
CREATE INDEX IF NOT EXISTS idx_bom_parent ON t_pos_bom_item(parent_product_id);
