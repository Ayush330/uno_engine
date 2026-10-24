package domain

import (
	"errors"
	"sync"
)

var (
	ErrRoomAlreadyExists = errors.New("domain: Room Already Exists")
	ErrRoomNotFound      = errors.New("domain: Room Not Found")
)

type RoomManager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room),
	}
}

func (rm *RoomManager) AddRoom(hostName string, hostId int) (*Room, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	room := CreateNewRoom(hostName, hostId)
	// check if room exists
	if _, ok := rm.rooms[room.id]; ok {
		return nil, ErrRoomAlreadyExists
	}
	rm.rooms[room.id] = room
	return room, nil
}

func (rm *RoomManager) GetRoom(roomId string) (*Room, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	room, ok := rm.rooms[roomId]
	if !ok {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

func (rm *RoomManager) RemoveRoom(roomId string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if _, ok := rm.rooms[roomId]; !ok {
		return ErrRoomNotFound
	}
	delete(rm.rooms, roomId)
	return nil
}
