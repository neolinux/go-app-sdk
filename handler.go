// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package ttnsdk

import (
	"github.com/TheThingsNetwork/go-utils/log"
)

func (c *client) connectHandler() (err error) {
	c.handler.Lock()
	defer c.handler.Unlock()
	if c.handler.conn != nil {
		return nil
	}
	if c.handler.announcement == nil {
		if err := c.discover(); err != nil {
			return err
		}
	}
	logger := c.Logger.WithFields(log.Fields{
		"handler-id": c.handler.announcement.Id,
		"address":    c.handler.announcement.NetAddress,
	})
	logger.Debug("ttn-sdk: connecting to handler")
	c.handler.conn, err = c.handler.announcement.Dial(c.connPool)
	if err != nil {
		logger.WithError(err).Debug("ttn-sdk: could not connect to handler")
		return err
	}
	logger.Debug("ttn-sdk: connected to handler")
	return nil
}

func (c *client) closeHandler() error {
	c.handler.Lock()
	defer c.handler.Unlock()
	if c.handler.conn != nil {
		c.connPool.CloseConn(c.handler.conn)
	}
	c.handler.conn = nil
	return nil
}
