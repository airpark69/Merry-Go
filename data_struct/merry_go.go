package data_struct

import (
	"errors"
)

type Rider interface {
	Info() (int, int, int)
	Update(start int, end int)
}

type Horse struct {
	Rider Rider
	Right *Horse
	Left  *Horse
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
Append 해당 함수는 MerryGo에 Horse와 Rider를 추가합니다. 더 이상 MerryGo에 자리가 없을 경우 에러가 납니다

Rider Rider: MerryGo에 들어갈 데이터
*/
func (m *MerryGo) Append(rider Rider) error {
	if m.IsFull() {
		return errors.New("MerryGo is full")
	}

	newHorse := &Horse{Rider: rider}
	if m.Head == nil && m.Tail == nil {
		m.Head = newHorse
		m.Tail = newHorse
		m.Head.Left = newHorse
		m.Head.Right = newHorse
		m.Tail.Left = newHorse
		m.Tail.Right = newHorse
		m.Count++
		return nil
	}

	newHorse.Right = m.Head
	newHorse.Left = m.Tail
	m.Tail.Right = newHorse
	m.Tail = newHorse
	m.Head.Left = m.Tail
	m.Count++

	return nil
}

/*
PopTail 해당 함수는 MerryGo의 Tail에 해당하는 Horse를 빼냅니다.
*/
func (m *MerryGo) PopTail() (horse *Horse, err error) {
	var popHorse *Horse

	if m.IsEmpty() {
		return popHorse, errors.New("MerryGo is empty")
	}

	popHorse = m.Tail

	m.Tail.Left.Right = m.Head
	m.Tail = m.Tail.Left
	m.Head.Left = m.Tail

	m.Count--

	return popHorse, nil
}

/*
PopHead 해당 함수는 MerryGo의 Head에 해당하는 Horse를 빼냅니다.
*/
func (m *MerryGo) PopHead() (horse *Horse, err error) {
	var popHorse *Horse

	if m.IsEmpty() {
		return popHorse, errors.New("MerryGo is empty")
	}

	popHorse = m.Head

	m.Head.Right.Left = m.Tail
	m.Head = m.Head.Right
	m.Tail.Right = m.Head

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
Rotate MerryGo 내의 모든 Rider의 위치를 left로 이동시킵니다.
*/
func (m *MerryGo) Rotate() error {
	if m.IsEmpty() {
		return errors.New("MerryGo is empty")
	}

	m.Head = m.Head.Right
	m.Tail = m.Head
	return nil
}

/*
Display MerryGo 내의 모든 데이터를 Head -> Tail 순으로 순환하여 리턴합니다.
*/
func (m *MerryGo) Display() ([]Rider, error) {
	if m.IsEmpty() {
		return nil, errors.New("MerryGo is empty")
	}

	RiderList := make([]Rider, m.Count)
	start := m.Head
	for i := 0; i < m.Count; i++ {
		RiderList[i] = start.Rider
		if start.Right == m.Head {
			break
		}
		start = start.Right
	}

	return RiderList, nil
}

type Segment struct {
	Start  int
	End    int
	Length int
}

func (s *Segment) Info() (int, int, int) {
	return s.Start, s.End, s.Length
}

func (s *Segment) Update(updateStart int, updateEnd int) {
	s.Start = updateStart
	s.End = updateEnd
}
