package data_struct

import (
	"errors"
	"fmt"
)

type Rider interface {
	Info() string
}

type Horse struct {
	rider Rider
	right *Horse
	left  *Horse
}

type MerryGo struct {
	Head  *Horse
	Tail  *Horse
	Size  int
	Count int
}

func NewMerryGo(size int) *MerryGo {
	return &MerryGo{Size: size, Count: 0}
}

/*
append 해당 함수는 MerryGo에 Horse와 Rider를 추가합니다. 더 이상 MerryGo에 자리가 없을 경우 에러가 납니다

rider Rider: MerryGo에 들어갈 데이터
*/
func (m *MerryGo) append(rider Rider) error {
	if m.IsFull() {
		return errors.New("MerryGo is full")
	}

	newHorse := &Horse{rider: rider}
	if m.Head == nil && m.Tail == nil {
		m.Head = newHorse
		m.Tail = newHorse
		m.Head.left = newHorse
		m.Head.right = newHorse
		m.Tail.left = newHorse
		m.Tail.right = newHorse
		return nil
	}

	newHorse.right = m.Head
	newHorse.left = m.Tail
	if m.Head == m.Tail {
		m.Head.right = newHorse
	}
	m.Tail = newHorse
	m.Head.left = m.Tail

	m.Count++

	return nil
}

/*
popTail 해당 함수는 MerryGo의 Tail에 해당하는 Horse를 빼냅니다.
*/
func (m *MerryGo) popTail() (horse *Horse, err error) {
	var popHorse *Horse

	if m.IsEmpty() {
		return popHorse, errors.New("MerryGo is empty")
	}

	popHorse = m.Tail
	if m.Count == 1 {
		m.Head = nil
		m.Tail = nil
		return popHorse, nil
	}

	m.Tail.left.right = m.Head
	m.Tail = m.Tail.left
	m.Head.left = m.Tail

	m.Count--

	return popHorse, nil
}

/*
popHead 해당 함수는 MerryGo의 Head에 해당하는 Horse를 빼냅니다.
*/
func (m *MerryGo) popHead() (horse *Horse, err error) {
	var popHorse *Horse

	if m.IsEmpty() {
		return popHorse, errors.New("MerryGo is empty")
	}

	popHorse = m.Head
	if m.Count == 1 {
		m.Head = nil
		m.Tail = nil
		return popHorse, nil
	}

	m.Head.right.left = m.Tail
	m.Head = m.Head.right
	m.Tail.right = m.Head

	m.Count--

	return popHorse, nil
}

func (m *MerryGo) IsFull() bool {
	return m.Count == m.Size
}

func (m *MerryGo) IsEmpty() bool {
	return m.Count == 0
}

/*
rotate MerryGo 내의 모든 Rider의 위치를 left로 이동시킵니다.
*/
func (m *MerryGo) rotate() error {
	if m.IsEmpty() {
		return errors.New("MerryGo is empty")
	}

	m.Head = m.Head.right
	return nil
}

/*
display MerryGo 내의 모든 데이터를 Head -> Tail 순으로 순환하여 리턴합니다.
*/
func (m *MerryGo) display() ([]*Horse, error) {
	if m.IsEmpty() {
		return nil, errors.New("MerryGo is empty")
	}

	var HorseList []*Horse
	start := m.Head
	for {
		HorseList = append(HorseList, start)
		if start.right == m.Head {
			break
		}
		start = start.right
	}

	return HorseList, nil
}

type Segment struct {
	start int
	end   int
	name  string
}

func NewSegment(start, end int, name string) *Segment {
	return &Segment{start: start, end: end, name: name}
}

func (s Segment) Info() string {
	return fmt.Sprintf("start: %d, end: %d, name: %s", s.start, s.end, s.name)
}
