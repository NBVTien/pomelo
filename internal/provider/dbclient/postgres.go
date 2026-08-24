package dbclient

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

func init() { Register("postgres", openPostgres) }

type pgDriver struct{ conn *pgx.Conn }

func openPostgres(ctx context.Context, dsn string) (Driver, error) {
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &pgDriver{conn: c}, nil
}

func (d *pgDriver) Ping(ctx context.Context) error { return d.conn.Ping(ctx) }
func (d *pgDriver) Close() error                   { return d.conn.Close(context.Background()) }

func (d *pgDriver) Schemas(ctx context.Context) ([]string, error) {
	rows, err := d.conn.Query(ctx, `SELECT schema_name FROM information_schema.schemata
		WHERE schema_name NOT LIKE 'pg\_%' AND schema_name <> 'information_schema' ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *pgDriver) Tables(ctx context.Context, schema string) ([]Table, error) {
	q := `SELECT table_schema, table_name, table_type FROM information_schema.tables
		WHERE table_schema NOT LIKE 'pg\_%' AND table_schema <> 'information_schema'
		AND table_name NOT LIKE 'pg\_%'`
	args := []any{}
	if schema != "" {
		q += ` AND table_schema = $1`
		args = append(args, schema)
	}
	q += ` ORDER BY table_schema, table_name`
	rows, err := d.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Table
	for rows.Next() {
		var t Table
		var typ string
		if err := rows.Scan(&t.Schema, &t.Name, &typ); err != nil {
			return nil, err
		}
		t.Type = "table"
		if strings.Contains(strings.ToUpper(typ), "VIEW") {
			t.Type = "view"
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *pgDriver) Columns(ctx context.Context) ([]Column, error) {
	rows, err := d.conn.Query(ctx, `SELECT table_schema, table_name, column_name, data_type
		FROM information_schema.columns
		WHERE table_schema NOT LIKE 'pg\_%' AND table_schema <> 'information_schema'
		AND table_name NOT LIKE 'pg\_%'
		ORDER BY table_schema, table_name, ordinal_position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Column
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.Schema, &c.Table, &c.Name, &c.Type); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *pgDriver) Query(ctx context.Context, sql string, limit int) (*Result, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := d.conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	res := &Result{Columns: make([]string, len(fds))}
	for i, f := range fds {
		res.Columns[i] = string(f.Name)
	}
	for rows.Next() {
		if len(res.Rows) >= limit {
			res.Truncated = true
			break
		}
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make([]string, len(vals))
		for i, v := range vals {
			row[i] = stringify(v)
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(res.Columns) == 0 {
		res.RowsAffected = rows.CommandTag().RowsAffected()
	}
	return res, nil
}

func (d *pgDriver) ExportCSV(ctx context.Context, sql, path string) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	rows, err := d.conn.Query(ctx, sql)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	header := make([]string, len(fds))
	for i, fd := range fds {
		header[i] = string(fd.Name)
	}
	if err := w.Write(header); err != nil {
		return 0, err
	}
	var n int64
	rec := make([]string, len(fds))
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return n, err
		}
		for i, v := range vals {
			rec[i] = stringify(v)
		}
		if err := w.Write(rec); err != nil {
			return n, err
		}
		n++
		if n%2000 == 0 {
			w.Flush()
			if err := w.Error(); err != nil {
				return n, err
			}
		}
	}
	return n, rows.Err()
}

func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(x)
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}
