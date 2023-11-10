package main

import "sync"

type GroupList map[string]*Group

type Group struct {
	name    string
	queue   chan *Client
	clients map[*Client]bool

	sync.RWMutex
}

func NewGroup(name string) *Group {
	return &Group{
		name:    name,
		queue:   make(chan *Client, 2),
		clients: make(map[*Client]bool),
	}
}

func (g *Group) AddClient(c *Client) {
	g.Lock()
	defer g.Unlock()

	g.clients[c] = true
}

func (g *Group) RemoveClient(c *Client) {
	g.Lock()
	defer g.Unlock()

	if _, ok := g.clients[c]; !ok {
		return
	}
	delete(g.clients, c)
}
