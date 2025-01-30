package app

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/g3n/engine/app"
	"github.com/g3n/engine/camera"
	"github.com/g3n/engine/core"
	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/gls"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/light"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/g3n/engine/renderer"
	"github.com/g3n/engine/util"
	"github.com/g3n/engine/util/helper"
	"github.com/g3n/engine/window"
)

const (
	targetFPS = 60
)

//todo: move all of the stack-initialsied members to class properties for global access

type App struct {
	*app.Application
	scene *core.Node

	// GUI
	mainPanel *gui.Panel

	footer *gui.Panel
	srm_l  *gui.Label
	srm    *gui.ItemScroller

	sidebar *gui.Panel

	serial_p   *gui.Panel
	serial_l   *gui.Label
	serial_dd  *gui.DropDown
	serial_btn *gui.Button

	trail_p  *gui.Panel
	trail_l  *gui.Label
	trail_sl *gui.Slider

	graphs_tb_l      *gui.Label
	graphs_tb        *gui.TabBar
	graphs_accel_tab *gui.Tab
	graph_imu_accel  *gui.Chart
	graphs_orio_tab  *gui.Tab
	graph_imu_orio   *gui.Chart
	graphs_of_tab    *gui.Tab
	graph_of_delta   *gui.Chart

	kalman_tb_l  *gui.Label
	kalman_p_s2  *gui.Splitter
	kalman_p_s   *gui.Splitter
	kalman_p_t1  *gui.Tree
	k_state_n    *gui.TreeNode
	k_state_tb   *gui.Table
	kalman_p_t2  *gui.Tree
	k_pc_n       *gui.TreeNode
	k_pc_tb      *gui.Table
	k_oc_n       *gui.TreeNode
	k_oc_tb      *gui.Table
	k_tabs       *gui.TabBar
	k_tabs_st_tb *gui.Tab
	k_tabs_ci_tb *gui.Tab

	// Scene
	camera *camera.Camera
	orbit  *camera.OrbitControl

	vdisk_g *geometry.Geometry
	vcube_g *geometry.Geometry
	mat1    *material.Standard
	mat2    *material.Standard
	vdisk   *graphic.Mesh
	vcube   *graphic.Mesh

	trail_m *material.Standard
	trail_s []*graphic.Sprite

	frameRater *util.FrameRater
	labelFPS   *gui.Label
	t          time.Duration

	// HAL Connector
	con *Connector
}

func Create() *App {
	a := new(App)
	a.Application = app.App()
	fmt.Println("Starting OF_IMU_LocationCore-Viz...")

	// Log OpenGL version
	glVersion := a.Gls().GetString(gls.VERSION)
	fmt.Printf("OpenGL ver: %s\n", glVersion)

	// HAL Connector
	a.con = new(Connector)

	// Create scenes
	a.scene = core.NewNode()

	// Create camera & orbit control
	width, height := a.GetSize()
	aspect := float32(width) / float32(height)
	a.camera = camera.New(aspect)
	// a.camera.SetPosition(5.557, 3.657, 1.824)
	a.camera.SetPositionVec(&math32.Vector3{X: 5.5, Y: 3.0, Z: 1.5})
	a.camera.LookAt(&math32.Vector3{X: 0, Y: 0, Z: 0}, &math32.Vector3{X: 0, Y: 1.0, Z: 0})
	// a.camera.SetRotationVec(&math32.Vector3{X: -0.644, Y: 0.531, Z: 0.365})
	// a.camera.SetProjection(camera.Orthographic)
	a.scene.Add(a.camera)
	a.orbit = camera.NewOrbitControl(a.camera)

	// Create frame rater
	a.frameRater = util.NewFrameRater(targetFPS)

	// Build user interface
	a.buildGUI()

	// Create perspective selector
	pers := gui.NewCheckBox("Orthographic")
	pers.SetValue(false)
	pers.SetEnabled(true)
	pers.Subscribe(gui.OnChange, func(evname string, ev interface{}) {
		if pers.Value() {
			a.camera.SetProjection(camera.Orthographic)
		} else {
			a.camera.SetProjection(camera.Perspective)
		}
	})
	a.mainPanel.Add(pers)
	pers.SetPosition(0, 16)

	// window resize handler
	a.Subscribe(window.OnWindowSize, func(evname string, ev interface{}) {
		a.OnWindowResize()
	})
	a.OnWindowResize()

	// key events

	// Setup scene
	a.setupScene()

	return a
}

func (a *App) OnWindowResize() {
	width, height := a.GetFramebufferSize()
	a.Gls().Viewport(0, 0, int32(width), int32(height))
	a.camera.SetAspect(float32(width) / float32(height))

	a.mainPanel.SetSize(float32(width), float32(height))
}

func (a *App) setupScene() {
	// Set background color
	a.Gls().ClearColor(0.05, 0.05, 0.05, 1.0)

	// Ambient light
	ambientLight := light.NewAmbient(&math32.Color{R: 1.0, G: 1.0, B: 1.0}, 0.8)
	a.scene.Add(ambientLight)

	// Helper axis
	haxis := helper.NewAxes(1)
	a.scene.Add(haxis)

	// Helper grid
	hgrid := helper.NewGrid(20, 1, &math32.Color{R: 0.5, G: 0.5, B: 0.5})
	a.scene.Add(hgrid)

	// Create a disk geometry
	a.vdisk_g = geometry.NewDisk(1, 3)
	a.mat1 = material.NewStandard(&math32.Color{R: 1, G: 0, B: 1})
	a.mat1.SetWireframe(true)
	a.mat1.SetLineWidth(2)
	a.vdisk = graphic.NewMesh(a.vdisk_g, a.mat1)
	a.vdisk.SetRotation(-math32.Pi/2, 0, 0)
	a.scene.Add(a.vdisk)

	// Create a cube geometry
	a.vcube_g = geometry.NewCube(1)
	a.mat2 = material.NewStandard(&math32.Color{R: 1, G: 1, B: 0})
	a.mat2.SetWireframe(true)
	a.mat2.SetLineWidth(2)
	a.vcube = graphic.NewMesh(a.vcube_g, a.mat2)
	a.vcube.SetScale(0.5, 0.5, 0.5)
	a.vcube.SetPosition(0, 0, 0.25)
	a.vdisk.Add(a.vcube)

	// Create a trail sprites
	a.trail_m = material.NewStandard(&math32.Color{R: 0, G: 1, B: 1})
	a.trail_m.SetTransparent(true)
	a.trail_m.SetOpacity(0.5)
	// a.trail_m.SetEmissiveColor(&math32.Color{R: 0, G: 1, B: 1})
	a.trail_s = make([]*graphic.Sprite, 100)
	for i := 0; i < 100; i++ {
		a.trail_s[i] = graphic.NewSprite(0.2, 0.1, a.trail_m)
		a.trail_s[i].SetPosition(0, 0, 0)
		a.scene.Add(a.trail_s[i])
	}

}

func (a *App) buildGUI() {
	// Create dock layout
	dl := gui.NewDockLayout()
	width, height := a.GetSize()
	a.mainPanel = gui.NewPanel(float32(width), float32(height))
	a.mainPanel.SetRenderable(false)
	a.mainPanel.SetEnabled(false)
	a.mainPanel.SetLayout(dl)
	a.scene.Add(a.mainPanel)
	gui.Manager().Set(a.mainPanel)

	// Serial Monitor Footer
	a.footer = gui.NewPanel(float32(width)-float32(width)*0.3-10, float32(height)*0.2)
	a.footer.SetBorders(1, 0, 0, 0)
	a.footer.SetPaddings(2, 2, 2, 2)
	a.footer.SetColor4(&math32.Color4{R: 0.25, G: 0.25, B: 0.25, A: 1.0})
	a.footer.SetLayoutParams(&gui.DockLayoutParams{Edge: gui.DockBottom})
	footer_vb := gui.NewVBoxLayout()
	a.footer.SetLayout(footer_vb)
	a.mainPanel.Add(a.footer)

	a.srm_l = gui.NewLabel("Serial Monitor:")
	a.footer.Add(a.srm_l)
	a.srm = gui.NewVScroller(float32(width)-float32(width)*0.3-10, float32(height)*0.2-a.srm_l.Height()-2)
	a.mainPanel.SubscribeID(gui.OnResize, a, func(evname string, ev interface{}) {
		width, height := a.GetSize()
		a.srm.SetSize(float32(width)-float32(width)*0.3-10, float32(height)*0.2-a.srm_l.Height()-2)
	})
	a.srm.SetColor(&math32.Color{R: 0, G: 0, B: 0})
	a.srm.SetPaddings(0, 5, 0, 5)
	a.srm.SetPaddings(0, 5, 0, 5)
	a.srm.Add(gui.NewLabel("Waiting for serial connection..."))
	a.srm.Subscribe(gui.OnCursorEnter, func(evname string, ev interface{}) {
		a.srm.SetColor(&math32.Color{R: 0, G: 0, B: 0})
		a.srm.ScrollDown()
	})
	a.srm.Subscribe(gui.OnCursorLeave, func(evname string, ev interface{}) {
		a.srm.SetColor(&math32.Color{R: 0, G: 0, B: 0})
		a.srm.ScrollDown()
	})
	a.footer.Add(a.srm)

	//? link!
	a.con.setScroller(a.srm)

	// Graph & Table sidebar
	a.sidebar = gui.NewPanel(float32(width)*0.3, float32(height))
	a.sidebar.SetBorders(0, 0, 0, 1)
	a.sidebar.SetPaddings(2, 2, 2, 2)
	a.sidebar.SetColor4(&math32.Color4{R: 0.25, G: 0.25, B: 0.25, A: 1.0})
	a.sidebar.SetLayoutParams(&gui.DockLayoutParams{Edge: gui.DockRight})
	a.mainPanel.Add(a.sidebar)

	sidebar_v := gui.NewVBoxLayout()
	sidebar_v.SetSpacing(5)
	sidebar_v.SetAutoHeight(true)
	sidebar_v.SetAutoWidth(false)
	sidebar_v.SetAlignV(gui.AlignTop)
	a.sidebar.SetLayout(sidebar_v)

	// Serial port selector
	serial_hb := gui.NewHBoxLayout()
	serial_hb.SetAlignH(gui.AlignLeft)
	serial_hb.SetAutoWidth(false)
	serial_hb.SetSpacing(5)
	a.serial_p = gui.NewPanel(a.sidebar.Width(), 18)
	a.serial_p.SetLayout(serial_hb)
	a.sidebar.Add(a.serial_p)
	a.serial_l = gui.NewLabel("Serial Port: ")
	a.serial_p.Add(a.serial_l)
	a.serial_dd = gui.NewDropDown(a.serial_p.Width(), gui.NewImageLabel("Scanning..."))
	a.serial_p.Add(a.serial_dd)
	a.serial_btn = gui.NewButton("Connect")
	a.serial_btn.SetHeight(a.serial_dd.Height())
	a.serial_dd.SetWidth(a.serial_p.Width() - a.serial_l.Width() - a.serial_btn.Width() - 18)
	a.serial_p.Add(a.serial_btn)
	a.serial_p.SetHeight(a.serial_dd.Height())
	// Refresh ports
	go func() {
		time.Sleep(1 * time.Second)
		ports := a.con.GetPorts()
		// a.serial_dd.DisposeChildren(false)
		for i, p := range ports {
			a.serial_dd.Add(gui.NewImageLabel(p))
			a.srm.Add(gui.NewImageLabel("Found serial port: " + p))
			a.serial_dd.SetSelected(a.serial_dd.ItemAt(i))
		}
	}()
	// Set port
	a.serial_btn.Subscribe(gui.OnClick, func(evname string, ev interface{}) {
		port := a.serial_dd.Selected()
		if port == nil {
			return
		}
		a.con.ConnectPort(port.Text())
	})

	// Trail slider
	trail_hb := gui.NewHBoxLayout()
	trail_hb.SetAlignH(gui.AlignLeft)
	trail_hb.SetAutoWidth(false)
	trail_hb.SetSpacing(5)
	a.trail_p = gui.NewPanel(a.sidebar.Width(), 16)
	a.trail_p.SetLayout(trail_hb)
	a.sidebar.Add(a.trail_p)
	a.trail_l = gui.NewLabel("Trail Length: ")
	a.trail_p.Add(a.trail_l)
	a.trail_sl = gui.NewHSlider(a.sidebar.Width()-a.trail_l.Width()-15, a.trail_l.Height())
	a.trail_sl.SetValue(1)
	a.trail_sl.SetText(fmt.Sprintf("%d frames", int(a.trail_sl.Value()*100)))
	a.trail_sl.Subscribe(gui.OnChange, func(evname string, ev interface{}) {
		// process change
		a.trail_sl.SetText(fmt.Sprintf("%d frames", int(a.trail_sl.Value()*100)))
	})
	a.trail_p.Add(a.trail_sl)
	a.trail_p.SetHeight(a.trail_sl.Height())

	// Graphs
	a.graphs_tb_l = gui.NewLabel("Sensor Data: ")
	a.sidebar.Add(a.graphs_tb_l)
	a.graphs_tb = gui.NewTabBar(a.sidebar.Width()-4, a.graphs_tb_l.Height()*12)
	a.graphs_tb.SetPaddings(0, 2, 0, 2)
	a.graphs_tb.SetMargins(0, 2, 0, 2)
	a.sidebar.Add(a.graphs_tb)
	a.mainPanel.SubscribeID(gui.OnResize, a, func(evname string, ev interface{}) {
		a.graphs_tb.SetSize(a.sidebar.Width()-4, a.graphs_tb_l.Height()*12)
	})

	// accel graph
	a.graphs_accel_tab = a.graphs_tb.AddTab("Linear Accel.")
	a.graphs_accel_tab.SetPinned(true)

	a.graph_imu_accel = gui.NewChart(a.sidebar.Width()-16, a.graphs_tb_l.Height()*12)
	a.graph_imu_accel.SetMargins(0, 2, 0, 2)
	a.graph_imu_accel.SetBorders(2, 2, 2, 2)
	a.graph_imu_accel.SetBordersColor(math32.NewColor("black"))
	a.graph_imu_accel.SetPaddings(0, 2, 0, 2)
	a.graph_imu_accel.SetColor(math32.NewColor("white"))
	a.graph_imu_accel.SetRangeY(-2, 2) //todo: set this from device config
	a.graph_imu_accel.SetScaleY(11, &math32.Color{R: 0.8, G: 0.8, B: 0.8})
	a.graph_imu_accel.SetFontSizeX(12)
	a.graph_imu_accel.SetFormatY("%2.1f")
	a.graphs_accel_tab.SetContent(a.graph_imu_accel)

	// orientation graph
	a.graphs_orio_tab = a.graphs_tb.AddTab("Orientation")
	a.graphs_orio_tab.SetPinned(true)

	a.graph_imu_orio = gui.NewChart(a.sidebar.Width()-16, a.graphs_tb_l.Height()*12)
	a.graph_imu_orio.SetMargins(0, 2, 0, 2)
	a.graph_imu_orio.SetBorders(2, 2, 2, 2)
	a.graph_imu_orio.SetBordersColor(math32.NewColor("black"))
	a.graph_imu_orio.SetPaddings(0, 2, 0, 2)
	a.graph_imu_orio.SetColor(math32.NewColor("white"))
	a.graph_imu_orio.SetRangeY(-180, 180)
	a.graph_imu_orio.SetScaleY(9, &math32.Color{R: 0.8, G: 0.8, B: 0.8})
	a.graph_imu_orio.SetFontSizeX(12)
	a.graph_imu_orio.SetFormatY("%2.1f")
	a.graphs_orio_tab.SetContent(a.graph_imu_orio)

	// OF graph
	a.graphs_of_tab = a.graphs_tb.AddTab("Optical Flow")
	a.graphs_of_tab.SetPinned(true)

	a.graph_of_delta = gui.NewChart(a.sidebar.Width()-16, a.graphs_tb_l.Height()*12)
	a.graph_of_delta.SetMargins(0, 2, 0, 2)
	a.graph_of_delta.SetBorders(2, 2, 2, 2)
	a.graph_of_delta.SetBordersColor(math32.NewColor("black"))
	a.graph_of_delta.SetPaddings(0, 2, 0, 2)
	a.graph_of_delta.SetColor(math32.NewColor("white"))
	a.graph_of_delta.SetRangeY(-50, 50)
	a.graph_of_delta.SetScaleY(11, &math32.Color{R: 0.8, G: 0.8, B: 0.8})
	a.graph_of_delta.SetFontSizeX(12)
	a.graph_of_delta.SetFormatY("%2.f")
	a.graph_of_delta.SetRangeYauto(true)
	a.graphs_of_tab.SetContent(a.graph_of_delta)

	// Kalman parameters viewer
	//todo: dont use tabs but show everything at once? nested panels?

	a.kalman_tb_l = gui.NewLabel("Kalman Parameters:")
	a.sidebar.Add(a.kalman_tb_l)

	// Sidebar and First column
	sidebar_r_height := a.mainPanel.Height() - a.kalman_tb_l.Position().Y
	a.kalman_p_s = gui.NewHSplitter(a.sidebar.Width()-16, sidebar_r_height*0.6)
	a.kalman_p_s.SetSplit(0.4)
	a.kalman_p_s.P0.SetBorders(0, 1, 0, 0)

	a.kalman_p_s2 = gui.NewVSplitter(a.sidebar.Width()-16, sidebar_r_height)
	a.kalman_p_s2.SetSplit(0.6)
	a.sidebar.Add(a.kalman_p_s2)
	a.kalman_p_s2.P0.Add(a.kalman_p_s)
	// sidebar.Add(kalman_p_s)

	// Kalman State
	a.kalman_p_t1 = gui.NewTree(a.kalman_p_s.P0.ContentWidth(), sidebar_r_height*0.6)
	a.kalman_p_s.P0.Add(a.kalman_p_t1)
	a.k_state_n = a.kalman_p_t1.AddNode("State (x)")
	var err error
	a.k_state_tb, err = gui.NewTable(a.kalman_p_s.P0.ContentWidth(), 32*6, []gui.TableColumn{
		{Id: "1", Header: "x", Width: 48, Minwidth: 32, Align: gui.AlignLeft, Format: "%3.3f", Expand: 0, Resize: false},
		{Id: "2", Header: "param", Width: 48, Minwidth: 32, Align: gui.AlignLeft, Format: "%s", Expand: 1, Resize: false},
	})
	if err != nil {
		panic(err)
	}
	a.k_state_tb.ShowHeader(false)
	state_params :=
		map[int]string{
			0: "Position X",
			1: "Position Y",
			2: "Position Z",
			3: "Velocity X",
			4: "Velocity Y",
			5: "Velocity Z",
		}
	state_vals := make([]map[string]interface{}, 0, 6)
	for i := 0; i < 6; i++ {
		rval := make(map[string]interface{})
		rval["1"] = float32(i * 2)
		rval["2"] = state_params[i]
		state_vals = append(state_vals, rval)
	}
	a.k_state_tb.SetRows(state_vals)
	a.k_state_n.Add(a.k_state_tb)
	a.k_state_n.SetExpanded(true)

	// Second Column
	a.kalman_p_t2 = gui.NewTree(a.kalman_p_s.P1.ContentWidth(), sidebar_r_height*0.6)
	a.kalman_p_s.P1.Add(a.kalman_p_t2)

	// Kalman Proccess Covariance
	a.k_pc_n = a.kalman_p_t2.AddNode("Proccess Covariance (Q)")
	a.k_pc_tb, err = gui.NewTable(a.kalman_p_s.P1.ContentWidth(), 24*3, []gui.TableColumn{
		{Id: "1", Width: 48, Minwidth: 32, Align: gui.AlignLeft, Format: "%3.3f", Expand: 0, Resize: false},
		{Id: "2", Width: 48, Minwidth: 32, Align: gui.AlignLeft, Format: "%3.3f", Expand: 0, Resize: false},
		{Id: "3", Width: 48, Minwidth: 32, Align: gui.AlignLeft, Format: "%3.3f", Expand: 0, Resize: false},
	})
	if err != nil {
		panic(err)
	}
	k_pc_vals := make([]map[string]interface{}, 0, 3)
	for i := 0; i < 3; i++ {
		rval := make(map[string]interface{})
		rval["1"] = rand.Float32()
		rval["2"] = rand.Float32()
		rval["3"] = rand.Float32()
		k_pc_vals = append(k_pc_vals, rval)
	}
	a.k_pc_tb.SetRows(k_pc_vals)
	a.k_pc_tb.ShowHeader(false)
	a.k_pc_n.Add(a.k_pc_tb)
	a.k_pc_n.SetExpanded(true)

	// Kalman Observation Covariance
	a.k_oc_n = a.kalman_p_t2.AddNode("Observation Covariance (R)")
	a.k_oc_tb, err = gui.NewTable(a.kalman_p_s.P1.ContentWidth(), 24*3, []gui.TableColumn{
		{Id: "1", Width: 48, Minwidth: 32, Align: gui.AlignLeft, Format: "%3.3f", Expand: 0, Resize: false},
		{Id: "2", Width: 48, Minwidth: 32, Align: gui.AlignLeft, Format: "%3.3f", Expand: 0, Resize: false},
		{Id: "3", Width: 48, Minwidth: 32, Align: gui.AlignLeft, Format: "%3.3f", Expand: 0, Resize: false},
	})
	if err != nil {
		panic(err)
	}
	k_oc_vals := make([]map[string]interface{}, 0, 3)
	for i := 0; i < 3; i++ {
		rval := make(map[string]interface{})
		rval["1"] = rand.Float32()
		rval["2"] = rand.Float32()
		rval["3"] = rand.Float32()
		k_oc_vals = append(k_oc_vals, rval)
	}
	a.k_oc_tb.SetRows(k_oc_vals)
	a.k_oc_tb.ShowHeader(false)
	a.k_oc_n.Add(a.k_oc_tb)
	a.k_oc_n.SetExpanded(true)

	// Bottom Row fixed matrices
	a.k_tabs = gui.NewTabBar(a.kalman_p_s2.ContentWidth(), a.kalman_p_s2.P1.ContentHeight())
	a.k_tabs.SetPaddings(0, 2, 0, 2)
	a.k_tabs.SetMargins(0, 2, 0, 2)
	a.kalman_p_s2.P1.Add(a.k_tabs)
	a.mainPanel.SubscribeID(gui.OnResize, a, func(evname string, ev interface{}) {
		a.k_tabs.SetSize(a.kalman_p_s2.ContentWidth(), a.kalman_p_s2.P1.ContentHeight())
	})

	// State Transition Matrix (F)
	a.k_tabs_st_tb = a.k_tabs.AddTab("State Transition Matrix (F)")
	a.k_tabs_st_tb.SetPinned(true)

	// Control Input Model (B)
	a.k_tabs_ci_tb = a.k_tabs.AddTab("Control-Input Model (B)")
	a.k_tabs_ci_tb.SetPinned(true)

	// FPS label
	a.labelFPS = gui.NewLabel("FPS: 000.0")
	a.labelFPS.SetColor(&math32.Color{R: 1, G: 1, B: 1})
	a.mainPanel.Add(a.labelFPS)

	// Return focus to viz scene when leaving GUI
	a.mainPanel.Subscribe(gui.OnCursorLeave, func(name string, ev interface{}) {
		gui.Manager().SetKeyFocus(nil)
	})
}

func (a *App) Run() {
	a.Application.Run(a.Update)
}

func (a *App) Update(rend *renderer.Renderer, deltaTime time.Duration) {
	// Start measuring this frame
	a.frameRater.Start()

	// Clear the color, depth and stencil buffers
	a.Gls().Clear(gls.DEPTH_BUFFER_BIT | gls.STENCIL_BUFFER_BIT | gls.COLOR_BUFFER_BIT)

	// Update Viz scene
	a.updateViz(deltaTime)

	// Render scene
	err := rend.Render(a.scene, a.camera)
	if err != nil {
		panic(err)
	}

	// Update GUI timers
	gui.Manager().TimerManager.ProcessTimers()

	// Control and update FPS
	a.frameRater.Wait()
	a.updateFPS()
}

func (a *App) updateFPS() {
	fps, _, ok := a.frameRater.FPS(time.Duration(targetFPS) * time.Millisecond)
	if !ok {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in updateFPS:", r)
		}
	}()
	a.labelFPS.SetText(fmt.Sprintf("Render FPS: %3.1f", fps))
	// fmt.Println(fps)
}

func (a *App) updateViz(deltaTime time.Duration) {
	// Rotate the disk
	a.vdisk.RotateZ(0.01)
	// Move the disk along a circular path
	a.t += deltaTime
	timeElapsed := float64(a.t.Seconds())
	radius := 2.0
	speed := 2.0
	angle := speed * timeElapsed

	x := float32(radius) * math32.Cos(float32(angle))
	z := float32(radius) * math32.Sin(float32(angle))
	a.vdisk.SetPosition(
		x,
		0,
		z,
	)

	// Update the trail sprites
	for i := len(a.trail_s) - 1; i > 0; i-- {
		pos := a.trail_s[i-1].Position()
		a.trail_s[i].SetPositionVec(&pos)
		rot := a.trail_s[i-1].Rotation()
		a.trail_s[i].SetRotationVec(&rot)
	}
	a.trail_s[0].SetPosition(x, 0, z)
	a.trail_s[0].RotateZ(0.01)
}
