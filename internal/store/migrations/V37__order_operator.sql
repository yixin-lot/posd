-- V37：订单记录收银员（操作者）。
-- 订单查询页「收银员」列此前恒为空：Operator 只写进了审计表，没落在订单行上。
-- 注：本迁移在 feat/d14 上编号为 V31，与 main 的 V31__bundle 撞号；
-- 合并后空库初始化会因 schema_version 主键冲突失败，故并入时改号 V37。
-- V31__bundle 已随 v0.1.1 发布（存量库已有 V31 记录），不能反向挪号。
ALTER TABLE t_pos_order ADD COLUMN operator TEXT NOT NULL DEFAULT '';
