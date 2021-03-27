package scoreboard

import (
	"fmt"
	"strings"
)

const (
	SortOrderAscending = iota
	SortOrderDescending
)

// Scoreboard represents a scoreboard that may be sent to a player. The scoreboard is shown on the right side
// of the player's screen.
type Scoreboard struct {
	name  string
	lines []string
	order int
}

// New returns a new scoreboard with the order and display name passed. Once returned, lines may be added to
// the scoreboard to add text to it. The name is formatted according to the rules of fmt.Sprintln. The
// different order values can be found above.Changing the scoreboard after sending it to a player will not
// update the scoreboard of the player automatically: Player.SendScoreboard() must be called again to update
// it.
func New(order int, name ...interface{}) *Scoreboard {
	return &Scoreboard{name: format(name), order: order}
}

// Name returns the display name of the scoreboard, as passed during the construction of the scoreboard.
func (board *Scoreboard) Name() string {
	return board.name
}

// Add adds a new line to the scoreboard using the content passed. The values passed are formatting according
// to the rules of fmt.Sprintln.
func (board *Scoreboard) Add(a ...interface{}) *Scoreboard {
	board.lines = append(board.lines, board.pad(format(a)))
	return board
}

// Addf adds a new line to the scoreboard using a custom format. The formatting specifiers are the same as
// those of fmt.Sprintf.
func (board *Scoreboard) Addf(format string, a ...interface{}) *Scoreboard {
	board.lines = append(board.lines, board.pad(fmt.Sprintf(format, a...)))
	return board
}

// Set sets a line on the scoreboard to a new value passed, formatting the values according to the rules of
// fmt.Sprintln. Set panics if the index passed is out of range: New lines must be added using Scoreboard.Add.
func (board *Scoreboard) Set(index int, a ...interface{}) *Scoreboard {
	if index >= len(board.lines) || index < 0 {
		panic(fmt.Sprintf("scoreboard: index out of range: index %v is not valid for scoreboard of size %v", index, len(board.lines)))
	}
	board.lines[index] = board.pad(format(a))
	return board
}

// Setf sets a line on the scoreboard to a new value passed, formatting the values according to the rules of
// fmt.Sprintf with a custom format. Setf panics if the index passed is out of range: New lines must be added
// using Scoreboard.Addf.
func (board *Scoreboard) Setf(index int, format string, a ...interface{}) *Scoreboard {
	if index >= len(board.lines) || index < 0 {
		panic(fmt.Sprintf("scoreboard: index out of range: index %v is not valid for scoreboard of size %v", index, len(board.lines)))
	}
	board.lines[index] = board.pad(fmt.Sprintf(format, a...))
	return board
}

// Remove removes the line with the index passed and shifts down all lines after it. Remove panics if the
// index passed is out of range.
func (board *Scoreboard) Remove(index int) *Scoreboard {
	if index >= len(board.lines) || index < 0 {
		panic(fmt.Sprintf("scoreboard: index out of range: index %v is not valid for scoreboard of size %v", index, len(board.lines)))
	}
	board.lines = append(board.lines[:index], board.lines[index+1:]...)
	return board
}

// RemoveLast removes the last line of the scoreboard. Nothing happens if the scoreboard is empty.
func (board *Scoreboard) RemoveLast() *Scoreboard {
	if len(board.lines) == 0 {
		return board
	}
	board.lines = board.lines[:len(board.lines)-1]
	return board
}

// Clear clears all lines from the scoreboard and resets it to its state directly after initialising the
// scoreboard.
func (board *Scoreboard) Clear() *Scoreboard {
	board.lines = nil
	return board
}

// Lines returns a list of all lines of the scoreboard. The order is the order in which they were added using
// Scoreboard.Add().
func (board *Scoreboard) Lines() []string {
	return board.lines
}

// Order returns the order the scoreboard is in. The different order values can be found above.
func (board *Scoreboard) Order() int {
	return board.order
}

// pad pads the string passed for as much as needed to achieve the same length as the name of the scoreboard.
// If the string passed is already of the same length as the name of the scoreboard or longer, the string will
// receive one space of padding.
func (board *Scoreboard) pad(s string) string {
	if len(board.name)-len(s)-2 <= 0 {
		return " " + s + " "
	}
	return " " + s + strings.Repeat(" ", len(board.name)-len(s)-2)
}

// format is a utility function to format a list of values to have spaces between them, but no newline at the
// end, which is typically used for sending messages, popups and tips.
func format(a []interface{}) string {
	return strings.TrimSuffix(fmt.Sprintln(a...), "\n")
}
