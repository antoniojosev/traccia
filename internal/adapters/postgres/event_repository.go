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

// filterClause builds the extra AND conditions shared by every stats query
// below. Concatenation is safe here: the fragments are fixed strings picked
// from filter booleans, never raw user input — all user-supplied values
// stay bound as query parameters.
func filterClause(f domain.StatsFilter) string {
	clause := ""
	if f.ExcludeNamed {
		clause += ` AND NOT EXISTS (
			SELECT 1 FROM visitors v
			WHERE v.project_id = e.project_id AND v.visitor_id = e.visitor_id AND v.name <> ''
		)`
	}
	if !f.IncludeBots {
		clause += ` AND e.device_type <> 'bot'`
	}
	return clause
}

func (r *EventRepository) Stats(ctx context.Context, filter domain.StatsFilter) (domain.Stats, error) {
	exclude := filterClause(filter)

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
	if stats.DeviceTypes, err = r.groupBy(ctx, filter, exclude, "device_type", "e.device_type <> ''"); err != nil {
		return domain.Stats{}, err
	}
	if stats.Browsers, err = r.groupBy(ctx, filter, exclude, "browser", "e.browser <> ''"); err != nil {
		return domain.Stats{}, err
	}
	if stats.OperatingSystems, err = r.groupBy(ctx, filter, exclude, "os", "e.os <> ''"); err != nil {
		return domain.Stats{}, err
	}
	if stats.CustomEventNames, err = r.eventNameBreakdown(ctx, filter, exclude, domain.EventTypeCustom); err != nil {
		return domain.Stats{}, err
	}
	if stats.ErrorEventNames, err = r.eventNameBreakdown(ctx, filter, exclude, domain.EventTypeError); err != nil {
		return domain.Stats{}, err
	}
	return stats, nil
}

// groupBy runs the "group events by <column>, count, top 10" shape shared
// by the device/browser/OS breakdowns, which differ only in which column
// and non-empty predicate they use. column and nonEmptyPredicate are always
// fixed string literals supplied by this package, never user input, so
// concatenating them is safe — the same reasoning as filterClause above.
func (r *EventRepository) groupBy(ctx context.Context, f domain.StatsFilter, exclude, column, nonEmptyPredicate string) ([]domain.NameCount, error) {
	query := `
		SELECT ` + column + `, COUNT(*) AS c
		FROM events e
		WHERE e.project_id = $1 AND e.created_at >= $2 AND e.created_at < $3 AND ` + nonEmptyPredicate + exclude + `
		GROUP BY ` + column + `
		ORDER BY c DESC
		LIMIT 10
	`
	rows, err := r.pool.Query(ctx, query, f.ProjectID, f.Since, f.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.NameCount
	for rows.Next() {
		var nc domain.NameCount
		if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, err
		}
		out = append(out, nc)
	}
	return out, rows.Err()
}

func (r *EventRepository) eventNameBreakdown(ctx context.Context, f domain.StatsFilter, exclude string, eventType domain.EventType) ([]domain.NameCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT event_name, COUNT(*) AS c
		FROM events e
		WHERE e.project_id = $1 AND e.created_at >= $2 AND e.created_at < $3
			AND e.event_type = $4 AND e.event_name <> '' `+exclude+`
		GROUP BY event_name
		ORDER BY c DESC
		LIMIT 20
	`, f.ProjectID, f.Since, f.Until, string(eventType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.NameCount
	for rows.Next() {
		var nc domain.NameCount
		if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, err
		}
		out = append(out, nc)
	}
	return out, rows.Err()
}

// RecentByName drills into a single event name with its full metadata — a
// shape a grouped aggregate can't express, so it's a plain ordered list
// instead of a GROUP BY.
func (r *EventRepository) RecentByName(ctx context.Context, filter domain.StatsFilter, eventType domain.EventType, name string, limit int) ([]domain.EventDetail, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT visitor_id, metadata, created_at
		FROM events e
		WHERE e.project_id = $1 AND e.created_at >= $2 AND e.created_at < $3
			AND e.event_type = $4 AND e.event_name = $5
		ORDER BY e.created_at DESC
		LIMIT $6
	`, filter.ProjectID, filter.Since, filter.Until, string(eventType), name, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.EventDetail
	for rows.Next() {
		var detail domain.EventDetail
		var metadataJSON []byte
		if err := rows.Scan(&detail.VisitorID, &metadataJSON, &detail.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(metadataJSON, &detail.Metadata); err != nil {
			return nil, err
		}
		out = append(out, detail)
	}
	return out, rows.Err()
}

// MetadataBreakdown groups a single event name's occurrences by one
// metadata key's value — e.g. "calculator_used" events grouped by their
// "from_currency" value. Events missing that key entirely are excluded
// (the `metadata ? $5` "key exists" check) rather than lumped into a
// null/empty bucket.
func (r *EventRepository) MetadataBreakdown(ctx context.Context, filter domain.StatsFilter, eventType domain.EventType, eventName, metadataKey string, limit int) ([]domain.NameCount, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx, `
		SELECT metadata->>$5 AS value, COUNT(*) AS c
		FROM events e
		WHERE e.project_id = $1 AND e.created_at >= $2 AND e.created_at < $3
			AND e.event_type = $4 AND e.event_name = $6
			AND metadata ? $5
		GROUP BY value
		ORDER BY c DESC
		LIMIT $7
	`, filter.ProjectID, filter.Since, filter.Until, string(eventType), metadataKey, eventName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.NameCount
	for rows.Next() {
		var nc domain.NameCount
		if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, err
		}
		out = append(out, nc)
	}
	return out, rows.Err()
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
