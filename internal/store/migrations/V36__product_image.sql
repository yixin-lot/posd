-- V36：商品图片（存相对路径，文件放数据目录 images/ 下）
ALTER TABLE t_pos_product ADD COLUMN image TEXT NOT NULL DEFAULT '';
