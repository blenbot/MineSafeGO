package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func GetSupervisorNameByUserID(ctx context.Context, supervisorUserID string) (string, error) {
	if DB == nil {
		return "", errors.New("database not initialized")
	}

	const query = `
        SELECT name
        FROM users
        WHERE user_id = $1
          AND role = 'SUPERVISOR'
        LIMIT 1
    `

	var name string
	err := DB.QueryRowContext(ctx, query, supervisorUserID).Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", sql.ErrNoRows
		}
		return "", fmt.Errorf("GetSupervisorNameByUserID query failed: %w", err)
	}

	return name, nil
}

func GetUserByEmail(ctx context.Context, email string, role string) (*User, error) {
	if DB == nil {
		return nil, errors.New("database not initialized")
	}

	const query = `
        SELECT 
            id,
            user_id,
            name,
            email,
            phone,
            password,
            role,
            mining_site,
            location,
            supervisor_id,
            created_at,
            updated_at
        FROM users
        WHERE LOWER(email) = LOWER($1)
          AND role = $2
        LIMIT 1
    `

	var u User
	err := DB.QueryRowContext(ctx, query, email, role).Scan(
		&u.ID,
		&u.UserID,
		&u.Name,
		&u.Email,
		&u.Phone,
		&u.Password,
		&u.Role,
		&u.MiningSite,
		&u.Location,
		&u.SupervisorID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("GetMinerByEmail query failed: %w", err)
	}

	return &u, nil
}
