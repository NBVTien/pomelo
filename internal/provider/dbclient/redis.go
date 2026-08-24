package dbclient

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/redis/go-redis/v9"
)

func init() { Register("redis", openRedis) }

type redisDriver struct{ c *redis.Client }

func openRedis(ctx context.Context, dsn string) (Driver, error) {
	opt, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, err
	}
	c := redis.NewClient(opt)
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, err
	}
	return &redisDriver{c: c}, nil
}

func (d *redisDriver) Ping(ctx context.Context) error { return d.c.Ping(ctx).Err() }
func (d *redisDriver) Close() error                   { return d.c.Close() }

func (d *redisDriver) Schemas(ctx context.Context) ([]string, error) { return nil, nil }
func (d *redisDriver) Columns(ctx context.Context) ([]Column, error) { return nil, nil }
func (d *redisDriver) ExportCSV(ctx context.Context, sql, path string) (int64, error) {
	return 0, fmt.Errorf("CSV export not supported for redis")
}

func (d *redisDriver) Tables(ctx context.Context, _ string) ([]Table, error) {
	counts := map[string]int{}
	var cursor uint64
	scanned := 0
	for {
		keys, next, err := d.c.Scan(ctx, cursor, "*", 500).Result()
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			ns := k
			if i := strings.IndexByte(k, ':'); i > 0 {
				ns = k[:i]
			}
			counts[ns]++
		}
		scanned += len(keys)
		cursor = next
		if cursor == 0 || scanned >= 20000 {
			break
		}
	}
	out := make([]Table, 0, len(counts))
	for ns, n := range counts {
		out = append(out, Table{Name: fmt.Sprintf("%s (%d)", ns, n), Type: "keyspace"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (d *redisDriver) Query(ctx context.Context, q string, limit int) (*Result, error) {
	if limit <= 0 {
		limit = 500
	}
	q = strings.TrimSpace(q)
	if i := strings.Index(q, " ("); i > 0 && strings.HasSuffix(q, ")") {
		q = q[:i] + ":*"
	}
	if q == "" {
		return &Result{}, nil
	}
	fields := strings.Fields(q)
	if len(fields) == 1 && looksLikePattern(fields[0]) {
		return d.scan(ctx, fields[0], limit)
	}
	args := make([]any, len(fields))
	for i, f := range fields {
		args[i] = f
	}
	res, err := d.c.Do(ctx, args...).Result()
	if err != nil {
		return nil, err
	}
	return &Result{Columns: []string{"result"}, Rows: [][]string{{stringifyRedis(res)}}}, nil
}

func looksLikePattern(s string) bool {
	return strings.ContainsAny(s, "*?[") || (!strings.ContainsAny(s, " ") && strings.Contains(s, ":"))
}

func (d *redisDriver) scan(ctx context.Context, pattern string, limit int) (*Result, error) {
	res := &Result{Columns: []string{"key", "type", "ttl", "value"}}
	var cursor uint64
	for {
		keys, next, err := d.c.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			if len(res.Rows) >= limit {
				res.Truncated = true
				return res, nil
			}
			typ, _ := d.c.Type(ctx, k).Result()
			ttl, _ := d.c.TTL(ctx, k).Result()
			ttlStr := "-"
			if ttl > 0 {
				ttlStr = ttl.String()
			}
			res.Rows = append(res.Rows, []string{k, typ, ttlStr, d.preview(ctx, k, typ)})
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return res, nil
}

func (d *redisDriver) preview(ctx context.Context, key, typ string) string {
	switch typ {
	case "string":
		v, _ := d.c.Get(ctx, key).Result()
		return truncate(v, 400)
	case "list":
		v, _ := d.c.LRange(ctx, key, 0, 20).Result()
		return truncate(strings.Join(v, ", "), 400)
	case "set":
		v, _ := d.c.SMembers(ctx, key).Result()
		return truncate(strings.Join(v, ", "), 400)
	case "zset":
		v, _ := d.c.ZRange(ctx, key, 0, 20).Result()
		return truncate(strings.Join(v, ", "), 400)
	case "hash":
		v, _ := d.c.HGetAll(ctx, key).Result()
		parts := make([]string, 0, len(v))
		for f, val := range v {
			parts = append(parts, f+"="+val)
		}
		sort.Strings(parts)
		return truncate(strings.Join(parts, ", "), 400)
	default:
		return ""
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func stringifyRedis(v any) string {
	switch x := v.(type) {
	case nil:
		return "(nil)"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = stringifyRedis(e)
		}
		return strings.Join(parts, ", ")
	case map[any]any:
		parts := make([]string, 0, len(x))
		for k, val := range x {
			parts = append(parts, fmt.Sprintf("%v=%v", k, val))
		}
		sort.Strings(parts)
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", x)
	}
}
