package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/math32"
	"go.bug.st/serial"
)

func rotateOnAxis(xx, yy, zz int, a float32) *math32.Quaternion {
	factor := math32.Sin(a / 2)

	x := float32(xx) * factor
	y := float32(yy) * factor
	z := float32(zz) * factor

	w := math32.Cos(a / 2)

	return &math32.Quaternion{X: x, Y: y, Z: z, W: w}
}

type Connector struct {
	// Serial port connection

	// Link back to display
	srm *gui.ItemScroller
	rso bool

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

	// fmt.Println("Received: " + recv)

	go func() {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(recv), &data); err != nil {
			fmt.Println("Error parsing JSON:", err)
			return
		}
		// fmt.Println("Parsed JSON Data:", data)
		c.rso = true
		if data["motion"] != nil {
			c.of_d = math32.Vector3{
				X: c.of_d.X + float32(data["delta_x"].(float64)),
				Z: c.of_d.Y + float32(data["delta_y"].(float64)),
				Y: 0,
			}
		} else {
			if data["quat9"] != nil {
				quat9 := data["quat9"].(map[string]interface{})
				c.orin = math32.Quaternion{
					X: float32(quat9["y"].(float64)),
					Y: float32(quat9["x"].(float64)),
					Z: -float32(quat9["z"].(float64)),
					W: float32(quat9["w"].(float64)),
				}
				// Flip Z, Rotate by 90 on X axis, Rotate by 90 on Z axis
				rotation := rotateOnAxis(1, 0, 0, math32.Pi/2).Multiply(rotateOnAxis(0, 0, 1, math32.Pi/2))
				c.orin = *c.orin.MultiplyQuaternions(rotation, &c.orin)
				c.orin_e.SetFromQuaternion(&c.orin)
				c.orin_e.MultiplyScalar(180 / math32.Pi)
			}
		}
		fmt.Printf("Optical Flow Delta: %+v\n", c.of_d)
		fmt.Printf("Orientation Quaternion: %+v\n", c.orin)
		fmt.Printf("Orientation Euler: %+v\n", c.orin_e)
		c.rso = false
	}()
}
