package sql

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			age INTEGER NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO users (name, age) VALUES ('Alice', 30), ('Bob', 25), ('Charlie', 35)`)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}
	return db
}

type User struct {
	ID   int
	Name string
	Age  int
}

func TestQuery(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	stream := Query(db, "SELECT id, name, age FROM users ORDER BY id", func(rows *sql.Rows) (User, error) {
		var u User
		err := rows.Scan(&u.ID, &u.Name, &u.Age)
		return u, err
	})

	var users []User
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		users = append(users, res.Value())
	}

	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}
	if users[0].Name != "Alice" {
		t.Errorf("expected first user 'Alice', got %q", users[0].Name)
	}
	if users[1].Name != "Bob" {
		t.Errorf("expected second user 'Bob', got %q", users[1].Name)
	}
	if users[2].Name != "Charlie" {
		t.Errorf("expected third user 'Charlie', got %q", users[2].Name)
	}
}

func TestQueryWithArgs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	stream := Query(db, "SELECT id, name, age FROM users WHERE age > ?", func(rows *sql.Rows) (User, error) {
		var u User
		err := rows.Scan(&u.ID, &u.Name, &u.Age)
		return u, err
	}, 26)

	var users []User
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		users = append(users, res.Value())
	}

	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestQueryRow(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	stream := QueryRow(db, "SELECT id, name, age FROM users WHERE name = ?", func(row *sql.Row) (User, error) {
		var u User
		err := row.Scan(&u.ID, &u.Name, &u.Age)
		return u, err
	}, "Alice")

	results := stream.Collect(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IsError() {
		t.Fatalf("unexpected error: %v", results[0].Error())
	}
	user := results[0].Value()
	if user.Name != "Alice" || user.Age != 30 {
		t.Errorf("expected Alice(30), got %s(%d)", user.Name, user.Age)
	}
}

func TestExec(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	stream := Exec(db, "INSERT INTO users (name, age) VALUES (?, ?)", "David", 40)

	results := stream.Collect(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IsError() {
		t.Fatalf("unexpected error: %v", results[0].Error())
	}
	result := results[0].Value()
	if result.RowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", result.RowsAffected)
	}
	if result.LastInsertId != 4 {
		t.Errorf("expected last insert id 4, got %d", result.LastInsertId)
	}
}

func TestTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	stream := Transaction(db, func(tx *sql.Tx) (int64, error) {
		result, err := tx.Exec("INSERT INTO users (name, age) VALUES (?, ?)", "Eve", 28)
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	})

	results := stream.Collect(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IsError() {
		t.Fatalf("unexpected error: %v", results[0].Error())
	}
	if results[0].Value() != 4 {
		t.Errorf("expected last insert id 4, got %d", results[0].Value())
	}

	// Verify the data was committed
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count != 4 {
		t.Errorf("expected 4 users after transaction, got %d", count)
	}
}

func TestQueryStrings(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	stream := QueryStrings(db, "SELECT name, age FROM users ORDER BY id LIMIT 1")

	results := stream.Collect(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IsError() {
		t.Fatalf("unexpected error: %v", results[0].Error())
	}
	row := results[0].Value()
	if len(row) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(row))
	}
	if row[0] != "Alice" {
		t.Errorf("expected name 'Alice', got %q", row[0])
	}
}

func TestQuery_Error(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	stream := Query(db, "SELECT * FROM nonexistent_table", func(rows *sql.Rows) (User, error) {
		var u User
		err := rows.Scan(&u.ID, &u.Name, &u.Age)
		return u, err
	})

	results := stream.Collect(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].IsError() {
		t.Error("expected error for nonexistent table")
	}
}
