package postgres

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thevault/backend/internal/blackbox"
	"thevault/backend/internal/blackboxgame"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// All server processes use the same lock while applying schema changes.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(812473601)"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS blackbox_schema_versions (version INTEGER PRIMARY KEY)"); err != nil {
		return err
	}
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	for _, file := range files {
		version, err := strconv.Atoi(strings.SplitN(path.Base(file), "_", 2)[0])
		if err != nil {
			return err
		}
		var applied bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM blackbox_schema_versions WHERE version=$1)", version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		sql, err := migrations.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %s: %w", file, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO blackbox_schema_versions(version) VALUES ($1)", version); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) Create(ctx context.Context, game blackboxgame.Session) (blackboxgame.Session, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return game, err
	}
	defer tx.Rollback(ctx)
	var date any
	if game.Mode == "daily" {
		date = game.Date
	}
	var layout int64
	err = tx.QueryRow(ctx, `INSERT INTO blackbox_puzzles(puzzle_date, rules_version, layout)
		VALUES ($1::date, $2, $3) ON CONFLICT(puzzle_date) DO NOTHING RETURNING id, layout`,
		date, blackbox.RulesVersion, int64(game.Board.Mask())).Scan(&game.PuzzleID, &layout)
	if errors.Is(err, pgx.ErrNoRows) {
		var version int
		err = tx.QueryRow(ctx, "SELECT id, layout, rules_version FROM blackbox_puzzles WHERE puzzle_date=$1::date", date).
			Scan(&game.PuzzleID, &layout, &version)
		if err == nil && version != blackbox.RulesVersion {
			err = errors.New("unsupported Black Box rules version")
		}
	}
	if err != nil {
		return game, err
	}
	game.Board, err = blackbox.FromMask(uint64(layout))
	if err != nil {
		return game, err
	}
	data, err := json.Marshal(game)
	if err != nil {
		return game, err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO blackbox_sessions(id,puzzle_id,state) VALUES ($1,$2,$3::jsonb)", game.ID, game.PuzzleID, data); err != nil {
		return game, err
	}
	return game, tx.Commit(ctx)
}

func (s *Store) Get(ctx context.Context, id string) (blackboxgame.Session, error) {
	return read(s.pool.QueryRow(ctx, selectSession+" WHERE s.id=$1", id))
}

func (s *Store) Update(ctx context.Context, id string, update func(*blackboxgame.Session) error) (blackboxgame.Session, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return blackboxgame.Session{}, err
	}
	defer tx.Rollback(ctx)
	game, err := read(tx.QueryRow(ctx, selectSession+" WHERE s.id=$1 FOR UPDATE OF s", id))
	if err != nil {
		return game, err
	}
	if err := update(&game); err != nil {
		return blackboxgame.Session{}, err
	}
	data, err := json.Marshal(game)
	if err != nil {
		return game, err
	}
	if _, err := tx.Exec(ctx, "UPDATE blackbox_sessions SET state=$2::jsonb WHERE id=$1", id, data); err != nil {
		return game, err
	}
	return game, tx.Commit(ctx)
}

const selectSession = `SELECT s.puzzle_id, s.state, p.layout, p.rules_version
	FROM blackbox_sessions s JOIN blackbox_puzzles p ON p.id=s.puzzle_id`

func read(row pgx.Row) (blackboxgame.Session, error) {
	var game blackboxgame.Session
	var data []byte
	var puzzleID, layout int64
	var version int
	if err := row.Scan(&puzzleID, &data, &layout, &version); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return game, blackboxgame.ErrNotFound
		}
		return game, err
	}
	if version != blackbox.RulesVersion {
		return game, errors.New("unsupported Black Box rules version")
	}
	if err := json.Unmarshal(data, &game); err != nil {
		return game, err
	}
	board, err := blackbox.FromMask(uint64(layout))
	game.Board, game.PuzzleID = board, puzzleID
	return game, err
}
