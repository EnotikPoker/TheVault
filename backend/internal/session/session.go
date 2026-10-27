package session

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"

	"thevault/backend/internal/vault"
)

var (
	ErrNotFound      = errors.New("session: not found")
	ErrAlreadyOpened = errors.New("session: vault is already open")
	ErrIDConflict    = errors.New("session: ID already exists")
)

type Status string

const (
	Locked Status = "locked"
	Opened Status = "opened"
)

type Attempt struct {
	Number     int
	Switches   vault.Combination
	Indicators [vault.SystemCount]vault.IndicatorResult
	Elapsed    time.Duration
}

// Session is the stored game, including its private puzzle. HTTP uses Snapshot instead.
type Session struct {
	ID           string
	Puzzle       vault.Vault
	CreatedAt    time.Time
	OpenedAt     *time.Time
	History      []Attempt
	Explanations *[vault.SystemCount]vault.Explanation
}

type Snapshot struct {
	ID           string
	CreatedAt    time.Time
	OpenedAt     *time.Time
	Status       Status
	Attempts     int
	Elapsed      time.Duration
	History      []Attempt
	Explanations *[vault.SystemCount]vault.Explanation
}

type Repository interface {
	Create(Session) error
	Get(id string) (Session, error)
	// Update commits the callback's changes atomically, or discards them on error.
	Update(id string, update func(*Session) error) (Session, error)
}

type Options struct {
	Now      func() time.Time
	NewID    func() (string, error)
	NewVault func() (vault.Vault, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
	newID      func() (string, error)
	newVault   func() (vault.Vault, error)
}

func NewService(repository Repository, options Options) *Service {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.NewID == nil {
		options.NewID = randomID
	}
	if options.NewVault == nil {
		options.NewVault = randomVault
	}
	return &Service{repository: repository, now: options.Now, newID: options.NewID, newVault: options.NewVault}
}

func (s *Service) Create() (Snapshot, error) {
	id, err := s.newID()
	if err != nil {
		return Snapshot{}, err
	}
	puzzle, err := s.newVault()
	if err != nil {
		return Snapshot{}, err
	}
	now := s.now()
	game := Session{ID: id, Puzzle: puzzle, CreatedAt: now, History: []Attempt{}}
	if err := s.repository.Create(game); err != nil {
		return Snapshot{}, err
	}
	return snapshot(game, now), nil
}

func (s *Service) Get(id string) (Snapshot, error) {
	game, err := s.repository.Get(id)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot(game, s.now()), nil
}

func (s *Service) Check(id string, combination vault.Combination) (Snapshot, error) {
	var now time.Time
	game, err := s.repository.Update(id, func(game *Session) error {
		if game.OpenedAt != nil {
			return ErrAlreadyOpened
		}
		now = s.now()
		result := game.Puzzle.Check(combination)
		if result.Opened {
			explanations, err := game.Puzzle.Explain(combination)
			if err != nil {
				return err
			}
			game.OpenedAt = &now
			game.Explanations = &explanations
		}
		game.History = append(game.History, Attempt{
			Number:     len(game.History) + 1,
			Switches:   combination,
			Indicators: result.Indicators,
			Elapsed:    now.Sub(game.CreatedAt),
		})
		return nil
	})
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot(game, now), nil
}

func snapshot(game Session, now time.Time) Snapshot {
	status := Locked
	if game.OpenedAt != nil {
		status = Opened
		now = *game.OpenedAt
	}
	return Snapshot{
		ID: game.ID, CreatedAt: game.CreatedAt, OpenedAt: game.OpenedAt,
		Status: status, Attempts: len(game.History), Elapsed: now.Sub(game.CreatedAt),
		History: game.History, Explanations: game.Explanations,
	}
}

func randomID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func randomVault() (vault.Vault, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return vault.Vault{}, err
	}
	return vault.Generate(int64(binary.BigEndian.Uint64(bytes[:])))
}
