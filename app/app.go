package app

import (
	"fmt"
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
	targetFPS   = 120
	historySize = 10 * 200
	posScale    = 100 // m to cm
)

//todo: move all of the stack-initialsied members to class properties for global access
//todo: colour history particles based on the velocity of the device

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
	histShow int

	graphs_tb_l      *gui.Label
	graphs_tb        *gui.TabBar
	graphs_accel_tab *gui.Tab
	graph_imu_accel  *gui.Chart
	graphs_orio_tab  *gui.Tab
	graph_imu_orio   *gui.Chart
	graphs_of_tab    *gui.Tab
	graph_of_delta   *gui.Chart

	lacel_x *gui.Graph
	lacel_y *gui.Graph
	lacel_z *gui.Graph
	orio_x  *gui.Graph
	orio_y  *gui.Graph
	orio_z  *gui.Graph
	of_x    *gui.Graph
	of_y    *gui.Graph
	of_z    *gui.Graph

	kalman_tb_l *gui.Label
	// kalman_p_s2 *gui.Splitter
	// kalman_p_s  *gui.Splitter
	kalman_p_t1 *gui.Tree

	k_state_n  *gui.TreeNode
	k_state_tb *gui.Table

	// kalman_p_t2 *gui.Tree

	k_pc_n  *gui.TreeNode
	k_pc_tb *gui.Table

	k_oc_n  *gui.TreeNode
	k_oc_tb *gui.Table

	k_K_n  *gui.TreeNode
	k_K_tb *gui.Table

	k_yh_n  *gui.TreeNode
	k_yh_tb *gui.Table

	// k_tabs       *gui.TabBar
	// k_tabs_st_tb *gui.Tab
	// k_tabs_ci_tb *gui.Tab

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
	con                *Connector
	pos_offset         math32.Vector3
	pos_offset_readout []float32
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
	a.pos_offset_readout = make([]float32, 3)

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
	a.Subscribe(window.OnKeyDown, a.onKey)
	a.Subscribe(window.OnKeyUp, a.onKey)

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

func (a *App) onKey(evname string, ev interface{}) {
	kev := ev.(*window.KeyEvent)
	switch kev.Key {
	case window.KeyF5:
		a.pos_offset = *a.con.x_pos.MultiplyScalar(-1)
		// a.pos_offset_readout[0] = a.con.x_pos.X / float32(a.con.posS)
		// a.pos_offset_readout[1] = a.con.x_pos.Y / float32(a.con.posS)
		// a.pos_offset_readout[2] = a.con.x_pos.Z / float32(a.con.posS)
		// fmt.Printf("x_pos: %v\n", a.con.x_pos)
		// fmt.Printf("Pos offset: %v\n", a.pos_offset.MultiplyScalar(-1/float32(a.con.posS)))
		// panic("exit")
		// pos -0.3 -1.2 -1.4
		// offset -1.3 -1.4 -0.3
		// pos_offset_scaled := a.pos_offset.MultiplyScalar(-1 / float32(a.con.posS))
		a.pos_offset_readout[0] = -1 * a.con.x[0]
		a.pos_offset_readout[1] = -1 * a.con.x[1]
		a.pos_offset_readout[2] = -1 * a.con.x[2]

		for i := range a.trail_s {
			a.trail_s[i].SetPosition(0, 0, 0)
		}
	}
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
	a.vdisk.SetCullable(false)
	// a.vdisk.SetRotation(-math32.Pi/2, 0, 0)
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
	a.trail_s = make([]*graphic.Sprite, historySize)
	for i := 0; i < historySize; i++ {
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
		a.srm.SetSize(float32(width)-float32(width)*0.35-10, float32(height)*0.2-a.srm_l.Height()-2)
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
	a.con.setUpdateGraphsFunc(a.updateGraphs, historySize, posScale)

	// Graph & Table sidebar
	a.sidebar = gui.NewPanel(float32(width)*0.35, float32(height))
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
			// Auto-Connect
			time.Sleep(200 * time.Millisecond)
			a.con.ConnectPort(p)
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
	a.histShow = historySize
	a.trail_sl.SetText(fmt.Sprintf("%d frames", int(a.trail_sl.Value()*historySize)))
	a.trail_sl.Subscribe(gui.OnChange, func(evname string, ev interface{}) {
		// process change
		a.histShow = int(a.trail_sl.Value() * historySize)
		a.trail_sl.SetText(fmt.Sprintf("%d frames", a.histShow))
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
	a.graph_of_delta.SetFormatY("%2.1f")
	a.graph_of_delta.SetRangeYauto(true)
	a.graphs_of_tab.SetContent(a.graph_of_delta)

	// Kalman parameters viewer
	//todo: dont use tabs but show everything at once? nested panels?

	a.kalman_tb_l = gui.NewLabel("Kalman Parameters:")
	a.sidebar.Add(a.kalman_tb_l)

	// Sidebar and First column
	sidebar_r_height := a.mainPanel.Height() - a.kalman_tb_l.Position().Y
	// a.kalman_p_s = gui.NewHSplitter(a.sidebar.Width()-16, sidebar_r_height*0.6)
	// a.kalman_p_s.SetSplit(0.4)
	// a.kalman_p_s.P0.SetBorders(0, 1, 0, 0)

	// a.kalman_p_s2 = gui.NewVSplitter(a.sidebar.Width()-16, sidebar_r_height)
	// a.kalman_p_s2.SetSplit(0.6)
	// a.sidebar.Add(a.kalman_p_s2)
	// a.kalman_p_s2.P0.Add(a.kalman_p_s)
	// sidebar.Add(kalman_p_s)

	// Kalman State
	// a.kalman_p_t1 = gui.NewTree(a.kalman_p_s.P0.ContentWidth(), sidebar_r_height*0.6)
	a.kalman_p_t1 = gui.NewTree(a.sidebar.Width()-16, sidebar_r_height*1.0)
	// a.kalman_p_s.P0.Add(a.kalman_p_t1)
	a.sidebar.Add(a.kalman_p_t1)

	a.k_state_n = a.kalman_p_t1.AddNode("State (x)")
	var err error
	// a.k_state_tb, err = gui.NewTable(a.kalman_p_s.P0.ContentWidth(), 32*6, []gui.TableColumn{
	a.k_state_tb, err = gui.NewTable(a.kalman_p_t1.ContentWidth(), 24*7, []gui.TableColumn{
		{Id: "1", Header: "x", Width: 64, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.3f", Expand: 0, Resize: false},
		{Id: "2", Header: "param", Width: 64, Minwidth: 48, Align: gui.AlignLeft, Format: "%s", Expand: 1, Resize: false},
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
		rval["1"] = float32(-1.0)
		rval["2"] = state_params[i]
		state_vals = append(state_vals, rval)
	}
	a.k_state_tb.SetRows(state_vals)
	a.k_state_n.Add(a.k_state_tb)
	a.k_state_n.SetExpanded(true)

	// Second Row
	/*
		// a.kalman_p_t2 = gui.NewTree(a.kalman_p_s.P1.ContentWidth(), sidebar_r_height*0.6)
		a.kalman_p_t2 = gui.NewTree(a.sidebar.Width()-16, sidebar_r_height*0.6)
		// a.kalman_p_s.P0.Add(a.kalman_p_t2)
		a.sidebar.Add(a.kalman_p_t2)
	*/

	// Kalman Proccess Covariance
	a.k_pc_n = a.kalman_p_t1.AddNode("State Error Covariance (P)")
	// a.k_pc_tb, err = gui.NewTable(a.kalman_p_s.P1.ContentWidth(), 24*3, []gui.TableColumn{
	a.k_pc_tb, err = gui.NewTable(a.kalman_p_t1.ContentWidth(), 24*7, []gui.TableColumn{
		{Id: "1", Width: 64 + 8, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.7f", Expand: 0, Resize: false},
		{Id: "2", Width: 64 + 8, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.7f", Expand: 0, Resize: false},
		{Id: "3", Width: 64 + 8, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.7f", Expand: 0, Resize: false},
		{Id: "4", Width: 64 + 8, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.7f", Expand: 0, Resize: false},
		{Id: "5", Width: 64 + 8, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.7f", Expand: 0, Resize: false},
		{Id: "6", Width: 64 + 8, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.7f", Expand: 0, Resize: false},
	})
	if err != nil {
		panic(err)
	}
	k_pc_vals := make([]map[string]interface{}, 0, 6)
	for i := 0; i < 6; i++ {
		rval := make(map[string]interface{})
		for j := 0; j < 6; j++ {
			rval[fmt.Sprintf("%d", j+1)] = float32(-1.0)
		}
		k_pc_vals = append(k_pc_vals, rval)
	}
	a.k_pc_tb.SetRows(k_pc_vals)
	a.k_pc_tb.ShowHeader(false)
	a.k_pc_n.Add(a.k_pc_tb)
	a.k_pc_n.SetExpanded(true)

	// Kalman State Transisiton
	a.k_oc_n = a.kalman_p_t1.AddNode("State Transition (f)")
	// a.k_oc_tb, err = gui.NewTable(a.kalman_p_s.P1.ContentWidth(), 24*3, []gui.TableColumn{
	a.k_oc_tb, err = gui.NewTable(a.kalman_p_t1.ContentWidth(), 24*7, []gui.TableColumn{
		{Id: "1", Width: 64, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.6f", Expand: 0, Resize: false},
	})
	if err != nil {
		panic(err)
	}
	k_oc_vals := make([]map[string]interface{}, 0, 6)
	for i := 0; i < 6; i++ {
		rval := make(map[string]interface{})
		rval["1"] = float32(-1.0)
		k_oc_vals = append(k_oc_vals, rval)
	}
	a.k_oc_tb.SetRows(k_oc_vals)
	a.k_oc_tb.ShowHeader(false)
	a.k_oc_n.Add(a.k_oc_tb)
	a.k_oc_n.SetExpanded(true)

	// Kalman Kalman Gain
	a.k_K_n = a.kalman_p_t1.AddNode("Kalman Gain (K)")
	a.k_K_tb, err = gui.NewTable(a.kalman_p_t1.ContentWidth(), 24*4, []gui.TableColumn{
		{Id: "1", Width: 64, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.6f", Expand: 0, Resize: false},
		{Id: "2", Width: 64, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.6f", Expand: 0, Resize: false},
		{Id: "3", Width: 64, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.6f", Expand: 0, Resize: false},
		{Id: "4", Width: 64, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.6f", Expand: 0, Resize: false},
		{Id: "5", Width: 64, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.6f", Expand: 0, Resize: false},
		{Id: "6", Width: 64, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.6f", Expand: 0, Resize: false},
	})
	if err != nil {
		panic(err)
	}
	k_K_vals := make([]map[string]interface{}, 0, 3)
	for i := 0; i < 3; i++ {
		rval := make(map[string]interface{})
		for j := 0; j < 6; j++ {
			rval[fmt.Sprintf("%d", j+1)] = float32(-1.0)
		}
		k_K_vals = append(k_K_vals, rval)
	}
	a.k_K_tb.SetRows(k_K_vals)
	a.k_K_tb.ShowHeader(false)
	a.k_K_n.Add(a.k_K_tb)
	a.k_K_n.SetExpanded(true)

	// Kalman Innovation
	a.k_yh_n = a.kalman_p_t1.AddNode("Innovation (y-h)")
	a.k_yh_tb, err = gui.NewTable(a.kalman_p_t1.ContentWidth(), 24*4, []gui.TableColumn{
		{Id: "1", Width: 64, Minwidth: 48, Align: gui.AlignLeft, Format: "%3.6f", Expand: 0, Resize: false},
	})
	if err != nil {
		panic(err)
	}
	k_yh_vals := make([]map[string]interface{}, 0, 3)
	for i := 0; i < 3; i++ {
		rval := make(map[string]interface{})
		rval["1"] = float32(-1.0)
		k_yh_vals = append(k_yh_vals, rval)
	}
	a.k_yh_tb.SetRows(k_yh_vals)
	a.k_yh_tb.ShowHeader(false)
	a.k_yh_n.Add(a.k_yh_tb)
	a.k_yh_n.SetExpanded(true)

	/*
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
	*/

	// FPS label
	a.labelFPS = gui.NewLabel("FPS: 000.0")
	a.labelFPS.SetColor(&math32.Color{R: 1, G: 1, B: 1})
	a.mainPanel.Add(a.labelFPS)

	// Return focus to viz scene when leaving GUI
	a.mainPanel.Subscribe(gui.OnCursorLeave, func(name string, ev interface{}) {
		gui.Manager().SetKeyFocus(nil)
	})
}

func (a *App) updateGraphs() {
	if !a.con.rso {
		// Linear Acceleration
		if a.lacel_x != nil {
			a.graph_imu_accel.RemoveGraph(a.lacel_x)
			a.lacel_x = nil
		}
		if a.lacel_y != nil {
			a.graph_imu_accel.RemoveGraph(a.lacel_y)
			a.lacel_y = nil
		}
		if a.lacel_z != nil {
			a.graph_imu_accel.RemoveGraph(a.lacel_z)
			a.lacel_z = nil
		}
		var lacel_x_d, lacel_y_d, lacel_z_d []float32
		for i := 0; i < len(a.con.lin_accel_a); i++ {
			lacel_x_d = append(lacel_x_d, a.con.lin_accel_a[i].X)
			lacel_y_d = append(lacel_y_d, a.con.lin_accel_a[i].Y)
			lacel_z_d = append(lacel_z_d, a.con.lin_accel_a[i].Z)
			if i == a.histShow {
				break
			}
		}
		a.lacel_x = a.graph_imu_accel.AddLineGraph(&math32.Color{R: 1, G: 0, B: 0}, lacel_x_d)
		a.lacel_y = a.graph_imu_accel.AddLineGraph(&math32.Color{R: 0, G: 1, B: 0}, lacel_y_d)
		a.lacel_z = a.graph_imu_accel.AddLineGraph(&math32.Color{R: 0, G: 0, B: 1}, lacel_z_d)

		// Orientation
		if a.orio_x != nil {
			a.graph_imu_orio.RemoveGraph(a.orio_x)
			a.orio_x = nil
		}
		if a.orio_y != nil {
			a.graph_imu_orio.RemoveGraph(a.orio_y)
			a.orio_y = nil
		}
		if a.orio_z != nil {
			a.graph_imu_orio.RemoveGraph(a.orio_z)
			a.orio_z = nil
		}
		var orio_x_d, orio_y_d, orio_z_d []float32
		for i := 0; i < len(a.con.orin_e_a); i++ {
			orio_x_d = append(orio_x_d, a.con.orin_e_a[i].X)
			orio_y_d = append(orio_y_d, a.con.orin_e_a[i].Z) // swap Y and Z
			orio_z_d = append(orio_z_d, a.con.orin_e_a[i].Y)
			if i == a.histShow {
				break
			}
		}
		a.orio_x = a.graph_imu_orio.AddLineGraph(&math32.Color{R: 1, G: 0, B: 0}, orio_x_d)
		a.orio_y = a.graph_imu_orio.AddLineGraph(&math32.Color{R: 0, G: 1, B: 0}, orio_y_d)
		a.orio_z = a.graph_imu_orio.AddLineGraph(&math32.Color{R: 0, G: 0, B: 1}, orio_z_d)

		// Optical Flow
		if a.of_x != nil {
			a.graph_of_delta.RemoveGraph(a.of_x)
			a.of_x = nil
		}
		if a.of_y != nil {
			a.graph_of_delta.RemoveGraph(a.of_y)
			a.of_y = nil
		}
		if a.of_z != nil {
			a.graph_of_delta.RemoveGraph(a.of_z)
			a.of_z = nil
		}
		var of_x_d, of_y_d, of_z_d []float32
		for i := 0; i < len(a.con.of_d_a); i++ {
			of_x_d = append(of_x_d, a.con.of_d_a[i].X)
			of_y_d = append(of_y_d, a.con.of_d_a[i].Y)
			of_z_d = append(of_z_d, a.con.of_d_a[i].Z)
			if i == a.histShow {
				break
			}
		}
		a.of_x = a.graph_of_delta.AddLineGraph(&math32.Color{R: 1, G: 0, B: 0}, of_x_d)
		a.of_y = a.graph_of_delta.AddLineGraph(&math32.Color{R: 0, G: 1, B: 0}, of_y_d)
		a.of_z = a.graph_of_delta.AddLineGraph(&math32.Color{R: 0, G: 0, B: 1}, of_z_d)

		// State Table
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
			// rval["1"] = a.con.x[i]
			// add in offset and set 1st column

			if i < 3 {
				rval["1"] = a.con.x[i] + a.pos_offset_readout[i]
			} else {
				rval["1"] = a.con.x[i]
			}

			rval["2"] = state_params[i]
			state_vals = append(state_vals, rval)

		}
		// state_vals[0]["1"] = a.con.x[0] + a.pos_offset.X
		// state_vals[1]["1"] = a.con.x[1] + a.pos_offset.Y
		// state_vals[2]["1"] = a.con.x[2] + a.pos_offset.Z
		a.k_state_tb.SetRows(state_vals)

		// State Covariance Table
		k_pc_vals := make([]map[string]interface{}, 0, 6)
		for i := 0; i < 6; i++ {
			rval := make(map[string]interface{})
			for j := 0; j < 6; j++ {
				rval[fmt.Sprintf("%d", j+1)] = a.con.P[i*6+j]
			}
			k_pc_vals = append(k_pc_vals, rval)
		}
		a.k_pc_tb.SetRows(k_pc_vals)

		// State Transition Table
		k_oc_vals := make([]map[string]interface{}, 0, 6)
		for i := 0; i < 6; i++ {
			rval := make(map[string]interface{})
			rval["1"] = a.con.f[i]
			k_oc_vals = append(k_oc_vals, rval)
		}
		a.k_oc_tb.SetRows(k_oc_vals)

		// Kalman Gain Table
		k_K_vals := make([]map[string]interface{}, 0, 3)
		for i := 0; i < 3; i++ {
			rval := make(map[string]interface{})
			for j := 0; j < 6; j++ {
				rval[fmt.Sprintf("%d", j+1)] = a.con.K[i*6+j]
			}
			k_K_vals = append(k_K_vals, rval)
		}
		a.k_K_tb.SetRows(k_K_vals)

		// Innovation Table
		k_yh_vals := make([]map[string]interface{}, 0, 3)
		for i := 0; i < 3; i++ {
			rval := make(map[string]interface{})
			rval["1"] = a.con.yh[i]
			k_yh_vals = append(k_yh_vals, rval)
		}
		a.k_yh_tb.SetRows(k_yh_vals)

	}
}

func (a *App) Run() {
	a.Application.Run(a.Update)
}

func (a *App) Update(rend *renderer.Renderer, deltaTime time.Duration) {
	// Start measuring this frame
	a.frameRater.Start()

	a.updateGraphs()

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
	/*
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
	*/

	// If we are connected to the device, update the 3D scene
	if !a.con.rso {
		// Get the current position, subtracting the zero point offset
		currentPos := a.con.x_pos.Add(&a.pos_offset)
		// Set the position and orientation of the device visual
		a.vdisk.SetRotationQuat(&a.con.orin)
		a.vdisk.SetPositionVec(currentPos)

		// var x, z float32
		// x, z = 0.0, 0.0
		// Update the trail sprites, moving them back according to the history buffer
		for i := len(a.trail_s) - 1; i > 0; i-- {
			// Retrieve and set the position
			pos := a.trail_s[i-1].Position()
			a.trail_s[i].SetPositionVec(&pos)
			// Retrieve and set the rotation
			rot := a.trail_s[i-1].Rotation()
			a.trail_s[i].SetRotationVec(&rot)

			// Hide sprites older than the specified history size
			if i <= a.histShow {
				a.trail_s[i].SetVisible(true)
			} else {
				a.trail_s[i].SetVisible(false)
			}
		}
		// a.trail_s[0].SetPosition(x, 0, z)
		// a.trail_s[0].RotateZ(0.01)

		// Set the position and rotation of the current position trail sprite
		a.trail_s[0].SetPositionVec(currentPos)
		a.trail_s[0].SetRotationQuat(&a.con.orin)

		// Update the tail sprite colours based on velocity
		/*
			for i := 0; i < len(a.trail_s)-1; i++ {
				// Calculate velocity
				currentPos := a.trail_s[i].Position()
				nextPos := a.trail_s[i+1].Position()
				delta := math32.Abs(currentPos.DistanceTo(&nextPos))
				vel := delta / float32(1.0/100)
				fmt.Printf("delta: %v, vel: %v\n", delta, vel)
				// Set the colour based on velocity
				maxVel := 10
				velScale := vel / float32(maxVel)
				if velScale > 1.0 {
					velScale = 1.0
				}
				// fmt.Printf("velScale: %v\n", velScale)
				r, g, b, _ := colorconv.HSLToRGB(float64(velScale), 1.0, 1.0)
				// fmt.Printf("r: %v, g: %v, b: %v\n", r, g, b)

				// a.trail_s[i].GetMaterial(0).Dispose()
				a.trail_s[i].AddMaterial(a.trail_s[i], material.NewStandard(&math32.Color{R: float32(r / 255.0), G: float32(g / 255.0), B: float32(b / 255.0)}), 0, 0)
			}
		*/

	}
}
