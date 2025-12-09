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

func GetUserByEmail(ctx context.Context, email string, role string) (interface{}, error) {
	if DB == nil {
		return nil, errors.New("database not initialized")
	}
	switch role {
	case "MINER":
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
	case "SUPERVISOR":
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
						created_at,
						updated_at
					FROM users
					WHERE LOWER(email) = LOWER($1)
					AND role = 'SUPERVISOR'
					LIMIT 1
				`

		var s Supervisor
		err := DB.QueryRowContext(ctx, query, email).Scan(
			&s.ID,
			&s.UserID,
			&s.Name,
			&s.Email,
			&s.Phone,
			&s.Password,
			&s.Role,
			&s.MiningSite,
			&s.Location,
			&s.CreatedAt,
			&s.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, sql.ErrNoRows
			}
			return nil, fmt.Errorf("GetSupervisorByEmail query failed: %w", err)
		}
		return &s, nil
	case "ADMIN":
		const query = `
					SELECT 
						id,
						user_id,
						name,
						email,
						phone,
						password,
						role,
						created_at,
						updated_at
					FROM users
					WHERE LOWER(email) = LOWER($1)
					AND role = 'ADMIN'
					LIMIT 1
				`

		var a Admin
		err := DB.QueryRowContext(ctx, query, email).Scan(
			&a.ID,
			&a.UserID,
			&a.Name,
			&a.Email,
			&a.Phone,
			&a.Password,
			&a.Role,
			&a.CreatedAt,
			&a.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, sql.ErrNoRows
			}
			return nil, fmt.Errorf("GetAdminByEmail query failed: %w", err)
		}
		return &a, nil
	}
	return nil, fmt.Errorf("invalid role")
}
