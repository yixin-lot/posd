-- V10：商品来源（seed 示例 / template 导入 / manual 自建）
ALTER TABLE t_pos_product ADD COLUMN source TEXT NOT NULL DEFAULT 'manual';

-- V4 的 12 个种子商品没有 barcode（NULL），只能按「名称 + 无条码」认定，
-- 避免把店主后来手建的同名商品误标成示例而清掉。
UPDATE t_pos_product SET source='seed'
WHERE COALESCE(barcode,'')=''
  AND name IN ('珍珠奶茶','椰果奶茶','芋泥波波','厚乳拿铁','杨枝甘露','葡萄冻冻',
               '柠檬绿茶','生椰拿铁','美式咖啡','焦糖玛奇朵','海盐芝士','蛋挞');
