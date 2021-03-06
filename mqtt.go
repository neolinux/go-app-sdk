// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package ttnsdk

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/ttn/core/types"
	"github.com/TheThingsNetwork/ttn/mqtt"
)

func (c *client) connectMQTT() (err error) {
	c.mqtt.Lock()
	defer c.mqtt.Unlock()
	if c.mqtt.client != nil {
		return nil
	}
	c.handler.RLock()
	defer c.handler.RUnlock()
	if c.handler.announcement == nil {
		if err := c.discover(); err != nil {
			return err
		}
	}
	if c.handler.announcement.MqttAddress == "" {
		c.Logger.WithField("handler-id", c.handler.announcement.Id).Debug("ttn-sdk: handler does not announce mqtt address")
		return errors.New("ttn-sdk: handler does not announce mqtt address")
	}
	mqttAddress, err := cleanMQTTAddress(c.handler.announcement.MqttAddress)
	if err != nil {
		return err
	}
	if strings.HasPrefix(mqttAddress, "ssl://") {
		c.mqtt.client = mqtt.NewTLSClient(c.Logger, c.ClientName, c.appID, c.appAccessKey, c.TLSConfig, mqttAddress)
	} else {
		c.mqtt.client = mqtt.NewClient(c.Logger, c.ClientName, c.appID, c.appAccessKey, mqttAddress)
	}
	c.mqtt.ctx, c.mqtt.cancel = context.WithCancel(context.Background())
	logger := c.Logger.WithField("address", mqttAddress)
	logger.Debug("ttn-sdk: connecting to mqtt")
	if err := c.mqtt.client.Connect(); err != nil {
		logger.WithError(err).Debug("ttn-sdk: could not connect to mqtt")
		return err
	}
	logger.Debug("ttn-sdk: connected to mqtt")
	return nil
}

func (c *client) closeMQTT() error {
	c.mqtt.Lock()
	defer c.mqtt.Unlock()
	if c.mqtt.client == nil {
		return nil
	}
	c.Logger.Debug("ttn-sdk: disconnecting from mqtt")
	c.mqtt.cancel()
	c.mqtt.client.Disconnect()
	c.mqtt.client = nil
	return nil
}

var mqttBufferSize = 10

// DevicePub interface for publishing downlink messages to the device
type DevicePub interface {
	Publish(*types.DownlinkMessage) error
}

// DeviceSub interface for subscribing to uplink messages and events from the device
type DeviceSub interface {
	SubscribeUplink() (<-chan *types.UplinkMessage, error)
	UnsubscribeUplink() error
	SubscribeEvents() (<-chan *types.DeviceEvent, error)
	UnsubscribeEvents() error
	SubscribeActivations() (<-chan *types.Activation, error)
	UnsubscribeActivations() error
	Close()
}

// DevicePubSub combines the DevicePub and DeviceSub interfaces
type DevicePubSub interface {
	DevicePub
	DeviceSub
}

type devicePubSub struct {
	logger log.Interface
	client mqtt.Client
	ctx    context.Context
	cancel context.CancelFunc

	appID string
	devID string

	sync.RWMutex
	uplink      chan *types.UplinkMessage
	events      chan *types.DeviceEvent
	activations chan *types.Activation
}

func (d *devicePubSub) Publish(downlink *types.DownlinkMessage) error {
	msg := *downlink
	msg.AppID = d.appID
	msg.DevID = d.devID
	token := d.client.PublishDownlink(msg)
	token.Wait()
	return token.Error()
}

func (d *devicePubSub) SubscribeUplink() (<-chan *types.UplinkMessage, error) {
	if err := d.ctx.Err(); err != nil {
		return nil, err
	}
	d.Lock()
	defer d.Unlock()
	if d.uplink != nil {
		return d.uplink, nil
	}
	d.uplink = make(chan *types.UplinkMessage, mqttBufferSize)
	token := d.client.SubscribeDeviceUplink(d.appID, d.devID, func(_ mqtt.Client, appID string, devID string, msg types.UplinkMessage) {
		msg.AppID = appID
		msg.DevID = devID
		d.RLock()
		defer d.RUnlock()
		if d.uplink == nil {
			return
		}
		select {
		case d.uplink <- &msg:
		default:
		}
	})
	token.Wait()
	err := token.Error()
	if err != nil {
		close(d.uplink)
		d.uplink = nil
	}
	return d.uplink, err
}

func (d *devicePubSub) UnsubscribeUplink() error {
	d.Lock()
	defer d.Unlock()
	if d.uplink == nil {
		return nil
	}
	token := d.client.UnsubscribeDeviceUplink(d.appID, d.devID)
	token.Wait()
	close(d.uplink)
	d.uplink = nil
	return token.Error()
}

func (d *devicePubSub) SubscribeEvents() (<-chan *types.DeviceEvent, error) {
	if err := d.ctx.Err(); err != nil {
		return nil, err
	}
	d.Lock()
	defer d.Unlock()
	if d.events != nil {
		return d.events, nil
	}
	d.events = make(chan *types.DeviceEvent, mqttBufferSize)
	token := d.client.SubscribeDeviceEvents(d.appID, d.devID, "#", func(_ mqtt.Client, appID string, devID string, eventType types.EventType, payload []byte) {
		msg := types.DeviceEvent{
			AppID: appID,
			DevID: devID,
			Event: eventType,
		}
		eventData := eventType.Data()
		if eventData != nil {
			if err := json.Unmarshal(payload, eventData); err == nil {
				msg.Data = eventData
			}
		}
		d.RLock()
		defer d.RUnlock()
		if d.events == nil {
			return
		}
		select {
		case d.events <- &msg:
		default:
		}
	})
	token.Wait()
	err := token.Error()
	if err != nil {
		close(d.events)
		d.events = nil
	}
	return d.events, err
}

func (d *devicePubSub) UnsubscribeEvents() error {
	d.Lock()
	defer d.Unlock()
	if d.events == nil {
		return nil
	}
	token := d.client.UnsubscribeDeviceEvents(d.appID, d.devID, "#")
	token.Wait()
	close(d.events)
	d.events = nil
	return token.Error()
}

func (d *devicePubSub) SubscribeActivations() (<-chan *types.Activation, error) {
	if err := d.ctx.Err(); err != nil {
		return nil, err
	}
	d.Lock()
	defer d.Unlock()
	if d.activations != nil {
		return d.activations, nil
	}
	d.activations = make(chan *types.Activation, mqttBufferSize)
	token := d.client.SubscribeDeviceActivations(d.appID, d.devID, func(_ mqtt.Client, appID string, devID string, mqg types.Activation) {
		mqg.AppID = appID
		mqg.DevID = devID
		d.RLock()
		defer d.RUnlock()
		if d.activations == nil {
			return
		}
		select {
		case d.activations <- &mqg:
		default:
		}
	})
	token.Wait()
	err := token.Error()
	if err != nil {
		close(d.activations)
		d.activations = nil
	}
	return d.activations, err
}

func (d *devicePubSub) UnsubscribeActivations() error {
	d.Lock()
	defer d.Unlock()
	if d.activations == nil {
		return nil
	}
	token := d.client.UnsubscribeDeviceActivations(d.appID, d.devID)
	token.Wait()
	close(d.activations)
	d.activations = nil
	return token.Error()
}

func (d *devicePubSub) Close() {
	d.cancel()
}

// ApplicationPubSub interface for publishing and subscribing to devices in an application
type ApplicationPubSub interface {
	Device(devID string) DevicePubSub
	AllDevices() DeviceSub
	Close()
}

type applicationPubSub struct {
	logger log.Interface
	client mqtt.Client
	ctx    context.Context
	cancel context.CancelFunc

	appID string
}

func (a *applicationPubSub) Device(devID string) DevicePubSub {
	d := &devicePubSub{
		logger: a.logger,
		client: a.client,
		appID:  a.appID,
		devID:  devID,
	}
	d.ctx, d.cancel = context.WithCancel(a.ctx)
	go func() {
		<-d.ctx.Done()
		d.UnsubscribeUplink()
		d.UnsubscribeEvents()
		d.UnsubscribeActivations()
	}()
	return d
}

func (a *applicationPubSub) AllDevices() DeviceSub {
	return a.Device("+")
}

func (a *applicationPubSub) Close() {
	a.cancel()
}

func (c *client) PubSub() (ApplicationPubSub, error) {
	if err := c.connectMQTT(); err != nil {
		return nil, err
	}
	if err := c.mqtt.ctx.Err(); err != nil {
		return nil, err
	}
	a := &applicationPubSub{
		logger: c.Logger,
		client: c.mqtt.client,
		appID:  c.appID,
	}
	a.ctx, a.cancel = context.WithCancel(c.mqtt.ctx)
	return a, nil
}
