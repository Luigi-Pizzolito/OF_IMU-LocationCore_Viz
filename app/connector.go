package app

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
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
	x     math32.ArrayF32
	x_pos math32.Vector3
	P     math32.ArrayF32
	f     math32.ArrayF32
	K     math32.ArrayF32
	yh    math32.ArrayF32

	predict_cpu float32
	update_cpu  float32

	lin_accel math32.Vector3
	orin      math32.Quaternion
	orin_e    math32.Vector3
	of_d      math32.Vector3

	x_pos_a     []math32.Vector3
	lin_accel_a []math32.Vector3
	orin_e_a    []math32.Vector3
	of_d_a      []math32.Vector3

	historySize      int
	posS             int
	updateGraphsFunc func()

	logFile   *os.File
	logWriter *csv.Writer
}

var (
	qRobotProjection = rotateOnAxis(1, 0, 0, math32.Pi/2).Multiply(rotateOnAxis(0, 0, 1, math32.Pi/2))
)

func (c *Connector) setScroller(scroller *gui.ItemScroller) {
	c.srm = scroller
}

func (c *Connector) setUpdateGraphsFunc(f func(), hist int, pS int) {
	c.historySize = hist
	c.posS = pS
	c.lin_accel_a = make([]math32.Vector3, c.historySize)
	c.orin_e_a = make([]math32.Vector3, c.historySize)
	c.of_d_a = make([]math32.Vector3, c.historySize)
	c.x_pos_a = make([]math32.Vector3, c.historySize)

	c.x = make(math32.ArrayF32, 6)
	c.P = make(math32.ArrayF32, 6*6)
	c.f = make(math32.ArrayF32, 6)
	c.K = make(math32.ArrayF32, 3*6)
	c.yh = make(math32.ArrayF32, 3)

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

func (c *Connector) StartNewLog() {
	if c.logFile != nil {
		c.logFile.Close()
	}
	if c.logWriter != nil {
		c.logWriter.Flush()
		c.logWriter = nil
	}
	// Generate log filename as log_DDMMYY_HHMMSS.csv in folder log
	t := time.Now()
	logFileName := fmt.Sprintf("log_%02d%02d%02d_%02d%02d%02d.csv", t.Day(), t.Month(), t.Year()%100, t.Hour(), t.Minute(), t.Second())
	if _, err := os.Stat("log"); os.IsNotExist(err) {
		os.Mkdir("log", os.ModePerm)
	}
	logFileName = "log/" + logFileName
	fmt.Println("Log file: " + logFileName)
	// Clear all files in log folder
	files, err := os.ReadDir("log")
	if err != nil {
		panic("Error reading log folder: " + err.Error())
	}
	for _, file := range files {
		if file.Name() != logFileName {
			err := os.Remove("log/" + file.Name())
			if err != nil {
				panic("Error removing file: " + file.Name() + " with error: " + err.Error())
			}
		}
	}
	// Create log file
	c.logFile, err = os.Create(logFileName)
	if err != nil {
		panic("Error opening file: " + logFileName + " with error: " + err.Error())
	}

	// Create writer
	c.logWriter = csv.NewWriter(c.logFile)

	// write header
	c.WriteHeader()
}

func (c *Connector) WriteHeader() {
	// Prepare header
	header := []string{
		"t",
		"predict_cpu",
		"update_cpu",
		"quat_x",
		"quat_y",
		"quat_z",
		"quat_w",
		"accel_x",
		"accel_y",
		"accel_z",
		"of_x",
		"of_y",
		"of_z",
		"x_x",
		"x_y",
		"x_z",
		"x_vx",
		"x_vy",
		"x_vz",
		"dt"}
	for i := range 6 * 6 {
		header = append(header, fmt.Sprintf("P_%d", i))
	}
	// Write header
	if err := c.logWriter.Write(header); err != nil {
		panic("Error writing header to log file: " + err.Error())
	}
	// Flush immediately
	c.logWriter.Flush()
	if err := c.logFile.Sync(); err != nil {
		panic("Error syncing logfile: " + err.Error())
	}
}

func (c *Connector) WriteLog(data map[string]interface{}) {
	// Write header
	if c.logWriter == nil {
		// c.StartNewLog()
		return
	}

	// Extract data
	var micros float64
	var quat_x, quat_y, quat_z, quat_w, accel_x, accel_y, accel_z, of_x, of_y, of_z float32
	var x_x, x_y, x_z, x_vx, x_vy, x_vz, dt float32
	var predict_cpu, update_cpu float32
	P := make([]float32, 6*6)
	if data["sensor_input"] != nil {
		sensor_input := data["sensor_input"].(map[string]interface{})
		if sensor_input["quat"] != nil {
			quat := sensor_input["quat"].(map[string]interface{})
			quat_x = float32(quat["x"].(float64))
			quat_y = float32(quat["y"].(float64))
			quat_z = float32(quat["z"].(float64))
			quat_w = float32(quat["w"].(float64))
		}
		if sensor_input["accel"] != nil {
			accel := sensor_input["accel"].(map[string]interface{})
			accel_x = float32(accel["x"].(float64))
			accel_y = float32(accel["y"].(float64))
			accel_z = float32(accel["z"].(float64))
		} else {
			accel_x = -math.MaxFloat32
			accel_y = -math.MaxFloat32
			accel_z = -math.MaxFloat32
		}
		if sensor_input["of"] != nil {
			of := sensor_input["of"].(map[string]interface{})
			of_x = float32(of["x"].(float64))
			of_y = float32(of["y"].(float64))
			of_z = float32(of["z"].(float64))
		} else {
			of_x = -math.MaxFloat32
			of_y = -math.MaxFloat32
			of_z = -math.MaxFloat32
		}
	}
	if data["state"] != nil {
		state := data["state"].(map[string]interface{})
		x_x = float32(state["x"].(float64))
		x_y = float32(state["y"].(float64))
		x_z = float32(state["z"].(float64))
		x_vx = float32(state["vx"].(float64))
		x_vy = float32(state["vy"].(float64))
		x_vz = float32(state["vz"].(float64))
		dt = float32(state["dt"].(float64))
		micros = data["micros"].(float64) / 1e6

		if data["f"] != nil {
			// We are in predict step
			predict_cpu = dt / (1.0 / 50) // 50 Hz predict
			// }
		} else if data["y-h"] != nil {
			// We are in update step
			update_cpu = dt / (1.0 / 10) // 10 Hz update
		}
	}
	if data["P"] != nil {
		for i := 0; i < 6*6; i++ {
			P[i] = float32(data["P"].([]interface{})[i].(float64))
		}
	} else {
		for i := 0; i < 6*6; i++ {
			P[i] = -math.MaxFloat32
		}
	}

	// Write data
	var row []string
	row = append(row, fmt.Sprintf("%3.7f", micros))
	if data["f"] != nil {
		row = append(row, fmt.Sprintf("%3.7f", predict_cpu), "")
	} else {
		row = append(row, "", fmt.Sprintf("%3.7f", update_cpu))
	}
	row = append(row,
		fmt.Sprintf("%3.7f", quat_x),
		fmt.Sprintf("%3.7f", quat_y),
		fmt.Sprintf("%3.7f", quat_z),
		fmt.Sprintf("%3.7f", quat_w))
	if accel_x != -math.MaxFloat32 {
		row = append(row,
			fmt.Sprintf("%3.7f", accel_x),
			fmt.Sprintf("%3.7f", accel_y),
			fmt.Sprintf("%3.7f", accel_z))
	} else {
		row = append(row, "", "", "")
	}
	if of_x != -math.MaxFloat32 {
		row = append(row,
			fmt.Sprintf("%3.7f", of_x),
			fmt.Sprintf("%3.7f", of_y),
			fmt.Sprintf("%3.7f", of_z))
	} else {
		row = append(row, "", "", "")
	}
	row = append(row,
		fmt.Sprintf("%3.7f", x_x),
		fmt.Sprintf("%3.7f", x_y),
		fmt.Sprintf("%3.7f", x_z),
		fmt.Sprintf("%3.7f", x_vx),
		fmt.Sprintf("%3.7f", x_vy),
		fmt.Sprintf("%3.7f", x_vz),
		fmt.Sprintf("%3.7f", dt))
	for i := range 6 * 6 {
		if P[i] != -math.MaxFloat32 {
			row = append(row, fmt.Sprintf("%3.7f", P[i]))
		} else {
			row = append(row, "")
		}
	}

	if err := c.logWriter.Write(row); err != nil {
		fmt.Println("Error writing to log file:", err)
	}
	// Flush immediately
	c.logWriter.Flush()
	if err := c.logFile.Sync(); err != nil {
		panic("Error syncing logfile: " + err.Error())
	}
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

		// todo: add logging routine
		go c.WriteLog(data)

		go func() {
			// wait for flag to release
			for c.rso {
				time.Sleep(1 * time.Millisecond)
			}
			c.rso = true // lock flag

			/*
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
							// dt := float32(1) / 10 // assuming a fixed time step, you may need to adjust this
							// c.lin_accel_v.Add(c.lin_accel.MultiplyScalar(dt))
							// c.of_d.Add(c.lin_accel_v.MultiplyScalar(dt).MultiplyScalar(10))
					}
				}
			*/

			if data["sensor_input"] != nil {
				sensor_input := data["sensor_input"].(map[string]interface{})
				if sensor_input["quat"] != nil {
					quat := sensor_input["quat"].(map[string]interface{})
					c.orin = math32.Quaternion{
						X: float32(quat["x"].(float64)),
						Y: float32(quat["y"].(float64)),
						Z: float32(quat["z"].(float64)),
						W: float32(quat["w"].(float64)),
					}
					// Flip Z, Rotate by 90 on X axis, Rotate by 90 on Z axis
					c.orin = *c.orin.MultiplyQuaternions(qRobotProjection, &c.orin)
					// Convert to Euler
					c.orin_e.SetFromQuaternion(&c.orin)
					c.orin_e.MultiplyScalar(180 / math32.Pi)
					c.orin_e.X += 90 //? not sure why this is needed
				}
				if sensor_input["accel"] != nil {
					accel := sensor_input["accel"].(map[string]interface{})
					c.lin_accel = math32.Vector3{
						X: float32(accel["x"].(float64)),
						Y: float32(accel["y"].(float64)),
						Z: float32(accel["z"].(float64)),
					}
					c.lin_accel.ApplyQuaternion(qRobotProjection)
				}
				if sensor_input["of"] != nil {
					of := sensor_input["of"].(map[string]interface{})
					c.of_d = math32.Vector3{
						X: float32(of["x"].(float64)),
						Y: float32(of["y"].(float64)),
						Z: float32(of["z"].(float64)),
					}
					c.of_d.ApplyQuaternion(qRobotProjection)
				}
			}
			if data["state"] != nil {
				state := data["state"].(map[string]interface{})

				c.x[0] = float32(state["x"].(float64))
				c.x[1] = float32(state["y"].(float64))
				c.x[2] = float32(state["z"].(float64))
				c.x[3] = float32(state["vx"].(float64))
				c.x[4] = float32(state["vy"].(float64))
				c.x[5] = float32(state["vz"].(float64))

				c.x_pos = math32.Vector3{
					X: float32(state["x"].(float64)),
					Y: -float32(state["y"].(float64)),
					Z: -float32(state["z"].(float64)),
				}
				c.x_pos.ApplyQuaternion(qRobotProjection)
				c.x_pos.MultiplyScalar(float32(c.posS))

				if data["f"] != nil {
					// We are in predict step
					// if state["dt"] != nil {
					dt := float32(state["dt"].(float64))
					c.predict_cpu = dt / (1.0 / 50) // 50 Hz predict
					// }
				} else if data["y-h"] != nil {
					// We are in update step
					// if state["dt"] != nil {
					dt := float32(state["dt"].(float64))
					c.update_cpu = dt / (1.0 / 10) // 10 Hz update
					// }
				}
				fmt.Printf("predict_cpu: %.2f, update_cpu: %.2f\n", c.predict_cpu, c.update_cpu)

				// //! temp for testing
				// c.of_d = math32.Vector3{
				// 	X: float32(state["x"].(float64)),
				// 	Y: -float32(state["y"].(float64)),
				// 	Z: -float32(state["z"].(float64)),
				// }
				// c.of_d.ApplyQuaternion(qRobotProjection)
				// c.of_d.MultiplyScalar(float32(c.posS))

				// c.of_d.ApplyQuaternion(&c.orin)
			}

			if data["P"] != nil {
				for i := 0; i < 6*6; i++ {
					c.P[i] = float32(data["P"].([]interface{})[i].(float64))
				}
				// fmt.Printf("P: %+v\n", c.P)
			}

			if data["f"] != nil {
				for i := 0; i < 6; i++ {
					c.f[i] = float32(data["f"].([]interface{})[i].(float64))
				}
				// fmt.Printf("f: %+v\n", c.f)
			}

			if data["K"] != nil {
				for i := 0; i < 3*6; i++ {
					c.K[i] = float32(data["K"].([]interface{})[i].(float64))
				}
				// fmt.Printf("K: %+v\n", c.K)
			}

			if data["y-h"] != nil {
				for i := 0; i < 3; i++ {
					c.yh[i] = float32(data["y-h"].([]interface{})[i].(float64))
				}
				// fmt.Printf("yh: %+v\n", c.yh)
			}

			// Ensure history buffers are initialized
			if len(c.lin_accel_a) == 0 {
				c.lin_accel_a = make([]math32.Vector3, c.historySize)
			}
			if len(c.orin_e_a) == 0 {
				c.orin_e_a = make([]math32.Vector3, c.historySize)
			}
			if len(c.of_d_a) == 0 {
				c.of_d_a = make([]math32.Vector3, c.historySize)
			}
			if len(c.x_pos_a) == 0 {
				c.x_pos_a = make([]math32.Vector3, c.historySize)
			}

			// Shift history buffers back by one
			for i := len(c.lin_accel_a) - 1; i > 0; i-- {
				c.lin_accel_a[i] = c.lin_accel_a[i-1]
			}
			c.lin_accel_a[0] = c.lin_accel

			for i := len(c.orin_e_a) - 1; i > 0; i-- {
				c.orin_e_a[i] = c.orin_e_a[i-1]
			}
			c.orin_e_a[0] = c.orin_e

			for i := len(c.of_d_a) - 1; i > 0; i-- {
				c.of_d_a[i] = c.of_d_a[i-1]
			}
			c.of_d_a[0] = c.of_d

			for i := len(c.x_pos_a) - 1; i > 0; i-- {
				c.x_pos_a[i] = c.x_pos_a[i-1]
			}
			c.x_pos_a[0] = c.x_pos

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
