package catalog

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

//
// CSV 列（表头）：名称,条码,类目,售价,成本,热销
//  - 名称必填；售价 > 0；成本 >= 0；条码可选，但文件内重复或与库内已存在都报错；
//  - 热销取值：是/否（也兼容 1/0/true/false）；
//  - 金额单位：元（导出时以元输出，导入时转分入库）。
// 失败行带行号（表头为第 1 行，数据从第 2 行起）。

// CSVRow 是导入 CSV 的一行数据（金额单位：元）。Line 是数据行号（从 2 起）。
type CSVRow struct {
	Line     int
	Name     string
	Barcode  string
	Category string
	Price    float64
	Cost     float64
	Hot      bool
}

// RowError 是某一行校验失败的原因（Line 从 2 起，即数据行号）。
type RowError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ParseCSV 解析并做行内校验与文件内条码去重：
// 返回可导入的行（成功）+ 校验失败的行（含行号）。
// data 整体无法解析（列数不一致等结构性错误）时返回 err。
func ParseCSV(data []byte) ([]CSVRow, []RowError, error) {
	r := csv.NewReader(bytes.NewReader(data))
	records, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("CSV 无法解析: %w", err)
	}
	if len(records) < 2 {
		return nil, nil, errors.New("CSV 至少需要表头与一行数据")
	}

	var rows []CSVRow
	var errs []RowError
	seenBarcode := map[string]int{} // barcode -> 首个出现的数据行号

	for i := 1; i < len(records); i++ {
		rec := records[i]
		line := i + 1
		if len(rec) < 6 {
			errs = append(errs, RowError{Line: line, Message: "列数不足：需要 名称,条码,类目,售价,成本,热销 六列"})
			continue
		}
		row, msg := validateRow(rec, seenBarcode, line)
		if msg != "" {
			errs = append(errs, RowError{Line: line, Message: msg})
			continue
		}
		rows = append(rows, row)
	}
	return rows, errs, nil
}

func validateRow(rec []string, seenBarcode map[string]int, line int) (CSVRow, string) {
	name := strings.TrimSpace(rec[0])
	if name == "" {
		return CSVRow{}, "商品名不能为空"
	}
	barcode := strings.TrimSpace(rec[1])
	if barcode != "" {
		if firstLine, dup := seenBarcode[barcode]; dup {
			return CSVRow{}, fmt.Sprintf("条码 %s 与第 %d 行重复", barcode, firstLine)
		}
	}
	price, err := parseYuan(rec[3])
	if err != nil || price <= 0 {
		return CSVRow{}, "售价必须为大于 0 的数字"
	}
	cost, err := parseYuan(rec[4])
	if err != nil || cost < 0 {
		return CSVRow{}, "成本必须为不小于 0 的数字"
	}
	hot, err := parseBool(rec[5])
	if err != nil {
		return CSVRow{}, "热销只能填 是/否"
	}
	if barcode != "" {
		seenBarcode[barcode] = line
	}
	return CSVRow{
		Line:     line,
		Name:     name,
		Barcode:  barcode,
		Category: strings.TrimSpace(rec[2]),
		Price:    price,
		Cost:     cost,
		Hot:      hot,
	}, ""
}

func parseYuan(s string) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "是", "1", "true", "yes":
		return true, nil
	case "否", "0", "false", "no", "":
		return false, nil
	}
	return false, errors.New("invalid bool")
}

// ImportCSV 把解析后的行写入商品库（source='manual'，与店主自建一致）。
// 库内已存在相同条码的行跳过并记入失败（含行号——行号由调用方按顺序回填）。
func (s *Service) ImportCSV(rows []CSVRow) (int, []RowError, error) {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var errs []RowError
	imported := 0
	for _, row := range rows {
		p, err := normalize(SaveInput{
			Name: row.Name, Barcode: row.Barcode, Category: row.Category,
			Price: row.Price, Cost: row.Cost, Hot: row.Hot,
		})
		if err != nil {
			errs = append(errs, RowError{Line: row.Line, Message: strings.TrimPrefix(err.Error(), "参数不合法: ")})
			continue
		}
		res, err := tx.Exec(
			"INSERT OR IGNORE INTO t_pos_product(shop_id,name,barcode,category,price,cost,hot,active,sort,"+
				"source,created_at,updated_at)"+
				" VALUES(1,?,?,?,?,?,?,1,(SELECT COALESCE(MAX(sort),0)+1 FROM t_pos_product),"+
				"'manual',datetime('now','localtime'),datetime('now','localtime'))",
			p.Name, nullable(p.Barcode), p.Category, p.Price, p.Cost, boolInt(p.Hot))
		if err != nil {
			return imported, errs, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			imported++
		} else {
			errs = append(errs, RowError{Line: row.Line, Message: fmt.Sprintf("条码 %s 已存在", row.Barcode)})
		}
	}
	if err := tx.Commit(); err != nil {
		return imported, errs, err
	}
	return imported, errs, nil
}
