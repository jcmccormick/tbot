package main

type Convos struct {
	Chatters map[string]Convo
}

type Convo struct {
	Messages []Message
}

const previous = 6

func (co *Convos) AddUser(u string) {
	if co.Chatters[u].Messages == nil {
		co.Chatters[u] = Convo{
			Messages: []Message{},
		}
	}
}

func (co *Convos) AddMessage(u string, m Message) {
	msgs := co.Chatters[u].Messages
	if len(msgs) >= previous {
		copy(msgs, msgs[len(msgs)-previous+1:])
		msgs = msgs[:previous-1]
	}
	co.Chatters[u] = Convo{Messages: append(msgs, m)}
}
