package app

import (
	"bufio"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/math32"
	"go.bug.st/serial"
)

type Connector struct {
	// Serial port connection

	// Link back to display
	srm *gui.ItemScroller
	rso sync.Mutex

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

	// go c.recvRoutine(port)
	var buffer strings.Builder
	var mu sync.Mutex
	go func() {
		reader := bufio.NewReader(port)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				panic(err)
			}
			mu.Lock()
			buffer.WriteString(line)
			if strings.Contains(line, "\n") {
				data := buffer.String()
				c.portRecvCb(data)
				buffer.Reset()
			}
			mu.Unlock()
		}
	}()

}

func (c *Connector) portRecvCb(recv string) {
	if len(recv) == 0 {
		return
	}

	var recvLabel *gui.ImageLabel
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in portRecvCb:", r)
		}
	}()
	recvLabel = gui.NewImageLabel(recv)

	c.srm.Add(recvLabel)
	if len(c.srm.Children()) > 10 {
		c.srm.RemoveAt(0)
	}
	c.srm.ScrollDown()

	fmt.Println("Received: " + recv)
}
