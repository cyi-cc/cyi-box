package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/cyi-cc/fun"
	"github.com/xuri/excelize/v2"

	"github.com/cyi-cc/cyi-box/backend/internal/config"
	"github.com/cyi-cc/cyi-box/backend/internal/dbx"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

const exportRowCap = 50000

// DbExportHandler POST /dbm/export 表单字段：id, db, sql, format(json|csv|xlsx)
func DbExportHandler(st *store.Store, cfg *config.Config) fun.RouteHandler {
	return func(rc *fun.RouteCtx) error {
		ctx := rc.RequestCtx
		if err := routeAdmin(st, ctx); err != nil {
			ctx.SetBodyString(`{"status":2,"msg":"未授权"}`)
			return nil
		}
		id, _ := strconv.ParseInt(string(ctx.QueryArgs().Peek("id")), 10, 64)
		if id == 0 {
			id, _ = strconv.ParseInt(rc.Data["id"], 10, 64)
		}
		dbName := rc.Data["db"]
		sqlText := strings.TrimSpace(rc.Data["sql"])
		format := rc.Data["format"]
		if format == "" {
			format = "json"
		}
		if id == 0 || sqlText == "" {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"参数不完整"}`)
			return nil
		}
		c, _, err := openConn(st, cfg.VaultKey, id, dbName)
		if err != nil {
			ctx.SetStatusCode(502)
			ctx.SetBodyString("连接失败：" + err.Error())
			return nil
		}
		defer c.Close()

		qctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		res, err := dbx.Query(qctx, c, sqlText, exportRowCap)
		if err != nil {
			ctx.SetStatusCode(500)
			ctx.SetBodyString("执行失败：" + err.Error())
			return nil
		}
		if !res.IsSelect {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"非查询语句没有可导出的结果集"}`)
			return nil
		}

		var body []byte
		var ext, mime string
		switch format {
		case "xlsx":
			body, err = toXLSX(res)
			ext, mime = "xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		case "csv":
			body, err = toCSV(res)
			ext, mime = "csv", "text/csv; charset=utf-8"
		default:
			ext, mime = "json", "application/json"
			body, err = toJSONRows(res)
		}
		if err != nil {
			ctx.SetStatusCode(500)
			ctx.SetBodyString("导出失败：" + err.Error())
			return nil
		}
		fname := fmt.Sprintf("export_%d.%s", time.Now().Unix(), ext)
		ctx.Response.Header.Set("Content-Disposition", "attachment; filename=\""+fname+"\"")
		ctx.SetContentType(mime)
		ctx.SetBody(body)
		return nil
	}
}

func toJSONRows(res *dbx.Result) ([]byte, error) {
	list := make([]map[string]any, 0, len(res.Rows))
	for _, row := range res.Rows {
		obj := map[string]any{}
		for i, col := range res.Columns {
			obj[col] = row[i]
		}
		list = append(list, obj)
	}
	return json.MarshalIndent(list, "", "  ")
}

func toCSV(res *dbx.Result) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM，Excel 打开不乱码
	w := csv.NewWriter(&buf)
	if err := w.Write(res.Columns); err != nil {
		return nil, err
	}
	for _, row := range res.Rows {
		rec := make([]string, len(row))
		for i, v := range row {
			rec[i] = cellText(v)
		}
		if err := w.Write(rec); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func toXLSX(res *dbx.Result) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"
	for i, c := range res.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, c); err != nil {
			return nil, err
		}
	}
	for ri, row := range res.Rows {
		for ci, v := range row {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
			if v == nil {
				continue
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return nil, err
			}
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func cellText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprint(t)
	}
}

// DbImportHandler POST /dbm/import multipart：file + 字段 id/db/table；
// 按扩展名解析 json(对象数组) / csv / xlsx（首行为表头），列名与表列交集后批量插入
func DbImportHandler(st *store.Store, cfg *config.Config) fun.RouteHandler {
	return func(rc *fun.RouteCtx) error {
		ctx := rc.RequestCtx
		if err := routeAdmin(st, ctx); err != nil {
			ctx.SetBodyString(`{"status":2,"msg":"未授权"}`)
			return nil
		}
		id, _ := strconv.ParseInt(string(ctx.QueryArgs().Peek("id")), 10, 64)
		dbName := string(ctx.QueryArgs().Peek("db"))
		table := strings.TrimSpace(string(ctx.QueryArgs().Peek("table")))
		if id == 0 || table == "" {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"参数不完整"}`)
			return nil
		}
		fh, err := ctx.FormFile("file")
		if err != nil {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"缺少文件"}`)
			return nil
		}
		src, err := fh.Open()
		if err != nil {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"读取上传失败"}`)
			return nil
		}
		defer src.Close()
		raw, err := io.ReadAll(io.LimitReader(src, 64<<20))
		if err != nil {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"读取文件失败"}`)
			return nil
		}
		rows, err := parseImport(strings.ToLower(fh.Filename), raw)
		if err != nil {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"` + err.Error() + `"}`)
			return nil
		}
		if len(rows) == 0 {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"文件里没有数据行"}`)
			return nil
		}

		c, e, err := openConn(st, cfg.VaultKey, id, dbName)
		if err != nil {
			ctx.SetStatusCode(502)
			ctx.SetBodyString(`{"status":2,"msg":"连接失败"}`)
			return nil
		}
		defer c.Close()

		// 表名校验：必须在 introspection 列表里，顺便拿到列清单做交集
		tables, cols, err := dbx.TablesAndColumns(c, e.Engine, dbName)
		if err != nil {
			ctx.SetStatusCode(500)
			ctx.SetBodyString(`{"status":2,"msg":"读表结构失败"}`)
			return nil
		}
		validTable := false
		tableCols := map[string]bool{}
		for _, t := range tables {
			if t.Name == table {
				validTable = true
			}
		}
		for _, cl := range cols {
			if cl.Table == table {
				tableCols[cl.Name] = true
			}
		}
		if !validTable {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"表不存在"}`)
			return nil
		}

		// 列名 = 文件表头 ∩ 表列
		fileCols := orderedKeys(rows)
		useCols := []string{}
		for _, k := range fileCols {
			if tableCols[k] {
				useCols = append(useCols, k)
			}
		}
		if len(useCols) == 0 {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"文件列名与表列没有交集"}`)
			return nil
		}

		inserted, skipped, err := bulkInsert(c, e.Engine, table, useCols, rows)
		if err != nil {
			ctx.SetStatusCode(500)
			ctx.SetBodyString(`{"status":2,"msg":"导入失败：` + err.Error() + `"}`)
			return nil
		}
		ctx.SetContentType("application/json")
		fmt.Fprintf(ctx, `{"status":0,"inserted":%d,"skipped":%d}`, inserted, skipped)
		return nil
	}
}

// bulkInsert 逐行插入（不开事务：PG 事务内单行失败会整批回滚，
// 逐行保证「能导多少导多少」，跳过坏行计数返回）。单次最多 1 万行。
func bulkInsert(d *sql.DB, engine, table string, cols []string, rows []map[string]any) (int64, int64, error) {
	if len(rows) > 10000 {
		rows = rows[:10000]
	}
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	if engine == "mysql" {
		quote = func(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }
	}
	quoted := make([]string, len(cols))
	placeholders := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = quote(c)
		if engine == "postgres" {
			placeholders[i] = "$" + strconv.Itoa(i+1)
		} else {
			placeholders[i] = "?"
		}
	}
	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quote(table), strings.Join(quoted, ","), strings.Join(placeholders, ","))
	ps, err := d.Prepare(stmt)
	if err != nil {
		return 0, 0, err
	}
	defer ps.Close()
	var inserted, skipped int64
	for _, r := range rows {
		vals := make([]any, len(cols))
		for i, c := range cols {
			vals[i] = r[c]
		}
		if _, err := ps.Exec(vals...); err != nil {
			skipped++
			continue
		}
		inserted++
	}
	return inserted, skipped, nil
}

// parseImport 按扩展名分派：.json 对象数组 / .xlsx 首行表头 / 其余按 CSV（兼容 BOM）
func parseImport(filename string, raw []byte) ([]map[string]any, error) {
	switch {
	case strings.HasSuffix(filename, ".json"):
		var rows []map[string]any
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, fmt.Errorf("JSON 解析失败：文件应为对象数组 [{...}, ...]")
		}
		return rows, nil
	case strings.HasSuffix(filename, ".xlsx"):
		f, err := excelize.OpenReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("Excel 解析失败")
		}
		defer f.Close()
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, fmt.Errorf("Excel 文件没有工作表")
		}
		recs, err := f.GetRows(sheets[0])
		if err != nil {
			return nil, fmt.Errorf("读取工作表失败")
		}
		return rowsFromTable(recs), nil
	default:
		r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})))
		r.FieldsPerRecord = -1 // 容忍长短不一的行
		recs, err := r.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("CSV 解析失败")
		}
		return rowsFromTable(recs), nil
	}
}

// rowsFromTable 二维表（首行表头）→ 对象数组
func rowsFromTable(recs [][]string) []map[string]any {
	if len(recs) < 2 {
		return nil
	}
	head := recs[0]
	out := []map[string]any{}
	for _, rec := range recs[1:] {
		m := map[string]any{}
		for i, h := range head {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			if i < len(rec) {
				m[h] = rec[i]
			}
		}
		out = append(out, m)
	}
	return out
}

// orderedKeys 取文件列名（按首行出现顺序）
func orderedKeys(rows []map[string]any) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, r := range rows {
		for k := range r {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	return out
}
