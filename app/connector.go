package app

import (
	"fmt"
	"time"

	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/math32"
	"go.bug.st/serial"
)

type Connector struct {
	// Serial port connection

	// Link back to display
	srm *gui.ItemScroller

	// Kalman State
	x math32.ArrayF32
	Q math32.ArrayF32
	R math32.ArrayF32

	lin_accel math32.Vector3
	orin      math32.Quaternion
	orin_e    math32.Vector3
	of_d      math32.Vector3
}

func (c *Connector) setScroller(scroller *gui.ItemScroller) {
	c.srm = scroller
}

func (c *Connector) GetPorts() []string {
	ports, err := serial.GetPortsList()
	if err != nil {
		panic(err)
	}
	if len(ports) == 0 {
		// fmt.Println("No serial ports found!")
		time.Sleep(1 * time.Second)
		return c.GetPorts()
	}
	return ports
}

func (c *Connector) ConnectPort(portname string) {
	fmt.Printf("Connecting to port %s\n", portname)

	mode := &serial.Mode{
		BaudRate: 115200,
	}
	port, err := serial.Open(portname, mode)
	if err != nil {
		panic(err)
	}

	go c.recvRoutine(port)
}

func (c *Connector) recvRoutine(port serial.Port) {
	buff := make([]byte, 2000)
	for {
		n, err := port.Read(buff)
		if err != nil {
			c.srm.Add(gui.NewImageLabel("Serial Error: " + err.Error()))
			break
		}
		if n == 0 {
			c.srm.Add(gui.NewImageLabel("EOF"))
			break
		}
		recv := fmt.Sprintf("%v", string(buff[:n]))
		c.srm.Add(gui.NewImageLabel(recv))
		c.srm.ScrollDown()
		c.portRecvCb(recv)
	}
}

func (c *Connector) portRecvCb(recv string) {
	fmt.Println("Received: " + recv)
}
