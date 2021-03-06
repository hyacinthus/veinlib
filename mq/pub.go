package mq

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/streadway/amqp"
)

// Publisher a rabbitmq worker in work queues model.
type Publisher struct {
	sync.RWMutex
	Config   Config
	Exchange string
	ready    bool
}

// IsReady tall you is this publisher ready to ues.
func (p *Publisher) IsReady() bool {
	p.RLock()
	defer p.RUnlock()
	return p.ready
}

// Start a new worker
func (p *Publisher) Start(payloadchan chan []byte) {
	conn, ch, err := p.getChannel()
	if err != nil {
		log.Errorf("publisher of %s not start! %s", p.Exchange, err)
		return
	}
	p.Lock()
	p.ready = true
	p.Unlock()
	log.Infof("A new publisher on %s", p.Exchange)
	defer func() {
		ch.Close()
		conn.Close()
		log.Errorf("publisher of %s terminate!", p.Exchange)
	}()
	// 发送失败的直接报错丢弃，尝试重连。
	for payload := range payloadchan {
		err = ch.Publish(
			p.Exchange, // exchange
			"",         // routing key
			false,      // mandatory
			false,      // immediate
			amqp.Publishing{
				Timestamp:   time.Now(),
				ContentType: "text/plain",
				Body:        payload,
			})
		if err != nil {
			log.Error(err, "connection refused,reconnecting...")
			ch.Close()
			conn.Close()
			conn, ch, err = p.getChannel()
			if err != nil {
				log.Errorf("publisher of %s terminate! %s", p.Exchange, err)
				return
			}
		}
		//log.Debugf("send a payload to %s", p.Exchange)
	}
}

func (p *Publisher) getChannel() (*amqp.Connection, *amqp.Channel, error) {
	var conn *amqp.Connection
	var ch *amqp.Channel
	var err error
	for {
		p.RLock()
		log.Debug("mq url: ", p.Config.URL())
		conn, err = amqp.Dial(p.Config.URL())
		p.RUnlock()
		if err != nil {
			log.Error(err, "Retry in 2 seconds")
			time.Sleep(time.Second * 2)
			continue
		}
		log.Info("RabbitMQ connect successful.")
		// receive channel
		ch, err = conn.Channel()
		if err != nil {
			log.Errorf("Publisher on %s is terminated,%s", p.Exchange, err)
			conn.Close()
			return nil, nil, err
		}
		err = ch.ExchangeDeclare(
			p.Exchange, // name
			"fanout",   // type
			true,       // durable
			false,      // auto-deleted
			false,      // internal
			false,      // no-wait
			nil,        // arguments
		)
		if err != nil {
			log.Errorf("Publisher on %s is terminated,%s", p.Exchange, err)
			conn.Close()
			return nil, nil, err
		}
		return conn, ch, nil
	}
}
