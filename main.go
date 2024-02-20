package main

import (
	"log"
	"time"
)

func main() {
	config, err := NewConfigFromEnv()
	if err != nil {
		log.Fatalln("Failed to initialize config:", err)
	}

	t := config.MQTTClient.Connect()
	if t.WaitTimeout(10 * time.Second) {
		if t.Error() != nil {
			log.Fatalln("Failed to connect to MQTT:", t.Error())
		}
	} else {
		log.Fatalln("Timeout connecting to MQTT")
	}

	var subscriber *Subscriber
	if config.SubscribeTopic != "" {
		subscriber, err = NewSubscriber(config.MQTTClient, config.SubscribeTopic, *config.MDNSInterface)
		if err != nil {
			log.Fatalln("Failed to initialize subscriber:", err)
		}
	}
	if config.PublishTopic != "" {
		filter := func(id ServiceIdentity) bool {
			return subscriber == nil || !subscriber.HasService(id)
		}
		publisher := NewPublisher(config.MQTTClient, config.PublishTopic, *config.MDNSInterface, filter)
		publisher.Run("_hap._tcp", "local")
	} else if config.SubscribeTopic != "" {
		// TODO: Find a better way to block
		for {
			time.Sleep(time.Minute)
		}
	}
	log.Println("Nothing to do.")
}
