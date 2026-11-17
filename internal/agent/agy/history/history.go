// Package history reads Antigravity CLI SQLite/protobuf history without invoking
// the CLI. Format support is limited to descriptor-backed native fields.
package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/rainchan/webmux/internal/historysource"
	_ "modernc.org/sqlite"
)

const FormatVersion = "agy-sqlite-step-protobuf-v1"
const ParserVersion = "2"

func decimal(n int64) string { return strconv.FormatInt(n, 10) }

func safeError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == historysource.ErrLimit || err == historysource.ErrChanged || err == historysource.ErrFormat || err == historysource.ErrIdentity {
		return err
	}
	return historysource.ErrUnavailable
}

func open(ctx context.Context, path string) (*sql.DB, os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, nil, historysource.ErrUnavailable
	}
	if info.Size() > historysource.MaxBytes {
		return nil, nil, historysource.ErrLimit
	}
	for _, suffix := range []string{"-wal", "-journal"} {
		if side, err := os.Lstat(path + suffix); err == nil && (!side.Mode().IsRegular() || side.Size() > historysource.MaxBytes) {
			return nil, nil, historysource.ErrLimit
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, historysource.ErrUnavailable
	}
	u := url.URL{Scheme: "file", Path: absolute}
	db, err := sql.Open("sqlite", u.String()+"?mode=ro&_pragma=query_only(1)&_pragma=trusted_schema(0)&_pragma=busy_timeout(100)")
	if err != nil {
		return nil, nil, safeError(ctx, err)
	}
	db.SetMaxOpenConns(1)
	return db, info, nil
}

// A cascade is the runtime ACP conversation; its trajectory identifies the
// ordered steps stored in this database. Neither ID is inferred from the path.
type nativeIdentity struct {
	CascadeID    string
	TrajectoryID string
}

func identity(ctx context.Context, tx *sql.Tx) (nativeIdentity, error) {
	var id nativeIdentity
	rows, err := tx.QueryContext(ctx, `SELECT
		CASE WHEN typeof(cascade_id)='text' AND length(CAST(cascade_id AS BLOB))<=1024 THEN cascade_id END,
		CASE WHEN typeof(trajectory_id)='text' AND length(CAST(trajectory_id AS BLOB))<=1024 THEN trajectory_id END
		FROM trajectory_meta LIMIT 2`)
	if err != nil {
		return id, historysource.ErrFormat
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return id, historysource.ErrFormat
	}
	if err = rows.Scan(&id.CascadeID, &id.TrajectoryID); err != nil {
		return nativeIdentity{}, historysource.ErrFormat
	}
	if strings.TrimSpace(id.CascadeID) == "" || strings.TrimSpace(id.TrajectoryID) == "" || rows.Next() {
		return nativeIdentity{}, historysource.ErrFormat
	}
	return id, rows.Err()
}

// Read captures one read-only SQLite transaction. Cursor freezes the ordered step
// range and its digest; appends are excluded and prefix edits fail closed.
func Read(ctx context.Context, req historysource.Request) (historysource.Document, error) {
	out := historysource.Document{Source: req.Source, FormatVersion: FormatVersion, ParserVersion: ParserVersion}
	fail := func(err error) (historysource.Document, error) { return historysource.Document{}, safeError(ctx, err) }
	if req.Source.Agent != "agy" || req.Source.NativeID == "" {
		return fail(historysource.ErrIdentity)
	}
	if req.Source.Branch != "" {
		return fail(historysource.ErrFormat)
	}
	db, before, err := open(ctx, req.Source.Path)
	if err != nil {
		return fail(err)
	}
	out.Generation = historysource.FileIdentity(before)
	defer func() { _ = db.Close() }()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fail(err)
	}
	defer func() { _ = tx.Rollback() }()
	id, err := identity(ctx, tx)
	if err != nil {
		return fail(err)
	}
	if id.CascadeID != req.Source.NativeID {
		return fail(historysource.ErrIdentity)
	}
	var boundary int64
	var expected string
	if req.Cursor == "" {
		if err = tx.QueryRowContext(ctx, "SELECT coalesce(max(idx),-1) FROM steps").Scan(&boundary); err != nil {
			return fail(historysource.ErrFormat)
		}
	} else {
		parts := strings.SplitN(req.Cursor, ":", 4)
		if len(parts) != 4 || parts[0] != "agy3" || parts[3] != out.Generation || len(parts[2]) != 64 {
			return fail(historysource.ErrChanged)
		}
		boundary, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || boundary < -1 {
			return fail(historysource.ErrChanged)
		}
		expected = parts[2]
	}
	var count, total, maxSize int64
	err = tx.QueryRowContext(ctx, `SELECT count(*),coalesce(sum(coalesce(length(step_payload),0)+coalesce(length(metadata),0)),0),coalesce(max(max(coalesce(length(step_payload),0),coalesce(length(metadata),0))),0) FROM steps WHERE idx<=?`, boundary).Scan(&count, &total, &maxSize)
	if err != nil {
		return fail(historysource.ErrFormat)
	}
	if count > historysource.MaxRecords || total > historysource.MaxBytes || maxSize > historysource.MaxRecordBytes {
		return fail(historysource.ErrLimit)
	}
	hash := sha256.New()
	if st, ok := before.Sys().(*syscall.Stat_t); ok {
		_, _ = fmt.Fprintf(hash, "%d:%d:", st.Dev, st.Ino)
	}
	_, _ = fmt.Fprintf(hash, "%d:%s:%d:%s:%d:", len(id.CascadeID), id.CascadeID, len(id.TrajectoryID), id.TrajectoryID, boundary)
	rows, err := tx.QueryContext(ctx, `SELECT idx,step_type,status,has_subtrajectory,step_format,coalesce(metadata,x''),coalesce(step_payload,x'') FROM steps WHERE idx<=? ORDER BY idx LIMIT ?`, boundary, historysource.MaxRecords+1)
	if err != nil {
		return fail(historysource.ErrFormat)
	}
	var previous int64 = -1
	var parent string
	recordPrefix := id.CascadeID + "/trajectory/" + id.TrajectoryID + "/step/"
	for rows.Next() {
		if err = ctx.Err(); err != nil {
			_ = rows.Close()
			return fail(err)
		}
		var idx, typ, status, sub, format int64
		var meta, payload []byte
		if err = rows.Scan(&idx, &typ, &status, &sub, &format, &meta, &payload); err != nil {
			_ = rows.Close()
			return fail(historysource.ErrFormat)
		}
		if idx < 0 || idx <= previous || format != 0 {
			_ = rows.Close()
			return fail(historysource.ErrFormat)
		}
		_, _ = fmt.Fprintf(hash, "%d:%d:%d:%d:%d:%d:", idx, typ, status, sub, format, len(meta))
		hash.Write(meta)
		_, _ = fmt.Fprintf(hash, ":%d:", len(payload))
		hash.Write(payload)
		r := historysource.Record{ID: recordPrefix + decimal(idx), NativeID: decimal(idx), ParentID: parent, Status: stepStatus(status), Offset: idx, End: idx + 1}
		if idx != previous+1 {
			out.Gaps = append(out.Gaps, historysource.Gap{RecordID: r.ID, Category: "missing-step", Offset: idx})
		}
		if sub != 0 {
			out.Gaps = append(out.Gaps, historysource.Gap{RecordID: r.ID, Category: "external-subtrajectory", Offset: idx})
		}
		if status != 3 {
			out.Pending = true
		}
		records, gaps, e := project(payload, typ, status, r)
		if e != nil {
			_ = rows.Close()
			return fail(e)
		}
		out.Records = append(out.Records, records...)
		out.Gaps = append(out.Gaps, gaps...)
		if len(out.Records) > historysource.MaxRecords {
			_ = rows.Close()
			return fail(historysource.ErrLimit)
		}
		parent, previous = r.ID, idx
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return fail(err)
	}
	if previous != boundary {
		return fail(historysource.ErrChanged)
	}
	// Parent-reference payloads have no recovered schema. Never infer their chain.
	var external int
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM parent_references)").Scan(&external); err != nil {
		return fail(historysource.ErrFormat)
	}
	if external != 0 {
		out.Gaps = append(out.Gaps, historysource.Gap{Category: "external-parent-unresolved"})
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	out.Fingerprint = out.Generation + ":" + digest
	if expected != "" && expected != digest {
		return fail(historysource.ErrChanged)
	}
	out.Cursor = "agy3:" + decimal(boundary) + ":" + digest + ":" + out.Generation
	if err = tx.Commit(); err != nil {
		return fail(err)
	}
	after, err := os.Lstat(req.Source.Path)
	if err != nil || !os.SameFile(before, after) {
		return fail(historysource.ErrChanged)
	}
	return out, nil
}

func stepStatus(status int64) string {
	switch status {
	case 1:
		return "pending"
	case 2:
		return "running"
	case 3:
		return "done"
	case 8:
		return "generating"
	default:
		return "unknown"
	}
}

// Discovery retains identifiable candidates even if their body or trajectory
// mapping is unsupported. Read validates the unique candidate in a transaction.
func discoverIDs(ctx context.Context, path string) ([]string, error) {
	db, _, err := open(ctx, path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	rows, err := db.QueryContext(ctx, `SELECT
		CASE WHEN typeof(cascade_id)='text' AND length(CAST(cascade_id AS BLOB))<=1024 THEN cascade_id END
		FROM trajectory_meta LIMIT 1001`)
	if err != nil {
		return nil, safeError(ctx, historysource.ErrFormat)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for count := 0; rows.Next(); count++ {
		if count == 1000 {
			return nil, historysource.ErrLimit
		}
		var id sql.NullString
		if err := rows.Scan(&id); err != nil {
			return nil, safeError(ctx, historysource.ErrFormat)
		}
		if id.Valid && strings.TrimSpace(id.String) != "" {
			ids = append(ids, id.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, safeError(ctx, err)
	}
	return ids, nil
}

// Discover examines only the explicitly supplied data root, with bounded depth
// and entries. Runtime IDs come from trajectory_meta.cascade_id, never filenames.
func Discover(ctx context.Context, root string) ([]historysource.Source, error) {
	var sources []historysource.Source
	entries := 0
	var walk func(string, int) error
	walk = func(dir string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return historysource.ErrUnavailable
		}
		f, err := os.Open(dir)
		if err != nil {
			return historysource.ErrUnavailable
		}
		defer func() { _ = f.Close() }()
		for {
			children, readErr := f.ReadDir(64)
			if readErr != nil && readErr != io.EOF {
				return historysource.ErrUnavailable
			}
			for _, d := range children {
				if err := ctx.Err(); err != nil {
					return err
				}
				entries++
				if entries > 10000 {
					return historysource.ErrLimit
				}
				path := filepath.Join(dir, d.Name())
				if d.IsDir() {
					if depth < 4 {
						if err := walk(path, depth+1); err != nil {
							return err
						}
					}
					continue
				}
				if d.Type()&os.ModeSymlink != 0 || filepath.Ext(path) != ".db" {
					continue
				}
				ids, err := discoverIDs(ctx, path)
				if err != nil {
					if ctx.Err() != nil || err == historysource.ErrLimit {
						return err
					}
					// An unrelated database without a readable identity is not
					// a candidate. Unsupported bodies with an ID remain visible.
					continue
				}
				for _, id := range ids {
					sources = append(sources, historysource.Source{Agent: "agy", Path: path, NativeID: id})
					if len(sources) > 1000 {
						return historysource.ErrLimit
					}
				}
			}
			if readErr == io.EOF {
				return nil
			}
		}
	}
	if err := walk(filepath.Clean(root), 0); err != nil {
		return nil, safeError(ctx, err)
	}
	return sources, nil
}
