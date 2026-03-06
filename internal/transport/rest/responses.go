package rest

import (
	"fmt"
	"strings"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type connectionResponse struct {
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

func newConnectionResponse(c *connection.Connection) connectionResponse {
	return connectionResponse{
		ID:        c.ID.String(),
		Name:      c.Name,
		Driver:    c.Driver.String(),
		Host:      c.Host,
		Port:      c.Port,
		Username:  c.Username,
		UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}
}

type databaseResponse struct {
	Name          string `json:"name"`
	Owner         string `json:"owner"`
	Encoding      string `json:"encoding"`
	SizeFormatted string `json:"size_formatted"`
}

func newDatabaseResponse(d connection.Database) databaseResponse {
	return databaseResponse{
		Name:          d.Name,
		Owner:         d.Owner,
		Encoding:      d.Encoding,
		SizeFormatted: formatBytes(d.Size),
	}
}

type tableResponse struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	RowCount int64  `json:"row_count"`
}

func newTableResponse(t connection.Table) tableResponse {
	return tableResponse{
		Name:     t.Name,
		Type:     t.Type,
		Size:     t.Size,
		RowCount: t.RowCount,
	}
}

type overviewResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	Sessions  string `json:"sessions"`
	LatencyMs int64  `json:"latency_ms"`
}

func newOverviewResponse(h *connection.ServerHealth, latencyMs int64) overviewResponse {
	if h == nil {
		return overviewResponse{
			Status:    "unreachable",
			LatencyMs: latencyMs,
		}
	}
	return overviewResponse{
		Status:    "ONLINE",
		Version:   h.Version,
		Uptime:    formatUptime(h.Uptime),
		Sessions:  fmt.Sprintf("%d/%d", h.ActiveSessions, h.MaxConnections),
		LatencyMs: latencyMs,
	}
}

type sessionResponse struct {
	PID      int    `json:"pid"`
	User     string `json:"user"`
	Database string `json:"database"`
	State    string `json:"state"`
	Query    string `json:"query"`
	Duration string `json:"duration"`
	IsSlow   bool   `json:"is_slow"`
}

func newSessionResponse(s connection.Session) sessionResponse {
	return sessionResponse{
		PID:      s.PID,
		User:     s.User,
		Database: s.Database,
		State:    s.State,
		Query:    s.Query,
		Duration: formatClock(s.Duration),
		IsSlow:   s.Duration > time.Minute,
	}
}

type dbUserResponse struct {
	Name        string `json:"name"`
	IsSuperUser bool   `json:"is_superuser"`
	CanLogin    bool   `json:"can_login"`
	ConnLimit   int    `json:"conn_limit"`
}

func newDBUserResponse(u connection.DBUser) dbUserResponse {
	return dbUserResponse{
		Name:        u.Name.String(),
		IsSuperUser: u.IsSuperUser,
		CanLogin:    u.CanLogin,
		ConnLimit:   u.ConnLimit,
	}
}

type columnResponse struct {
	Name         string  `json:"name"`
	DataType     string  `json:"data_type"`
	IsNullable   bool    `json:"is_nullable"`
	IsPrimary    bool    `json:"is_primary"`
	DefaultValue *string `json:"default_value,omitempty"`
}

func newColumnResponse(c connection.Column) columnResponse {
	var defaultValue *string
	if c.DefaultValue != nil {
		s := c.DefaultValue.String()
		defaultValue = &s
	}
	return columnResponse{
		Name:         c.Name.String(),
		DataType:     c.DataType.Format(),
		IsNullable:   c.IsNullable,
		IsPrimary:    c.IsPrimary,
		DefaultValue: defaultValue,
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
