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

	lin_accel_a []math32.Vector3
	orin_e_a    []math32.Vector3
	of_d_a      []math32.Vector3

	updateGraphsFunc func()

	lin_accel_v math32.Vector3
}

var (
	qRobotProjection = rotateOnAxis(1, 0, 0, math32.Pi/2).Multiply(rotateOnAxis(0, 0, 1, math32.Pi/2))
)

func (c *Connector) setScroller(scroller *gui.ItemScroller) {
	c.srm = scroller
}

func (c *Connector) setUpdateGraphsFunc(f func()) {
	c.lin_accel_a = make([]math32.Vector3, 100)
	c.orin_e_a = make([]math32.Vector3, 100)
	c.of_d_a = make([]math32.Vector3, 100)
	c.updateGraphsFunc = f
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

	c.srm.Add(gui.NewImageLabel("Connected to port: " + portname))

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

var (
	start = time.Now()
	count = 0
)

func (c *Connector) portRecvCb(recv string) {
	if len(recv) == 0 {
		return
	}

	// defer func() {

	// }()

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in portRecvCb:", r)
		}

		count++
		elapsed := time.Since(start).Seconds()
		if elapsed >= 1 {
			fmt.Printf("Function called %d times per second\n", count)
			count = 0
			start = time.Now()
		}
	}()

	/*
		var recvLabel *gui.ImageLabel
		Label = gui.NewImageLabel(recv)

		c.srm.Add(recvLabel)
		if len(c.srm.Children()) > 10 {
			c.srm.RemoveAt(0)
		}
		c.srm.ScrollDown()
	*/

	// fmt.Println("Received: " + recv)

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(recv), &data); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	} else {
		// fmt.Println("Parsed JSON Data:", data)

		go func() {
			// wait for flag to release
			for c.rso {
				time.Sleep(1 * time.Millisecond)
			}
			c.rso = true // lock flag

			if data["motion"] != nil {
				c.of_d = math32.Vector3{
					X: c.of_d.X + float32(data["delta_x"].(float64)),
					Y: c.of_d.Y + float32(data["delta_y"].(float64)),
					Z: 0,
				}

				//todo: map thru orien quaternion into 3D space
				c.of_d.ApplyQuaternion(&c.orin)

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
					c.orin = *c.orin.MultiplyQuaternions(qRobotProjection, &c.orin)
					// Convert to Euler
					c.orin_e.SetFromQuaternion(&c.orin)
					c.orin_e.MultiplyScalar(180 / math32.Pi)
					c.orin_e.X += 90 //? not sure why this is needed
				}
				if data["linear_accel"] != nil {
					lin_accel := data["linear_accel"].(map[string]interface{})
					c.lin_accel = math32.Vector3{
						X: float32(lin_accel["x"].(float64)),
						Y: float32(lin_accel["y"].(float64)),
						Z: float32(lin_accel["z"].(float64)),
					}
					c.lin_accel.ApplyQuaternion(qRobotProjection)
					//! needs to be converted to global frame
					//c.lin_accel.ApplyQuaternion(&c.orin)
					// c.of_d.Add(&c.lin_accel) // dead reckoning
					// Integrate acceleration to update position
					dt := float32(1) / 10 // assuming a fixed time step, you may need to adjust this
					c.lin_accel_v.Add(c.lin_accel.MultiplyScalar(dt))
					c.of_d.Add(c.lin_accel_v.MultiplyScalar(dt).MultiplyScalar(10))
				}
			}

			// Ensure history buffers are initialized
			if len(c.lin_accel_a) == 0 {
				c.lin_accel_a = make([]math32.Vector3, 100)
			}
			if len(c.orin_e_a) == 0 {
				c.orin_e_a = make([]math32.Vector3, 100)
			}
			if len(c.of_d_a) == 0 {
				c.of_d_a = make([]math32.Vector3, 100)
			}

			// Shift history buffers back by one
			if len(c.lin_accel_a) > 1 {
				copy(c.lin_accel_a[1:], c.lin_accel_a[:len(c.lin_accel_a)-1])
			}
			c.lin_accel_a[0] = c.lin_accel

			if len(c.orin_e_a) > 1 {
				copy(c.orin_e_a[1:], c.orin_e_a[:len(c.orin_e_a)-1])
			}
			c.orin_e_a[0] = c.orin_e

			if len(c.of_d_a) > 1 {
				copy(c.of_d_a[1:], c.of_d_a[:len(c.of_d_a)-1])
			}
			c.of_d_a[0] = c.of_d

			// fmt.Printf("Optical Flow Delta: %+v\n", c.of_d)
			// fmt.Printf("Orientation Quaternion: %+v\n", c.orin)
			// fmt.Printf("Orientation Euler: %+v\n", c.orin_e)
			c.rso = false // release flag

			// Update graphs
			// if c.updateGraphsFunc != nil {
			// 	c.updateGraphsFunc()
			// }
		}()
	}
}
