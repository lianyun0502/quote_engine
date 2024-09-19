package rotatefile

import (
	"os"
	"sync"
	"time"

	strftime "github.com/lestrrat-go/strftime"
)

type Handler interface {
	Handle(Event)
}

type HandlerFunc func(Event)

type Event interface {
	Type() EventType
}

type EventType int

const (
	InvalidEventType EventType = iota
	FileRotatedEventType
)

type FileRotatedEvent struct {
	prev    string // previous filename
	current string // current, new filename
}

func (h HandlerFunc) Handle(e Event) {
	h(e)
}

func (e *FileRotatedEvent) Type() EventType {
	return FileRotatedEventType
}

func (e *FileRotatedEvent) PreviousFile() string {
	return e.prev
}

func (e *FileRotatedEvent) CurrentFile() string {
	return e.current
}

// RotateLogs represents a log file that gets
// automatically rotated as you write to it.
type RotateFile struct {
	clock         Clock
	curFn         string
	curBaseFn     string
	globPattern   string
	generation    int
	linkName      string
	maxAge        time.Duration
	mutex         sync.RWMutex
	eventHandler  Handler
	outFh         *os.File
	pattern       *strftime.Strftime
	rotationTime  time.Duration
	rotationSize  int64
	rotationCount uint
	forceNewFile  bool
}

// Clock is the interface used by the RotateLogs
// object to determine the current time
type Clock interface {
	Now() time.Time
}
type clockFn func() time.Time

// UTC is an object satisfying the Clock interface, which
// returns the current time in UTC
var UTC = clockFn(func() time.Time { return time.Now().UTC() })

// Local is an object satisfying the Clock interface, which
// returns the current time in the local timezone
var Local = clockFn(time.Now)

// Option is used to pass optional arguments to
// the RotateLogs constructor
type IOption interface {
	Name() string
	Value() interface{}
}

type Interface interface {
	Name() string
	Value() interface{}
}

type Option struct {
	name  string
	value interface{}
}

func NewOption(name string, value interface{}) *Option {
	return &Option{
		name:  name,
		value: value,
	}
}

func (o *Option) Name() string {
	return o.name
}
func (o *Option) Value() interface{} {
	return o.value
}
