package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/grandcat/zeroconf"
)

type subscribedService struct {
	record ServiceRecord
	server *zeroconf.Server
}

type Subscriber struct {
	interfaces []net.Interface
	mutex      sync.Mutex
	services   map[ServiceIdentity]subscribedService
}

func NewSubscriber(client mqtt.Client, topic string, iface net.Interface) (*Subscriber, error) {
	s := Subscriber{
		interfaces: []net.Interface{iface},
		services:   map[ServiceIdentity]subscribedService{},
	}
	t := client.Subscribe(topic, 1, s.handleMessage)
	if t.WaitTimeout(10 * time.Second) {
		if t.Error() != nil {
			return nil, t.Error()
		}
	} else {
		return nil, fmt.Errorf("timeout subscribing to MQTT topic")
	}
	return &s, nil
}

func (s *Subscriber) HasService(identity ServiceIdentity) bool {
	s.mutex.Lock()
	_, ok := s.services[identity]
	s.mutex.Unlock()
	return ok
}

func (s *Subscriber) handleMessage(_ mqtt.Client, message mqtt.Message) {
	if len(message.Payload()) == 0 {
		s.mutex.Lock()
		for _, service := range s.services {
			s.removeService(service)
		}
		s.services = map[ServiceIdentity]subscribedService{}
		s.mutex.Unlock()
		return
	}
	m := Message{}
	err := json.Unmarshal(message.Payload(), &m)
	if err != nil {
		log.Println("Failed to deserialize message", err)
		return
	}
	seenServices := map[ServiceIdentity]interface{}{}
	for _, record := range m.Records {
		seenServices[record.ServiceIdentity] = struct{}{}

		s.mutex.Lock()
		existingService, ok := s.services[record.ServiceIdentity]
		if ok {
			if record.Equal(existingService.record) {
				s.mutex.Unlock()
				continue
			}
			existingService.server.Shutdown()
		}
		s.mutex.Unlock()

		if ok {
			log.Println("Updated remote service, announcing", record.Instance)
		} else {
			log.Println("Received remote service, announcing", record.Instance)
		}

		ips := []string{}
		for _, ip := range record.IPs {
			ips = append(ips, ip.String())
		}
		s.mutex.Lock()
		server, err := zeroconf.RegisterProxy(record.Instance, record.Service, record.Domain, record.Port, record.HostName, ips, record.TXTRecords, s.interfaces)
		if err != nil {
			log.Println("Failed to publish zone", err)
			s.mutex.Unlock()
			continue
		}
		s.services[record.ServiceIdentity] = subscribedService{record: record, server: server}
		s.mutex.Unlock()
	}
	for _, service := range s.services {
		if _, ok := seenServices[service.record.ServiceIdentity]; !ok {
			s.removeService(service)
			delete(s.services, service.record.ServiceIdentity)
		}
	}
}

func (s *Subscriber) removeService(service subscribedService) {
	log.Println("Remote service has stopped, no longer announcing", service.record.Instance)
	service.server.Shutdown()
}
