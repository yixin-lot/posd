package catalog

import (
	"database/sql"
	"fmt"
)

// BOMItem 是配方中的一行：组件 + 用量（×1000）+ 投入损耗（万分比）。
type BOMItem struct {
	ComponentID int64
	QtyMilli    int64
	LossBp      int64
	Sort        int
}

// BOM 读取配方。
func (s *Service) BOM(parentID int64) ([]BOMItem, error) {
	rows, err := s.db.SQL().Query(
		"SELECT component_product_id,qty_milli,loss_bp,sort FROM t_pos_bom_item WHERE parent_product_id=? ORDER BY sort,id", parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BOMItem
	for rows.Next() {
		var b BOMItem
		if err := rows.Scan(&b.ComponentID, &b.QtyMilli, &b.LossBp, &b.Sort); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// SetBOM 覆盖式保存配方；保存前做环检测（允许两级以上，最多 8 层）。
func (s *Service) SetBOM(parentID int64, items []BOMItem) error {
	if _, err := s.Get(parentID); err != nil {
		return err
	}
	for _, it := range items {
		if it.QtyMilli <= 0 {
			return fmt.Errorf("%w: 用量必须大于 0", ErrBadRequest)
		}
		if it.LossBp < 0 || it.LossBp > 5000 {
			return fmt.Errorf("%w: 损耗系数应在 0~50%% 之间", ErrBadRequest)
		}
		if it.ComponentID == parentID {
			return fmt.Errorf("%w: 配方不能引用自己", ErrBadRequest)
		}
		if err := s.checkCycle(parentID, it.ComponentID, 1); err != nil {
			return err
		}
	}
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("DELETE FROM t_pos_bom_item WHERE parent_product_id=?", parentID); err != nil {
		return err
	}
	for i, it := range items {
		if _, err := tx.Exec(
			"INSERT INTO t_pos_bom_item(parent_product_id,component_product_id,qty_milli,loss_bp,sort) VALUES(?,?,?,?,?)",
			parentID, it.ComponentID, it.QtyMilli, it.LossBp, i); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) checkCycle(root, node int64, depth int) error {
	if depth > 8 {
		return fmt.Errorf("%w: 配方层级过深（最多 8 层）", ErrBadRequest)
	}
	var exists int
	err := s.db.SQL().QueryRow("SELECT 1 FROM t_pos_product WHERE id=?", node).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("%w: 组件商品不存在", ErrBadRequest)
	}
	if err != nil {
		return err
	}
	rows, err := s.db.SQL().Query("SELECT component_product_id FROM t_pos_bom_item WHERE parent_product_id=?", node)
	if err != nil {
		return err
	}
	var children []int64
	for rows.Next() {
		var child int64
		if err := rows.Scan(&child); err != nil {
			rows.Close()
			return err
		}
		children = append(children, child)
	}
	rows.Close()
	for _, child := range children {
		if child == root {
			return fmt.Errorf("%w: 配方存在循环引用", ErrBadRequest)
		}
		if err := s.checkCycle(root, child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// RollupCost 递归卷算成本（分）。有配方按配方算，没有则用商品自身进价。
func (s *Service) RollupCost(id int64, depth int) (int64, error) {
	if depth > 8 {
		return 0, fmt.Errorf("%w: 配方层级过深", ErrBadRequest)
	}
	p, err := s.Get(id)
	if err != nil {
		return 0, err
	}
	type line struct{ cid, qty, loss int64 }
	var lines []line
	rows, err := s.db.SQL().Query(
		"SELECT component_product_id,qty_milli,loss_bp FROM t_pos_bom_item WHERE parent_product_id=?", id)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.cid, &l.qty, &l.loss); err != nil {
			rows.Close()
			return 0, err
		}
		lines = append(lines, l)
	}
	rows.Close()
	if len(lines) == 0 {
		return p.Cost, nil
	}
	var total int64
	for _, l := range lines {
		cc, err := s.RollupCost(l.cid, depth+1)
		if err != nil {
			return 0, err
		}
		part := (cc*l.qty*(10000+l.loss) + 1000*10000 - 1) / (1000 * 10000)
		total += part
	}
	return total, nil
}
