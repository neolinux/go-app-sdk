// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package ttnsdk

import (
	"context"
	"errors"
	"testing"
	"time"

	ttnlog "github.com/TheThingsNetwork/go-utils/log"
	testlog "github.com/TheThingsNetwork/go-utils/log/test"
	"github.com/TheThingsNetwork/ttn/api/handler"
	"github.com/TheThingsNetwork/ttn/api/protocol/lorawan"
	"github.com/TheThingsNetwork/ttn/core/types"
	. "github.com/smartystreets/assertions"
)

func TestDeviceManager(t *testing.T) {
	a := New(t)

	log := testlog.NewLogger()
	ttnlog.Set(log)
	defer log.Print(t)

	mock := new(mockApplicationManagerClient)
	devMock := new(mockDevAddrManagerClient)

	manager := &deviceManager{
		logger:         log,
		client:         mock,
		devAddrClient:  devMock,
		getContext:     func(ctx context.Context) context.Context { return ctx },
		requestTimeout: time.Second,
		appID:          "test",
	}

	someErr := errors.New("some error")

	{
		mock.reset()
		mock.err = someErr
		_, err := manager.List(0, 0)
		a.So(err, ShouldNotBeNil)

		mock.reset()
		mock.deviceList = &handler.DeviceList{Devices: []*handler.Device{
			&handler.Device{
				DevId: "dev-id",
				Device: &handler.Device_LorawanDevice{LorawanDevice: &lorawan.Device{
					AppEui: &types.AppEUI{1, 2, 3, 4, 5, 6, 7, 8},
					DevEui: &types.DevEUI{1, 2, 3, 4, 5, 6, 7, 8},
				}},
			},
		}}
		sparseDevices, err := manager.List(0, 0)
		a.So(err, ShouldBeNil)
		a.So(mock.applicationIdentifier, ShouldNotBeNil)
		a.So(mock.applicationIdentifier.AppId, ShouldEqual, "test")
		devices := sparseDevices.AsDevices()
		a.So(devices, ShouldHaveLength, 1)
		a.So(devices[0].DevID, ShouldEqual, "dev-id")
		a.So(devices[0].AppEUI, ShouldEqual, types.AppEUI{1, 2, 3, 4, 5, 6, 7, 8})
		a.So(devices[0].DevEUI, ShouldEqual, types.DevEUI{1, 2, 3, 4, 5, 6, 7, 8})
	}

	{
		mock.reset()
		mock.err = someErr
		_, err := manager.Get("dev-id")
		a.So(err, ShouldNotBeNil)

		mock.reset()
		mock.device = &handler.Device{
			DevId: "dev-id",
			Device: &handler.Device_LorawanDevice{LorawanDevice: &lorawan.Device{
				AppEui:   &types.AppEUI{1, 2, 3, 4, 5, 6, 7, 8},
				DevEui:   &types.DevEUI{1, 2, 3, 4, 5, 6, 7, 8},
				FCntDown: 42,
			}},
		}
		device, err := manager.Get("dev-id")
		a.So(err, ShouldBeNil)
		a.So(mock.deviceIdentifier, ShouldNotBeNil)
		a.So(mock.deviceIdentifier.AppId, ShouldEqual, "test")
		a.So(mock.deviceIdentifier.DevId, ShouldEqual, "dev-id")
		a.So(device.DevID, ShouldEqual, "dev-id")
		a.So(device.AppEUI, ShouldEqual, types.AppEUI{1, 2, 3, 4, 5, 6, 7, 8})
		a.So(device.DevEUI, ShouldEqual, types.DevEUI{1, 2, 3, 4, 5, 6, 7, 8})
		a.So(device.FCntDown, ShouldEqual, 42)
	}

	{
		mock.reset()
		mock.err = someErr
		err := manager.Set(&Device{})
		a.So(err, ShouldNotBeNil)

		mock.reset()
		err = manager.Set(&Device{
			SparseDevice: SparseDevice{
				DevID:  "dev-id",
				AppEUI: types.AppEUI{1, 2, 3, 4, 5, 6, 7, 8},
				DevEUI: types.DevEUI{1, 2, 3, 4, 5, 6, 7, 8},
			},
			FCntDown: 42,
		})
		a.So(err, ShouldBeNil)
		a.So(mock.device, ShouldNotBeNil)
		a.So(mock.device.DevId, ShouldEqual, "dev-id")
		a.So(mock.device.GetLorawanDevice().DevId, ShouldEqual, "dev-id")
		a.So(mock.device.GetLorawanDevice().AppEui, ShouldResemble, &types.AppEUI{1, 2, 3, 4, 5, 6, 7, 8})
		a.So(mock.device.GetLorawanDevice().DevEui, ShouldResemble, &types.DevEUI{1, 2, 3, 4, 5, 6, 7, 8})
		a.So(mock.device.GetLorawanDevice().FCntDown, ShouldEqual, 42)
	}

	{
		mock.reset()
		mock.err = someErr
		err := manager.Delete("dev-id")
		a.So(err, ShouldNotBeNil)

		mock.reset()
		err = manager.Delete("dev-id")
		a.So(err, ShouldBeNil)
		a.So(mock.deviceIdentifier, ShouldNotBeNil)
		a.So(mock.deviceIdentifier.AppId, ShouldEqual, "test")
		a.So(mock.deviceIdentifier.DevId, ShouldEqual, "dev-id")
	}

	{
		dev := new(Device)
		// Can't call these funcs on a new device
		a.So(func() { dev.Update() }, ShouldPanic)
		a.So(func() { dev.Personalize(types.NwkSKey{}, types.AppSKey{}) }, ShouldPanic)
		a.So(func() { dev.Delete() }, ShouldPanic)
	}

	{
		mock.reset()
		mock.device = &handler.Device{
			DevId: "dev-id",
			Device: &handler.Device_LorawanDevice{LorawanDevice: &lorawan.Device{
				AppEui:   &types.AppEUI{1, 2, 3, 4, 5, 6, 7, 8},
				DevEui:   &types.DevEUI{1, 2, 3, 4, 5, 6, 7, 8},
				FCntDown: 42,
			}},
		}
		device, err := manager.Get("dev-id")
		a.So(err, ShouldBeNil)

		device.FCntDown = 0
		err = device.Update()
		a.So(err, ShouldBeNil)
		a.So(mock.device.GetLorawanDevice().FCntDown, ShouldEqual, 0)

		mock.reset()
		devMock.reset()
		devMock.err = someErr
		err = device.Personalize(types.NwkSKey{}, types.AppSKey{})
		a.So(err, ShouldNotBeNil)

		mock.reset()
		devMock.reset()
		devMock.devAddrResponse = &lorawan.DevAddrResponse{DevAddr: &types.DevAddr{1, 2, 3, 4}}
		err = device.Personalize(types.NwkSKey{}, types.AppSKey{})
		a.So(err, ShouldBeNil)
		a.So(mock.device.GetLorawanDevice().DevAddr, ShouldResemble, &types.DevAddr{1, 2, 3, 4})

		mock.reset()
		err = device.Delete()
		a.So(err, ShouldBeNil)
		a.So(mock.deviceIdentifier.DevId, ShouldEqual, "dev-id")
	}

}
