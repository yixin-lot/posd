-- V4：商品（收银台从服务端取，不再用前端内置样例）
CREATE TABLE IF NOT EXISTS t_pos_product (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  shop_id    INTEGER NOT NULL,
  name       TEXT    NOT NULL,
  barcode    TEXT,
  category   TEXT    NOT NULL DEFAULT '',
  price      INTEGER NOT NULL,
  cost       INTEGER NOT NULL DEFAULT 0,
  hot        INTEGER NOT NULL DEFAULT 0,
  active     INTEGER NOT NULL DEFAULT 1,
  sort       INTEGER NOT NULL DEFAULT 0,
  created_at TEXT    NOT NULL,
  updated_at TEXT    NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_product_barcode
  ON t_pos_product(shop_id, barcode) WHERE barcode IS NOT NULL AND barcode <> '';
CREATE INDEX IF NOT EXISTS idx_product_active ON t_pos_product(shop_id, active, sort);

INSERT INTO t_pos_product(shop_id, name, category, price, hot, sort, created_at, updated_at) VALUES
 (1, '珍珠奶茶', '奶茶', 1300, 1, 10, datetime('now','localtime'), datetime('now','localtime')),
 (1, '椰果奶茶', '奶茶', 1200, 1, 20, datetime('now','localtime'), datetime('now','localtime')),
 (1, '芋泥波波', '奶茶', 1600, 0, 30, datetime('now','localtime'), datetime('now','localtime')),
 (1, '厚乳拿铁', '奶茶', 1500, 0, 40, datetime('now','localtime'), datetime('now','localtime')),
 (1, '杨枝甘露', '果茶', 1800, 1, 50, datetime('now','localtime'), datetime('now','localtime')),
 (1, '葡萄冻冻', '果茶', 1600, 0, 60, datetime('now','localtime'), datetime('now','localtime')),
 (1, '柠檬绿茶', '果茶', 1000, 0, 70, datetime('now','localtime'), datetime('now','localtime')),
 (1, '生椰拿铁', '咖啡', 1700, 1, 80, datetime('now','localtime'), datetime('now','localtime')),
 (1, '美式咖啡', '咖啡', 1200, 0, 90, datetime('now','localtime'), datetime('now','localtime')),
 (1, '焦糖玛奇朵', '咖啡', 1900, 0, 100, datetime('now','localtime'), datetime('now','localtime')),
 (1, '海盐芝士', '甜品', 1800, 1, 110, datetime('now','localtime'), datetime('now','localtime')),
 (1, '蛋挞', '甜品', 600, 0, 120, datetime('now','localtime'), datetime('now','localtime'));
