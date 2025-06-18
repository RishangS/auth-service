package utils

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int
	Username     string
	PasswordHash string
	Email        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	IsActive     bool
}

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository() *UserRepository {
	user := "guest"
	password := "guest"
	dbname := "messanger"
	host := "localhost"
	port := "5432"

	// Standard connection string format
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(10) // Tune this based on DB config
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		log.Fatalf("Error pinging the database: %v", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Error opening database connection: %v", err)
	}

	log.Println("Successfully connected to the PostgreSQL database!")

	return &UserRepository{db: db}
}

// CreateUser creates a new user with hashed password
func (r *UserRepository) CreateUser(ctx context.Context, username, password, email string) (*User, error) {
	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Insert user into database
	query := `
		INSERT INTO users (username, password_hash, email)
		VALUES ($1, $2, $3)
		RETURNING id, username, password_hash, email, created_at, updated_at, is_active
	`

	user := &User{}
	err = r.db.QueryRowContext(ctx, query, username, string(hashedPassword), email).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsActive,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, errors.New("username or email already exists")
		}
		return nil, err
	}

	return user, nil
}

// AuthenticateUser verifies username and password
func (r *UserRepository) AuthenticateUser(ctx context.Context, username, password string) (*User, error) {
	query := `
		SELECT id, username, password_hash, email, created_at, updated_at, is_active
		FROM users
		WHERE username = $1
	`

	user := &User{}
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	if !user.IsActive {
		return nil, errors.New("account is not active")
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (r *UserRepository) GetUserByID(ctx context.Context, id int) (*User, error) {
	query := `
		SELECT id, username, email, created_at, updated_at, is_active
		FROM users
		WHERE id = $1
	`

	user := &User{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsActive,
	)

	if err != nil {
		return nil, err
	}

	return user, nil
}
