package data_struct

import (
	"errors"
	"fmt"
)

type Rider interface {
	Info() string
}

type horse struct {
	rider Rider
	right *horse
	left  *horse
}

type MerryGo struct {
	horses     []horse
	aroundSize int
	riderCount int
}

func NewMerryGo(size int) *MerryGo {
	rawHorses := make([]horse, size)
	for i, h := range rawHorses {
		if i == 0 {
			h.left = &rawHorses[len(rawHorses)-1]
			h.right = &rawHorses[i+1]
		} else if i == len(rawHorses)-1 {
			h.left = &rawHorses[i-1]
			h.right = &rawHorses[0]
		} else {
			h.left = &rawHorses[i-1]
			h.right = &rawHorses[i+1]
		}
	}
	return &MerryGo{horses: rawHorses, aroundSize: size, riderCount: 0}
}

func (m *MerryGo) appendRider(rider Rider) error {
	if m.IsFull() {
		return errors.New("MerryGo is full")
	}
	m.horses[m.riderCount].rider = rider
	m.riderCount++
	return nil
}

func (m *MerryGo) popRider(index ...int) (Rider, error) {
	var defaultIndex = 0
	var popRider Rider
	if len(index) > 0 {
		defaultIndex = index[0]
	}

	if m.IsEmpty() {
		return popRider, errors.New("MerryGo is empty")
	}

	popRider = m.horses[defaultIndex].rider

	for {
		if defaultIndex == m.aroundSize {
			break
		}
		m.horses[defaultIndex].rider = m.horses[defaultIndex].next.rider
		m.horses[defaultIndex].next.rider = nil
		defaultIndex++
	}

	m.riderCount--
	return popRider, nil
}

func (m *MerryGo) IsFull() bool {
	return m.aroundSize == m.riderCount
}

func (m *MerryGo) IsEmpty() bool {
	return m.riderCount == 0
}

func (m *MerryGo) rotate() error {
	if m.IsEmpty() {
		return errors.New("MerryGo is empty")
	}

	var tmpRider Rider
	for i, h := range m.horses {
		tmpRider = h.rider
		h.next.rider = h.rider
	}

	return nil
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
