package utils

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
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

// Message represents a message in the database
type Message struct {
	ID          int       `json:"id"`
	SenderID    int       `json:"sender_id"`
	RecipientID int       `json:"recipient_id"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
	IsRead      bool      `json:"is_read"`
}

type UserRepository struct {
	db *sql.DB
}

func NewDBService() *UserRepository {
	user := getEnv("DB_USER", "guest")
	password := getEnv("DB_PASSWORD", "guest")
	dbname := getEnv("DB_NAME", "messanger")
	host := getEnv("DB_HOST", "postgres")
	port := getEnv("DB_PORT", "5432")

	// Standard connection string format
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbname)
	fmt.Println("\n-------->", connStr)
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

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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

// CreateMessage inserts a new message into the database
func (r *UserRepository) CreateMessage(sender, recipient, content string) (int, error) {
	var messageID int
	err := r.db.QueryRow(
		`INSERT INTO messages (sender_id, recipient_id, content) 
		VALUES ((SELECT id FROM users WHERE username = $1), (SELECT id FROM users WHERE username = $2), $3) 
		RETURNING id`,
		sender, recipient, content,
	).Scan(&messageID)

	if err != nil {
		return 0, fmt.Errorf("error creating message: %w", err)
	}
	return messageID, nil
}

// GetMessage retrieves a single message by ID
func (r *UserRepository) GetMessage(messageID int) (*Message, error) {
	var msg Message
	err := r.db.QueryRow(
		`SELECT id, sender_id, recipient_id, content, created_at, is_read 
		FROM messages 
		WHERE id = $1`,
		messageID,
	).Scan(&msg.ID, &msg.SenderID, &msg.RecipientID, &msg.Content, &msg.CreatedAt, &msg.IsRead)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("error getting message: %w", err)
	}
	return &msg, nil
}

// GetMessagesByUser retrieves all messages for a specific user
func (r *UserRepository) GetMessagesByUser(userID int, limit, offset int) ([]Message, error) {
	rows, err := r.db.Query(
		`SELECT id, sender_id, recipient_id, content, created_at, is_read 
		FROM messages 
		WHERE recipient_id = $1 OR sender_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(
			&msg.ID, &msg.SenderID, &msg.RecipientID,
			&msg.Content, &msg.CreatedAt, &msg.IsRead,
		); err != nil {
			return nil, fmt.Errorf("error scanning message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return messages, nil
}

// GetConversation retrieves messages between two users
func (r *UserRepository) GetConversation(user1ID, user2ID int, limit, offset int) ([]Message, error) {
	rows, err := r.db.Query(
		`SELECT id, sender_id, recipient_id, content, created_at, is_read 
		FROM messages 
		WHERE (sender_id = $1 AND recipient_id = $2)
		OR (sender_id = $2 AND recipient_id = $1)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`,
		user1ID, user2ID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying conversation: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(
			&msg.ID, &msg.SenderID, &msg.RecipientID,
			&msg.Content, &msg.CreatedAt, &msg.IsRead,
		); err != nil {
			return nil, fmt.Errorf("error scanning message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return messages, nil
}

// UpdateMessage updates a message's content
func (r *UserRepository) UpdateMessage(messageID int, newContent string) error {
	result, err := r.db.Exec(
		`UPDATE messages 
		SET content = $1 
		WHERE id = $2`,
		newContent, messageID,
	)
	if err != nil {
		return fmt.Errorf("error updating message: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("message not found")
	}

	return nil
}

// MarkAsRead updates the read status of a message
func (r *UserRepository) MarkAsRead(messageID int) error {
	result, err := r.db.Exec(
		`UPDATE messages 
		SET is_read = true 
		WHERE id = $1`,
		messageID,
	)
	if err != nil {
		return fmt.Errorf("error marking message as read: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("message not found")
	}

	return nil
}

// DeleteMessage removes a message from the database
func (r *UserRepository) DeleteMessage(messageID int) error {
	result, err := r.db.Exec(
		`DELETE FROM messages 
		WHERE id = $1`,
		messageID,
	)
	if err != nil {
		return fmt.Errorf("error deleting message: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("message not found")
	}

	return nil
}

// GetUnreadCount returns the count of unread messages for a user
func (r *UserRepository) GetUnreadCount(userID int) (int, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) 
		FROM messages 
		WHERE recipient_id = $1 AND is_read = false`,
		userID,
	).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("error getting unread count: %w", err)
	}
	return count, nil
}
