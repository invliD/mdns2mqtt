package main

import (
	"fmt"
	"hash/crc32"
	"math/rand"
	"net"
	"strings"

	"github.com/Netflix/go-env"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Config struct {
	MDNSInterface  *net.Interface
	MQTTClient     mqtt.Client
	PublishTopic   string
	SubscribeTopic string
}

type environmentConfig struct {
	MQTTAddress    string `env:"MQTT_ADDRESS,required=true"`
	MQTTUsername   string `env:"MQTT_USERNAME"`
	MQTTPassword   string `env:"MQTT_PASSWORD"`
	PublishTopic   string `env:"MQTT_PUBLISH_TOPIC"`
	SubscribeTopic string `env:"MQTT_SUBSCRIBE_TOPIC"`
	MDNSInterface  string `env:"MDNS_INTERFACE,required=true"`
}

// NewConfigFromEnv creates a new config object with configuration loaded from environment variables.
func NewConfigFromEnv() (*Config, error) {
	envConfig := environmentConfig{}
	if _, err := env.UnmarshalFromEnviron(&envConfig); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var mdnsIface net.Interface
	ifaceNames := []string{}
	for _, iface := range ifaces {
		if iface.Name == envConfig.MDNSInterface {
			mdnsIface = iface
		}
		ifaceNames = append(ifaceNames, iface.Name)
	}
	if mdnsIface.Name == "" {
		return nil, fmt.Errorf("failed to find MDNS interface %s. Found interfaces %s", envConfig.MDNSInterface, strings.Join(ifaceNames, ","))
	}

	options := mqtt.NewClientOptions().AddBroker(envConfig.MQTTAddress).SetAutoReconnect(true)
	var clientIDUnique uint32
	if envConfig.PublishTopic != "" {
		clientIDUnique = crc32.ChecksumIEEE([]byte(envConfig.PublishTopic))
		options.SetBinaryWill(envConfig.PublishTopic, []byte{}, 1, true)
	} else {
		clientIDUnique = rand.Uint32()
	}
	options.SetClientID(fmt.Sprintf("mdns2mqtt-%4x", clientIDUnique))
	if envConfig.MQTTUsername != "" {
		options.SetUsername(envConfig.MQTTUsername)
	}
	if envConfig.MQTTPassword != "" {
		options.SetPassword(envConfig.MQTTPassword)
	}
	return &Config{
		MDNSInterface:  &mdnsIface,
		MQTTClient:     mqtt.NewClient(options),
		PublishTopic:   envConfig.PublishTopic,
		SubscribeTopic: envConfig.SubscribeTopic,
	}, nil
}
