package dtos

import (
	"fmt"
	"strings"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type ConnectionDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	UpdatedAt string `json:"updated_at"`
	CreatedAt string `json:"created_at"`
	Status    string `json:"status"`
	StatusErr string `json:"status_error,omitempty"`
}

type DatabaseDTO struct {
	Name          string `json:"name"`
	Owner         string `json:"owner"`
	Encoding      string `json:"encoding"`
	SizeFormatted string `json:"size_formatted"`
}

type TableDTO struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	RowCount int64  `json:"row_count"`
}

const (
	StatusOK          = "ok"
	StatusError       = "error"
	StatusOnline      = "ONLINE"
	StatusUnreachable = "UNREACHABLE"
)

type OverviewDTO struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	Sessions  string `json:"sessions"`
	LatencyMs int64  `json:"latency_ms"`
}

type SessionDTO struct {
	PID      int    `json:"pid"`
	User     string `json:"user"`
	Database string `json:"database"`
	State    string `json:"state"`
	Query    string `json:"query"`
	Duration string `json:"duration"`
	IsSlow   bool   `json:"is_slow"`
}

type DBUserDTO struct {
	Name        string `json:"name"`
	IsSuperUser bool   `json:"is_superuser"`
	CanLogin    bool   `json:"can_login"`
	ConnLimit   int    `json:"conn_limit"`
}

func NewConnectionDTO(conn *connection.Connection) *ConnectionDTO {
	return &ConnectionDTO{
		ID:        conn.ID.String(),
		Name:      conn.Name,
		Driver:    conn.Driver.String(),
		Host:      conn.Host,
		Port:      conn.Port,
		Username:  conn.Username,
		UpdatedAt: conn.UpdatedAt.Format(time.RFC3339),
		CreatedAt: conn.CreatedAt.Format(time.RFC3339),
	}
}

func NewDatabaseDTO(database connection.Database) *DatabaseDTO {
	return &DatabaseDTO{
		Name:          database.Name,
		Owner:         database.Owner,
		Encoding:      database.Encoding,
		SizeFormatted: formatBytes(database.Size),
	}
}

func NewTableDTO(table connection.Table) *TableDTO {
	return &TableDTO{
		Name:     table.Name,
		Type:     table.Type,
		Size:     table.Size,
		RowCount: table.RowCount,
	}
}

func NewOverviewDTO(health *connection.ServerHealth, latencyMs int64) *OverviewDTO {
	return &OverviewDTO{
		Status:    StatusOnline,
		Version:   health.Version,
		Uptime:    formatUptime(health.Uptime),
		Sessions:  fmt.Sprintf("%d/%d", health.ActiveSessions, health.MaxConnections),
		LatencyMs: latencyMs,
	}
}

func NewOverviewUnreachableDTO(latencyMs int64) *OverviewDTO {
	return &OverviewDTO{
		Status:    StatusUnreachable,
		LatencyMs: latencyMs,
	}
}

func NewSessionDTO(session connection.Session) *SessionDTO {
	return &SessionDTO{
		PID:      session.PID,
		User:     session.User,
		Database: session.Database,
		State:    session.State,
		Query:    session.Query,
		Duration: formatClock(session.Duration),
		IsSlow:   session.Duration > time.Minute,
	}
}

func NewDBUserDTO(user connection.DBUser) *DBUserDTO {
	return &DBUserDTO{
		Name:        user.Name.String(),
		IsSuperUser: user.IsSuperUser,
		CanLogin:    user.CanLogin,
		ConnLimit:   user.ConnLimit,
	}
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div := float64(unit)
	exp := 0
	for n := float64(size) / div; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}

	suffixes := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(size)/div, suffixes[exp])
}

func formatUptime(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}

	days := d / (24 * time.Hour)
	hours := (d % (24 * time.Hour)) / time.Hour
	minutes := (d % time.Hour) / time.Minute
	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 && days == 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", int(d/time.Second)))
	}

	return strings.Join(parts, " ")
}

func formatClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	d = d.Round(time.Second)
	hours := int(d / time.Hour)
	minutes := int((d % time.Hour) / time.Minute)
	seconds := int((d % time.Minute) / time.Second)

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
