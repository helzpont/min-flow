// Package sql provides stream adapters for database operations using database/sql.
// It enables querying databases as part of flow pipelines.
package sql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lguimbarda/min-flow/flow/core"
)

// DefaultBufferSize is re-exported from core for convenience.
const DefaultBufferSize = core.DefaultBufferSize

// Scanner is a function that scans a row into a value.
type Scanner[T any] func(*sql.Rows) (T, error)

// Query creates a Stream that executes a query and emits results.
// The scanner function is called for each row to convert it to the output type.
func Query[T any](db *sql.DB, query string, scanner Scanner[T], args ...any) core.Stream[T] {
	return QueryBuffered(db, query, scanner, DefaultBufferSize, args...)
}

// QueryBuffered creates a Query stream with a specified buffer size.
func QueryBuffered[T any](db *sql.DB, query string, scanner Scanner[T], bufferSize int, args ...any) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T], bufferSize)
		go func() {
			defer close(out)
			rows, err := db.QueryContext(ctx, query, args...)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[T](err):
				}
				return
			}
			defer rows.Close()
			for rows.Next() {
				value, err := scanner(rows)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](err):
					}
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(value):
				}
			}
			if err := rows.Err(); err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[T](err):
				}
			}
		}()
		return out
	})
}

// QueryRow creates a Stream that executes a query expecting a single row.
func QueryRow[T any](db *sql.DB, query string, scanner func(*sql.Row) (T, error), args ...any) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T], 1)
		go func() {
			defer close(out)
			row := db.QueryRowContext(ctx, query, args...)
			value, err := scanner(row)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[T](err):
				}
				return
			}
			select {
			case <-ctx.Done():
			case out <- core.Ok(value):
			}
		}()
		return out
	})
}

// ExecResult contains the result of an exec operation.
type ExecResult struct {
	LastInsertId int64
	RowsAffected int64
}

// Exec creates a Stream that executes a statement and emits the result.
func Exec(db *sql.DB, query string, args ...any) core.Stream[ExecResult] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[ExecResult] {
		out := make(chan core.Result[ExecResult], 1)
		go func() {
			defer close(out)
			result, err := db.ExecContext(ctx, query, args...)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[ExecResult](err):
				}
				return
			}
			lastID, _ := result.LastInsertId()
			rowsAffected, _ := result.RowsAffected()
			execResult := ExecResult{
				LastInsertId: lastID,
				RowsAffected: rowsAffected,
			}
			select {
			case <-ctx.Done():
			case out <- core.Ok(execResult):
			}
		}()
		return out
	})
}

// ExecMany creates a Transformer that executes a statement for each input value.
// The binder function converts the input value to query arguments.
func ExecMany[T any](db *sql.DB, query string, binder func(T) []any) core.Transformer[T, ExecResult] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[ExecResult] {
		out := make(chan core.Result[ExecResult], DefaultBufferSize)
		go func() {
			defer close(out)
			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[ExecResult](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[ExecResult](res.Error()):
					}
					continue
				}
				args := binder(res.Value())
				result, err := db.ExecContext(ctx, query, args...)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[ExecResult](err):
					}
					continue
				}
				lastID, _ := result.LastInsertId()
				rowsAffected, _ := result.RowsAffected()
				execResult := ExecResult{
					LastInsertId: lastID,
					RowsAffected: rowsAffected,
				}
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(execResult):
				}
			}
		}()
		return out
	})
}

// Transaction executes a function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, it is committed.
func Transaction[T any](db *sql.DB, fn func(tx *sql.Tx) (T, error)) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T], 1)
		go func() {
			defer close(out)
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[T](err):
				}
				return
			}
			value, err := fn(tx)
			if err != nil {
				tx.Rollback()
				select {
				case <-ctx.Done():
				case out <- core.Err[T](err):
				}
				return
			}
			if err := tx.Commit(); err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[T](err):
				}
				return
			}
			select {
			case <-ctx.Done():
			case out <- core.Ok(value):
			}
		}()
		return out
	})
}

// QueryStrings is a convenience function that queries for string slices.
// Each row is scanned into a slice of strings.
func QueryStrings(db *sql.DB, query string, args ...any) core.Stream[[]string] {
	return Query(db, query, func(rows *sql.Rows) ([]string, error) {
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		result := make([]string, len(cols))
		for i, v := range values {
			switch val := v.(type) {
			case nil:
				result[i] = ""
			case []byte:
				result[i] = string(val)
			case string:
				result[i] = val
			case int64:
				result[i] = fmt.Sprintf("%d", val)
			case float64:
				result[i] = fmt.Sprintf("%g", val)
			case bool:
				result[i] = fmt.Sprintf("%t", val)
			default:
				result[i] = fmt.Sprintf("%v", val)
			}
		}
		return result, nil
	}, args...)
}

// QueryMaps is a convenience function that queries for map results.
// Each row is scanned into a map with column names as keys.
func QueryMaps(db *sql.DB, query string, args ...any) core.Stream[map[string]any] {
	return Query(db, query, func(rows *sql.Rows) (map[string]any, error) {
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		result := make(map[string]any, len(cols))
		for i, col := range cols {
			result[col] = values[i]
		}
		return result, nil
	}, args...)
}
