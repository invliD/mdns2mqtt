package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/grandcat/zeroconf"
)

type publishedService struct {
	record   ServiceRecord
	lastSeen time.Time
}

type Publisher struct {
	client   mqtt.Client
	filter   func(ServiceIdentity) bool
	iface    net.Interface
	mutex    sync.Mutex
	services map[ServiceIdentity]publishedService
	topic    string
}

func NewPublisher(client mqtt.Client, topic string, iface net.Interface, filter func(ServiceIdentity) bool) *Publisher {
	publisher := Publisher{
		client:   client,
		filter:   filter,
		iface:    iface,
		services: map[ServiceIdentity]publishedService{},
		topic:    topic,
	}
	go func() {
		for {
			time.Sleep(time.Minute)
			publisher.removeOldEntries(time.Now().Add(-3 * time.Minute))
		}
	}()
	return &publisher
}

func (p *Publisher) Run(service string, domain string) {
	for {
		err := p.resolveServices(context.TODO(), service, domain)
		if err != nil {
			log.Fatalln("Failed to listen for services:", err)
		}
	}
}

func (p *Publisher) publish() error {
	allRecords := []ServiceRecord{}
	for _, service := range p.services {
		allRecords = append(allRecords, service.record)
	}

	b, err := json.Marshal(Message{Records: allRecords})
	if err != nil {
		return err
	}
	p.client.Publish(p.topic, 1, true, b)
	return nil
}

func (p *Publisher) resolveServices(ctx context.Context, service string, domain string) error {
	resolver, err := zeroconf.NewResolver(zeroconf.SelectIfaces([]net.Interface{p.iface}))
	if err != nil {
		return err
	}
	entriesCh := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entriesCh {
			ips := []net.IP{}
			ips = append(ips, entry.AddrIPv4...)
			ips = append(ips, entry.AddrIPv6...)

			id := ServiceIdentity{
				Instance: entry.Instance,
				Service:  entry.Service,
				Domain:   entry.Domain,
			}
			if !p.filter(id) {
				continue
			}
			record := ServiceRecord{
				ServiceIdentity: id,
				HostName:        entry.HostName,
				Port:            entry.Port,
				IPs:             ips,
				TXTRecords:      entry.Text,
			}
			p.mutex.Lock()
			existingService, ok := p.services[id]
			p.services[id] = publishedService{record: record, lastSeen: time.Now()}
			if ok && existingService.record.Equal(record) {
				p.mutex.Unlock()
				continue
			}
			log.Println("Found new local service, publishing", record.Instance)
			p.publish()
			p.mutex.Unlock()
		}
	}()
	browseCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err = resolver.Browse(browseCtx, service, domain, entriesCh)
	if err != nil {
		return err
	}
	time.Sleep(30 * time.Second)
	return nil
}

func (p *Publisher) removeOldEntries(expiry time.Time) {
	anyDeleted := false
	p.mutex.Lock()
	for key, value := range p.services {
		if value.lastSeen.Before(expiry) {
			anyDeleted = true
			log.Println("Local service expired, removing", value.record.Instance)
			delete(p.services, key)
		}
	}
	defer p.mutex.Unlock()
	if anyDeleted {
		p.publish()
	}
}
