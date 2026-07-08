package postgres

import (
	"context"
	"encoding/json"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventRepository struct {
	pool *pgxpool.Pool
}

func NewEventRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{pool: pool}
}

func (r *EventRepository) Save(ctx context.Context, event domain.Event) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	var ip any
	if event.IPAnonymized != "" {
		ip = event.IPAnonymized
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO events (
			project_id, visitor_id, event_type, event_name, path, referrer,
			ip_anonymized, country, city, device_type, browser, os, metadata, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
	`,
		event.ProjectID, event.VisitorID, string(event.Type), event.Name, event.Path, event.Referrer,
		ip, event.Geo.Country, event.Geo.City, event.Device.DeviceType, event.Device.Browser, event.Device.OS,
		metadataJSON, event.CreatedAt,
	)
	return err
}

func (r *EventRepository) Stats(ctx context.Context, filter domain.StatsFilter) (domain.Stats, error) {
	exclude := ""
	if filter.ExcludeNamed {
		exclude = `AND NOT EXISTS (
			SELECT 1 FROM visitors v
			WHERE v.project_id = e.project_id AND v.visitor_id = e.visitor_id AND v.name <> ''
		)`
	}

	var stats domain.Stats
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT visitor_id)
		FROM events e
		WHERE e.project_id = $1 AND e.created_at >= $2 AND e.created_at < $3 `+exclude,
		filter.ProjectID, filter.Since, filter.Until,
	).Scan(&stats.TotalEvents, &stats.UniqueVisitors)
	if err != nil {
		return domain.Stats{}, err
	}

	if stats.TopPaths, err = r.topPaths(ctx, filter, exclude); err != nil {
		return domain.Stats{}, err
	}
	if stats.TopReferrers, err = r.topReferrers(ctx, filter, exclude); err != nil {
		return domain.Stats{}, err
	}
	if stats.VisitsOverTime, err = r.visitsOverTime(ctx, filter, exclude); err != nil {
		return domain.Stats{}, err
	}
	return stats, nil
}

func (r *EventRepository) topPaths(ctx context.Context, f domain.StatsFilter, exclude string) ([]domain.PathCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT path, COUNT(*) AS c
		FROM events e
		WHERE e.project_id = $1 AND e.created_at >= $2 AND e.created_at < $3 AND e.path <> '' `+exclude+`
		GROUP BY path
		ORDER BY c DESC
		LIMIT 10
	`, f.ProjectID, f.Since, f.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.PathCount
	for rows.Next() {
		var pc domain.PathCount
		if err := rows.Scan(&pc.Path, &pc.Count); err != nil {
			return nil, err
		}
		out = append(out, pc)
	}
	return out, rows.Err()
}

func (r *EventRepository) topReferrers(ctx context.Context, f domain.StatsFilter, exclude string) ([]domain.ReferrerCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT referrer, COUNT(*) AS c
		FROM events e
		WHERE e.project_id = $1 AND e.created_at >= $2 AND e.created_at < $3 AND e.referrer <> '' `+exclude+`
		GROUP BY referrer
		ORDER BY c DESC
		LIMIT 10
	`, f.ProjectID, f.Since, f.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ReferrerCount
	for rows.Next() {
		var rc domain.ReferrerCount
		if err := rows.Scan(&rc.Referrer, &rc.Count); err != nil {
			return nil, err
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}

func (r *EventRepository) visitsOverTime(ctx context.Context, f domain.StatsFilter, exclude string) ([]domain.TimeseriesPoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT date_trunc('day', e.created_at) AS bucket, COUNT(*) AS c
		FROM events e
		WHERE e.project_id = $1 AND e.created_at >= $2 AND e.created_at < $3 `+exclude+`
		GROUP BY bucket
		ORDER BY bucket ASC
	`, f.ProjectID, f.Since, f.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.TimeseriesPoint
	for rows.Next() {
		var tp domain.TimeseriesPoint
		if err := rows.Scan(&tp.Bucket, &tp.Count); err != nil {
			return nil, err
		}
		out = append(out, tp)
	}
	return out, rows.Err()
}
