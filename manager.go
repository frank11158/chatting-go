package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	websocketUpgrader = websocket.Upgrader{
		CheckOrigin:     checkOrigin,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

type Manager struct {
	pairs  ClientPairs
	groups GroupList
	sync.RWMutex

	handlers map[string]EventHandler
}

func NewManager() *Manager {
	m := &Manager{
		pairs:    make(map[*Client]*Client),
		handlers: make(map[string]EventHandler),
		groups:   make(map[string]*Group),
	}
	m.setupEventHandlers()
	return m
}

func SendMessage(event Event, c *Client) error {
	var chatEvent SendMessageEvent

	if err := json.Unmarshal(event.Payload, &chatEvent); err != nil {
		return fmt.Errorf("bad payload in request: %v", err)
	}

	var broadMessage NewMessageEvent

	broadMessage.Message = chatEvent.Message
	broadMessage.From = chatEvent.From
	broadMessage.Sent = time.Now()

	data, err := json.Marshal(broadMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal the broad message: %v", err)
	}

	outgoingEvent := Event{
		Type:    EventNewMessage,
		Payload: data,
	}

	c.manager.pairs[c].egress <- outgoingEvent
	return nil
}

func ChangeGroup(event Event, c *Client) error {
	var changeGroupEvent ChangeGroupEvent

	if err := json.Unmarshal(event.Payload, &changeGroupEvent); err != nil {
		return fmt.Errorf("bad payload in request: %v", err)
	}

	c.manager.groups[c.group].RemoveClient(c)

	c.group = changeGroupEvent.Group
	if _, ok := c.manager.groups[c.group]; !ok {
		group := NewGroup(c.group)
		c.manager.addGroup(group)
		group.AddClient(c)
	} else {
		c.manager.groups[c.group].AddClient(c)
	}

	c.manager.pairClient(c)
	return nil
}

func (m *Manager) serveWS(w http.ResponseWriter, r *http.Request) {
	groupName := r.URL.Query().Get("group")
	if groupName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, ok := m.groups[groupName]; !ok {
		group := NewGroup(groupName)
		m.addGroup(group)
	}

	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := NewClient(conn, m, groupName)
	m.groups[client.group].AddClient(client)
	m.pairClient(client)

	go client.readMessages()
	go client.writeMessages()

	log.Println("connection established")
}

func (m *Manager) setupEventHandlers() {
	m.handlers[EventSendMessage] = SendMessage
	m.handlers[EventChangeGroup] = ChangeGroup
}

func (m *Manager) routeEvent(event Event, client *Client) error {
	if handler, ok := m.handlers[event.Type]; ok {
		if err := handler(event, client); err != nil {
			return err
		}
	}
	return errors.New("there is no such handler")
}

func (m *Manager) pairClient(client *Client) {
	group := m.groups[client.group]
	// group.Lock()
	// defer group.Unlock()

	if len(group.queue) == 0 {
		group.queue <- client
		// group.Unlock()
		<-client.egress // block the client until a partner is found
		return
	}

	// if there is a client waiting for a partner, pair the two clients
	m.Lock()
	defer m.Unlock()

	client1 := <-group.queue
	client2 := client

	m.pairs[client1] = client2
	m.pairs[client2] = client1

	client1.egress <- Event{Type: EventPartnerFound}
}

func (m *Manager) addGroup(group *Group) {
	m.Lock()
	defer m.Unlock()

	m.groups[group.name] = group
}

func (m *Manager) removeClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.pairs[client]; ok {
		client.connection.Close()
		m.pairs[client].connection.Close()
		delete(m.pairs, client)
	}
}

func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	switch origin {
	case "http://localhost:8080":
		return true
	default:
		return false
	}
}
