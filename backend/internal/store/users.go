package store

import (
	"database/sql"
	"log"
	"time"

	"iss-dashboard-backend/internal/models"

	"golang.org/x/crypto/bcrypt"
)

func (s *Store) CreateUser(username, password, name, role string) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO user (username, password_hash, name, role, created_at) VALUES (?,?,?,?,?)`,
		username, string(hash), name, role, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &models.User{ID: id, Username: username, Name: name, Role: role, CreatedAt: now}, nil
}

func (s *Store) Authenticate(username, password string) (*models.User, error) {
	var u models.User
	row := s.db.QueryRow(`SELECT id, username, password_hash, name, role, created_at FROM user WHERE username=?`, username)
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Name, &u.Role, &u.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, nil
	}
	return &u, nil
}

func (s *Store) GetUserByID(id int64) (*models.User, error) {
	var u models.User
	row := s.db.QueryRow(`SELECT id, username, password_hash, name, role, created_at FROM user WHERE id=?`, id)
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Name, &u.Role, &u.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) ListUsers() ([]models.User, error) {
	rows, err := s.db.Query(`SELECT id, username, name, role, created_at FROM user ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM user WHERE id=?`, id)
	return err
}

func (s *Store) UpdateUserPassword(id int64, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE user SET password_hash=? WHERE id=?`, string(hash), id)
	return err
}

func (s *Store) UserCount() int {
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM user`).Scan(&count)
	return count
}

// EnsureDefaultAdmin creates a default admin user if no users exist.
func (s *Store) EnsureDefaultAdmin(adminToken string) {
	if s.UserCount() > 0 {
		return
	}
	password := adminToken
	if password == "" {
		password = generateRandomPassword(16)
		log.Printf("[INIT] ============================================")
		log.Printf("[INIT] Default admin user created")
		log.Printf("[INIT] Username: admin")
		log.Printf("[INIT] Password: %s", password)
		log.Printf("[INIT] CHANGE THIS PASSWORD IMMEDIATELY")
		log.Printf("[INIT] ============================================")
	}
	s.CreateUser("admin", password, "Administrateur", "admin")
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
