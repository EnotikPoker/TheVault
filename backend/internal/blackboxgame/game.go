package blackboxgame

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"
	_ "time/tzdata"

	"thevault/backend/internal/blackbox"
)

var (
	ErrInvalid  = errors.New("blackbox game: invalid request")
	ErrNotFound = errors.New("blackbox game: session not found")
	ErrOpened   = errors.New("blackbox game: already opened")
	ErrConflict = errors.New("blackbox game: request ID reused with different content")
)

type Mark struct {
	blackbox.Cell
	State string `json:"state"`
}

type Scan struct {
	Number    int             `json:"number"`
	Entry     blackbox.Port   `json:"entry"`
	Result    blackbox.Result `json:"result"`
	ElapsedMs int64           `json:"elapsedMs"`
	Path      []blackbox.Cell `json:"path,omitempty"`
}

type Answer struct {
	RequestID string          `json:"requestId"`
	Cells     []blackbox.Cell `json:"cells"`
	Correct   bool            `json:"correct"`
}

// Session is persisted privately; only Snapshot is returned over HTTP.
type Session struct {
	ID        string         `json:"id"`
	Mode      string         `json:"mode"`
	Date      string         `json:"date,omitempty"`
	PuzzleID  int64          `json:"-"`
	Board     blackbox.Board `json:"-"`
	CreatedAt time.Time      `json:"createdAt"`
	OpenedAt  *time.Time     `json:"openedAt,omitempty"`
	Marks     []Mark         `json:"marks"`
	Scans     []Scan         `json:"scans"`
	Answers   []Answer       `json:"answers"`
}

type Snapshot struct {
	ID           string          `json:"id"`
	Mode         string          `json:"mode"`
	Date         string          `json:"date,omitempty"`
	Size         int             `json:"size"`
	ElementCount int             `json:"elementCount"`
	RulesVersion int             `json:"rulesVersion"`
	Status       string          `json:"status"`
	CreatedAt    time.Time       `json:"createdAt"`
	OpenedAt     *time.Time      `json:"openedAt,omitempty"`
	ElapsedMs    int64           `json:"elapsedMs"`
	ScanCount    int             `json:"scanCount"`
	WrongAnswers int             `json:"wrongAnswers"`
	Score        int             `json:"score"`
	Marks        []Mark          `json:"marks"`
	Scans        []Scan          `json:"scans"`
	Answers      []Answer        `json:"answers"`
	Solution     []blackbox.Cell `json:"solution,omitempty"`
}

type DailyInfo struct {
	Date         string    `json:"date"`
	ResetsAt     time.Time `json:"resetsAt"`
	Size         int       `json:"size"`
	ElementCount int       `json:"elementCount"`
}

type Repository interface {
	// Create resolves an existing daily puzzle atomically before storing the session.
	Create(context.Context, Session) (Session, error)
	Get(context.Context, string) (Session, error)
	Update(context.Context, string, func(*Session) error) (Session, error)
}

type Options struct {
	Now      func() time.Time
	NewID    func() (string, error)
	NewBoard func() (blackbox.Board, error)
}

type Service struct {
	repository Repository
	options    Options
	location   *time.Location
}

func NewService(repository Repository, options Options) *Service {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.NewID == nil {
		options.NewID = randomID
	}
	if options.NewBoard == nil {
		options.NewBoard = randomBoard
	}
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		panic(err)
	}
	return &Service{repository: repository, options: options, location: location}
}

func (s *Service) Daily() DailyInfo {
	now := s.options.Now().In(s.location)
	year, month, day := now.Date()
	return DailyInfo{
		Date: now.Format(time.DateOnly), ResetsAt: time.Date(year, month, day+1, 0, 0, 0, 0, s.location).UTC(),
		Size: blackbox.Size, ElementCount: blackbox.ElementCount,
	}
}

func (s *Service) Create(ctx context.Context, mode string) (Snapshot, error) {
	if mode != "daily" && mode != "practice" {
		return Snapshot{}, ErrInvalid
	}
	id, err := s.options.NewID()
	if err != nil {
		return Snapshot{}, err
	}
	board, err := s.options.NewBoard()
	if err != nil {
		return Snapshot{}, err
	}
	now := s.options.Now().UTC()
	game := Session{ID: id, Mode: mode, Board: board, CreatedAt: now, Marks: []Mark{}, Scans: []Scan{}, Answers: []Answer{}}
	if mode == "daily" {
		game.Date = now.In(s.location).Format(time.DateOnly)
	}
	created, err := s.repository.Create(ctx, game)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot(created, now)
}

func (s *Service) Get(ctx context.Context, id string) (Snapshot, error) {
	game, err := s.repository.Get(ctx, id)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot(game, s.options.Now())
}

func (s *Service) Scan(ctx context.Context, id string, port blackbox.Port) (Snapshot, error) {
	if !port.Valid() {
		return Snapshot{}, ErrInvalid
	}
	return s.update(ctx, id, func(game *Session) error {
		for _, scan := range game.Scans {
			if scan.Entry == port || (scan.Result.Exit != nil && *scan.Result.Exit == port) {
				return nil
			}
		}
		if game.OpenedAt != nil {
			return ErrOpened
		}
		trace, err := game.Board.Trace(port)
		if err != nil {
			return err
		}
		game.Scans = append(game.Scans, Scan{
			Number: len(game.Scans) + 1, Entry: port, Result: trace.Result,
			ElapsedMs: elapsed(s.options.Now(), game.CreatedAt),
		})
		return nil
	})
}

func (s *Service) SetMarks(ctx context.Context, id string, marks []Mark) (Snapshot, error) {
	if marks == nil || len(marks) > blackbox.Size*blackbox.Size {
		return Snapshot{}, ErrInvalid
	}
	seen := make(map[blackbox.Cell]bool)
	selected := 0
	for _, mark := range marks {
		if !mark.Cell.Valid() || seen[mark.Cell] || (mark.State != "suspected" && mark.State != "empty") {
			return Snapshot{}, ErrInvalid
		}
		seen[mark.Cell] = true
		if mark.State == "suspected" {
			selected++
		}
	}
	if selected > blackbox.ElementCount {
		return Snapshot{}, ErrInvalid
	}
	return s.update(ctx, id, func(game *Session) error {
		if game.OpenedAt != nil {
			return ErrOpened
		}
		game.Marks = append([]Mark{}, marks...)
		return nil
	})
}

func (s *Service) Answer(ctx context.Context, id, requestID string, cells []blackbox.Cell) (Snapshot, error) {
	decoded, err := hex.DecodeString(requestID)
	guess, guessErr := blackbox.NewBoard(cells)
	if err != nil || len(decoded) != 16 || guessErr != nil {
		return Snapshot{}, ErrInvalid
	}
	requestID = strings.ToLower(requestID)
	cells = guess.Cells()
	return s.update(ctx, id, func(game *Session) error {
		for _, answer := range game.Answers {
			if answer.RequestID == requestID {
				if !slices.Equal(answer.Cells, cells) {
					return ErrConflict
				}
				return nil
			}
		}
		if game.OpenedAt != nil {
			return ErrOpened
		}
		correct := game.Board == guess
		game.Answers = append(game.Answers, Answer{RequestID: requestID, Cells: slices.Clone(cells), Correct: correct})
		if correct {
			now := s.options.Now().UTC()
			game.OpenedAt = &now
		}
		return nil
	})
}

func (s *Service) update(ctx context.Context, id string, change func(*Session) error) (Snapshot, error) {
	game, err := s.repository.Update(ctx, id, change)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot(game, s.options.Now())
}

func snapshot(game Session, now time.Time) (Snapshot, error) {
	state := Snapshot{
		ID: game.ID, Mode: game.Mode, Date: game.Date, Size: blackbox.Size, ElementCount: blackbox.ElementCount,
		RulesVersion: blackbox.RulesVersion, Status: "locked", CreatedAt: game.CreatedAt,
		ScanCount: len(game.Scans), Marks: append([]Mark{}, game.Marks...),
		Scans: append([]Scan{}, game.Scans...), Answers: make([]Answer, len(game.Answers)),
	}
	for index, answer := range game.Answers {
		answer.Cells = slices.Clone(answer.Cells)
		state.Answers[index] = answer
		if !answer.Correct {
			state.WrongAnswers++
		}
	}
	state.Score = state.ScanCount + 5*state.WrongAnswers
	if game.OpenedAt != nil {
		state.Status = "opened"
		openedAt := *game.OpenedAt
		state.OpenedAt = &openedAt
		now = openedAt
		state.Solution = game.Board.Cells()
		for index, scan := range state.Scans {
			trace, err := game.Board.Trace(scan.Entry)
			if err != nil {
				return Snapshot{}, err
			}
			state.Scans[index].Path = trace.Path
		}
	}
	state.ElapsedMs = elapsed(now, game.CreatedAt)
	return state, nil
}

func elapsed(now, start time.Time) int64 { return max(0, now.Sub(start).Milliseconds()) }

func randomID() (string, error) {
	var bytes [16]byte
	_, err := rand.Read(bytes[:])
	return hex.EncodeToString(bytes[:]), err
}

func randomBoard() (blackbox.Board, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return blackbox.Board{}, err
	}
	return blackbox.Generate(int64(binary.BigEndian.Uint64(bytes[:])))
}
